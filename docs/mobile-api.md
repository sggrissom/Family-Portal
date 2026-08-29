# Mobile API contract

What the iOS companion app may rely on, and what it must not. Written for the
1.1 beta: an app in TestFlight is a client I cannot redeploy, so this document
is the list of promises that outlive any one build of it.

Everything here is cited to a file. Where the server's behavior is surprising —
and several things below are — the citation is the point, because the surprise
is what a native client trips over.

> **Scope.** iOS only. Android push registration is refused outright
> (`backend/push_notifications.go`), and nothing here should be read as
> committing to it.

---

## 1. Two transports, one credential

| | `/api/*` | `/rpc/<ProcName>` |
| --- | --- | --- |
| Method | varies, mostly `POST` | always `POST` |
| Body | JSON, or multipart for upload | JSON, `{}` when the proc takes none |
| Success | handler's own shape | the proc's response struct, as JSON |
| Failure | JSON error envelope (§4) | **HTTP 400, plain text** (§4) |
| Auth | `Authorization: Bearer`, or the `authToken` cookie | same |

Procedures are the application. There are 107 of them and they cover people,
growth, milestones, photos, tags, chat, activities, membership, and settings.
The handful of plain HTTP endpoints exist because they do something JSON-RPC
cannot: set a cookie, stream a file, take a multipart body.

**The bearer header works on both.** That is recent and deliberate. vbeam's
`MakeContext` reads the `authToken` cookie or an `x-auth-token` header and
nothing else; `backend/bearer_auth.go` translates `Authorization: Bearer` into
the latter for every request. Before that wrapper, a bearer-only client was
authenticated on login and upload and anonymous on all 107 procedures — the app
worked solely because `URLSession` replayed a cookie out of a jar the app does
not own.

**Do not depend on the cookie jar.** It is cleared by the system, it is
per-process, and both cookies are `Secure`, so a build that ever reaches the
server over plaintext loses its session silently rather than loudly. Send the
bearer header on every request and treat cookies as something the server may
also set.

### Public endpoints

Two things answer before authentication, and only two:

- `GET /api/mobile-version?platform=ios&appVersion=1.2.3` — the version gate
  (§7). Cached 300s. Returns 400 for anything that is not strict
  `major.minor.patch`.
- `GET /.well-known/apple-app-site-association` — universal links
  (`backend/universal_links.go`).

`/healthz` and `/readyz` are for the deploy, not for the app. Do not poll them.

---

## 2. Sessions

### The two tokens

| | Access token | Refresh token |
| --- | --- | --- |
| What | HS256 JWT, subject is the email address | 32-byte random, stored hashed |
| Lifetime | 24 hours (`setAuthJwtCookie`) | 30 days from **login**, not from last use |
| Sent as | `Authorization: Bearer <token>` | request body (§2.3), or cookie |
| Rotates | on every refresh | on every refresh |

The refresh lifetime is a hard ceiling. `refreshTokenLifetime` is inherited by
each successor rather than extended, so a session ends thirty days after the
login that started it however often it is refreshed. A user who opens the app
daily still signs in monthly. That is intended; do not report it as a bug.

### 2.1 Signing in

```http
POST /api/login
Content-Type: application/json

{"email": "a@example.com", "password": "…"}
```

```json
{
  "success": true,
  "token": "<access token>",
  "auth": {
    "id": 4,
    "name": "…",
    "email": "…",
    "isAdmin": false,
    "familyId": 2,
    "families": [{"id": 2, "name": "…", "role": 3, "isPrimary": true}]
  }
}
```

`auth.id` is the user id — not `userId`; the field has the short name and the
app has to match it. `auth.families` lists every family the caller belongs to
and what they may do in each, which is the only place multi-family membership
becomes visible to a client that has not asked for it. `role` is an **integer**,
not a string — `AccessLevel` is a Go `int` and marshals as one: 0 none, 1 view,
2 contribute, 3 admin.

The refresh token is **not** in that body. It arrives as `Set-Cookie:
refreshToken=…`, and a native client has to read it off that header and put it
in the Keychain. This is the one place the cookie is load-bearing for the app.

A failed login is `{"success": false, "error": "Invalid credentials"}` — the
same string for a wrong password, an unknown address, and an account that only
signs in with Google or Apple,
at the same cost in time. Do not try to distinguish them in the UI, and do not
say "no account with that address."

### 2.2 Signing in with Apple

`ASAuthorizationAppleIDProvider` hands the app an identity token. Post it and
get the same envelope §2.1 returns.

```http
POST /api/login/apple/token
Content-Type: application/json

{"idToken": "<credential.identityToken as UTF-8>", "name": "Ada Lovelace"}
```

The server verifies the signature against Apple's published keys, requires the
issuer to be `https://appleid.apple.com`, and requires the audience to equal
`APPLE_IOS_CLIENT_ID` — the app's bundle ID. A token minted for any other
relying party is refused.

Two things about `name`:

- Apple releases `credential.fullName` **only in the response to the very first
  authorization**, and never again. Send it when you have it. The server uses it
  only when it is creating the account, so a later sign-in that omits it does
  not blank out the stored name.
- If the account is new and no name arrives, the server falls back to the local
  part of the address, or to a placeholder when the user chose "Hide My Email".
  The user can change it in settings.

The account is matched on the email in the token, the same key the password and
Google paths use. A user who picks "Hide My Email" gets a
`@privaterelay.appleid.com` address that is stable for this app — but it is a
*different* address than the one their Google or password account uses, so
signing in the two ways lands them in two separate accounts. There is nothing
the app can do about that; do not present them as one.

A failed verification is `{"success": false, "error": "Invalid Apple token"}`.

### 2.3 Refreshing

```http
POST /api/refresh
Content-Type: application/json

{"refreshToken": "<the one from the Keychain>"}
```

```json
{ "success": true, "token": "<new access token>", "refreshToken": "<successor>", "auth": {…} }
```

**`refreshToken` comes back in the body only because you sent it in the body.**
A browser refreshing by cookie does not get it, and must not: the cookie is
`HttpOnly` so that script cannot reach the value. If a request carries both, the
cookie wins — a stale value in a body must never displace the live session.

Store the successor before using the new access token. The old refresh token is
dead the moment this returns.

**Reuse ends the session.** Presenting a refresh token whose successor has
already been issued revokes the entire token family, and the response is a 401
that no retry recovers. There is a one-minute grace window for the genuine
concurrent case (two tabs, a resumed app), so a client that serializes its
refreshes never sees this. A client that fires two refreshes a minute apart with
the same token signs the user out.

### 2.4 Signing out

```http
POST /api/logout
{"refreshToken": "<the one you hold>", "deviceToken": "<APNs token>"}
```

Both fields are optional and both matter. Without `refreshToken` the server has
nothing to revoke and a working session outlives the sign-out for up to thirty
days. Without `deviceToken` the device keeps receiving push notifications for an
account that is no longer signed in on it.

Call this **while the session is still valid.** Clearing local state first
leaves nothing to authenticate with.

### 2.5 What invalidates a session out from under you

- A password change revokes every refresh token for the account.
- Account deletion removes sessions, refresh tokens, and device tokens.
- Refresh-token reuse revokes the family (§2.3).
- The 30-day ceiling.

All of these surface the same way: a 401 on refresh. The only correct response
is to clear local credentials and present the sign-in screen. Retrying is never
right.

---

## 3. Family scoping

Every procedure that touches family data resolves the acting family the same
way, through `ResolveActingFamily` (`backend/access.go`):

- **`familyId: 0` or absent means the caller's primary family.** This is the
  convention throughout, not a special case for one proc.
- A non-zero `familyId` is checked against the caller's memberships and links at
  the required access level. A caller reaching for a family they cannot see gets
  the same answer as one reaching for a family that does not exist.
- Requests naming an **existing record** carry no `familyId` at all. The record
  names its family; a second copy on the request could only disagree with it.

Access comes in four levels — none, view, contribute, admin — and crosses the
wire as the integers 0 through 3. Membership in a family grants admin. A
**family link** grants view and never more, so a linked household can read a
shared child's records and write nothing. If the app shows a write control for
linked content, the server refuses it and the user meets an error they cannot
act on; gate the control on the `role` in `auth.families` instead.

---

## 4. Errors

There are two envelopes, and the difference is not cosmetic.

### `/api/*` — JSON

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "That upload could not be read. Please try again.",
    "requestId": "a1b2c3d4e5f6",
    "timestamp": "2026-08-21T14:02:11Z",
    "request_path": "/api/upload-photo"
  }
}
```

| `code` | HTTP | What the app should do |
| --- | --- | --- |
| `AUTH_ERROR` | 401 | Refresh once; if that fails, sign out |
| `FORBIDDEN` | 403 | Show the message. Do not retry |
| `NOT_FOUND` | 404 | The record is gone, or was never visible. Drop it locally |
| `VALIDATION_ERROR` | 400 | Show the message against the field the user touched |
| `BAD_REQUEST` | 400 | A client bug. Log it |
| `CONFLICT` | 409 | State moved. Re-read, then let the user decide |
| `FILE_TOO_LARGE` | 400 | Message names the limit |
| `INVALID_FILE_TYPE` | 400 | Message names the allowed types |
| `RATE_LIMITED` | 429 | Honor `Retry-After`. Never retry sooner |
| `SERVICE_UNAVAILABLE` | 503 | A dependency is down. Retrying later may work |
| `INTERNAL_ERROR` | 500 | Show the reference code, offer support |

`message` is written for a user and is safe to display verbatim. There is no
second, more detailed field: `AppError.Details` is `json:"-"` and never crosses
the wire.

### `/rpc/*` — plain text, always 400

vbeam's `RespondError` writes `WriteHeader(400)` and then the error string. No
JSON, no `success: false`, no code, and **no 401 even for an auth failure**.

Consequences a client has to handle:

- **Status alone tells you nothing.** A 400 from a proc can be a validation
  failure, an authorization failure, or a missing record.
- **Match `"AuthFailure"` in the body** to detect an expired token on a proc
  call. It is one of four exact strings the server promises not to change
  (`publicErrors` in `backend/errors.go`):

  | Body | Means |
  | --- | --- |
  | `AuthFailure` | no valid token — refresh, then sign out |
  | `Access denied: record belongs to another family` | the record is not yours |
  | `User is not part of a family` | the account has no family yet |
  | `Face analysis is not available on this server` | an optional subsystem is off |
- **Some 400 bodies are finished sentences meant for the user.** The activities
  procedures in particular return things like *"That entry is not in the same
  season as this competition"*. Present a short, non-empty 400 body as-is rather
  than wrapping it in `Server error (400): …`.
- **Anything unexpected is already redacted.** `ProcError` replaces an
  unrecognized error with a fixed message plus `Reference: <12 hex chars>`. That
  code is in the server log. Surface it; it is the only thing support can act
  on.

Every response, both transports, carries `X-Request-Id`. Log it.

---

## 5. Dates and time zones

**The wire format for a timestamp is RFC 3339 with nanoseconds**, because Go's
`time.Time` marshals that way: `"2026-08-21T14:02:11.123456789Z"`. A strict
ISO-8601 decoder rejects fractional seconds. The app's decoder must accept
fractional, plain, and date-only forms.

**An absent date is year 1, not null.** Fields typed `time.Time` rather than
`*time.Time` — `Season.StartDate`, `Event.EndDate`, `Appearance.OccurredAt` —
marshal "not known yet" as `"0001-01-01T00:00:00Z"`. Check for it before
formatting or the UI prints *Jan 1, 1*.

**Date-only fields are `YYYY-MM-DD` strings**, parsed with `time.Parse`, which
yields UTC midnight.

### The one that actually bites

Growth records, milestones, and photo dates take an `inputType`:

| `inputType` | Fields | Server behavior |
| --- | --- | --- |
| `"today"` | none | `time.Now()` — **the server's clock, in the server's zone** |
| `"date"` | `measurementDate: "YYYY-MM-DD"` | parsed as given |
| `"age"` | `ageYears`, `ageMonths` (0–11) | birthday + that offset |

**Send `"date"` with the device's local calendar date. Never send `"today"`.**
The server is not in the user's time zone, so `"today"` from a phone at 8pm can
land on tomorrow. `"date"` is the only form where the record matches the day the
user believes they entered.

---

## 6. Pagination

There is one paginated read: `GetChatMessages`.

```json
{"familyId": 0, "limit": 50, "offset": 0}
```

- `limit` defaults to 100 and is clamped to 1–200. A value outside that range is
  silently replaced by the default rather than refused.
- **`offset` counts backwards from the newest message.** Offset 0 is the live
  end of the conversation; each further page is older.
- Because the pages are cut from the moving end, **a caller paging through
  history must keep its limit fixed**, and new messages arriving mid-scroll will
  shift page boundaries. Reconcile by message id, not by position.

Every other list proc returns the whole set: `ListPeople`, `ListFamilyPhotos`,
`GetPersonMilestones`, `GetFamilyTimeline`, `ListTags`. There is no cursor and
no total count. Sizes are family-scale, not internet-scale; if that stops being
true it is a server change, not a client workaround.

---

## 7. Version gate

```http
GET /api/mobile-version?platform=ios&appVersion=1.2.3
```

```json
{
  "status": "ok" | "update_available" | "update_required",
  "minimumVersion": "1.1.0",
  "latestVersion": "1.4.0",
  "updateUrl": "https://apps.apple.com/…",
  "updateMessage": "A newer version is required."
}
```

Deliberately pre-authentication, so a build too old to talk to the server finds
out before it shows a login form. Check it at launch, before auth.

- `appVersion` must be **exactly three decimal parts, with no prerelease or
  build metadata and no leading zeros** (`parseAppVersion`). `1.0`, `1.2.3-beta.1`
  and `1.01.0` are all 400s. `CFBundleShortVersionString` is what to send, so
  keep `MARKETING_VERSION` in that shape — a build number belongs in
  `CFBundleVersion`, which this endpoint never sees.
- `update_required` is a blocking screen. `updateUrl` is validated server-side
  to be https at `apps.apple.com`, `itunes.apple.com`, or
  `testflight.apple.com`, and a policy with no URL is refused outright, so the
  button always has somewhere to go. TestFlight is on that list precisely so a
  beta build can be forced forward before there is a store listing.
- An unconfigured platform, or a stored row that fails today's validation, comes
  back as `ok`. **Treat any failure of this call as `ok`.** A version gate that
  fails closed on a network error is an app that cannot start on a bad
  connection.

---

## 8. Photos

### Upload

```http
POST /api/upload-photo
Content-Type: multipart/form-data
```

| Field | Notes |
| --- | --- |
| `photo` | the file |
| `title`, `description` | trimmed |
| `inputType`, `photoDate`, `ageYears`, `ageMonths` | §5 |
| `personIds` | a **JSON array in a form field**: `"[3,7]"` |
| `familyId` | omit or blank for the primary family |

Hard limits: 50 MiB per file, 52 MiB for the whole body
(`maxPhotoRequestBytes`), 10-minute read deadline. Downscale on the device
before sending; a phone camera original is close enough to the ceiling to fail
on a slow connection.

The response returns the created `Image` immediately. **Processing has not
happened yet** — derived sizes are generated by a background worker.

### Download

```
GET /api/photo/<id>[/<variant>]
```

Variants: `thumb`, `small`, `medium`, `large` (the default), `xlarge`,
`xxlarge`, `original`. Grids should ask for `thumb` or `small`. Format is
negotiated from `Accept`, so send one.

**Status is expressed in the response, not only in the DTO:**

| `Image.status` | What `/api/photo/<id>` returns |
| --- | --- |
| 0 — ready | the image |
| 1 — processing | a **200 with an SVG placeholder**, `Cache-Control: no-store` |
| 2 — failed | 404 |

A freshly uploaded photo therefore answers 200 with an SVG that is not the
photo. A client that caches that result never shows the real image. The `ETag`
includes the processing status — `"<id>-<variant>-<created>-<status>"` — so a
conditional request after processing completes returns fresh bytes rather than a
304. Revalidate rather than caching by URL alone, or poll `GetPhotoStatus`.

Photo responses are `Cache-Control: private, max-age=300, must-revalidate` plus
that ETag. Five
minutes of free reuse, then revalidate. Nothing else authenticated is cacheable:
`/api`, `/rpc` and `/internal` default to `no-store`.

---

## 9. Chat and the WebSocket

`wss://<host>/ws/chat`, authenticated like any other request — the bearer
header works, since the upgrade goes through `AuthenticateRequest`.

Frames are JSON: `{"type": "...", "payload": {...}, "timestamp": "..."}`.

| Type | Direction | Payload |
| --- | --- | --- |
| `new_message` | server → client | `{"message": ChatMessage}` |
| `delete_message` | server → client | `{"messageId": 12, "userId": 4}` |
| `user_typing` | both | `{"userId": 4, "userName": "…", "isTyping": true}` |
| `user_online` / `user_offline` | server → client | `{"userId": 4, "userName": "…", "isOnline": true}` |
| `heartbeat` | both | echoed back |
| `error` | server → client | `{"message": "…"}` |

Field names are camelCase, like every other JSON body the server emits. Frames
are capped at 64 KiB and the read deadline is 60 seconds, so **send a
`heartbeat` well inside that.** A WebSocket ping frame is not a substitute: the
server's liveness check is the JSON heartbeat.

Connections are closed with a Going Away frame during shutdown. That is a
redeploy, not an error — reconnect with backoff.

The socket is a delivery channel, not a source of truth. `GetChatMessages` is
authoritative; the socket saves a poll.

### Push

`backend/chat.go` queues a push for every new message, to every recipient. The
payload is versioned:

```json
{
  "aps": {"alert": {"title": "New message", "body": "…"}, "category": "chat_message", "badge": 1},
  "data": {
    "v": 1,
    "type": "chat_message",
    "record_id": 812,
    "destination": "/chat",
    "family_id": 2,
    "sender_id": 4,
    "sender_name": "…",
    "message_id": 812
  }
}
```

- `destination` is a site-relative path that matches the web route for the same
  content, and is claimed by the app-site association. Route on it; it is the
  field that will keep working as new event types are added.
- `message_id` repeats `record_id` for chat only, kept so an older build keeps
  routing. New code should read `record_id`.
- **The alert text obeys the recipient's preference.** With previews off, the
  title and body name nothing about the family — no sender, no content. Do not
  reconstruct the preview from `data`; the user turned it off.
- `badge` is always `1` and the server never sends a corrected count. Only the
  app can take the badge down.

---

## 10. Nullable fields

Optional values are `*T` with `omitempty`, which means **absent, not null**.
`rank`, `outOf`, `score`, `personId`, `measurementDate`, `ageYears`,
`ageMonths`. Decode with `decodeIfPresent`, encode with `encodeIfPresent`, and
do not substitute zero — the server distinguishes "no placement" from "first
place" precisely by presence.

Two update procs treat absence as an instruction rather than as silence:
`UpdateSeason` and `UpdateEvent` assign whatever their date parser returns, so
**omitting a date sets it to unknown.** Editors must always send the current
value, including the values the user did not touch.

Over-length text is **truncated, not refused**: 200 characters for a name, 100
for a label, 4000 for notes. A field that lets the user type 300 characters
quietly loses 100 of them. Enforce the caps in the UI.

Whole-set writes replace their whole set — `SetEntryRoster`,
`SetAppearanceResults`, `SetAppearancePhotos`, `SetEventPhotos`,
`UpdatePhotoTags`, `UpdateMilestoneTags`. Send the complete list every time; a
partial list is a deletion.

---

## 11. Retries and idempotency

**There is none.** No endpoint accepts an idempotency key, and no create
deduplicates.

`SendMessage` takes a `clientMessageId`, and the server stores and echoes it —
but it does **not** check it. A retried send creates a second message. The field
exists so the client can match the server's echo to its own optimistic row, not
to make the call safe to repeat.

What follows for the app:

- **A create that times out may have succeeded.** Do not retry it blindly. Two
  measurements is worse than none, because only one of them is wrong and nobody
  can tell which.
- **Retry reads freely.** Every `Get*` / `List*` is safe.
- **Retry deletes freely.** They are idempotent by nature.
- **A queued offline write must be reconciled against a pull**, not replayed and
  assumed. This is why activities writes are deliberately not queued: their
  refusals — an entry from the wrong season, a result naming someone off the
  roster, a rank beyond its field — are ones the device cannot predict, and a
  queued write replayed hours later reports a success that never happened.

If idempotency is ever added it will be an `Idempotency-Key` header honored on
creates, and it will be additive. Do not anticipate it.

---

## 12. Limits

Exceed one and the response is 429 with `Retry-After` in seconds. Honor the
header; there is no faster path.

| Bucket | Burst | Refills over | Covers |
| --- | --- | --- | --- |
| login | 10 | 5 min | login, Google login, Apple login, password change, delete account |
| signup | 5 | 1 hour | `CreateAccount` |
| password-reset | 5 | 15 min | request, validate, reset |
| invite-code | 10 | 15 min | `JoinFamily`, `AcceptFamilyLink` |
| refresh | 30 | 5 min | `/api/refresh` |
| import | 5 | 1 hour | `ImportData`, `/api/import-bundle` |
| upload | 120 | 10 min | `/api/upload-photo` |
| websocket | 30 | 5 min | `/ws/chat` connects |
| photo-read | 600 | 5 min | `/api/photo/*` |
| **default** | **300** | **1 min** | everything else under `/api/` and `/rpc/` |

Buckets are per client and in-process; a restart resets them.

Body sizes: 1 MiB for any JSON request, 52 MiB for a photo upload, 512 MiB for
an import bundle. Deadlines: 30s read for an ordinary request, 10 min for an
upload, 30 min for an import, 2 min write for a response, 30 min for a download.

A photo grid firing 600 requests in five minutes is not a hypothetical — it is
one reason the app needs an image cache rather than a fresh request per
appearance.

---

## 13. What is not promised

- **No API versioning.** There is no `/v1/`, no deprecation window, and no
  contract test suite. The mitigation is the version gate in §7: when a change
  breaks an old build, raise `minimumVersion` and that build is told to update
  before it can do damage. Which means **the gate has to work in every shipped
  build**, and is the single most important thing on this list.
- **Procedure names are not stable by policy.** They are Go function names,
  auto-registered. Renaming one is a silent break for a native client, because
  the TypeScript frontend regenerates and the app does not. Treat a rename as a
  breaking change and raise the minimum version.
- **No Android.** Push registration refuses it and nothing should claim
  otherwise.
- **No offline write protocol.** No delta sync, no server-side conflict
  resolution, no tombstones. `GetFamilyTimeline` is a full read. Client-side
  queueing is the app's own arrangement and the server knows nothing about it.
- **Response shapes may gain fields.** Decode leniently; ignore what you do not
  recognize.

---

## 14. Before each backend release

The 1.0 plan calls for testing older supported builds against the server. In
practice that is:

1. Note the oldest `MARKETING_VERSION` still in the field — the TestFlight build
   list during beta, App Store Connect after.
2. Run that build against the release candidate: launch, sign in, refresh, one
   read of each entity, one write of each entity, a photo upload, a chat send,
   and a push tap.
3. Anything broken is either reverted or answered by raising `minimumVersion` at
   `/admin/app-versions` **in the same deploy**. Raising it afterwards means a
   window where old builds are talking to a server that cannot serve them.

Never raise `minimumVersion` past a build that exists. The admin page reads the
live policy back for exactly this reason.
