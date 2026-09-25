# Record ideas

Candidate features that fit what the app already is: a record of a family, not
a tool for running one. Nothing here is committed. It's a list to argue over.
Each idea notes what it builds on, and "new entity" means the full checklist in
[`../plan.md`](../plan.md#adding-a-new-domain-entity).

## Getting more out of what's already recorded

These add no new data. They present what's already stored in a new way, which
makes them the cheapest ideas on this list.

- **On this day.** A dashboard card showing what happened on this date in past
  years: milestones, photos, measurements. Optionally sent as a push or
  in a weekly email. For a record that's years deep, this is probably the
  single best reason to open the app.
- **Year in review per child.** One page per child per year, generated from
  existing data: growth over the year, milestones, the best photos, seasons and
  results. It works naturally as a birthday-week page ("Age 7").
- **Upcoming birthdays.** Birthdays already show on the family timeline. Add
  them to the dashboard with the age they're turning, including linked
  households' people (grandparents).
- **Search everything.** Search currently covers milestones only. Extend it to
  photo titles and descriptions, activity notes, entries, and events.
- **Timeline filters.** Filter the family and person timelines by tag, type,
  and date range. Tags exist on milestones and photos but can't narrow a
  timeline.
- **Photo series.** A tag like "first day of school" shown as a side-by-side
  across years or across siblings. The compare page already aligns growth by
  age, and this is the photo version of the same idea.
- **Print / PDF export.** A per-child or per-year book laid out for printing.
  It's the "take your data with you" story in a form Grandma can hold.

## Richer records of people

- **Birth details.** Birth time, weight, length, place. Weight and length seed
  the growth chart at day zero, which is currently a hole at the most
  interesting end of the curve.
- **Head circumference.** WHO has head-circumference charts for ages 0–2, and
  pediatricians measure it at every early visit. `MeasurementType` is only
  height and weight today.
- **Nicknames and a short bio.** A few free-text fields per person: what they
  go by, what they're into right now. They also feed the public summary page
  in `sharing.md` if that ships.
- **Dates of death / in memoriam.** Relations and linked households already
  reach grandparents. A multi-generation record needs a way to mark someone as
  passed without deleting them, and the dashboard shouldn't keep listing them
  as current.
- **Family tree view.** Relations are already a graph with derived labels.
  Drawing that graph is mostly frontend work.
- **School years.** School, grade, teacher, and a class photo per child per
  year. This might not need a new entity: a school year is structurally an
  activity season, so try it as an activity kind first.

## New kinds of entries

- **Quotes.** "Things they said" is one of the most-kept parts of a family
  record and doesn't fit a milestone well. At minimum this is a milestone
  category. A first-class entry with speaker and context is nicer.
- **Annual interview.** The same ten questions every birthday (favorite food,
  best friend, what do you want to be) and a view comparing answers across
  years. New entity, small.
- **Letters to the future.** A parent writes a letter to a child that stays
  sealed until a chosen date or age. New entity, and it needs a notification
  when it opens. It means a lot to families and the build is small.
- **Artwork and documents.** Report cards, certificates, drawings. These can be
  photos with a kind or tag plus a dedicated view, rather than a new pipeline.
- **Health record lite.** Vaccinations, allergies, doctor visits. It fits the
  record, and "health" is already a milestone category. It also brings real
  sensitivity and scope creep, so it needs discussion before building.
- **Pregnancy record.** `IsPregnancy` exists, but there's nothing to record
  against it. Add ultrasound photos and a weekly bump photo series, and carry
  them into the child's record at birth.

## Family members contributing

- **Who recorded it.** Milestones and growth don't store who added them.
  Contributors (nanny, grandparents) are a real role, and "added by Dana" is
  useful context.
- **Reactions and comments on entries.** Family members and linked households
  react to or comment on a milestone or photo. These are bounded like chat:
  internal only, no feed.
- **Activity notifications.** Push currently only covers chat. Add an opt-in
  "new milestone or photos for Clara" notification, which grandparents in
  linked households would want most.
- **Weekly digest email.** A summary of what was added this week, sent to
  opted-in members and linked households. It reuses the mail worker and the
  "what's new" query that on-this-day already needs.

## Durability

- **Scheduled export.** A reminder to download a full export, or an automatic
  one to a destination the family picks. A record meant to last decades
  shouldn't depend on one VPS.
- **Original photo download.** Make sure the full-resolution original can be
  downloaded from the photo page, not only through the full export.
