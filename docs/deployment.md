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

### `www.familyrecord.app` was failing TLS — fixed 20 August 2026

`www` has an A record pointing at this server, but Caddy had no site block for
it, and Caddy will not obtain a certificate for a name no site block claims. So
HTTP got Caddy's automatic 308 to `https://www.familyrecord.app/` and the HTTPS
handshake then died:

```
$ curl https://www.familyrecord.app/
TLS connect error: error:0A000438:SSL routines::tlsv1 alert internal error
```

Anyone who typed the `www.` form reached a browser security warning. The apex
was unaffected, and nothing in the app references `www` — links, the canonical
URL, the OG tags, and the manifest are all apex — so this was invisible from
inside the application and only findable from outside it.

`/etc/caddy/sites/family.caddy` now carries a second block, added by hand:

```
www.familyrecord.app {
    redir https://familyrecord.app{uri} permanent
}
```

Caddy issued a certificate for the name within seconds of the reload, and
`http://www.familyrecord.app/anything` now lands on
`https://familyrecord.app/anything` with the path intact.

**This edit is not reproducible from `tiny-server-helper`.** `appctl domain`
generates the apex block only, so the next run of it over this app would drop
the `www` block again. The durable fix is to teach that generator about `www`;
until then, treat `family.caddy` as hand-edited (there is a
`family.caddy.bak.20260820` next to it holding the pre-edit version).

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
| `/srv/apps/family/shared/logs/` | the rotating application log (`cfg.LogDir`) |
| `/srv/apps/family-face/shared/` | the face daemon's own env and models |
| `/run/family-face/face.sock` | app → daemon socket (`cfg.FaceAnalysisSocket`) |

Storage paths are compile-time constants in `cfg/release.go`, not environment
variables. Startup probes all three for writability and a release build refuses
to serve if any fails (`backend/config_check.go`).

Logs live under `shared/` deliberately. The unit sets
`WorkingDirectory=/srv/apps/%i/current`, so a relative log path put the file
*inside the release directory*: every deploy started an empty log and the sixth
deploy after an incident pruned the evidence, which is the wrong window — the
moment you most want logs is right after a deploy that broke something.

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
sudo mkdir -p /srv/apps/family/shared/data /srv/apps/family/shared/static \
  /srv/apps/family/shared/logs
sudo chown -R "$(id -un)" /srv/apps/family/shared
```

On a machine where that tree would be unwelcome, `bwrap` can supply a throwaway
one without `sudo` and without leaving anything behind:

```
bwrap --dev-bind / / --tmpfs /srv \
  --dir /srv/apps/family/shared/data --dir /srv/apps/family/shared/static \
  --dir /srv/apps/family/shared/logs \
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

## Host metrics

`/admin` shows a Host card and folds disk pressure and proxy-measured 5xx into
its problems feed, both read from
[`metrics-server`](https://github.com/sggrissom/tiny-server-helper), which is
already deployed on this box as an `internal@` unit. The traffic block is the
interesting half: it comes from Caddy's access log, so it answers "is the site
erroring for people" independently of anything this application logs about
itself.

Two settings in `shared/.env`, both or neither:

| variable | value |
| --- | --- |
| `METRICS_URL` | `http://127.0.0.1:<PORT>/metrics`, where `PORT` is the one in `/srv/apps/metrics-server/shared/.env` |
| `METRICS_API_KEY` | the `API_KEY` from that same file |

Point it at **loopback**, not `metrics.grissom.zone`: the service runs on this
box and binds `127.0.0.1`, so there is no reason for the request to leave the
machine or depend on public DNS and TLS. A non-loopback host is a startup
failure, as is setting one variable without the other — a URL with no key gets
a 401 the panel would degrade quietly past, which is the kind of
half-configuration `checkAPNs` already exists to prevent.

Unset is a legitimate state: the card is hidden and the feed simply has less to
say. A metrics service that is down never takes the panel with it — the fetch
has a three-second timeout, the result is cached for 30 seconds either way, and
a failure renders as one line naming the reason.

### Backup age and deploy history

The same fetch carries two more blocks per app, and both need a `metrics-server`
built from a checkout that has them:

- **`releases`** — the last five release directories, newest first, with the
  short SHA and the time the directory was created *on the box*. Not the
  timestamp in the release name: `bin/deploy` builds that from the deploying
  machine's clock, in whatever zone that machine happens to be in. Rendered as
  the deploy strip under the diagnostics row, so "this started after Tuesday's
  deploy" is visible rather than reconstructed.
- **`backups`** — whether `/srv/apps/family/shared/backup.conf` exists, and,
  from the status file `backupctl` publishes, when the last run succeeded and
  how large the repository is. Three states reach the problems feed: not
  registered at all, registered but never once successful, and older than two
  nightly windows.

`backupctl` writes that status file to
`/var/lib/tiny-server-helper/status/backup-<app>.json`, mode 644, only after
restic reports success. Its own cache under
`/var/lib/tiny-server-helper/backup/` stays mode 700 — it stages plaintext
database snapshots — so the published file is what an unprivileged reader gets.
`metrics-server` runs as `apps` and reads it there. This application never
reads either: it only sees what the metrics fetch hands it, which is the whole
reason backup state is not this repo's problem.

An older `metrics-server` simply omits both blocks. The deploy strip disappears
and the backup line reads as never registered, so redeploy `metrics-server` in
the same pass as anything that depends on them.

## Storage limits

Two limits keep uploads from filling the disk the database sits on. Both are
compile-time constants in `cfg/`, alongside the storage paths, and both are
enforced in `backend/storage_quota.go`.

| constant | default | what it stops |
| --- | --- | --- |
| `FamilyStorageQuotaBytes` | 10 GiB | one account costing an unbounded amount. Signup is open and accounts are free, so without it the only limit is the upload rate rule. |
| `MinFreeDiskBytes` | 1 GiB | the machine running out. Per-family quotas multiply by the number of families, so they cannot promise this on their own. |

The quota counts photo **originals**, which is what the `Image` row records.
Derived variants roughly triple what actually lands on disk, so the quota is
deliberately set well under the disk rather than close to it — raising it means
checking the real `shared/static/` size first, not just the free space.

Both the upload handler and the bundle importer check. Going over on an import
stops the photo loop and returns a warning rather than failing the whole
import: the people and measurements in that transaction are worth keeping.

A family over quota gets a message naming its usage and `cfg.SupportEmail`. A
breach of the disk floor is a 503 — the uploader is not the one who can fix it,
and it is logged at error level so it reaches the health check below.

If `Statfs` fails, the write is allowed. Refusing every upload because the
filesystem could not be measured trades a rare problem for a total one.

## Health alerting

`backend/health_monitor.go` evaluates the same `collectSystemHealth` the `/admin`
panel uses, every 15 minutes, and emails the admin account when the verdict
flips from green to red. It re-sends at most once every 24 hours while a problem
persists, and sends once more on recovery. The first check after a restart only
establishes a baseline unless it is already broken, so a restart does not
generate an alert for a problem that predates the process.

The recipient is the email on user 1 (`AdminUserId`) — there is no separate
variable to configure or forget. Alerts go through the ordinary mail worker, so
unconfigured mail means no alerts.

**This does not tell you the site is down.** A process that has died cannot
report that it died, and neither can it send mail. `/healthz` and `/readyz` are
unauthenticated for exactly this reason; something outside the box has to poll
them. That belongs in `tiny-server-helper` or an external uptime service, not
here.

## Email confirmation

New accounts are created unverified and are sent a confirmation link that
expires in seven days. **Nothing is gated on it.** An unconfirmed account can do
everything a confirmed one can; the only difference is a dismissible banner. The
point is to catch a mistyped address while the owner is still around to fix it,
rather than years later when they need a password reset and cannot get one.

Google sign-in never asks. Google asserts `verified_email` (or `email_verified`
on the ID-token path), and re-proving an address the identity provider already
proved is a round trip for nothing. An existing unverified account is upgraded
the first time its owner signs in with Google.

Failing to send does not fail the signup. The account is still good, and
`ResendVerificationEmail` issues a fresh link — which also invalidates the
previous one, so only the newest link in an inbox works.

Existing accounts from before this shipped decode as unverified, which is
accurate: those addresses were never confirmed. They will see the banner until
they confirm.

## Client-side error reporting

`POST /api/client-error` takes a JSON report from the browser and writes it to
the ordinary log under the `CLIENT` category. Three things feed it: a Preact
error boundary wrapping every route, `window.onerror`, and `unhandledrejection`.
Without it a render crash is a blank page that nobody outside the affected
browser ever hears about.

It is **unauthenticated on purpose** — the errors most worth seeing are the ones
that break the page before anyone can sign in. What bounds it instead:

- the rate limiter (20 per 5 minutes per IP)
- per-field caps, and a body cap under `MaxBytesReader`
- a client-side cap of 5 reports per page load, de-duplicated by message

Reports are logged at **WARN**, not ERROR. `collectSystemHealth` counts ERROR
entries toward the health verdict, so logging them at error level would let one
person's broken browser extension turn the site red and mail the admin.

## The admin account

User 1 (`AdminUserId`) is the only account that can reach `/admin`, and it is
where health alerts are sent. `deleteAccountHandler` refuses to delete it: doing
so would lock everyone out of the admin panel and silently stop alerting, with
no way back short of editing the database.

## Sign in with Apple

Optional, and all-or-nothing: `APPLE_CLIENT_ID`, `APPLE_TEAM_ID`,
`APPLE_KEY_ID`, and `APPLE_KEY_PATH` in `shared/.env`, or none of them. Set some
and the app refuses to start (`checkAppleOAuth` in `backend/config_check.go`),
because a half-set credential is a login button that hands users to Apple and
fails on the way back. Unset, `/api/auth/providers` reports Apple off and the
login page draws no Apple button.

Two things are easy to get wrong:

- `APPLE_CLIENT_ID` is the **Services ID**, not the app's bundle ID, and its
  return URL must be registered with Apple as `$SITE_ROOT/api/apple/callback`
  exactly. Apple compares it verbatim.
- `APPLE_KEY_PATH` is a Sign in with Apple `.p8`, a **different key** from the
  APNs one under `shared/apns/`. Both download once and cannot be retrieved
  again. Keep it mode 600 alongside the APNs key; it is not backed up, so losing
  the host means generating a new key in the developer portal.

Apple issues no static client secret. The server signs a short-lived one per
code exchange from the team ID, key ID, and key, which is why all four values
have to be present and consistent.

The browser flow cannot be tested on a development machine at all: Apple
requires an https return URL and rejects `localhost`. The companion app's flow
(`POST /api/login/apple/token`, see `docs/mobile-api.md` §2.2) can be, since it
only needs `APPLE_IOS_CLIENT_ID` to match the bundle ID in the identity token.

A user who chooses "Hide My Email" arrives with a `@privaterelay.appleid.com`
address. That address is stable and the account works normally, but mail sent to
it is delivered only if the sending domain is registered with Apple's private
email relay service. Apple sign-in never triggers a verification mail — the
identity token already asserts the address — but password resets and health
digests to such an address will bounce until that registration is done.

## Universal links

`/.well-known/apple-app-site-association` is what makes a `familyrecord.app`
link — including the `destination` every push payload carries — open the
companion app instead of Safari. The app serves it; Caddy passes it through with
everything else.

It is off until `IOS_APP_ID` is set in `shared/.env` to the app's
`<TeamID>.<BundleID>`. Unset, the path 404s, which is the correct answer for a
server with no app in the field. Malformed, the app refuses to start — see
`checkIOSAppID` in `backend/config_check.go` — because Apple's CDN and every
device that installed the app cache this file, so an association naming the
wrong app is not something a redeploy takes back.

Three things about the serving path have to stay true, and none of them are
enforceable from inside the application:

- **No redirect.** Apple fetches the file over https and gives up on a 3xx.
  `https://familyrecord.app/.well-known/apple-app-site-association` must answer
  200 directly. The `www` block redirects, which is fine — Apple only ever asks
  the domain named in the app's entitlement.
- **`Content-Type: application/json`**, which the handler sets, and no signing.
  The signed `.pkcs7` form was dropped in iOS 10.
- **The ACME challenge lives under the same prefix.** Caddy answers
  `/.well-known/acme-challenge/*` itself during issuance; it does not claim the
  rest of `/.well-known/`. If a future site block ever routes that whole prefix
  to a static directory, the association file goes with it.

Verify after a deploy that sets it:

```bash
curl -sS -D- -o /dev/null https://familyrecord.app/.well-known/apple-app-site-association
curl -sS https://familyrecord.app/.well-known/apple-app-site-association | jq .
```

The path list itself is in `backend/universal_links.go`, next to the reasoning
for what is on it. Adding a path there without a screen in the app that opens it
produces a link that opens the app and then does nothing — worse than the
browser it took the link from.
