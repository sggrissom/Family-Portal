# Family Portal 1.0 Release Plan

**1.0 is the website.** The iOS companion app ships in 1.1 and does not block this release.

The goal is a version of the site that can be handed to families outside my own without me being the support desk, the backup process, or the recovery plan. Everything here is either "the data survives", "a stranger can't break it", or "a user isn't stuck asking me for help."

Work top to bottom — the sections are ordered by what actually hurts if it's missing.

## Not in 1.0

Named so it stays cut, not so it gets rediscovered later:

- Companion app release, App Store submission, store listings and privacy labels, deep links. See [After 1.0](#after-10).
- Android / FCM. The backend already rejects Android push registration (`backend/push_notifications.go:198`). Keep it that way and don't claim Android support anywhere.
- Staging environment, release approval gates, rollback tooling. CI deploys main to the VPS; that's the process.
- On-call rotation, escalation policy, incident runbooks, RPO/RTO targets.
- Formal API versioning, deprecation windows, contract tests.
- Accessibility audit beyond the basics in [Polish](#7-polish).

---

## 1. Don't lose the data

Nothing in the repo backs anything up. If the VPS dies tonight, every photo and growth record is gone. This is the only section that is genuinely unrecoverable if skipped.

- [x] Nightly script: consistent Bolt snapshot (`vbolt` read-tx or file copy while quiesced) + originals from the photo tree.
- [x] Encrypt the archive and push it off-host. Derived image variants are regenerable — skip them, back up originals only.
- [x] Retention: keep 7 daily, 4 weekly.
- [x] **Actually restore one** into a scratch directory and confirm people, growth, milestones, tags, chat, and originals come back and the app boots against it.
- [x] Email myself if the newest successful backup is older than 48h.
- [x] Write the restore steps down in `docs/restore.md` — enough that I can follow them tired, on a fresh box.

## 2. Don't get owned or overrun

Signup is open to the internet and there is no rate limiting anywhere in the backend.

- [x] Fail release startup when `JWT_SECRET_KEY` is absent or weak.
- [x] Extend that check to the rest of the required production config: `SITE_ROOT`, Google OAuth, APNs, AI provider, storage paths. Required settings fail release startup; APNs is all-or-nothing because it belongs to the 1.1 app.
- [x] Add rate limiting middleware; apply to login, signup, password reset, invite-code attempts, refresh, Google token login, AI calls, imports, uploads, and WebSocket connects. Per-client token buckets, keyed by the address the trusted proxy reports, with a catch-all rule so new endpoints are bounded by default.
- [x] Hash refresh tokens at rest. Existing rows are migrated, so the deploy doesn't sign everyone out.
- [x] Rotate refresh tokens on use; on reuse detection, revoke that session family. A one-minute grace window keeps concurrent tabs from looking like theft.
- [x] Periodically purge expired refresh tokens.
- [x] Explicit JSON, multipart, import, upload, and WebSocket message-size limits.
- [x] Generic auth failure messages so login can't be used to enumerate accounts — including equal timing, so an unknown address costs the same bcrypt work as a known one.
- [x] Pass over every RPC, upload/download handler, WebSocket action, and admin action confirming family scoping — plus tests that one family cannot read or mutate another's data. `backend/cross_family_isolation_test.go` calls each procedure as an outsider holding another family's ids.
- [x] Grep the logs for emails, invite codes, tokens, and AI content; redact what doesn't need to be there. Addresses are masked, the undeliverable-mail fallback stops printing bodies in release, AI response previews stay out of the log, and the Google tokeninfo URL no longer reaches an error string.

## 3. Don't trap users

Anything a user can't undo themselves becomes a support request to me.

- [x] Password reset with single-use, short-lived, hashed tokens.
- [x] Enumeration-resistant password-reset responses.
- [x] Rate-limit password-reset requests and token attempts (covered by §2).
- [x] Password change with current-password verification.
- [x] Revoke other sessions after a password change. Every refresh token goes; the browser making the request is handed a fresh session in the same response.
- [x] Delete my account — with password/recent-auth confirmation. Must remove sessions, refresh tokens, device tokens, photos and derived files, face descriptors, and index entries; and queued background work must not resurrect any of it. Confirmation is the typed email address plus the password when the account has one; a Google-only account has no password to prove. A family the deletion empties is destroyed with everything in it, since nobody could ever reach it again. The photo and face-analysis workers now check the record still exists before writing, and the photo worker discards files for a photo deleted mid-processing.
- [x] Leave family. Refused for the last member, since dropping the only membership would strand the household's content; ownership passes to the longest-standing remaining member so a family is never headless.
- [x] Owner can remove a member.
- [x] Invite-code rotation. Generation also checks for collisions now.
- [x] Decide and implement what happens to shared content when a member leaves or deletes — **content stays with the family.** Written down at the top of `backend/membership_procs.go`. Leaving removes exactly one thing: the membership row. A user left with no family gets a fresh empty one, because family-less is not a state the app handles.
- [x] Tests: account deletion clears every store, last-member behavior, session revocation, cross-family isolation.

## 4. Don't ship blind

- [ ] `README.md`: what it is, architecture sketch, prerequisites, setup, common commands.
- [x] `.env.example` listing every required and optional variable, no secrets. Grouped by what fails without it: release-required, optional overrides, all-or-nothing APNs, and the face daemon's own variables.
- [x] Document production topology: reverse proxy, TLS, paths, permissions, service user, face daemon + models. `docs/deployment.md`. Turned up two mismatches worth fixing: the app hardcodes `:8666` while `PORT` in `shared/.env` drives the Caddy upstream and the deploy health check, and the unit's `TimeoutStopSec=15` kills the app before its own 30s drain finishes (both live in `tiny-server-helper`).
- [x] Post-deploy smoke check: landing page, login, `/readyz`, one photo loads, WebSocket connects. `make smoke` runs `cmd/smokecheck`; CI runs it as the last step of the deploy job and warns loudly when the account secrets are unset, rather than skipping quietly. The landing check fetches the content-hashed bundle `index.html` names, so a release whose HTML and `dist` came from different builds fails here instead of serving a blank page, and the WebSocket check sends a heartbeat and waits for the reply, since the handshake alone says nothing about the hub. Every check is read-only; the session it opens is closed by the logout check.
- [x] E2E coverage against a compiled release build for the five flows I'd notice breaking: signup/login, add person, add growth, upload photo, chat. `make e2e` runs `cmd/e2e`, which starts the release binary behind a TLS reverse proxy — the auth cookie is `Secure`, so nothing talking plaintext can hold a session — and adds readiness, the landing bundle, logout, and a `SIGTERM` that has to drain and exit zero. Waiting for the uploaded photo to finish is also the check that face analysis being unreachable doesn't take an upload down with it. It immediately found `BACKUP_TOKEN` missing from the startup config check, which killed a release build on its own *after* the report had claimed the configuration was fine. A release build resolves its storage paths at compile time, so the run needs the production tree and refuses to start — deleting nothing — if that tree holds a database, a `.env`, or any static file.
- [ ] Verify reverse-proxy limits and timeouts match the application's; confirm TLS renewal and WebSocket proxying work.

## 5. Legal and support surface

Four pages, not a compliance program. Needed because other people's kids' photos are in the database.

- [ ] `/privacy` — what's collected (names, birth dates, relationships, growth measurements, photos, chat, device tokens, logs), face-analysis processing and retention, what AI import sends externally, Google auth and push data, retention and deletion behavior, who inside a family can see what.
- [ ] `/terms`.
- [ ] `/support` with an address I actually read.
- [ ] Link all three from the footer, settings, and account creation.
- [ ] Read the privacy page against what production actually does, and fix whichever one is wrong.

## 6. Errors that don't leak or confuse

- [ ] Stable, user-friendly messages in place of raw technical errors; details stay in server logs.
- [ ] Correlation ID on unexpected error pages, easy to copy, linked to support.
- [ ] Distinguish validation / auth / not-found / conflict / rate-limit / server errors.
- [ ] Explicit handling for failed upload, failed processing, AI unavailable, face analysis unavailable.
- [ ] Test that error responses never expose file paths, DB internals, stack traces, secrets, or another family's data.

## 7. Polish

Cheap, visible, and each one is a real defect if left.

- [ ] Keyboard-navigate the primary flows; fix focus order and missing focus states. *(Partly done: a global `:focus-visible` ring and a skip link landed with the contrast work — buttons and links previously had no focus indicator at all. Per-flow tab-order review still outstanding.)*
- [ ] Accessible names on icon-only controls; labels wired to inputs and validation errors.
- [ ] Dialogs trap and restore focus.
- [x] Light and dark theme contrast check. Computed every token pair against WCAG. The dark theme passes throughout; the light theme did not — white on the primary-button gradient was 2.5:1 and the same green as text was 2.3:1, so the greens are deeper now. Form-control borders were 1.13:1 and get their own token.
- [ ] Primary flows at phone, tablet, and desktop widths.
- [ ] Confirm canonical URLs, favicon, Apple touch icon, PWA manifest, and an Open Graph image.
- [ ] Keep authenticated pages out of the index.
- [ ] The site is installable, not offline-capable — don't advertise offline, and don't cache authenticated API responses or private photos.

## 8. Server lifecycle

Mostly done; finishing the tail.

- [x] `SIGTERM` / `SIGINT` handling with graceful HTTP drain, plus an integration test.
- [x] Maximum request-header size.
- [x] `/readyz` covering database access and writable storage.
- [ ] HTTP read/write timeouts tuned for uploads and WebSockets (read-header and idle are already set).
- [ ] Close WebSocket connections gracefully on shutdown.
- [ ] Stop and drain the photo, face-analysis, and push workers where safe.
- [ ] Nonzero exit status and a log line on unexpected server failure.
- [ ] Define behavior when face analysis, AI, or push are down — they must never take primary user data with them.

## 9. Release

- [ ] Clean checkout passes the full CI gate without modifying tracked files.
- [ ] Protect `main`; require the checks before merge.
- [ ] Pin third-party GitHub Actions.
- [ ] Add dependency and secret scanning to CI.
- [ ] One source of truth for the application version; surface it in logs and an authenticated diagnostics view.
- [ ] Fresh backup taken and verified immediately before release.
- [ ] Tag `v1.0.0`, write release notes.
- [ ] Run the smoke check against production and watch it for a few days.

**CI already in place:** formatting checks, `go vet`, backend tests, TypeScript check, CSS validation, release frontend + binary build, `-race`, coverage reporting, and a guard that fails when a check mutates tracked files.

---

## After 1.0

### Companion app (1.1)

Backend groundwork already landed — keep it working, don't extend it.

- [x] Version policy checked before authentication, via a public cacheable endpoint with no user data.
- [x] Semver-compliant comparison, prerelease/build metadata handling, minimum-never-exceeds-latest validation, and tests for ok / optional / mandatory / missing-config / malformed / downgrade.
- [x] Push token ownership enforced, device-token reassignment reindexed correctly, bundle IDs and platforms restricted to server config, token format validated without logging it, cross-user authorization tests, logout deactivates the device token, APNs invalid/unregistered responses deactivate stale registrations.
- [x] Account deletion deactivates all of the user's device tokens — **do this in 1.0** as part of §3. Done: the rows are deleted outright rather than deactivated, since they hold a token identifying a physical device.
- [ ] Admin UI or operator command for setting minimum/latest versions.
- [ ] Audit forced-update messages and store URLs.
- [ ] Versioned push payload with event type, record ID, and deep-link destination.
- [ ] Keep sensitive family content out of lock-screen text by default.
- [ ] Notification preference controls.
- [ ] Real-device push testing, sandbox and production.
- [ ] Mobile API contract doc: supported endpoints, bearer vs cookie auth, refresh token storage and rotation for native clients, error envelope and codes, date/time and time-zone handling, pagination, upload behavior, idempotency for retried creates, nullable-field semantics, worked request/response examples.
- [ ] App Store submission: privacy labels, account-deletion requirement, listing assets, review notes.
- [ ] Universal links and deep links.
- [ ] Test older supported app builds against the server before each backend release.

### Deferred product work

- [ ] Offline-first sync and conflict resolution.
- [ ] Additional family roles, granular per-person permissions, ownership transfer, full-family deletion.
- [ ] Richer notification categories and preferences.
- [ ] Expanded face-tagging workflows.
- [ ] Broader import formats and AI-assisted cleanup.
- [ ] Localization and alternate measurement/date conventions.
- [ ] Sharing outside a family.
- [ ] Scheduled cleanup of abandoned upload/import temp files and stale device registrations.
- [ ] Off-host log storage, request/latency/error metrics, queue-depth and disk-space alerting, TLS expiry monitoring.
- [ ] Performance work beyond measured 1.0 needs.
