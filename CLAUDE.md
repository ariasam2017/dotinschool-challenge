# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A programming-challenge platform shipped the way **x-ui / 3x-ui** panels are: one
static Go binary that **embeds the built Vue SPA** (`web/assets/`, via `go:embed`)
and serves a small REST API over a **single SQLite file** (pure-Go driver,
`CGO_ENABLED=0`). It runs on a raw IP over plain HTTP with no errors, and speaks
TLS directly if cert/key paths are set. Two panels — participant and admin — are
built from the **same `index.html`**.

## Commands

```bash
make dev                 # Go server :8080 + Vite dev server :5173 (proxies /api). Primary dev loop.
go run ./cmd/server      # server only (needs web/assets/ already built)
go run ./cmd/seed        # wipe participants + reseed ~16 demo rows + a running 90-min event
                         #   (distinct from the boot-time database.Seed, which is 7 rows + a paused event)
make build               # web-fast + CGO_ENABLED=0 go build -> ./dotinschool
make test                # go test ./... + frontend typecheck (needs GOARCH=amd64 prefix below)
make tidy                # gofmt -w . + go mod tidy
make release             # cross-compiled artifacts into dist/ (scripts/build.sh)
make image               # Docker image (host-compile then package; scripts/docker-build.sh)

cd web && npm run build       # vue-tsc --noEmit + vite build -> web/assets/
cd web && npm run build:fast  # vite build, no typecheck
cd web && npm run typecheck   # vue-tsc only
```

Local run: user panel `http://127.0.0.1:8080/`, admin `http://127.0.0.1:8080/admin/`.
Admin password is read from `ADMIN_PASSWORD` in `.env`; if unset, a random one is
generated, written to `.env`, and printed to the log on first boot.

### Verifying changes on the Windows dev box (important)

The installed Go toolchain reports `windows/386` (`go env GOHOSTARCH` = 386), and
a bare `go build` / `go test` **fails inside `modernc.org/sqlite`** because that
driver has no 386 build. The host is really 64-bit, so prefix commands with
`GOARCH=amd64` and everything — including running the test binaries — works:

```bash
GOARCH=amd64 CGO_ENABLED=0 go test -count=1 ./internal/...          # runs the suites
GOARCH=amd64 go test ./internal/service/ -run TestName -v           # a single test
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build ./...              # the real Docker target
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...              # sometimes OOMs while LINKING
gofmt -l internal/ cmd/                                             # must print nothing before committing
GOARCH=amd64 make test                                              # `make test`/`make tidy` need the prefix too
```

Add `GOFLAGS=-p=2` if a build/test run OOMs (the 32-bit `go` process is
address-space-limited; the `windows/amd64` link step is the usual casualty —
`linux/amd64` always builds and is what Docker actually ships).

Test files: `internal/service/{ai,game,participant,ratelimit}_test.go`,
`internal/web/handlers_ai_test.go`. There is **no in-repo HTTP integration
harness** — endpoint behaviour is checked by building `./cmd/server`, running it
against a throwaway `DATA_DIR`, and curling it.

The **frontend build is memory-hungry** — `vue-tsc`/`vite` OOMs when free RAM
drops below ~2 GB. Recover with
`NODE_OPTIONS="--max-old-space-size=2560" npm run build` and by closing
browsers/containers.

## Architecture

### One binary, one index.html, two panels

`internal/web/static.go` serves the embedded SPA. A strict CSP (`middleware.go`,
`securityHeaders`) **blocks inline `<script>`**, so the server can't hand the SPA
runtime config as JS. Instead it rewrites a `<!--DOTIN_HEAD-->` token in
`index.html` into `<meta name="dotin-mode" content="user|admin">` and
`<meta name="dotin-base" content="/secret-path/">` per request. `web/src/lib/mode.ts`
reads those tags; `web/src/router/index.ts` then mounts either `userRoutes` or
`adminRoutes`. The admin panel lives at a **random secret path** on Linux installs
(or a dedicated sub-domain); the SPA never knows the path until the meta tag tells
it. `mountStatic` is registered **last** in `server.go` — it owns every non-`/api`
route via `NoRoute`.

### Go layering (`cmd/server/main.go` wires it top-down)

`config.Load` (reads `.env`, may append a generated admin password back to it) →
`database.Open` (GORM + SQLite) + `EnsureDefaults` + `Seed` (**demo data is always
seeded** at boot) → `service.New` → `web.NewRouter` (Gin) → `service.NewScheduler`
(cron; flips a scheduled event live the moment `eventStartsAt` passes, no admin
action).

- **`internal/service/`** is the business layer (`Settings`, `Participants`,
  `Sessions`, `RateLimiter`, `OTP`, `SMS`, `AI`, `ClientErrors`). It depends only
  on `model`, `config`, and the `*gorm.DB`. `Services` is the bundle handed to the
  web layer.
- **`internal/web/server.go` `mountAPI`** registers every `/api/*` route with its
  exact method, auth (`requireAdmin()` bearer token), and rate-limit rules.
  Handlers are split by domain: `handlers_{admin,ai,appearance,backup,
  client_errors,participants,public,sms,system}.go`.
- **`internal/web/render.go`**: `fail`/`failWith` render a typed `*apperr.Error`
  (from `internal/apperr`) as `{code, message}` JSON; `failWith` also logs
  non-`apperr` errors server-side without leaking them. `bindJSON` for request
  bodies. Use these rather than `c.JSON` for errors.
- **Config precedence for SMS (ippanel) and AI**: panel-saved settings (DB) >
  `data/*-config.json` sidecar > `.env`. It's resolved **fresh on every call**, so
  pasting a key in the admin panel takes effect with no restart. `SMS`/`AI` are
  always the real provider; "configured?" is computed per request.
- **Self-update**: `handleUpdate` runs `git fetch` + `git reset --hard origin/main`
  then `os.Exit(0)`, relying on systemd/Docker to bring the process back up (Docker
  installs print a manual command instead).

### API surface & auth model

Full endpoint-by-endpoint table lives in `README.md` ("قرارداد API") — this is
the cross-file business logic that table doesn't spell out:

- **`PATCH /api/participants/:id`** accepts either the admin bearer token or
  the participant's own session token, but the two callers can't do the same
  things: `name`/`aiScore` are admin-only, and a `status` transition to
  `started`/`submitted` is honored **only while the event is live** — the
  server enforces this itself (`handlePatchParticipant` in
  `handlers_participants.go`), ignoring a client-sent status otherwise, so the
  frontend's own state never has to be trusted.
- **Sessions**: admin bearer token lives 12h server-side in memory (not a
  JWT); participant session token is sliding 30 days. Both the admin password
  and OTP checks are timing-safe comparisons.
- **OTP**: 5-digit code, 2-minute TTL, 60s resend cooldown, 5 verify attempts.
- Sensitive routes (login, OTP request, AI review, client-error reporting)
  carry an in-memory rate limit (`internal/service/ratelimit.go`); failed
  admin-login attempts are logged by IP, and `dotinschool fail2ban`
  (`scripts/install.sh`) auto-bans after repeated failures on bare-metal
  installs.

### Quiz game ("لیگ قهرمانان برنامه‌نویسی")

A Quiz-of-Kings style individual quiz that is the **user panel's landing page**
(`/` → `views/game/GameApp.vue`); the coding challenge moved to `/challenge`.
Admin pages: `admin/game` (live board, default admin route) and
`admin/game-questions` (bank editor, CSV/JSON import). Backend:
`service/game.go` + `handlers_game.go`, tables `game_questions` / `game_players`,
settings key `game`. Registration is name + mobile, **no OTP**; the player token
is persisted on the row (survives restarts, unlike `SessionService`). The server
deals a random question subset with shuffled options, **never sends the answer
key before the answer**, and times each answer from when it served the question
(`answerGraceMs` latency allowance; an abandoned question times out lazily in
`State`). Ranking: correct count desc, then total time asc; score =
100 × correct + up to 99 speed bonus (`ComputeGameScore`); the player API never
sends time/score back — only the correct count (the admin board has both).

- **Concurrency** (sized for ~40 simultaneous players): per-player *striped*
  locks (`lockPlayer`, keyed by player id) instead of one global mutex,
  settings cached in an `atomic.Pointer` (refreshed by `SaveSettings`), and
  questions dealt from a shuffled **deck** (`deal`) so consecutive players get
  distinct questions until the bank is exhausted; any bank mutation bumps
  `bankRev` and the deck is rebuilt. `TestGameFortyConcurrentPlayers` covers it.
- **Built-in bank** (`model/game_bank.go`, correct answer written first, rotated
  on seed) is versioned: bump `GameBankVersion` when adding questions and
  `seedGameBank` adds the missing ones (by exact text) to existing DBs once;
  list replaced questions in `RetiredBankTexts`. `TestBuiltInBankIsClean`
  fails on a duplicate or invalid built-in question.
- **Test mode** (`settings.testMode`) exposes `POST /api/game/replay` so a
  player can replay from the result screen; refused (403) when off.
- **Winner reveal** (`components/WinnerReveal.vue` + `lib/revealSound.ts`,
  Web Audio — no audio files): rolls names per place (3rd→1st) for
  `reveal{Third,Second,First}Sec`, then holds on "؟" until Enter/tap. The
  admin board also has a demo mode with made-up players (no DB access).

### Data-movement paths (all in the admin overview "پشتیبان‌گیری دیتابیس" card)

- **Whole-DB backup / restore** (`handlers_backup.go`) — backup is `VACUUM INTO`.
  Restore stages the upload, then **`database.Validate`** opens + AutoMigrates it
  in a throwaway connection and smoke-queries the core tables; an incompatible or
  corrupt file is rejected (`RESTORE_INCOMPATIBLE`, 400) with the live DB
  untouched, instead of crash-looping on the next boot. The swap then calls
  **`database.CleanSidecars(path)`** (removes `-journal` **and** `-wal`/`-shm`)
  before `os.Rename` + `os.Exit(0)`. The app runs `journal_mode(TRUNCATE)`, so a
  stale `arena.db-journal` left next to a freshly swapped-in file is the exact bug
  that bricked a cross-server restore — **any code that replaces the DB file must
  `CleanSidecars` first.**
- **Participant-only transfer** (`handlers_participants_io.go`) — a portable
  `.json` of every participant row. Import is **replace-all in one transaction**
  (`ParticipantService.ReplaceAll`), validated up front, **no restart**. For
  moving entrants between differently-built servers.

### Frontend (`web/`, Vue 3 + Ant Design Vue 4 + Vite)

- **Build output goes to `web/assets/` and is committed on purpose** (see
  `.gitignore` note) — a bare-metal install must not need Node. `npm run build`
  fully prepares the tree for `go build`. When you change frontend source, rebuild
  and commit `web/assets/` alongside it.
- Ant Design Vue components are **auto-imported** (`unplugin-vue-components`,
  `src/components.d.ts`) — no explicit import needed for `a-*` components.
- **Pinia stores** (`web/src/stores/`): `admin` (auth gate + participants list
  polled every 3 s + unseen-error badge), `event` (countdown / event config),
  `participant`.
- **Persian everywhere**: `lib/fa.ts` + `lib/fa-digits.ts` (digit substitution),
  `lib/jalali.ts` (Jalali calendar). The `.num` CSS class keeps IRANYekanX (so
  Persian digits render) plus tabular figures; `.mono` is JetBrains Mono. Layout
  is RTL. Fonts are self-hosted in `styles/fonts.css` (no CDN — strict CSP).
- **Theming** (`styles/tokens.css` + `lib/theme.ts`): `theme.ts` stamps
  `data-panel` (= `APP_MODE`) and `data-theme` on `<html>`. `tokens.css` is a
  three-state theme — bare `:root` = light, `@media (prefers-color-scheme: dark)`
  guarded by `:root:not([data-theme="light"])`, and explicit `:root[data-theme="dark"]`.
  The admin panel additionally gets a denser spacing scale and its own dark
  surface override, both keyed on `[data-panel="admin"]`.
- **Design Studio** (`views/admin/AppearanceSection.vue` + `lib/appearance.ts`):
  the admin edits the *user* panel's appearance at runtime (CSS token overrides,
  text find/replace, per-element style/visibility overrides, viewport-aware) with
  **no rebuild**. Applied via an injected `<style id="dotin-appearance">`; a
  preview iframe communicates over `postMessage` (`appearance` / `device` / `pick`
  / `retarget`). The layer is stored server-side and served inside the user
  panel's config.

## Conventions & gotchas

- **Commit messages**: Conventional Commits, English (`feat/fix/docs/refactor/build/chore`).
- **The in-app "update" flow destroys uncommitted work.** The admin "بررسی و اعمال
  به‌روزرسانی" button, `dotinschool update`, and `update.ps1`/`update.sh` all run
  `git reset --hard origin/main`. Commit changes to a branch before testing via
  any update path.
- `scripts/windows-install.ps1` and `docker-up.ps1` must stay **pure ASCII** (no
  BOM) — PowerShell 5.1 mis-reads non-ASCII. There's a comment at the top of each
  explaining this.
- If editing `README.md`, keep the `<div dir="rtl">` open/close counts equal and
  keep non-ASCII characters out of fenced code blocks.
- `data/` (the SQLite file, generated `.env` password, AI/SMS sidecars) is runtime
  state and git-ignored.
- **AI "model" field autofill trap**: in the provider editor the "نام مدل" text
  input sits right before the API-key input, so a browser password manager
  autofills the *login username* into it; saved, every AI call then sends
  `model:"administrator"` and the gateway answers "No available channel for model
  administrator". `AIService.ModelOrDefault` self-heals `model == AdminUsername`
  (and empty) to the default model everywhere it's read.
- Deploy targets: bare-metal (`scripts/install.sh` + systemd + a `dotinschool` CLI
  menu) and Docker (`build/`, `docker-compose*.yml`, Caddy for auto-HTTPS in prod).
