package scanner

import (
	"context"
	"fmt"
	"reflect"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
	"github.com/MobDev-Hobby/telegram-nda-guard/utils"
)

func (d *Domain) CheckPermissions(_ context.Context, request ScanRequest) error {
	channel := request.channelInfo

	if request.requestType == None {
		return fmt.Errorf("no request type specified")
	}

	if d.userBot == nil {
		return fmt.Errorf(
			"skip check channel [%d]%s, because there is no active userbot",
			channel.id,
			channel.title,
		)
	}

	switch request.requestType {
	case None:
		return fmt.Errorf("no request type specified")
	case Scan, AutoScan:
		if !request.channelInfo.CanScan() {
			return fmt.Errorf(
				"skip check channel [%d]%s, because the bot have no rights to scan",
				channel.id,
				channel.title,
			)
		}
	case Clean, AutoClean:
		if !request.channelInfo.CanClean() {
			return fmt.Errorf(
				"skip check channel [%d]%s, because the bot have no rights to clean users",
				channel.id,
				channel.title,
			)
		}
	default:
		return fmt.Errorf("unknown request type %d", request.requestType)
	}

	return nil
}

func (d *Domain) CheckChannelsLoop(ctx context.Context) {
	for range make([]any, d.processingThreads) {
		for {
			select {
			case <-ctx.Done():
				return
			case request, ok := <-d.processRequestChan:
				if !ok {
					return
				}

				// Bot is not ready
				if err := d.CheckPermissions(ctx, request); err != nil {
					for _, commandChannelID := range request.channelInfo.commandChannelIDs {
						_ = d.telegramBot.SendMessage(
							ctx,
							&guard.Message{
								ChatID: commandChannelID,
								Text:   fmt.Sprintf("Skip channel scan due error: %s", err.Error()),
							},
						)
					}
					continue
				}
				d.ProcessRequest(ctx, request)
				<-time.After(d.taskDelayInterval)
			}
		}
	}
}

func (d *Domain) ProcessRequest(ctx context.Context, request ScanRequest) {
	d.log.Debugf("run check for channel [%d]%s", request.channelInfo.id, request.channelInfo.title)
	users, err := d.userBot.GetChannelUsers(
		ctx,
		request.channelInfo.id,
	)

	if err != nil {
		d.log.Errorf("can't get users: %s", err)
		return
	}

	report := processors.AccessReport{
		Channel: guard.ChannelInfo{
			ID:    request.channelInfo.id,
			Title: request.channelInfo.title,
			Type:  request.channelInfo.chatType,
		},
		AllowedUsers: []guard.User{},
		DeniedUsers:  []guard.User{},
		UnknownUsers: []guard.User{},
	}

	if ch, chOk := d.getChannel(request.channelInfo.id); chOk && ch.migratedFrom != nil {
		// Propagate migration target so the kicker bans the correct (new)
		// chat. Previously MigratedTo was never set, so bans targeted a
		// stale id.
		report.Channel.MigratedTo = utils.Ptr(request.channelInfo.id)
	}

	if request.reportChannels != nil {
		report.ReportChannels = *request.reportChannels
	} else {
		protectedChannel, ok := d.getProtectedChannel(request.channelInfo.id)
		if !ok {
			d.log.Errorf("protected channel for channel %d not found", request.channelInfo.id)
			return
		}
		report.ReportChannels = protectedChannel.CommandChannelIDs
	}

	checker := request.accessChecker
	if checker == nil {
		d.log.Infof("No access checker specified, skip")
		return
	}
	for _, user := range users {
		user := user
		hasAccess, err := checker.HasAccess(ctx, &user)
		phone := ""
		if user.Phone != nil {
			phone = *user.Phone
		}
		d.log.Infof(
			"User ID %d, username %s, phone %s, firstname %s, lastname %s, access %t, channel %d",
			user.ID,
			user.Username,
			phone,
			user.FirstName,
			user.LastName,
			hasAccess,
			request.channelInfo.id,
		)

		userReport := guard.User{
			ID:        user.ID,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Username:  user.Username,
		}
		if err != nil {
			report.UnknownUsers = append(report.UnknownUsers, userReport)
			continue
		}
		if hasAccess {
			report.AllowedUsers = append(report.AllowedUsers, userReport)
			continue
		}
		report.DeniedUsers = append(report.DeniedUsers, userReport)
	}
	request.reportProcessor.ProcessReport(ctx, report)
	d.log.Debugf("done check for channel [%d]%s", request.channelInfo.id, request.channelInfo.title)
}

func (d *Domain) RunUserAccessChecker(ctx context.Context) {
	go d.CheckChannelsLoop(ctx)
	go func(ctx context.Context) {
		contextDone := reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ctx.Done()),
		}
		for {
			// Snapshot the ticker slices under the lock so reflect.Select
			// does not race registerTicker/removeTickers/MigrateChannel, and
			// build a fresh slice to avoid aliasing d.tickerCases' backing
			// array (append below would otherwise write into shared capacity).
			d.tickerCasesMutex.Lock()
			cases := make([]reflect.SelectCase, 0, len(d.tickerCases)+1)
			cases = append(cases, d.tickerCases...)
			tickerChannels := make([]int64, len(d.tickerCasesChannels))
			copy(tickerChannels, d.tickerCasesChannels)
			d.tickerCasesMutex.Unlock()
			cases = append(cases, contextDone)

			selected, _, ok := reflect.Select(cases)
			// ctx.Done is the last case
			if selected == len(cases)-1 {
				d.log.Infof("context done, terminating user access checker")
				return
			}
			if !ok {
				d.log.Infof("channel closed, skip")
				return
			}
			// bounds check: selected indexes into cases (= tickerChannels),
			// so valid range is [0, len(tickerChannels)-1]; use >= (was >).
			if selected >= len(tickerChannels) {
				d.log.Errorf("wrong ticker %d, skip", selected)
				continue
			}
			channelID := tickerChannels[selected]
			protectedChannel, ok := d.getProtectedChannel(channelID)
			if !ok {
				d.log.Infof("channel for ticker %d error, skip", selected)
				continue
			}
			channel, ok := d.getChannel(protectedChannel.ID)
			if !ok {
				// was missing continue: fall-through enqueued a zero-value
				// ChannelInfo, producing misleading "no rights" errors
				d.log.Infof("channel ID %d not ready yet, skip", protectedChannel.ID)
				continue
			}
			if protectedChannel.AutoScan {
				d.enqueueScanRequest(ctx, ScanRequest{
					requestType:     AutoScan,
					channelInfo:     channel,
					accessChecker:   protectedChannel.AccessChecker,
					reportProcessor: protectedChannel.ScanReportProcessor,
				})
			}

			if protectedChannel.AutoClean {
				d.enqueueScanRequest(ctx, ScanRequest{
					requestType:     AutoClean,
					channelInfo:     channel,
					accessChecker:   protectedChannel.AccessChecker,
					reportProcessor: protectedChannel.CleanReportProcessor,
				})
			}
		}
	}(ctx)
}

// enqueueScanRequest sends a scan request without blocking forever when the
// worker queue is full. A full queue would otherwise wedge the ticker
// goroutine (leaking it on shutdown, since it could no longer service
// ctx.Done) and the bot update pipeline.
func (d *Domain) enqueueScanRequest(ctx context.Context, request ScanRequest) {
	select {
	case d.processRequestChan <- request:
	case <-ctx.Done():
		d.log.Warnf("scan request dropped, context done: %s", ctx.Err())
	default:
		d.log.Warnf("scan queue full, dropping %v request for channel %d", request.requestType, request.channelInfo.id)
	}
}
