# Roadmap

Family Record preserves who is in a family, how people grew, what they did,
and what it looked like. The first release should also turn those records into
beautiful books: the original baby-book idea, expanded to people, families,
years, and recurring occasions.

## Active plan and future backlog

| Document | Purpose |
| --- | --- |
| [First release](plans/v1.md) | Books are the main remaining feature; artwork and annual interviews complete the v1 feature scope. |
| [Future ideas](plans/future.md) | One categorized backlog with preserved technical notes. Video is wanted after v1; the rest are candidates. |

Only the first-release plan defines the remaining v1 feature commitments.
Future ideas are not launch prerequisites. Output formats and the exact
book controls still need a prototype; the book feature itself is committed.

## Completed work and reference

The web redesign, landing page, photo/timeline paging, multi-file web upload,
quotes, local analysis/search, and code compression phases 1–6 are recorded as
built. Their design descriptions are not unimplemented task lists.

| Reference | What it preserves |
| --- | --- |
| [UI redesign](reference/ui-redesign.md) | Original journeys, design decisions, implementation differences, and open validation questions. |
| [Offline analysis](reference/offline-analysis.md) | Implemented search, auto tags, photo features, milestone matching, and person insights, including prototype results. |
| [Code compression](reference/code-compression.md) | Completed cleanup phases and the architecture principles behind them. |

Verify deployment separately from code completion. The analysis reference
records staging setup and a production daemon/models follow-up; consult
[deployment](deployment.md) and current servers before treating it as resolved.
Native iOS parity has its own repository and must be reviewed there.

Existing correctness bugs and rough edges remain maintenance work. Resolve
navigation, age windows, Recently added, and reminder behavior when current
usage shows a problem; these are not a new feature expansion for v1.

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

- Household management is outside scope: calendars/scheduling, chores/allowance,
  meals/recipes, and live location tracking. Former proposals remain in Git history.
- No ads or third-party analytics.
- No public feed, follower graph, or discovery between families.
- Private records stay private by default. Creating a book is not publishing it.
- No AI requirement for capturing, browsing, or generating a book. Suggestions
  are reviewable and never silently become facts.
- External storage, public sharing, and any external AI processing are explicit
  choices; none silently apply to existing records.
- Optional dependencies degrade cleanly. The core is a self-hosted app with one
  database file and local images; deferred video proposes a bulk-storage exception.
