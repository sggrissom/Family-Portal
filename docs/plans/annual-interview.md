# Annual interview

Status: phases 1–6 built on web (PRs #157–#162). Phase 7 (iOS) is not
started. Replaces the one-paragraph "Annual interview" idea in
`record-ideas.md`.

## Goal

Every year, often around a birthday, a parent sits down with a child and asks
the same long list of questions: favorite snack, favorite show, best friend,
what do you want to be. The answers matter most when you can see them change
from year to year.

The feature has four parts:

1. **Conduct** an interview live. The parent reads each question aloud and
   types the answer. It has to be fast, forgiving, and comfortable on a phone.
2. **Own the question list.** It's fully editable: add, reword, reorder, retire
   and restore questions. A family that never touches it gets a good default
   list.
3. **Enter past interviews** that were recorded on paper, in a notes app or in
   a spreadsheet. A whole year's sheet should take minutes to enter, not an
   evening.
4. **Look back.** Show how a child's answers changed over the years, compare
   siblings at the same age, and include interviews everywhere the site
   already shows moments.

User-facing words: "Annual interview" and "Interview". Code, routes and API
use `interview`. Each answer is a **moment**, following the moment-wording
rule: people see "moment", and code keeps `milestone`.

## Core decision: each answer is a moment

Quotes and artwork set the precedent. A moment already carries the person,
the date or age, photos, tags, search, embeddings, export, family-link
visibility, deletion cascades and book support. If answers lived in a separate
table, every one of those would have to be rebuilt.

So:

- An **answer** is a `Milestone` with `Category = "interview"`:
  - `Description` holds the answer ("Goldfish crackers").
  - `Context` holds a snapshot of the question as it was asked ("What's your
    favorite snack?"). Quotes already use `Context`. This keeps the original
    wording even if the question is reworded later. It makes question text
    searchable with no new index, and an export or an older iOS build still
    reads as a sensible moment.
  - Two new stored fields, `InterviewId` and `QuestionId`. This is
    `PackMilestone` v3, behind an `if version >= 3` guard.
- An **interview** (`Interview`) is the sitting: who was interviewed, when,
  which questions were planned, which were skipped, an optional portrait
  photo, and notes.
- A **question** (`InterviewQuestion`) belongs to the family's list. Its id
  stays the same through rewording, which is what lets answers line up across
  years.

The answer moments are the only record of what was answered. The interview
stores which questions were planned and which were skipped. It never stores
copies of answers, so there's no second copy to drift. A question that was
asked but left blank has no moment.

There's one cost. A 45-question interview makes 45 moments. Every surface
that lists moments must **fold the answers of one interview into a single
card**. That rule shows up throughout this plan.

## Data model

```go
type InterviewQuestion struct {
    Id          int
    FamilyId    int
    Text        string    // may contain {name} and {age}
    Section     string    // "Favorites", "About me", ...
    Position    int       // order within the family list
    MinAgeYears int       // 0 = no lower bound
    MaxAgeYears int       // 0 = no upper bound
    DefaultKey  string    // "fav-snack" for built-ins, "" for family-added
    Archived    bool      // retired: not offered, history kept
    OneOff      bool      // added for one interview, not on the list
    CreatedAt   time.Time
}

type Interview struct {
    Id          int
    FamilyId    int
    PersonId    int
    Date        time.Time // record date (UTC midnight), via dayStart
    Title       string    // optional; default "7th birthday interview" from age
    Source      string    // "conducted" | "historical"
    Status      string    // "in-progress" | "complete"
    QuestionIds []int     // planned order for this sitting
    SkippedIds  []int     // asked, child declined or passed
    PhotoId     int       // optional portrait taken at the interview
    Notes       string    // "At the kitchen table, very silly mood"
    CreatedAt   time.Time
    CompletedAt time.Time
    Version     int       // optimistic save check, like Book
}
```

Buckets and indexes:

- `interview_questions` bucket, plus a `interview_questions_by_family` index.
- `interviews` bucket, plus `interviews_by_person` and `interviews_by_family`
  indexes.
- `milestones_by_interview` index (interview → answer moment ids), so loading
  an interview doesn't scan the person's moments.

`Milestone` JSON gains `interviewId` and `questionId` with `omitempty`. Older
clients ignore them.

### Placeholders

`{name}` and `{age}` render when the interview is created: "What's the best
thing about being {age}?" becomes "…being 7?". `Context` stores the rendered
text. The list keeps the template.

### Default list: copy on first write

The default questions live in Go (`interview_defaults.go`) as an ordered list
of `{Key, Section, Text, MinAge, MaxAge}`. iOS gets them over the API and
never keeps its own copy.

- When a family has no question rows, `GetInterviewQuestions` returns the
  defaults as unsaved rows (`Id: 0`, with a `DefaultKey`). Read procs never
  write.
- The first write (starting an interview, saving a past one, or saving the
  editor) calls `ensureFamilyQuestions(tx, familyId)`, which copies the
  defaults into rows. The copy is idempotent and keyed on `DefaultKey`.
- If the built-in list grows later, the editor offers the new questions
  ("3 new suggested questions"). They're never added on their own. A
  question the family deleted or retired is never offered again.

### Suggested default questions (~45)

**Favorites:** color · food · snack · dessert or treat · breakfast · drink ·
animal · TV show · movie · book · song or singer · toy · game · sport ·
outdoor thing to do · rainy-day thing to do · place to go · holiday · season ·
subject at school (5+) · video game or app (6+)

**About me:** What are you really good at? · What are you learning right
now? · What makes you happy? · What makes you sad? · What scares you? · What
makes you laugh? · What's a food you don't like? · What do you want to be when
you grow up? · What's the best thing about being {age}?

**People:** Who is your best friend? · Who is your teacher? (4+) · What do
you love doing with your family? · Who is the funniest person you know? ·
What does your mom or dad always say?

**This year:** What was the best part of this year? · What was something hard
you did? · What are you excited about next year? · What was the best present
you got?

**Just for fun:** If you could have one superpower? · If you could be any
animal? · One wish? · What would you buy with a million dollars? · What would
you eat every day forever? · Where in the world would you go? · What's your
favorite word?

The age ranges (5+, 6+, 4+) are filters for the setup screen, not rules.
Questions outside a child's ages start unchecked, and the parent can still
check them.

## Question list editor

Route: `/interview-questions`, linked from the Interviews page and from
interview setup. Requires contribute access.

- Sections with questions in order. Drag to reorder on web (up/down buttons
  as the non-drag fallback, since iOS has no hover). Questions can move
  between sections.
- Each question has inline-editable text, an optional age range, and archive
  / restore. Archiving is the "remove" action. It keeps history, and a
  retired question still has its row in the over-time views for years it was
  answered.
- **Delete** is offered only for a question with no answers, so deleting can
  never orphan a moment.
- **Merge into…** combines a question with another one, for duplicates that
  come from importing old interviews ("Fav. food" → "Favorite food"). It
  moves the answers' `QuestionId`, keeps their `Context` snapshots, and
  archives the source question.
- Add a question, add a section, rename a section.
- **Reset to defaults** restores archived built-ins and offers missing ones.
  It never deletes family-added questions.
- Each question shows how many interviews it was asked in.
- Saving is a single bulk call, `SaveInterviewQuestions`, with a `Version`
  check that reuses the `ErrBookChanged` pattern so two parents editing don't
  overwrite each other.

v1 has one list per family. The model allows named lists later ("first day
of school interview") by adding a `ListId`, but nothing is built for that now.

## Conducting an interview

### Setup: `/interview/new/:personId`

- Date defaults to today. The child's age is shown, worked out from the
  birthday.
- A checklist of the family's questions, grouped by section, with age
  filtering applied. Uncheck questions to skip this year. "Add a question
  just for this time" creates a `OneOff` question.
- Option: **Show last year's answers while asking** (on by default, and
  remembered per user in localStorage). Some parents want the comparison
  during the conversation. Others want answers that aren't influenced by it.
- **Start** calls `StartInterview` and goes to the conduct screen.

If the person already has an in-progress interview, the page offers to
continue it instead of starting another.

### Conduct screen: `/interview/:id/ask`

Built to be read aloud from a phone held in one hand.

- One question per screen in large type, with a section label and progress
  ("18 of 42").
- A multi-line answer field, focused when the screen opens. Enter moves to
  the next question and Shift+Enter adds a new line. On iOS, OS dictation
  works in the field.
- Below the field, when that option is on, a faded line shows the child's
  previous answers: "Age 6: Goldfish · Age 5: apple slices". Tapping it
  expands the full history for that question.
- Buttons: **Back**, **Skip** (records it in `SkippedIds`), **Next**. A
  jump list shows every question with a status dot (answered / skipped /
  pending).
- **Add a follow-up question** inserts a question after the current one,
  with "keep it for next year" (which adds it to the family list) or "just
  this once" (`OneOff`).
- **Every answer saves as soon as you move on** (`SaveInterviewAnswer`). This
  keeps nothing at risk if the phone sleeps or loses signal. The text being
  typed is also held in a localStorage draft keyed by interview and question
  until the save succeeds, then cleared. A failed save shows a quiet "not
  saved, retrying" message and doesn't interrupt the conversation.
- **Take a portrait**: an optional photo of the child at the interview,
  uploaded through the existing photo pipeline and tagged to the child. It
  becomes `Interview.PhotoId` and the cover in every view.
- Leaving partway is fine. The interview stays `in-progress`, and the profile
  and dashboard show "Continue Clara's interview · 18 of 42".
- **Finish** marks it complete and opens the interview page. Any questions
  still pending are listed with "ask now" or "leave blank".

Clearing an answer that was already saved deletes its moment. An answer is
either present or absent, never an empty moment.

### Later: group mode

Siblings are often interviewed at the same sitting. A later phase lets setup
pick several children. The conduct screen then shows one answer field per
child under the shared question, and each child gets their own `Interview`.
Nothing in the model needs to change for this, so it isn't in v1. The screen
should still be built to take a list of interviews, not just one.

## Entering past interviews

Two ways in, both from the Interviews page and the profile tab: **Type it in**
and **Paste it in**.

### Type it in: `/interview/past/:personId`

A single-page form instead of one question per screen.

- Date, or age. This reuses `resolveEntryDate` and the date/age input from
  `MilestoneFields`, since "when she was 4" is often all a family knows. The
  approximate-dates plan will later enrich this, and nothing here depends on
  it.
- Every question on the current list, in order, as a label beside a
  single-line input. Tab moves to the next field. Empty fields are fine and
  just mean "not asked".
- Retired questions sit behind a "show retired questions" toggle, because old
  interviews often asked things the family has since dropped.
- **Add a question that isn't on the list**. Each one gets an "add to my list"
  checkbox. Unchecked, it's saved as `OneOff`.
- A single save, `SaveHistoricalInterview`, creates the interview
  (`Source: historical`, `Status: complete`) and all its answer moments in
  one transaction.

### Paste it in: `/interview/import`

For interviews kept in a notes app, a document or a spreadsheet. A parent
should be able to enter many years in one go.

The pasted formats the parser understands:

1. **Q: A lines**: `Favorite color: blue`, `Favorite food - pizza`, or
   `What's your favorite snack? Goldfish`. Split on the first `:`, ` - ` or
   `?`.
2. **Alternating lines**: a question line ending in `?` followed by an answer
   line. This is detected when most odd-numbered lines end in `?`.
3. **A table** pasted from Google Sheets or Excel, which arrives as
   tab-separated text, or uploaded as CSV. The first column holds questions,
   and each later column is one interview whose header is a year, a date or
   an age ("2019", "2021-05-03", "Age 5"). This is the most common way
   families keep years of answers, and **one paste becomes several
   interviews**. The transposed layout (one row per year) is detected from a
   header row that holds the questions.

The flow:

1. Pick the person, paste the text (or drop a file), and pick the date or age
   when the text doesn't include one.
2. `PreviewInterviewImport` parses the text and returns proposed interviews,
   each with rows of `{rawQuestion, answer, match}`. Nothing is written yet.
3. **Matching** each pasted question to the family list happens in order:
   - an exact match after normalizing (lowercase, strip punctuation, drop
     filler words such as "what's your", "who is your" and "favorite"/"fav",
     so "Fav food" equals "What's your favorite food?")
   - token overlap above a threshold
   - when the vision daemon is up, MiniLM sentence similarity, using the same
     `embedSentences` call that moment search uses. It's an optional boost and
     degrades quietly, like the other analysis features.

   Each row reports how confident the match is.
4. **Review screen**: a table per interview. Each row shows the pasted
   question, the answer (editable), and a dropdown with the matched question,
   pre-filled and marked when the match is uncertain. The dropdown also offers
   **New question** (with an "add to my list" checkbox) and **Ignore this
   line**. Duplicate matches within one interview are flagged.
5. **Import** calls `ImportInterviews`, which saves all reviewed interviews in
   one transaction. The result screen links to each new interview.

The parser and the normalizer are pure Go functions with table-driven tests
built from real messy examples. Parsing happens on the server so iOS can use
it too.

Later, maybe: a photo of a paper questionnaire, read by OCR into the same
review screen. That needs an OCR model in the vision daemon, so it's out of
scope here.

## Looking at interviews over time

This is where the feature pays off. All of these read from
`GetPersonInterviews`, which returns a person's interviews, their answer
moments, and the questions involved in one call, so iOS on cellular needs a
single request.

### The profile's Interview tab

A new tab, `?tab=interviews`, alongside story, quotes, artwork, and the
rest. Like those tabs, it appears only when the person has at least one
interview, or when the viewer can contribute (in which case it shows a
"Start the first interview" empty state). It has three views behind a
segmented control.

**1. Years: a grid of questions by year (the default view).**
- Rows are questions in list order, grouped by section. Columns are
  interviews, oldest to newest, with headers like "Age 5 · 2024" and the
  portrait thumbnail.
- The question column is sticky and the grid scrolls sideways. On narrow
  screens it switches to the per-question cards described below.
- A cell shows the answer. A blank cell means not asked. A skipped question
  shows a faint dash.
- **Changes are highlighted.** When an answer differs from the year before
  (after normalizing case and punctuation), it's marked as new. An answer
  that stayed the same is drawn more quietly, so "pizza, pizza, pizza, tacos"
  reads at a glance.
- Retired questions appear only in the years they were asked. One-off
  questions collect in an "Also asked" group at the bottom.
- Clicking a cell opens the answer moment, where the parent can add photos,
  tag it or edit it.

**2. Questions: one card per question.**
- Each card shows the question, then a small vertical timeline of answers by
  age: "3 bananas → 4 pasta → 5 pizza → 6 pizza → 7 tacos".
- A **"Most consistent"** sort brings forward answers that have lasted (best
  friend for four years running). A **"Most changed"** sort brings forward
  questions with a different answer every year.
- A filter box searches question text and answers.

**3. One interview: a single interview read as a page** (also its own route,
`/interview/:id`).
- Portrait, title, date, age and notes at the top, then the questions and
  answers by section. It's quiet and readable, with a print stylesheet so the
  page prints as a one-page keepsake.
- A **"What changed since last year"** strip at the top lists the answers
  that differ from the previous interview, side by side: "Best friend: Sam →
  Lily". This is the summary a parent wants to read aloud at the birthday
  dinner.
- Actions for contributors: edit answers (an inline form that reuses the past
  interview form in edit mode), continue if in progress, change the
  date/title/notes/portrait, delete the interview.

### Family Interviews page: `/interviews`

Reached from the Moments area or navigation. It lists every child with the
date and age of their last interview. Each child gets **Start**, **Continue**
or "due soon" (see the dashboard nudge below), plus links to past-interview
entry, import, and the question editor.

### Comparing siblings at the same age

On the Interviews page or a profile, **"Compare at the same age"** picks a
question and lays out each child's answers by age: "At 6: Clara said pizza,
Ben said tacos, Mom said (from her own childhood interview, if entered)
spaghetti." It reuses the age bucketing from the same-age page. Parents and
grandparents can be interviewed too, since they're just people.

The same-age page itself also gains an interview row (see Integrations
below).

### Optional, later: answer lists

Some answers repeat across children and years (characters, foods, shows). A
later, small extra could show a family "word cloud", or a list of top answers
per question across the whole family. This isn't in the phases below. It's
noted so the data stays in a shape that allows it, and it already does.

## Integrations with the rest of the site

The rule everywhere: **answer moments of one interview fold into a single
interview card** ("Clara's annual interview · age 7 · 42 answers"), with the
portrait, a few highlighted answers, and a link to `/interview/:id`. The
grouping key is `Milestone.interviewId`. A pure helper, `groupInterviewAnswers(milestones)`
in `frontend/lib/interviews.ts`, takes moments and returns cards and loose
moments. It's vitest-covered and shared by every surface below. The Go side
needs the same thing for the dashboard and same-age responses.

| Surface | Change |
| --- | --- |
| **History** (`lib/history.ts`, `pages/history`) | Answers fold into one interview card on the interview's day. A history filter for moments includes interviews, and tag filters still match an answer that carries the tag (the card then shows just the matching answers). |
| **Profile story tab** | One card per interview in the story timeline, not 45 rows. |
| **Quotes / Artwork tabs** | No change. Interview answers are their own category. |
| **Dashboard: On this day** | "3 years ago Clara did her annual interview", with "best friend: Sam" as a preview line, folded. |
| **Dashboard: recent** | A finished or in-progress interview shows as one card. |
| **Dashboard: nudges** | A new `interview` nudge in `dashboardNudges` for contributors. It shows when a child's birthday is within ~14 days (or passed within 30) and they have no interview in the last 10 months. Children under about 3 are skipped. It shares timing helpers with `birthdayNudge`. An in-progress interview gets its own "Continue" nudge. |
| **Same-age page** | `SameAgeRow` folds interview answers into one `Interview` entry. The row shows the interview card, and "compare answers" opens the sibling comparison at that age. |
| **Moment detail** (`view-milestone`) | An answer moment shows the question (from `Context`) as a heading above the answer, with "Part of Clara's 2025 interview →" and previous and next answer links. |
| **Edit moment** | For `interview` moments, category and date are locked, because the date follows the interview (changing the interview date rewrites its answers' dates). Description, photos and tags stay editable. `interview` is left out of the add-moment category picker and out of `SuggestMilestoneCategory`. Interview answers are created only through interview procs. |
| **Validation** | `validateMilestoneFields` accepts `interview`, but `AddMilestone` and `UpdateMilestone` refuse to create one or switch a moment to or from it. |
| **Search** | Works already, because `Context` (the question) is indexed. Search results fold answers too: an answer hit shows as "Favorite snack: Goldfish · Clara's 2025 interview". |
| **Embeddings** | In `embedMilestone`, embed `Context + ": " + Description` for interview answers so "pizza" alone isn't the vector. Bump the embedding text version so existing ones re-embed (only interview answers are affected). |
| **Tags** | Unchanged. Answers are taggable, and the tag page shows them folded. |
| **Books** | A new `BookItemInterview` kind (source id = interview id) renders as a Q&A spread: portrait plus answers by section, with a "What changed" sidebar when there's a previous interview. Add `"interviews"` to `bookCategories`. The year and first-year presets include interviews dated in range. Answer moments are excluded from the plain moment stream so they don't appear twice. |
| **Export** | New `interviews` and `interviewQuestions` arrays. `ExportMilestone` gains `interviewId` and `questionId`. Bump the export format version. |
| **Import (bundle)** | Remap question, interview and answer ids, the same way people and tags are remapped. Duplicate check: same person, same date, same question. An older export with no interviews still imports. |
| **Person merge** | Move interviews to the target person along with their moments. When both people have an interview on the same day, keep both. |
| **Person / account deletion** | Delete interviews (their answer moments are already deleted by the moment cascade). The deletion summary counts interviews. Account deletion removes family questions. |
| **Record-date normalization** | `Interview.Date` goes through `dayStart`, and the normalization pass in `calendar_dates.go` learns the new bucket. |
| **Family links** | Interviews are visible under `ScopeMilestones`, with no new scope. A linked grandparent can read interviews and see the over-time views but can't start, edit or import. Add cases to `family_link_test.go` and `cross_family_isolation_test.go`. |
| **Universal links** | Claim `/interview/` for the iOS app, as #155 did for moments. |
| **Seed / landing** | `seed_dev.go` gives each sample child 2–3 years of interviews, with realistic drift between years so the over-time views have something to show. Consider an interview screenshot for the landing page. |
| **Admin digest / analytics** | Count interviews and answers. |
| **Copy** | All strings go in `frontend/lib/copy.ts`, mirrored in iOS `Copy.swift`. |

## API (vbeam procs)

All in `backend/interview.go`, `interview_questions.go` and
`interview_import.go`. Every proc checks access with
`CanAccessRecordOfPerson(..., ScopeMilestones, need)` or
`CanAccessFamily(..., AccessContribute)` for list edits, as moment procs do.
Following the vbolt rule, responses are built before `TxCommit`. Answer
moments are queued for embedding after commit.

| Proc | Purpose |
| --- | --- |
| `GetInterviewQuestions` | The family list, or unsaved defaults, plus suggested defaults the family doesn't have yet. |
| `SaveInterviewQuestions` | Bulk editor save: add, reword, reorder, archive, delete (only when unanswered), with a version check. |
| `MergeInterviewQuestions` | `{fromId, intoId}`: moves answers to the target and archives the source. |
| `StartInterview` | `{personId, date, questionIds, oneOffQuestions}` → the interview. Runs `ensureFamilyQuestions`. |
| `GetInterview` | The interview, its questions (rendered), its answer moments, and **previous answers per question** for this person, all in one bundle for the conduct screen. |
| `SaveInterviewAnswer` | `{interviewId, questionId, answer, skipped}`: creates, updates or deletes the answer moment and updates `SkippedIds`. Idempotent. |
| `AddInterviewQuestion` | Adds a question mid-interview at a position, either kept on the list or one-off. |
| `UpdateInterview` | Date (rewrites answer dates), title, notes, portrait, status. |
| `DeleteInterview` | Deletes the interview and its answer moments. |
| `GetPersonInterviews` | Everything the over-time views need for one person. |
| `GetFamilyInterviews` | The Interviews page summary: last interview per child, in-progress ones, and who's due. |
| `CompareInterviewAnswers` | `{questionId, personIds}`: answers by age for sibling comparison. |
| `SaveHistoricalInterview` | One past interview with answers and new questions, in one transaction. |
| `PreviewInterviewImport` | Parses pasted text or CSV and returns proposed interviews with matches. Writes nothing. |
| `ImportInterviews` | Saves the reviewed proposals. |

Then regenerate `frontend/server.ts` (see the memory note on this: there's no
make target, and the generated error block reorders on every run).

## Frontend files

- `frontend/lib/interviews.ts` (+ `.test.ts`): `groupInterviewAnswers`,
  answer-change detection (`normalizeAnswer`, `changedSince`), grid shaping,
  consistency/change scores, and age column labels.
- `frontend/pages/interviews/`:
  - `interviews.tsx`: the family Interviews page
  - `questions.tsx`: the question editor
  - `setup.tsx`: interview setup
  - `ask.tsx`: the conduct screen
  - `view.tsx`: the single interview page, with print styles
  - `past.tsx`: type in a past interview, and edit answers
  - `import.tsx`: paste and review
  - `compare.tsx`: sibling comparison
  - `*-styles.ts`, following the existing `block()` pattern
- `frontend/components/InterviewCard.tsx`: the folded card used by history,
  profile, dashboard, same-age and search.
- Profile: `InterviewsTab` with the Years / Questions views.
- Routes in `main.tsx`: `/interviews`, `/interview-questions`,
  `/interview/new/`, `/interview/past/`, `/interview/import`,
  `/interview/compare`, and `/interview/` (view, with `/ask` beneath it).

## iOS

iOS is the main way families enter data, and the conduct screen is mainly
used on a phone. So the procs are bundled (one call per screen) and all the
parsing, matching and change detection happens on the server or in shared,
pure logic. The iOS work is planned and built in the iOS repo after the web
versions settle the design (iOS copies the web design). In order:

1. Conduct and resume
2. Single interview view
3. Profile Interview tab (the Questions view first, since it suits narrow
   screens)
4. Past entry
5. Question editor

Paste import can stay web-only at first.

## Phases

Each phase is its own PR, with stacked PR bases cascaded and merged
bottom-up.

1. **Model and question list.** Add the `Interview` and `InterviewQuestion`
   types, `PackMilestone` v3, the default questions, `ensureFamilyQuestions`,
   question procs, the editor page, and the `interview` category locked out
   of the normal moment form. Tests cover copying defaults, editing, merging,
   delete protection, and access (including links being read-only).
2. **Conduct.** `StartInterview`, `GetInterview` with previous answers,
   `SaveInterviewAnswer`, `AddInterviewQuestion`, setup and conduct pages,
   autosave and draft recovery, portrait, resume. Playwright test: run an
   interview through to the end, reload in the middle and resume.
3. **Read and look back.** The single interview page with "What changed",
   the profile Interview tab (Years and Questions views), the family
   Interviews page, moment-detail context, and printing.
4. **Past interviews.** Type-in form, then paste or CSV import with parsing,
   matching and review. Parser tests use real samples. If the user has real
   historical interviews, use them as fixtures once anything personal is
   removed.
5. **Folding into the site.** The grouping helper across history, profile
   story, dashboard (cards and nudge), same-age, search, tags, embeddings
   text, deletion/merge/normalization, and seed data.
6. **Durability and books.** Export/import round trip, book interview spread,
   and sibling comparison.
7. **iOS parity** (iOS repo) and the `/interview/` universal link.

Phases 1–3 are the minimum useful feature. Phase 4 is next in priority,
because the family's existing paper and notes history is what makes the
over-time views worth having from day one.

## Open questions

- **Cadence.** The nudge assumes interviews around birthdays. Some families
  do them on the first day of school or at New Year. Add a per-family
  setting (birthday / school start / New Year / off) now or later? The plan
  assumes later, with birthday as the default.
- **Notes on single answers** ("said while giggling"). Right now this would
  go into the answer text. A separate note field on the answer moment is
  cheap if it's wanted.
- **Answers with several values** ("best friends: Sam, Lily"). This stays
  free text in v1, and change detection compares the normalized text.
  Splitting into lists could come later if the grid looks noisy.
- **Who asked.** Record the interviewer on `Interview`? This ties into "who
  recorded it" in `record-ideas.md`.
