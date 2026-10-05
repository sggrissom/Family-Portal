# Date and timezone review

Reviewed the web frontend and Go backend at `a4e1851`. The separate iOS repository
and existing production records were not inspected in this pass.

## Date conventions

Record dates (birthdays, measurements, milestones, activity dates, and photo
browsing days) use UTC calendar components. Explicit `YYYY-MM-DD` input is parsed
at UTC midnight. Displaying these values must not convert them to the viewer's
timezone. Actual timestamps such as chat messages, creation times, and log events
still need normal instant-to-local-time conversion.

“Today” is the viewer's local calendar date. Use `localDateString(new Date())`
before passing it to calendar-date helpers or the API. `toISOString()` represents
an instant and can cross midnight relative to the viewer.

## Findings fixed in this change

- The photo gallery converted record dates into local time, so midnight UTC
  photos appeared a day early west of UTC. The gallery and photo detail/edit
  screens now share `formatDate`.
- `formatDate` protected Z-suffixed values but let bare `YYYY-MM-DD` dates drift.
  It now formats the UTC day consistently.
- Growth percentile age calculations used local getters on stored dates. When
  one date crossed a month boundary and the other did not, fractional ages varied
  by timezone. Stored strings now use UTC components; caller-supplied `Date`
  objects (used for current age) use their local calendar day.
- Measurement “today/yesterday” labels compared elapsed hours with midnight UTC.
  They now compare the record day with the viewer's day, including across DST.
- Profile ages, family-chart child selection, the history current year, and the
  photo editor's maximum date now use the viewer's current calendar date.
- Imported birthday year labels and date-validity year checks use UTC components.

## Paths reviewed that already follow the convention

- `whenRequest` sends explicit local dates for today/yesterday; yesterday uses
  calendar subtraction rather than subtracting 24 hours.
- `resolveEntryDate` parses explicit dates without timezone conversion. Birthdays
  and activity date inputs also use date-only parsing.
- `calculateAge`, `monthsOld`, same-age rows, and growth chart record placement
  read stored date components in UTC.
- Dashboard, profile, and same-age requests supply a local `today` value.
- Photo range queries use UTC start/end boundaries, with an inclusive end day.
- Book day arithmetic and display use explicit UTC days.
- Timestamp formatting and chat grouping intentionally use local time.

## Explicit calendar-date storage

A follow-up made the convention a storage guarantee instead of a display rule.

- Record dates (birthdays, measurements, milestones, season and event dates,
  appearances) are always stored as UTC midnight of the day they name. Every
  write path normalizes: entry dates (including `inputType: "today"` and age
  offsets from a birthday), activity dates, and JSON and bundle imports.
- Photo dates are the local wall-clock time of capture, stored with a UTC
  label. EXIF `DateTimeOriginal` is read as a wall clock, ignoring the server's
  zone and any Canon timezone, so the UTC day of `photoDate` is the day the
  photo was taken where it was taken. No capture timezone is needed to recover
  the day.
- Automatic uploads take an optional `photoDate` (the uploader's local day) as
  the fallback when the file has no EXIF date. The web sends it, and the photo
  editor's "today" sends the viewer's local date.
- Migration `2026-1005-normalize-record-dates` rewrites stored values. vpack
  keeps each time's UTC offset, so each value is read in the offset it was
  saved with: record dates become that day at UTC midnight, and photo dates keep
  their wall clock relabelled as UTC. Values already in UTC (the server's own
  zone) keep the UTC day the web already shows, so no visible web date changes
  for them. The photo date index and milestone search index are rebuilt.

## Remaining limitations

- `inputType: "today"` and requests that omit `today` still use the server's
  UTC day. Clients should send their local date.
- A photo without EXIF and without a client `photoDate` falls back to the
  server's current UTC time.
- Records whose day was already wrong when saved, such as an evening `"today"`
  entry that the server put on the next UTC day, are not repaired. The
  intended day was never recorded.
- Photos keep their capture wall clock, not the true instant, so photos taken
  in different zones order by local clock time.

## Verification

`calendarDates.test.ts` covers UTC, Chicago, Los Angeles, Tokyo, and Kiritimati;
bare and Z-suffixed dates; month/year boundaries; local midnight; both DST
transitions; and continued local conversion of real timestamps. The complete
frontend suite passes (264 tests), as does `tsc --noEmit`. No backend code changed;
backend review was static, and Go tests were not run in this environment.
