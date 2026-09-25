# Roadmap

Family Record is a record of a family: who is in it, how the kids grew, what
they did, and what it looked like. The short term is making that record solid.
Anything that manages the household day to day, rather than recording it, is a
future idea and lives in [`plans/`](plans/).

## Short term: make what's here solid

The UI pass and the landing page rework from the previous version of this plan
have shipped. What's left is correctness and rough edges in features that
already exist.

### Finish face review

The backend for reviewing faces is in progress (`backend/faces.go`: grouped
unknown faces, assign, reject, dismiss), but nothing on the web calls it yet.
The privacy page already promises that "a tag is marked so you can tell an
automatic suggestion from one you made yourself", and `GetPhoto` returns plain
`[]Person`, so the photo page has no way to show that. Before face tagging
counts as done:

- A review screen for unknown and auto-tagged faces.
- Auto tags marked as auto tags on the photo page, with a one-tap confirm or
  reject.
- The privacy and support pages checked against what actually shipped.

### Person lifecycle

- **Merging drops activity history.** `MergePeople` moves growth, milestones,
  photo tags, faces, relations, and family rows, but not activity roster rows
  (`EntryMember`) or per-person results (`Result.PersonId`). After a merge
  those still point at the deleted person, so the merged person's seasons and
  results disappear. Move them, and extend `TestMergePeople` to cover
  activities.
- **There's no way to delete a person.** A mistaken add can only be merged
  into someone else. The privacy page says a face summary "is deleted when that
  person is deleted", which currently can't happen. Add `DeletePerson` that
  cascades through everything merge already knows how to find, plus activity
  rows, shares, and faces. It needs a cross-family isolation test and a
  confirmation that says what will be removed.

### Family timeline includes activities

A person's timeline shows performances and results. The family timeline shows
milestones, measurements, photos, and birthdays, but no activities.
`GetFamilyTimeline` doesn't return them either. The landing page describes the
timeline as pulling everything together, so it should.

### Loading everything doesn't scale

`GetFamilyTimeline` and `ListFamilyPhotos` return every row the family has,
and the photos page filters on the client. That's fine at a few hundred photos
and won't be after a few years, which is exactly how long a record is meant to
last. Add windowing or pagination before it hurts. The mobile app syncs through
`GetFamilyTimeline`, so any change there goes through `docs/mobile-api.md`
first.

### Dead AI code

`backend/ai.go` is a Gemini client that nothing calls, and `.env.example` still
lists `GEMINI_API_KEY`. It also contradicts "no data leaving the host". Delete
both.

### Photo rough edges

- Web upload takes one file at a time. Recording a trip or a birthday means
  uploading one photo, then the next. Accept several files and apply people,
  tags, and date to all of them.
- The photo viewer has no previous/next, so looking through photos means
  going back to the grid after each one. Add arrow keys and swipe, following
  whatever filter the grid had.

## Future

Each future feature has its own file. They split into two groups.

**Extending the record.** These fit what the app already is. They're big enough
that they aren't short-term work.

- [Video](plans/video.md): a second kind of media in the photo pipeline, with
  the bytes in object storage.
- [Sharing outside the family](plans/sharing.md): a public family summary page
  and Christmas-card style updates.

**Managing the family.** These are day-to-day logistics rather than record.
They'd be interesting, but they turn the app into a different product, so they
wait until the record is solid and there's a deliberate decision to widen it.

- [Shared calendar](plans/calendar.md)
- [Chores and allowance](plans/chores-and-allowance.md)
- [Meals and recipes](plans/meals.md)
- [Location sharing](plans/location.md)

Ideas that fit the record but haven't been scoped are collected in
[`plans/record-ideas.md`](plans/record-ideas.md).

### Adding a new domain entity

Most of the above means adding a new domain entity. In this codebase that
takes:

- a vbolt bucket plus the indexes to query it
- a versioned `vpack` pack function
- family scoping, with a cross-family isolation test
- `RegisterProc` handlers (the TypeScript client generates itself)
- coverage in export and import, so a family can still take their data with
  them
- handling in person merge and delete, if the entity refers to a person

## Not planned

Written down so these don't get reopened by accident:

- No ads, no third-party analytics, no data leaving the host.
- No public feed, no follower graph, no discovery between families. The sharing
  plans are links a family hands to a person, and that is as far as sharing
  goes.
- No dependency the app can't start without. New features follow the same
  rule: one binary, one database file, a directory of images.
