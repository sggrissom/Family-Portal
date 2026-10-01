# Offline analysis

Face tagging showed that a local model, run in the background after upload,
can do useful work nobody wants to do by hand. This plan extends that pattern
to more of the data: auto tags for photos, natural-language photo search,
duplicate grouping, places from GPS, and categorization and matching for
milestones.

The constraint is the same one face tagging was built under: everything runs
locally, on the VPS, with no third-party API. Photos of kids don't leave the
box. An LLM pass is noted at the end as an option, not a dependency.

## What we already have

- **A daemon pattern.** `cmd/faceanalysis` runs dlib behind a build tag as its
  own systemd unit (`internal@family-face`), and the app talks to it over
  `/run/family-face/face.sock`. The heavy native dependency stays out of the
  app binary, and the app degrades cleanly when the daemon is down.
- **A worker pipeline.** `photo_worker` resizes and converts; on success it
  queues `photo_analysis_worker`, which calls the daemon and writes results.
- **Backfill by version.** `Image.AnalysisVersion` marks which photos were
  analyzed with the current pipeline, and a sweep re-queues older ones.
- **A review model.** `PhotoPerson.AutoTagged` and the `/faces` page let people
  confirm or reject what the machine suggested instead of trusting it blindly.
- **EXIF parsing.** `extractExifDate` in `backend/photos.go` reads the date
  and nothing else.
- **Tags** (name, color) on photos and milestones, and six fixed milestone
  categories (`MILESTONE_CATEGORIES` in `frontend/lib/milestoneHelpers.ts`).

Everything below reuses those pieces rather than adding new ones.

## Tier 1: pure Go, no daemon

These run inside `photo_worker` right after resize. No new infrastructure.

### Near-duplicate and burst grouping

Compute a perceptual hash (dHash or pHash, e.g. `goimagehash`) and store it on
`Image`. Photos within a small Hamming distance, taken close together in time,
form a group. The photos grid shows a group as a stack, with the best-scoring
photo on top (see [quality](#quality-scoring)), and offers "hide the rest".
Nothing is deleted automatically.

### Places from GPS

Extend EXIF parsing to read GPS coordinates. Reverse-geocode offline against
the GeoNames `cities15000` dataset (about 25k rows, small enough to embed and
search with a simple spatial index). Store coordinates and the resolved place
on `Image`.

What that enables:

- Place names on photo detail, and filtering by place.
- Suggested place tags ("Yosemite"), reviewed like auto tags.
- Named family places (home, grandparents' house) defined as a point and
  radius, which take precedence over the city name.
- Trip detection: runs of photos far from home over consecutive days become a
  suggested "trip" grouping.

This records only the location where a photo was taken, already present in
the file. Live location tracking is outside the product scope. Coordinates should
still be visible only to the family, and stripped from any export or share
that leaves it.

### Quality scoring

A blur score (variance of the Laplacian on the downscaled image) and an
exposure score from the histogram. Used to pick the top of a burst, choose
cover photos, and rank suggestions below. Not shown as a number anywhere.

## Tier 2: a vision and text embedding daemon

A second daemon, `cmd/visionanalysis`, following `cmd/faceanalysis` exactly:
build tag, its own systemd unit, a socket under `/run/`, and an app-side
client that tolerates it being absent. It hosts two models via ONNX Runtime:

- **An image-text model** (OpenCLIP ViT-B/32 or SigLIP-base). Images and text
  map into the same vector space, so a photo can be compared against any
  phrase.
- **A sentence-embedding model** (`all-MiniLM-L6-v2`, about 22M parameters)
  for milestone text.

Endpoints: embed an image, embed a list of texts. The app stores the vectors
and does all comparison itself, so the daemon stays stateless like the face
daemon.

Measured cost: about 800 MB resident and 174 ms per image on staging's CPU
(see [prototype results](#prototype-results)).

### Storage

One image embedding per photo (512 or 768 float32s) in its own bucket keyed by
photo ID, packed with the existing `packFloat32Slice` helper. At family scale
(tens of thousands of photos) brute-force cosine similarity in memory is fast
enough; no vector index needed.

Replace the single `AnalysisVersion` with a version per analyzer (face, hash,
place, embedding) so each one backfills independently when it changes.

### Photo search

Embed the query text and rank photos by similarity. Combine with the existing
filters, especially people from face tags: "at the beach" plus person = Jack
is much sharper than either alone. This is the highest-value feature here for
the effort, and it directly serves the redesign's "finding something
remembered" job.

### Auto tags

Keep a list of prompt phrases per suggested tag ("a birthday cake with
candles" → Birthday, "a child at the beach" → Beach, snow, pool, bath, park,
first day of school, holiday tree, and so on). A photo scoring above a
threshold gets the tag as a suggestion, marked auto-tagged, and it goes
through a review screen like `/faces`. Families can add their own tags with a
phrase. Thresholds are tuned per prompt against the seed set.

### Milestone photo suggestions

When a milestone is saved, take photos of that person from about two weeks
either side of the milestone date and rank them by similarity to the
milestone text. "First bike ride" should surface the bike photos. Show them as
"Attach these?" on the milestone, never attach automatically.

### Better duplicate grouping

Embedding similarity catches near-duplicates that the perceptual hash misses
(slightly different framing, a moved subject). Use it to widen the groups
from Tier 1.

## Milestones

Using the sentence-embedding model:

- **Suggested category.** Embed a short description of each category and
  pre-select the nearest one when a milestone is entered. This matters most
  on iOS, where entry is fastest when fewer fields need touching.
- **Matching across siblings.** "First steps", "started walking", and "took a
  few steps today" land close together. Pairing siblings' milestones by
  similarity lets the same-age views say "Emma walked at 11 months, Jack at
  13" without anyone phrasing the milestone the same way twice.
- **Semantic milestone search.** `SearchMilestones` ranks by similarity in
  addition to the current text match.
- **Ages in the text.** Plain rules, no model: parse phrases like "at 14
  months" and use them to suggest or cross-check the date.

## Built on face data

No new models needed:

- **Best photo per person per month**, from face size, face tags, and quality
  score. Feeds a growing-up montage and a default profile header.
- **Who appears with whom.** Co-occurrence counts from `PhotoPerson`, for
  things like "photos of Grandma and Jack together" and how recently.

## Principles

- **Suggest, don't decide.** Everything machine-generated is marked as such
  and reviewable, the way `AutoTagged` is. Rejections are remembered so the
  same suggestion doesn't come back.
- **Degrade quietly.** If a daemon is down or disabled (as face tagging is
  locally via `cfg.EnableFaceTagging`), uploads and pages work normally and
  the analysis catches up later.
- **Local only.** No photo or milestone text leaves the server.
- **Backfill by version.** Changing a model or prompt list bumps that
  analyzer's version, and the sweep reprocesses in the background.

## Prototype results

Measured 2026-09-28 with `cmd/visionanalysis` (build tag `visionanalysis`),
which loads OpenAI CLIP ViT-B/32 and all-MiniLM-L6-v2 as ONNX exports through
ONNX Runtime 1.29 (`github.com/yalue/onnxruntime_go`). Tokenizers and CLIP
preprocessing are pure Go in `vision/`; they reproduce the Hugging Face
tokenizers token for token, and the Go image embeddings match a Python
reference pipeline at cosine ≥ 0.998.

The runtime is loaded with `dlopen`, so the daemon builds anywhere with cgo
and needs no native libraries at build time. `libonnxruntime.so` is the
official Microsoft build (glibc 2.28 and up), fetched into the models
directory by `scripts/fetch-vision-models.sh` along with the models, all
pinned by checksum. The same binary and library run on both boxes, so unlike
the face daemon there is nothing to build on the server.

| | laptop (i5-8350U) | staging (E5-2697 v2, AVX only) |
| --- | --- | --- |
| image embedding, 2 threads | 76 ms | 174 ms |
| image embedding, 1 thread | | 321 ms |
| CLIP text, batch of 20 prompts | 99 ms | 239 ms |
| MiniLM, batch of 5 sentences | 7 ms | 20 ms |
| resident memory | 830 MB | 790 MB |

Production's CPU (Xeon Gold 6240R, AVX-512) is newer than staging's, so
staging is the pessimistic case. At 174 ms an image, a 20,000-photo backfill
is about an hour of two cores.

**Quantized models were rejected.** The int8 exports cut memory to 265 MB,
but their embeddings sit at cosine 0.84–0.92 from the fp32 ones, top-1
labels flipped on 3 of the 16 seed photos, and on staging's AVX-only CPU they
were slower (217 ms) rather than faster. The daemon uses fp32; 800 MB fits
on both boxes (staging has about 3 GB available, production about 5.5 GB).

**Quality.** Zero-shot over the 16 seed photos against 20 prompts put the
intended label first on 15; the swaddled newborn scored "portrait" 0.274
against "newborn" 0.269. On 251 COCO validation images, labelled by
their captions:

- Search is good. Precision at 5 was 1.0 for "at the beach", "snow",
  "riding a bike", "cake", and "cooking in the kitchen", 0.8 for "birthday",
  "dog", and "christmas", and 0.6 for "baby".
- Auto-tag thresholds have to be per prompt, as planned. The cosine at which
  precision reaches about 0.9 ranged from 0.24 (snow, bike) to 0.28
  (birthday); a single global threshold of 0.26 gave precision 0.6–1.0 with
  recall between 0.1 and 1.0. Scoring against a neutral "a photo" baseline
  did not help.
- MiniLM separates milestone phrasings, but with modest margins: "First
  steps" scored 0.53 against "took a few steps today" and 0.34 against
  "Started walking", versus 0.24 for "First word: mama" and 0.19 for "Lost
  first tooth". Sibling matching should pair within a category and take the
  best match, not rely on a fixed cutoff alone.

SigLIP was not tried. It uses a SentencePiece tokenizer, which would need
its own Go port, and CLIP's results leave no gap that clearly calls for it.

Tier 2 is viable on both boxes with the fp32 models. To reproduce:

```bash
scripts/fetch-vision-models.sh ~/vision-models
make build-vision
build/family-vision -models ~/vision-models -bench backend/seedphotos -prompts prompts.txt
```

where `prompts.txt` has one `Label: phrase` per line.

## Tier 1 as built

- Results live in a `PhotoFeatures` record per photo rather than on `Image`,
  so coordinates never ride along in the `Image` JSON that every photo list
  returns. The hash, quality, and place analyzers each store a version;
  `Image.AnalysisVersion` stays as the face analyzer's.
- `photo_worker` queues a photo for features after resizing, and a backlog
  worker (`backend/backlog_worker.go`) runs them one at a time, starting with a
  sweep for anything missing or outdated.
- Similar photos: a 64-bit dHash. Two photos group when they are within 3
  bits anywhere in the family (re-uploads, edits) or within 12 bits and 2
  minutes of each other (bursts). Groups are stored as a group id and merged
  when a new photo bridges two. `ListFamilyPhotos` with `collapseSimilar`
  returns each group's best-quality photo with the rest in `similar`; the grid
  shows it as a "+N similar" stack that opens as a sequence in the viewer, and
  the “Show similar photos separately” option in Filters turns stacking off. A filtered-out cover never hides a
  member that matches. Groups are not split when a photo is deleted or its date
  edited, which only means a stale burst can stay stacked.
- Places: EXIF GPS resolved against GeoNames `cities15000` (34k places, 640 KB
  gzipped, embedded). The dataset has towns, not parks, so "Yosemite" comes
  from a family place, not the lookup. Family places are a point and radius,
  created from a photo's location on the photo page ("Name this place") and
  managed in Settings; the tightest one containing a photo wins over the city.
  Coordinates and place names are returned only to members of the family that
  owns the photo, not to linked families. The family's own data export still
  carries the untouched originals, EXIF included.
- Quality: Laplacian variance for sharpness, mean brightness and clipping for
  exposure, combined 0.7/0.3. Used only to pick a stack's cover.
- Not built yet: suggested place tags (they belong with the auto-tag review
  screen in step 5) and trip detection.

## Order of work

1. ~~**Prototype the vision daemon locally.**~~ Done; see
   [prototype results](#prototype-results).
2. ~~**Tier 1.**~~ Done; see [Tier 1 as built](#tier-1-as-built).
3. ~~**Vision daemon in production**~~, with embedding backfill. Done: running
   on staging since 2026-09-28; see "Vision analysis" in
   [deployment](../deployment.md). Production needs `make vision-models` and
   the unit set up with `DEPLOY_HOST=prod`.
4. ~~**Photo search.**~~ Done. `ListFamilyPhotos` takes a `query`: names of
   visible people in it become an all-of person filter ("Maeve at the park"),
   the rest is embedded as "a photo of …" and ranks the photos that pass the
   other filters. A result must score at least 0.23 and within 0.05 of the
   best match (calibrated on the seed photos, where absent subjects top out
   near 0.22); a title or description containing the words always matches.
   Without the daemon, search falls back to titles and descriptions and says
   so. Family vectors are cached in memory and dropped on any embedding write.
5. ~~**Auto tags**~~ with a review screen. Done. Absolute per-prompt cutoffs
   turned out brittle (on COCO a single threshold ranged from useless to
   overcautious by label), so a suggestion is a zero-shot classification
   instead: a softmax (CLIP's logit scale, 100) over a built-in catalog of 16
   labels, the family's own tag phrases, and 12 neutral background prompts
   ("a portrait photo", "a photo of people indoors", …). A label is suggested
   when it takes more than half the probability and its cosine is at least
   0.24. On the seed photos that suggests Dance, Sports, and Newborn where they
   belong and nothing for the portraits. A catalog label reuses a family tag
   of the same name, or creates one on first acceptance. Families describe
   their own tags on the tags page ("kids at the lake cabin"); changing a
   phrase rescores the family's photos. Suggestions live in `/suggestions`
   (grouped, click to exclude, accept or reject in bulk) and as dashed chips
   on the photo page. Suggested place tags were dropped: family places and the
   place filter already do that job without duplicating it as a tag.
6. ~~**Milestone features:**~~ Done. Milestones get a MiniLM embedding from
   their own backlog worker (re-embedded when the text changes).
   - Suggested category: a family milestone at similarity 0.6 or more lends
     its category (the family's own convention wins); otherwise the closest of
     a few example phrasings per category. 65% agreement with the seed
     templates' labels, whose misses are mostly genuinely ambiguous ("First
     real smile" filed under development). It only pre-selects a chip the
     person hasn't touched, on the web form and via `SuggestMilestoneCategory`
     for iOS.
   - Photo suggestions: `SuggestMilestonePhotos` takes the person and the same
     date fields as `AddMilestone` and returns up to 8 of their photos from two
     weeks either side, ranked by CLIP against the description (by nearness
     in time without the daemon). The add form shows them as "Photos from
     around then", unattached until tapped.
   - Matching: `GetMilestoneMatches` pairs a milestone with each visible
     person's closest milestone when each is the other's best match and the
     similarity is at least 0.45. On twelve paraphrase pairs ("First steps" /
     "took a few steps today", "Potty trained" / "out of diapers") the
     mutual-best rule got all twelve; true pairs scored 0.40 to 0.77 and the
     runner-ups at most 0.40. The milestone page lists them with each
     person's age ("Clara at 14 mo").
   - Search: `SearchMilestones` blends word matches with milestones at 0.35
     or more by meaning ("walking" finds "Took five steps").
   - Ages in the text: `parseAgeFromText` (web only) offers to record the
     milestone at an age the description states ("at 2.5 years", "an
     18-month-old"), ignoring durations like "for 2 weeks" or "two years
     ago".
7. ~~**Face-derived features**~~. Done: `GetPersonPhotoInsights` walks a
   person's photos once and returns
   - a growing-up montage, the best photo per month under two and per year
     after (per calendar year without a birthday), scored by the person's
     face size (0.45), photo quality (0.35), how few people share the photo
     (0.2), and a bonus for a confirmed face. The Photos tab shows it as face
     crops labelled by age;
   - who they are photographed with, by count then recency, linking to a
     photo search for the two of them;
   - a header photo: the best face from the last 18 months, shown as a face
     crop when the person has no profile photo set.

## Status

All seven steps are built (PRs #104 through #110, stacked). Staging runs the
whole stack with the vision daemon. Production has neither the daemon nor the
models yet; see "Vision analysis" in [deployment](../deployment.md). The edit-milestone
form suggests photos too (not a category, since one is already chosen). Left
for later: trip detection and the LLM pass below.

## Next proposals: collections and sibling montages

These are planned ideas, not part of the seven shipped steps above. Both use
existing local analysis results; neither requires an LLM or an external API.

### Suggested event collections

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

### Sibling comparison montages

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

**Acceptance examples:** siblings appear at comparable ages with actual ages
visible; a missing month shows a gap; manual replacements survive if saved;
a linked viewer never sees an inaccessible sibling or photo.
**Estimated complexity:** low to medium.

## Later: an LLM pass

Some things local embedding models can't do well: captions, or a monthly
"this month in the family" summary written from milestones, tags, and places.
A local vision-language model (Florence-2, Moondream) could cover captions at
a higher CPU cost. An API model would do both better but breaks the
local-only principle, so it would have to be opt-in per family and send only
text, not photos. Revisit after the embedding features are in use.
