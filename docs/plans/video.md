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
- R2 has zero egress. Storage runs a little more per GB, playback and the
  worker's own reads cost nothing, and a custom domain plus a Worker in front is
  the cleanest path to authenticated playback.
- B2 is about half R2's price per GB stored, and egress is free up to three
  times the average stored volume, which a family archive watched by a handful
  of relatives never approaches. So it doesn't need Cloudflare in front either.

R2 unless the storage bill actually starts to matter, which for one family it
won't. Confirm current pricing before committing; both are cheap enough that
predictability matters more than the rate.

Sequencing:
- The backend hands out URLs, never bytes. A `MediaStore` interface with
  `UploadURL`, `PlaybackURL`, `Open`, and `Delete`; clients ask for a URL and
  PUT to it or play from it, and never know which backend answered.
- Phase 1 is a local-disk backend. `UploadURL` and `PlaybackURL` return
  short-lived signed URLs on the app itself, and playback goes through
  `http.ServeContent`, which already handles range requests. The whole
  feature — model, worker, timeline, tagging, deletes — lands and settles on
  the one machine already understood, with no new vendor to debug at the same
  time.
- Phase 1 is for this family, not for opening video up. A small VPS disk holds
  one family's clips, not fifty, so video stays off per family until phase 2,
  and the existing quota and `CheckDiskHeadroom` checks hold the line
  meanwhile.
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
- New fields for the object key, duration, and the poster frame.
- Status states the UI can show honestly: uploading, processing, ready, failed.
  Processing takes real time, so "still processing" is a screen, not an edge
  case.
- Creation date pulled from QuickTime metadata the way `PhotoDate` comes from
  EXIF today, so clips land on the timeline where they belong.
- Videos skip face analysis — set `AnalysisStatus` to done at ingest so the
  analysis worker never picks them up.
- A size and duration cap enforced client-side before upload, and the mobile
  upload path in `docs/mobile-api.md` extended to match.

Processing, which is the part worth getting right:
- No transcoding ladder. Phones already record H.264 or HEVC in MP4, and a
  single progressive MP4 served over HTTP range requests from a CDN is a good
  experience for this audience. Building an HLS ladder to serve six people is
  the mistake the video platforms are priced around.
- Remux, don't re-encode, on the common path. `-c copy -movflags +faststart`
  moves the moov atom to the front so playback starts before the file is
  downloaded. It costs seconds and no quality, and it is the whole fix for
  "the video won't start until it's loaded".
- No re-encoding, including HEVC. iPhones record HEVC by default, so a
  re-encode path for it would be the common path, and on a one-vCPU box it
  runs well under real time. Safari, iOS, and Android play HEVC natively, and
  Chrome and Edge do wherever there's a hardware decoder, which is nearly every
  device from the last several years. Remux with `-tag:v hvc1`, since Safari
  refuses HEVC tagged `hev1`.
- The player checks `canPlayType` first, and where it fails — old hardware,
  some Firefox setups — shows the poster with a download link instead of a
  broken player. HDR clips from recent iPhones can look slightly flat off
  Apple devices; that's cosmetic and not a reason to re-encode. A re-encode
  path comes back only if unplayable clips turn out to be a real complaint, and
  then as a niced, one-at-a-time job, or as an H.264 export on the phone
  before upload.
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
  VPS: no request timeout changes, no multipart buffering, no disk pressure.
  S3 multipart above ~100 MB, so a phone on a flaky connection can resume
  instead of restarting, is a later addition; a single PUT covers 5 GB, and
  the v1 size cap sits well under that.
- The worker then pulls the object back down to process it. That reads oddly
  but it's right: the download happens on the worker's schedule rather than
  inside a request, and on R2 it's free. Delete the temp file in a defer, and
  cap total worker scratch space against `MinFreeDiskBytes`.
- The bucket is private, with no public access and no bucket-level anonymous
  read, ever. Playback is a presigned GET minted per request behind the same
  family membership check `servePhotoHandler` already makes. Never store a
  playable URL in the database; store the key and sign on demand.
- Presigned expiry has a real trap: a range-request playback session outlives a
  short-lived URL, and the player fails mid-scrub when it tries to seek. Expiry
  has to cover a whole viewing session, so it's hours, not minutes. If that's
  too loose, the alternative is a Worker in front of R2 validating a signed
  cookie — more moving parts, and only worth it if hours-long links actually
  bother you.
- Quota goes back to counting bytes, which `FamilyStorageUsage` already does.
  Count remote bytes separately from local ones so the disk floor and the family
  quota stay distinct numbers; a family filling a bucket must not read as a full
  disk, and vice versa.
- Deletion has to reach the bucket. Photo deletes today only touch local disk,
  so orphaned objects accruing forever is the obvious failure — reconcile in
  `photo_maintenance.go` alongside the existing sweeps.
- The app must start and run with the bucket unconfigured or unreachable, like
  face tagging does: a `cfg.EnableVideo` flag, credentials from the environment,
  the local backend standing in during development, and an entry in
  `docs/degraded-dependencies.md`. With it off,
  uploads refuse with a clear message and existing rows render as placeholders.
- Export has to include video or the data isn't the family's. A multi-gigabyte
  ZIP is not a download, so the manifest carries presigned per-video links and
  the data-only export keeps listing them as metadata. Originals stay in the
  bucket unmodified, so "give me my files" is a bulk copy, not a conversion.

This is still the first feature that puts family content on a machine the family
doesn't own, and the plan shouldn't skip past it — but the trade is a much
better one than a video platform. Nothing outside processes, indexes, or
transcodes the files; the app does that. Video is opt-in per family and off by
default, photos stay where they are, and nothing already uploaded moves. The
README promises no third-party service the app depends on, and once video
ships that becomes: self-hosted app, one binary, one database file, with bulk
video storage the single named exception.

Open question: whether photos eventually follow video into the bucket. Once the
storage client, the presigned URL path, and the deletion reconciliation all
exist, the argument for keeping photos on local disk is mostly that they already
work — which is a real argument, and the reason not to do both at once.
