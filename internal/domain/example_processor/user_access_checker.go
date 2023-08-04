package example_processor

import (
	"context"
	"time"
)

func (d *Domain) RunUserAccessChecker(ctx context.Context) {

	ticker := time.NewTicker(d.accessCheckInterval)
	for channelId, checker := range d.accessCheckers {
		go func(channelId int64, checker CheckUserAccess) {
			for {
				select {
				case <-ticker.C:
					{
					
						if d.userBot != nil {
							d.log.Debugf("run check for channel %d", channelId)
							users, err := d.userBot.GetChannelUsers(
								ctx,
								channelId,
							)
							
							if err != nil {
								d.log.Errorf("can't get users: %s", err)
								continue
							}
							
							for _, user := range users {
								hasAccess := checker.HasAccess(user)
								d.log.Infof(
									"User ID %d, username %s, phone %s, firstname %s, lastname %s, access %v, channel %d",
									user.ID, user.Username, user.Phone, user.FirstName, user.LastName, hasAccess, channelId,
								)
							}
						}
					}
				case <-ctx.Done():
					ticker.Stop()
					return
				}
			}
		}(channelId, checker)
	}

}
