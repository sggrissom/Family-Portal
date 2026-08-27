# Family Portal

A private, self-hosted family portal: growth measurements, milestones, photos,
activities, and family chat, scoped to a household

It is a single Go binary that serves its own frontend and stores everything in
one BoltDB file plus a directory of images. There is no external database, no
object store, and no third-party service the app cannot start without.

Production lives at [familyrecord.app](https://familyrecord.app). An iOS
companion app has its own repository

## What it does

- **People** — one record per family member, with birth dates and relationships.
- **Growth** — height and weight over time, charted against WHO/CDC percentiles,
  with side-by-side comparison between siblings.
- **Milestones** — dated events with search.
- **Photos** — upload, automatic resizing to responsive variants, EXIF-derived
  dates, tagging, and face recognition that suggests who is
  in a picture.
- **Activities** — seasons, competitions, and routines with per-event results.
- **Chat** — real-time family chat over WebSocket.
- **Import / export** — bulk import from JSON and a full family export.

## Architecture

```
browser ──HTTP/WS──> family_site (Go)
                       ├── vbeam RPC ──> BoltDB (single file)
                       ├── static/    ──> photo originals + derived variants
                       └── unix sock  ──> family-face (dlib embeddings, release only)
```

**Backend** (`backend/`) is Go on [vbeam](https://go.hasen.dev/vbeam). Handlers
are registered with `RegisterProc()`, and vbeam generates the matching
TypeScript client into `frontend/server.ts` at startup in local builds — so a Go
struct and its TypeScript type cannot drift. Storage is BoltDB through
[vbolt](https://go.hasen.dev/vbolt): typed buckets, secondary indexes, and
versioned `vpack` serialization.

**Frontend** (`frontend/`) is TypeScript and Preact on
[vlens](https://www.npmjs.com/package/vlens). `main.tsx` declares every route
with a lazy `import()`, so each page is its own chunk. Styling is CSS-in-JS via
vlens `block()` with CSS custom properties for light/dark theming.

**Config** (`cfg/`) is selected by build tag: `local.go` by default,
`release.go` behind `//go:build release`. Storage paths, the site URL, and
whether face tagging exists at all are compile-time constants, which is why a
release binary cannot be pointed at a development directory by accident.

**Background work** runs in in-process worker queues started from
`MakeApplication()`: photo processing, face analysis, push notifications, and
outbound mail. Each is a queue with a bounded buffer; none of them can take a
user-facing request down with them.

Three entry points build on the same application:

| entry point | build tag | what it is |
| --- | --- | --- |
| `local/local.go` | *(none)* | dev server with esbuild watch and TS binding generation |
| `release/release.go` | `release` | production binary with the frontend embedded |
| `cmd/faceanalysis/` | `faceanalysis` | separate dlib daemon, talks over a unix socket |

Supporting commands live in `cmd/`: `smokecheck` (post-deploy verification),
`e2e` (five core flows against a compiled release build), `verifydb` and
`restoredrill` (backup verification).

## Prerequisites

- **Go 1.24+**
- **Node 20+** (frontend build and Prettier)
- **CGO enabled** with a C toolchain — the release build links BoltDB and image
  codecs with `CGO_ENABLED=1`.
- **dlib** — only to build `cmd/faceanalysis`. Not needed for the app itself;
  local builds use a no-op stub (`backend/photo_analysis_worker_stub.go`).

## Setup

```bash
git clone git@github.com:sggrissom/Family-Portal.git
cd Family-Portal
npm ci
cp .env.example .env    # fill in what you need; see below
make local
```

The dev server listens on <http://localhost:8666>. It creates
`.serve/db.bolt` and `.serve/static/` on first run, so there is nothing to
provision.

Local builds are deliberately forgiving: every required variable is logged as
missing and startup continues, and a throwaway JWT signing key is generated if
`JWT_SECRET_KEY` is unset. Only the features you actually configure will work —
Google sign-in needs `GOOGLE_CLIENT_ID`/`GOOGLE_CLIENT_SECRET`, password reset
needs mail. A **release** build refuses to
start when any required variable is missing (`backend/config_check.go`).

`.env.example` documents every variable and what breaks without it.

## Common commands

| command | what it does |
| --- | --- |
| `make local` | dev server with frontend watch and TS binding regeneration |
| `make build` | frontend bundle + release binary into `build/` |
| `make test` | Go unit tests, verbose |
| `make test-race` | the same under the race detector |
| `make test-coverage` | coverage profile and HTML report in `build/` |
| `make typecheck` | `tsc --noEmit` |
| `make lint` | `go vet`, `gofmt`, Prettier, CSS block validation |
| `make format` | `go fmt` + Prettier, in place |
| `make check` | test + typecheck + lint |
| `make e2e` | five core flows against the compiled release binary over TLS |
| `make smoke` | read-only checks against a running deployment |
| `make deploy` | build and ship to the VPS |

Run `npm ci` before `make lint` or `make typecheck`. Without `node_modules`,
`npx` fetches the latest Prettier and TypeScript instead of the pinned versions,
and local results stop matching CI.

## Testing

`make test` covers the backend. Tests use `backend/testing_helpers.go` to open a
temporary BoltDB and call procedures directly, so they exercise real storage
rather than mocks. Note that `vbolt.TxCommit` closes the transaction — build a
response before committing, and use one procedure call per `WithWriteTx`.

`make e2e` is the higher-confidence check: it starts the actual release binary
behind a TLS reverse proxy and drives signup, adding a person, adding growth,
uploading a photo, and chat, then sends `SIGTERM` and requires a clean drain. It
needs the production directory tree and refuses to run where a real deployment
lives.

CI runs formatting, `go vet`, backend tests, TypeScript checks, CSS validation,
a release build, the race detector, coverage, `make e2e`, and a guard that fails
if any check modified a tracked file.

## Deployment and operations

- [`docs/deployment.md`](docs/deployment.md) — production topology: Caddy, TLS,
  systemd units, paths and ownership, the face daemon.
- [`docs/restore.md`](docs/restore.md) — backups and how to actually restore
  one, written to be followed on a fresh box.
- [`docs/mobile-api.md`](docs/mobile-api.md) — the contract the iOS companion
  app is built against: what a shipped build may rely on, and what changing it
  would break.
- [`docs/permissions.md`](docs/permissions.md) — who can see what: household
  membership, and the family links that share one person with another household.

Pushing to `main` builds and deploys to the VPS, then runs the smoke check.

## Repository layout

```
app.go              application wiring: config check, DB, procs, workers, middleware
backend/            domain logic, RPC handlers, storage, workers
cfg/                build-tag configuration
cmd/                smokecheck, e2e, faceanalysis, verifydb, restoredrill
docs/               runbooks and the mobile API contract
frontend/           Preact/vlens SPA; server.ts is generated — do not edit
local/              development server entry point
release/            production server entry point and embedded dist
scripts/            build-time checks
plan.md             the 1.0 release plan
```
