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

Expected cost: a few hundred MB resident, a few hundred ms per image on the
VPS CPU. Prototype locally against the seed photos to confirm before
committing (see [order](#order-of-work)).

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

## Order of work

1. **Prototype the vision daemon locally.** Run CLIP or SigLIP over the seed
   photos, measure CPU time and memory, and check auto-tag precision on a
   handful of prompts. This decides whether Tier 2 is viable on the VPS
   before anything is built around it.
2. **Tier 1.** Perceptual hash, GPS and places, quality score, and per-analyzer
   versions. Useful on its own and independent of the prototype's outcome.
3. **Vision daemon in production**, with embedding backfill, deployed like the
   face daemon.
4. **Photo search.**
5. **Auto tags** with a review screen.
6. **Milestone features:** suggested category, photo suggestions, sibling
   matching.
7. **Face-derived features**, as the redesigned person page needs them.

## Later: an LLM pass

Some things local embedding models can't do well: captions, or a monthly
"this month in the family" summary written from milestones, tags, and places.
A local vision-language model (Florence-2, Moondream) could cover captions at
a higher CPU cost. An API model would do both better but breaks the
local-only principle, so it would have to be opt-in per family and send only
text, not photos. Revisit after the embedding features are in use.
