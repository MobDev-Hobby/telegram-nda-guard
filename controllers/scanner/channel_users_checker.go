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

	report := processors.AccessReport{
		Channel: guard.ChannelInfo{
			ID:    request.channelInfo.id,
			Title: request.channelInfo.title,
		},
		AllowedUsers: []guard.User{},
		DeniedUsers:  []guard.User{},
		UnknownUsers: []guard.User{},
	}

	if d.channels[request.channelInfo.id].migratedFrom != nil {
		report.Channel.ID = *d.channels[request.channelInfo.id].migratedFrom
		report.Channel.MigratedTo = utils.Ptr(d.channels[request.channelInfo.id].id)
	}

	if err != nil {
		d.log.Errorf("can't get users: %s", err)
		return
	}

	checker := request.accessChecker
	for _, user := range users {
		user := user
		hasAccess, err := checker.HasAccess(ctx, &user)
		d.log.Infof(
			"User ID %d, username %s, phone %s, firstname %s, lastname %s, access %v, channel %d",
			user.ID,
			user.Username,
			user.Phone,
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
			cases := append(d.tickerCases, contextDone)
			selected, _, ok := reflect.Select(cases)
			// ctx.Done
			if selected == len(cases)-1 {
				d.log.Infof("context done, terminating user access checker")
				return
			}
			if !ok {
				d.log.Infof("channel closed, skip")
				return
			}
			if selected > len(d.tickerCasesChannels) {
				d.log.Errorf("wrong ticker %d, skip", selected)
				continue
			}
			channelID := d.tickerCasesChannels[selected]
			protectedChannel, ok := d.protectedChannels[channelID]
			if !ok {
				d.log.Infof("channel for ticker %d error, skip", selected)
				continue
			}
			channel, ok := d.channels[protectedChannel.ID]
			if !ok {
				d.log.Infof("channel ID %d not ready yet, skip", protectedChannel.ID)
			}
			if protectedChannel.AutoScan {
				d.processRequestChan <- ScanRequest{
					requestType:     AutoScan,
					channelInfo:     channel,
					accessChecker:   protectedChannel.AccessChecker,
					reportProcessor: protectedChannel.ScanReportProcessor,
				}
			}

			if protectedChannel.AutoClean {
				d.processRequestChan <- ScanRequest{
					requestType:     AutoClean,
					channelInfo:     channel,
					accessChecker:   protectedChannel.AccessChecker,
					reportProcessor: protectedChannel.CleanReportProcessor,
				}
			}
		}
	}(ctx)
}
