package scanner

import (
	"context"
	"errors"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/audit"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/joinrequests"
)

// Join request modes of a protected chat that approves new members.
const (
	// JoinModeOff leaves join requests to the chat's administrators.
	JoinModeOff = "off"
	// JoinModeAuto approves everyone who passes the access check and leaves
	// the rest pending, rechecking them daily.
	JoinModeAuto = "auto"
	// JoinModeManual keeps requests for managers to approve or decline in the
	// Mini App, each marked with its check result.
	JoinModeManual = "manual"
)

const (
	joinRecheckEvery     = 24 * time.Hour
	joinRecheckInterval  = time.Hour
	maxJoinResolveBatch  = 200
	errRequesterGone     = "HIDE_REQUESTER_MISSING"
	errAlreadyMember     = "USER_ALREADY_PARTICIPANT"
	errRequestNotPending = "no pending join request"
)

var (
	ErrJoinRequestsDisabled = errors.New("join requests are not configured")
	ErrJoinRequestNotFound  = errors.New(errRequestNotPending)
	ErrBadJoinMode          = errors.New("join request mode must be off, auto or manual")
	// ErrJoinRequestGone: Telegram no longer has the request (withdrawn, or
	// the user joined some other way). It is already dropped from storage.
	ErrJoinRequestGone = errors.New("the request is no longer pending")
)

// JoinRequestStorage keeps pending join requests.
// storage/joinrequests/redis.Domain implements it.
type JoinRequestStorage interface {
	LoadJoinRequests(ctx context.Context, chatID int64) ([]joinrequests.Request, error)
	StoreJoinRequest(ctx context.Context, chatID int64, request joinrequests.Request) error
	DropJoinRequest(ctx context.Context, chatID, userID int64) error
}

// joinResolver is implemented by telegram/bots/bot.Domain.
type joinResolver interface {
	ApproveJoinRequest(ctx context.Context, chatID, userID int64) error
	DeclineJoinRequest(ctx context.Context, chatID, userID int64) error
}

// JoinRequestView is a pending request as shown in the Mini App.
type JoinRequestView struct {
	joinrequests.Request
	Whitelisted bool `json:"whitelisted"`
}

// JoinResult is the outcome of approving or declining one request.
type JoinResult struct {
	UserID int64  `json:"userId"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
}

func validJoinMode(mode string) bool {
	return mode == JoinModeOff || mode == JoinModeAuto || mode == JoinModeManual || mode == ""
}

func (d *Domain) channelJoinRequests(ctx context.Context, chatID int64) (map[int64]joinrequests.Request, error) {
	d.joinMutex.Lock()
	defer d.joinMutex.Unlock()
	if reqs, ok := d.joinRequests[chatID]; ok {
		return reqs, nil
	}
	loaded, err := d.joinRequestStorage.LoadJoinRequests(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("load join requests of %d: %w", chatID, err)
	}
	reqs := make(map[int64]joinrequests.Request, len(loaded))
	for _, r := range loaded {
		reqs[r.UserID] = r
	}
	d.joinRequests[chatID] = reqs
	return reqs, nil
}

func (d *Domain) putJoinRequest(ctx context.Context, chatID int64, r joinrequests.Request) error {
	if _, err := d.channelJoinRequests(ctx, chatID); err != nil {
		return err
	}
	storeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := d.joinRequestStorage.StoreJoinRequest(storeCtx, chatID, r); err != nil {
		return fmt.Errorf("persist join request: %w", err)
	}
	d.joinMutex.Lock()
	d.joinRequests[chatID][r.UserID] = r
	d.joinMutex.Unlock()
	return nil
}

func (d *Domain) dropJoinRequest(ctx context.Context, chatID, userID int64) {
	storeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := d.joinRequestStorage.DropJoinRequest(storeCtx, chatID, userID); err != nil {
		d.log.Errorf("can't drop join request %d of %d: %s", userID, chatID, err)
	}
	d.joinMutex.Lock()
	delete(d.joinRequests[chatID], userID)
	d.joinMutex.Unlock()
}

// checkJoinUser runs the channel's checker (whitelist included) for a
// requester. fresh drops the checker's cached verdict first.
func (d *Domain) checkJoinUser(ctx context.Context, pc ProtectedChannel, r joinrequests.Request, fresh bool) string {
	checker := pc.AccessChecker
	if checker == nil {
		checker = d.defaultAccessChecker
	}
	if checker == nil {
		return UserStatusUnknown
	}
	if fresh {
		if invalidator, ok := checker.(interface{ Invalidate(userID int64) }); ok {
			invalidator.Invalidate(r.UserID)
		}
	}
	user := guard.User{ID: r.UserID, FirstName: r.FirstName, LastName: r.LastName, Username: r.Username}
	return checkStatus(d.withWhitelist(pc.ID, checker).HasAccess(ctx, &user))
}

// JoinRequestHandler handles chat_join_request updates of protected chats.
func (d *Domain) JoinRequestHandler(ctx context.Context, update *guard.Update) {
	jr := update.JoinRequest
	if jr == nil || d.joinRequestStorage == nil {
		return
	}
	pc, ok := d.getProtectedChannel(jr.Chat.ID)
	if !ok || pc.JoinRequestMode == "" || pc.JoinRequestMode == JoinModeOff {
		return
	}
	d.NoteUserName(jr.User.ID, strings.TrimSpace(jr.User.FirstName+" "+jr.User.LastName))
	now := d.now()
	r := joinrequests.Request{
		UserID:      jr.User.ID,
		FirstName:   jr.User.FirstName,
		LastName:    jr.User.LastName,
		Username:    jr.User.Username,
		Bio:         jr.Bio,
		RequestedAt: time.Unix(jr.At, 0).UTC(),
		CheckedAt:   now,
	}
	r.Check = d.checkJoinUser(ctx, pc, r, false)

	if pc.JoinRequestMode == JoinModeAuto && r.Check == UserStatusGood {
		if err := d.resolveJoin(ctx, pc, r, true, 0); err == nil || errors.Is(err, ErrJoinRequestGone) {
			return
		}
		// Approval failed (rights, network): keep it pending for later.
	}
	if err := d.putJoinRequest(ctx, pc.ID, r); err != nil {
		d.log.Errorf("%s", err)
		return
	}
	u := joinAuditUser(r)
	d.recordAudit(ctx, pc, audit.Event{
		Action: audit.ActionJoinRequested, Users: []audit.User{u},
		Details: map[string]any{"check": r.Check, "mode": pc.JoinRequestMode},
	}, "")
	if pc.JoinRequestMode == JoinModeManual {
		text := fmt.Sprintf("<b>Join request to %s</b>\n\n%s — access check: <b>%s</b>.\nApprove or decline it in the app: /app → Requests.",
			html.EscapeString(d.channelTitle(pc.ID)), html.EscapeString(joinName(r)), r.Check)
		d.notifyControlChats(ctx, pc, text)
		d.notifyManagers(ctx, pc, text)
	}
}

// resolveJoin approves or declines r in Telegram, drops it from the pending
// list and logs it. actorID 0 means the bot decided (auto mode).
func (d *Domain) resolveJoin(ctx context.Context, pc ProtectedChannel, r joinrequests.Request, approve bool, actorID int64) error {
	resolver, ok := d.telegramBot.(joinResolver)
	if !ok {
		return errors.New("the bot transport can't resolve join requests")
	}
	var err error
	if approve {
		err = resolver.ApproveJoinRequest(ctx, pc.ID, r.UserID)
	} else {
		err = resolver.DeclineJoinRequest(ctx, pc.ID, r.UserID)
	}
	gone := err != nil && (strings.Contains(err.Error(), errRequesterGone) || strings.Contains(err.Error(), errAlreadyMember))
	if err != nil && !gone {
		d.log.Errorf("can't resolve join request %d of %d: %s", r.UserID, pc.ID, err)
		return err
	}
	d.dropJoinRequest(ctx, pc.ID, r.UserID)
	if gone {
		// The user withdrew the request or joined some other way.
		return fmt.Errorf("%w: %w", ErrJoinRequestGone, err)
	}

	action := audit.ActionJoinDeclined
	verb := "declined"
	if approve {
		action, verb = audit.ActionJoinApproved, "approved"
	}
	details := map[string]any{"check": r.Check, "auto": actorID == 0}
	d.recordAudit(ctx, pc, audit.Event{
		ActorID: actorID, Action: action, Users: []audit.User{joinAuditUser(r)}, Details: details,
	}, "")
	if actorID == 0 {
		d.log.Infof("auto-%s join request of %d to %d", verb, r.UserID, pc.ID)
	}
	return nil
}

// ListJoinRequests implements MiniAppService.
func (d *Domain) ListJoinRequests(ctx context.Context, channelID int64) ([]JoinRequestView, error) {
	if d.joinRequestStorage == nil {
		return nil, ErrJoinRequestsDisabled
	}
	if _, ok := d.getProtectedChannel(channelID); !ok {
		return nil, fmt.Errorf("channel %d is not protected", channelID)
	}
	if _, err := d.channelJoinRequests(ctx, channelID); err != nil {
		return nil, err
	}
	d.joinMutex.Lock()
	out := make([]JoinRequestView, 0, len(d.joinRequests[channelID]))
	for _, r := range d.joinRequests[channelID] {
		out = append(out, JoinRequestView{Request: r})
	}
	d.joinMutex.Unlock()
	for i := range out {
		_, out[i].Whitelisted = d.whitelistEntry(ctx, channelID, out[i].UserID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RequestedAt.Before(out[j].RequestedAt) })
	return out, nil
}

// ResolveJoinRequests implements MiniAppService: a manager approves or
// declines pending requests.
func (d *Domain) ResolveJoinRequests(ctx context.Context, channelID int64, userIDs []int64, approve bool, callerID int64) ([]JoinResult, error) {
	if d.joinRequestStorage == nil {
		return nil, ErrJoinRequestsDisabled
	}
	if len(userIDs) > maxJoinResolveBatch {
		return nil, fmt.Errorf("at most %d requests at once", maxJoinResolveBatch)
	}
	pc, ok := d.getProtectedChannel(channelID)
	if !ok {
		return nil, fmt.Errorf("channel %d is not protected", channelID)
	}
	reqs, err := d.channelJoinRequests(ctx, channelID)
	if err != nil {
		return nil, err
	}
	results := make([]JoinResult, 0, len(userIDs))
	for _, id := range userIDs {
		d.joinMutex.Lock()
		r, found := reqs[id]
		d.joinMutex.Unlock()
		if !found {
			results = append(results, JoinResult{UserID: id, Error: errRequestNotPending})
			continue
		}
		if err := d.resolveJoin(ctx, pc, r, approve, callerID); err != nil {
			results = append(results, JoinResult{UserID: id, Error: err.Error()})
			continue
		}
		results = append(results, JoinResult{UserID: id, OK: true})
	}
	return results, nil
}

// RecheckJoinRequest implements MiniAppService: run the checker again for one
// requester. In auto mode a pass approves the request right away.
func (d *Domain) RecheckJoinRequest(ctx context.Context, channelID, userID, callerID int64) (JoinRequestView, error) {
	if d.joinRequestStorage == nil {
		return JoinRequestView{}, ErrJoinRequestsDisabled
	}
	pc, ok := d.getProtectedChannel(channelID)
	if !ok {
		return JoinRequestView{}, fmt.Errorf("channel %d is not protected", channelID)
	}
	reqs, err := d.channelJoinRequests(ctx, channelID)
	if err != nil {
		return JoinRequestView{}, err
	}
	d.joinMutex.Lock()
	r, found := reqs[userID]
	d.joinMutex.Unlock()
	if !found {
		return JoinRequestView{}, ErrJoinRequestNotFound
	}
	r, _ = d.recheckJoin(ctx, pc, r, callerID)
	view := JoinRequestView{Request: r}
	_, view.Whitelisted = d.whitelistEntry(ctx, channelID, userID)
	return view, nil
}

// recheckJoin refreshes r's verdict and, in auto mode, approves a pass. It
// reports whether the request was resolved.
func (d *Domain) recheckJoin(ctx context.Context, pc ProtectedChannel, r joinrequests.Request, actorID int64) (joinrequests.Request, bool) {
	r.Check = d.checkJoinUser(ctx, pc, r, true)
	r.CheckedAt = d.now()
	d.recordAudit(ctx, pc, audit.Event{
		ActorID: actorID, Action: audit.ActionJoinRechecked, Users: []audit.User{joinAuditUser(r)},
		Details: map[string]any{"check": r.Check},
	}, "")
	if pc.JoinRequestMode == JoinModeAuto && r.Check == UserStatusGood {
		if err := d.resolveJoin(ctx, pc, r, true, 0); err == nil || errors.Is(err, ErrJoinRequestGone) {
			return r, true
		}
	}
	if err := d.putJoinRequest(ctx, pc.ID, r); err != nil {
		d.log.Errorf("%s", err)
	}
	return r, false
}

// RunJoinRequestRechecks rechecks pending requests once a day; in auto mode
// the ones that pass now are approved.
func (d *Domain) RunJoinRequestRechecks(ctx context.Context) {
	if d.joinRequestStorage == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(joinRecheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.recheckJoinRequests(ctx)
			}
		}
	}()
}

func (d *Domain) recheckJoinRequests(ctx context.Context) {
	now := d.now()
	d.channelsMutex.RLock()
	protected := make([]ProtectedChannel, 0, len(d.protectedChannels))
	for _, pc := range d.protectedChannels {
		if pc.JoinRequestMode == JoinModeAuto || pc.JoinRequestMode == JoinModeManual {
			protected = append(protected, pc)
		}
	}
	d.channelsMutex.RUnlock()

	for _, pc := range protected {
		reqs, err := d.channelJoinRequests(ctx, pc.ID)
		if err != nil {
			d.log.Errorf("%s", err)
			continue
		}
		d.joinMutex.Lock()
		due := make([]joinrequests.Request, 0, len(reqs))
		for _, r := range reqs {
			if now.Sub(r.CheckedAt) >= joinRecheckEvery {
				due = append(due, r)
			}
		}
		d.joinMutex.Unlock()
		for _, r := range due {
			d.recheckJoin(ctx, pc, r, 0)
		}
	}
}

func joinName(r joinrequests.Request) string {
	name := strings.TrimSpace(r.FirstName + " " + r.LastName)
	if name == "" {
		name = fmt.Sprintf("%d", r.UserID)
	}
	if r.Username != "" {
		name += " (@" + r.Username + ")"
	}
	return name
}

func joinAuditUser(r joinrequests.Request) audit.User {
	return audit.User{ID: r.UserID, Name: strings.TrimSpace(r.FirstName + " " + r.LastName), Username: r.Username}
}
