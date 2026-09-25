package scanner

import (
	"context"
	"fmt"
	"html"
	"sort"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/audit"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/knownchats"
)

// KnownChatStorage remembers chats where the bot is an administrator.
// storage/knownchats/redis.Domain implements it.
type KnownChatStorage interface {
	LoadKnownChats(ctx context.Context) ([]knownchats.Chat, error)
	StoreKnownChat(ctx context.Context, chat knownchats.Chat) error
	DropKnownChat(ctx context.Context, chatID int64) error
}

// startPayloadAdd is the /start payload the Mini App uses to open the /add
// chat picker in the bot's private chat.
const startPayloadAdd = "add"

// loadKnownChats fills the in-memory copy at start.
func (d *Domain) loadKnownChats(ctx context.Context) {
	if d.knownChatStorage == nil {
		return
	}
	chats, err := d.knownChatStorage.LoadKnownChats(ctx)
	if err != nil {
		d.log.Errorf("can't load known chats: %s", err)
		return
	}
	d.knownMutex.Lock()
	for _, c := range chats {
		d.knownChats[c.ID] = c
	}
	d.knownMutex.Unlock()
}

// BotMembershipHandler tracks where the bot is an administrator, from
// my_chat_member updates.
func (d *Domain) BotMembershipHandler(ctx context.Context, update *guard.Update) {
	m := update.MyChatMember
	if m == nil || d.knownChatStorage == nil || m.Chat.Type == guard.ChatTypePrivate {
		return
	}
	storeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if m.Status != guard.MemberStatusAdministrator {
		d.knownMutex.Lock()
		delete(d.knownChats, m.Chat.ID)
		d.knownMutex.Unlock()
		if err := d.knownChatStorage.DropKnownChat(storeCtx, m.Chat.ID); err != nil {
			d.log.Errorf("can't forget chat %d: %s", m.Chat.ID, err)
		}
		return
	}
	chat := knownchats.Chat{
		ID:                 m.Chat.ID,
		Title:              m.Chat.Title,
		Type:               m.Chat.Type,
		AddedBy:            m.From.ID,
		AddedAt:            d.now(),
		CanRestrictMembers: m.CanRestrictMembers,
		CanInviteUsers:     m.CanInviteUsers,
	}
	d.knownMutex.Lock()
	d.knownChats[chat.ID] = chat
	d.knownMutex.Unlock()
	if err := d.knownChatStorage.StoreKnownChat(storeCtx, chat); err != nil {
		d.log.Errorf("can't remember chat %d: %s", chat.ID, err)
	}
	d.log.Infof("bot is now an administrator of %s %d (%s)", chat.Type, chat.ID, chat.Title)
}

// ListAvailableChats implements MiniAppService: chats where the bot is an
// administrator but which are not protected yet. The transport filters them
// down to the ones the caller administers.
func (d *Domain) ListAvailableChats(_ context.Context) ([]knownchats.Chat, error) {
	d.knownMutex.Lock()
	out := make([]knownchats.Chat, 0, len(d.knownChats))
	for _, c := range d.knownChats {
		if _, protected := d.getProtectedChannel(c.ID); !protected {
			out = append(out, c)
		}
	}
	d.knownMutex.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out, nil
}

// ConnectChat implements MiniAppService: protect a chat where the bot is
// already an administrator. Reports go to the caller's private chat with the
// bot, and the caller becomes the chat's first manager.
func (d *Domain) ConnectChat(ctx context.Context, chatID, callerID int64) error {
	d.knownMutex.Lock()
	_, known := d.knownChats[chatID]
	d.knownMutex.Unlock()
	if !known {
		return fmt.Errorf("the bot is not an administrator of chat %d", chatID)
	}
	if _, protected := d.getProtectedChannel(chatID); protected {
		return fmt.Errorf("chat %d is already protected", chatID)
	}
	err := d.AddDefaultProtectedChannel(&ProtectedChannel{
		ID:                chatID,
		CommandChannelIDs: []int64{callerID},
		AutoScan:          true,
		AllowClean:        true,
		Managers:          []int64{callerID},
	})
	if err != nil {
		return fmt.Errorf("protect chat %d: %w", chatID, err)
	}
	if err := d.CheckRights(ctx); err != nil {
		d.log.Errorf("ConnectChat: can't check rights: %s", err)
	}
	pc, _ := d.getProtectedChannel(chatID)
	d.recordAudit(ctx, pc, audit.Event{
		ActorID: callerID, Action: audit.ActionChannelAdded,
		Details: map[string]any{"source": "miniapp"},
	}, fmt.Sprintf("%s is now protected. Connected by %s via the Mini App; reports come to this chat.",
		html.EscapeString(d.channelTitle(chatID)), actorLink(callerID, d.userName(callerID))))
	return nil
}
