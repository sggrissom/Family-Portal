# Family Portal 1.0 Release Plan

This checklist tracks the work required for a reasonable, supportable 1.0 release of the Family Portal website and its companion app. Check an item (`[x]`) only after its acceptance criteria are satisfied. Keep deferred work unchecked and move it to a clearly named future milestone rather than silently removing it.

## Step 1: Define the release and ownership model

- [ ] Confirm the final product name, production domain, support email, and supported countries.
- [ ] Decide which companion-app platforms are included in 1.0: iOS, Android, or both.
- [ ] Define the supported account-holder age and how information about children is handled.
- [ ] Define family roles: owner, administrator, regular adult member, and any read-only role.
- [ ] Document who may invite or remove members, rotate invite codes, export data, merge or delete people, delete content, and delete the family.
- [ ] Define what happens to shared family content when a member leaves or deletes their account.
- [ ] Define the last-member and family-owner deletion/transfer flows.
- [ ] Choose measurable recovery point and recovery time objectives for family data and photos.
- [ ] Freeze the 1.0 feature scope; defer non-release-critical product enhancements to 1.1.

**Exit criteria**

- [ ] Product scope, data ownership, roles, supported platforms, and recovery objectives are written down and approved.

## Step 2: Add legal, privacy, and support surfaces

- [ ] Add a public `/privacy` page.
- [ ] Add a public `/terms` page.
- [ ] Add a public `/support` page with a monitored contact method.
- [ ] Add a public `/delete-account` explanation or link to the authenticated deletion flow.
- [ ] Link privacy and terms from account creation before submission.
- [ ] Link privacy, terms, support, and account deletion from the site footer and settings.
- [ ] Describe all collected data, including names, birth dates, family relationships, growth measurements, photos, chat, device tokens, logs, and analytics.
- [ ] Explain face-analysis processing, storage, retention, and deletion.
- [ ] Explain what AI import sends to external services and whether prompts or responses are retained.
- [ ] Document Google authentication and push-notification data processing.
- [ ] Document data retention, backup retention, deletion delays, and export procedures.
- [ ] Document who inside a family can access shared content.
- [ ] Review production behavior against the published policy.
- [ ] Verify current Apple App Store privacy, account-deletion, and data-use requirements before submission.
- [ ] Verify current Google Play privacy, account-deletion, and Data Safety requirements before submission.
- [ ] Complete the App Store privacy labels and Google Play Data Safety form consistently with the policy.

**Exit criteria**

- [ ] Legal and support pages are publicly reachable, linked throughout the product, accurate for production, and usable in store listings.

## Step 3: Complete account and family lifecycle features

- [ ] Add “delete my account” with recent-authentication confirmation.
- [ ] Revoke all access sessions, refresh tokens, and push tokens during account deletion.
- [ ] Remove or anonymize personal data according to the documented ownership policy.
- [ ] Ensure account deletion also handles derived files, indexes, queued work, face data, logs, and backups according to policy.
- [ ] Add password reset with single-use, short-lived tokens.
- [ ] Make password-reset responses resistant to email/account enumeration.
- [ ] Rate-limit password-reset requests and token attempts.
- [ ] Add password change with current-password verification for password accounts.
- [ ] Revoke other sessions after password or email changes.
- [ ] Add display-name and email management, including email verification if email can change.
- [ ] Add “sign out all devices.”
- [ ] Add a session/device list if feasible for 1.0.
- [ ] Add leave-family behavior.
- [ ] Add owner/admin member removal.
- [ ] Add family ownership transfer.
- [ ] Add invite-code rotation and revocation.
- [ ] Add full-family deletion for the authorized owner.
- [ ] Add tests for account deletion, last-member behavior, ownership transfer, session revocation, and cross-family isolation.

**Exit criteria**

- [ ] A user can recover access, manage credentials, leave a family, and delete their account without direct developer intervention.

## Step 4: Protect authentication and API boundaries

- [ ] Make release builds fail at startup when `JWT_SECRET_KEY` is absent or weak.
- [ ] Validate required production values such as `SITE_ROOT`, Google OAuth settings, APNs/FCM settings, AI settings, and storage paths.
- [ ] Document secret creation, storage, access, rotation, and incident-revocation procedures.
- [ ] Hash refresh tokens at rest.
- [ ] Rotate refresh tokens on use.
- [ ] Detect refresh-token reuse and revoke the affected session family.
- [ ] Periodically purge expired refresh tokens.
- [ ] Shorten access-token lifetime once refresh behavior is reliable.
- [ ] Rate-limit login by IP and account identifier.
- [ ] Rate-limit signup, password reset, invite-code attempts, refresh, Google token login, AI calls, imports, uploads, and WebSocket connections/messages.
- [ ] Use generic authentication failures to avoid account enumeration.
- [ ] Apply explicit JSON, multipart, import, upload, and WebSocket message-size limits.
- [ ] Verify every state-changing endpoint enforces the intended HTTP method.
- [ ] Document the CSRF threat model and add CSRF tokens wherever cookie/SameSite behavior is insufficient.
- [ ] Validate request origins where appropriate.
- [ ] Tighten the Content Security Policy and remove unsafe directives where practical.
- [ ] Add HSTS at the application or reverse-proxy layer after HTTPS behavior is verified.
- [ ] Run a dependency vulnerability scan and remediate release-blocking findings.
- [ ] Add automated secret scanning.
- [ ] Conduct an authorization review of every RPC, upload/download handler, WebSocket action, admin action, and direct file route.
- [ ] Add adversarial tests proving one family cannot read or mutate another family's data.

**Exit criteria**

- [ ] Authentication, session handling, abuse controls, request limits, and family isolation pass documented security tests.

## Step 5: Correct and finish push notifications

- [ ] Require a push token to belong to the authenticated user before it can be unregistered.
- [ ] Correctly remove the old user index when a device token is reassigned.
- [ ] Restrict accepted bundle IDs, environments, and platforms to server configuration.
- [ ] Validate device-token length and format without logging the token.
- [ ] Add cross-user push-token authorization tests.
- [ ] Ensure logout and account deletion deactivate the relevant device tokens.
- [ ] Handle APNs invalid/unregistered token responses and deactivate stale registrations.
- [ ] Confirm whether Android 1.0 includes real FCM delivery.
- [ ] If FCM is not supported in 1.0, reject Android registration and remove Android claims from release materials.
- [ ] Define a versioned push payload with event type, record identifier, and deep-link destination.
- [ ] Avoid including sensitive family content in lock-screen notification text by default.
- [ ] Add notification preference controls and document default behavior.
- [ ] Test production and sandbox push delivery on real devices.

**Exit criteria**

- [ ] Every advertised platform receives secure, preference-aware notifications, and tokens cannot be managed across users.

## Step 6: Establish reliable backup and restore operations

- [ ] Implement scheduled, transactionally consistent Bolt database snapshots.
- [ ] Back up original photos and every non-regenerable file.
- [ ] Identify which derived image assets can be regenerated instead of backed up.
- [ ] Encrypt backups before transfer or at rest with a separately managed key.
- [ ] Store backups off-host in a separate failure domain.
- [ ] Configure daily, weekly, and monthly retention tiers.
- [ ] Monitor backup completion, size, age, and integrity.
- [ ] Alert when the latest successful backup exceeds the recovery-point objective.
- [ ] Document a complete bare-server restore procedure.
- [ ] Test restoration into a disposable environment.
- [ ] Verify restored people, growth, milestones, tags, chat, original photos, and authentication.
- [ ] Check database-to-file consistency after backup and restore.
- [ ] Schedule recurring restore drills and record their results.
- [ ] Ensure deletion behavior across retained backups matches the privacy policy.

**Exit criteria**

- [ ] A replacement server can be restored from off-host backup within the stated recovery objectives, and the procedure has been successfully rehearsed.

## Step 7: Harden the production server lifecycle

- [ ] Configure HTTP read-header, read, write, and idle timeouts appropriate for uploads and WebSockets.
- [ ] Set a maximum request-header size.
- [ ] Handle `SIGTERM` and `SIGINT`.
- [ ] Gracefully stop accepting requests and drain active HTTP requests.
- [ ] Gracefully close WebSocket connections.
- [ ] Stop and drain photo, face-analysis, and push workers where safe.
- [ ] Log and return a nonzero exit status for unexpected server failures.
- [ ] Keep `/healthz` as a lightweight liveness probe.
- [ ] Add `/readyz` checks for database access, writable storage, and required dependencies.
- [ ] Define behavior when face analysis, AI, or push services are unavailable.
- [ ] Prevent optional dependency failures from corrupting or losing primary user data.
- [ ] Verify reverse-proxy limits and timeouts match application limits.
- [ ] Verify TLS, certificate renewal, secure redirects, HSTS, and WebSocket proxying.
- [ ] Add a graceful-shutdown integration test.

**Exit criteria**

- [ ] The service starts only with valid configuration, reports readiness accurately, and shuts down without corrupting data or abandoning requests unexpectedly.

## Step 8: Define a stable companion-app API contract

- [ ] Document every mobile-supported endpoint and RPC.
- [ ] Define cookie versus bearer authentication for native clients.
- [ ] Define how native clients securely receive, store, rotate, and revoke refresh tokens.
- [ ] Decide whether native refresh uses a cookie jar or an explicit secure bearer-token flow.
- [ ] Define stable machine-readable error codes and a consistent error envelope.
- [ ] Define date/time formats, UTC behavior, and user time-zone handling.
- [ ] Define pagination, ordering, limits, and continuation behavior for collections.
- [ ] Define upload types, sizes, compression, progress, retry, and cancellation behavior.
- [ ] Define idempotency behavior for retried creates and uploads.
- [ ] Define conflict behavior for offline edits and stale records.
- [ ] Define API compatibility, deprecation, and support windows.
- [ ] Version any contract that cannot remain backward compatible.
- [ ] Define optional, nullable, and omitted JSON-field semantics.
- [ ] Publish representative request, response, and error examples.
- [ ] Document push payloads and app deep links.
- [ ] Document data export and account-deletion calls for native clients.
- [ ] Add contract tests that use the API exactly as a native client does.
- [ ] Add a small Swift and/or Kotlin integration fixture if generated clients will be used.

**Exit criteria**

- [ ] A mobile developer can build login, refresh, core data flows, photo upload, push registration, export, and deletion without reading server implementation code.

## Step 9: Complete mobile version and compatibility controls

- [ ] Decide whether app-version checks must work before login.
- [ ] If required, expose a public, cacheable version-policy endpoint containing no user data.
- [x] Replace the custom numeric version comparison with a standards-compliant semver implementation or explicitly restrict versions to `major.minor.patch`.
- [x] Define handling for prerelease and build metadata.
- [x] Validate that minimum version never exceeds latest version.
- [ ] Add an admin UI or documented operator command for changing minimum/latest versions.
- [ ] Audit all forced-update messages and store URLs before release.
- [ ] Define a server-side kill switch for broken app versions if needed.
- [ ] Test `ok`, optional-update, mandatory-update, missing-config, malformed-version, and downgrade cases.
- [ ] Test older supported app builds against the current server before every backend release.

**Exit criteria**

- [ ] Version policy is operable, tested, and cannot lock all users out through malformed configuration.

## Step 10: Make CI a trustworthy, read-only quality gate

- [x] Format the five frontend files currently failing Prettier.
- [x] Change linting so it checks Go formatting without modifying source files.
- [x] Run formatting checks in pull-request CI.
- [x] Run `go vet` in pull-request CI.
- [x] Run backend tests in pull-request CI.
- [x] Run TypeScript type checking in pull-request CI.
- [x] Run CSS validation in pull-request CI.
- [x] Build the production frontend and release binary in pull-request CI.
- [x] Add `go test -race` where compatible and document any exclusions.
- [ ] Produce coverage reports and set a pragmatic minimum threshold.
- [ ] Add dependency and secret scans.
- [ ] Ensure every CI check is deterministic and does not mutate tracked files.
- [ ] Protect the main branch and require all release checks before merge.
- [ ] Pin or otherwise review third-party GitHub Actions and deployment dependencies.

**Exit criteria**

- [ ] A clean checkout passes the complete quality gate without changing files, and merging is blocked when any required check fails.

## Step 11: Add end-to-end and release smoke coverage

- [ ] Test account creation and login in a compiled release build.
- [ ] Test family creation/join and invite-code rotation.
- [ ] Test adding, editing, viewing, and deleting a person.
- [ ] Test growth and milestone create/read/update/delete flows.
- [ ] Test photo upload, processing, display, editing, download, and deletion.
- [ ] Test chat send, receive, reconnect, delete, and family isolation.
- [ ] Test tag management, search, filtering, and timelines.
- [ ] Test data-only and full-photo exports.
- [ ] Test import and restore into an empty family.
- [ ] Test access-token expiry, refresh, logout, and sign-out-all-devices.
- [ ] Test account and family deletion.
- [ ] Test admin authorization and non-admin rejection.
- [ ] Run critical flows at phone, tablet, and desktop viewport sizes.
- [ ] Add a post-deploy smoke test for landing, login, readiness, API access, static photos, and WebSockets.

**Exit criteria**

- [ ] The primary website and native-client journeys pass against the same immutable artifact intended for production.

## Step 12: Add production observability and incident response

- [ ] Send logs to durable off-host storage.
- [ ] Add request counts, latency, status, and error metrics.
- [ ] Add login failure and rate-limit metrics without exposing personal data.
- [ ] Monitor upload and image-processing failures.
- [ ] Monitor photo, face-analysis, and push queue depth and oldest-job age.
- [ ] Monitor database size, transaction errors, and storage latency.
- [ ] Alert on disk space and inode exhaustion.
- [ ] Monitor external uptime and readiness.
- [ ] Monitor TLS certificate expiry.
- [ ] Monitor backup freshness and restore-drill status.
- [ ] Add deployment/release annotations to logs and metrics.
- [ ] Use request/correlation IDs in server logs and user-facing error pages.
- [ ] Review logs for emails, invite codes, tokens, photo metadata, AI content, and other personal data; remove or redact unnecessary fields.
- [ ] Set log retention and access controls consistent with the privacy policy.
- [ ] Write incident runbooks for outage, data loss, compromised credentials, abusive traffic, and accidental data exposure.
- [ ] Define who receives alerts and how incidents are escalated.

**Exit criteria**

- [ ] Operators can detect, investigate, communicate, and recover from the main expected failure modes without relying on a user report.

## Step 13: Formalize data retention and deletion

- [ ] Inventory every database bucket, index, file tree, temporary file, log, worker queue, and third-party data flow.
- [ ] Set retention periods for photos, variants, failed uploads, face data, chat, logs, sessions, device tokens, AI content, imports, exports, and backups.
- [ ] Remove abandoned upload and import temporary files automatically.
- [ ] Ensure deleted records are removed from indexes and search structures.
- [ ] Ensure deleted originals and derived assets are removed from storage.
- [ ] Ensure queued background work cannot recreate deleted data.
- [ ] Add scheduled cleanup for expired tokens and stale device registrations.
- [ ] Document backup expiry as part of deletion expectations.
- [ ] Add tests that create and delete a representative family, then verify all relevant storage locations.

**Exit criteria**

- [ ] Retention and deletion behavior is documented, automated, tested, and consistent with public policy.

## Step 14: Establish staging, release, and rollback procedures

- [ ] Create a staging environment that closely matches production.
- [ ] Deploy main-branch builds to staging rather than directly to production.
- [ ] Promote an immutable, previously tested artifact to production.
- [ ] Require an explicit approval or signed/tagged release for production.
- [ ] Protect deployments from running concurrently.
- [ ] Back up production before migrations or risky releases.
- [ ] Add migration compatibility and rollback rules.
- [ ] Document and test application rollback.
- [ ] Document what happens when a database migration cannot be reversed.
- [ ] Generate artifact checksums and retain release artifacts.
- [ ] Use one source of truth for the application version.
- [ ] Show the release version in logs and an authenticated diagnostics view.
- [ ] Create release notes for user-visible, operational, privacy, and API changes.
- [ ] Tag the final release as `v1.0.0`.

**Exit criteria**

- [ ] Production releases are deliberate, observable, reproducible, and reversible within the documented constraints.

## Step 15: Complete accessibility and responsive QA

- [ ] Navigate all primary flows using only a keyboard.
- [ ] Verify visible focus states and logical focus order.
- [ ] Associate form inputs with labels, descriptions, and validation errors.
- [ ] Verify dialogs trap and restore focus.
- [ ] Give icon-only controls accessible names.
- [ ] Provide accessible tables or textual equivalents for charts.
- [ ] Check light and dark theme contrast.
- [ ] Support reduced-motion preferences.
- [ ] Test at 200% text zoom without loss of functionality.
- [ ] Test screen-reader landmarks, headings, status messages, and live updates.
- [ ] Test touch targets and gestures on real phones.
- [ ] Handle iOS safe areas and Android system UI in the companion app.
- [ ] Test photo crop/upload controls with keyboard and assistive technology.
- [ ] Add automated accessibility checks to representative E2E pages.

**Exit criteria**

- [ ] No release-blocking accessibility or responsive defects remain in primary workflows.

## Step 16: Finish product metadata, PWA positioning, and store assets

- [ ] Confirm canonical URLs and remove environment-specific hard-coding where appropriate.
- [ ] Decide which public pages should be indexed and keep private/authenticated content excluded.
- [ ] Add a production Open Graph image and validate social metadata.
- [ ] Verify favicon, maskable icon, Apple touch icon, and PWA manifest behavior.
- [ ] Decide whether the website is merely installable or supports offline use.
- [ ] If offline is not supported, do not advertise offline functionality.
- [ ] If offline is supported, implement safe app-shell caching, cache versioning, update prompts, an offline page, and secure handling of family data.
- [ ] Do not broadly cache authenticated API responses or private photos on shared devices.
- [ ] Prepare App Store and/or Play Store names, descriptions, screenshots, icons, categories, age ratings, and review notes.
- [ ] Ensure website domain, app bundle/package IDs, associated domains, OAuth client IDs, push configuration, and store listings agree.
- [ ] Test universal links/app links and all supported deep links.

**Exit criteria**

- [ ] Public metadata and store materials accurately represent the released product and its actual platform capabilities.

## Step 17: Add project and operations documentation

- [ ] Add a repository `README.md` with product overview, architecture, prerequisites, setup, and common commands.
- [ ] Add a safe `.env.example` describing every required and optional variable without secrets.
- [ ] Document local database and static-file setup.
- [ ] Document production topology, reverse proxy, TLS, paths, permissions, and service users.
- [ ] Document the face-analysis daemon and model installation.
- [ ] Document APNs and/or FCM configuration.
- [ ] Document Google OAuth client setup for web and native apps.
- [ ] Document AI provider configuration and failure behavior.
- [ ] Add the backup and restore runbook.
- [ ] Add the mobile API contract.
- [ ] Add the security and secret-rotation runbook.
- [ ] Add the deployment and rollback runbook.
- [ ] Add the incident-response runbook.
- [ ] Add a repeatable release checklist.

**Exit criteria**

- [ ] A new contributor or operator can build, test, deploy, monitor, back up, restore, and troubleshoot the service from repository documentation.

## Step 18: Improve production error and support experiences

- [ ] Replace raw technical errors with stable, user-friendly messages.
- [ ] Preserve technical details in server logs rather than exposing them to users.
- [ ] Include a correlation ID on unexpected error pages.
- [ ] Distinguish validation, authentication, authorization, offline, not-found, conflict, rate-limit, and server errors.
- [ ] Provide retry controls only for idempotent or safely repeatable operations.
- [ ] Link unexpected errors to support and make the diagnostic ID easy to copy.
- [ ] Add explicit full-disk, failed-upload, failed-processing, unavailable-AI, and unavailable-face-analysis experiences.
- [ ] Test that error responses never expose file paths, SQL/database internals, stack traces, secrets, or other families' information.

**Exit criteria**

- [ ] Users receive actionable errors while operators retain enough correlated detail to diagnose failures safely.

## Step 19: Run the final 1.0 release rehearsal

- [ ] Start from a clean checkout and pass the entire CI-equivalent quality gate.
- [ ] Build the exact immutable release artifact.
- [ ] Restore a recent production-like backup into staging.
- [ ] Run database/file consistency checks.
- [ ] Run browser E2E, native contract, accessibility, and post-deploy smoke tests.
- [ ] Test email/password and Google authentication.
- [ ] Test real-device push notifications on every advertised platform.
- [ ] Test export, import, account deletion, family deletion, and session revocation.
- [ ] Test a mandatory mobile update and then restore normal version policy.
- [ ] Exercise graceful shutdown and restart while background queues contain work.
- [ ] Exercise application rollback.
- [ ] Confirm dashboards, alerts, on-call contacts, support inbox, and status communication paths.
- [ ] Review privacy policy, terms, store disclosures, and release notes one final time.
- [ ] Take a pre-release production backup and verify its integrity.
- [ ] Promote the tested artifact to production.
- [ ] Run production smoke tests.
- [ ] Monitor the release closely through the agreed stabilization period.
- [ ] Publish the `v1.0.0` release notes.

**Exit criteria**

- [ ] The same artifact passed staging rehearsal and production smoke tests, rollback is available, monitoring is healthy, and no P0 issue remains open.

## Post-1.0 candidates

These items are valuable but should not delay 1.0 unless the release scope explicitly promises them.

- [ ] Advanced offline-first synchronization and conflict resolution.
- [ ] Additional family roles or granular per-person permissions.
- [ ] Richer notification preferences and notification categories.
- [ ] Additional analytics and product-growth reporting.
- [ ] Expanded face-tagging workflows.
- [ ] Broader import formats and AI-assisted cleanup.
- [ ] Localization and additional measurement/date conventions.
- [ ] Advanced sharing outside a family.
- [ ] Performance work beyond measured 1.0 service-level objectives.
