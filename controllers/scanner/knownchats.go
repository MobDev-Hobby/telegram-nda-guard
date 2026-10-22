package scanner

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strings"
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
	for _, c := range chats {
		if _, ok := d.getProtectedChannel(c.ID); !ok {
			continue
		}
		_ = d.updateProtectedChannel(ctx, c.ID, func(pc *ProtectedChannel) {
			if pc.Title == "" {
				pc.Title = c.Title
			}
			if pc.ChatType == "" {
				pc.ChatType = c.Type
			}
			if pc.AddedBy == 0 {
				pc.AddedBy, pc.AddedByName, pc.AddedAt = c.AddedBy, c.AddedByName, c.AddedAt
			}
		})
		d.channelsMutex.Lock()
		ch := d.channels[c.ID]
		if ch.title == "" {
			ch.title = c.Title
		}
		if ch.chatType == "" {
			ch.chatType = c.Type
		}
		d.channels[c.ID] = ch
		d.channelsMutex.Unlock()
	}
}

// BotMembershipHandler tracks chats containing the bot, including ones still
// waiting for administrator rights, from
// my_chat_member updates.
func (d *Domain) BotMembershipHandler(ctx context.Context, update *guard.Update) {
	m := update.MyChatMember
	if m == nil || m.Chat.Type == guard.ChatTypePrivate {
		return
	}
	storeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if m.Status == guard.MemberStatusLeft || m.Status == guard.MemberStatusKicked {
		d.knownMutex.Lock()
		delete(d.knownChats, m.Chat.ID)
		d.knownMutex.Unlock()
		d.channelsMutex.Lock()
		if ch, ok := d.channels[m.Chat.ID]; ok {
			ch.botOnChannel, ch.botCanClean, ch.botCanInvite = false, false, false
			d.channels[m.Chat.ID] = ch
		}
		d.channelsMutex.Unlock()
		if d.knownChatStorage != nil {
			if err := d.knownChatStorage.DropKnownChat(storeCtx, m.Chat.ID); err != nil {
				d.log.Errorf("can't forget chat %d: %s", m.Chat.ID, err)
			}
		}
		return
	}
	chat := knownchats.Chat{
		ID:                 m.Chat.ID,
		Title:              m.Chat.Title,
		Type:               m.Chat.Type,
		AddedBy:            m.From.ID,
		AddedByName:        strings.TrimSpace(m.From.FirstName + " " + m.From.LastName),
		Status:             m.Status,
		AddedAt:            d.now(),
		CanRestrictMembers: m.CanRestrictMembers,
		CanInviteUsers:     m.CanInviteUsers,
	}
	d.knownMutex.Lock()
	if old, ok := d.knownChats[chat.ID]; ok {
		chat.AddedBy, chat.AddedByName, chat.AddedAt = old.AddedBy, old.AddedByName, old.AddedAt
		if chat.Title == "" {
			chat.Title = old.Title
		}
	}
	d.knownChats[chat.ID] = chat
	d.knownMutex.Unlock()
	if d.knownChatStorage != nil {
		if err := d.knownChatStorage.StoreKnownChat(storeCtx, chat); err != nil {
			d.log.Errorf("can't remember chat %d: %s", chat.ID, err)
		}
	}
	if _, ok := d.getProtectedChannel(chat.ID); ok {
		_ = d.updateProtectedChannel(storeCtx, chat.ID, func(pc *ProtectedChannel) {
			pc.Title, pc.ChatType = chat.Title, chat.Type
			if pc.AddedBy == 0 {
				pc.AddedBy, pc.AddedByName, pc.AddedAt = chat.AddedBy, chat.AddedByName, chat.AddedAt
			}
		})
		d.channelsMutex.Lock()
		ch := d.channels[chat.ID]
		ch.title, ch.chatType = chat.Title, chat.Type
		ch.botOnChannel = true
		ch.botCanClean, ch.botCanInvite = chat.CanRestrictMembers, chat.CanInviteUsers
		d.channels[chat.ID] = ch
		d.channelsMutex.Unlock()
	}
	d.log.Infof("bot membership %s in %s %d (%s)", chat.Status, chat.Type, chat.ID, chat.Title)
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
	chat, known := d.knownChats[chatID]
	d.knownMutex.Unlock()
	if !known {
		return fmt.Errorf("the bot is not an administrator of chat %d", chatID)
	}
	if _, protected := d.getProtectedChannel(chatID); protected {
		return fmt.Errorf("chat %d is already protected", chatID)
	}
	err := d.AddDefaultProtectedChannel(&ProtectedChannel{
		ID:                chatID,
		Title:             chat.Title,
		ChatType:          chat.Type,
		AddedBy:           chat.AddedBy,
		AddedByName:       chat.AddedByName,
		AddedAt:           chat.AddedAt,
		CommandChannelIDs: []int64{callerID},
		AutoScan:          true,
		AllowClean:        true,
		Managers:          []int64{callerID},
	})
	if err != nil {
		return fmt.Errorf("protect chat %d: %w", chatID, err)
	}
	d.refreshConnectedChat(chatID)

	pc, _ := d.getProtectedChannel(chatID)
	d.recordAudit(ctx, pc, audit.Event{
		ActorID: callerID, Action: audit.ActionChannelAdded,
		Details: map[string]any{"source": "miniapp"},
	}, fmt.Sprintf("%s is now protected. Connected by %s via the Mini App; reports come to this chat.",
		html.EscapeString(d.channelTitle(chatID)), actorLink(callerID, d.userName(callerID))))
	return nil
}
