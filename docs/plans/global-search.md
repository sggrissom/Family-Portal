# Global search

One search box, reachable from every screen, that finds anything a family would
go looking for. The trigger was a specific moment that couldn't be found. Search
already exists, but it's split up and hard to discover:

- `SearchMilestones` only runs from the History page's filter bar. It matches
  whole words from a term index (`MilestoneSearchIndex`), so `walk` doesn't find
  "walked", and it can't scope by person or age.
- `ListFamilyPhotos` with `q` runs CLIP semantic search plus title/description
  matching, but only from the Photos page.
- People, tags, places, activities and books have no search at all.

Global search ties these into one entry point. It reuses the existing matchers
where they're good (CLIP for photos, MiniLM for moments) and adds a query
understanding step, so `ada first steps`, `ada at 2`, `beach 2024` and `quote`
all do what you'd expect.

The iOS side is planned in `Family-Portal-Ios/global-search-plan.md`. Both use
the same backend proc.

## What is searchable

| Kind | Matched on | Opens |
|---|---|---|
| People | name (prefix as you type), relationship label ("grandma") | `/profile/:id` |
| Moments (incl. interview answers, quotes, artwork) | description, context, category, tag names; MiniLM meaning when the daemon is up | `/milestone/:id` |
| Photos | title, description, tag names, place name; CLIP content when the daemon is up | `/view-photo/:id`; "See all" → `/photos?q=…&people=…` |
| Tags | name | `/photos?tags=:id` (photos) and History filtered by tag |
| Places | family place name, resolved city name | `/photos?place=:key` |
| Activities | activity, season and event names; event host/location | `/activities/…`, `/season/:id`, `/competition/:id` |
| Books | title | `/book/:id` |

**Measurements aren't searched as records.** Nobody looks up "the 14.2 lb
entry". A person match does get quick links ("Ada's growth", "Ada's moments",
"Ada's photos"), which covers the real need of getting to a growth chart fast.
Chat is also left out. It's a separate surface with its own scrollback, and
search results that link into chat history would need anchor support that
doesn't exist yet. That's noted as a follow-up.

## Query understanding

A pure function, `parseSearchQuery(query, people, tags, now)`, pulls structure
out of the text. What it doesn't consume stays as free text. It lives in
`backend/search_query.go`, is table-tested, and its cases get mirrored in a
Swift test (see the iOS plan) so both clients describe a query the same way.

Recognised pieces:

- **People.** Full name, first name, or possessive (`ada`, `ada's`,
  `ada grissom`), plus relationship labels from `relation_label.go` when they
  identify exactly one person. If two visible people share a first name, both
  match: their cards show and the filter is "either". (The photo search's
  `peopleInQuery` leaves ambiguous names as text. Global search is more
  forgiving because it shows the person cards for the user to pick from.)
- **Age.** `2`, `at 2`, `age 2`, `2 years`, `2yo`, `2 years old` → months
  [24, 36). `18 months`, `18mo`, `18m` → [18, 19). `newborn` → [0, 1).
  `baby` → [0, 12). A bare number counts as an age only when a person is also
  named and it's < 25. Otherwise it stays text, so `25 pumpkins` still works.
  Age is measured from the subject's birthday: the moment's person, or for a
  photo the named person (any tagged person when no one is named).
- **Dates.** A 4-digit year 1900–2100 → that year. A month name, with or
  without a year (`june`, `june 2024`) → that month. A month with no year
  matches it in any year.
- **Categories.** Category words (`quote`, `quotes`, `artwork`, `drawing`, and
  any other category values) → category filter for moments.
- **Tags.** An exact tag name (multi-word tags matched greedily) → tag filter,
  *and* the tag shows in the Tags group.
- **Filler.** `the`, `a`, `of`, `photo(s)`, `picture(s)`, `when`, `was`, `old`
  are dropped from the text (`searchFillerWords`, extended).

The response echoes what was understood. The UI shows it as removable chips
("Ada · age 2 · 2024"), so a wrong guess can be undone in one tap. Removing a
chip re-runs the search with `ignore: ["age"]` and similar. Search should
explain itself rather than silently filter.

## Matching and ranking

Text matching becomes **prefix-of-word, every term required**, case- and
accent-folded (`walk` ⊂ "walked", "Walking"; `cafe` matches "café"). Light
normalisation only: no stemming library. Prefix matching covers most of what
stemming would, and it's predictable.

Within a group:

1. Text hits rank above semantic-only hits, and among text hits, ones where
   every term matched in the description beat ones matched via context or tag.
   Ties go to the newest.
2. Semantic hits (MiniLM for moments, CLIP for photos) are blended exactly as
   today (`blendMilestoneResults`, `rankPhotos`), reusing their calibrated
   thresholds.
3. Structured filters (person, age, date, category, tag) are hard filters. With
   no free text left (`ada at 2`), the group lists everything matching the
   filters, newest first.

Group order is fixed (People, Moments, Photos, Tags, Places, Activities, Books)
with one exception: a group with zero results is hidden. Nothing about the
ordering should change between keystrokes.

## Backend

New file `backend/search.go`, registered from `RegisterSearchMethods`:

```go
type SearchRequest struct {
	Query  string   `json:"query"`
	Ignore []string `json:"ignore,omitempty"`
	Limit  int      `json:"limit,omitempty"`
}

type SearchUnderstood struct {
	PersonIds    []int  `json:"personIds"`
	AgeFromMonth *int   `json:"ageFromMonth,omitempty"`
	AgeToMonth   *int   `json:"ageToMonth,omitempty"`
	AgeLabel     string `json:"ageLabel,omitempty"`
	Year         int    `json:"year,omitempty"`
	Month        int    `json:"month,omitempty"`
	Category     string `json:"category,omitempty"`
	TagIds       []int  `json:"tagIds"`
	Text         string `json:"text"`
}

type SearchResponse struct {
	Query       string            `json:"query"`
	Understood  SearchUnderstood  `json:"understood"`
	People      []Person          `json:"people"`
	Moments     []Milestone       `json:"moments"`
	MomentCount int               `json:"momentCount"`
	Interviews  []Interview       `json:"interviews"`
	Photos      []PhotoWithPeople `json:"photos"`
	MorePhotos  bool              `json:"morePhotos"`
	Tags        []Tag             `json:"tags"`
	Places      []PhotoPlace      `json:"places"`
	Activities  []ActivityHit     `json:"activities"`
	Books       []BookSummary     `json:"books"`
	SearchMode  string            `json:"searchMode"`
}
```

`ActivityHit` is `{kind: "activity"|"season"|"event", id, name, detail, date}`,
which keeps three entity types in one flat list.

Implementation notes:

- **One read transaction, in-memory scan.** A family is at most a few thousand
  moments and tens of thousands of photos. `SearchMilestones` already loads
  every visible moment for its semantic pass, and `searchPhotos` already walks
  the photo listing. Global search does the same once, with access checks via
  `visibleMilestones` and `newPhotoListing`, so visibility rules are inherited,
  not reimplemented. People, tags, places, activities and books are small
  per-family lists.
- **Photos reuse `searchPhotos`** with the understood people/tags/date as
  listing filters, `limit` 12, and the free text as the query. Age filtering on
  photos happens after the listing filter, against the person's birthday.
- **Moments reuse `semanticMilestoneScores`/`blendMilestoneResults`**. The
  whole-word `MilestoneSearchIndex` is left in place for now (History still
  uses it). See the History item below.
- **Daemon down** → `searchMode: "text"` and the UI shows the same "matching
  words only" hint the Photos page already has.
- **Limits.** Per group: people 8, moments 20 (with `momentCount` for "See all
  N"), photos 12, others 6. The request `limit` scales the moments group only,
  for a "show more".
- **Performance budget.** < 50 ms server time on the seed family without the
  daemon, and < 150 ms with it (one cached MiniLM + one cached CLIP embed). A
  benchmark test over a generated 3k-moment / 20k-photo family keeps us
  honest. If it's blown, the first fix is a per-family in-memory moment text
  cache invalidated on write, the same pattern as `familyVectors`.
- **Query length** is capped at 200 chars, as photo search does.
- Errors: an empty query returns an empty response, not an error, because the
  client fires on every keystroke.

Add an entry to `docs/mobile-api.md`.

### History page

Point History's search at the new matcher, so prefix matching and person/age
understanding work there too, without a second implementation:
`SearchMilestones` keeps its signature and calls the shared moment-matching
function. After that, `MilestoneSearchIndex` has no readers and can be deleted
in a follow-up. Its write path runs on every moment save, so removing it is a
small win.

## Web UI

### Entry point: always one tap away

- **Desktop top bar:** a search button left of **+ Add**, styled as a
  compact field ("🔍 Search  /") so it reads as a box, not an icon. Shortcuts:
  `/` and `Ctrl/⌘ K` from anywhere except text inputs.
- **Phone:** the bottom bar is full (Home · Photos · + · Growth · More). Search
  goes in the top bar as a 🔍 icon button next to the account initial. That's
  where people look for it, and it's visible on every page.
- Not hidden behind More, and not hover-only (iOS-parity rule).

### `/search` page

The button navigates to `/search` (and `/search?q=…` as you type, via
`replaceState`). It's a route rather than a modal, so that:

- tapping a result and pressing **Back** returns to the same results, which is
  the core loop of "is it this one? no…".
- a search can be linked or bookmarked.

Layout, top to bottom:

1. Autofocused input, with a clear (×) button. 150 ms debounce, and stale
   responses dropped by request sequence number.
2. Understood chips, each removable.
3. With an empty query: the family's people as chips (tap to search them)
   and recent searches (last 8, `localStorage`, try/catch-wrapped).
4. Groups in fixed order, each with a header and a "See all" link where one
   makes sense (photos → `/photos?q=`, moments → `/history` with the same
   filters, a tag → its filtered views).
   - People: avatar, name, age, plus quick links (Moments, Photos, Growth).
   - Moments: reuse the History row/card rendering (`MilestoneText`, folded
     `InterviewCard` for answers), so a moment looks the same everywhere.
   - Photos: a thumbnail strip using `ResponsiveImage`.
   - Others: simple rows with an icon and secondary detail.
5. Matched words get `<mark>` highlighting in moment and photo text.
6. Keyboard: ↑/↓ moves through results across groups, Enter opens.
7. Empty state: "Nothing matched 'x'". If filters were understood, add a hint to
   remove one ("Try without age 2").

New files: `frontend/pages/search/search.tsx`, `search-styles.ts`, and
`frontend/lib/search.ts` for pure helpers (highlight ranges, recent-search
list, chip labels, result flattening for keyboard nav) with vitest tests. Wording
goes in `copy.ts` under `copy.search`, using "Moment", never "milestone".
`appNav.ts` gets a `search` destination so the top-bar button shows active.

## Tests

- `search_query_test.go`: the parse table (names, possessives, shared first
  names, ages in every form, bare-number rules, years, months, categories,
  multi-word tags, filler). The same cases are copied into the iOS test.
- `search_test.go`: each group finds what it should; prefix matching; person +
  age + text combined; `ignore`; daemon-down mode; **cross-family isolation**
  (add the proc to `cross_family_isolation_test.go`) and linked-person scope
  (a person shared without photos must not leak photos); empty query.
- Benchmark as above.
- Frontend: vitest for `lib/search.ts`. One Playwright flow: sign up with seed
  data, open search from the top bar, type a moment's partial word, open it,
  go Back, and see the results still there.

## Phases (web repo, stacked PRs)

1. Query parser + tests.
2. `Search` proc, groups, isolation and benchmark tests, regenerated
   `server.ts`, and a `mobile-api.md` entry.
3. `/search` page, top-bar/phone entry points, shortcuts, copy.
4. History's `SearchMilestones` switched to the shared matcher; drop
   `MilestoneSearchIndex`.

Ship 1–3 to staging before starting the iOS work, so the app is built against
the real API.

## Follow-ups (not in this plan)

- Chat message search, with jump-to-message.
- Typo tolerance (edit distance 1 on words ≥ 5 chars) if prefix matching proves
  not to be enough.
- Searching inside book text and interview questions that haven't been answered.
