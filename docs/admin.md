# The admin section

What the admin panel is for, how it is put together, and the constraints that
shaped it. Companion to [deployment.md](deployment.md), which describes the box,
and [restore.md](restore.md), which describes getting the data back.

The bar here is not "complete admin console". There is exactly one operator, and
the panel exists for three things: **awareness, error investigation, and the
small set of actions worth having a button for.** Anything that does not serve
one of those is out of scope, and several things below were rejected on exactly
that ground.

---

## The pages

Seven routes, all gated on `user.Id == AdminUserId`.

| route | what it answers |
| --- | --- |
| `/admin` | is anything wrong — diagnostics strip, config status, problems feed, host card, deploy strip, weekly digest |
| `/admin/analytics` | four tabs: overview, users, content, system |
| `/admin/users` | every user; revoke sessions, resend a password reset |
| `/admin/photos` | processing and face-analysis stats, requeue stuck photos, disk consistency check |
| `/admin/logs` | file picker, reference lookup, cross-file search, level/category/time/duration filters, percentiles |
| `/admin/push` | APNs config, devices, delivery attempts, test send |
| `/admin/app-versions` | min/latest mobile build policy |

`/admin/push` is the model the rest follow. It answers the three questions you
actually have when something doesn't work — is it configured, is there anything
to send to, what did the last attempts say — it names the common failure
directly (`EnvironmentMismatch`), and it lets you reproduce the problem without
a second person.

Backend: `admin.go` (users and logs), `admin_actions.go`, `admin_health.go`,
`admin_push.go`, `admin_mail.go`, `admin_digest.go`, `analytics.go`,
`photo_maintenance.go`, `log_reader.go`, `host_metrics.go`, `diagnostics.go`.
Twenty-six procs. `RegisterAdminMethods` calls the per-area registrars rather
than listing procs itself — registering from `app.go` instead would reorder
every proc after it in the generated `server.ts` for no gain.

---

## Rules the panel is built on

These are the load-bearing ones. Breaking any of them is how the panel stops
being worth opening.

**A green page means something.** The problems feed and the config status are
silent when clean. That is the whole point — if they render noise on a healthy
day, nobody reads them on a bad one.

**Never invent a number.** Two metrics used to be fabricated: an "average
process time" that was an arithmetic expression over file metadata, and an error
analysis that returned empty slices with `TotalErrors` set to the count of
failed photos. A dashboard that invents plausible numbers is worse than one with
a gap in it, because you stop being able to tell which numbers to trust. If a
figure isn't measured, it doesn't ship.

**No button that has never worked.** Four permanently-disabled buttons and a
"Coming Soon" card were removed. A dead control teaches you to ignore the row
it's in.

**Statistics come from the whole file, not a sample.** `GetLogStats` used to
derive its histograms and its p50/p90/p95/p99 from the last 50 lines — under a
minute of traffic, presented as a performance summary. Percentiles are computed
over the whole file now. This is an admin page loaded by one person; it can
afford the read.

**Long work queues, it does not block.** BoltDB allows one writer.
`ReprocessAllPhotos` originally decoded and re-encoded every unprocessed photo
at seven sizes *inside an open write transaction*, blocking every upload, chat
message and milestone save in the application for the duration — and the
two-minute RPC write timeout (`request_timeouts.go`) would sever the response
long before it finished, leaving rows stuck in `Status = 1` with nothing to move
them. Admin actions queue into the relevant worker and return a count
immediately. Queue depth is already on the diagnostics strip, so progress comes
for free.

**Optional subsystems are all-or-nothing, and degrade quietly.** `checkAPNs` set
the pattern and host metrics follows it: neither variable set is a legitimate
state that hides the card; one of the two set is a startup failure. A URL with
no key would get a 401 the panel would degrade quietly past, which is the
half-configuration worth refusing at boot. But a subsystem that is *down* must
never take the panel with it — the metrics fetch has a three-second timeout, is
cached 30 seconds either way, and renders a failure as one line naming the
reason.

---

## Logs

Error investigation is the stated purpose, and the logs are the only place the
answers live.

**They live in `shared/`, not in the release directory.** `app@.service` sets
`WorkingDirectory=/srv/apps/%i/current` and lumberjack writes relative to it, so
logs used to live *inside* the release directory — which `deploy` recreates per
deploy and prunes to the last five. Every deploy started an empty log, and the
sixth deploy after an incident deleted the evidence. That is exactly the wrong
window: the moment you most want logs is right after a deploy that broke
something, and the deploy that broke it was the one that reset them. `cfg.LogDir`
now points at `/srv/apps/family/shared/logs` in release builds, and
`checkStoragePaths` probes it for writability at startup.

**Reference codes are the intended workflow.** `ProcError` mints a request id,
logs the real cause against it, and returns `"Something went wrong on our end.
Reference: <id>"`. Someone sends you the code; you find the cause. The log
viewer has a dedicated lookup mode for this (`LookupLogReference`) that returns
the one entry plus surrounding context, and search runs across all log files
rather than one.

**Defaults assume something is wrong now.** Newest-first, with an "errors in the
last 24 hours" preset one click away.

One scanner, in `log_reader.go` — ANSI stripping, the fallback parse strategies,
the timing-line regex and stack-trace continuation. The line-accumulation loop
used to be written twice, differing only in whether it paginated or
ring-buffered.

---

## Host metrics, backups, and releases

The panel used to stop at the process boundary: it knew its own uptime, its own
queues and its own log file, and nothing about the disk it was filling, the
traffic Caddy was seeing, or whether last night's backup ran. All of that is
computed by [`tiny-server-helper`](https://github.com/sggrissom/tiny-server-helper).

`GetHostMetrics` fetches one JSON document from `metrics-server` and gets three
things from it. Configuration is two variables in `shared/.env`, both or neither
— see [deployment.md](deployment.md) for the values.

**Host and traffic.** Load, memory, CPU including iowait, disk usage on the
20 GB shared disk, per-app disk, and per-app traffic from Caddy's JSON access
log over a 15-minute window. The traffic block is the interesting half: it is
measured **at the proxy**, so it answers "is the site erroring for people"
independently of anything the application logs about itself. Disk pressure and
5xx feed the problems feed.

**Backups.** The app does **not** read `backupctl`'s state directly, and must
not learn how. `STATE_DIR` is mode **700**, not merely root-owned, because
`stage/` inside it holds a plaintext database snapshot while a run is in flight
— a process running as `apps` cannot traverse into it at all, and relaxing the
mode would trade a backup timestamp for a readable database. So `backupctl` has
a `publish_status` step writing `/var/lib/tiny-server-helper/status/backup-<app>.json`
at mode 644, from the one place `<app>.last` is written — after restic reports
success and nowhere else, so the two can never disagree. `metrics-server` reads
that file; the panel gets it through the fetch it was already doing. Three
states reach the problems feed: not registered, registered but never once
successful, and older than two nightly windows.

Note the ordering trap: the status file is only created by a backup run that
happens *after* `publish_status` was installed. Between the install and the next
nightly run, the panel legitimately reports "never succeeded" for a backup
system that is working fine.

**Releases.** The last five release directories, newest first, with short SHA
and the time the directory was created **on the box** — not the timestamp in the
release name, which `bin/deploy` builds from the deploying machine's clock in
whatever zone that machine happens to be in. Rendered as the deploy strip under
the diagnostics row, so "this started after Tuesday's deploy" is visible rather
than reconstructed.

---

## Actions

Real actions the panel has, and the reasoning that isn't visible in the code:

- **Requeue stuck photos.** They are requeued rather than marked failed: the
  original is still on disk, and a photo whose original is genuinely gone fails
  once, visibly.
- **Revoke a user's sessions.** One button per row on `/admin/users`.
- **Verify the backup path.** Fetches a snapshot over loopback with the token
  this process would accept and reads the whole body, so a truncated stream
  fails rather than passing. Two traps: the check shares the endpoint's
  ten-per-hour budget with `backupctl`, because both call from 127.0.0.1, and an
  exhausted budget is disguised as a 404 — the same answer a stale token gets.
  Hence the ten-minute cooldown, and a 404 that names both causes rather than
  guessing between them.
- **Photo/disk consistency check.** `cmd/verifydb` and the panel both call
  `ScanPhotoConsistency`, so the drill and the panel can never disagree about
  what "consistent" means. Each list is capped at 50 while the true counts are
  reported, and orphans sort largest-first, since the reason to look at them is
  the space they take. An unreadable `photos/` directory is a reported field
  rather than an error, because the row-to-disk direction — the one that finds
  actual data loss — still works without it.
- **Resend a password reset.** Deliberately skips the one-minute throttle on the
  public endpoint, which exists to stop account enumeration by a stranger and
  has no bearing on an operator acting on a known account. Minting a link
  invalidates any link the user is already holding, since
  `createPasswordResetTokenTx` deletes the previous token — so the response says
  whether that happened and the confirm dialog warns before it does. Queued is
  not sent, so the button is paired with the mail worker stats on the same page.
- **Reprocess photos / requeue face analysis / send a test push / set version
  policy.**

---

## Worker observability

`PushWorkerStats` carries `Sent`, `Failed`, `Deactivated`, `Suppressed`,
`LastSentAt`, `LastError`, `LastErrorAt` and `RecentAttempts` — that shape is
why the push page is good, and the photo and mail workers now match it.

Photo processing failure is the most common real problem this site has and used
to be the least visible. Its recorded durations also replaced the fabricated
"average process time".

The mail worker records three things a queue length could never show: the
attempt count behind a success (a message that took three tries is a mail server
that is struggling), whether a failure was permanent or a give-up, and a message
dropped by a full queue — which never reaches the worker at all, so nothing
downstream would otherwise know it existed. `GetMailQueueLength` is left alone;
the diagnostics strip still uses it.

---

## The weekly digest

"General usage patterns" for a family site with a handful of accounts is not a
retention curve. Retention used to be computed as "registered at least N days
ago AND logged in within the last N days" divided by *all* users, which meant
Day-1 retention could never exceed the share of the whole user base that logged
in yesterday, and the four numbers were comparable to nothing. It was replaced
with `GetWeeklyDigest`: totals for photos, milestones, growth measurements and
chat, then one line per person, then how many accounts did nothing at all.

Two traps it has to work around:

- **"Logged in this week" is not "used the site this week."** `User.LastLogin`
  is written only where a session is *created*; the refresh path in `auth.go`
  deliberately does not touch it, so a phone that has stayed signed in for
  months shows a stale login beside a week of uploads. The per-person list is
  the union of "signed in" and "added something", and says which of the two it
  was rather than picking one and being wrong.
- **A milestone records no author.** Milestones count in the totals and appear
  against nobody. Inventing an author from the family would have been the
  fabricated-number mistake again.

It is its own proc rather than a second reader of `GetAnalyticsOverview`,
because the digest needs names and the overview only ever produced counts.

---

## Styling

Five stylesheets under `frontend/pages/admin/`, sharing nine tokens from
`admin-tokens.ts`.

**Every admin stylesheet must import `admin-tokens.ts`.** Pages are lazy-loaded
chunks. `analytics-styles.ts` and `logs-styles.ts` once used tokens defined in
`admin-styles.ts`, which neither imports — so loading `/admin/analytics` or
`/admin/logs` directly, rather than clicking through from `/admin`, left those
variables undefined and dropped white header text onto an indigo gradient as
inherited dark text.

**Class names are namespaced by stylesheet.** Same reason. Equal-specificity
rules with the same name in two lazy chunks resolve by injection order, so which
one wins depends on the order the operator happened to visit pages in.
`admin-styles.ts` owns the `admin-` prefix, `logs-styles.ts` owns `logs-`,
`analytics-styles.ts` owns `analytics-`. Generic names like `.stat-card`,
`.empty-state` and `.card-icon` previously collided both inside the admin folder
and with the landing, dashboard and settings pages.

**Shared base, page modifier.** `analytics.tsx` imports `admin-styles` and uses
`.admin-container`, `.admin-page`, `.admin-header`, `.admin-badge` and
`.admin-icon`; where it needs to differ it layers a modifier written at higher
specificity — `.admin-container.analytics-container`, not a bare
`.analytics-container` relying on import order.

`.admin-stat-card` and `.metric-card` are deliberately **not** merged. They read
as parallel vocabulary but they are two different card designs — different
radius, shadow, transition and type scale — so unifying them is a visual change
to the analytics page, not a refactor. That is a design decision, not cleanup.

---

## Deliberately not built

- **Triggering deploys or rollbacks from the panel.** `appctl rollback` needs
  root, the app runs as `apps`, and a web button that can restart the process
  serving it is a bad trade for a command you can already type. CI deploys on
  merge with a health gate and auto-rollback.
- **Reading `journalctl` from the app.** The app's own logs are richer, and the
  journal is one `appctl logs family` away.
- **Making `deploy` backup-aware or log-aware.** Both repos are explicit that
  deploy stays ignorant of these, and both are right. The log and backup
  arrangements above are shaped to preserve that.
- **Maintenance mode and cache clearing.** No mechanism behind either.
- **User data export as an admin action.** Account export already exists as a
  user-facing feature.

---

## Known, and fine

- **`GetAnalyticsOverview` does four full bucket scans per page load.** Users,
  families, images and milestones, each fully materialized into a slice, and
  `calculateStorageGrowthTrend` then re-walks the photo slice once per day for
  thirty days. Fine at today's size and will stay fine for a long time. Recorded
  so it isn't a surprise later, not as work to do.
~~- **Class-name collisions outside the admin folder.**~~ **Done**, and it was
  worse than the count suggested. The sixty duplicate names were the visible
  half; the other half was thirty-eight classes a page *used* with no rule
  reachable from its own chunk — `.auth-card` and the rest of the form-card
  vocabulary lived only in `create-account-styles.ts`, so Add Person rendered
  unstyled unless you had visited Create Account first, and `.error-page` lived
  only in three page stylesheets, so `ErrorPage` — the component `AdminGuard`
  renders for a signed-in non-admin — was unstyled on fourteen routes including
  every admin one. Twenty-five CSS variables were undefined everywhere:
  `--color-text-muted` and its family were a second, never-defined naming scheme
  parallel to `--muted`. Shared vocabulary now lives in `global.ts` and
  `styles/timeline-item.ts`; page-local rules that reused a shared name are
  scoped under their page container, so a chunk can no longer restyle a page it
  is not on.
