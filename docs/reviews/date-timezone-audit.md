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

## Remaining limitations

- Legacy API `inputType: "today"` uses `time.Now()` on the server. Clients must
  send `inputType: "date"` with their local date as documented in `mobile-api.md`.
- Automatic photo uploads without readable EXIF still fall back to the server's
  current instant; the resulting UTC photo day can differ from the uploader's
  local day. A future API change should accept a client-supplied fallback date
  while retaining EXIF precedence. This change does not guess the client's zone.
- Server-generated `Person.Age` and requests that omit `today` still use the
  server clock. Other clients should supply dates or calculate local current age.
- EXIF/photo storage does not provide a universal original capture timezone;
  UTC photo-day consistency does not recover a missing original local date.
- No historical dates are migrated. Previously stored incorrect dates cannot be
  reliably repaired without the intended calendar date.

## Verification

`calendarDates.test.ts` covers UTC, Chicago, Los Angeles, Tokyo, and Kiritimati;
bare and Z-suffixed dates; month/year boundaries; local midnight; both DST
transitions; and continued local conversion of real timestamps. The complete
frontend suite passes (264 tests), as does `tsc --noEmit`. No backend code changed;
backend review was static, and Go tests were not run in this environment.
