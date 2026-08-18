# Production topology

What is running on the VPS, where its files are, and who owns them. Written so a
fresh box can be rebuilt from it, and so the [restore
procedure](restore.md) has something to restore *into*.

The server is managed by
[tiny-server-helper](https://github.com/sggrissom/tiny-server-helper): a bash
toolkit that owns the systemd unit templates, the Caddy vhosts, Postfix/OpenDKIM,
and restic backups. Nothing in this repo configures the server. What this repo
owns is the binary, and the contract it expects the box to satisfy.

## The pieces

| unit | what it is | runs as |
| --- | --- | --- |
| `caddy` | TLS termination and reverse proxy for every site on the box | `caddy` |
| `app@family` | the site: `family_site`, one process, listening on `:8666` | `apps` |
| `internal@family-face` | dlib face-recognition daemon on a unix socket, no vhost | `apps` |
| `postfix` + `opendkim` | outbound mail on `127.0.0.1:25`, DKIM-signed per domain | system |
| `backupctl.timer` | nightly restic backup at 03:00 (±30m jitter) | `root` |
| `backupctl-check.timer` | mails if the newest backup is older than 48h | `root` |
| `backupctl-forget.timer` | weekly retention and prune, Sunday 04:30 | `root` |

## Paths

```
/srv/apps/family/
├── current -> releases/<timestamp>_<sha>    # symlink flipped atomically on deploy
├── releases/                                # last 5 kept
├── shared/.env                              # PORT + every secret; mode 600, apps:apps
├── shared/backup.conf                       # what backupctl snapshots and includes
├── shared/data/db.bolt                      # the database
├── shared/static/photos/                    # originals + derived variants
└── shared/models/                           # dlib model files

/srv/apps/family-face/                       # same layout; the face daemon
/etc/caddy/sites/family.caddy                # vhost, written by `appctl domain`
/etc/tiny-server-helper/backup.env           # restic repo URL + password, root, 600
/var/log/caddy/access-family.log             # 50 MB × 3 rolled
```

Everything under `shared/` is `apps:apps`. The service runs as `apps` and will
fail to open a database it cannot write. Release binaries are immutable — a
deploy never writes into an existing release directory, it makes a new one — so
`shared/` is the only mutable state, and therefore the only thing worth backing
up.

The paths above are **compile-time constants**, not environment variables
(`cfg/release.go`). They cannot be moved without rebuilding.

## Ports and the reverse proxy

The binary listens on `:8666`, a constant in `release/release.go`. `PORT` in
`shared/.env` does not change that — it is read by the deploy tool's health check
and by `appctl domain` when it writes the Caddy vhost, so **`PORT` must be 8666**
or the two disagree and every deploy fails its health check.

The vhost is deliberately plain:

```
familyrecord.app {
    reverse_proxy localhost:8666
    log {
        output file /var/log/caddy/access-family.log {
            roll_size 50mb
            roll_keep 3
        }
        format json
    }
}
```

Three things follow from it being plain, and all three are what this application
wants:

- **No proxy-level body limit.** Caddy does not cap request bodies by default,
  so the application's own limits are the only ones in play: 1 MiB for JSON,
  52 MiB for a photo upload, 512 MiB for a full-family import
  (`backend/security.go`). A proxy limit below those would reject an upload with
  a 413 the app never sees and cannot explain.
- **No proxy-level read/write timeout.** Caddy's are unset by default, which is
  what lets a slow upload and a long-lived chat WebSocket both finish. The
  application sets a 10s read-header timeout and a 2m idle timeout instead
  (`app.go`) — bounds on the parts that can be bounded without breaking the
  parts that cannot.
- **WebSocket upgrades pass through untouched.** `reverse_proxy` handles
  `Upgrade` natively; there is no extra directive to get wrong.

TLS is automatic: Caddy provisions and renews certificates from Let's Encrypt on
its own, and reloads without dropping connections.

Caddy proxies from loopback, so every request arrives at the application from
`127.0.0.1`. Rate limiting knows this: it trusts `X-Forwarded-For` only when the
direct peer is loopback, private, or link-local, and reads only that header's
rightmost entry — the one Caddy appended itself (`backend/rate_limit.go`).

## Deploying

Pushing to `main` is the deploy. CI (`.github/workflows/test.yml`) runs the full
gate — build, lint, typecheck, tests, race detector, coverage, and a check that
nothing rewrote a tracked file — and only then runs `make deploy`, which:

1. uploads the binary to a new timestamped release directory,
2. flips `current`,
3. restarts `app@family`,
4. health-checks `http://127.0.0.1:8666/healthz`,
5. rolls back to the previous release if that fails,
6. prunes to the last 5 releases.

There is no staging environment and no approval gate. Rollback is
`sudo appctl rollback family`.

`make deploy` from a laptop does the same thing, and is the path for deploying
something that is not on `main`.

### The face daemon is deployed separately

`family-face` needs cgo and dlib on the build machine, which CI does not have, so
it is **not** part of the automatic deploy. It changes rarely; deploy it by hand
when it does:

```bash
make deploy-face-remote     # builds on the VPS, where dlib is installed
```

It is an `internal@` unit: no Caddy vhost, no public port — but the deploy tool
health-checks `internal@` services the same way it does public ones, so
`/srv/apps/family-face/shared/.env` needs a `PORT`, and the daemon serves
`/healthz` on it. systemd creates
`/run/family-face` for it (`RuntimeDirectory=%i`), and `FACE_SOCKET` in
`/srv/apps/family-face/shared/.env` must name the socket the main app expects —
`/run/family-face/face.sock`, a constant in `cfg/release.go`. The daemon's own
default is a different path, so leaving `FACE_SOCKET` unset silently breaks face
tagging and nothing else.

`FACE_MODELS` points at the dlib models, which are **not** in git and not in the
backup: `shape_predictor_5_face_landmarks.dat` and
`dlib_face_recognition_resnet_model_v1.dat`, from dlib's model releases, in
`/srv/apps/family/shared/models`.

## Startup refuses to serve a broken configuration

A release build checks its whole environment before it opens a socket
(`backend/config_check.go`) and calls `log.Fatalf` with the complete list of
problems if anything is wrong: `SITE_ROOT` present, https, origin-only;
`JWT_SECRET_KEY` and `BACKUP_TOKEN` at least 32 characters; Google OAuth
credentials present; a Gemini key present; a usable outbound mail path; the data
and static directories present and *actually writable by this process*, tested by
creating a file rather than by reading mode bits. APNs is all-or-nothing —
entirely unset is fine, partially set is fatal.

The failure mode this replaces is a deploy that comes up green and then breaks in
one corner of the app hours later, for one user, in a way nobody connects to the
deploy. `systemctl status app@family` and `appctl logs family` show the list.

## Health endpoints

| endpoint | answers | used by |
| --- | --- | --- |
| `/healthz` | the process is up | deploy health check, monitor |
| `/readyz` | the database opens and the static directory accepts a file | post-deploy check, restore verification |
| `/internal/snapshot` | streams a consistent copy of the database | `backupctl`, bearer `BACKUP_TOKEN` |

`/readyz` is the interesting one: it is the pair of things a bad restore or a
permissions mistake breaks, and it is checked by doing them, not by asserting
them.

## Backups

`shared/backup.conf` registers the app with `backupctl`. It declares two things:
the snapshot URL to fetch the database through, and the photo originals to
include. Derived variants are excluded on purpose — they are regenerable from the
originals, and backing them up would multiply the archive for no recovery value.

Restic encrypts with a password in `/etc/tiny-server-helper/backup.env`. **Lose
that password and every archive is permanently unreadable**; it needs one copy on
the box and one copy off it that is not the backup target.

Retention is 7 daily and 4 weekly, plus an unconditional keep-last-1 so an app
whose backups broke months ago never has its final surviving archive pruned.
`backupctl-check.timer` mails if the newest successful backup is older than 48
hours — including an app that has never had one.

Restoring is [docs/restore.md](restore.md), which has been rehearsed end to end,
not just written down.

## Rebuilding the box from nothing

1. `sudo ./bin/bootstrap-vps` from tiny-server-helper — creates the `apps` user,
   `/srv/apps/`, the systemd templates, and points Caddy at `/etc/caddy/sites/`.
2. `sudo appctl init family 8666` and `sudo appctl init family-face 8667`.
3. Write `/srv/apps/family/shared/.env` from [`.env.example`](../.env.example),
   mode 600, owned by `apps`. **None of these values are in the backup** — §3 of
   [restore.md](restore.md) lists what breaks without each one.
4. `sudo appctl domain family familyrecord.app` — vhost and TLS.
5. `sudo mailctl setup`, `add`, and `verify` for the domain, so password reset
   and backup alerts can send.
6. `sudo backupctl setup <repo>`, then write `shared/backup.conf`.
7. Put the dlib models in `shared/models/`.
8. Deploy: push to `main`, then `make deploy-face-remote` for the daemon.
9. Restore the data ([restore.md](restore.md)), then confirm `/readyz` and a real
   login.
