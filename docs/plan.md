# Roadmap

Where the app is going next. Short term is work on what already exists; the
rest are new features, roughly in the order they'd be worth building.

## Short term

### UI pass for the current feature set

The interface grew one feature at a time and shows it. Activities (seasons,
competitions, routines, results) landed last and never got folded into the
overall navigation and visual language the way growth, milestones, and photos
were. Go through the app as a whole and make it look like one product:

- Navigation that reflects all the top-level areas, not just the original ones.
- Consistent page structure — headers, empty states, and action placement that
  match across people, growth, milestones, photos, activities, and the timeline.
- Empty and first-run states everywhere. A brand new family currently sees a lot
  of blank pages with no path forward.
- Mobile layout check on every page, not just the ones built mobile-first.

### Landing page rework

The landing page describes a different app than the one that exists. It
advertises a shared calendar (not built) and group chat, photos, and a "family
space", while saying nothing about growth charts, milestones, activities, the
family timeline, or sibling comparison — which are the actual reasons to use it.

Rewrite it around what's really here:

- Growth tracking and side-by-side sibling
  comparison.
- Milestones with tags and search.
- Photos with automatic resizing, EXIF dates, tagging, and face recognition
  that suggests who's in a picture.
- Activities — seasons, competitions, routines, and per-event results.
- Family timeline pulling all of it into one chronological view.
- Real-time family chat.
- Private and self-hosted: one Go binary, one database file, no third-party
  services, no ads, no algorithm.

Drop the calendar claim until the calendar actually ships. Screenshots of the
real thing would carry more weight than the current icon cards.

## Future features

Each of these is a new domain entity, which in this codebase means: a vbolt
bucket plus the indexes to query it, a versioned `vpack` pack function, family
scoping with a cross-family isolation test, `RegisterProc` handlers (the
TypeScript client generates itself), and coverage in export and import so a
family can still take their data with them.

### Shared calendar

Dance schedules, appointments, practices, school events, birthdays — the
recurring logistics that currently live in someone's head or a separate app.

Scope:
- Events with a title, start/end, all-day flag, location, notes, and the family
  members they apply to.
- Recurrence, at least weekly and monthly. This is the part that gets expensive
  to retrofit, so decide the recurrence model before writing the bucket.
- Birthdays derived automatically from existing person birth dates rather than
  entered twice.
- Reminders through the existing push worker.
- Read-only iCal feed per family so the built-in phone calendar can subscribe.
  A subscription URL is far less work than two-way sync with Google or Apple
  Calendar and covers the actual need: seeing family events next to everything
  else on your phone.

Open question: whether writes ever need to flow back from the phone calendar,
or whether a subscribe-only feed plus adding events in the app is enough. Start
subscribe-only.

### Chores and shared to-do list

The kids have rotating weekly chores. The app should track whose turn it is for
what, on what day, instead of anyone having to remember the rotation.

Scope:
- Chore definitions: name, description, which days it happens, and the rotation
  of people it cycles through.
- Automatic assignment — given the rotation and the date, the app knows whose
  turn it is. No manual reassignment each week.
- Completion marking, with history, so "did anyone actually do it" is
  answerable.
- One-off tasks alongside recurring chores, so the same screen works as a plain
  family to-do list.
- Optional reminders via push.

Open questions: whether a missed chore rolls over or is just skipped, and
whether a parent needs to approve completion or the kid marking it done is
enough. Approval matters if this feeds the money tracking below.

### Family bank — allowance and kid finance

Tie into chores. Some chores earn money; kids also get gifted money at Christmas
and birthdays. Track what each kid has and what they've spent.

Scope:
- A balance per kid, built from a ledger of entries rather than a stored number.
- Entry sources: chore completion, recurring allowance, gifts, and manual
  adjustments.
- Spending entries — a parent marks that some of it was spent, with a note on
  what for.
- Per-kid history so a kid can see where their money came from and where it
  went.

This is bookkeeping, not banking: no real money moves, so it just has to be
honest and easy to correct. Store amounts as integer cents; never floats.

Open questions: whether kids can log their own spending or only parents can, and
whether to support goals ("saving for X") — probably worth it, since that's most
of why a kid wants to see the balance at all.

### Meal tracking with kid ratings

Track what gets cooked and what everyone actually thought of it, then use the
history to decide what to make.

Scope:
- Meals composed of parts — entree, sides, dessert — each rated on its own. A
  kid can like the chicken and hate the green beans, and averaging that into one
  score loses the useful information.
- Optional ingredients and method per component, so this doubles as a recipe
  box.
- A cooked-meal record: date plus which components were served.
- Ratings per person per component, 1-5, entered after the meal.
- Photos, reusing the existing photo pipeline.

The payoff is the queries, so build with those in mind:
- What does a specific picky kid actually like?
- What does everyone like that hasn't been made in a while?
- What's never been rated, so it's untested?
- What is nobody's favorite and can quietly retire?

Open question: whether ratings are per-cooking or per-component-overall. Per
cooking is more honest (the same dish varies) and can still be averaged up.

### Video

Families take video, not just photos, and right now there is nowhere to put it.
The clips that matter are the ones nobody edits — a first walk, a recital, a kid
explaining something at length — and they belong next to the photos, on the same
timeline, tagged with the same people.

Unlike the entries above this is not really a new domain entity, it's a second
kind of media moving through the pipeline that already exists. What makes it its
own item is that video breaks the one assumption the photo path is built on:
that a family's media fits on the VPS disk and can be served off it. A phone
clip is one to two orders of magnitude larger than a photo. The 10 GB
`FamilyStorageQuotaBytes` is a few hundred videos, the 1 GB `MinFreeDiskBytes`
floor is one careless upload away, and `servePhotoHandler` streaming a long clip
to a phone on cellular ties up a connection for the length of the video.

So the bytes move to object storage — Backblaze B2 or Cloudflare R2 — served
from the edge, while the app keeps everything it already does well: metadata,
permissions, family scoping, and the processing itself.

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
- B2 is cheaper per GB stored, with egress free through Cloudflare and metered
  otherwise — so in practice it also wants Cloudflare in front, at which point
  it's R2's setup with an extra vendor.

R2 unless the storage bill actually starts to matter, which for one family it
won't. Confirm current pricing before committing; both are cheap enough that
predictability matters more than the rate.

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
- Re-encode only when the codec won't play broadly — HEVC from an iPhone is the
  main case — and only then. That's the expensive path, so it should be the
  uncommon one.
- Optionally one 720p rendition for 4K originals, decided after seeing whether
  originals are actually painful to stream. Skip it until then.
- Poster frame extracted as a single still and pushed through the existing photo
  pipeline, so grids, timelines, and link previews are ordinary images.
- A video worker mirroring `InitializePhotoWorker`, but concurrency 1 and
  niced. Photo resizes are cheap and parallel; an ffmpeg re-encode will eat the
  box if you let several run. Queue depth and failures belong in the existing
  admin health view.
- ffmpeg is exec'd, not linked, so a missing binary disables video and the app
  still starts — the same bargain face tagging already makes with dlib.

Technical notes:
- Upload goes direct from the client to the bucket against a presigned PUT the
  backend mints, so nothing large transits the VPS: no request timeout changes,
  no multipart buffering, no disk pressure. Use S3 multipart above ~100 MB so a
  phone on a flaky connection can resume instead of restarting.
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
  a local stub, and an entry in `docs/degraded-dependencies.md`. With it off,
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
landing page rewrite above claims "no third-party services", and once video
ships that becomes: self-hosted app, one binary, one database file, with bulk
video storage the single named exception.

Open question: whether photos eventually follow video into the bucket. Once the
storage client, the presigned URL path, and the deletion reconciliation all
exist, the argument for keeping photos on local disk is mostly that they already
work — which is a real argument, and the reason not to do both at once.

### Public family summary page

One public page per family at a URL the family picks, meant to be handed to
someone you've just met: here's my family, here's who's in it, here's a couple
of pictures. Not a profile and not a feed — a single page that stays roughly the
same for years.

Scope:
- A family-chosen slug, unique across the install and changeable, with a
  reserved-word list so nobody claims `login` or `admin`.
- Off by default. Publishing is an explicit action by a family owner, and
  unpublishing takes the page down immediately.
- Per-person opt-in: which members appear, what name is shown (a first name or
  nickname, not necessarily the name used inside the app), and whether an age
  shows at all. Full birth dates for kids never belong on a public page — the
  default is nothing, and the most it should ever offer is an age in years.
- A short intro paragraph plus an optional one-liner per person.
- A handful of photos chosen from the existing library.
- Nothing else. No growth data, no milestones, no chat, no timeline. The page
  renders only what was explicitly marked public; it is not a filtered view of
  the private app.

Technical notes:
- This is the first read path in the app with no session behind it. It gets its
  own handler and its own slug-to-page resolver rather than relaxing auth on any
  existing proc.
- Serve a published snapshot, not live data. Editing a person in the app should
  not silently change what strangers see, and unpublishing should be one delete.
- Photos need a public derivative path with unguessable filenames, separate from
  the authenticated photo endpoints. The Christmas-card updates below need the
  same thing, so build it once.
- Rate limit and cache by slug. It is the one URL in the app anyone can hammer.
- `robots.txt` currently disallows everything but the marketing pages. Default
  published pages to noindex and make indexing an explicit opt-in.

Open question: a chosen slug is memorable but guessable and enumerable, while a
random token is neither guessable nor speakable. Slug plus noindex is probably
the right trade for a page whose whole purpose is being handed to a person.

### Christmas-card updates

Periodic "here's what we've been up to" posts — some text, a few photos, a
recap of a vacation — shared with specific people rather than the world. The
model is a Christmas card or a family newsletter, not a timeline.

Scope:
- An update: title, date, body text, photos from the library, and optionally the
  people it's about.
- Draft and publish states. Drafts are visible only to the family.
- A share link per update: unguessable token, revocable, optionally expiring.
- Built for link previews. Open Graph and Twitter card tags, a chosen cover
  image at the right dimensions, and a title and description that read well in a
  Facebook, iMessage, or WhatsApp preview. The preview is what recipients
  actually see, so it is the feature, not a detail.
- Lite social elements, deliberately bounded:
  - Reactions from a small fixed set, attributed if the viewer is a known
    recipient and anonymous otherwise.
  - Comments — name and plain text — that the family can delete, and can turn
    off per update.
  - No follower graph, no feed of other families, no notifications to anyone
    outside the family, nothing algorithmic.
- A per-family archive of past updates, optionally linked from the summary page
  above.
- Notify family members on publish through the existing mail and push workers,
  plus an optional list of outside email addresses — sending it to Grandma is
  the actual use case.

Technical notes:
- The share token is the entire authorization story, so it goes in the path
  rather than a query string that leaks through referrers, and the page sets
  `Referrer-Policy: no-referrer` and noindex.
- Anything with a share link should be assumed cached by whatever it was sent
  through — link previews work by the platform fetching and storing the page and
  its cover image. That is inherent to the feature and the first time this app
  hands content to a third party at all, so say it plainly in the UI when a
  share link is created.
- Reactions and comments are the first writes from unauthenticated visitors:
  hard rate limits, length caps, no HTML, and family-side moderation.

Open question: whether the summary page and updates share one audience concept
or stay separate. Separate is simpler — a summary page is one public thing, an
update is many private links — and nothing yet needs them unified.

### Location sharing

Show where family members are on a map, with optional notifications for arriving
at or leaving known places like home, school, or work.

This one is different from everything above in ways that matter:
- It needs the mobile app to report position in the background, so it is
  primarily mobile app work, not backend work.
- It generates continuous writes rather than occasional ones, so retention has
  to be decided up front. Keeping current position plus a short trail is very
  different from keeping history forever.
- It is the most sensitive data the app would ever hold, and the kids being
  tracked are not the ones who chose to install it. Per-person visibility
  controls and a clear, obvious indicator that sharing is on are requirements,
  not polish.

Given the mobile dependency and the privacy weight, this should come last.

## Not planned

Things worth naming so they don't get accidentally re-litigated:

- No ads, no third-party analytics, no data leaving the host.
- No public feed, no follower graph, no discovery between families. The sharing
  features above are links a family hands to a person, and that is the ceiling.
- No dependency the app can't start without. New features hold to the same rule:
  one binary, one database file, a directory of images.
