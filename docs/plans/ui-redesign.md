# UI redesign

The app grew outward from people, growth, and milestones. Photos, face
tagging, tags, chat, and activities were each added as their own page, reached
from their own menu entry. Each one works on its own, but the app doesn't feel
like one thing, and the navigation is a list of those features rather than a
way to get anything done.

This plan redesigns navigation, the dashboard, the person page, the
timelines, and the entry flows around two jobs:

1. **Putting data in, fast.** This is most of the day-to-day use: I just took
   a photo, I just measured someone, something happened.
2. **Seeing it in context.** The measurement detail page is the best page in
   the app because it answers "what were the others like at this age?" That
   idea should be everywhere it makes sense, and it should replace the compare
   page.

Most day-to-day entry happens in the iOS app, and the app should look and work
like the web. So this web design is also the reference the app copies: the
same destinations, the same add sheet, the same page layouts. See
[iOS parity](#ios-parity) for what that constrains.

The work is frontend only. The few places that could use a new backend proc are
called out. They're optional for the web, but worth doing for the app (see
[iOS parity](#ios-parity)).

## What we have to work with

Everything the family records hangs off a **person** and a **date**. Since
every person has a birthday, every date is also an **age**.

| Data | Shape | Where it shows now |
| --- | --- | --- |
| People | name, birthday, gender, profile photo, relations, pregnancy flag, linked households | dashboard cards, profile |
| Growth | height or weight, value, unit, date | profile timeline cards, measurement detail, family chart |
| Milestones | text, one of six categories, date, photos, tags | profile timeline, family timeline, search |
| Photos | date (EXIF), people (manual and face-tagged), tags, title and description | photos grid, both timelines, photo detail |
| Activities | activity → season → events and entries → appearances with results (rank, score) and photos | its own section, with a link from the profile; performances appear on the person timeline |
| Tags | name and color, on milestones and photos | filters on the timelines and photos |
| Chat | messages | chat page |

The age axis is what makes this data interesting. Chronologically, a family's
record is a feed. Aligned by age, it's a comparison between siblings, and
nothing else the family owns shows them that.

## What's wrong now

**Navigation.** On every screen size there's a single hamburger menu with a
"Quick add" grid and a "Go to" grid of seven destinations. Adding anything
takes three taps and a page load before you start typing. Some pages can only
be reached from other pages: faces from photos, tags from the timeline,
compare from the dashboard, activities for a person from their profile.

**Entry flows.**
- *Measurement:* only one of height or weight per submit, so a checkup takes
  two trips through the form. The person is picked from a `<select>`. The
  "when" block is a four-way radio (today, date, age) even though it's almost
  always today. After saving, you land on the profile timeline, when the
  comparison-to-family page is the one you'd want to see.
- *Photo:* the order is person checkboxes, file, title, description, then a
  four-way "when" radio. The server already reads the EXIF date, and face
  tagging suggests people, so most of that form is asking for things the app
  already knows.
- *Milestone:* the person is a `<select>` and the category is a `<select>`,
  so it's two dropdowns before the text box.
- *Activities:* results are entered deep in season → competition → results
  editor. Nothing surfaces "there's a competition this weekend."

**Dashboard.** A greeting, person cards, and the same three add buttons that
are already in the menu. Nothing on it changes from day to day, so there's no
reason to look at it.

**Timelines.** The person and family timelines are flat lists of equal-weight
cards. Each measurement gets a card of its own, so a checkup is two cards that
say "Height 38 in" and "Weight 32 lb". Photos are single cards with a
thumbnail. A week with 40 photos and one real milestone looks like 40 photos.
The family timeline doesn't include activities at all. Filters (type, person,
tag, age year, sort order) take up the top of the page, and they're the only
way to do anything with it.

**Compare page.** You pick up to five people and get one column per person,
each a full timeline, filterable by age year. It doesn't work because:
- The columns don't line up. Row 3 in one column is a different age than row
  3 in the next, so you're comparing nothing to nothing.
- Age-year buckets are too coarse. "Age 1" covers crawling through running.
- Two to five columns of cards don't fit on a phone.
- It compares everything at once, which in practice means nothing. The
  measurement page works because it's anchored to one data point.

The family growth chart, by contrast, does work: it aligns people by age on the
x-axis. It's the same "at this age" idea, done right, but for growth only.

## Design principles

- **Anchor comparison to a moment.** Comparison always starts from one thing
  (this measurement, this photo, this age) and shows what everyone else looked
  like at the same age. Never "pick five people and look at everything."
- **Rows per person, not columns.** Comparisons stack vertically, so they
  work at phone width.
- **Default everything, ask for nothing.** Today's date, the last person you
  picked, the units you used last time, the EXIF date, face-suggested people.
  Every field on an entry form should already be filled with its most likely
  value.
- **Summarize, then drill in.** Timelines show a day or month as one unit
  (a photo mosaic, a checkup row, a result) rather than one card per record.
- **Activities are part of the person's life, not a separate app.** They show
  on the person's page and the family history, and they surface on the
  dashboard when a season is active.
- **Context-aware add.** The add button knows where you are. On Clara's page
  it adds for Clara; on a competition page it adds a result or photos for that
  competition.

## Navigation

Replace the hamburger with a small set of permanent destinations plus one
add button.

**Phone (bottom bar, always visible):**

```
 Home   Photos   [ + ]   Growth   Chat
```

**Desktop (top bar):**

```
 Family Record   Home  Photos  Growth  History  Chat      [+ Add]  (avatar ▾)
```

- **Home** is the dashboard, and its family strip is the way into each person
  (see below). There's no separate "People" destination. With fewer than ten
  people, a strip of faces beats a list page.
- **Photos** is the existing grid, with person chips across the top.
- **Growth** is the age-aligned chart, which is the family chart promoted to
  top level (see below).
- **History** is the reworked family timeline. On phones it's reached from
  Home ("See all") and the account menu instead of taking a bottom-bar slot.
- **Chat** keeps its slot, since it's the one thing people open to reply to a
  push notification.
- **+** opens the add sheet (see below).
- **Account menu** (avatar, top right): Activities setup, Tags, Face review
  (with a count badge), Settings, Import/Export, Admin (admins only), Log out.
  These are occasional or administrative and don't need a top-level slot.

The marketing header for signed-out visitors stays as is.

Old URLs keep working: `/family-timeline` → `/history`, `/family-chart` →
`/growth`, `/compare` → `/same-age`, `/dashboard` stays. Push notifications
and bookmarks both point at these.

## The add sheet

One sheet, opened from **+** anywhere. It replaces the menu's quick-add grid
and the dashboard's action cards.

```
┌──────────────────────────────┐
│  (Clara)  Jake   Mom   Dad   │  ← person chips; preselected from context
│                              │     or the last person used
│  📷 Photos                   │
│  📏 Measurement              │
│  ⭐ Milestone                │
│  🏆 Result — Regionals (Sat) │  ← only while an event is in its window
└──────────────────────────────┘
```

Picking a person on the sheet carries through to the form, so the form doesn't
ask again. Each form below still shows the chips, to change or add people.

### Photo: upload first, describe after

1. Tap **Photos** and the file picker opens right away (on a phone, that's the
   camera roll or camera).
2. Uploads start as soon as files are picked. The existing multi-file upload
   and processing queue handle the rest.
3. While they upload, a light form shows the people chips (preselected from
   the sheet), an optional note, and tags. Date is the EXIF date, shown as a
   read-only line with a "change" link. The "When was this taken?" radio goes
   away.
4. **Done** goes back to wherever you started. Face tagging adds people in
   the background, as it already does.

Being able to skip the form entirely matters most here. Picking the files is
the whole job, and everything after it is optional.

### Measurement: a checkup is one entry

```
Clara · 3y 4m                          Today ▾
Height  [ 38.5 ] in        Weight  [ 32 ] lb  [ 4 ] oz
                                       [ Save ]
```

- Height and weight on the same screen, both optional, at least one required.
  Saving sends one `AddGrowthData` per filled field. No backend change.
- Units are remembered per person (babies in lb/oz, adults in lb, and so on),
  falling back to the family's last-used unit. The existing lb-oz and ft-in
  inputs stay; they just stop being a choice you make every time.
- Date is a "Today ▾" control that opens to Yesterday, a date picker, or
  "by age." Today is the default and needs no tap.
- The field shows the last value as a placeholder ("last: 37.75 in, 4 months
  ago"). That catches a mistyped unit before it's saved.
- **After saving, land on the result.** Show the measurement detail for what
  was just entered, with the percentile and the "compared to family" cards. That
  context is the reward for entering the data, so it's what you should see
  next. With both height and weight entered, show both on one result page,
  stacked.

### Milestone

- Person chips, then the text box with focus.
- Category is a row of six icon chips instead of a dropdown, defaulting to
  "First time" (most milestones are firsts).
- Date defaults to today, using the same control as measurements.
- Photo attach and tags live behind "Add photos / tags", collapsed by default.

### Activity result

This only shows while an event is in progress or just ended, meaning an event
whose dates include today or the previous three days. It jumps straight to that
event's results editor, filtered to the family's entries. Outside that window,
results are entered from the season pages as they are now.

## Home (dashboard)

Home is for "what's new, and what should I add." From the top:

1. **Family strip.** Avatars with name and current age ("3y 4m"), kids
   first, then adults, then linked households. Tapping one opens that
   person's page. A pregnancy shows as a card with the countdown, as it does
   now.
2. **Nudges** (at most two, dismissible, and only when they're true):
   - "4 photos have faces to review" → face review.
   - "Clara hasn't been measured since March" → measurement form for Clara.
     Shown when a child under 18 has no measurement in 6 months.
   - "Jake turns 7 on Saturday."
3. **In season.** For each activity season active today: the next or most
   recent event, its date, and quick links to add photos or results. Hidden
   when nothing is in season.
4. **On this day.** Photos and milestones from this date in past years, one
   row per year ("3 years ago"). Hidden when there's nothing. This comes
   from `plans/record-ideas.md`, and it's the most likely reason to open the
   app when you aren't adding something.
5. **Recent.** The last two weeks, grouped by day, in the summary style
   described under History below. Links to **History**.

The dashboard's add buttons go away, because **+** is always on screen.

**Data:** `ListPeople`, plus `GetFamilyTimeline` over a recent range for
Recent and nudges, `GetFaceReview` for its counts, and `ListActivities` /
`ListSeasons` for In season. For On this day, a handful of narrow
`GetFamilyTimeline` date-range calls (one per past year, a few days wide) works
with no backend change. If that's too chatty, a `GetDashboard` proc bundling
these is the natural optional backend addition.

## Person page

The person page is where people go most after Home. It gets a summary at the
top and the story below.

```
┌───────────────────────────────────────────────┐
│ (photo)  Clara                     [ + Add ]   │
│          3 years 4 months · born May 2, 2023   │
├───────────────────────────────────────────────┤
│ Height 38.5 in  71st %ile   ▁▂▃▅▆▇ (sparkline)│
│ Weight 32 lb    55th %ile   ▁▂▃▄▅▆             │
│ Measured 2 weeks ago · Jake was 37 in at 3y4m  │
│                                                │
│ 🩰 Dance 2025–26 · 3 routines · next: Sat      │
├───────────────────────────────────────────────┤
│ At 3y 4m, Jake was…   [photo] [photo] [photo] →│
├───────────────────────────────────────────────┤
│  Story | Photos | Growth | Activities          │
└───────────────────────────────────────────────┘
```

- **Snapshot:** the latest height and weight with percentile, a sparkline,
  when they were last measured, and one line of sibling context at the same
  age (reusing `lib/growthComparison.ts`). Then any active activity season.
- **"At this age" strip:** photos of siblings at this person's current age,
  from the same-age lens below. For the oldest child there's no one ahead of
  them, so the strip flips to "…and Jake at this age was" for the next-younger
  sibling's future, or hides.
- **Tabs:**
  - **Story** replaces the unified timeline (see History, which uses the same
    layout), grouped into chapters by age: "Age 3", "Age 2", … Each chapter
    opens with a line like "Grew 3 in and 4 lb" when it has measurements.
  - **Photos** is the photo grid filtered to this person (`ListFamilyPhotos`
    with `personId`).
  - **Growth** is this person's chart with percentile bands, siblings' curves
    as a faint overlay you can toggle, and the measurement list.
  - **Activities** is the current `/person-activities/:id` page, folded in.
    That route redirects here.
- **+ Add** opens the add sheet with this person preselected.

The profile page's filter bar (type toggles, sort, age-year pills, tag pills)
goes away. Chapters replace the age pills, the tabs replace the type toggles,
and tag filtering moves to History and Photos, where it's actually used.

## Same age (replaces Compare)

This is the "what were they like at this age?" lens as a page of its own. It's
also the component behind the strips on the person, photo, and milestone pages.

```
At  [ ◀  3 years 4 months  ▶ ]        (slider)

Clara   now      [photo][photo][photo]  38.5 in · 32 lb
                 ⭐ First time: wrote her name

Jake    Jan 2022 [photo][photo]          37 in · 31 lb
                 ⭐ Started preschool

Mom     —  no records at this age
```

- One age at a time, one row per person who has reached that age. That fixes
  the old page's misaligned columns.
- The age control steps by one month under 2, three months from 2 to 6, and
  six months after that. The window around the age scales the same way,
  reusing the tolerance idea in `growthComparison.ts`.
- By default, the age is the youngest child's current age, which answers the
  question you actually have: "what were the older ones doing at this age?"
- Each row shows the nearest photos in the window, the height and weight
  nearest that age (from `computeFamilyComparisons`), and the milestones in the
  window.
- People without data at that age collapse to one line. Adults usually have
  none, and that's fine.
- It can be opened from anywhere with a person and an age:
  `/same-age?age=40m&from=<personId>`.

**Where it shows up in context:**
- **Measurement detail:** as now, with a link to "see everything at this
  age."
- **Photo detail:** a "Siblings at this age" strip under the photo, using the
  age of the first tagged child in it.
- **Milestone detail:** a new read-only milestone page (only an edit page
  exists today). It shows the milestone, its photos, and "at this age, Jake
  was…". `GetMilestone` already exists.
- **Person page:** the strip described above.

**Data:** `GetFamilyTimeline` with `skipPhotos` gives growth and milestones for
everyone. That's already what the measurement page does. Photos come from
one `ListFamilyPhotos` call per person, with `personId` and a `dateFrom` /
`dateTo` window computed from their birthday. No backend change.

`/compare`, `ComparePeople`'s only frontend caller, and the compare styles
are deleted. Leave the proc in place until a release has passed, since the
mobile app might call it. Check `docs/mobile-api.md` before removing it.

## Growth

The family chart becomes **Growth**, a top-level page:

- It's the age-aligned multi-person chart, as now, with one height or weight
  toggle instead of solid and dashed lines on one plot. Mixing both on one
  axis is hard to read.
- It defaults to the kids selected. People are avatar chips rather than
  checkboxes.
- Optional CDC/WHO percentile bands for the selected sex. `growthPercentiles`
  already has the data.
- Tapping a point opens that measurement's detail page.
- There's a "Measure" button, since checking the chart and adding a
  measurement usually happen together.

## History (replaces the family timeline)

This is the chronological view, redesigned to summarize.

- **Grouped by month**, with sticky month headers and a year jump (the
  existing year pills).
- **Within a day, related records merge:**
  - Photos taken on the same day become one mosaic tile ("12 photos · Clara,
    Jake"), which opens the photo grid filtered to that day.
  - Height and weight for one person on one day become one "Checkup" row with
    both values and percentiles.
  - Birthdays are dividers ("🎂 Jake turned 6"), not cards.
  - Milestones stay as full cards. They're the rarest and most meaningful
    records, and they should stand out.
  - Activity events show as one card per event, with the family's results
    ("Regionals · Clara 1st place, Solo Jazz") and photos.
- **Filters** collapse to one row: person chips, and a "More" popover for
  type and tags. Search stays.
- It keeps the existing year-based paging (`lib/photoPages.ts`).

The person page's Story tab uses the same components, grouped by age instead
of by month.

**Activities data:** `GetFamilyTimeline` doesn't return activity events.
Frontend-only, History can call `GetPersonSeason` per person and merge the
results. That's N small calls, which is fine for a family. The cleaner
optional backend change is adding events and appearances to
`GetFamilyTimeline`'s response under the same date range.

## Activities

Activities stop being a separate island:

- **Person page:** the Activities tab (the current person-activities page)
  plus the season line in the snapshot.
- **Home:** the In season card.
- **History:** event cards with results.
- **Add sheet:** "Result" during an event's window.
- **Setup** (programs, seasons, events, entries, rosters) stays as its
  current pages, reached from the account menu under "Activities." That's
  administration, done a few times a year.

The season, competition, and routine pages keep their layouts, but get the new
nav and a context-aware **+** (photos for this event, results for this event).

## Detail pages

The measurement, photo, and new milestone detail pages share one layout:
the record, its actions (edit, delete), then **context**, which is the
same-age lens scoped to that record. Back links go to where you came from
rather than always to the profile.

## Out of scope

- New record types (quotes, head circumference, birth details). They're in
  `record-ideas.md`. This redesign should make them easy to slot in: one more
  option on the add sheet, one more summary component in History.
- The signed-out landing page, auth pages, legal pages, and admin pages,
  beyond picking up the new header.
- Chat's internals.
- Building the iOS screens. This plan designs them, and the app work follows
  each web phase.

## iOS parity

The web is the design reference, and the app implements the same design
natively. That's a constraint on the web design, not just a follow-up task.

**Every pattern has to have a native equivalent.**

| Web | iOS |
| --- | --- |
| Bottom bar: Home, Photos, +, Growth, Chat | `TabView` with the same five items. **+** presents a sheet instead of switching tabs |
| Add sheet | a sheet with a medium detent |
| Person chips | the same horizontal chip row |
| "Today ▾" date control | a menu (Today, Yesterday, Pick…) plus a date picker |
| Upload-first photos | `PhotosPicker`, then upload in the background |
| Account menu | a profile button in the Home toolbar |
| Same-age stepper | a stepper or slider, with the same age steps |

- **No hover-only interactions.** Everything has to be reachable by tapping,
  which is right for phone-width web anyway.
- **Wording is shared.** Page names, button labels, empty states, and nudge
  text should be identical, so a family using both doesn't notice the switch.
  Keep them in one place on the web (a small `lib/copy.ts`) so the app has a
  single list to copy from.
- **Logic the app would need is kept pure.** The age-window rules, nudge
  thresholds, merging a day's records into summaries, and the same-age lookup
  go in plain functions in `lib/` with vitest coverage, next to the existing
  `growthComparison.ts`. The app ports them, and the tests become the spec for
  what the port must do.

**Backend procs become worth adding.** On the web, a few extra round trips
for Home, Same age, and History are fine. On a phone on cellular they aren't,
and the app would otherwise have to reimplement the fan-out logic. So the
optional procs this plan names become part of the plan, added as each phase
needs them:

- `GetDashboard`: people with current ages, nudges, in-season activities, On
  this day, and recent records, in one call. This also moves the nudge rules
  to the server, so the web and the app can't disagree.
- `GetSameAge(age, fromPersonId)`: one row per person, with the nearest
  measurements, milestones, and photo IDs in the window. This puts the window
  rules on the server too.
- Activity events and appearances in `GetFamilyTimeline`, under the same date
  range, for History.

These are all additive, so older app builds are unaffected. They go into
`docs/mobile-api.md` once the app depends on them. Where a phase above says
"frontend only, N calls", treat that as the fallback if the proc isn't ready
yet.

**Each phase ships web first, then app.** The web version settles the design.
The app follows once it's been used for a bit, rather than both being built
at the same time and drifting apart.

## Phases

Each phase ships on its own and leaves the app coherent.

1. **Shell.** Bottom bar and top bar, the account menu, and the add sheet
   (linking to the existing forms), plus the route redirects. The hamburger
   menu is deleted. Small, and it changes how the whole app feels.
2. **Fast entry.** Person chips and the date control as shared components.
   The combined height and weight form with remembered units and a result
   page after saving, upload-first photos, and the milestone form with
   category chips. This is the day-to-day win. It goes early because it's
   also the part the iOS app copies first, since that's where entry happens.
3. **Home.** Family strip, nudges, In season, On this day, and Recent.
4. **Same age.** The lens component, the `/same-age` page, and the strips on
   measurement and photo detail. The milestone detail page. Delete compare.
5. **Person page.** Snapshot, the at-this-age strip, tabs, and Story
   chapters. Fold in person-activities.
6. **History and Growth.** Month grouping with merged day summaries,
   activity events, and the Growth page's toggles and bands.
7. **Cleanup.** Delete dead code. `pages/profile/tabs/growth.tsx`,
   `photos.tsx`, and `timeline.tsx` already have no importers. Also delete the
   old unified timeline and whatever styles are orphaned in `styles/global.ts`.

`tests/ui/smoke.spec.ts` navigates by clicking (see the Playwright notes), so
phase 1 changes its selectors. Update it in the same PR.

## Status

All seven web phases are built on the `ui-*` branch stack. Where the build
differs from the text above:

- **Backend procs.** `GetDashboard`, `ListOpenEvents` (the add sheet's Result
  option), `GetSameAge`, and `includeActivities` on `GetFamilyTimeline` were
  built with their phases rather than left as fallbacks. `UpdatePhoto` also
  takes `inputType: "keep"`, so a caption or tag edit doesn't reset the EXIF
  time. Only `includeActivities` and `keep` are in `docs/mobile-api.md` so far.
- **Dates.** Entry forms send today as the device's local date with
  `inputType: "date"`, never `"today"`.
- **Same age.** `ageMonths` is nullable: null means the person's current age,
  0 means birth. Linked households are included; people without records
  collapse to one line.
- **Person page.** The at-this-age strip hides for the oldest child rather than
  flipping to a younger sibling's age.
- **Nudges.** Measure nudges only fire for children who have been measured
  before, at 3 months under two and 6 months after. Birthday nudges cover
  everyone visible, including linked households.
- **Shared logic** for the app to port lives in `lib/` with vitest coverage:
  `when`, `checkup`, `familyGroups` (chip order and labels), `daySummary`,
  `story`, `history`, `sameAge`, `ageChart`, `familyStrip`, `appNav`.
  Wording is in `lib/copy.ts`.
- **Not done.** `ComparePeople` is still registered, with no web caller.
  History, like the old timeline, only shows photos tagged with someone.

## Open questions

- **Chat in the bottom bar**, or in the account menu with an unread badge on
  the avatar? It depends on how much it gets used.
- **Linked households on Same age.** Grandparents rarely have records at a
  child's age, but a cousin in a linked household might. Include shared
  people by default, or behind a toggle?
- **Nudge thresholds.** Six months without a measurement is a guess. Under
  two, well-child visits are every two to three months.
