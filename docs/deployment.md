# Production topology

What runs where, as which user, and which knob lives in which repo. Companion
to [restore.md](restore.md), which covers getting the data back; this covers the
box the data sits on.

The VPS is managed by [`tiny-server-helper`](https://github.com/sggrissom/tiny-server-helper).
Systemd units, the Caddy site config, the `deploy` script, and `backupctl` all
come from there, not from this repo. When something below looks like it should
be configurable and isn't, that repo is usually why.

## The request path

```
internet → Caddy (:443, TLS) → 127.0.0.1:$PORT → family_site → BoltDB
                                                    └─ unix /run/family-face/face.sock → family-face
```

Caddy terminates TLS, provisions and renews the certificate from Let's Encrypt
on its own, and reverse-proxies to the app on loopback. The site config that
`appctl domain family <domain>` generates is three lines of `reverse_proxy` plus
a rolling JSON access log at `/var/log/caddy/access-family.log` — **no body-size
limit, no proxy timeouts, no added headers.** Everything protective is in the
application, which matters in both directions: the app's limits are the only
ones, and nothing upstream will truncate a large photo upload before the app
decides about it.

Because Caddy connects over loopback, every production request arrives from a
trusted peer, which is what lets the rate limiter read `X-Forwarded-For` at all
(`backend/rate_limit.go:338`). It takes the *rightmost* entry — the one Caddy
appended — so a client-supplied header cannot mint fresh limit budgets. If the
proxy ever moves off-box, that trust check is the thing to revisit.

## What was actually measured

Checked against production on 20 August 2026. Caddy is 2.6.2 and the effective
config was read back from its admin API (`localhost:2019/config/apps/http/servers`),
not from the file, so what follows is what is running rather than what was
written.

| question | answer |
| --- | --- |
| Does the proxy cap request bodies? | No. A 2.7 MB `POST /rpc/GetAuthContext` came back `413` carrying `X-Request-Id` — the **app's** header, so the body reached the app and the app rejected it. The limits in `backend/security.go` are the only ones. |
| Does the proxy impose timeouts shorter than the app's? | No. The server object has no `read_timeout`, `write_timeout`, or `idle_timeout` key at all, so nothing upstream can cut short the 30-minute import read or the 30-minute download write in `backend/request_timeouts.go`. Caddy's default idle timeout applies between requests on a keep-alive connection, not during a streaming body. |
| Does WebSocket proxying work? | Yes, and it is checked every deploy rather than once: `cmd/smokecheck` dials `/ws/chat` with a real session, sends a heartbeat, and waits for the reply. Caddy 2 passes upgrades through `reverse_proxy` without configuration. |
| Is TLS renewing? | Yes. Let's Encrypt, issued 28 July 2026, expiring 26 October 2026, HTTP/2 negotiated. There is no `tls` automation block, so Caddy's defaults apply and it renews at roughly two-thirds of the lifetime — about 27 September. |

### `www.familyrecord.app` does not work

`www` has an A record pointing at this server, but Caddy has no site block for
it. HTTP gets Caddy's automatic 308 to `https://www.familyrecord.app/`, and the
HTTPS handshake then fails outright:

```
$ curl https://www.familyrecord.app/
TLS connect error: error:0A000438:SSL routines::tlsv1 alert internal error
```

Caddy will not obtain a certificate for a name no site block claims, so anyone
who types the `www.` form reaches a browser security warning rather than the
site. The apex is fine and is what every link, the canonical URL, and the OG
tags use, so nothing inside the app is affected — but a name that resolves to
this box and fails TLS is worse than one that does not resolve at all.

The fix belongs in `tiny-server-helper`, not here: either drop the DNS record,
or have `appctl domain` emit a redirect block for `www` so Caddy claims the name
and can get a certificate for it.

```
www.familyrecord.app {
    redir https://familyrecord.app{uri} permanent
}
```

## Services

| unit | binary | what it is |
| --- | --- | --- |
| `app@family` | `/srv/apps/family/current/family` | the web app |
| `internal@family-face` | `/srv/apps/family-face/current/family-face` | dlib face embeddings, loopback-only |

Both are template units from `tiny-server-helper/systemd/`. Both run as
`apps:apps`, with `WorkingDirectory` at the release symlink and
`EnvironmentFile` at `shared/.env`. `internal@` additionally gets
`RuntimeDirectory=%i`, which is what creates `/run/family-face` for the socket.

## Paths and ownership

Everything under `shared/` is `apps:apps`; the service cannot open a database it
cannot write.

| path | what |
| --- | --- |
| `/srv/apps/family/releases/<ts>_<sha>/` | one directory per deploy, last 5 kept |
| `/srv/apps/family/current` | symlink to the active release |
| `/srv/apps/family/shared/.env` | secrets, mode 600, **not backed up** |
| `/srv/apps/family/shared/data/db.bolt` | the database (`cfg.DBPath`) |
| `/srv/apps/family/shared/static/` | uploads and derived variants (`cfg.StaticDir`) |
| `/srv/apps/family-face/shared/` | the face daemon's own env and models |
| `/run/family-face/face.sock` | app → daemon socket (`cfg.FaceAnalysisSocket`) |

Storage paths are compile-time constants in `cfg/release.go`, not environment
variables. Startup probes both for writability and a release build refuses to
serve if either fails (`backend/config_check.go`).

## Deploys

CI deploys `main` after the full check gate passes (`.github/workflows/test.yml`).
`make deploy` builds the frontend and a CGO Linux binary, then hands it to the
`deploy` script, which uploads to a new timestamped release directory, repoints
`current`, restarts the unit, and **rolls back to the previous release if
`/healthz` does not answer**. The face daemon is deployed separately and rarely:
`make deploy-face-remote` builds it on the VPS, because it needs dlib present at
build time.

## End-to-end check before deploy

`make e2e` runs `cmd/e2e`, which starts the compiled release binary on a
scratch deployment and drives the flows a release cannot ship broken: readiness,
the landing page and its bundle, signup, login, adding a person, recording a
growth measurement, uploading a photo and serving it back, chat over the
WebSocket, and logout. Then it sends `SIGTERM` and insists the process drains
and exits zero.

The unit tests cannot answer any of this. They call handlers in-process against
a local build; what ships is a release-tagged binary with the frontend embedded
in it, compile-time storage paths, a config check that refuses to serve, rate
limiting that cannot be switched off, a face analysis worker that really does
look for its daemon, and `Secure` cookies no plaintext client can hold. So the
run puts a TLS reverse proxy in front, the way Caddy sits in front of
production, and talks to the binary as a browser would.

Because a release build resolves its storage paths at compile time
(`cfg/release.go`), the scratch deployment has to live where production's does:

```
sudo mkdir -p /srv/apps/family/shared/data /srv/apps/family/shared/static
sudo chown -R "$(id -un)" /srv/apps/family/shared
```

On a machine where that tree would be unwelcome, `bwrap` can supply a throwaway
one without `sudo` and without leaving anything behind:

```
bwrap --dev-bind / / --tmpfs /srv \
  --dir /srv/apps/family/shared/data --dir /srv/apps/family/shared/static \
  --chdir "$PWD" -- make e2e
```

Sharing production's paths is also why the run refuses to start when the tree
already holds a database, a `shared/.env`, or anything at all under `static/` —
on a real box those mean a real deployment, and the run would be pointed at
somebody's photos. A refusal deletes nothing; a run that did start removes what
it created. **Do not run it on the VPS.** CI runs it on a throwaway runner,
after the race detector.

## Post-deploy smoke check

`make smoke` runs `cmd/smokecheck` against a deployment and exits nonzero if
anything a visitor would notice is broken. The deploy script's own gate only
asks whether the process answers `/healthz`, which a release that serves last
build's frontend, cannot read its database, or refuses every login still
passes. This asks the six questions that come after that:

| check | what a failure means |
| --- | --- |
| `/readyz` | the database is unreadable or `shared/static/` is not writable |
| landing page | `index.html` is missing, or names a bundle the release does not contain |
| login | auth is broken, or `JWT_SECRET_KEY` changed under a running session |
| photo | the RPC layer, the database, or `shared/static/` is not serving files |
| websocket | `/ws/chat` upgrade, hub registration, or `SITE_ROOT`'s origin list is wrong |
| logout | the session was not revoked |

Every check is read-only. The one piece of state it creates is a login
session, which the last check disposes of.

It needs a real account, given as `SMOKE_EMAIL` and `SMOKE_PASSWORD`
(`-email` and `-password` also work), in a family holding **at least one
processed photo** — without one, "a photo loads" has nothing to answer. Make it
a dedicated account rather than a person's, so a failure here is never
confused with somebody's own session, and so rotating its password costs
nobody anything.

CI runs it as the last step of the deploy job, from the same two repository
secrets. When they are unset the step logs a warning and passes, so an
un-armed check is visible on every deploy rather than silent.

## Face analysis

The daemon serves `/recognize` and `/embed` over the Unix socket, plus `/healthz`
over TCP when `PORT` is set (the deploy script's health check needs the latter).
It takes no flags from the unit file, so both paths come from its `.env`:

- `FACE_MODELS` — dlib model directory, required, no default. Must contain
  `shape_predictor_5_face_landmarks.dat` and
  `dlib_face_recognition_resnet_model_v1.dat`. Models are **not** in the repo,
  not in a deploy, and not in a backup; a rebuilt box needs them fetched again.
- `FACE_SOCKET` — must be set to `/run/family-face/face.sock`. The compiled-in
  default is `/run/family/face.sock`, which is a directory systemd never creates
  and the app never dials.

Face analysis is optional, and degrades quietly: if the socket is missing,
photos still upload, process, and serve. But the reachability check runs **once,
at app startup** (`backend/photo_analysis_worker.go:71`) — if the daemon is down
when `app@family` starts, the worker is never created and stays off until the
app is restarted, however healthy the daemon becomes later. Restart `family` after
`family-face`, not before.
