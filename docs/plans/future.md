# Future ideas

Status: deferred beyond v1. This is one consolidated backlog, not a release
checklist. The active scope is [v1.md](v1.md). Video is explicitly wanted after
v1; the remaining entries are candidates to reconsider when useful.

## Inventory and categories

Complexity is relative: small, medium, or large. Estimates are planning aids,
not implementation commitments.

| Category | Feature | Remaining work | Priority | Complexity |
| --- | --- | --- | --- | --- |
| Media | Video | Upload, processing, private playback, storage, quotas, and durable export. | Wanted after v1 | Large |
| Books | Additional outputs and styles | Extend the renderer(s) chosen for v1, print-ready layouts, richer themes, or vendor integrations. | Candidate | Medium–large |
| Books | Recurring-occasion comparison series | Saved photo choices across school years/holidays and richer comparison layouts. Manual themed books are already in v1. | Candidate | Medium |
| Historical accuracy | Approximate dates and ages | Manual partial dates/ranges/age assertions, honest browsing/matching, then optionally reviewed estimation. | Candidate | Large |
| Discovery | Suggested event collections / trips | Reviewable historical photo groupings; preserve manual corrections and dismissals. | Candidate | Medium |
| Discovery | Search everything | Extend existing photo/milestone search to activity notes, entries, and events. | Candidate | Medium |
| Discovery | Saved sibling montage choices | Age montages already work; persist replacements and add recurring-occasion views. | Candidate | Small–medium |
| Person records | Birth details | Time, weight, length, place; consider initial growth measurements. | Candidate | Small–medium |
| Person records | Head circumference | Add a measurement type and appropriate infant charts/contracts. | Candidate | Medium |
| Person records | Nicknames and bio | Small descriptive fields; decide whether changing interests should retain history. | Candidate | Small |
| Person records | Dates of death / in memoriam | Preserve a person's history and make age/birthday/current-state behavior appropriate. | Candidate | Medium |
| Person records | Family tree | Render the existing relation graph with sensible navigation. | Candidate | Medium |
| Person records | School years | School, grade, teacher, class photo; try existing activity seasons first. | Candidate | Small–medium |
| Entries | Letters to the future | Dated/age-triggered sealed letters and opening notifications. | Candidate | Medium |
| Entries | Rich documents | PDF/document ingestion, OCR, and richer certificate/report-card support beyond photographed artwork. | Candidate | Medium–large |
| Entries | Health record lite | Vaccinations, allergies, visits; evaluate sensitivity and scope before building. | Needs discussion | Medium–large |
| Entries | Pregnancy record | Ultrasounds/bump series associated with pregnancy and carried into the child's history. | Candidate | Medium |
| Contribution | Who recorded it | Attribution on milestones/growth and contributor workflows. | Candidate | Small–medium |
| Contribution | Reactions and comments | Internal entry discussion for authorized family/linked viewers. | Candidate | Medium |
| Contribution | Activity notifications | Opt-in new-photo/milestone notifications; push currently covers chat. | Candidate | Medium |
| Contribution | Weekly digest | Opt-in summary of recently added records, distinct from occurrence dates. | Candidate | Medium |
| Sharing | Public family summary | Explicit published snapshot with selected people/photos. | Candidate | Medium |
| Sharing | Christmas-card updates | Drafted updates, revocable links, previews, optional reactions/comments and distribution. | Candidate | Medium–large |
| Durability | Scheduled export | Export reminders or automatic copies to an explicitly chosen destination. | Candidate | Medium |
| Optional assistance | Captions and written summaries | Evaluate local models; external text processing only through an explicit choice. | Experiment | Medium–large |
| Optional assistance | Help estimate old photos | Selected-record, reviewed age/date suggestions after manual uncertainty support. | Experiment | Large |

## Carried-forward scope notes

- **Book synthesis is v1.** Former year-in-review and print/PDF ideas are folded
  into the book workflow. The initial output format is an early v1 decision;
  this backlog must not silently defer the entire artifact.
- **Artwork and annual interviews are v1.** Only richer document formats and
  audio/video interview answers are deferred.
- **Quotes are built**, using a milestone category and per-person Quotes tab.
  Review older quotes filed under behavior when convenient; do not rebuild them.
- **Age comparison montages are built.** Remaining work is recurring occasions
  and saving replacement choices.
- **Suggested place tags were deliberately dropped.** Family places and place
  filtering already serve that purpose; they are not an outstanding task.
- **Birth details/head circumference** must avoid duplicate measurements and
  retain export/import and mobile compatibility.
- **Letters** need a defined policy for sealed access, date/age changes,
  notifications, and backups; the old “small” estimate understates those decisions.
- **School years** should first be tried as an activity kind rather than a new entity.
- **Contribution notifications/digests** must honor current permissions at delivery;
  a family link is not unrestricted membership.
- **Durability** is distinct from beautiful books. Existing raw export/import
  remains the complete-data portability mechanism.

## Video — wanted after v1

The preserved proposal below contains design options, not a finalized provider,
schema, codec policy, or pricing claim. Recheck the current upload pipeline,
browser support, object-store behavior, and prices when implementation begins.


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
README promises no third-party service the app depends on, and once video
ships that becomes: self-hosted app, one binary, one database file, with bulk
video storage the single named exception.

Open question: whether photos eventually follow video into the bucket. Once the
storage client, the presigned URL path, and the deletion reconciliation all
exist, the argument for keeping photos on local disk is mostly that they already
work — which is a real argument, and the reason not to do both at once.

## Approximate dates and ages


Status: planned direction; not implemented. Build manual support first.
AI assistance is a later experiment, not a dependency or provider commitment.

### Goal

An old photo or remembered milestone should be useful even when nobody knows
its exact date. Preserve what the family knows without inventing precision.
This particularly supports same-age comparisons between children, parents,
and grandparents.

Initial scope is photo dates, followed by remembered milestones. Measurements
keep their existing exact-date requirements; do not infer clinical measurements
or silently broaden every date field. Partial birth dates are a separate
decision, but comparison must tolerate an unknown birthday.

### Entry and representation

Allow:

- an exact date
- a known month and year, or year only
- an approximate date such as "around 1970"
- an explicit range such as "between 1968 and 1971"
- a person's approximate age, such as "about age 8" or "age 6–9"
- an unknown date

Distinguish precision from uncertainty: "sometime in 1970" is not the same
claim as "around 1970." For an approximate point, show and allow editing the
range used for matching; do not apply an invisible universal tolerance.

Keep the original assertion as well as derived bounds. An age assertion names
its anchor person and depends on that person's birth date; it must survive
correction of the birthday without turning into an unrelated fixed date.
If the birthday is unknown, keep the age assertion and leave calendar dates
unresolved. A photo with several people uses an explicitly chosen anchor.

Preserve:

- the entered date/age expression, precision, and uncertainty
- earliest/latest plausible bounds where derivable
- the source: family recollection, inscription, original metadata, or AI suggestion
- an optional supporting note and the person who accepted or changed it

The final schema needs design before implementation. Do not squeeze this into
one timestamp plus a display label. Any internal sorting key is not the
recorded date and must never be exposed as one.

A scan's creation/upload date is not evidence of the original photo date.
Keep occurrence dates separate from file metadata and creation timestamps.
Do not reinterpret existing exact dates during migration.

### How it appears throughout the app

| Surface | Behavior |
| --- | --- |
| Photo or milestone detail | Show the date/age label, uncertainty, source, and supporting note. Make refinement easy. |
| History | Label approximate periods honestly. Broad ranges must not appear to have a known day or exact order relative to other records. Keep undated records reachable. |
| Date filters | Include overlapping ranges with an approximate-match label; explain the matching behavior. |
| Same age | Label estimated ages and distinguish possible overlap from a precise match. Do not let a broad range masquerade as the nearest exact-age record. |
| Future family-at-a-date views | Mark an overlapping estimate as "Possibly from this period." |
| On this day | Use known anniversary dates; a year-only or estimated day must not produce a false anniversary. |
| Export/import and sharing | Preserve precision, bounds, source, and notes where appropriate to the audience. Never flatten an estimate into an exact date. |

Keep exact matches and possible matches distinguishable. Decide and test the
ranking rules for wide ranges before enabling approximate Same age results.
When a birthday is missing, omit calendar-derived ages rather than guessing.
Any future partial-birthday support must propagate that uncertainty too.

Refining an estimate must update derived age/date matches. Preserve the
distinction between the family's assertion and a computed interpretation.

### Optional "Help estimate"

The manual workflow is complete on its own. A later, explicitly invoked helper
may propose a range from a selected photo and information the family supplies.

Illustrative flow:

1. The user identifies the person: "This is my mom when she was young."
2. They may add context: her birth year, a known school year, or "before they
   moved in 1970." Show which existing facts would be used.
3. The helper suggests an age/date range and a brief explanation of the clues
   and limitations. Example: "Estimated age 6–9; approximately 1968–1971."
   These are illustrative values, not an accuracy promise.
4. Offer Use this estimate, Adjust, and Leave undated.
5. Only an explicit acceptance writes the proposed estimate to the record.
   Keep it labeled AI-suggested even after acceptance.

Do not infer identity from the image. The user supplies the person.
Appearance, clothing, and image quality can be misleading; the helper can say
there is not enough evidence. Do not manufacture narrow ranges or numeric
confidence scores. User-provided constraints must be honored, and conflicting
evidence should trigger a question rather than a silent override.
Never overwrite existing metadata or family-supplied facts automatically.

Avoid ambient analysis, automatic backfills, or processing the whole library
for this feature. It operates only on selected records at the user's request.

No provider is selected. Evaluate a small, consented sample of historical
photos with known dates before deciding whether the estimates are useful.
Check range coverage, misleading precision, correction effort, latency, and
cost. Broad age estimates may be the honest limit of the feature.

If an external service is considered, it is an explicit exception to local
processing: disclose what photo and contextual information will be sent and
obtain an affirmative choice before sending it. Do not send unrelated records.
Provider retention and data handling need evaluation before integration.
A local model is an option only if its quality and operating cost justify it.

The app must work normally with the helper absent, disabled, or unavailable.
Removing an integration later must leave accepted records readable and editable.

### Delivery sequence

1. Define date/age semantics, serialization, migration, and mobile contracts.
2. Ship manual photo entry/editing and honest detail labels.
3. Add History/filter behavior, unknown-date retrieval, and export/import support.
4. Add Same age matching with visible uncertainty and shared backend rules.
5. Extend the representation to remembered milestones.
6. Evaluate optional AI estimation; build only if it saves useful effort.

Validation should include year-only and unknown dates, overlapping broad
ranges, scans with recent metadata, changing an anchor's birthday, missing
birthdays, multi-person photos, rejected AI suggestions, and export/import
round trips. Existing exact-date behavior must remain intact.

## Discovery and comparison details

#### Suggested event collections

Propose a collection from photos close together in time, using shared people,
family places/cities, and semantic similarity as supporting signals. Examples
include a birthday, dance competition, or vacation. Date gaps should be the
primary boundary; similar scenes alone must not merge separate occasions.
Missing GPS, faces, or embeddings should reduce confidence, not prevent a
simple date-based proposal. Trip detection belongs here as one type of collection.

- Compute proposals in the background after uploads and relevant photo edits,
  with a versioned startup backfill. Coalesce work per family rather than
  recomputing the same collection for every photo in an upload burst.
- Show a proposed date range, a few representative photos, and the people and
  place evidence that explains the grouping. Suggested labels are editable.
- Let the family accept, rename, add/remove photos, split, merge, or dismiss a
  proposal. Nothing becomes a recorded event automatically. Remember dismissals
  and preserve manual membership choices when analysis reruns.
- Link an accepted collection to an existing activity event when appropriate;
  otherwise define a historical photo collection, not a calendar or scheduling
  feature. Evaluate existing entities before introducing a new one.
- Derive membership only from the owning family's photos. Linked viewers get
  only the members they can access; GPS/place evidence stays owner-only.
  Public sharing is a separate, explicit choice.
- If persisted as a new entity, cover export/import, photo deletion, person
  merge/deletion, and cross-family isolation before shipping. Approximate dates
  should be honored once that foundation is available.

**Acceptance examples:** a birthday burst yields one editable proposal; photos
without GPS still group by date; two competitions weeks apart stay separate;
a dismissed or manually corrected collection stays corrected after backfill.
**Estimated complexity:** medium.

#### Sibling comparison montages

Extend the existing per-person growing-up montage and Same age views to place
siblings side by side: "everyone at six months", "everyone at age five", or a
recurring occasion such as the first day of school.

- Reuse `GetPersonPhotoInsights`, face positions, confirmed people tags, and
  quality scores. For age comparisons, choose within an explicit age window
  rather than assuming a monthly bucket's cover was taken at the exact age.
- For recurring occasions, combine tags or semantic search with year/date
  windows. Clearly distinguish age comparisons from event comparisons.
- Show each person's actual age and photo date, allow a replacement photo,
  and leave a labelled gap when there is no suitable photo. Do not imply
  siblings reached a milestone at the same age merely because pictures look alike.
- Work with manually tagged photos when face analysis is unavailable; use a
  full-photo fallback when there is no trustworthy face crop. Allow the family
  to choose a crop or confirm a proposed person match.
- Keep person selections and photo access checks consistent with Same age.
  Approximate dates must retain their uncertainty labels rather than displaying
  a precise inferred age. Sharing/export remains an explicit later action.
- Start as a view over existing data. Only persist montage selections if
  manual choices need to survive revisits; add export/import if those become
  records. Background precomputation is optional if measured page cost warrants it.

**Status:** the age comparison is built as a montage on the Same age page.
Each row carries `portraits`, the window's photos ranked by the growing-up
portrait score less a distance penalty. Tiles show the actual age and date,
people without a photo are listed as a gap, and "Another photo" cycles picks
in page state only. Recurring occasions and saved replacements remain.

**Acceptance examples:** siblings appear at comparable ages with actual ages
visible; a missing month shows a gap; manual replacements survive if saved;
a linked viewer never sees an inaccessible sibling or photo.
**Estimated complexity:** low to medium.


## Sharing outside the family


Two ways to hand part of the record to people who aren't in the app. Both are
new read paths with no session behind them, and they share the public photo
derivative path, so they're planned together.

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

## Optional captions and summaries


Some things local embedding models can't do well: captions, or a monthly
"this month in the family" summary written from milestones, tags, and places.
A local vision-language model (Florence-2, Moondream) could cover captions at
a higher CPU cost. An API model would do both better but breaks the
local-only principle, so it would have to be opt-in per family and send only
text, not photos. Revisit after the embedding features are in use.
