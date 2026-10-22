# Roadmap

Family Record preserves who is in a family, how people grew, what they did,
and what it looked like. The product direction is to make those records easy
to capture, durable, and meaningful in context: across a person's life,
between siblings, and across generations.

Nothing here is a release commitment. Household management (calendars, chores
and allowance, meals, location tracking) is deliberately out of scope; those
proposals are kept in [Not planned](not-planned.md).

## Planned, with their own plans

| Plan | Purpose |
| --- | --- |
| [Approximate dates and ages](plans/approximate-dates.md) | Record partial historical knowledge honestly and keep the uncertainty when browsing and comparing. Manual entry first; optional AI estimation only after evaluation. The foundation for old photos and cross-generation views. |
| [Video](plans/video.md) | Videos alongside photos, with private object storage for the bytes. |
| [Video analysis](plans/video-analysis.md) | Local frame, face, and speech analysis of videos: tag and people suggestions, transcripts, and search over what's said. |
| [Sharing outside the family](plans/sharing.md) | Explicitly published family summaries and Christmas-card updates shared by link. |

## Follow-ups to shipped features

**Books.** First-year, personal year, family year and custom books are built.

- Browser print stylesheet / Save as PDF (small to medium). A server-generated
  PDF, an offline HTML package, or a professionally printed book come later,
  if demand justifies them.
- Revocable share links with frozen snapshots (medium to large). Build edition
  snapshots, preview, restricted media and revocation together; this overlaps
  with [Sharing](plans/sharing.md).
- More presets as people ask for them: traditions (every Halloween),
  whole-history books, same-age comparisons. AI may suggest choices or draft
  text for review, but must never invent events, dates or feelings.

**Same age.** Portraits and Details are built, with a newborn window of birth
through day 27.

- Recurring-occasion comparisons (first day of school, Halloween) alongside
  age comparisons: tags or semantic search within year windows, clearly
  labelled as event comparisons rather than age comparisons.
- Saved portrait choices. "Another photo" only lasts for the page today.
- A children-by-ages matrix, and montage export.

**Annual interview.** Web and iOS are built. Open questions:

- A per-family cadence setting (birthday, school start, New Year, off).
  Birthday is the only option today.
- A note field on single answers ("said while giggling").
- Recording who asked (ties into "who recorded it" below).
- Group mode: interviewing siblings together.

**Redesign.** Open product questions:

- Whether Family as a destination works, compared with keeping every stable
  entrance on Home.
- What Recently added should show, and how it pages.
- Whether measurement reminders are on by default, and their dismiss/snooze
  behavior.

Correctness bugs and rough edges in existing features go here as they're found.

## Ideas

Candidates that fit the record but aren't scoped. They're options to argue
over, not a checklist. "New entity" means the checklist in
[Adding a new domain entity](#adding-a-new-domain-entity).

### Getting more out of what's recorded

- **Suggested event collections** (medium). Propose groupings of a birthday, a
  competition or a trip from photos close together in time, with shared people,
  places and semantic similarity as supporting evidence. Date gaps are the
  primary boundary; similar scenes alone must not merge separate occasions, and
  missing GPS, faces or embeddings lower confidence rather than block a
  date-only proposal. Compute in the background per family with a versioned
  backfill. The family accepts, renames, edits membership, splits, merges or
  dismisses a proposal; nothing becomes a record automatically, and dismissals
  and manual corrections survive reruns. Link to an existing activity event
  where it fits rather than inventing a calendar. Linked viewers see only photos
  they can access, and place evidence stays owner-only.
- **Search everything.** Photos and moments have local semantic search. Extend
  unified search to activity notes, entries and events.
- **An LLM pass.** Captions, or a monthly "this month in the family" summary.
  A local vision-language model keeps everything on the host at a CPU cost; an
  API model would be better but must be opt-in per family and send text only,
  never photos.

### Richer records of people

- **Birth details.** Birth time, weight, length, place. Weight and length seed
  the growth chart at day zero, currently a hole at the most interesting end
  of the curve.
- **Head circumference.** WHO has charts for ages 0–2 and pediatricians
  measure it at every early visit. `MeasurementType` is only height and weight.
- **Nicknames and a short bio.** What they go by, what they're into right now.
  They'd also feed the public summary page in [Sharing](plans/sharing.md).
- **In memoriam.** Relations already reach grandparents. Mark someone as passed
  without deleting them, and stop listing them as current on the dashboard.
- **Family tree view.** Relations are already a graph with derived labels;
  drawing it is mostly frontend work.
- **School years.** School, grade, teacher and a class photo per year. Try it
  as an activity kind before reaching for a new entity.

### New kinds of entries

- **Documents.** Report cards and certificates go under achievement with a
  photo. Give them a category, like artwork, if that proves too loose.
- **Pregnancy record.** `IsPregnancy` exists with nothing to record against it.
  Ultrasound photos and a weekly bump series, carried into the child's record
  at birth.

### Family members contributing

- **Who recorded it.** Moments and growth don't store who added them.
  Contributors (nanny, grandparents) are a real role.
- **Reactions and comments** on a moment or photo, by family and linked
  households. Internal only, no feed.

## Adding a new domain entity

A new entity needs:

- a vbolt bucket plus the indexes to query it
- a versioned vpack pack function
- family scoping, with a cross-family isolation test
- RegisterProc handlers (the TypeScript client generates itself)
- coverage in export and import, so a family can take its data with it
- handling in person merge and delete, if the entity refers to a person

Changes to existing record formats also need compatible serialization,
documented mobile contracts, and preservation of meaning through export/import.

## Product boundaries

- No ads or third-party analytics.
- No public feed, follower graph, or discovery between families. Sharing means
  content a family explicitly chooses to hand to someone.
- Private records stay private by default. External storage, public sharing,
  or any future external AI processing require explicit choices described in
  their plans; none should silently apply to existing records.
- No AI requirement for recording or viewing family history. Suggestions are
  reviewable and never silently become facts.
- Optional dependencies must degrade cleanly. The core remains a self-hosted
  app with one database file and local images; video proposes an optional
  bulk-storage exception.
- No household management. See [Not planned](not-planned.md).
