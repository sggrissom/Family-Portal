# Video

Families take video, not just photos, and right now there is nowhere to put it.
The clips that matter are the ones nobody edits — a first walk, a recital, a kid
explaining something at length — and they belong next to the photos, on the same
timeline, tagged with the same people.

Unlike most future features this is not really a new domain entity, it's a second
kind of media moving through the pipeline that already exists. What makes it its
own item is that video breaks the one assumption the photo path is built on:
that a family's media fits on the VPS disk and can be served off it. A phone
clip is one to two orders of magnitude larger than a photo. The 10 GB
`FamilyStorageQuotaBytes` is a few hundred videos, the 1 GB `MinFreeDiskBytes`
floor is one careless upload away, and `servePhotoHandler` streaming a long clip
to a phone on cellular ties up a connection for the length of the video.

So the bytes eventually move to object storage — Backblaze B2 or Cloudflare
R2 — while the app keeps everything it already does well: metadata,
permissions, family scoping, and the processing itself. Eventually, because the
feature ships first on local disk behind a storage interface, and the bucket is
a second backend for that interface rather than the starting point (see
Sequencing).

Not a video platform. Mux, Cloudflare Stream, and the like solve a problem this
app doesn't have. They price per stored minute and per delivered minute, they
own the file, they make export somebody else's API, and what they're selling —
an adaptive bitrate ladder for an audience of strangers on unknown connections —
is worth very little for clips a handful of relatives watch. A bucket is a hard
drive somewhere else, which is exactly the amount of outside dependency this
feature justifies.

Not AWS S3 either, though note the S3 *API* is how you talk to both B2 and R2.
One S3-compatible client — `aws-sdk-go-v2` or `minio-go` — pointed at a
different endpoint covers either, so the choice stays a config value rather than
a rewrite.

Choosing between the two:
- R2 has zero egress. Storage runs a little more per GB. Start with private
  bucket access through presigned URLs on the S3 API endpoint; those URLs do
  not work on custom domains. A custom domain plus a Worker using an R2 binding
  and validating signed cookies is a separate, optional playback design.
- B2 is about half R2's price per GB stored, and egress is free up to three
  times the average stored volume, which a family archive watched by a handful
  of relatives never approaches. So it doesn't need Cloudflare in front either.

R2 unless the storage bill actually starts to matter, which for one family it
won't. Confirm current pricing before committing; both are cheap enough that
predictability matters more than the rate.

Sequencing:
- Clients use the same upload and playback contract for either backend. A
  `MediaStore` interface supplies `UploadURL`, `PlaybackURL`, `Open`, `Put`,
  `Stat`, and `Delete`: clients transfer through URLs, while the worker opens
  originals, writes remuxes/posters, and checks object size through the store.
  Upload instructions include the URL, HTTP method, required headers, and
  expiry; clients must use those instructions rather than assume same-origin
  authentication or attach the app's bearer token to a bucket request.
- Phase 1 is a local-disk backend. `UploadURL` and `PlaybackURL` return
  signed URLs on the app itself, with separate upload and viewing-session
  expiries (see Technical notes), and playback goes through
  `http.ServeContent`, which already handles range requests. The whole
  feature — model, worker, timeline, tagging, deletes — lands and settles on
  the one machine already understood, with no new vendor to debug at the same
  time.
- Phase 1 is for this family, not for opening video up. A small VPS disk holds
  one family's clips, not fifty, so video stays off per family until phase 2,
  with quota reservations and disk budgeting extending the existing quota
  and `CheckDiskHeadroom` checks as described below.
- Phase 2 adds the S3-compatible backend and a one-time copy of existing
  files into the bucket. Because clients only ever see URLs, it's a config
  change and a migration, not a client release — which matters most for iOS,
  where releases are the slow part.
- The local backend outlives phase 1 as the dev stub, so little of it is
  throwaway. What is throwaway is the handler that takes the upload through
  the VPS: it must stream to disk rather than buffer, and the reverse proxy's
  body size and timeout limits need raising for it.
- Real sizes and playback behaviour from phase 1 decide the open choices in
  phase 2: which provider, and whether a 720p rendition is needed at all.

Scope:
- Videos alongside photos in the library, on the family timeline, on a person's
  page, and attached to activities, using the same people tagging and the same
  tag vocabulary.
- A `MediaKind` discriminator on `Image` with a `PackImage` version bump, rather
  than a parallel `Video` bucket. A separate bucket is cleaner in isolation and
  worse everywhere else: photo people, tags, timeline, activity attachment, and
  export would each grow a second code path for no gain.
- New fields for original and playback object keys, duration, codec metadata,
  and the poster frame. Keep original and remux distinct so archive/export
  retains the exact uploaded file. Old `Image` records default to photo when
  decoded; preserve their existing paths and status meanings.
- Status states the UI can show honestly: uploading, processing, ready, failed.
  Processing takes real time, so "still processing" is a screen, not an edge
  case.
- Creation date pulled from QuickTime metadata the way `PhotoDate` comes from
  EXIF today, so clips land on the timeline where they belong.
- Videos skip face analysis through an explicit `MediaKind` guard in
  `imageNeedsAnalysis`, worker entry points, and admin/maintenance requeue
  paths. Setting `AnalysisStatus` to done is not sufficient: completed images
  are requeued when `AnalysisVersion` becomes outdated.
- Audit every consumer of `Image`. Videos participate in the library,
  timeline, people tagging, and activity attachments. Keep them out of portrait
  ranking, Growing Up, profile-photo selection, photo-only analysis/search,
  and book photo layouts unless a surface explicitly supports a poster and a
  route to playback. Reuse poster resizing without treating the video original
  as an image; photo processing and consistency scans must branch by kind.
- Choose explicit v1 size and duration caps before implementation. Check them
  client-side for immediate feedback and enforce them server-side during
  transfer and verification. Extend `docs/mobile-api.md` with the same upload
  lifecycle and instructions used by the web client.

Upload lifecycle (the same for local disk and object storage):
- Begin an authenticated upload after checking family membership, upload
  permission, and video enablement. In a write transaction, create an upload
  session with an expiry and a server-generated temporary key, and reserve the
  declared bytes against the family quota. Outstanding reservations count
  toward quota so concurrent uploads cannot each claim the same free space.
- Stream to the temporary key. The local handler validates the signed upload
  scope, enforces the reserved byte limit while reading (including unknown-length bodies),
  and removes partial files on failure. For the bucket backend, constrain the
  signed request's size where supported; never trust declared size or MIME
  type as verification. Pending objects remain private and unplayable.
- The client calls an authenticated completion endpoint with the upload session
  ID. The server verifies ownership and upload state, uses `Stat` to check
  actual size, and records durable pending work. Probe the uploaded file with
  bounded ffprobe execution to validate duration, container, and codecs before
  accepting it for remuxing. Reject over-limit or unsupported files, delete
  them, and release their reservations. Show a clear reason in the UI.
- Completion is idempotent: retrying returns the existing media ID/status
  rather than creating another row or another logical job. Recover pending
  work after restart; do not rely only on an in-memory queue. Process into
  separate final keys so a reusable upload URL cannot overwrite accepted
  content. Only publish playback once the required outputs are complete.
- On success, replace the reservation with recorded original and derivative
  usage atomically. Expired sessions and abandoned or rejected uploads are
  swept after their upload URLs expire, with retries for failed deletes.
  Reconcile late uploads too: an expired session must never become a playable
  row or leave an object stored forever. Deleting media during processing must
  prevent the worker from publishing it and clean up any outputs it created.

Processing, which is the part worth getting right:
- No transcoding ladder. Phones already record H.264 or HEVC in MP4, and a
  single progressive MP4 served over HTTP range requests from a CDN is a good
  experience for this audience. Building an HLS ladder to serve six people is
  the mistake the video platforms are priced around.
- Remux, don't re-encode, on the common path. `-c copy -movflags +faststart`
  moves the moov atom to the front so playback starts before the file is
  downloaded. It costs seconds and no quality, and it is the whole fix for
  "the video won't start until it's loaded".
- HEVC uploads are converted to a browser-compatible H.264/AAC MP4 while
  retaining the untouched original. HDR is tone-mapped to SDR and rotation is
  applied before scaling to at most 1920 pixels on either axis. Encoding runs
  serially in the existing niced worker, with two codec threads, one filter
  thread, and a one-hour timeout. The host needs FFmpeg with libx264, AAC,
  zscale and tonemap support; a missing encoder/filter leaves the original
  available and reports a retryable processing failure.
- Admin → Video Processing → Reprocess failed / HEVC videos upgrades existing
  HEVC playback files from stored originals and retries failures. Videos show
  processing while queued. Run this once after deployment; converted videos
  are no longer eligible. No iOS change or re-upload is required. Playback
  derivatives may be larger than HEVC originals; stored-byte accounting uses
  the actual completed output size.
- Probe and retain video/audio codec information. The player checks
  `canPlayType` with a codec-specific MIME string, not just `video/mp4`;
  its answer is a hint, not proof that the actual clip will play. Handle
  runtime playback errors as well, showing the poster and an original-download
  link with a useful explanation. Distinguish expired URLs/network failures
  from unsupported media; renew an expired URL through authenticated API access
  and preserve playback position when retrying.
- Validate actual iPhone H.264, HEVC, HDR, and portrait-orientation clips on
  iOS/Safari and the family's Linux/Chrome setup before settling this policy.
  Check audio, rotation, seeking, and poster orientation/colour as well as
  whether playback starts. HDR appearance needs inspection, not an assumption
  that differences are always minor. Add a low-priority, one-at-a-time H.264
  conversion path or offer a phone-side H.264 export only if those results or
  family feedback justify it.
- Optionally one 720p rendition for 4K originals, decided after seeing whether
  originals are actually painful to stream. Skip it until then.
- Poster frame extracted as a single still and pushed through the existing photo
  pipeline, so grids, timelines, and link previews are ordinary images.
- A video worker mirroring `InitializePhotoWorker`, but concurrency 1 and
  niced. A remux is cheap on CPU but reads and writes the whole file, so
  several at once compete for disk with everything else the box does. Queue
  depth and failures belong in the existing admin health view.
- ffmpeg is exec'd, not linked, so a missing binary disables video and the app
  still starts — the same bargain face tagging already makes with dlib.

Technical notes:
- With the bucket backend, upload goes direct from the client to the bucket
  against a presigned PUT the backend mints, so nothing large transits the
  VPS during the transfer. Configure bucket CORS for the web client's origins,
  methods, and required headers. Processing still needs local scratch space.
  S3 multipart above ~100 MB, so a phone on a flaky connection can resume
  instead of restarting, is a later addition; a single PUT covers 5 GB, and
  the v1 size cap sits well under that.
- The worker then pulls the object back down to process it. That reads oddly
  but it's right: the download happens on the worker's schedule rather than
  inside a request. Bound ffmpeg/ffprobe execution and clean temporary files on
  success, failure, cancellation, and restart. One worker limits processing
  concurrency; it does not limit concurrent uploads.
- Extend `CheckDiskHeadroom` with coordinated reservations for local uploads
  and worker scratch allocations, shared by all writers using that disk
  budget. A free-space snapshot alone cannot prevent concurrent uploads from
  consuming the same headroom. Budget for the retained original, separate
  remux, poster variants, and any downloaded scratch copy; check again before
  allocation and processing. Wait for capacity rather than starting a job that
  would cross `MinFreeDiskBytes`. Keep the same floor protecting photo writes
  and the database.
- The bucket is private, with no public access and no bucket-level anonymous
  read, ever. Playback is a presigned GET minted per request behind the same
  media access checks `servePhotoHandler` already makes, including family
  membership and scoped permissions. Never store a
  playable URL in the database; store the key and sign on demand.
- Presigned expiry has a real trap: a range-request playback session outlives a
  short-lived URL, and the player fails mid-scrub when it tries to seek. Expiry
  has to cover a whole viewing session, so it's hours, not minutes. If that's
  too loose, the alternative is a Worker in front of R2 validating a signed
  cookie — more moving parts, and only worth it if hours-long links actually
  bother you.
- `FamilyStorageUsage` currently counts original bytes only. Extend video
  accounting to retained originals, remuxes, poster variants, and outstanding
  upload/output reservations; reserve bounded processing output space before
  starting work. Keep logical family usage, actual local bytes, remote bytes,
  and temporary disk reservations distinct. A family filling a bucket must not
  read as a full disk, and a bucket upload must not evade its family quota.
- Deletion has to reach the bucket. Photo deletes today only touch local disk,
  so orphaned objects accruing forever is the obvious failure — reconcile in
  `photo_maintenance.go` alongside the existing sweeps.
- The app must start and run with the bucket unconfigured or unreachable, like
  face tagging does: a `cfg.EnableVideo` flag, credentials from the environment,
  the local backend standing in during development, and an entry in
  `docs/degraded-dependencies.md`. With it off,
  uploads refuse with a clear message and existing rows render as placeholders.
- Export has to include video or the data isn't the family's. The data-only
  export lists stable media IDs and original-file metadata. A downloadable
  manifest may include presigned links for immediate use, but expiring links
  alone are not a durable backup. Provide an authenticated bulk-originals
  download workflow that can request fresh links and retry individual files,
  so a family can save all its actual files without finishing one enormous ZIP
  request before it times out. It must work with local storage as well as the
  bucket. Originals remain unmodified; remuxing is a playback derivative.

This is still the first feature that puts family content on a machine the family
doesn't own, and the plan shouldn't skip past it — but the trade is a much
better one than a video platform. Nothing outside processes, indexes, or
transcodes the files; the app does that. Video is opt-in per family and off by
default, photos stay where they are, and nothing already uploaded moves. The
README promises no third-party service the app depends on, and once video
ships that becomes: self-hosted app, one binary, one database file, with bulk
video storage the single named exception.

Analysis is not part of this plan: videos skip every analyzer here. Bringing
them into search, tag and people suggestions, and adding transcripts is
[Video analysis](video-analysis.md).

Open question: whether photos eventually follow video into the bucket. Once the
storage client, the presigned URL path, and the deletion reconciliation all
exist, the argument for keeping photos on local disk is mostly that they already
work — which is a real argument, and the reason not to do both at once.

Implementation acceptance checks:
- Old photo rows and photo uploads still work; video rows never enter photo
  analysis or portrait selection, including after an analysis version bump,
  admin requeue, or restart.
- Exercise concurrent uploads near quota and disk limits, interrupted transfers,
  invalid/over-limit files, repeated completion, restart during processing, and
  deletion during processing. No duplicate media, leaked reservations, playable
  partial files, or unbounded orphan storage.
- Verify authorized and unauthorized upload/completion/playback access, byte
  range seeking, expired playback URL renewal, and the same client contract
  against both storage backends when the bucket backend lands.
- Run the real-device clip checks above and download exported originals after
  the initial manifest links have expired.

References for implementation:
- [HTML media capability checks](https://html.spec.whatwg.org/multipage/media.html#dom-navigator-canplaytype-dev):
  `canPlayType` returns confidence, not a playback guarantee.
- [R2 presigned URLs](https://developers.cloudflare.com/r2/api/s3/presigned-urls/):
  required headers, CORS, URL reuse, and the S3-endpoint/custom-domain distinction.
