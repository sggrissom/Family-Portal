# Approximate dates, age-based records, and periods

Status: web/backend in review as stacked PRs #230–#236 (2026-11-02); writes are
behind `APPROXIMATE_DATES=on`. Contract and inventory:
[approximate-dates-contract.md](approximate-dates-contract.md). Still to do before
enabling in prod: the iOS companion changes, raising the iOS minimum version, and
the staging acceptance run on migrated data.
Manual entry and end-to-end date semantics are the release scope. AI estimation
remains an optional later experiment, not a dependency or provider commitment.

## Problem and intended outcome

Families record both events and descriptions of a stage of life. “She loved
painting when she was 4” does not mean it happened on her fourth birthday.
Entering twenty such records must not produce twenty birthday anniversaries.
An old photo dated “around 1970” has a different problem: it captures an event,
but its occurrence date is uncertain. Support both without inventing precision.

The implementation must preserve three independent facts:

1. **Meaning:** an occurrence sometime within a window, or a description of a
   period of life. A period description need not be true every day of that period.
2. **Precision:** day, month, year, explicit range, age in years/months, or unknown.
3. **Uncertainty:** known at that precision versus estimated, with explicit,
   user-visible matching bounds for estimates.

This is a shared date/age overhaul, not a display-label change. Moments
(including quotes, artwork, and remembered milestones) and media (photos and
videos) ship together. Keeping moments for a later phase would leave the
reported birthday problem unsolved.

## Current implementation and audit starting points

Reviewed against main commit `3ecb95e` (2026-11-01). Paths below are starting
points, not a claim that every downstream consumer has already been audited.

- `backend/entry_date.go`: `resolveEntryDate` reduces age input to birthday plus
  years/months; omitted months become zero. No assertion survives in the record.
  Its tests explicitly expect age 4 to become the fourth birthday.
- `backend/milestone.go` and `backend/photos.go`: requests accept age input, but
  stored `MilestoneDate` / `PhotoDate` are single timestamps.
- `frontend/lib/when.ts`: shared input similarly defaults omitted months to zero
  and computes a single date. Update the shared control and its callers together.
- `backend/dashboard.go`: `onThisDay` uses a nearby-day window against milestone
  dates and photo date queries. `dashboardRecent` also assumes occurrence days.
- `backend/photo_index.go`: family/person date indexes, filtering, and cursors
  use `PhotoDate.Unix()`. Overlap matching cannot be implemented by merely
  substituting a lower bound in the current point-date index.
- `backend/same_age.go`, `backend/person_photo_insights.go`, `backend/person.go`,
  `backend/search.go`, and `backend/book.go` use single dates for age matching,
  portraits, timelines, search, book selection, or ordering.
- `backend/calendar_dates.go` normalizes stored record dates; preserve its
  calendar-day and capture-wall-clock rules while introducing the new model.
- Audit `frontend/lib/{dateUtils,history,timelineHelpers,photoSequence,book}.ts`,
  moment/media forms and detail pages, dashboard, History, Same age, and books.
  Also audit interview folding/import/export, video upload/metadata workers,
  family exports/imports, seed data, mobile API DTOs, and iOS offline/widgets.

Before implementing, enumerate all reads/writes of `MilestoneDate`, `PhotoDate`,
`MeasurementDate`, `Birthday`, `OccurredAt`, date serializers, and age helpers.
Classify each as occurrence, period, derived age, or administrative timestamp;
record its new policy and test in the implementation PR. Generated TypeScript
contracts must be regenerated rather than hand-edited.

## Semantics and examples

| Family's assertion | Stored meaning | Display | Anniversary eligible? |
| --- | --- | --- | --- |
| Happened on 2022-10-31 | Exact occurrence day | Oct 31, 2022 | Yes |
| Happened sometime in October 2022 | Occurrence window, month precision | October 2022 | No |
| Photo from 1970 | Occurrence window, year precision | 1970 | No |
| Photo around 1970; bounds 1968–1972 | Estimated occurrence window | Around 1970 (1968–1972) | No |
| First rode a bike while age 4 | Occurrence somewhere in the age-4 window | Sometime at age 4 | No |
| Loved painting at age 4 | Description associated with age-4 period | At age 4 | No |
| At age 4 years, 3 months | Age window with month precision | At age 4 years, 3 months | No |
| Around age 8; bounds ages 6–9 | Estimated age window | Around age 8 (ages 6–9) | No |
| Date unknown | Unresolved occurrence | Date unknown | No |

Age 4 means the half-open interval from the fourth birthday to the fifth:
`[birthday + 4 years, birthday + 5 years)`. Explicit ages 6–9 include the entire
ninth year and end at the tenth birthday. A month-precision age uses the next
month boundary. Blank months means year precision; explicitly choosing zero
months means month precision. Neither becomes a known anniversary.

For a child born 2020-06-15, age 4 resolves to `[2024-06-15, 2025-06-15)`.
It belongs in age-4 context and overlaps both calendar years, but is never
presented as an event on June 15. An exact birthday event must use a confirmed
calendar day. An estimate with a one-day bound remains estimated.

Calendar arithmetic must clamp to the last valid day of the target month:
Jan 31 plus one month becomes Feb 28/29, and Feb 29 plus a non-leap year becomes
Feb 28. Derive each boundary from the original birthday, not repeatedly from
an already clamped result. Replace the current `AddDate` overflow behavior for
new age assertions only; do not change stored legacy dates retroactively.
Use civil dates, not elapsed hours, for these calculations. Today/yesterday
must resolve in the user's selected/local date context consistently across clients.

## Proposed authoritative representation

Introduce a versioned shared `RecordTime` value on moments and media. Final Go
field names can follow repository conventions, but preserve this contract:

| Field | Contract |
| --- | --- |
| `version` | Schema version, independent of the containing vpack version. |
| `meaning` | `occurrence` or `period`; media captures use occurrence. |
| `basis` | `calendar`, `age`, or `unknown`. |
| `precision` | `day`, `month`, `year`, `range`, or `unknown`, valid for the selected basis. |
| `estimated` | Explicit uncertainty flag; independent of precision and meaning. |
| Calendar assertion | Structured entered day/month/year, or inclusive range endpoints with explicit precision. Estimates also retain their central expression and chosen bounds. |
| Age assertion | Anchor person ID, entered years/months or range, and explicit precision. No implicit “first tagged person” anchor. |
| Provenance | Source (family recollection, inscription, original metadata, legacy, accepted AI suggestion), optional note, changed/accepted by and at. |
| Derived resolution | Nullable `startInclusive` / `endExclusive`, resolution status, and resolver version; computed by the server, never accepted as authoritative client facts. |

Store civil date strings/components without timezone conversions. Normalize
inclusive input ranges to half-open bounds using their precision. Unknown or
unresolvable age assertions have null bounds, not year 1, epoch, today, or upload
date. Do not combine contradictory calendar and age assertions silently.
Reject invalid/reversed ranges, invalid dates, negative ages, inaccessible
anchors, inconsistent modes, and unsupported schema versions before writes.
Historical entries cannot start wholly in the future. A current-age window
may end in the future without claiming future events; retain its full semantics.

Keep actual capture timestamps/EXIF, file creation, upload time, `CreatedAt`,
and `UpdatedAt` separate from `RecordTime`. A scan's timestamp is not its
historical occurrence date. Explicit user dates take precedence over later
metadata extraction; workers must not overwrite them. Attaching media to a
moment does not automatically copy either record's time assertion.

An age assertion remains anchored when a birthday is corrected. Recompute
bounds, indexes, cached comparisons, and book selections from the assertion.
Without a birthday, retain age-based retrieval for that anchor while leaving
calendar placement unresolved. Other people's ages require a resolvable calendar
window and their birthdays; show a range or omit the age, never use a midpoint.
Partial birthdays remain a separate feature; unknown birthdays must work now.

Validate anchor access on writes and prevent hidden-person details leaking in
shared/linked views. Person merges must remap anchors; removing a photo tag must
not silently replace its anchor. Person deletion must explicitly handle dependent
assertions (re-anchor with confirmation or retain a readable unresolved assertion
under the existing deletion policy), not leave dangling IDs or fabricate dates.
Import must remap anchor IDs just like other person references.

## Shared rules and behavior across the application

Implement server helpers for validation, resolution, overlap, anniversary
eligibility, age bounds, and ordering. Web/iOS share contract fixtures for labels
and civil-date arithmetic. Feature code must not independently derive a fake day.

| Surface | Required behavior |
| --- | --- |
| Entry/edit forms | Keep Today / Yesterday / Exact date quick paths. Add Month, Year, Range, Age, Unknown, and an explicit estimate option. For moments ask “Happened sometime then” or “Describes this period”; age entry requires this choice. Show a plain-language preview and editable estimate bounds before save. |
| Detail cards, lists, lightbox | Show the original date/age label and period/estimate meaning. Computed calendar bounds are secondary (“Based on birthday”), never the headline asserted date. Editing unrelated fields preserves the assertion. |
| History and person timeline | Put period descriptions in labelled period/age groups; broad occurrences carry “Sometime in…” labels. No invented day headers. Cross-year records appear once per result set. Unknown/unresolved records have a reachable undated section. |
| Date filters and global search | Match interval overlap: `record.start < query.end && record.end > query.start`. Label occurrence overlap as possible and period overlap as context. Unknowns appear in unfiltered/undated results, not every dated query. Apply the same rule to parsed date and age search syntax. |
| On this day | Require exact, non-estimated, calendar-day occurrence before matching the existing nearby-day window. Apply this gate to moments, photos, videos, and folded interview results. A range containing the anniversary is insufficient. Filter before limits so excluded estimates do not starve exact results. |
| Recent content / reminders | “Recently added” uses creation time. A recent-occurrence section must not claim a broad window happened this week. Keep uncertain/contextual matches separate and labelled or omit them from that section. Actual birthday reminders continue using the person's birthday. |
| Same age / sibling comparisons | Separate precise occurrences, period context, and possible occurrence matches. Rank exact candidates first; approximate candidates by narrower overlap window, then stable ID. Do not rank a broad interval as distance zero or use a midpoint as a precise age. Preserve the newborn window of days 0–27; age 0 years is not a newborn claim. |
| Growing Up / portrait selection | Use exact candidates first. Approximate candidates need a visible age/window label and must not fill every month bucket. Assign at most one deterministic overview placement with its full span shown, or put it in a separate period group. |
| Books / exports / print / sharing | Use overlap selection with explicit labels; deduplicate cross-period records. Keep undated records selectable. Preserve labels and structured assertions in exports; snapshots retain the assertion and rendered interpretation of that edition. Never flatten a range into a date in captions or filenames. |
| Interview workflows | Keep the interview session's actual date. Audit answers represented as milestones and folding so a recorded interview day is not replaced by an age-period label. Historical standalone answers may use period assertions; do not count them as a dated session automatically. |
| Growth and other exact domains | Measurements, activity/event dates, and interview session dates retain their domain contracts. Growth's existing age shortcut must require confirmation of an actual measurement day or reject an imprecise input; do not plot a period at a guessed age or infer clinical data. Audit all shared controls so this restriction is explicit. |
| iOS, offline queues, widgets | Encode/decode the full assertion, display honest labels, preserve it through edits and queued uploads, and use the same anniversary policy. No mobile fallback to a birthday or upload day. |

For stable pagination, use a versioned internal key (resolved lower bound plus
record ID, with a separate undated partition). It is a deterministic order, not
a claim of exact chronology. UI grouping must convey overlapping uncertainty.
Date edits invalidate affected cursors; return a restart response instead of
silently skipping results. Keep authorization checks in every candidate path.

For photo filtering, retain the efficient exact-date path and add a family/person
scoped collection/index for non-exact records. Filter that candidate set by
interval overlap, merge, deduplicate, order, then paginate. Never expand a wide
range into one index entry per day or rely only on lower-bound range scans.
Benchmark against realistic libraries before choosing a more complex interval
index. Mutations and backfills must keep indexes consistent with assertions;
birthday changes must not leave stale results visible during a background rebuild.

## Migration, compatibility, and correcting existing records

1. Add versioned vpack readers/writers and a restartable, idempotent migration.
   Retain existing timestamps as legacy data and map them to exact occurrences
   with legacy provenance. Reconcile record counts and indexes after backfill.
   **This preserves existing behavior; it does not prove those dates were exact.**
2. The old persisted records do not retain age input, so do not automatically
   reinterpret all birthday-dated records. Real birthdays would be corrupted.
   Add a review workflow that finds candidate clusters on birthdays, previews
   selected records, and lets the family change them to “At age 4” or “Sometime
   at age 4.” Include single-record editing and bulk correction with per-record
   authorization, before/after values, and an undo path. Suggestions never save
   automatically. This workflow is part of delivery, not a later cleanup idea.
3. Add API capabilities/schema versions before enabling non-exact writes.
   Legacy omitted temporal fields preserve an existing assertion on unrelated
   edits. Explicit legacy date changes on a non-exact record must fail clearly
   unless the client supports an intentional conversion. Reject unsupported
   future assertion versions without mutating data.
4. Do not return a fabricated timestamp to older clients as compatibility data.
   Require a compatible mobile version for affected reads/writes once non-exact
   records are enabled, using the existing mobile-version mechanism as a starting
   point. Cover pending offline requests and cached widget data in rollout.
   Deploy server readers, compatible web/iOS readers, then enable new writes.
5. Extend native JSON/CSV/bundle export and import with versioned assertions,
   precision, meaning, provenance, anchor remapping, and null resolution. Older
   imports retain their previous date behavior and get legacy provenance.
   Unsupported export formats must use labelled text or explicitly reject lossy
   export, never silently coerce. Derived bounds are recomputed after import.
6. Back up before migration and test restore. Once new assertions exist, a
   rollback may disable new writes but must retain compatible readers; do not
   run an old binary that can overwrite the new schema. Re-run backfills safely.

## Implementation sequence and release gates

Each phase may be a separate PR, but non-exact writes stay disabled until all
consumers of enabled record types can preserve and display their meaning.

1. **Contract and inventory:** finalize DTO/vpack versions, assertion examples,
   boundary rules, audit checklist, compatibility strategy, and cross-client
   fixtures. Identify the companion iOS changes before backend rollout.
2. **Foundation:** implement shared resolution/validation/query helpers, versioned
   storage, migration, provenance, indexes, and export/import. Prove legacy reads,
   lossless round trips, restartability, and access isolation.
3. **Consumers first:** update dashboard anniversary gating, History, filters,
   search, Same age, Growing Up, books, detail formatting, and exact-only domain
   controls. Add API capability handling and compatible iOS/widget readers.
4. **Manual capture and repair:** enable full moment/media entry and editing,
   age-period choice, explicit multi-person anchor, unknown dates, and bulk
   correction preview/undo. Verify web and iOS writes and offline behavior.
5. **Release verification:** run the acceptance suite below on migrated staging
   data, including a birthday cluster. Enable writes only after all gates pass.
6. **Later:** separately evaluate AI estimation and optional period-memory
   resurfacing (e.g. “When she was 4”). Do not repurpose On this day for this.

Minimum useful release includes age-period moments, uncertain media dates, honest
labels/query behavior everywhere, legacy correction, and compatible mobile/export
paths. If delivery must be split further, keep unsupported write modes disabled;
a label-only or photo-only release does not solve this feature's core problem.

## Acceptance tests

- Twenty “describes age 4” moments survive save/reload/edit/export/import as age
  periods, appear in age-4 History/Same age context, and produce **zero** On this
  day results around the fourth birthday. A separate actual birthday event still
  appears. Repeat with “happened sometime at age 4,” photos, and videos.
- Exact day, month, year, inclusive range, estimate, age year, explicit zero
  months, age range, and unknown inputs preserve meaning and labels. Estimates
  never gain anniversary eligibility simply because their bounds narrow.
- Boundaries include the lower day and exclude the upper day; test leap years,
  Feb 29 births, Jan 31 births, DST and opposing timezones, and current-age periods.
  New year/date arithmetic cannot shift an asserted calendar day.
- Birthday correction changes derived bounds and all affected query/index/cache
  results without changing “at age 4.” Missing birthdays remain unresolved;
  anchor-age search still works. Multi-person age displays preserve uncertainty.
- Mixed exact, broad, and undated results paginate without duplicates or missing
  overlap matches. Cross-year book/search results deduplicate. Broad intervals
  neither flood newborn portraits nor starve exact anniversary results.
- Legacy birthday clusters remain unchanged until explicitly corrected. Bulk
  correction previews only authorized records, supports undo, and updates all
  downstream surfaces. Real birthday events can be left exact.
- Old vpack fixtures, repeated/interrupted migration, rollback/restore, legacy
  API updates, stale offline payloads, unknown schema versions, and mobile gating
  cannot erase or invent temporal precision.
- Export/import remaps anchors and retains provenance; person merge/delete and
  photo tag removal obey their policies. Hidden family/anchor information never
  leaks via candidate indexes, labels, or exports.
- Upload metadata extraction and video workers preserve manual assertions; scans
  without a known occurrence date remain unknown. Linked media/moments retain
  independent dates. Interview folding and growth retain their domain semantics.
- Run relevant Go and frontend suites plus iOS contract/UI tests in its repository;
  add shared golden fixtures for resolution and labels and a realistic overlap
  query/pagination benchmark. Document manual staging checks on every audited
  surface before enabling the feature.

## Optional "Help estimate"

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
