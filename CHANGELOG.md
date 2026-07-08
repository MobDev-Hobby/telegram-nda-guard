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

- **Channel users view.** New `/users <id>` command (message and inline-button
  callback) that lists a channel's members, classified into Good / Unknown / Bad
  by the channel's access checker — a read-only on-demand counterpart to
  `/scan`. Large listings are truncated per-section with a hint to run `/scan`
  for a full persisted report. `/list` now shows a `/users` button per channel.
- Established this `CHANGELOG.md` and the migration-note convention documented
  above. Future interface/contract changes will be recorded under this section
  until the next tagged release.

### Migration

No action required for consumers at this point. This entry only formalizes the
changelog going forward.

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
