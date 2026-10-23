# Video analysis

Videos shipped as sealed boxes. Every analyzer the photos go through has an
explicit `IsVideo` guard (embeddings, tag suggestions, features, face
analysis), deliberately, because none of them knew what to do with a clip. So
today a video is findable only by its title and description, gets no tag
suggestions, no face tags, and nothing hears what anyone says in it. For the
clips the video plan was written for — a kid explaining something at length,
a recital, a first walk — what's said is often the whole point.

This plan brings video up to what photos already get from local analysis, and
adds the one thing photos never needed: listening. It follows the offline
analysis that photos went through (the vision daemon, tag suggestions,
milestone matching), under the same rules:

- **Local only.** Frames, faces, and audio never leave the box. That matters
  more here than for photos: what children say at home is more private than
  what they look like. No transcription API, now or as an option.
- **Suggest, don't decide.** Tags and people from a video are suggestions that
  go through the existing review screens. A transcript is labelled as automatic
  until someone edits it, and edits are never overwritten.
- **Degrade quietly.** Playback never waits on analysis. Each analyzer that is
  missing (no vision daemon, face tagging off, no speech model) skips its part
  and catches up when it appears.
- **Backfill by version.** Each part has its own version, so changing one
  re-runs only that part.

## What already exists

- **The video worker** (`backend/video_worker.go`) probes, remuxes, cuts a
  poster at one second, pushes the poster through the photo pipeline, and
  publishes. ffmpeg is exec'd with bounded runs (`mediaTools.run`), and
  `localCopyOf` gets a file path for local or bucket storage, with
  `diskBudget.reserve` covering the scratch copy.
- **The vision daemon** (`cmd/visionanalysis`) embeds images
  (`/embed/image`) and text with CLIP or MiniLM (`/embed/text`). It is
  stateless and knows nothing about photos, so frames are just more images to
  it.
- **The face daemon** recognizes faces in any image bytes it's sent.
- **Videos are `Image` rows.** `TagSuggestion`, `PhotoPerson`, `PhotoFace`, and
  milestone `PhotoIds` all key on the image id, so the `/suggestions` and
  `/faces` review screens and people tagging already accept a video. They only
  need something to produce suggestions for one.
- **Search.** `PhotoWordIndex`, maintained by `ReindexPhotoDates`, gives word
  matches on title and description, and video rows are already in it.
  Semantic ranking uses `familyVectors` (one CLIP vector per photo) with the
  0.23 floor and 0.05 window from `backend/photo_search.go`.
- **`backlogWorker`** with `errRetryLater` backoff and a startup sweep, used by
  every analysis worker.

The guards that stay put are portrait ranking, person insights, Same age, and
book layouts. Analysis makes a video easier to find and tag. It does not make it
a portrait.

## What a video can tell us

Three kinds of signal, roughly in order of cost:

1. **How it looks**: sampled frames through CLIP. Search, tag suggestions,
   moment suggestions.
2. **Who's in it**: faces in those frames, as people suggestions.
3. **What's said**: speech-to-text. A transcript, captions, and search over
   the words.

Out of scope, with reasons:

- **Video-native models** (X-CLIP, VideoMAE, action recognition). CLIP over
  frames already handles searches like "blowing out candles" or "at the pool".
  Video models are heavier and have no ONNX-plus-Go path like the one CLIP has.
  Revisit only if frame search clearly misses actions.
- **Speaker diarization** (who said what). It's unreliable on small children
  and overlapping voices, and a wrong "Maeve said" is worse than none.
- **Sound events** (laughing, singing, applause) are a good fit for tags but
  need a separate audio model. Listed under Later.

## Step 0: the poster as a photo

The poster is already an ordinary image at `FilePath`. Let `embedPhoto`,
`photosNeedingEmbedding`, and `suggestTagsForPhoto` take ready videos
(`Status == 0`) and use the poster. That gives videos semantic search and tag
suggestions with no new infrastructure, in one small PR.

It's one frame, taken one second in, so it is weak evidence. Step 1 replaces it
as the main signal, but the poster embedding stays as the video's single
`PhotoEmbedding` so code that expects one vector per image keeps working.

Check while doing it: `SuggestMilestonePhotos` will start offering videos, and
the moment page should render an attached video with its badge and player
link. Embedding coverage in `photo_maintenance.go` will start counting videos.

## Step 1: sampled frames

A new `videoAnalysisWorker` (a `backlogWorker`), separate from the processing
worker so publish never waits on it. The processing worker queues a video when
it publishes, and the startup sweep picks up ready videos whose parts are
outdated. It runs one job at a time, niced, and reserves disk the way the
processing worker does. It reads the playback remux rather than the original,
since the remux is what's known to decode.

Sampling:

- One frame every `max(5s, duration/48)`, so at most 48 frames for a 10-minute
  clip. Add scene-change frames (`select='gt(scene,0.3)'`) under the same cap,
  then drop a frame whose CLIP vector is within 0.95 cosine of the previous kept
  frame. A static shot of a recital shouldn't use up 48 frames.
- HDR clips go through the same `hdrTonemap` the poster uses.
- Embed each frame at `embedSourceSize`. At 174 ms a frame on staging, a full
  10-minute clip costs under ten seconds of CLIP, which is small next to the
  remux.

Storage:

- A `VideoFrames` record per video in its own bucket: `ImageId`, `FamilyId`,
  `Model`, `Version`, and `Frames []{OffsetMs, Vector}`. It is kept off `Image`
  for the same reason `PhotoFeatures` is: every photo list returns `Image`.
- Keep each sampled frame as a small JPEG (320 px, roughly 20 KB) next to the
  poster, named from the video's media id. That's about 1 MB for a long clip,
  counted in `DerivedBytes` and deleted with the video. The frames serve as
  match thumbnails, face crops for review, and later a scrub strip.

Uses:

- **Search.** A video scores as the best of its frames, and the hit remembers
  which frame. `familyVectors` grows to hold several vectors per image, or a
  sibling cache does. The 0.23 floor and 0.05 window apply unchanged. The photo
  hit gains an optional `match` (`source: "frame" | "transcript"`, `offsetMs`,
  `snippet`). The web player honours `?t=` so a hit opens at that moment, and
  the result tile shows the matching frame instead of the poster.
- **Tag suggestions.** Score each frame with `scoreLabels` against the same
  catalog, family phrases, and background prompts. A label applies to the
  video when it wins (probability > 0.5, cosine ≥ 0.24) in at least two frames,
  or in one frame for clips under 15 seconds. The two-frame rule needs
  calibration on real clips. Results are ordinary `TagSuggestion`s, and the
  review screen shows the best-scoring frame for each suggestion.
- **Moment suggestions.** `SuggestMilestonePhotos` scores a video by its best
  frame, like search does.

## Step 2: people from faces

Only when `cfg.EnableFaceTagging` is on. Send the sampled frames to the face
daemon and match against the same references and thresholds photos use
(auto-tag below 0.5 with the 0.04 margin, suggest below 0.62).

- **Aggregate per video, not per frame.** A person is auto-tagged only when
  they match below 0.5 in at least two frames. One-frame matches on a moving,
  blurry subject are where the false positives come from, so a single strong
  frame makes a suggestion, not a tag.
- **One face per person per video in `/faces`.** Group the video's unknown
  faces with the existing complete linkage at 0.45, and keep the clearest face
  (largest box, best distance) of each person or group. Otherwise one video puts
  48 copies of the same child into review.
- `PhotoFace` gains `FrameMs` (a pack version bump), and its box is relative to
  that frame. Crops come from the stored frame JPEG.
- **Video faces are not references** at first. Motion blur and compression
  make them worse examples than photos, so only profile descriptors and
  confirmed photo faces drive matching. Revisit if confirmed video faces look
  clean.
- `imageNeedsAnalysis` and the photo face worker keep their `IsVideo` guards.
  The video analysis worker owns faces in videos and tracks them with its own
  version, so an `AnalysisVersion` bump can never send a video down the photo
  path.

## Step 3: transcripts

### Engine

**whisper.cpp, exec'd like ffmpeg.** It's a single C++ binary and one model
file, CPU only, with no Python, and it writes JSON with segment timestamps.
It makes the same deal as video processing: a missing binary or model turns
transcription off and nothing else.

Alternatives considered:

- *faster-whisper* (CTranslate2) is quicker on CPU with int8, but it brings a
  Python environment onto the boxes. The face daemon already shows what native
  dependencies on the server cost.
- *Whisper on ONNX Runtime inside `family-vision`* would reuse the runtime
  that's already shipped. But the decoder loop, timestamp tokens, beam search,
  and byte-level BPE would all be Go ports, which is several times the work
  the CLIP tokenizer took, to save one binary.
- *Vosk* is light but noticeably less accurate, and accuracy on kids' speech
  is already the weak point.

Building it: the vision daemon got away with one binary for both boxes, but
ggml chooses CPU features at build time, and staging is AVX only while
production has AVX-512. Either build on each box, as the face daemon does, or
ship a portable build with runtime CPU dispatch if the prototype shows it isn't
much slower. The prototype decides.

### Model

The prototype chooses among `base.en`, `small.en`, `small` (multilingual), and
a quantized `large-v3-turbo`. whisper.cpp's own figures put `small` at about
850 MB of memory. It runs as a short-lived process, one at a time, beside the
vision daemon's 800 MB resident, and staging has about 3 GB available. The
10-minute cap bounds the worst case. My expectation, to be checked, is that
`small.en` is the minimum usable size for children's speech and that turbo is
too slow on staging's CPU.

### Pipeline

1. Skip the video if it has no audio (`AudioCodec == ""`) and record it as
   having no speech.
2. `ffmpeg -vn -ac 1 -ar 16000 -c:a pcm_s16le` writes a WAV into scratch,
   about 19 MB per 10 minutes, included in the disk reservation.
3. Run `whisper-cli` with the model, JSON output, two threads, niced, and a
   timeout scaled to the duration (around 3× the duration plus a minute,
   adjusted after the prototype). Use voice activity detection if the pinned
   version supports it.
4. Parse the segments and apply the guards below, then write the result.

Whisper's known failure modes, and the guards against them:

- **Hallucinating on silence or music.** Whisper invents text such as "Thanks
  for watching!" or the same line over and over. Use VAD, drop segments with a
  high no-speech probability or a low average log-probability, drop runs of
  repeated segments, and drop isolated stock outro phrases. A recital with no
  speech should produce *no transcript*, not a made-up one.
- **Toddlers.** Speech under about age three will often be wrong. That's
  acceptable because a transcript is a search aid, not a record. Keep
  per-segment confidence and show low-confidence segments dimmed.
- **Language.** Detect it and store it. Search folding and MiniLM assume
  English. A transcript in another language is still stored and shown; it just
  searches less well.

### Storage

A `VideoTranscript` bucket keyed by image id: `FamilyId`, `Model`, `Version`,
`Language`, `Status` (done, no speech, failed), and
`Segments []{StartMs, EndMs, Text, Confidence}`, plus `EditedBy` and
`EditedAt`. It is separate from `Image` for the same reason frames are.

**Edits are permanent.** Anyone who can edit the video can correct a segment's
text. An edited transcript is never overwritten: version bumps and reruns skip
it. A "Remove transcript" action deletes it and marks the video so it isn't
transcribed again.

### Showing it

- **Web.** A transcript panel under the player on the media page: segments with
  timestamps, a click seeks, and the current segment is highlighted. It is
  collapsed by default with "Automatic transcript — may contain mistakes"
  until someone edits it. Captions are served as WebVTT generated on request
  from the segments by an authenticated app handler and attached as
  `<track kind="captions">`, off by default. The track is served from the app's
  own origin, so it still works when playback comes from a presigned bucket
  URL.
- **iOS.** AVPlayer only reads WebVTT from inside HLS, and playback is
  progressive MP4, so the app draws captions and the transcript itself from the
  same segments API. The web design is the reference, as usual.

### Search

- **Words.** Add the transcript text to the fields `ReindexPhotoDates` feeds to
  `PhotoWordIndex`, leaving out `searchFillerWords`, since queries drop those
  anyway and transcripts are full of them. The existing rule extends: any
  transcript write calls `ReindexPhotoDates`. The hit's snippet comes from the
  first segment where `termsMatch` succeeds, along with its `offsetMs`. A
  10-minute clip has about 1,500 words and a few hundred distinct ones, which
  is fine at family scale.
- One quirk: the query parser turns names into person filters, so "Maeve
  dinosaur" means a video *tagged* with Maeve whose words include dinosaur.
  That's usually what's meant. Don't special-case it.
- Access needs no new checks. A transcript hit is a hit on the video, so
  `photoListing.passes` and `CanAccessPhoto` already decide who sees it.

## Step 4: meaning in what's said

- **Semantic transcript search.** Embed overlapping windows of about 30
  seconds with MiniLM and keep the vectors with the transcript. A query is
  also embedded with MiniLM and matches at 0.35 or more, the threshold moment
  search uses, so "talking about dinosaurs" finds "the T-rex is my favorite".
  Search then has two vector spaces, CLIP for how a video looks and MiniLM for
  what's said. Keep their thresholds separate, merge the hits, and label each
  one by its source.
- **Moments from speech.** `SuggestMilestonePhotos` adds transcript similarity
  for videos in the date window, so "First word: mama" can surface the clip
  where it was said. It suggests, and never attaches.

## Export, deletion, and access

- **Export.** Each exported video carries its transcript, automatic or
  edited, flagged as which. It's text the family may want even without the
  app. Frames and frame vectors are derived and are not exported. Import does
  not restore videos yet; when it does, it should restore edited transcripts
  and queue the rest for analysis.
- **Deletion.** Deleting a video removes its frames record, frame files,
  transcript, `PhotoFace`s, and suggestions. Deleting during analysis must not
  leave any of them behind; check before the final write, as the processing
  worker checks before publish.
- **Person merge and delete.** `PhotoFace` and `PhotoPerson` handling already
  covers people. Confirm it with a video in the test.
- **Isolation.** A cross-family test confirms transcript words and frame
  vectors never match another family's search.

## Operations

- The admin health view gets queue length, per-part counts (frames, faces,
  transcript), and the last errors, like the video processing stats.
  `docs/degraded-dependencies.md` gets a row for each part.
- Gates: frames need the vision socket, faces need `cfg.EnableFaceTagging`,
  transcripts need the whisper binary and model (new config paths). A part
  that's unavailable leaves its version outdated rather than recording a
  failure, so it backfills once the dependency appears. A daemon that's down
  returns `errRetryLater`.
- Deployment: binary and model under `shared/models/whisper`, fetched and
  checksum-pinned by a script like `scripts/fetch-vision-models.sh`, plus a
  `make` target and a section in `docs/deployment.md`.

## Prototype results

Measured 2026-10-22 on the laptop (i5-8350U, AVX2) with whisper.cpp v1.9.5,
two threads, on public speech samples (a 127-second radio address, a
30-second fast-talking ad) plus a minute of silence and a minute of synthetic
tones. Real family clips were not available, so quality on children's speech,
real music and wind is still to check.

| model | decoding | time / clip length | peak memory |
| --- | --- | --- | --- |
| base.en | greedy | 0.18 | 340 MB |
| base.en | beam 5 | 0.27 | 380 MB |
| small.en | greedy | 0.58 (0.51 with VAD) | 735 MB |
| small.en | beam 5 | 0.82 | 850 MB |
| small | greedy | 0.61 | 735 MB |
| large-v3-turbo q5_0 | greedy | 2.75 | 830 MB |
| large-v3-turbo q5_0 | beam 5 | 3.00 | 900 MB |

- **Model: `small.en`, greedy, with Silero VAD.** Turbo would take about 27
  minutes for a 10-minute clip on the laptop, and staging is slower. Beam
  search cost 40% more time for this use. `base.en` is three times faster, so
  it stays the fallback if staging turns out too slow.
- **Silence and music.** Without VAD both models emitted only annotations
  (`[BLANK_AUDIO]`, `(soft music)`, `(wind blowing)`), which the transcript
  cleaner drops. With VAD they emitted nothing and finished in under a second.
- **Build.** A portable build (`GGML_CPU_ALL_VARIANTS`, picked at run time)
  includes Ivy Bridge (staging) and Cascade Lake (production) variants and was
  about 10% slower than a native build. It has to be built on a server: the
  laptop's glibc is newer than the boxes'. `scripts/install-speech.sh` does
  this.
- **Frames.** `family-vision -frames DIR -prompts FILE` reports kept frames
  and poster against best-frame scores per prompt. It has not been run on real
  clips yet. The two-frame tag rule and the 0.95 dedupe threshold are
  uncalibrated.

Still to measure: real-time factor on staging, and transcript and frame
quality on real family clips.

## Order of work

0. **Poster through the existing analyzers.** One small PR.
1. **Prototype bench.** Done for speed and silence; real-clip quality is still
   open (see Prototype results).
2. **Video analysis worker and frames.** Frame search with `match`, `?t=` deep
   links, frame tag suggestions, moment suggestions by frame.
3. **Faces from frames.**
4. **Transcripts.** Worker part, storage, panel, captions, word search with
   snippets, editing and removal, export.
5. **Semantic transcript search and moment suggestions from speech.**
6. **iOS.** Transcript panel, captions, search snippets and deep links.

Steps 2–5 are a cascaded PR stack, merged bottom-up.

## Acceptance checks

- A video uploaded while the vision daemon is down plays as soon as it's
  processed and is analyzed later. Deleting it mid-analysis leaves no frames,
  transcript, faces, or suggestions.
- "candles" finds a birthday clip whose poster shows no cake, and opens it at
  the cake. "dinosaur" finds the clip where it's said and shows the line.
- A music-only recital gets no transcript and no hallucinated text.
- An edited transcript survives a version bump and an export/import round trip.
  A removed transcript stays removed.
- One video adds at most one face per person to `/faces`.
- A linked viewer without access to a video never gets a hit from its frames
  or transcript.
- Photo paths are unchanged: portraits, Growing Up, Same age, and books still
  exclude video, and an analysis version bump never sends a video to the photo
  face worker.

## Open questions

- **Transcription on by default?** Recommended: on for any family with video
  enabled, with per-video removal. It's local, and it reaches the same people
  who can already watch the video. A family-level off switch is cheap if
  wanted.
- **Do linked families see transcripts?** They can already watch and listen,
  so probably yes, but making a family's speech text-searchable is a step
  beyond letting them watch it.
- **Should video faces become references** once confirmed ones look clean?

## Later

- **Sound events.** Laughing, singing, applause, music, crying as tag
  suggestions from an audio model (YAMNet or CLAP as ONNX in `family-vision`).
- **Chapters and a scrub strip** from scene-change frames.
- **Suggested titles** from the transcript and tags, as part of the LLM pass on
  the roadmap. Locally only, under the same rule.
- **Interview prefill.** An annual interview recorded on video gets its
  transcript offered as draft answers for review.
