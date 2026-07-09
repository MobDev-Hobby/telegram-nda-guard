package scanner

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

// UsersHandler lists the members of a channel, split into Good/Unknown/Bad by
// the channel's access checker. It is the read-only counterpart to /scan: same
// data gathering, but rendered on demand and without invoking the report or
// cleaner processors.
//
// Triggered by the "/users <id>" message command and the /users <id> inline
// button (callback). For large channels the listing is truncated with a hint to
// run /scan for a full persisted report, since Telegram limits member listings.
func (d *Domain) UsersHandler(
	ctx context.Context,
	update *guard.Update,
) {
	commandChatID, channelID, ok := d.resolveUsersTarget(update)
	if !ok {
		return
	}

	rendering, err := d.renderChannelUsers(ctx, channelID)
	if err != nil {
		_ = d.telegramBot.SendMessage(
			ctx, &guard.Message{
				ChatID:   commandChatID,
				ThreadID: threadIDOf(update),
				Text:     fmt.Sprintf("Can't list users: %s", err.Error()),
			},
		)
		return
	}

	_ = d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID:   commandChatID,
			ThreadID: threadIDOf(update),
			Text:     rendering,
		},
	)
}

// resolveUsersTarget extracts the originating chat id and the requested channel
// id from either a message ("/users <id>") or a callback ("/users <id>").
func (d *Domain) resolveUsersTarget(update *guard.Update) (commandChatID, channelID int64, ok bool) {
	switch {
	case update.Message != nil:
		commandChatID = update.Message.ChatID
		channelID, ok = parseUsersChannelArg(update.Message.Text)
		if !ok {
			_ = d.telegramBot.SendMessage(
				context.Background(), &guard.Message{
					ChatID:   commandChatID,
					ThreadID: update.Message.ThreadID,
					Text:     "Usage: /users <channel_id> (pick a channel from /list)",
				},
			)
			return 0, 0, false
		}
	case update.CallbackQuery != nil && update.CallbackQuery.Message != nil:
		commandChatID = update.CallbackQuery.Message.ChatID
		channelID, ok = parseUsersChannelArg(update.CallbackQuery.Data)
		if !ok {
			d.telegramBot.CallbackResponse(
				context.Background(),
				guard.CallbackResponse{ID: update.CallbackQuery.ID, Text: "Bad channel id", ShowAlert: true},
			)
			return 0, 0, false
		}
		d.telegramBot.CallbackResponse(
			context.Background(),
			guard.CallbackResponse{ID: update.CallbackQuery.ID},
		)
	default:
		return 0, 0, false
	}
	return commandChatID, channelID, true
}

// parseUsersChannelArg extracts the integer channel id from a payload of the
// form "/users <id>". Returns false on parse failure.
func parseUsersChannelArg(data string) (int64, bool) {
	parts := strings.Split(data, " ")
	if len(parts) < 2 {
		return 0, false
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// renderChannelUsers fetches the members of channelID, classifies them with the
// channel's access checker, and returns an HTML rendering split into Good /
// Unknown / Bad. Large listings are truncated to keep within Telegram's message
// length limit.
func (d *Domain) renderChannelUsers(ctx context.Context, channelID int64) (string, error) {
	protectedChannel, found := d.protectedChannels[channelID]
	channel, channelFound := d.channels[channelID]

	if !found {
		return "", fmt.Errorf("channel %d is not protected", channelID)
	}

	title := fmt.Sprintf("%d", channelID)
	if channelFound {
		title = channel.title
	}

	if d.userBot == nil {
		return "", fmt.Errorf("userbot is not running")
	}

	users, err := d.userBot.GetChannelUsers(ctx, channelID)
	if err != nil {
		return "", fmt.Errorf("get channel users: %w", err)
	}

	checker := protectedChannel.AccessChecker
	if checker == nil {
		checker = d.defaultAccessChecker
	}
	if checker == nil {
		return "", fmt.Errorf("no access checker configured")
	}

	var good, unknown, bad []guard.User
	for _, user := range users {
		user := user
		hasAccess, err := checker.HasAccess(ctx, &user)
		if err != nil {
			unknown = append(unknown, user)
			continue
		}
		if hasAccess {
			good = append(good, user)
		} else {
			bad = append(bad, user)
		}
	}

	return formatUsersReport(title, good, unknown, bad), nil
}

// formatUsersReport builds the HTML message. It truncates each section to a
// per-section cap so the whole message stays comfortably under Telegram's 4096
// character limit even for large channels.
func formatUsersReport(title string, good, unknown, bad []guard.User) string {
	const perSectionCap = 40

	var b strings.Builder
	fmt.Fprintf(&b, "<b>Users of %s</b>\n\n", title)
	fmt.Fprintf(&b, "<b>Summary</b>: Good <b>%d</b> · Unknown <b>%d</b> · Bad <b>%d</b>\n",
		len(good), len(unknown), len(bad))

	appendSection(&b, "Good", good, perSectionCap)
	appendSection(&b, "Unknown", unknown, perSectionCap)
	appendSection(&b, "Bad", bad, perSectionCap)

	if len(good)+len(unknown)+len(bad) > perSectionCap*3 {
		b.WriteString("\n<i>List truncated. Run /scan for a full persisted report.</i>")
	}
	return b.String()
}

func appendSection(b *strings.Builder, label string, users []guard.User, cap int) {
	if len(users) == 0 {
		return
	}
	fmt.Fprintf(b, "\n<b>%s (%d):</b>\n", label, len(users))
	shown := users
	truncated := false
	if len(shown) > cap {
		shown = shown[:cap]
		truncated = true
	}
	for _, u := range shown {
		b.WriteString("• ")
		b.WriteString(userLink(u))
		b.WriteString("\n")
	}
	if truncated {
		fmt.Fprintf(b, "… and %d more\n", len(users)-cap)
	}
}

// userLink renders an HTML link to a user, preferring username, then phone, then
// an inline tg://user link. Mirrors the reporter's getUserLink logic but kept
// local to avoid a cross-package dependency.
func userLink(u guard.User) string {
	lastname := ""
	if len(u.LastName) > 0 {
		lastname = " " + u.LastName
	}
	switch {
	case len(u.Username) > 0:
		return fmt.Sprintf(`<a href="https://t.me/%s">%s%s</a>`, u.Username, u.FirstName, lastname)
	case u.Phone != nil:
		return fmt.Sprintf(`<a href="https://t.me/+%s">%s%s</a>`, *u.Phone, u.FirstName, lastname)
	default:
		return fmt.Sprintf(`<a href="tg://user?id=%d">%s%s</a>`, u.ID, u.FirstName, lastname)
	}
}

// threadIDOf is a small helper to pull the thread id from whichever message
// shape an update carries.
func threadIDOf(update *guard.Update) *int {
	if update.Message != nil {
		return update.Message.ThreadID
	}
	if update.CallbackQuery != nil && update.CallbackQuery.Message != nil {
		return update.CallbackQuery.Message.ThreadID
	}
	return nil
}
