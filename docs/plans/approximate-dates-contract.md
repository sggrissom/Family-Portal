# Approximate dates: contract and inventory

Phase 1 of [approximate-dates.md](approximate-dates.md). Written against main
`1b0a6de` (2026-11-02). This is the reference the later phases build to.

## RecordTime

`backend/record_time.go` defines the assertion; `frontend/lib/recordTime.ts`
mirrors resolution and labels. `frontend/lib/recordTimeFixtures.json` is the
shared golden file: Go (`record_time_test.go`) and vitest (`recordTime.test.ts`)
both run it, and the iOS repo should copy it verbatim into its contract tests.

| Field | Values / rule |
| --- | --- |
| `version` | `1`. Anything else is rejected before a write. Independent of the containing vpack version (`PackRecordTime` has its own). |
| `meaning` | `occurrence` or `period`. Media is always `occurrence` (`validateRecordTime(rt, allowPeriod=false)`). |
| `basis` | `calendar`, `age`, `unknown`. |
| `precision` | calendar: `day` / `month` / `year` / `range`. age: `year` / `month` / `range` (range is whole years). unknown: `unknown`. |
| `estimated` | Independent flag. Never makes a record anniversary-eligible, whatever its precision. |
| `from`, `to` | Calendar civil strings `YYYY`, `YYYY-MM`, `YYYY-MM-DD`. `to` only for `range`; the range ends at the end of `to`'s own precision (inclusive input, half-open bounds). |
| `around` | Estimates only. Central value in the basis' notation: a civil string, or whole years for an age. Display only; matching uses the bounds. |
| `anchorPersonId`, `ageYears`, `ageMonths`, `ageToYears` | Age assertions only. Anchor is always explicit; there is no "first tagged person" fallback. `month` precision with `ageMonths = 0` is the "explicitly zero months" case. |
| `source`, `note`, `changedBy`, `changedAt` | Provenance. `source` ∈ `family`, `inscription`, `metadata`, `legacy`, `ai`. |
| `start`, `end`, `resolved`, `resolverVersion` | Derived by the server: half-open civil days, both `""` when unresolved. Clients never send them as facts; the server recomputes on every write and on birthday changes. |

Age bounds are computed from the original birthday with month-end clamping
(`civilDate.addMonths`), never by repeated `AddDate`. A person with
`IsPregnancy` has a due date, not a birthday, so age assertions anchored to
them stay unresolved, like a missing birthday.

Helpers and their contract:

- `validateRecordTime(rt, allowPeriod)` — structural validation, no I/O.
  Anchor access and future checks need the database and today, so callers do
  those (`checkRecordTimeNotFuture`, and anchor access in phase 2).
- `resolveRecordTime(&rt, anchor)` — fills the derived fields.
- `rt.ExactDay()` — the anniversary gate (occurrence, calendar, day, not estimated).
- `rt.Overlaps(start, end)` — `rt.start < end && rt.end > start`; unresolved never matches.
- `rt.SortKey()` — start of the derived range, `ok=false` when unresolved.
- `rt.Label()` — the headline label. Matches the plan's examples table.

## Legacy timestamps after phase 2

`Milestone.MilestoneDate` and `Image.PhotoDate` stay, demoted to derived
ordering keys:

- Exact records: unchanged. A photo keeps its capture wall clock.
- Non-exact, resolved: the start of the range (`SortKey`). It is a position,
  not a claim; nothing may display it as the record's date.
- Unresolved: zero time, and the record lives in an undated partition.

That keeps the existing point-date indexes valid for exact records and gives
every consumer one rule to check: if `!recordTime.ExactDay()`, the timestamp
is an ordering key only. Older clients never receive non-exact records with a
plain timestamp, because non-exact writes stay off until the mobile minimum
version requires a reader that understands `recordTime` (see Rollout).

## Inventory

Every read/write of the affected timestamps on main, classified. "Occurrence"
means it must move to the RecordTime rule named in the policy column.

### Writes

| Site | Class | Policy |
| --- | --- | --- |
| `milestone.go` `AddMilestoneTx` / `UpdateMilestone` (via `resolveEntryDate`) | occurrence | Phase 4: accept a `recordTime`; legacy `inputType` path writes an exact calendar-day assertion with source `family`. Legacy `age` input stays as it is today until phase 4 replaces it, then becomes an age assertion. |
| `photos.go` `uploadPhotoHandler` / `UpdatePhoto` (`calculatePhotoDate`) | occurrence | Same. EXIF → source `metadata`. Fallback-to-today when there's no EXIF stays exact `metadata`; phase 4 offers "unknown". |
| `video_uploads.go` (`FallbackDate`), `video_worker.go` probe | occurrence | Worker may only replace the date when the assertion is still `metadata`/`legacy` and unchanged; a `family` assertion is never overwritten. |
| `interview_procs.go`, `interview_import.go`, `interview_export.go` | interview session date | Answer milestones keep the session's exact day (source `family`). Never age-period. |
| `import.go`, `import_bundle.go` | occurrence | Read `recordTime` when present; otherwise exact + `legacy`. Remap `anchorPersonId` like other person references; recompute bounds after import. |
| `book_export.go` (copy into book) | occurrence | Copies the full assertion. |
| `calendar_dates.go` `NormalizeStoredRecordDates` | maintenance | Leaves `RecordTime` alone; the backfill runs after it. |
| `seed*.go` | fixtures | Exact assertions, plus a few non-exact ones once writes are on. |

### Reads

| Site | Current behaviour | Policy |
| --- | --- | --- |
| `dashboard.go` `onThisDay` | nearby-day window on milestones + `ListFamilyPhotos` date range | `ExactDay()` gate before matching, applied before limits. |
| `dashboard.go` `dashboardRecent` | milestones/photos by occurrence day | Only exact occurrences in the recent window; non-exact excluded from that section. |
| `photo_index.go` family/person date indexes, `photoListing` | point-date index + cursors | Exact records keep the index. Non-exact go in a separate family/person-scoped index, overlap-filtered and merged (phase 3b). |
| `search.go` `dateMatches`, `ageMatches`, ordering | point date | Overlap. Unresolved only in undated results. |
| `same_age.go` | `abs(date − target) ≤ window` | Exact first; non-exact separated as context/possible, ranked by window width, never distance 0. |
| `person_photo_insights.go` (portraits, timeline buckets, header photo) | point date | Exact only for buckets/header; non-exact listed separately or once with span. |
| `person.go` person timeline window (`inWindow`) | point date | Overlap; one appearance per result set. |
| `book.go` selection (`inRange`) and ordering | point date | Overlap selection, `SortKey` ordering, keep undated selectable. |
| `milestone_analysis.go` photo suggestions, `ageInMonths` | point date | Exact only; skip suggestions for non-exact. |
| `photo_features.go` similar-photo grouping | capture time | Capture-time based; exact only (non-exact never groups by date). |
| `interview_views.go`, `interview_procs.go` (`ageYearsAt`) | answer date | Unchanged (session dates are exact). |
| `export.go`, `video_export.go` (filenames) | point date | Export the assertion and label. Filenames use the exact day only, else the label slug, never a fabricated day. |
| `account_deletion.go`, `person.go` tag changes (`ReindexPhotoDates`) | index upkeep | Also maintain the non-exact index; tag removal never rewrites an anchor. |
| Frontend: `lib/{history,daySummary,book,story,interviews,photoUpload}.ts`, milestone/photo add/edit/view, `family-photos`, `profile`, `search`, `books/edit-book`, `history` | display/group by date | Show `recordTimeLabel`; group non-exact by period; never derive a day. |

### Exact-only domains (unchanged)

`GrowthData.MeasurementDate`, `Season`/`Event` dates, `Interview.Date`,
`Appearance.OccurredAt`, `Person.Birthday`, `CreatedAt`/`UpdatedAt`. Growth's
age shortcut (`growth.go` → `resolveEntryDate`) keeps resolving to a single
day; phase 3 makes the form state that it records the day the child reached
that age and asks for confirmation, rather than sharing the moment control's
period options.

## Rollout

1. Phase 2 ships storage, the backfill (every existing record → exact +
   `legacy`), and `recordTime` in API responses. Writes still produce only
   exact assertions, so behaviour is unchanged.
2. Phase 3 changes consumers to the rules above; with only exact data they
   behave as before, which the existing tests pin.
3. Phase 4 adds non-exact writes behind `cfg.ApproximateDates` (off). The
   capability is reported to clients so web/iOS hide the new modes while off.
4. Enable on staging, run the acceptance list on migrated data including a
   birthday cluster, raise the iOS minimum version, then enable in prod.

## iOS companion changes

Needed before the flag is enabled in prod (iOS repo, built in its CI):

- Decode `recordTime` on Milestone/Image DTOs; copy `recordTimeFixtures.json`
  and port `recordTimeLabel` / resolution with the same tests.
- Display labels instead of formatting `milestoneDate`/`photoDate` when the
  assertion is not an exact day; undated section in timelines.
- On this day, widgets, and offline caches use the `ExactDay` gate.
- Entry/edit: preserve an existing non-exact assertion on unrelated edits
  (omit date fields); add the new modes when the server reports the capability.
- Offline queue: queued legacy date edits against a now non-exact record fail
  with the server's clear error instead of flattening it.
