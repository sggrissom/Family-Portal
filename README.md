# Family Portal

A private site for one family's record of its kids: people and their
relationships, growth measurements plotted against percentile curves, milestones,
activities, photos with face tagging, and a family chat. Data belongs to a
family, and nothing is shared outside one.

Live at [familyrecord.app](https://familyrecord.app). One Go binary, one BoltDB
file, one photo directory on disk — there is no cluster, no queue service, and
no external database.

## Architecture

```
browser ──── Caddy (TLS) ──── family_site (Go, :8666) ──── db.bolt
                                    │                      static/photos/
                                    ├── photo worker      (resize, WebP/AVIF)
                                    ├── analysis worker ──── family-face (unix socket, dlib)
                                    ├── push worker      ──── APNs        (1.1)
                                    └── mail worker      ──── Postfix on 127.0.0.1:25
```

**Backend** (`backend/`) is Go on [vbeam](https://go.hasen.dev/vbeam). Handlers
are registered with `RegisterProc()`, and vbeam generates the matching TypeScript
client into `frontend/server.ts` from the Go types — so a renamed field is a
frontend compile error rather than a runtime surprise. Persistence is BoltDB
through vbolt (buckets, indexes, `vpack` serialization).

**Frontend** (`frontend/`) is TypeScript and Preact on
[vlens](https://www.npmjs.com/package/vlens), routed from `main.tsx` with
lazy-loaded pages. Styling is CSS-in-JS via vlens `block()` with CSS variables
for theming; `data-theme` on `<html>` switches light and dark.

**Build** is a Makefile over `go build` and esbuild. The release binary embeds
the built frontend, so a deploy is a single file.

Everything long-running happens in background workers with bounded queues
(`app.go`): photo processing, face analysis, push, and outbound mail. Face
recognition runs in a separate `family-face` daemon over a unix socket, because
it needs cgo and dlib and the main binary should not.

### Layout

| path | what's in it |
| --- | --- |
| `app.go` | wiring: `MakeApplication`, migrations, HTTP server, middleware chain |
| `backend/` | domain code, RPCs, workers, auth, rate limiting, security headers |
| `frontend/pages/` | one directory per feature area |
| `frontend/server.ts` | **generated** — do not edit; it is rewritten on every local run |
| `cfg/` | build-tag config: `local.go` by default, `release.go` under `-tags release` |
| `cmd/` | operator tools: `verifydb`, `restoredrill`, `faceanalysis` |
| `local/`, `release/` | the two entry points |
| `docs/` | [deployment](docs/deployment.md), [restore](docs/restore.md) |

## Prerequisites

- Go 1.24+
- Node 20+ (`npm ci` — the pinned prettier and tsc are what CI runs)
- A C toolchain. `CGO_ENABLED=1`; the release build links cgo dependencies.
- dlib, **only** to build the optional `family-face` daemon. Face tagging is
  compiled out of local builds (`cfg.EnableFaceTagging = false`), so day-to-day
  development does not need it.

## Setup

```bash
git clone https://github.com/sggrissom/Family-Portal
cd Family-Portal
npm ci
cp .env.example .env      # then fill in what you need — see below
make local
```

That serves <http://family.localhost:8666> with the frontend rebuilding on
change. Local builds write everything under `.serve/` in the working directory:
`.serve/db.bolt`, `.serve/static/` for photos, `.serve/frontend/` for the built
assets. Deleting `.serve/` is how you reset.

**A local build starts with an empty `.env`.** It logs the settings that would
fail a release build and keeps going, generating a throwaway JWT secret at each
boot. Fill in only what you are working on: `GOOGLE_CLIENT_ID` and
`GOOGLE_CLIENT_SECRET` for Google sign-in, `GEMINI_API_KEY` for AI import,
`MAIL_FROM` (or `EMAIL` and `APP_PASSWORD`) for password-reset mail. Every
variable is documented in [`.env.example`](.env.example).

## Common commands

```bash
make local        # dev server on http://family.localhost:8666
make check        # everything CI gates on: test + typecheck + lint
make test         # go test ./backend/ -v
make test-race    # race detector
make typecheck    # tsc --noEmit
make lint         # go vet, gofmt, prettier --check, CSS block validation
make format       # go fmt + prettier --write
make build        # release frontend + linux/amd64 binary into build/
make deploy       # build, then ship it to the VPS
```

`make check` before pushing. CI runs the same targets plus a coverage report and
a guard that fails when any check rewrites a tracked file.

### Operator tools

```bash
go run ./cmd/verifydb -db <copy>.bolt [-static <dir>]   # row counts, sanity checks
go run ./cmd/restoredrill -replay-migrations <paths...> # boot the app against a restored tree
```

Both work on a *copy* of the database — bolt takes an exclusive lock, so neither
can read the live file while the service is running.

## Deployment

`main` deploys itself: CI builds, tests, and ships to the VPS on every push that
passes. See [docs/deployment.md](docs/deployment.md) for the server topology, and
[docs/restore.md](docs/restore.md) for getting the data back when something eats
it.
