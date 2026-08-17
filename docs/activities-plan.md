# Competitive Activities Plan

Tracking dance competition seasons — competitions, routines, adjudications, placements,
awards, and photos — on a schema that generalizes to sports seasons without a later
migration.

## Design decisions

These were settled before writing the plan; the rest of the document follows from them.

| Question | Decision |
| --- | --- |
| Adjudication levels | **Free text.** No ordered scales, no cross-competition normalization in v1. |
| Routine lifecycle | **Season-scoped, no lineage.** An Entry belongs to exactly one Season; carry-over routines are re-created. |
| Generalization | **Generic bones, dance-only UI.** Schema is activity-agnostic from day one; only dance vocabulary ships. |
| Data entry | **Manual forms.** AI import is a later phase, not v1. |
| Loose awards | **Appearance-only.** Every result hangs off a routine at a competition. |
| Sharing | **Yes** — add `ScopeActivities` to the family-link scope mask. |
| Extra fields | **Photos and free-text notes.** No schedules/call times, no costs. |
| UI | Low priority. Backend-first; minimal screens in a late phase. |

### Consequence of free-text adjudications

A season view can **list** results and **count** them by exact label ("3× Diamond, 5× High
Gold"), but it cannot rank or trend them — `Diamond` and `Gold` are just strings, and
different competitions use different vocabularies. Two cheap mitigations that keep the door
open:

1. **Suggest prior labels.** A `ListActivityVocabulary` proc returns the distinct values the
   family has already used for adjudications, styles, divisions, and hosts. Autocomplete
   from it so `High Gold` doesn't become `high gold` / `Hi-Gold`. Without this, even
   counting is unreliable.
2. **Normalization stays additive.** If ranking is wanted later, add `ScaleId` and
   `TierRank` to `Result` behind a `vpack.Version` bump — the same pattern already used for
   `PackImage` v3 and `PackPerson` v4. No data migration, no reshaping.

## Model

Six entities plus join tables. Dance terms in parentheses.

```
Activity (a program: "Dance")
└── Season ("2025–26 Competition Season")
    ├── Event (a competition: "Nuvo Nashville")
    └── Entry (a routine: "Solo — Rise Up", "Senior Large Jazz")
        └── EntryMember → Person   (roster; many kids per entry, many entries per kid)

Appearance = Entry × Event        (this routine, at this competition)
└── Result                          (adjudication | placement | award | score)
```

`Appearance` is the hinge. It is what makes both of the requested views cheap:

- **"How did this dance do across competitions?"** → walk `AppearanceByEntryIndex`.
- **"How did this competition go overall?"** → walk `AppearanceByEventIndex`.

Neither needs a scan, and neither view is privileged over the other.

### A note on naming

`Appearance` is deliberately colorless. The obvious name for it in dance is *performance* —
but you don't go see a soccer performance, and this is the one table that has to hold still
across every activity. `Appearance` is the neutral English word for "a competitor turning up
at an event," and it reads correctly in both directions: *the routine's six appearances this
season*, *the team's appearances this season*.

The domain word comes back at the label layer, not the schema layer. The `Activity.Kind`
label pack (phase 6) renders `Appearance` as **"Performance"** in the dance UI and would
render it as **"Game"** for soccer or **"Swim"** for swim. So nothing is lost by the neutral
type name — the user never sees it.

Note the codebase already uses "appearance" for two unrelated things: `AppearanceSettings`
(theme) in `frontend/pages/settings/settings.tsx`, and "Person appearances tagged" in the
face-tagging admin stats. Neither is a Go identifier, so there's no collision in `backend/`,
and because the word never surfaces in this feature's UI, the overlap stays confined to grep
noise. The runner-up, `EventEntry`, was rejected for a worse problem: `entryId` and
`eventEntryId` sitting next to each other in the same function is a genuine misreading
hazard.

### Why `Entry` is season-scoped

A routine's roster, age division, and competitive level are properties of a season. Binding
`Entry` to `Season` keeps those accurate without a separate per-season overlay, at the cost
of re-entering a group that carries over year to year. If multi-year history is wanted later,
add a nullable `PriorEntryId` — additive, and it does not disturb any existing query.

### Go types

New file: `backend/activity.go` (types, packing, buckets, indexes, tx helpers, procs). Split
to `activity_procs.go` if it outgrows ~800 lines, following `membership.go` /
`membership_procs.go`.

```go
// Activity is a program a family participates in. Kind drives vocabulary and
// nothing else — the schema below is identical for dance, soccer, and swim.
type Activity struct {
	Id        int       `json:"id"`
	FamilyId  int       `json:"familyId"`
	Name      string    `json:"name"`      // "Dance"
	Kind      string    `json:"kind"`      // ActivityKindDance | ActivityKindSport | ActivityKindGeneric
	CreatedAt time.Time `json:"createdAt"`
}

type Season struct {
	Id         int       `json:"id"`
	ActivityId int       `json:"activityId"`
	FamilyId   int       `json:"familyId"`
	Name       string    `json:"name"`      // "2025–26 Competition Season"
	StartDate  time.Time `json:"startDate"`
	EndDate    time.Time `json:"endDate"`
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Event is one competition (sport: one game, meet, or tournament).
type Event struct {
	Id        int       `json:"id"`
	SeasonId  int       `json:"seasonId"`
	FamilyId  int       `json:"familyId"`
	Name      string    `json:"name"`      // "Nuvo Nashville"
	Host      string    `json:"host"`      // free text: "Nuvo", "Showstopper"
	Location  string    `json:"location"`
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`   // zero for single-day events
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"createdAt"`
}

// Entry is the recurring competitive unit within a season — a routine in dance,
// a team in soccer, an event ("50 Free") in swim.
type Entry struct {
	Id        int       `json:"id"`
	SeasonId  int       `json:"seasonId"`
	FamilyId  int       `json:"familyId"`
	Name      string    `json:"name"`      // "Rise Up"
	Format    string    `json:"format"`    // "solo" | "duet" | "trio" | "group" (free text; sport: "team")
	Style     string    `json:"style"`     // "Jazz", "Lyrical" (sport: position/discipline)
	Division  string    `json:"division"`  // "Teen", "Senior"
	Level     string    `json:"level"`     // "Elite", "Rec"
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"createdAt"`
}

// EntryMember is the roster join. Two siblings in the same group dance are two
// rows; one child in eight dances is eight rows.
type EntryMember struct {
	Id        int
	EntryId   int
	PersonId  int
	FamilyId  int
	CreatedAt time.Time
}

// Appearance is one Entry at one Event.
type Appearance struct {
	Id          int       `json:"id"`
	EventId     int       `json:"eventId"`
	EntryId     int       `json:"entryId"`
	FamilyId    int       `json:"familyId"`
	OccurredAt  time.Time `json:"occurredAt"` // zero if unknown; ordering falls back to Event.StartDate
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Result is deliberately one flat record with a Kind discriminator rather than
// four tables. The fields a placement uses (Rank, OutOf, Category) and the ones
// a score uses (Score) are disjoint, but they are all small, all optional, and
// all read together — splitting them buys nothing and costs three more buckets.
type Result struct {
	Id            int      `json:"id"`
	AppearanceId int      `json:"appearanceId"`
	FamilyId      int      `json:"familyId"`
	Kind          string   `json:"kind"`             // ResultKind* below
	Label         string   `json:"label"`            // "High Gold", "Judges' Choice", "Overall"
	Rank          *int     `json:"rank,omitempty"`   // placement: 1, 2, 3…
	OutOf         *int     `json:"outOf,omitempty"`  // placement: "…of 14"
	Category      string   `json:"category"`         // "Teen Small Group Jazz"
	Score         *float64 `json:"score,omitempty"`  // numeric score or time
	PersonId      *int     `json:"personId,omitempty"` // narrows an award to one dancer in a group
	Notes         string   `json:"notes"`
	SortOrder     int      `json:"sortOrder"`        // display order within a appearance
	CreatedAt     time.Time `json:"createdAt"`
}

const (
	ResultKindAdjudication = "adjudication" // "Diamond", "High Gold", "Blown Speaker"
	ResultKindPlacement    = "placement"    // Rank / OutOf / Category
	ResultKindAward        = "award"        // judges' award, special award, title
	ResultKindScore        = "score"        // numeric — sports and scored dance formats
)
```

`Result.PersonId` is the one place a result narrows below the appearance: a judges' award
given to a single dancer inside a group number. It does not reopen event-level or
person-level awards — every `Result` still requires a `AppearanceId`. If it proves unused
after a season, it can be dropped.

`Rank`, `OutOf`, `Score`, and `PersonId` are pointers so "no placement" is distinguishable
from "1st". `vpack` has no native optional-int; pack them as a present-flag byte followed by
the value, or as a small `packOptionalInt` helper alongside the existing `packFloat32Slice`
in `person.go`.

### Photo joins

Two join tables, mirroring `MilestonePhoto` exactly (bucket + by-subject, by-photo, and
by-family indexes each):

- `AppearancePhoto` — photos of a specific routine at a specific competition.
- `EventPhoto` — photos from the weekend that aren't of one routine.

A polymorphic single table keyed by `(subjectKind, subjectId)` was considered and rejected:
vbolt indexes are typed term→target pairs, so the composite term would have to be encoded by
hand, and every read would need a kind filter. Two tables is more lines and less cleverness.

### Buckets and indexes

Naming follows `milestone.go`.

```go
var ActivityBkt         = vbolt.Bucket(&cfg.Info, "activities", vpack.FInt, PackActivity)
var SeasonBkt           = vbolt.Bucket(&cfg.Info, "seasons", vpack.FInt, PackSeason)
var EventBkt            = vbolt.Bucket(&cfg.Info, "activity_events", vpack.FInt, PackEvent)
var EntryBkt            = vbolt.Bucket(&cfg.Info, "activity_entries", vpack.FInt, PackEntry)
var EntryMemberBkt      = vbolt.Bucket(&cfg.Info, "entry_members", vpack.FInt, PackEntryMember)
var AppearanceBkt      = vbolt.Bucket(&cfg.Info, "appearances", vpack.FInt, PackAppearance)
var ResultBkt           = vbolt.Bucket(&cfg.Info, "activity_results", vpack.FInt, PackResult)
var AppearancePhotoBkt = vbolt.Bucket(&cfg.Info, "appearance_photos", vpack.FInt, PackAppearancePhoto)
var EventPhotoBkt       = vbolt.Bucket(&cfg.Info, "activity_event_photos", vpack.FInt, PackEventPhoto)
```

| Index | Term → target | Serves |
| --- | --- | --- |
| `ActivityByFamilyIndex` | family → activity | activity list |
| `SeasonByActivityIndex` | activity → season | season list |
| `SeasonByFamilyIndex` | family → season | deletion, export |
| `EventBySeasonIndex` | season → event | season overview |
| `EventByFamilyIndex` | family → event | deletion, export |
| `EntryBySeasonIndex` | season → entry | season overview |
| `EntryByFamilyIndex` | family → entry | deletion, export |
| `EntryMemberByEntryIndex` | entry → member | roster of a routine |
| `EntryMemberByPersonIndex` | person → member | **"which dances is this kid in?"** |
| `EntryMemberByFamilyIndex` | family → member | deletion, export |
| `AppearanceByEventIndex` | event → appearance | **competition view** |
| `AppearanceByEntryIndex` | entry → appearance | **routine-across-competitions view** |
| `AppearanceByFamilyIndex` | family → appearance | deletion, export |
| `ResultByAppearanceIndex` | appearance → result | every read of results |
| `ResultByPersonIndex` | person → result | a kid's individual awards |
| `ResultByFamilyIndex` | family → result | season stats, deletion, export |
| `AppearancePhotoBy{Appearance,Photo,Family}Index` | | photo attach/detach, photo deletion |
| `EventPhotoBy{Event,Photo,Family}Index` | | photo attach/detach, photo deletion |

The `*ByPhotoIndex` entries are not optional: deleting a photo must clear its joins, exactly
as `MilestonePhotoByPhotoIndex` does today.

`ResultByPersonIndex` is only written when `Result.PersonId` is non-nil.

## Access control

`Activity`, `Season`, and `Event` are family-scoped with no person dimension → plain
`CanAccessFamily(tx, user, familyId, need)`.

`Entry`, `Appearance`, `Result`, and the photo joins reach people through the roster, so
they use a new helper:

```go
// canAccessEntry allows a member of the owning family, or any user who can
// reach at least one rostered person through an accepted family link with
// ScopeActivities. A group dance with three siblings from two households is
// visible to both — the routine is the shared object, not any one child.
func canAccessEntry(tx *vbolt.Tx, user User, entry Entry, need AccessLevel) bool
```

It calls the existing `CanAccessRecordOfPerson(tx, user, entry.FamilyId, personId,
ScopeActivities, need)` per rostered person and short-circuits on the first success.
`Appearance` and `Result` resolve to their `Entry` and defer to it.

Note the deliberate consequence: a link that shares one child exposes group routines that
child is in, including co-performers' names. That is what a shared routine means, but it is
worth stating rather than discovering.

### Adding `ScopeActivities`

In `backend/family_link.go`:

1. Append `ScopeActivities` to the `LinkScope` iota **after `ScopeGrowth`** (bit 5). Existing
   link masks store bits 0–4, so every current link reads back as `Activities: false` — no
   migration, and no accidental widening of a granted link.
2. Add `Activities bool` to `LinkScopes`; handle it in `ToMask` and `linkScopesFromMask`.
3. In `normalizeLinkScopes`, add `Activities` to the set that implies `People` — activity
   reads resolve through a rostered person, so activities-without-people is incoherent for
   the same reason milestones-without-people is.
4. Leave `DefaultLinkScopes()` alone. Existing defaults are People/Milestones/Photos;
   activities should be opt-in rather than silently granted to links created before the
   feature existed.
5. Frontend: one more checkbox in the link-scope editor.

## RPC surface

Registered as `backend.RegisterActivityMethods(app)` in `app.go` next to
`RegisterMilestoneMethods`.

**CRUD** — `Create`/`Update`/`Delete` for `Activity`, `Season`, `Event`, `Entry`,
`Appearance`; `ListActivities`, `ListSeasons(activityId)`.

**Roster** — `SetEntryRoster(entryId, personIds[])`. Replace-all rather than add/remove
procs: rosters are small and always edited as a set.

**Results** — `SetAppearanceResults(appearanceId, results[])`, also replace-all. A
appearance has a handful of results, they're entered together off one results sheet, and
per-result CRUD would triple the proc count for no gain.

**Aggregate reads** — one proc per view, each returning everything the page needs so the
frontend makes a single call:

| Proc | Returns | Answers |
| --- | --- | --- |
| `GetSeasonOverview(seasonId)` | season, events, entries, rosters, appearances, results | "How did the whole season go?" |
| `GetEventDetail(eventId)` | event, its appearances with entry + results + photos | "How did this competition go?" |
| `GetEntryHistory(entryId)` | entry, roster, every appearance across events with results | "How has this dance done all season?" |
| `GetPersonSeason(personId, seasonId)` | the kid's entries, appearances, results | "How is this kid's season going?" |

**Photos** — `SetAppearancePhotos(appearanceId, photoIds[])`,
`SetEventPhotos(eventId, photoIds[])`.

**Vocabulary** — `ListActivityVocabulary(activityId)` → distinct prior values for
adjudication labels, award labels, styles, divisions, levels, formats, and hosts, for
autocomplete. Computed by walking `ResultByFamilyIndex` and `EntryByFamilyIndex`; small
enough to compute per call at family scale, no separate index.

## Deletion and cascades

Deleting anything must clear its children and every index entry, following the existing
`account_deletion.go` walk:

- Season → its Events, Entries, and everything under them.
- Event → its Appearances (and their Results and photo joins) and EventPhotos.
- Entry → its EntryMembers and Appearances (and their Results and photo joins).
- Appearance → its Results and AppearancePhotos.
- Person deletion → EntryMember rows, plus `Result.PersonId` cleared (not the result
  deleted — the routine still placed).
- Photo deletion → AppearancePhoto and EventPhoto rows, via the by-photo indexes.
- Family/account deletion → all nine buckets.

`account_deletion.go` and its tests must be extended in the same phase that adds the
buckets, not later. A bucket that account deletion doesn't know about is a data-retention
bug, and the existing test suite is the only thing that catches it.

## Phases

Each phase ends green on `make check`.

1. **Schema.** ✅ *Done.* Types, `Pack*` functions, buckets, indexes, tx helpers.
   `canAccessEntry`. `ScopeActivities` wired through `family_link.go`. Unit tests for packing
   round-trips (including nil pointers) and for access decisions across linked families —
   mirror `cross_family_isolation_test.go`.

   The family-wide sweep (`deleteFamilyActivitiesTx`) and its hook into
   `account_deletion.go` landed here rather than in phase 5, for the reason stated above:
   there must be no window in which a bucket exists that account deletion does not know
   about. Phase 5 still owns the per-entity cascades. `cmd/verifydb` counts the nine buckets
   too, so a restore drill cannot silently report a good restore that dropped them.
2. **Structure CRUD.** ✅ *Done.* Activity, Season, Event, Entry, roster. Tests per proc,
   including cross-family rejection.
3. **Results.** ✅ *Done.* Appearance CRUD, `SetAppearanceResults`, the four aggregate read
   procs, `ListActivityVocabulary`.

   The access split across the four read procs is worth restating, because it is not
   uniform. `GetSeasonOverview` and `GetEventDetail` are whole-family: a season and a
   competition have no person dimension, so a link never reaches them. `GetEntryHistory`
   and `GetPersonSeason` resolve through a roster and are the two a linked household
   actually uses — so they carry `SeasonSummary` and `EventSummary` rather than the full
   records. A performance with no competition name attached is unreadable; that
   competition's notes are nobody else's business.

   `GetPersonSeason` takes `seasonId` as optional for the same reason: a linked household
   cannot list seasons, so requiring one would leave it no way to ask the question.
4. **Photos.** Both join tables, the set-photos procs, and the photo-deletion hook in
   `photos.go`.
5. **Deletion integration.** Per-entity cascades (Season → Events → …), person deletion
   clearing `Result.PersonId`, photo deletion clearing both join tables. The family-wide
   sweep and its account-deletion test landed in phase 1.
6. **Minimal UI.** Season list → season overview → competition detail → routine history, plus
   entry forms. Routes under `/activities`, `/season`, `/competition`, `/routine` in
   `frontend/main.tsx`, pages in `frontend/pages/activities/`. Dance vocabulary hardcoded in
   a label map keyed by `Activity.Kind`, so a sport label pack is a second entry in that map:

   ```ts
   const labels = {
     dance: { event: "Competition", entry: "Routine",  appearance: "Performance" },
     sport: { event: "Game",        entry: "Team",     appearance: "Game" },
   };
   ```

   Every user-facing string for these three types goes through this map. Nothing in
   `backend/` knows the word "routine" or "performance."
7. **Export/import.** `export.go` and `import.go` both enumerate entity types explicitly
   (`import.go` has ~40 milestone references), so this is real work and is deliberately last.
   **Until it lands, activity data is absent from backups and exports** — worth knowing
   before a full season goes in.

## Deferred

- **AI import** — paste a results email or photograph a results sheet, parse to proposed
  appearances and results for confirmation. `ai_import.go` is the natural home.
- **Adjudication scales and normalization** — ordered tier lists per host, `TierRank` on
  `Result`, trend charts. Additive via a `PackResult` version bump.
- **Cross-season lineage** — nullable `Entry.PriorEntryId`.
- **Full-text search** — a `vbolt.IndexExt` over entry and event names, following
  `MilestoneSearchIndex`.
- **Video** — `isValidImageType` in `photos.go` accepts images only, and the photo worker
  resizes stills. Dance video is a photo-subsystem project, not an activities one.
- **Schedules and costs** — explicitly out of v1 scope.
- **Timeline and dashboard surfacing** — deliberately open; the aggregate read procs give
  whatever is chosen later everything it needs.

## Appendix: sports mapping

The same seven tables, no changes:

| Concept | Dance | Soccer | Swim |
| --- | --- | --- | --- |
| Activity | Dance | Soccer | Swim |
| Season | 2025–26 competition season | Fall 2026 | Short course 2026 |
| Event | A competition | A game or tournament | A meet |
| Entry | A routine | The team | An event ("50 Free") |
| EntryMember | Dancers in the routine | The kid on the team | The kid |
| Appearance | The routine at that competition | The team in that game | That swim at that meet |
| Result | adjudication, placement, award | score (2–1), placement, player-of-the-game | score (time), placement |

The one thing sports need that this doesn't model is direction — a swim time is better when
lower, a soccer score better when higher. That belongs on a future result-kind definition,
not on `Result` itself, and nothing in the dance path depends on it.
