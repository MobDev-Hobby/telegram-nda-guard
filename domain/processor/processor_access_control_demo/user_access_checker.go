package processor_access_control_demo

import (
	"context"
	"time"

	"github.com/MobDev-Hobby/telegram-nda-guard/domain/report_processor"
)

func (d *Domain) RunUserAccessChecker(ctx context.Context) {

	ticker := time.NewTicker(d.accessCheckInterval)

	for channelId, checker := range d.accessCheckers {
		go func(channelId int64, checker CheckUserAccess) {
			for {
				select {
				case <-ticker.C:
					if d.userBot == nil {
						continue
					}

					d.log.Debugf("run check for channel %d", channelId)
					users, err := d.userBot.GetChannelUsers(
						ctx,
						channelId,
					)

					report := report_processor.AccessReport{
						ChannelId:    channelId,
						AllowedUsers: []report_processor.User{},
						DeniedUsers:  []report_processor.User{},
						UnknownUsers: []report_processor.User{},
					}

					if err != nil {
						d.log.Errorf("can't get users: %s", err)
						continue
					}

					for _, user := range users {
						hasAccess, err := checker.HasAccess(ctx, user)
						d.log.Infof(
							"User ID %d, username %s, phone %s, firstname %s, lastname %s, access %v, channel %d",
							user.ID,
							user.Username,
							user.Phone,
							user.FirstName,
							user.LastName,
							hasAccess,
							channelId,
						)

						userReport := report_processor.User{
							ID:        user.ID,
							Firstname: user.FirstName,
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
					d.reportProcessor.ProcessReport(ctx, report)
					d.log.Debugf("done check for channel %d", channelId)
				case <-ctx.Done():
					ticker.Stop()
					return
				}
			}
		}(channelId, checker)
	}

}
