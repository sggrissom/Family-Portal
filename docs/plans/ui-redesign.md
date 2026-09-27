# UI redesign

The app grew outward from people, growth, and milestones. Photos, face
tagging, tags, chat, and activities were each added as their own page, reached
from their own menu entry. Each one works on its own, but the app doesn't feel
like one thing, and the navigation is a list of those features rather than a
way to get anything done.

This plan redesigns navigation, the dashboard, the person page, the
timelines, and the entry flows around three jobs:

1. **Putting data in, fast.** This is most of the day-to-day use: I just took
   a photo, I just measured someone, something happened.
2. **Seeing it in context.** The measurement detail page is the best page in
   the app because it answers "what were the others like at this age?" That
   idea should be everywhere it makes sense, and it should replace the compare
   page.
3. **Finding something remembered.** A particular photo, a first, a summer,
   or a routine's results should be reachable without scrolling through years
   of unrelated records.

Most day-to-day entry happens in the iOS app, and the app should look and work
like the web. So this web design is also the reference the app copies: the
same destinations, the same add sheet, the same page layouts. See
[iOS parity](#ios-parity) for what that constrains.

This is primarily a UI and flow redesign, with additive backend work where
needed to make grouped history, same-age matching, and web/iOS behavior
consistent. Existing APIs can support prototypes; their current shape should
not dictate the finished experience. See [iOS parity](#ios-parity).

## What we have to work with

The main browsing axes are **person**, **date**, and **age** (when a birthday
and record date are known). Existing activity events, appearances, and milestone
attachments also connect records by **occasion**. Photos may include several
people or none; chat belongs to a household rather than a person.

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
- **Remember preferences; visibly suggest inferred facts.** Default today's
  date and remembered units. Carry an explicitly selected person or event
  forward, and make that selection visible. A globally opened photo upload
  must not silently tag the last person used in an unrelated form.
- **Summarize, then drill in.** Group explicit relationships first (an event
  and its photos, a milestone and its attachments), then use day grouping
  for remaining records. Do not repeat attachments as standalone feed items.
- **Preserve context.** Person, date or age, filters, and the originating
  scroll position survive opening a detail, adding a record, and going back.
- **Keep retrieval available.** Summaries and chapters do not replace search,
  date/age jumps, or a milestones-only view.
- **Activities are part of the person's life, not a separate app.** They show
  on the person's page and the family history, and they surface on the
  dashboard when a season is active.
- **Context-aware add.** The add button knows where you are. On Clara's page
  it adds for Clara; on a competition page it adds a result or photos for that
  competition.

## Navigation

Replace the hamburger with permanent entrances to the main browsing jobs and
one add action. Chat is a minor, experimental feature and does not receive a
primary navigation slot.

**Proposed phone order:** Home · History · **+** · Photos · Family.

**Desktop:** Family Record · Home · History · Photos · Family, with
**+ Add** and the account menu on the right. The destinations match mobile;
desktop can expose secondary links within the current section.

- **Home:** a changing overview of recent memories and timely highlights.
  A compact family strip provides shortcuts to people.
- **History:** the complete chronological family record, with search, date
  jumps, and person/type/tag filters. Always directly reachable on phones.
- **Photos:** visual retrieval and browsing, with people/date/tag filters.
- **Family:** a stable overview of people, plus clear links to family-wide
  **Growth**, **Same age**, and **Activities**. Own household comes first;
  linked people are visibly grouped separately. This is the full directory,
  not a second dashboard. Home's strip remains a shortcut.
- **+** opens the add sheet; it is an action, not a selected destination.
- **Account menu:** Chat (with unread count), Tags, Face review (with count),
  Settings, Import/Export, Admin where allowed, and Log out. Chat notifications
  still deep-link to chat. No separate persistent chat button is needed.

Validate the Family destination in phase 0. If it duplicates Home without
helping retrieval, fold its directory and stable Growth/Same age/Activities
links into Home and use fewer navigation items. Do not fill a spare slot with
Chat merely to reach five items.

The marketing header for signed-out visitors stays as is.

Old URLs keep working: `/family-timeline` → `/history`,
`/family-chart` → `/growth`, `/compare` → `/same-age`, and
`/dashboard` stays. Add `/family` if the proposed Family destination survives
the prototype. Keep `/activities` as the stable activities entrance.
Preserve meaningful query parameters in redirects. Ship each redirect with
its working destination, not before it exists.

## The add sheet

One sheet, opened from **+** anywhere. It replaces the menu's quick-add grid
and the dashboard's duplicate action cards.

- Photos
- Measurement
- Milestone
- Activity result

Show person chips when relevant, visibly preselected from the current person
page or an explicit selection. A global add can suggest the last person for
single-person entry, but the form must show who will receive the record.
Global photo upload starts without inherited person tags.
Measurement requires one person; photos can have multiple people or none.
Keep milestone cardinality consistent with the existing API.

Within an event, show its name and promote results/photos for that event.
During an event or the three days after it, suggest that event on the global
sheet. This changes shortcut priority, never action availability. If several
events qualify, let the user choose.

### Photo: upload first, describe after

1. Tap **Photos** and open the file picker immediately.
2. Begin uploads after selection using the existing processing queue.
3. Show optional people, note, and tags while uploading. An explicit person
   selection is visible and removable, with batch edits labeled as applying
   to all selected photos. Face-derived suggestions must be distinguishable
   from manual selections; preserve existing confirmation rules.
4. Keep each photo's EXIF date independently. A batch can span many dates.
   Where EXIF is missing, display the fallback date and its source with a
   change action. Do not silently assign a whole batch one date.
5. **Done** returns to the origin. Show which uploads completed, are still
   pending, or failed, with retry for failures. Leaving optional metadata
   must not silently discard the uploads.

Picking files can be the whole job. Additional description is optional.
A native background transfer and a web upload do not have identical lifecycle
guarantees; each must clearly communicate pending work.

### Measurement: a checkup is one entry

Show the selected person and their age on the chosen date, a compact
**Today ▾** date control, and height and weight together.

- Both measurements are optional, with at least one required.
- Units are remembered per person, falling back to the family's preference.
  Keep the existing lb/oz and ft/in inputs.
- The date menu offers Today, Yesterday, a date picker, and by-age entry.
- Show the last value and date as persistent helper text rather than a
  placeholder that disappears while typing.
- Existing `AddGrowthData` calls can save each filled field. If one succeeds
  and one fails, show that explicitly and retry only the failed field;
  never present a partially saved checkup as fully complete.
- **After saving, land on the result:** both saved measurements, with their
  percentiles and family comparisons where available. Include **Add another
  measurement** for entering another child's data and **Done** to return to
  the original view.
- Grouping height and weight is a display convenience, not an assumption that
  all measurements on a date are a single underlying record.

### Milestone

- Person chips, then the text box with focus.
- Category is a compact row or wrapping set of six labeled chips. A default
  category remains visible and easy to change.
- Date defaults to today, with the same date control.
- Photo attach and tags are behind **Add photos / tags**.
- Save opens the new milestone detail; Done returns to the origin.

### Activity result

Always available when the user can record results. Offer current/recent events
first and a route to all seasons and events for backfilling. An event shortcut
opens its results editor filtered to the family's entries.

## Home (dashboard)

Home answers "what's happening, and what would I enjoy revisiting?" It is not
the only entrance to the archive.

1. **Family strip:** compact avatars and names linking to people, with current
   ages where known. Pregnancy can retain its countdown. The full directory
   and linked-household grouping live in Family.
2. **One timely highlight:** an upcoming birthday, current competition, or
   On this day memory. Avoid stacking one large card per active season.
3. **Recent memories:** grouped summaries of recent occurrences, linking to
   History. Keep this visible without scrolling through maintenance prompts.
4. **On this day:** additional past-year photos and milestones when available,
   limited initially with a way to expand.
5. **In season:** compact current-season links, including upcoming/recent
   events and permitted result/photo actions. All activities remain reachable
   from Family even when nothing is active.
6. **Maintenance:** at most two dismissible prompts, such as faces to review
   or an optional measurement reminder. Show only actionable prompts the
   viewer has permission to resolve. Reminder thresholds are product choices,
   not medical guidance.

Distinguish **Recently happened** from **Recently added**. An old photo
uploaded today belongs at its original date in History, but can appear in a
clearly labeled Recently added section on Home. Do not falsify occurrence
dates to make backfilled records discoverable. Verify creation timestamps and
query support before committing that section; add an API query if needed.

The persistent **+** replaces duplicate generic add buttons. Contextual
actions on an event or measurement remain useful.

**Data:** Prototype with `ListPeople`, `GetFamilyTimeline`,
`GetFaceReview`, and activity/season calls as supported. A recent timeline
range cannot determine that someone has not been measured for six months;
reminders need their latest measurement dates. On this day can initially use
per-year date-range queries, but bound the work and use `GetDashboard` for
the finished aggregated response. Return occurrence and creation dates
separately when supporting Recently added.

## Person page

The person page is the main place to browse one life.

**Header:** photo, name, age/birthday, person switcher, and **+ Add** where
permitted. Put **Story · Photos · Growth · Activities** immediately below it.
Do not stack a growth dashboard and multiple comparison strips above the tabs.

- **Story:** a short overview followed by history grouped into age chapters.
  Include a compact active-season link and at most one relevant same-age
  preview. Keep a **Jump to age** control, search, and an easy
  **All / Milestones** filter; other type/tag controls can collapse into More.
  Provide oldest/newest order without restoring the old large filter wall.
  Chapter growth summaries must describe the actual recorded interval, not
  imply full-year growth from incomplete data.
- **Photos:** the photo grid scoped to this person, with date and tag filters.
- **Growth:** latest height/weight and their individual measurement dates,
  percentiles where supported, chart, measurement list, and optional faint
  sibling curves. Detailed growth summaries belong here.
- **Activities:** current and past seasons, routines, and appearances, folding
  in `/person-activities/:id`. Redirect that URL to this tab when ready.

An **At this age** preview uses the person's current age only when meaningful
matches exist. For the oldest child with no matches, omit it. Do not reverse
the meaning to a younger child's hypothetical future. Older records of that
child still provide useful comparison anchors.

Switching person keeps the current tab where applicable. Preserve an explicit
age filter as an age, not a calendar year; if the new person has not reached it,
show that clearly rather than silently substituting another age.

## Same age (replaces Compare)

This is the "what were they like at this age?" lens as a page of its own. It's
also the component behind the strips on the person, photo, and milestone pages.

**Around age 3 years 4 months** — previous/next, direct entry, and slider.

| Person | Photos | Measurements | Milestones |
| --- | --- | --- | --- |
| Clara | Preview, age/date on each | 38.5 in at 3y 4m; 32 lb at 3y 4m | Wrote her name · 3y 4m |
| Jake | Preview, age/date on each | 37 in at 3y 3m; 31 lb at 3y 5m | Started preschool · 3y 5m |

Values are illustrative. Each cell links to its records.
**Include people without matching records** reveals otherwise hidden rows.

- One age at a time, one row per person who has reached that age. That fixes
  the old page's misaligned columns.
- Proposed navigation steps are one month under 2, three months from 2 to 6,
  and six months after that, plus direct age entry (including weeks for babies).
  Navigation steps and match tolerance are separate rules. Prototype newborn
  and sparse-data cases before fixing photo/milestone windows. Existing growth
  tolerances are a starting point for measurements, not all record types.
- Label the target as **Around this age** and show each matched record's actual
  age and date. Do not silently widen an empty window or imply exact matches.
- On direct entry, default to the youngest own child's current age. When
  reached from a record, use that record's person and exact age. Without a
  suitable child/birthday, offer explicit person/age selection.
- Each row shows nearest photos, measurements, and milestones within their
  respective windows. Extract a shared nearest-measurement-by-age helper from
  `growthComparison.ts`; `computeFamilyComparisons` currently requires a real
  target measurement and also computes same-value comparisons. Do not fabricate
  a measurement to browse an arbitrary age.
- Hide people without matching records by default; an explicit control can
  reveal them. Never label an inaccessible record scope as missing data.
- Default to own-household people. Offer **Include shared relatives**, retaining
  the viewer's choice and honoring permissions per record type. Do not assume
  everyone on a roster is a sibling.
- It can be opened from anywhere with a person and an age:
  `/same-age?age=40m&from=<personId>`.

**Where it shows up in context:**
- **Measurement detail:** as now, with a link to "see everything at this
  age."
- **Photo detail:** a same-age strip with a visible anchor person. Use the
  originating person's context if present; for a group photo, offer tagged
  people as choices. Do not use an invisible first-tag ordering.
- **Milestone detail:** a new read-only milestone page (only an edit page
  exists today). It shows the milestone, its photos, and "at this age, Jake
  was…". `GetMilestone` already exists.
- **Person page:** the strip described above.

This answers "what was everyone doing around this age?", not "at what age did
everyone reach the same milestone?" Matching milestone meanings is out of scope.

**Data:** Existing timeline and per-person photo queries can prototype this.
Verify scope coverage for linked people; a household-wide endpoint must not be
assumed to include every visible person's data. `GetSameAge` should own the
finished window rules and return actual record ages/dates and scope availability.

Replace the old compare UI and its frontend caller; retain `/compare` as a
redirect to the working new page. Leave the proc in place until a release has
passed, and verify supported mobile builds no longer call it before removal.
Check `docs/mobile-api.md`; elapsed time alone does not establish compatibility.

## Growth

The family chart becomes **Growth**, reachable from Family and person Growth
tabs. It keeps its own deep-linkable route but does not need a primary tab:

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
- **Group explicit relationships before grouping by day:**
  - Milestone attachments stay with their milestone; event/appearance photos
    stay with their activity summary. Remove these photo IDs from the remaining
    daily mosaics only when their parent summary is visible in the current filtered
    view; filtering out an event must not hide its photos from a photos-only view.
    A photo attached to multiple meaningful records can remain
    in those contexts without also becoming a standalone history item.
  - Remaining photos on the same day become one mosaic tile ("12 photos · Clara,
    Jake"). Open the corresponding photo set, preserving the person/tag scope;
    a day-only filter must not unexpectedly include unrelated photos.
  - Height and weight for one person on one day can become one "Measurements"
    row with both values and percentiles. If multiple values of either type
    exist, show the count and let the user expand; never discard extra readings.
  - Birthdays are dividers ("🎂 Jake turned 6"), not cards.
  - Milestones stay as full cards. They're the rarest and most meaningful
    records, and they should stand out.
  - Activity events show as one card per event, with the family's results
    ("Regionals · Clara 1st place, Solo Jazz") and photos.
- **Filters** collapse to one row: person chips, and a "More" popover for
  type and tags. Search stays.
- Keep date jumps and paging (`lib/photoPages.ts` where applicable). Merge
  groups across page boundaries, deduplicate by stable IDs, and make incomplete
  counts explicit until a group's records are loaded.
- History orders by occurrence date. Recently added belongs on Home; the same
  historical event must not move when edited or uploaded later.

The person page's Story tab uses the same components, grouped by age instead
of by month.

**Activities data:** `GetFamilyTimeline` doesn't return activity events.
For a prototype, call `GetPersonSeason` per person and merge by stable event
and appearance IDs so a shared routine is not duplicated for each child.
For the finished view, add events and appearances to `GetFamilyTimeline` under
the same date range, with permission-aware summaries and grouping metadata.

## Activities

Activities have a stable entrance from Family, including outside active seasons.
The activities overview distinguishes **Browse** (current/past seasons, events,
routines, and people) from **Manage** (programs, seasons, entries, rosters).

- **Person page:** Activities tab and a compact current-season link in Story.
- **Home:** current/recent event shortcuts.
- **History:** event summaries with results and linked photos.
- **Add sheet:** Result is always available where permitted; current events
  get priority without excluding late entry.
- **Setup:** retain existing management pages, reached through Manage within
  Activities. Do not make the account menu their only entrance.

The existing season, competition, and routine layouts are starting points.
Add contextual photo/result actions and explicit cross-links:
person → appearance → event → another participant's appearance, and
appearance → routine → its results across the season. Reuse existing
relationships; a new general-purpose occasion entity is out of scope.
Show links only where the destination is accessible.

## Detail pages

Measurement, photo, and milestone detail share a basic layout: record, permitted
actions, then useful context. Context can be the person's story, a linked event
or milestone, a routine's season history, or Same age. Do not force an age
comparison onto every record when its occasion is more useful.

### Navigation state

- Put bookmarkable state in the URL: person/tab, date or age, and meaningful
  filters. Opening an item retains the source list's scroll position.
- Back returns to the actual origin with its filters and position, not always
  to the profile. A deep link without an origin uses an explicit parent link.
- Add carries visible person/event context. A historical viewing date must not
  silently become the entry date; offer **Add for this date** explicitly.
- Cancel returns without changing the source view. Save/Done behavior is
  specified per entry flow above. Preserve unfinished text when a recoverable
  error occurs.

### Linked people and read-only views

Use the existing model in `docs/permissions.md`; this redesign does not change
who can see or edit records.

- Label linked people and show only the record scopes available to the viewer.
- Suppress edit/add/manage actions for read-only people. A global Add may still
  operate on the viewer's own writable household, with its scope explicit.
- Do not show measurement reminders or face-review tasks the viewer cannot act on.
- Distinguish inaccessible scopes from permitted scopes with no records.
- Chat remains household-only; a family link is not membership.
- Validate an own-household editor, a linked viewer with photos/milestones only,
  and a linked viewer with all record scopes in the same core journeys.

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
| Primary destinations: Home, History, Photos, Family (pending phase 0) | Native tabs with the same destinations; **+** is a separate action presenting a sheet, not a content tab |
| Add sheet | a sheet with a medium detent |
| Person chips | the same horizontal chip row |
| "Today ▾" date control | a menu (Today, Yesterday, Pick…) plus a date picker |
| Upload-first photos | `PhotosPicker`, then upload in the background |
| Account menu, including minor Chat entry | a toolbar profile/menu button; chat notifications still deep-link |
| Same-age stepper | a stepper or slider, with the same age steps |

- **No hover-only interactions.** Everything has to be reachable by tapping,
  which is right for phone-width web anyway.
- **Wording is shared.** Page names, button labels, empty states, and nudge
  text should be identical, so a family using both doesn't notice the switch.
  Keep them in one place on the web (a small `lib/copy.ts`) so the app has a
  single list to copy from.
- **Share behavior, not pixel layouts.** Use native navigation, pickers, and
  accessible controls while preserving destinations, labels, and task outcomes.
- **One owner for domain rules.** Put final age-window, reminder, visibility,
  and grouping rules in the aggregate backend APIs. Clients render the results.
  Prototype helpers can live as pure functions in `lib/` with fixture tests;
  carry those fixtures into server tests when moving the rules. Avoid permanent
  independent web and iOS implementations of the same matching logic.

**Additive backend work:** both mobile web and native apps benefit from bounded
requests and consistent grouping. Add these as their corresponding views ship,
using client fan-out only as a temporary prototype fallback:

- `GetDashboard`: permitted people, latest-measurement dates, actionable nudges,
  activity highlights, On this day, and recent occurrences. Add a separately
  ordered recently-created query/section if timestamps support it. Date-sensitive
  results use the viewer's intended local date, not the server's UTC day.
- `GetSameAge(age, fromPersonId, includeShared)`: matched records per visible
  person, with actual ages/dates, per-type windows, and scope availability.
- Activity events and appearances in `GetFamilyTimeline`, under the same date
  range, for History.

Keep changes additive and preserve existing response contracts for older app
builds. Document new contracts in `docs/mobile-api.md` before the app depends
on them. Verify available timestamps, associations, and pagination rather than
assuming the current responses contain everything a summary needs.

**Each phase ships web first, then app.** The web version settles the design.
The app follows once it's been used for a bit, rather than both being built
at the same time and drifting apart.

## Phases

Each shipping phase leaves the app coherent. Validate the flow before fixing
the permanent shell.

0. **Journey prototype.** Use realistic dense and sparse records at phone width.
   Walk through the five journeys below. Decide whether Family is distinct
   enough from Home to warrant a tab, and settle Back, Save, and scope behavior.
1. **Shell and stable entrances.** Implement the validated navigation, account
   menu with secondary Chat, add sheet, and Family overview if retained.
   Existing History/Growth/Activities views can serve their new entrances.
   Redirect URLs only when the destination works. Update click-based smoke
   selectors in the same PR.
2. **Fast entry.** Shared person/date controls, combined measurement entry and
   result, upload-first photos, milestone entry/detail, and always-available
   result entry. Include partial failures and late/backfilled entries.
3. **Person browsing and History.** Compact profile, tabs, milestone retrieval,
   age/date jumps, related-record grouping, and activity cross-links. Add the
   timeline API support needed for consistent grouping and pagination.
4. **Same age and Growth.** Age lens/API, explicit anchor selection, matching
   labels, contextual strips, and clearer Growth charts. Redirect Compare when
   the replacement ships.
5. **Home.** Build the overview using the established summary components and
   aggregate API. Add recent memories, highlights, On this day, and restrained
   maintenance prompts.
6. **Cleanup.** Verify importers before deleting old profile tabs, unified
   timeline code, compare code, and orphaned styles. Keep legacy redirects.
   Check mobile API callers before removing any old proc.

Each phase ships web first, then native after the flow has been used and settled.

### Journey acceptance checks

| Journey | What the prototype must demonstrate |
| --- | --- |
| Enter a checkup | Select the child once, save height/weight, see the result, add another child's measurement, and return to the origin. Handle one-field save failure without duplication. |
| Upload mixed photos | A batch spans children and dates; individual dates survive, no unrelated last-used child is silently tagged, and failures remain visible/retryable. |
| Find an old milestone | From Home, reach a person's milestones, jump to the right age, open one, then return with filter and position intact. |
| Explore siblings at one age | Open from a group photo, choose the anchor child, see actual match ages, change age, and handle no matches honestly. |
| Review a competition | Reach a past event across multiple children, inspect a routine's season history, add late results, and return without losing the event context. |

Also exercise a linked read-only viewer and a household with little history:
no dead-end edit actions, inaccessible-scope nudges, or empty dashboards made
entirely of prompts. Test keyboard/touch navigation and browser Back as part
of the flows, rather than only checking that each page renders.

## Open questions

- **Family as a destination:** validate the proposed directory plus family-wide
  views against keeping all stable entrances on Home. Chat remains secondary
  in either version.
- **Same-age windows:** choose and document separate photo/milestone tolerances,
  including newborn and sparse-history cases. Show actual ages regardless.
- **Recently added:** verify creation timestamps and retrieval support before
  committing its exact contents and pagination.
- **Measurement reminders:** decide whether they are useful enough to enable
  by default; thresholds and dismiss/snooze behavior need deliberate choices.
