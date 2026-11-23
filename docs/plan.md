# Roadmap

Family Record preserves who is in a family, how people grew, what they did,
and what it looked like. The product direction is to make those records easy
to capture, durable, and meaningful in context: across a person's life,
between siblings, and across generations.

Nothing here is a release commitment. Household management (calendars, chores
and allowance, meals, location tracking) is deliberately out of scope; those
proposals are kept in [Not planned](not-planned.md).

## Planned

Video follow-ups, worth doing if they stay simple:

- Resumable S3 multipart uploads above ~100 MB.
- Use the transcript: suggest a title, tags or people mentioned, for the family
  to accept or edit.

## Ideas

Not scoped and not planned for now; kept in case we return to them. "New
entity" means the checklist in
[Adding a new domain entity](#adding-a-new-domain-entity).

- **Christmas-card updates.** Periodic "here's what we've been up to" posts
  (text, photos, the people involved) shared with specific people by a
  revocable link, built for link previews, with bounded reactions and comments
  and an optional email to outside recipients. The full scope was in
  `docs/plans/sharing.md`, removed after the family summary page shipped (see
  `git show f61f468:docs/plans/sharing.md`).
- **Interview notes while conducting.** A note field on the conduct screen;
  notes can only be added afterwards, from Edit answers.
- **Same-age by occasion.** Comparisons by recurring occasion (first day of
  school, Halloween) alongside age comparisons, clearly labelled as such.
- **Suggested event collections.** Propose groupings of a birthday, a trip or a
  competition from photos close together in time, with shared people and places
  as supporting evidence. The family accepts, edits or dismisses; nothing
  becomes a record automatically.
- **In memoriam.** Mark someone as passed without deleting them.
- **Family tree view.** Relations are already a graph with derived labels;
  drawing it is mostly frontend work.
- **Pregnancy record.** `IsPregnancy` exists with nothing to record against it.

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
