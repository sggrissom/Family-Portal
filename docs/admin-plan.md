# Admin section plan

What the admin panel is for, what is actually there today, and what to do about
the gap. Written for the only person who will ever use it, so the bar is not
"complete admin console" — it is **awareness, error investigation, and the small
set of actions worth having a button for.**

Companion to [deployment.md](deployment.md), which describes the box, and
[restore.md](restore.md), which describes getting the data back. Several items
below are joint work with
[`tiny-server-helper`](https://github.com/sggrissom/tiny-server-helper); those
are called out and collected in §4.

---

## 0. What exists today

Seven routes, all gated on `user.Id == 1`:

| route | what it does | state |
| --- | --- | --- |
| `/admin` | diagnostics strip + problems feed + card grid | answers "is anything wrong" without a click |
| `/admin/analytics` | 4 tabs | 1 of 4 works |
| `/admin/users` | table of every user | works, read-only |
| `/admin/photos` | processing + face analysis stats, two reprocess buttons | stats good, one action is dangerous |
| `/admin/logs` | file picker, reference lookup, search, level/category/time/duration filters, perf percentiles | works |
| `/admin/push` | APNs config, devices, delivery attempts, test send | **the best page in the panel** |
| `/admin/app-versions` | min/latest mobile build policy | works |

The push page is the model the rest should follow. It answers the three
questions you actually have when something doesn't work ("is it configured", "is
there anything to send to", "what did the last attempts say"), it names the
common failure directly (`EnvironmentMismatch`), and it gives you a way to
reproduce the problem without a second person. Everything below is, roughly,
"make the other six pages like the push page."

Backend surface: `admin.go` (1286 lines), `admin_push.go`, `analytics.go`,
`diagnostics.go`. Sixteen procs registered.

---

## 1. Things that are wrong (fix before adding anything)

### 1.1 Three of the four analytics tabs are permanent placeholders — **done**

`analytics.tsx:157-159` renders `UsersViewPlaceholder`, `ContentViewPlaceholder`,
and `SystemViewPlaceholder`. Each says "Loading … data…" forever. Nothing is
loading. Meanwhile `GetUserAnalytics`, `GetContentAnalytics`, and
`GetSystemAnalytics` are fully implemented in `analytics.go`, registered in
`RegisterAnalyticsMethods`, and **called by nothing** — the frontend's entire
RPC surface is `GetAnalyticsOverview`.

So: three implemented backend procs are dead code, and three tabs are a lie that
looks like a bug in the loading state.

Fix: fetch on tab selection into the `AnalyticsState` slots that already exist
(`userData` / `contentData` / `systemData` / `loading` are all declared and
unused). This is the highest ratio of value to work in the whole panel — the
backend is already written and tested.

### 1.2 The time-range selector does nothing — **done** (deleted)

`analytics.tsx:148` binds a `<select>` to `state.selectedTimeRange`. Nothing
reads it. Every window in `analytics.go` is hardcoded — 7 days for the activity
summary, 30 for the trends. Either thread a range through the request types or
delete the control. Deleting it is fine; a fixed 30-day window is the right
default for this site.

### 1.3 `ReprocessAllPhotos` will take the site down — **done**

`admin.go` — the proc calls `vbeam.UseWriteTx(ctx)`, then loops over every
unprocessed photo doing a full decode and re-encode to AVIF and WebP at seven
sizes each, writing each result to disk, **all inside the open write
transaction**, and only then commits.

Two consequences:

- bolt allows one writer. For however long that loop runs, every upload, every
  chat message, every milestone save in the entire application blocks.
- The RPC write timeout is two minutes (`request_timeouts.go:37`). A backlog of
  any real size will have the response severed long before the loop ends, with
  no way for the caller to learn what happened. Photos are set to `Status = 1`
  (Processing) *before* the attempt, so an interrupted run leaves rows stuck in
  Processing with nothing that will ever move them.

The correct shape already exists two functions away: `ReanalyzeAllPhotos` checks
that the worker is running, resets the failed rows, and **queues**, returning a
count immediately. Reprocess should do the same into the photo worker. That also
gets progress for free, since the queue depth is already on the diagnostics
strip.

Until it's fixed the button should be disabled rather than left armed.

### 1.4 Log statistics are computed from 50 lines — **done**

`GetLogStats` calls `readRecentLogEntries(path, 50)` per file, and then derives
from those 50 lines: the level histogram, the category histogram, the error
list, `TotalRequests`, and the p50/p90/p95/p99 latency percentiles and slowest-
endpoint table. On a normal day that is well under a minute of traffic.

The percentiles are the problem — they read as a performance summary and are
sampled from whatever happened in the last few seconds before you loaded the
page. Either compute over the whole file (it's a few hundred MB at worst, and
this is an admin page loaded by one person) or label the window honestly and
make it a parameter. Prefer computing properly; §2.1 makes the files small
enough per day that it's cheap.

### 1.5 Two analytics numbers are fabricated — **done**

- `calculateAverageProcessingTime` measures nothing. It returns
  `0.5s + 0.1s per MB + 0.05s per megapixel`, an arithmetic expression over file
  metadata, presented in the UI as "Average Process Time". Delete it, or record
  real durations in the photo worker (which is worth doing anyway — see §3.3).
- `ErrorAnalysis` returns `ErrorsByCategory: []`, `ErrorsByLevel: []`,
  `RecentErrors: []` hardcoded, with `TotalErrors` set to the count of *failed
  photos*. The log data to fill it in properly is already parsed by
  `GetLogStats`.

A dashboard that invents plausible numbers is worse than one with a gap in it,
because you stop being able to tell which numbers to trust.

### 1.6 Retention is not measuring retention — **done**

`GetUserAnalytics` computes Day1/7/30/90 as "registered at least N days ago AND
logged in within the last N days", divided by *all* users. Day-1 retention can
therefore never exceed the share of the whole user base that logged in
yesterday, and the four numbers are not comparable to each other or to anything
else. For a site with a handful of accounts this is noise wearing a percent
sign. Drop it, or replace it with something a single operator can act on —
"accounts that have never logged in since signup" is a real answer to a real
question.

### 1.7 The log viewer's "today" badge never lights — **done**

`admin.go:609` special-cases `family_portal.log`. The logger is initialized as
`family_record` (`app.go`), so the file is `family_record.log` and `IsToday` is
false for the one file you always want. One-line fix; it should read the name
from the same constant `InitRotatingLogger` is given rather than a literal.

### 1.8 Four disabled buttons and a "Coming Soon" card — **done**

`admin.tsx` ships Export User Data, System Health Check, Clear Cache, and
Maintenance Mode as permanently `disabled`, plus a System Settings card that
says "Coming Soon". Of the five, exactly one is worth building —
**System Health Check**, and it is already written (§3.1). Remove the other
four. A button that has never worked teaches you to ignore the row it's in.

### 1.9 `GetAnalyticsOverview` does four full bucket scans per page load

Users, families, images, milestones, each fully materialized into a slice, on
every load. `calculateStorageGrowthTrend` then re-walks the entire photo slice
once per day for thirty days. This is fine at today's size and will stay fine
for a long time — noted so it isn't a surprise later, not as work to do now.

---

## 2. Make the logs trustworthy

This is the part that most changes what the panel is worth, because error
investigation is the stated purpose and the logs are the only place the answers
live.

### 2.1 Logs are destroyed by every deploy — **done**

`app@.service` sets `WorkingDirectory=/srv/apps/%i/current`. lumberjack writes
to `logs/family_record.log` **relative to the working directory**, so logs live
*inside the release directory*. `deploy` creates a new release directory per
deploy and prunes to the last five.

So: every deploy starts an empty log, and the sixth deploy after an incident
deletes the evidence. The log viewer is effectively "since the last deploy",
which is exactly the wrong window — the moment you most want logs is right after
a deploy that broke something, and the deploy that broke it is the one that
reset them.

Fix, in this repo, needing nothing from `tiny-server-helper`: add a `LogDir`
constant to `cfg` — `"logs"` in `local.go`, `/srv/apps/family/shared/logs` in
`release.go` — pass it to `InitRotatingLogger`, and replace the three
`"logs"` literals in `admin.go`. `shared/` survives deploys by design and is
already where the database and photos live. Add a writability probe to
`checkStoragePaths` in `config_check.go` alongside the two that are there.

Everything else in §2 is only worth building on top of this.

### 2.2 There is no way to look up a reference code — **done**

The whole error design converges on this: `ProcError` mints a request id, logs
the real cause against it, and returns `"Something went wrong on our end.
Reference: <id>"` to the user. `ReferencePrefix` exists so the frontend can put
a copy button on it. The intended workflow is that someone sends you the code
and you find the cause.

`GetLogContentRequest` has `Level`, `Category`, `Limit`, `Offset`,
`MinDuration`, `SortBy`, `SortDesc` — and no text search. There is no way to
find a reference code in the panel. You SSH and grep.

Add a `Search string` field, matched against the message, the serialized `Data`,
and the stack trace, across *all* log files rather than one. Then add a single
search box at the top of `/admin/logs` that takes a reference code, and — since
the id is already in `data.requestId` — a dedicated "look up reference" mode
that returns the one entry plus surrounding context lines.

This is perhaps twenty lines of backend and is the single most useful thing
missing from the panel.

### 2.3 Filter by time, and default to errors — **done**

Level and category filters exist; a time range does not, and the default view is
chronological from the start of the file. When you open the log viewer, it is
almost always because something is wrong *now*. Default to most-recent-first,
add a `Since` field, and put an "errors in the last 24h" preset one click away.

### 2.4 Consolidate the log parser — **done**

`admin.go` holds ~600 lines of log parsing — ANSI stripping, three fallback
parse strategies, timing-line regex, stack-trace continuation — with the
line-accumulation loop written twice, once in `GetLogContent` and once in
`readRecentLogEntries`, differing only in whether it paginates or ring-buffers.
Move all of it to `backend/log_reader.go` with one scanner, and `admin.go` drops
to about half its size.

---

## 3. One page that answers "is anything wrong"

Today the panel is organized by subsystem, so noticing a problem means visiting
six pages and knowing what normal looks like on each. For a site with one
operator who checks in occasionally, that is backwards. The landing page should
answer "is anything wrong" without a click, and the subsystem pages should be
where you go *after* it says yes.

### 3.1 Surface `CheckProductionConfig` — **done**

`config_check.go` already validates SITE_ROOT, Google OAuth, mail, the AI
provider, BACKUP_TOKEN, APNs, IOS_APP_ID, and both storage paths, and returns
the complete list rather than the first failure. In a release build it runs once
at startup and the results go to the startup log and nowhere else. The push page
shows the APNs subset; the other seven checks are invisible from the browser.

Add `GetConfigStatus` returning the same `[]ConfigIssue` the startup path
computes — re-run live, so it reflects a `.env` edit and restart, not just what
was true at boot. Render it on `/admin` above the card grid, silent when clean.
That *is* the "System Health Check" button, already written, needing a proc and
a card.

While there: `checkAPNs`'s all-or-nothing pattern is the right model for the new
optional subsystem in §4.1.

### 3.2 A "recent problems" feed on `/admin` — **done**

One panel, above the fold, aggregating what is already available:

- ERROR-level log entries in the last 24h, with their reference codes as links
  into the log viewer (§2.2)
- HTTP 5xx and 4xx counts from the parsed timing lines
- photos in `Status = 2` (failed) and photos stuck in `Status = 1` for more than
  an hour
- `AnalysisStatus = 3` (failed) face analysis count
- push `LastError` / `LastErrorAt` and the failure count from `PushWorkerStats`
- config issues from §3.1
- backup age, which arrived with §4.2

Silent when everything is clean. The point is that a green page means something.

### 3.3 Even out worker observability — **done**

`PushWorkerStats` carries `Sent`, `Failed`, `Deactivated`, `Suppressed`,
`LastSentAt`, `LastError`, `LastErrorAt`, and `RecentAttempts`. That is why the
push page is good.

`ProcessingStats` carries `QueueLength` and `IsRunning`. `AnalysisWorkerStats`
carries the same two. The mail worker exposes only `GetMailQueueLength()`.

Photo processing failure is the most common real problem this site has and the
least visible one. Give the photo worker the same shape as the push worker —
processed/failed counters, last error, and a small ring of recent attempts with
their durations. The durations also replace the fabricated number in §1.5 with a
measured one.

The mail worker now carries the same shape, and it arrived with §5.5 rather than
with the photo worker, because a resend button that can only report "handed to
the worker" is not worth having. Three things get recorded that the queue length
alone could never show: the attempt count behind a success (a message that took
three tries is a mail server that is struggling), whether a failure was permanent
or a give-up, and a message dropped by a full queue — which never reaches the
worker at all, so nothing downstream would otherwise know it existed.
`GetMailQueueLength` is left alone; the diagnostics strip still uses it.

### 3.4 A usage feed instead of retention percentages — **done**

"General usage patterns" for a family site with a handful of accounts is not a
retention curve. It's: who logged in this week, what got uploaded, what got
created, and did anything unusual happen. `GetAnalyticsOverview` already builds
a 7-day photo/milestone/login summary — surface it as a readable weekly digest
on `/admin` rather than as input to a chart, and drop the metrics that need a
larger population to mean anything (§1.6).

`GetWeeklyDigest` in `backend/admin_digest.go` is its own proc rather than a
second reader of `GetAnalyticsOverview`, because the digest needs names and the
overview only ever produced counts. Totals for photos, milestones, growth
measurements and chat, then one line per person, then how many accounts did
nothing at all.

Two things the plan did not anticipate. First, "who logged in this week" is not
the same question as "who used the site this week": `User.LastLogin` is written
only where a session is *created*, and the refresh path at `auth.go:429`
deliberately does not touch it, so a phone that has stayed signed in for months
shows a stale login beside a week of uploads. The per-person list is therefore
the union of "signed in" and "added something", and says which of the two it
was rather than picking one and being wrong. Second, a milestone records no
author, so milestones count in the totals and appear against nobody; inventing
an author from the family would have been §1.5's mistake again.

---

## 4. Where `tiny-server-helper` fits

The admin panel currently stops at the process boundary. It knows its own
uptime, its own queues, and its own log file. It knows nothing about the disk it
is filling, the traffic Caddy is seeing, whether last night's backup ran, or
which release this is relative to the last one.

All of that is already computed by `tiny-server-helper`. None of it is reachable
from the browser.

### 4.1 Consume `metrics-server` (highest value) — **done**

`metrics-server` is deployed and reachable at `metrics.grissom.zone/metrics`. It
already collects, on a 30-second loop:

- host load average, memory, CPU breakdown including iowait, and **disk usage** —
  the 20 GB shared disk that `backupctl`'s retention policy exists to protect
- per-app disk usage under `/srv/apps/<app>/`
- per-app traffic from the Caddy JSON access log over a 15-minute window:
  `requests_total`, `requests_per_min`, `error_4xx`, `error_5xx`, `error_pct`

That last block is the interesting one. It is **real traffic data measured at the
proxy**, independent of the app's own logging, and it directly answers "is the
site actually erroring for people" — a question the panel currently cannot ask
at all.

Shape:

- add `METRICS_URL` and `METRICS_API_KEY` to `/srv/apps/family/shared/.env`.
  Point `METRICS_URL` at the **internal** port on loopback, not
  `metrics.grissom.zone` — same box, no reason for the request to leave it or
  depend on public DNS and TLS.
- a `GetHostMetrics` proc that fetches, caches for ~30s (the collector's own
  interval — polling faster than the source updates gains nothing), and returns
  the `family` app's slice plus the system block.
- treat it as an optional subsystem in `config_check.go` using the APNs
  all-or-nothing pattern: neither variable set is fine and the card is hidden;
  one of the two set is a startup failure.
- render as a "Host" card on `/admin`: disk headroom, memory, and the 4xx/5xx
  rate for `family`. Feed disk and 5xx into the problems feed (§3.2).

The app must degrade quietly if the fetch fails, the same way face analysis does
— a metrics service that is down should not take the admin panel with it. The
existing `/admin` fetch already has this instinct (`diagnostics ?? null`, with a
comment explaining why); follow it.

### 4.2 Surface backup state — via `metrics-server`, not directly — **done**

`backupctl` writes `/var/lib/tiny-server-helper/backup/<app>.last` after a
verified successful run, and its README is emphatic that the failure mode that
matters is a backup nobody is watching. `backupctl check` mails on staleness.
Right now, staleness is visible only in email and in `systemctl`.

The app should **not** read that file. It's root-owned and the app runs as
`apps`, and more importantly "deploying an app does not enable backups" and
"`bin/deploy` needs no backup awareness" are stated design invariants of that
repo. Teaching the application to read backup state would put backup knowledge
in the one place that repo has decided it doesn't belong.

The clean shape puts it in the repo that already owns backups:

- extend `metrics-server` (already an `internal@` unit, already scanning
  `/srv/apps`) with a `backups` block per app: `last_success`, `age_seconds`,
  `size_kb`, and a `registered` flag derived from whether
  `shared/backup.conf` exists.
- the admin panel gets it for free through the §4.1 fetch it is already doing.
- render as one line — "Last backup: 9h ago, 412 MB" — and make
  `registered && never succeeded` a loud entry in the problems feed. That is
  `backupctl check`'s own most important alert class, and it deserves to be
  visible somewhere other than an inbox.

If `metrics-server` shouldn't grow privileges for this, the alternative is for
`backupctl` to write a mode-644 JSON status file that `metrics-server` reads —
same result, and the registry stays the filesystem, as that repo insists.

That alternative is the one that shipped, and it was not optional: `STATE_DIR`
is mode **700**, not just root-owned, because `stage/` inside it holds a
plaintext database snapshot while a run is in flight. A process running as
`apps` cannot traverse into it to read `<app>.last` at all, and relaxing the
mode would trade a backup timestamp for a readable database. So `backupctl`
gained `publish_status`, writing `/var/lib/tiny-server-helper/status/backup-<app>.json`
at mode 644 from the one place `<app>.last` is written — after restic reported
success, and nowhere else, so the two can never disagree. Nothing in that repo
reads it back; it is output, not state, and CLAUDE.md now says so.

`registered` is *not* in that file. `metrics-server` derives it by stat-ing
`shared/backup.conf` itself, which keeps the filesystem as the registry rather
than caching the registry into the status file, and makes "registered, never
succeeded" — the alarm class this section is about — representable as the
absence of a file rather than a flag someone has to remember to clear.

Two things the plan did not anticipate. `size_kb` needs `restic stats`, which
needs the repository password, so it can only be computed by root at run time
and is best-effort for the same reason the `status` column is: a cosmetic
number must never be what turns a successful backup into a failed unit. And
"not registered at all" turned out to deserve its own entry in the problems feed
next to "never succeeded" and "stale" — an app nobody ever wrote a `backup.conf`
for is the quietest way to have no backups. All three are silent when the
metrics service is unreachable: unknown is not the same as broken, or every
development machine would carry a permanent alarm.

### 4.3 Release history next to the error history — **done**

The diagnostics strip already shows version, commit, build time, and uptime from
the linker stamps — most of what you want. The gap is *history*: "what was the
previous release and when did this one go out", which is the first question
after any error spike.

`metrics-server`'s `apps.rs` already walks `/srv/apps`. Listing `releases/` and
resolving the `current` symlink is a handful of lines and gives every app a
deploy timeline. Rendered as a row of the last five deploys with their
timestamps and SHAs, next to the problems feed, it makes "this started after the
Tuesday deploy" a thing you can see instead of a thing you reconstruct.

This also finally makes the `family.caddy` hand-edit visible in principle — see
deployment.md's note that `appctl domain` would silently drop the `www` block on
its next run.

It was indeed a handful of lines. One correction to the shape: the timestamp
does not come from the release *name*. `bin/deploy` builds that name with
`date +%Y-%m-%d_%H%M%S` on the **deploying** machine, in whatever zone that
machine is in, so reading it as an instant would misdate every deploy made from
a laptop in another timezone. The directory's mtime is a real instant, recorded
on the box, and is what the strip shows; the name still supplies the short SHA.
Sorting stays lexical on the name, which is chronological regardless.

While in `apps.rs`: `disk_usage_kb` followed the `current` symlink and counted
the running release twice. `file_type()` does not follow links, so the "This
app" figure on the Host card is now the size of the app rather than the size of
the app plus its newest release.

### 4.4 The site isn't being monitored

`monitor-tui/sites.toml` lists stevengrissom.com, grissom.zone, releve.live,
chess.grissom.zone, and karaoke.grissom.zone. It does **not** list
`familyrecord.app`.

The production site with by far the most moving parts — a database, four
background workers, a WebSocket, a photo pipeline, a face-recognition daemon
over a Unix socket, APNs — is the one site not in the monitor.

One-line fix in that repo. Point it at **`/readyz`, not `/healthz`**:
`/healthz` returns `ok` unconditionally from a closure that touches nothing,
so it stays green through an unreadable database or an unwritable static
directory. `/readyz` opens a bolt transaction and round-trips a temp file in
`shared/static/`, which is exactly the failure class worth being paged about.

Worth considering while there: the `[settings.alerts]` block supports per-site
overrides, and this is the site where `consecutive_failures = 1` is justified.

### 4.5 Not worth doing

For completeness, integrations considered and rejected:

- **Triggering deploys or rollbacks from the panel.** `appctl rollback` needs
  root, the app runs as `apps`, and a web button that can restart the process
  serving it is a bad trade for a command you can already type. CI already
  deploys on merge with a health gate and auto-rollback.
- **Reading `journalctl` from the app.** The app's own logs are richer once §2.1
  lands, and the journal is one `appctl logs family` away.
- **Making `deploy` backup-aware or log-aware.** Both repos are explicit that
  deploy stays ignorant of these, and both are right. §2.1 and §4.2 are
  deliberately shaped to preserve that.

---

## 5. Actions worth having a button for

The panel has four real actions today: reprocess photos (broken, §1.3), requeue
face analysis, send a test push, set mobile version policy. Candidates to add,
in rough order of how often they'd be used:

1. ~~**Clear a stuck photo.**~~ **Done** — `RequeueStuckPhotos`, on the photo
   page. They are requeued rather than marked failed: the original is still on
   disk, and a photo whose original is genuinely gone fails once, visibly.
2. ~~**Revoke a user's sessions.**~~ **Done** — `RevokeUserSessions`, one button
   per row on `/admin/users`.
3. ~~**Verify the backup path end-to-end.**~~ **Done** — `VerifyBackupPath`,
   on `/admin`. It fetches a snapshot over loopback with the token this process
   would accept and reads the whole body, so a truncated stream fails rather
   than passing. Two things the plan did not anticipate: the check shares the
   endpoint's ten-per-hour budget with `backupctl`, because both call from
   127.0.0.1, and an exhausted budget is disguised as a 404 — the same answer a
   stale token gets. Hence a ten-minute cooldown on the check, and a 404 that
   names both causes rather than guessing between them.
4. ~~**Photo/disk consistency check.**~~ **Done** — `CheckPhotoConsistency`,
   a button on `/admin/photos`. `cmd/verifydb` no longer has its own copy of the
   cross-check: both call `ScanPhotoConsistency`, so the drill and the panel can
   never disagree about what "consistent" means. The proc caps each list at 50
   while reporting the true counts, and sorts orphans largest-first, since the
   reason to look at them is the space they take. One thing the plan did not
   anticipate: an unreadable `photos/` directory has to be a reported field
   rather than an error, because the row-to-disk direction — the one that finds
   actual data loss — still works without it.
5. ~~**Resend a password reset**~~ **Done** — `ResendPasswordReset`, one button
   per row on `/admin/users`, next to the revoke button. It deliberately skips
   the one-minute throttle on the public endpoint, which exists to stop account
   enumeration by a stranger and has no bearing on an operator acting on a known
   account. Two things the plan did not anticipate. First, minting a link
   invalidates any link the user is already holding — `createPasswordResetTokenTx`
   deletes the previous token — so the response says whether that happened and
   the confirm dialog warns before it does. Second, "the mail worker's failures
   are otherwise entirely invisible" turned out to be the harder half: queued is
   not sent, so the button is paired with the mail worker stats from §3.3, on the
   same page, and `LastError` feeds the problems feed the way push already did.

Deliberately not on the list: maintenance mode, cache clearing, and user data
export as an admin action. The first two have no mechanism behind them, and
account export already exists as a user-facing feature.

---

## 6. Cleanup

Not urgent, but it's what makes the above cheaper to build.

- ~~**`user.Id != 1` appears 16 times, with 19 copies of the string
  `"Unauthorized: Admin access required"`.**~~ **Done** — one
  `requireAdminAccess` helper, one exported `ErrAdminRequired`, and an
  `AdminUserId` constant in place of the magic number.
- ~~**`admin.go` is 656 lines** after §2.4 extracted the log parser, down from
  1286. Photo maintenance still belongs next to the photo worker.~~ **Done** —
  `backend/photo_maintenance.go` now holds the photo procs and the §5.4 scan, and
  `admin.go` is 444 lines of users and logs. `RegisterAdminMethods` calls
  `RegisterPhotoMaintenanceMethods` in place of the five procs, the way it
  already called `RegisterAnalyticsMethods` — registering from `app.go` instead
  would have reordered every proc after it in the generated `server.ts` for no
  gain.
- ~~**Every admin page reimplements the same "Access Denied" block** — six
  near-identical copies of a `Header`/`error-page`/`Footer` tree.~~ **Done** —
  seven copies, in fact. `components/AdminGuard.tsx` exports `adminView`, which
  takes the page body as a callback and owns both gates: the redirect for a
  signed-out visitor and the denial page for a signed-in non-admin. The denial
  now reuses the existing `ErrorPage` component, and every page links back to
  `/dashboard` rather than to `/admin`, which a non-admin cannot open either.
- ~~**`admin-styles.ts` is 813 lines and `analytics-styles.ts` is 979** — 1792
  lines of CSS for seven pages, more than the pages themselves. The two files
  overlap heavily (cards, tables, badges, breadcrumbs). Worth one merge pass.~~
  **Partly done, and the premise was wrong in an interesting way.** The two
  files share exactly *one* class name (`.family-name`); the "overlap" is
  parallel vocabulary under different names — `.admin-container` beside
  `.analytics-container`, `.stat-card` beside `.metric-card` — so merging them
  means renaming classes across the pages, not deduplicating rules. Across all
  five admin stylesheets there are 24 byte-identical declaration blocks, worth
  perhaps a hundred lines. Not nothing, but not where the 3,000 lines are.

  What the pass did find is a real bug. `analytics-styles.ts` used
  `--admin-text-on-accent` and `logs-styles.ts` used `--admin-accent`,
  `--admin-border` and `--admin-surface`, but those variables were defined in
  `admin-styles.ts`, which neither page imports — and pages are lazy-loaded
  chunks, so `chunk-EQHNNXUN.js` (the one carrying the `:root` block) never
  reached either. Loading `/admin/analytics` or `/admin/logs` directly, rather
  than clicking through from `/admin`, left those variables undefined: white
  header text fell back to inherited dark text on an indigo gradient. The nine
  admin tokens and their dark-theme overrides now live in `admin-tokens.ts`,
  which all five stylesheets import, and the built chunk graph confirms every
  admin page pulls it.

  Also deleted: seven rules nothing referenced any more — `.time-selector`
  (§1.2), `.error-categories`/`.error-category` (§1.5), `.progress-bar`,
  `.progress-fill`, `.card-placeholder` and `.admin-btn-danger`. What remains of
  this bullet is the class-renaming half, which is a visual-diff refactor and
  should be done when someone can look at the pages.
- ~~**`admin.tsx` and `users.tsx` duplicate date formatting**~~ **Done** — the
  file is `lib/dateUtils.ts`, and it gained `formatDateTime` (users.tsx's local
  copy) and `formatRelativeTime` (identical `formatWhen`/`shortWhen` in
  admin.tsx and photos.tsx). The two differed only in what they showed for an
  unparseable timestamp, now an optional `fallback` argument; the shared one
  also treats a zero-value time as absent rather than rendering it as two
  thousand years ago. `chat.tsx` has a fourth variant, left alone because its
  wording and its seven-day cutoff are deliberately different.
- ~~**Three copies of a byte formatter.**~~ **Done** — `lib/formatBytes.ts`.
  `admin.tsx` and `analytics.tsx` each had one, differing only in that the
  analytics copy always printed a decimal ("512.0 B"); the shared one drops it
  above ten units and for bare bytes.

---

## Suggested order

1. ~~**§1** — the things that are wrong.~~ **Done** (§1.9, bucket scans, was
   never work to do; §1.4 landed with the §2 log work).
2. ~~**§2.1** — logs out of the release directory.~~ **Done**, along with §2.4
   (one scanner in `backend/log_reader.go`) and §1.4 (stats over the whole
   file, not the last 50 lines).
3. ~~**§2.2** — reference-code search.~~ **Done**, with §2.3 (time range,
   newest-first default, and an "errors in the last 24h" preset). §2 is
   complete.
4. ~~**§3.1 + §3.2** — config status and the problems feed.~~ **Done.** Backup
   age is the one entry still missing from the feed; it arrives with §4.2.
5. ~~**§4.1** — `metrics-server`.~~ **Done** on this side: `GetHostMetrics`,
   the Host card, and disk + proxy 5xx in the problems feed. Needs
   `METRICS_URL` and `METRICS_API_KEY` in `shared/.env` (see deployment.md).
6. ~~**§4.4** — add the site to `monitor-tui`.~~ **Done** in that repo:
   `familyrecord.app/readyz`, with the per-site
   `consecutive_failures = 1` override the section argued for.
7. ~~**§6** — the `AdminGuard` wrapper and the date-formatting duplication.~~
   **Done.** What remains of §6 is the styles merge and moving photo
   maintenance out of `admin.go`.
8. ~~**§5.4** — photo/disk consistency.~~ **Done**, sharing one scan with
   `cmd/verifydb`.
9. ~~**§5.5** — resend a password reset.~~ **Done**, with the mail-worker half
   of §3.3 and the mail entry in the problems feed, and §6's photo-maintenance
   move alongside it.
10. ~~Everything else as it becomes annoying. In rough order of appeal: §3.4
    (weekly usage digest), the §6 styles merge, then §4.2 and §4.3, which are
    both work in `tiny-server-helper` first.~~ **Done**, except the class-
    renaming half of the §6 styles merge. §4.2 and §4.3 needed
    `metrics-server` to grow a `backups` and a `releases` block per app, and
    §4.2 needed `backupctl` to publish a status file a non-root reader can
    see; both are in `tiny-server-helper` and **neither reaches production
    until `metrics-server` is redeployed and `backupctl` reinstalled on the
    box** — until then the app simply sees neither block, the deploy strip
    stays hidden, and the backup line reads as never registered.

What is left in this document: §1.9 (bucket scans, explicitly not work to do),
the class-renaming half of §6, and whatever §5 candidates become annoying.
