# Same age: portrait-first comparison

Status: proposed; this document does not implement the feature or make it a
release prerequisite.

## Purpose

Answer “What did everyone look like as a newborn?” with a calm, immediately
comparable set of portraits. Growing up answers how one child changed over
time; Same age answers how several children looked at one age. Give these
existing destinations distinct jobs rather than add another comparison page.

## Existing foundation

- `frontend/pages/same-age/same-age.tsx` currently stacks `SameAgeMontage`
  above `SameAgeRows`, mixing portraits with measurements and milestones.
- `frontend/components/SameAgeMontage.tsx` already uses ranked portraits,
  face crops, actual ages, and temporary “Another photo” choices. It currently
  renders nothing when fewer than two people have portraits.
- `backend/same_age.go` supplies portraits and records for age windows, plus
  available ages. Available ages currently describe records, not specifically
  usable portrait comparisons. A direct visit chooses a rich comparison;
  explicit ages and profile entry points preserve their requested age.
- `frontend/pages/profile/profile.tsx` contains Growing up and its compact
  Story preview. Keep the full timeline in Photos and retain that preview.

Reuse these paths and existing portrait ranking. This is chiefly a focused
presentation and navigation change, with targeted age-selection adjustments.

## Proposed experience

### Portraits is the default Same age view

Keep the existing Family → Same age entry. Show a descriptive heading such as
“Everyone as newborns,” a compact age control, and one large portrait per child.
Use a two-column grid on typical phone widths and more columns on wider screens;
allow an accessible single-column fallback when text size or width requires it.
Do not require swiping between children to compare their faces.

Each tile shows only the portrait, name, and actual age at capture (for example,
“8 days old”). Use consistent crop dimensions and existing face-crop support.
Opening the tile reveals the full photograph and an accessible way to choose
another eligible photo. Avoid always-visible buttons, dates, badges, record
counts, and measurement summaries on every tile. Photo alternatives remain
temporary; this change does not introduce saved portrait preferences.

Use a deterministic person order, preferably the existing family order, and
preserve relative order when ages change. Do not sort by photo quality or age
coverage. Default to the existing family comparison population; do not require
people configuration before viewing. Avoid implying that every family member
is a sibling; any future person filter belongs behind a secondary control.

### One selected age, compact controls

Offer Newborn, 3 months, 6 months, and 1 year shortcuts plus an age picker.
Show one age at a time. Consolidate controls instead of stacking shortcuts,
picker, slider, and verbose coverage summaries in the primary layout.

Keep fixed-position previous/next controls and navigation among useful recorded
ages. In Portraits, useful ages must reflect eligible portraits, not merely
measurements or milestones. Preserve the latest requested age during rapid
input, visible loading/retry feedback, and an honest relationship between the
heading and the portraits currently displayed.

On an ordinary visit, preserve the existing principle of opening a useful
comparison, selecting by portrait coverage for this view. Newborn remains one
tap away. Contextual links and explicit selections must retain the requested
age even when it has poor coverage. Never silently move a requested newborn
comparison to an older age.

### Details is secondary

Provide a clearly labeled Portraits / Details switch within Same age. Details
retains the existing measurements, milestones, and related records. Switching
views preserves the selected age and people; do not repeat the portrait gallery
above the Details rows or append all records beneath Portraits.

Details may expose ages with other records even when portraits are missing.
When switching to Portraits at such an age, retain the age and explain the gap;
do not change it automatically. Keep view and age in navigable URL state so
reload, browser back, and contextual entry behave predictably.

### Connection to Growing up

When opening a Growing up portrait, offer “Compare at this age” and navigate to
Same age → Portraits with that age and person context. Preserve the established
full-photo interaction; place the comparison action in the opened portrait
context rather than overlaying every timeline thumbnail. Use the actual photo
age and documented age-bucket mapping, not today's age or its calendar year.

Do not add another permanent comparison gallery to Story, Photos, or Home.
The existing Family entry and contextual action provide discoverability.

## Age accuracy and sparse data

- Proposed Newborn definition: birth through 27 completed days (age >= 0 and
  < 28 calendar days). Confirm this product definition during implementation
  review and document it in the UI. Do not treat the existing zero-month window
  as equivalent without checking its bounds.
- Apply the eligibility window before portrait ranking and alternative-photo
  selection. Never substitute a three-month photo for a missing newborn photo.
- For other ages, reuse the existing documented windows initially. Show actual
  capture ages so “6 months” does not imply all children were photographed on
  exactly the same day of life. Audit window boundaries and date-only handling.
- Use the same eligibility rules for available-age discovery and results.
  Newborn coverage must use the newborn window, not a rounded month count.
- Missing birthdays or photo dates must not be guessed. If approximate dates
  are supported later, label uncertainty and only classify a newborn image
  when its possible age range fits the window.
- Show pictured children without large empty cards. A small disclosure such as
  “No newborn photo for 2 children” can list missing names on demand. Distinguish
  missing dates or unavailable data when known; do not mislabel a failed request
  as absence of photos.
- With one portrait, show it and explain that other children's photos are
  missing. With zero, show a concise age-specific empty state and a way to pick
  another age. Keep the requested age selected.

## Implementation sequence

1. Review the current Same age navigation, portrait ranking, Growing up photo
   links, and date helpers. Identify the smallest shared changes needed; avoid
   creating another ranking system or broad abstraction layer.
2. Define newborn eligibility and portrait-based available-age coverage in the
   backend, retaining family authorization and existing Details behavior.
   Extend the generated client contract only if necessary; do not hand-edit it.
3. Refactor the existing Same age page into Portraits and Details. Enlarge and
   simplify the montage, support zero/one-photo states, and move alternative
   selection into the portrait interaction.
4. Add age shortcuts, view/age URL state, and the contextual Growing up action.
   Preserve existing deep links and recent async-navigation fixes.
5. Review realistic desktop and mobile layouts before considering further
   comparison controls. Keep this first iteration narrow.

No new persistent entity, migration, AI integration, or external service is
expected. Preserve existing family access checks and photo authorization.

## Acceptance and validation

- A family can reach everyone's newborn portraits from Same age with one age
  shortcut, and from Growing up at the selected portrait's age.
- Portraits shows one large photo per pictured child, names, and actual ages;
  measurement and milestone rows appear only in Details.
- A six-child family remains readable in two columns on a typical phone, with
  stable person ordering and no horizontal scrolling or moving arrow buttons.
- Birth-day and day-27 photos qualify as newborn; day-28, pre-birth, undated,
  and three-month photos do not. Validate calendar boundaries across time zones.
- Age availability agrees with portrait eligibility. Ages containing only
  measurements remain usable in Details and produce honest Portraits gaps.
- Zero, one, and several matching portraits render intentionally. Missing data,
  missing dates, and request failure are not conflated.
- Switching views, reloading, following a deep link, and using browser back
  preserve the intended age. Rapid age changes cannot display stale results
  under a new heading.
- Full-photo viewing and alternate selection remain keyboard accessible, with
  visible focus, descriptive labels, and no hover-only controls.
- Add targeted eligibility and navigation regression coverage to the existing
  Same age tests, plus manual mobile/desktop review with realistic photos.

## Outside the first iteration

Children-by-ages matrices, recurring-occasion comparisons (Halloween, etc.),
saved portrait overrides, montage export, sharing, book generation, automatic
resemblance analysis, and a native iOS implementation are separate follow-ups.
This plan refines the existing age-based sibling montage described in
[Offline analysis](../offline-analysis.md); it does not replace that plan's
remaining occasion-based ideas.
