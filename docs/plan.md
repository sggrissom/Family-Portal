# Roadmap

Family Record preserves who is in a family, how people grew, what they did,
and what it looked like. The product direction is to make those records easy
to capture, durable, and meaningful in context: across a person's life,
between siblings, and across generations.

Household management is outside the current scope: calendars and scheduling,
chores and allowance, meals and recipes, and live location tracking are not
planned. Their former proposal files have been removed; Git history retains
them if that direction is deliberately reconsidered.

## Shipped foundation and remaining follow-up

The web UI redesign, landing page rework, photo and timeline paging, and
multi-file web upload have shipped. The redesign includes fast entry, grouped
History, person browsing, Same age, Growth, and the changing Home dashboard.
Do not treat their original design descriptions as a list of unbuilt features.

- [UI redesign](plans/ui-redesign.md): retained as the design reference and
  implementation record. Native iOS parity follows in its separate repository;
  verify its current status there.
- The documented remaining web gap is photos without people tags in History.
- Resolve the redesign's open product questions as needed: same-age windows,
  Recently added behavior, reminder defaults, and navigation validation.
- Fix correctness bugs and rough edges in existing record features as found.

## Planned direction

These are proposals, not release commitments or prerequisites for launch.

| Plan | Purpose |
| --- | --- |
| [Approximate dates and ages](plans/approximate-dates.md) | Record partial historical knowledge honestly and preserve uncertainty in browsing and comparison. Manual entry first; optional AI estimation only after evaluation. |
| [Video](plans/video.md) | Videos alongside photos, with private object storage for the bytes. |
| [Sharing outside the family](plans/sharing.md) | Explicitly published family summaries and Christmas-card updates shared by link. |
| [Offline analysis](plans/offline-analysis.md) | Search, tags, photo features, milestone matching, and person insights are built. Next proposals: suggested event collections and sibling comparison montages. |

The two next analysis proposals are detailed in
[Offline analysis](plans/offline-analysis.md#next-proposals-collections-and-sibling-montages):

- **Suggested event collections:** reviewable historical collections inferred
  from photo dates, people, places, and similarity (medium complexity).
- **Sibling comparison montages:** photos side by side at a comparable age or
  recurring occasion, with actual ages and manual photo choices (low to medium).

Unscoped candidates live in [Record ideas](plans/record-ideas.md). They are
options to evaluate, not an implementation checklist. Approximate-date support
is the foundation for historical and cross-generation views.

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
