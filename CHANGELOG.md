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

### Added

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
- `processors.CleanOptions` (`KeepBanned`, `CleanMessages`, `CleanUnknown`) and
  an optional `AccessReport.CleanOptions` field, so cleanup behavior can be
  configured per-channel instead of only process-wide.
- `kicker.UserRestrictor` — a transport-agnostic domain interface
  (`SendReportMessage`, `Ban`, `Unban`) that the kicker consumes. The bundled
  `telegram/bots/bot.Domain` implements it (`Ban`/`Unban`/`SendReportMessage`),
  including the Bot API `-100` chat-ID normalization that previously lived
  inside the kicker.
- `scanner.ProtectedChannel.CleanOptions` (`*processors.CleanOptions`) —
  per-channel cleanup overrides forwarded into the cleaner's `AccessReport`.
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
- Established this `CHANGELOG.md` and the migration-note convention documented
  above. Future interface/contract changes will be recorded under this section
  until the next tagged release.

### Security

- Previously **any member of a controlling chat** could run `/scan`, `/clean`,
  `/list`, `/add` and similar commands — there was no authorization at all. The
  new `Authorizer` is a pluggable chokepoint; protected commands now go through
  `Domain.requireAuth`. The default remains allow-all unless an authorizer is
  configured, so existing deployments keep working.

### Changed

- **`kicker.New` now takes a `UserRestrictor` instead of `*bot.Bot`.** The
  kicker no longer imports `github.com/go-telegram/bot`; it operates purely on
  domain types. Per-channel `CleanOptions` (when present) override the
  kicker's process-wide defaults.
- `kicker.TelegramBotUserKicker` is **removed** in favor of `UserRestrictor`.

### Fixed

- **Layering violation:** `storage/channels/redis/drop.go` previously imported
  `controllers/scanner` (a controller→storage dependency in reverse) and used
  `scanner.ProtectedChannel`. It now accepts a plain `channelID int64` and uses
  only the storage layer, removing the dependency cycle.
- `cmd/example/main.go` called `kicker.WithKeepBanned(options.KickUnknownUsers)`
  twice; the second call should have been `kicker.WithCleanUnknown(...)`, so the
  "kick unknown users" flag was silently ignored. Fixed.

### Migration

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
- **`kicker.New(restrictor UserRestrictor, opts...)`** — callers that passed
  `telegramBotDomain.GetBot()` (`*bot.Bot`) must now pass the bot **domain**
  (`telegramBotDomain`), which implements `UserRestrictor`.
- Any custom type passed to the kicker must implement the new
  `UserRestrictor` interface (`SendReportMessage`, `Ban`, `Unban`). The old
  `TelegramBotUserKicker` interface is gone.
- Optional: to enable per-channel cleanup behavior, set
  `scanner.ProtectedChannel.CleanOptions`. Otherwise the kicker's configured
  defaults apply as before.
- **`scanner.TelegramBot` gained `GetChatAdministrators`.** Any custom
  implementation of this interface must add it. The bundled
  `telegram/bots/bot.Domain` already provides it.
- Optional, non-breaking: to restrict who can run commands, pass
  `scanner.WithAuthorizer(...)`. Without it, behavior is unchanged.

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
