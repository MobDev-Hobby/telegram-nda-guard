# Changelog

All notable changes to **telegram-nda-guard** are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project aims to adhere to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## How this project uses the changelog

`telegram-nda-guard` is a framework: other projects consume its packages
(`controllers/scanner`, `processors/*`, `storage/*`, `telegram/*`) and implement
its interfaces. **Any change to a public interface, constructor, functional option,
or DTO that consumers depend on must be recorded here** so consumers can migrate.

Each release section contains:

- **Added** — new capabilities, new interface methods, new options, new packages.
- **Changed** — modifications to existing behavior, signatures, or defaults.
- **Deprecated** — soon-to-be-removed features.
- **Removed** — deleted capabilities (always breaking).
- **Fixed** — bug fixes.
- **Migration** — concrete steps consumers must take when a change is breaking or
  requires wiring adjustments. If an interface gained a method, `Migration` lists
  every other implementation (including custom ones) that must add it.

When in doubt, add a `Migration` note. It is cheaper than a silent break.

---

## [Unreleased]

## [0.4.0] — 2026-09-25

Both batches below ship in this release.

### Channels, Mini App, per-channel cleanup (September 2026)

#### Added

- **Channels can be added from the bot.** `/add` now offers "Add channel" and
  "Add group" buttons. The single "Select channel" button used to send
  `chat_is_channel: false` (go-telegram/bot serialises the field even when
  false), so Telegram only ever listed groups. New
  `guard.Button.RequestChatIsChannel` selects the picker kind.
- **Telegram Mini App.** `webapi.WithMiniApp(service, channelAuth)` serves an
  embedded Mini App under `/miniapp/` with an API under `/api/miniapp/`:
  channel administrators see the channels they administer, run a scan with
  live progress, tick members to remove and change every per-channel setting.
  Auth is Telegram `initData` (validated with the bot token) exchanged for a
  Bearer session. New `scanner.MiniAppService` (implemented by `*Domain`),
  `scanner.ChannelSettings`, `scanner.ScanView`, `scanner.ScannedUser`,
  `scanner.UserKicker`, options `scanner.WithUserKicker`,
  `scanner.WithDefaultCleanOptions`, `scanner.WithMiniApp`, the `/app` command
  and a Mini App menu button. `webapi.Server.Handler()` exposes the handler
  for mounting into an existing server.
- **Mini App v2.**
  - *Employees only:* sign-in runs the default access checker on the Telegram
    user; failures get `403 {"code":"not_employee"}`. Sessions last 1 hour
    (`webapi.WithMiniAppSessionTTL`), so the check repeats.
  - *Managers:* a channel's Telegram admins see it in the app; to manage it
    they press **Join**, which adds them to `ProtectedChannel.Managers`
    (persisted). All channel routes need admin + manager (or privileged);
    managers get reminders in private.
  - *Connect from the app:* chats where the bot became an admin
    (`my_chat_member`, `WithKnownChatStorage`) and that aren't protected are
    offered under **Available to connect**; **Add via bot** opens
    `t.me/<bot>?start=add`, which runs `/add` in the private chat. In private
    chats `/add` is allowed for anyone passing the checker — Telegram's picker
    only lists chats the user administers.
  - *Health traffic light:* `ChannelView.Health` — red for violations in the
    last check or no check for 7 days, yellow for never / 3+ days, green
    otherwise. Every scan (bot, schedule, Mini App), recheck and kick updates
    `ProtectedChannel.LastCheck`.
  - *Member traffic light & recheck:* scan statuses `good` / `whitelisted` /
    `unknown` / `bad` / `kicked`; `POST …/scans/{id}/users/{uid}/recheck`
    invalidates the checker cache (`checker/cached.Domain.Invalidate`) and
    asks again, whitelisted users included (their `check` shows the raw
    verdict).
  - *Action log:* `WithAuditStorage` (`storage/audit/redis`, capped Redis
    list, 500 per channel) records adds/joins/settings/scans/cleans/kicks/
    rechecks/whitelist/join-request events; `GET …/audit`, shown in Settings.
  - *Join request manager:* `WithJoinRequestStorage`,
    `ChannelSettings.JoinRequests` = `off` | `auto` | `manual`. Auto approves
    requesters who pass and leaves the rest pending, rechecking daily; manual
    keeps them for managers (`…/joins`, `approve`, `decline`, recheck).
    The Bot API can't list pending requests, so only ones arriving after
    deploy are seen. Needs the bot's `can_invite_users`.
- **Per-channel whitelist with periodic re-approval.** `scanner.WithWhitelistStorage`
  (+ `storage/whitelist` model and `storage/whitelist/redis`). Channel admins
  add users from a Mini App scan; an active entry makes the user pass every
  access check in that channel and protects them from Mini App kicks. An
    approval must be reviewed every `WithWhitelistTTL` (default 30 days); an
  overdue entry keeps protecting the user and triggers a daily reminder to
  the control chats and managers until renewed or removed. An optional term
  (`ttlDays`, 1–365) makes an entry temporary: it is removed at the end of
  the term. Entries carry the approver's note (≤500 chars). Control chats get
  a reminder `WithWhitelistRemindBefore` (default 3 days) ahead, a notice on
  expiry, and an audit message on every add/renew/remove. Mini App: a
  "Whitelist" tab and "Add to whitelist" in scan results; API
  `GET|POST /api/miniapp/channels/{id}/whitelist`,
  `POST …/whitelist/{userId}/renew`, `DELETE …/whitelist/{userId}`.
  `scanner.MiniAppService` gained `ListWhitelist`, `AddToWhitelist`,
  `RenewWhitelistEntry`, `RemoveWhitelistEntry`.
- **Per-channel cleanup settings.** `processors.CleanOptions` (`KeepBanned`,
  `CleanMessages`, `CleanUnknown`) on `scanner.ProtectedChannel`,
  `channels.ProtectedChannel` (persisted, omitted when unset) and
  `processors.AccessReport`; the kicker uses them over its defaults.
  `kicker.Domain.KickUsers` removes a given list of users and returns
  `[]processors.KickResult`.
- **Partial member lists are reported.** `guard.ScanStats` (fetched vs total);
  the userbot records Telegram's participant `Count`, and scan/clean reports
  and the Mini App warn when Telegram returned only part of the members.
- `authorizer.HybridAuthorizer.AuthorizeChannel` / `IsPrivileged` and
  `WithAdminCacheTTL` (administrator lists are cached for a minute).
- `guard.InlineButton.URL` / `WebAppURL`, `guard.CallbackQuery.From`.

#### Security

- Inline-button presses were authorized against
  `CallbackQuery.Message.User` — the author of the message carrying the
  button, i.e. the bot. With `REQUIRE_ADMIN_AUTH` the bot is an admin of the
  chat, so any member could run `/scan`, `/clean`, … by pressing a button.
  Presses are now authorized by `CallbackQuery.From`.
- `/settings`, `/setflag`, `/users`, `/remove`, `/rmconfirm` and the chat
  share of `/add` ran without the authorizer, and the ones taking a channel id
  did not check that the channel is controlled from the current chat: any
  chat could list the members of, reconfigure or detach any protected channel.
  They now require authorization and a linked channel.
- Web dashboard: `?chat=` authorized the caller for that chat but was never
  matched against the channel, so an admin of any chat could act on every
  channel. The channel must now be controlled from `?chat=`.

#### Fixed

- Mini App: controls styled with `all: unset` ignored the `hidden`
  attribute.
- The kicker skipped the ban when both `KeepBanned` and `CleanMessages` were
  off and only issued an `OnlyIfBanned` unban — a no-op — while counting the
  user as kicked. It now always bans first.
- Scan workers: `WithNProcessingThreads` workers ran one after another in one
  goroutine, so only the first ever processed requests.
- The launch notification swapped "Auto scan" and "Auto clean".
- Data race on the cached userbot's member cache between scans and MTProto
  update handlers.

#### Migration

- `ChannelView` gained `keepBanned`, `cleanMessages`, `cleanUnknown`,
  `customCleanOptions`; consumers of the JSON API see extra fields only.
- The kicker no longer relies on `WithCleanMessages(true)` to actually remove
  users; if you set `KeepBanned=false, CleanMessages=false` expecting a no-op,
  that configuration now removes users.
- Custom `Authorizer` implementations should read `CallbackQuery.From` for
  callbacks (see Security).
- **`telegram/bots/bot.New` now requests only `message`, `callback_query`,
  `my_chat_member` and `chat_join_request` updates** (explicit
  `allowed_updates`; Telegram used to reuse whatever list was set last). If
  you register your own handlers for other update types (e.g. `channel_post`,
  `chat_member`), they stop receiving updates; build the bot with your own
  list instead.
- `guard.CallbackQuery` and `telegram/userbots/userbot.Domain` are no longer
  comparable (`==`), because of the new `From` field and internal state.
- Scan workers now really run in parallel (`WithNProcessingThreads`, default
  4): expect up to that many concurrent member listings and checker calls.
- `HybridAuthorizer` caches chat admin lists for a minute
  (`WithAdminCacheTTL(0)` restores a lookup per command); a demoted admin
  keeps access up to that long.
- Bot commands that take a channel id (`/settings`, `/setflag`, `/users`,
  `/remove`, `/rmconfirm`) now go through the authorizer and must come from a
  control chat of that channel; `/add` in a private chat is allowed for anyone
  who passes the default access checker.
- Storage: new optional ports (`WithWhitelistStorage`, `WithAuditStorage`,
  `WithKnownChatStorage`, `WithJoinRequestStorage`); records in `pChannel`
  gain optional fields and stay readable by older versions. A custom Redis
  client for the audit log needs `ListPush`/`ListRange`.
- Mini App users must **Join** each channel once before managing it
  (`ProtectedChannel.Managers` starts empty).
- This is a pre-1.0 minor release: it contains the breaking changes above.
- Mini App (optional): serve `webapi.Server` over public HTTPS, pass
  `webapi.WithMiniApp(domain, hybridAuthorizer)` and
  `scanner.WithUserKicker(kicker)`, `scanner.WithDefaultCleanOptions(kicker.DefaultCleanOptions())`,
  `scanner.WithMiniApp(url, shortName)`; register the Mini App in BotFather.

### Management UI, authorization, hardening (July 2026)

#### Added

- `guard.ChannelInfo.Type` (`string`) — the Telegram chat type, plus
  `ChatType*` constants (`ChatTypePrivate`, `ChatTypeGroup`,
  `ChatTypeSupergroup`, `ChatTypeChannel`) and a `guard.ChatTypeNoun(type)`
  helper that returns `"channel"` for broadcast channels and `"chat"`
  otherwise. This lets consumers distinguish channels from chats/groups.
  `bots/bot.GetChat` now populates `Type` from the Bot API response, and the
  scan/clean reports (reporter and kicker) use the correct noun instead of the
  hardcoded "chat".
- `scanner.ProtectedChannelStorage.Drop(ctx, channelID int64) error` — removes
  a protected channel's persisted record. `CleanProtectedChannel` now calls
  `Drop` when a channel is fully detached (no remaining controlling chats) and
  `Store` when its set of controlling chats changes, so in-memory removals
  survive restarts.
- `kicker.TelegramBotUserKicker` is now transport-agnostic (`Ban`, `Unban`,
  `SendReport`). `telegram/sender/ratelimited.Restrictor` implements it on top
  of `telegram/bots/bot.Domain` (`Ban`/`Unban`/`SendReportMessage`, including
  the Bot API `-100` chat-ID normalization that previously lived in the
  kicker), adding rate limiting and FLOOD_WAIT retries.
- **Command authorization subsystem.** New `scanner.Authorizer` interface
  (`Authorize(ctx, *guard.Update) (bool, error)`) and a `scanner.WithAuthorizer`
  option. The bundled default lives in `controllers/scanner/authorizer` as
  `HybridAuthorizer`, which allows the configured owner, an explicit allowlist,
  and (optionally) administrators of the originating chat.
- `scanner.TelegramBot.GetChatAdministrators(ctx, chatID) ([]int64, error)` —
  implemented by `telegram/bots/bot.Domain`, used by the hybrid authorizer's
  admin check.
- `REQUIRE_ADMIN_AUTH` env var in `cmd/example` to opt into owner+admin
  authorization (off by default for backwards compatibility).
- **Channel settings UI.** New `/settings` command and `/settings <id>` +
  `/setflag <id> <flag>` inline-button callbacks that toggle `AutoScan`,
  `AutoClean`, `AllowClean` per channel at runtime, persisted and synced with
  the periodic ticker. `/list` now shows a ⚙ button per channel.
- **Channel users view.** New `/users <id>` command (message and inline-button)
  listing a channel's members classified into Good / Unknown / Bad. `/list`
  shows a `/users` button per channel.
- **Channel removal UI.** New `/remove <id>` command with a two-step inline
  confirmation (`/rmconfirm <id>`). Detaching a channel is now possible from
  the bot; previously `CleanProtectedChannel` existed but was dead code.
- **Web management dashboard.** New transport-neutral `scanner.ManagementService`
  interface (implemented by `*Domain`), a `scanner.WebAuthenticator` interface,
  and the `controllers/scanner/webapi` package — a JSON REST API + embedded SPA
  dashboard authenticated via the Telegram Login Widget. The same
  `HybridAuthorizer` serves both the Telegram `Authorizer` and the
  `WebAuthenticator` roles. Enabled in `cmd/example` via `WEB_ADDR` +
  `WEB_SESSION_SECRET`. `Domain.Ready()` signals initialization completion.
- Established this `CHANGELOG.md` and the migration-note convention documented
  above. Future interface/contract changes will be recorded under this section
  until the next tagged release.

#### Security

- Previously **any member of a controlling chat** could run `/scan`, `/clean`,
  `/list`, `/add` and similar commands — there was no authorization at all. The
  new `Authorizer` is a pluggable chokepoint; protected commands now go through
  `Domain.requireAuth`. The default remains allow-all unless an authorizer is
  configured, so existing deployments keep working.

#### Changed

- **`kicker.New` now takes a `kicker.TelegramBotUserKicker` (`Ban`, `Unban`,
  `SendReport`) instead of `*bot.Bot`.** The kicker no longer imports
  `github.com/go-telegram/bot`.

#### Fixed

- **Layering violation:** `storage/channels/redis/drop.go` previously imported
  `controllers/scanner` (a controller→storage dependency in reverse) and used
  `scanner.ProtectedChannel`. It now accepts a plain `channelID int64` and uses
  only the storage layer, removing the dependency cycle.
- `cmd/example/main.go` called `kicker.WithKeepBanned(options.KickUnknownUsers)`
  twice; the second call should have been `kicker.WithCleanUnknown(...)`, so the
  "kick unknown users" flag was silently ignored. Fixed.

#### Migration

- **`ProtectedChannelStorage` gained a `Drop` method.** Any custom
  implementation of this interface (including non-redis backends) must add:
  ```go
  Drop(ctx context.Context, channelID int64) error
  ```
  The method must be idempotent (deleting a missing record must not error).
  The bundled `storage/channels/redis` implementation already provides it.
- `redis.Domain.Drop` changed its signature from
  `Drop(ctx, *scanner.ProtectedChannel)` to `Drop(ctx, channelID int64)`.
  Anyone calling the old (previously unused) method must update the call site.
- **`kicker.New`** — callers that passed `telegramBotDomain.GetBot()`
  (`*bot.Bot`) must now pass a `kicker.TelegramBotUserKicker`, normally
  `ratelimited.NewRestrictor(telegramBotDomain, time.Second, 5)`.
- Any custom type passed to the kicker must implement `Ban`, `Unban` and
  `SendReport`.
- **`scanner.TelegramBot` gained `GetChatAdministrators`.** Any custom
  implementation of this interface must add it. The bundled
  `telegram/bots/bot.Domain` already provides it.
- Optional, non-breaking: to restrict who can run commands, pass
  `scanner.WithAuthorizer(...)`. Without it, behavior is unchanged.
- **Web dashboard (optional):** to enable the management API, set `WEB_ADDR`
  (e.g. `:8080`) and `WEB_SESSION_SECRET` (>=32 bytes) in `cmd/example`. The
  dashboard is off by default. `scanner.ManagementService` and
  `scanner.WebAuthenticator` are new interfaces; `*Domain` implements the
  former, and the bundled `HybridAuthorizer` implements both. No existing
  consumer is forced to adopt them.

---

## [0.1.0] — 2024 (full refactoring, no backward compatibility)

### Changed

- **Breaking:** project-wide refactoring. The phone-auth user bot was removed;
  the user bot now uses the bot account (more secure). The `SessionStorage`
  contract was changed to match the new userbot version. Existing callers that
  constructed the previous user bot / session storage need to be updated to the
  new constructors and options.

### Migration

This release intentionally broke backward compatibility. Consumers must:

1. Update user bot construction to the new bot-account-based flow.
2. Update `SessionStorage` implementations to the new method contract.
3. Review `cmd/example/main.go` for the current wiring reference.

> Note: this historical release shipped **without** a migration document at the
> time. It is recorded here retroactively for completeness.

---

## Conventional locations of the consumer-facing interfaces

The framework's public surface lives in these files. Changes here are the most
likely to require a `Migration` note:

| Interface / type | File | Purpose |
|---|---|---|
| `guard.User`, `guard.ChannelInfo`, `guard.Message`, `guard.InlineButton`, `guard.Permission` | `defs.go` | Core domain DTOs |
| `scanner.ProtectedChannelStorage`, `scanner.Logger`, `scanner.UserReportProcessor`, `scanner.CheckUserAccess`, `scanner.UserBot`, `scanner.TelegramBot` | `controllers/scanner/dep.go` | Controller-side contracts |
| `scanner.ProtectedChannel`, `scanner.ChannelInfo` (controller), `scanner.ScanRequest` | `controllers/scanner/defs.go` | Controller domain model |
| `scanner.ProcessorOption` + `With*` options | `controllers/scanner/options.go` | Controller configuration |
| `processors.AccessReport` | `processors/defs.go` | Report DTO shared with processors |
| `kicker.TelegramBotUserKicker`, `kicker.Option` | `processors/kicker/dep.go`, `options.go` | Cleaner processor contract |
| `reporter.TelegramBotMessageSender`, `reporter.Option` | `processors/reporter/dep.go`, `options.go` | Reporter processor contract |
| `channels.ProtectedChannel` (storage model) | `storage/channels/defs.go` | Persisted channel model |
