# Telegram NDA Guard — Development Guidelines

Go service that protects Telegram channels: a **userbot** (MTProto via
`gotd/td`) and a **bot** (`go-telegram/bot`) scan channel members and
kick/ban those who fail an access check. Optional Redis persistence and a
web management UI.

## Tech stack (actual)

- **Language:** Go 1.21 (`go.mod`), toolchain `go1.23.x`
- **Telegram:** `github.com/go-telegram/bot` (Bot API), `github.com/gotd/td` + `github.com/gotd/contrib` (MTProto userbot, with `floodwait` middleware)
- **Storage:** Redis via `github.com/redis/go-redis/v9` (sessions + channel config), file fallback for sessions
- **Config:** `github.com/caarlos0/env/v11` + `github.com/joho/godotenv`
- **Logging:** `go.uber.org/zap`
- **Rate limiting:** `golang.org/x/time/rate`
- **Crypto:** stdlib `crypto/aes` + `crypto/cipher` (AES-GCM) for session + channel data at rest
- **Tests/mocks:** `github.com/stretchr/testify`, `github.com/golang/mock` (`mockgen`)

## Architecture (hexagonal / ports-and-adapters)

```
cmd/example          entrypoint + DI wiring (main.go, env_vars.go)
defs.go              shared DTOs (User, Message, Update, ChannelInfo, …)
controllers/scanner  orchestration: command handlers, scan/clean loop,
                     access-rights checker, authorization, management API
  authorizer/        command/web authorization (hybrid: owner + chat admins)
  webapi/            optional HTTP management API + Telegram-Login auth
checker/             access-check abstraction
  demo/              placeholder (admits users by ID parity — demo only)
  cached/            TTL cache wrapper (must wrap any real checker)
processors/          scan/clean result handlers
  kicker/            ban/unban via a rate-limited restrictor
  reporter/          send scan reports to control chats
  multiplexor/       fan-out over multiple processors
storage/             persistence ports + adapters
  channels/          protected-channel config (defs + redis)
  session/           Telegram session blob (file + redis, AES-GCM)
  drivers/go-redis/  go-redis → internal RedisClient adapter
telegram/            Telegram transport
  bots/bot/          Bot API wrapper (+ Ban/Unban/SendReportMessage restrictor)
  sender/ratelimited/  rate-limited SendMessage + Restrictor (FLOOD_WAIT-aware)
  userbots/          MTProto userbot (userbot/ + cached/ wrapper)
utils/               small generics helpers (Ptr, UnPtr)
```

### Key contracts
- A checker implements `HasAccess(ctx, *guard.User) (bool, error)` — `true` = allow, `false` = kick, non-nil `err` = classify as "Unknown" (not kicked). See `controllers/scanner/dep.go`.
- The scanner always wraps a checker in `checker/cached` (concurrency + TTL). Never call a raw checker from a scan path.
- The kicker takes a `TelegramBotUserKicker` (Ban/Unban/SendReport); wire it through `ratelimited.NewRestrictor(...)` — never pass the raw `*bot.Bot`, or Telegram will rate-limit/ban the account.

## Concurrency rules (IMPORTANT)

Several maps on `scanner.Domain` (`commandChannels`, `protectedChannels`, `channels`, `addChannelHandlers`) are accessed from multiple goroutines (rights-checker loop, scan worker pool, ticker goroutine, per-update handlers). **Always** use the accessors in `channels_mutex.go` (`getChannel`, `getProtectedChannel`, `getCommandChannels`, `hasCommandChannels`, `snapshotChannelIDs`) or hold `channelsMutex` directly. Never range over `d.channels` while the body may mutate it — snapshot first. The cached checker's map is guarded by its own `sync.RWMutex`.

## Development commands

```bash
make mocks             # regenerate mocks via go generate ./...
make test              # go test + govulncheck + gosec + go vet + golangci-lint
make test-coverage     # go test -cover + HTML report

go run cmd/example/main.go          # run locally (needs .env)
go test ./...
go vet ./...
```

### Mocks
```bash
mockgen -source=checker/cached/dep.go -destination=checker/cached/dep_mock_test.go
```

## Environment variables

See `cmd/example/env_vars.go` for the source of truth. Copy `.env.template`
to `.env` (gitignored). Highlights:

| Variable | Purpose |
|---|---|
| `TELEGRAM_BOT_KEY` | Bot API token |
| `TELEGRAM_APP_ID` / `TELEGRAM_APP_KEY` | MTProto api_id / api_hash (for the userbot) |
| `SESSION_KEY` | AES key for session/channel data at rest — **must be 16, 24 or 32 bytes** |
| `ADMIN_CHAT_ID` | Owner chat id (receives notifications; gates `/admin`) |
| `ADMIN_SECRET` | Shared secret for `/admin` bootstrap (≥32 chars; compared constant-time) |
| `CHANNELS` | Comma-separated protected channel ids |
| `COMMAND_CHANNELS` | `channelID:controlChatID` pairs, comma-separated |
| `ACCESS_CHECK_INTERVAL` / `CHANNEL_SCAN_INTERVAL` | Durations (e.g. `30m`, `2h`) |
| `CHANNEL_MEMBERS_CACHE_TTL` / `ACCESS_CHECKER_CACHE_TTL` | Cache TTLs |
| `AUTO_KICK_USERS` / `KEEP_KICKED_USERS_BANNED` / `HIDE_MESSAGES_FOR_KICKED_USERS` / `KICK_UNKNOWN_USERS` | Kicker behavior flags |
| `USE_REDIS_SESSION_STORAGE` | `true` → Redis sessions, else file |
| `REDIS_HOST` | Redis address (empty → no Redis) |
| `REQUIRE_ADMIN_AUTH` | `true` → restrict commands to owner + chat admins |
| `WEB_ADDR` / `WEB_SESSION_SECRET` | Optional web management UI (`WEB_ADDR` empty → disabled; secret ≥32 bytes when set) |

## Telegram-specific guidance

- **Rate limits:** 30 msg/s globally, 20 msg/min per chat. The ratelimited sender caps well below these; the kicker goes through `ratelimited.Restrictor` which also handles 429 (`bot.TooManyRequestsError` → sleep `RetryAfter` → retry once).
- **Userbot:** uses session storage (AES-GCM at rest), `floodwait` middleware for MTProto flood waits.
- **Channel management:** verify the bot has admin rights (invite + restrict members) before enabling AutoClean.

## Security notes

- `SESSION_KEY` is the single key protecting session + channel data at rest; generate 32 random bytes.
- Admin secret is compared with `crypto/subtle.ConstantTimeCompare`.
- `.env` is gitignored and **must not** be committed; rotate any leaked tokens/keys out of band.
- The `checker/demo` package is a **demo placeholder only** — it admits users by ID parity. Do not run it against real channels expecting meaningful access control.
