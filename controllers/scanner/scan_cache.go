package scanner

import (
	"context"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/whitelist"
)

const scanDisplayTTL = 24 * time.Hour

func scanReadOnly(view ScanView) bool {
	return view.FinishedAt != nil && time.Since(*view.FinishedAt) > scanJobTTL
}

// Keep the same classified data for the Mini App when a bot/scheduled scan
// already did the work. If admins cannot be fetched the snapshot is view-only:
// it must never enable actions without knowing which members are protected.
func (d *Domain) cacheReport(ctx context.Context, ch ChannelInfo, report processors.AccessReport) {
	admins, err := d.telegramBot.GetChatAdministrators(ctx, ch.id)
	protected := map[int64]string{d.telegramBot.UserID(): "this bot", d.userBot.UserID(): "this bot"}
	for _, id := range admins {
		if _, ok := protected[id]; !ok {
			protected[id] = "administrator"
		}
	}
	now := time.Now()
	job := &scanJob{view: ScanView{ID: newScanID(), ChannelID: ch.id, Title: ch.title, ChatType: ch.chatType, State: ScanStateDone, StartedAt: now, FinishedAt: &now, Stats: report.Stats, Partial: report.Stats.Partial(), ReadOnly: err != nil}, users: map[int64]guard.User{}}
	for _, group := range []struct {
		users  []guard.User
		status string
	}{{report.AllowedUsers, UserStatusGood}, {report.DeniedUsers, UserStatusBad}, {report.UnknownUsers, UserStatusUnknown}} {
		for _, user := range group.users {
			note, isProtected := protected[user.ID]
			su := ScannedUser{ID: user.ID, FirstName: user.FirstName, LastName: user.LastName, Username: user.Username, Status: group.status, Protected: isProtected, Note: note}
			if entry, ok := d.whitelistEntry(ctx, ch.id, user.ID); ok {
				markWhitelisted(&su, entry, d.now())
			}
			job.users[user.ID] = user
			job.view.Users = append(job.view.Users, su)
		}
	}
	job.view.Checked = len(job.users)
	d.scansMutex.Lock()
	defer d.scansMutex.Unlock()
	d.expireScansLocked()
	d.scans[job.view.ID] = job
}

func (d *Domain) invalidateChannelScans(channelID int64) {
	d.scansMutex.Lock()
	defer d.scansMutex.Unlock()
	for _, job := range d.scans {
		if job.view.ChannelID == channelID && job.view.State == ScanStateDone {
			job.view.ReadOnly = true
		}
	}
}

// Reconcile every cached scan after a whitelist change, including scans from
// other sessions. Update health immediately so an approved admin no longer
// leaves the channel marked as violating until the next scheduled scan.
func (d *Domain) syncWhitelistScans(ctx context.Context, channelID int64) {
	if _, err := d.channelWhitelist(ctx, channelID); err != nil {
		return
	}
	d.whitelistMutex.Lock()
	entries := make(map[int64]whitelist.Entry, len(d.whitelists[channelID]))
	for id, entry := range d.whitelists[channelID] {
		entries[id] = entry
	}
	d.whitelistMutex.Unlock()
	d.scansMutex.Lock()
	var latest *scanJob
	for _, job := range d.scans {
		if job.view.ChannelID != channelID || job.view.State != ScanStateDone {
			continue
		}
		for i := range job.view.Users {
			u := &job.view.Users[i]
			if u.Status == UserStatusKicked {
				continue
			}
			if entry, ok := entries[u.ID]; ok && !entry.Deleted(d.now()) {
				markWhitelisted(u, entry, d.now())
			} else if u.Status == UserStatusWhitelisted {
				u.Whitelist = nil
				u.Status = UserStatusUnknown
				if u.Check != "" {
					u.Status = u.Check
				}
				if u.Note == noteWhitelisted {
					u.Note = ""
					u.Protected = false
				}
			}
		}
		if latest == nil || job.view.StartedAt.After(latest.view.StartedAt) {
			latest = job
		}
	}
	var summary processors.CheckSummary
	if latest != nil {
		summary = summarizeScan(latest.view, d.now())
	}
	d.scansMutex.Unlock()
	if latest != nil {
		d.recordCheck(ctx, channelID, summary)
	}
}
