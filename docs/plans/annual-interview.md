# Annual interview

Status: web implementation merged in PRs #157–#162. Reviewed against commit
`e0c1907` on 2026-10-11. The original six web phases are implemented; the
follow-up work below is still proposed, not a claim that those fixes shipped.
iOS remains a separate workstream; its repository was not reviewed in this pass.

This document replaces the original build checklist with an implementation
reference and a prioritized follow-up plan. It supersedes the short idea in
[Record ideas](record-ideas.md). This revision changes documentation only.

## Product intent

A parent can ask a child recurring questions, capture their words comfortably
on a phone, and later see how the answers changed. Families can customize the
questions, enter old interviews, compare siblings at similar ages, and include
interviews in books. Parents and grandparents can have interviews too.

Keep the experience forgiving: unanswered questions are fine, a sitting can
be resumed, and a large family should not acquire a dashboard full of chores.
No AI dependency is needed to record or read an interview. Semantic matching
is an optional import aid; the parent decides what an imported question means.

People see **interview**, **answer**, and **moment**. Code retains `milestone`
for answer records. Keep one answer as one moment, grouped under one interview;
do not replace the shipped model with a parallel answer store.

## What is implemented

| Area | Implementation and source |
| --- | --- |
| Model | `backend/interview.go`: sitting metadata, ordered `QuestionIds`, `SkippedIds`, portrait, status and `Version`. `PackMilestone` v3 adds `InterviewId` and `QuestionId`; `Context` preserves wording when an answer is first created. |
| Family question list | `backend/interview_questions.go`: persistent `InterviewQuestionList` with `Revision`, `Seeded` and `DismissedKeys`; copy-on-first-write defaults, reword/reorder/archive/restore, deletion protection and question merging. Default questions live in `backend/interview_defaults.go`. |
| Capture | `backend/interview_procs.go` and `frontend/pages/interviews/{setup,ask}.tsx`: setup, one question at a time, previous answers, skip, follow-ups, local drafts, save on navigation, portrait, resume and completion. One in-progress sitting per person is enforced. |
| Reading | `backend/interview_views.go` and `frontend/pages/interviews/{view,person-tab,interviews}.tsx`: single interview, print view, profile Years/Questions views, family overview and birthday eligibility. |
| Past entry | `backend/interview_import.go` and `frontend/pages/interviews/{past,import}.tsx`: typed entry, notes/CSV/TSV parsing, preview, question matching and transactional import. The parser already uses `encoding/csv`; text and batch-size limits already exist. |
| Site integration | `backend/interview_fold.go`, `frontend/lib/interviews.ts` and `InterviewCard.tsx`: grouped cards in existing record surfaces, birthday/continue nudges, question-aware embeddings, person merge/deletion, family teardown, portrait deletion and record-date normalization. |
| Books and portability | `backend/interview_export.go`, `backend/book*.go` and `frontend/lib/book.ts`: interview export/import, book source references, Q&A rendering and comparison summaries. Sibling comparison is implemented in `CompareInterviewAnswers` and `compare.tsx`. |
| Coverage | Backend `interview*_test.go`, frontend interview helper tests and `tests/ui/interview.spec.ts` cover the main flows. The scenarios below extend that coverage; this documentation review did not run the application or test suites. |

Preserve the working safeguards rather than reimplementing them: initialized
empty question lists do not reseed; default selections have `DefaultKeys` before
rows exist; referenced questions cannot simply be deleted; merging two answered
questions in one sitting is rejected; ordinary moment edits preserve interview
dates/context; portrait deletion and record cascades already have hooks.

## Follow-up order

| Priority | Work | Reason |
| --- | --- | --- |
| First | Safe saves, conflict handling and completion | Avoid silent overwrite and premature completion during real phone use. |
| First | Preserve distinct sittings through import | Same-day interviews must survive export/import without answers being discarded. |
| Next | Historical wording and honest dates | Preserve what was actually asked and what is actually known about when. |
| Next | Import matching and comparison consistency | Prevent misleading question alignment and inconsistent change summaries. |
| Before iOS parity | Stable API contracts and compatibility checks | Native clients should copy corrected behavior, not existing gaps. |
| Polish | Mobile controls, grouped result limits and quieter defaults | Improve day-to-day use without another model redesign. |

These can be separate, focused PRs. Do not reopen completed web phases as an
unbuilt feature list. Schema changes must include compatible reads, migration
behavior, export/import and mobile-contract notes in the same change.

## 1. Safe saves and completion

**Observed:** `SaveInterviewAnswerRequest` has no expected version, and
`saveInterviewAnswerTx` does not increment `Interview.Version`.
`UpdateInterview` does check and increment it, so protection currently covers
metadata but not the whole sitting. Generic moment edits and question merges
also change interview content without advancing that token.

In `ask.tsx`, requests are serialized per question, not per sitting. Each
response replaces `state.interview`, allowing responses for different questions
to restore older skip state. `finish` waits for up to five seconds, then checks
failed saves but not whether requests remain in flight. A slow request can
therefore still be pending when completion proceeds.

Planned changes:

- Keep the existing `Version` name; extend its semantics consistently. Every
  answer, skip/clear, question-list change within a sitting, metadata change
  and interview-aware moment edit/delete must check and advance it. Reuse
  common transaction helpers so generic entry points cannot bypass protection.
  The family question list keeps its separate `Revision`.
- Serialize writes per sitting and return the acknowledged state/version.
  Debounce saves while typing as well as saving on navigation. A timeout is
  ambiguous: refetch before retrying. If the intended state is already stored,
  acknowledge it; otherwise preserve the draft and resolve the version conflict.
  Never blindly overwrite the other parent's answer.
- Keep the existing protection against clearing newer typing when an older
  response arrives. Extend it to stale whole-interview responses and retries.
  Stop automatic retry on authentication/access loss or validation errors;
  offer sign-in or correction while retaining unsaved text.
- Finish must wait for all queued/in-flight writes to be acknowledged. A timeout
  leaves the interview in progress with a retry action, not complete. Disable
  new edits during the final flush or include them in the queue before finishing.
- Scope drafts and preferences by account and family, in addition to interview
  and question. Current draft keys only contain interview/question ids and
  storage failures are swallowed. Distinguish Saving, Saved, Saved on this
  device, and Couldn't save; do not promise durability when storage fails.
  Define logout cleanup and prevent another account from seeing/replaying drafts.
- Preserve photo/tag associations on text-only saves. Clear and skip are
  explicit states: no empty answer moments, and no question both answered and
  skipped. Explain that in-progress is workflow state, not private visibility;
  persisted moments already follow normal family/link access.

Acceptance: delay an answer beyond five seconds; save two different questions
with responses arriving out of order; edit from two devices and from moment
detail; lose a response after commit; expire authentication; fill/disable local
storage; reload mid-answer. No newer text is erased, completion cannot outrun
saves, and conflicts remain recoverable.

## 2. Import identity and round trips

**Observed:** `interviewImport.run` in `backend/interview_export.go` finds an
existing sitting by person and date. It then skips answers for questions already
answered there. Two different same-day sittings can collapse even when importing
a bundle into an empty family. Typed/paste historical saves create new sittings
without a request id, so repeating a submission after a lost response can create
duplicates. These are different problems and need different solutions.

Planned changes:

- Preserve each source interview as a distinct sitting using source-id mappings
  within a bundle. Date is useful for suggesting a possible duplicate, never
  enough to establish identity. If importing into existing data, present an
  explicit skip/keep-separate choice; never silently discard a conflicting answer.
- Add request ids for historical save/import and start. Persist a family- and
  operation-scoped receipt with a payload fingerprint and result ids in the same
  transaction. A replay returns the original result after authorization; a
  reused id with different content is rejected. Retrying a deleted result must
  not recreate it. Start's existing in-progress guard remains useful but is not
  a substitute for acknowledging a lost response.
- Export/import family-list state as well as question rows. `Seeded` and
  `DismissedKeys` are not in the current interview export structures; an
  intentionally empty/customized list should not regain deleted defaults after
  restoration. Add backward-compatible defaults for older bundles.
- Keep question order, skip state, answer context, attachments, tags, portrait
  and book references coherent after remapping. Reject or visibly report dangling
  references; never flatten malformed linked answers into unrelated moments.
  Reconcile skipped ids when adding an answer to an existing sitting.

Acceptance: export two same-day interviews with different answers to the same
question, import into an empty family and preserve both; retry a committed
historical import and add nothing; restore an intentionally empty question list;
round-trip edited/archived questions, skipped questions, answer attachments and
book references. Older exports without interviews must still import.

## 3. Preserve the wording of the sitting

**Observed:** an existing answer retains its `Context`, which the single-page
view prefers. But `interviewQuestionsFor` renders current family question text
and sections each time. Unanswered/skipped prompts have no snapshot. Clearing
and re-answering a question creates a new moment using the current wording.
Changing sections can also reorganize an old interview.

Planned changes:

- Store ordered question snapshots on the interview: question id, rendered text
  and section. Capture them at setup and when adding a follow-up. Answer context
  copies the snapshot; editing the family's list affects future sittings only.
- Migrate existing sittings without pretending to recover lost history. Prefer
  an answer's existing context for its prompt; for unanswered prompts/sections,
  current list data is only a best-available fallback. Preserve existing answers
  exactly and document that earlier wording cannot be reconstructed.
- Keep question identity through minor rewording; a change in meaning should
  create a new question. Make that distinction clear in the editor.
- Existing merge protection covers two answers. Extend it to both planned
  question ids occurring in the same sitting: conservatively refuse that merge
  and show the affected sittings instead of collapsing distinct prompts. Merges
  preserve wording and advance affected sitting versions atomically.

Acceptance: reword, reorder, move and archive questions midway through an
interview, then clear/re-answer a prompt. Historical wording and section order
remain stable. Migrate a pre-snapshot sitting and round-trip its snapshots.

## 4. Historical dates and unknown ages

**Observed:** `proposalDate` converts a year-only header to that year's birthday
(or January 1 without a birthday), and age-only input to a birthday-derived day.
`saveHistoricalTx` persists one exact date through `resolveEntryDate`. The preview
is editable, but the stored record does not preserve that the day was inferred.
`ageYearsAt` returns zero for a missing birthday, which can look like a newborn
age or render “being 0” in a question.

Planned changes:

- Show the original header/date assertion in review and label inferred dates as
  suggestions. Until partial-date storage exists, require a known date or leave
  that sitting unsaved with its input retained. Do not silently convert “2019”
  or “when she was 4” into a precise anniversary. “On her fourth birthday” can
  explicitly confirm a birthday date.
- Support age/year-only persistence through the representation in
  [Approximate dates](approximate-dates.md), including uncertainty on answers,
  exports, age comparisons and anniversary eligibility. A sorting timestamp is
  not a known event date. Do not reinterpret existing exact dates automatically;
  inferred records already saved need an explicit correction flow.
- Represent unknown age separately from zero. Hide unavailable age labels and
  ask for wording without `{age}` when it cannot be resolved. Same-age comparisons
  retain actual dates/ages and every sitting in a bucket, including multiple
  sittings in a year.
- Preserve the current client-local setup date (`localDateString`) and UTC-midnight
  record storage. `parseInterviewDate` still falls back to server-UTC today when
  the request omits a date; require an explicit local date for creation instead.
  Exercise leap-day birthdays, year boundaries and missing dates.

## 5. Matching and comparison meaning

**Observed:** `questionTokens` removes “favorite” and its variants. That can
merge distinct meanings such as “favorite food” and “food.” Web
`normalizeAnswer` strips punctuation/diacritics, whereas book `sameAnswer` uses
lowercasing, whitespace and limited punctuation trimming. The two surfaces can
therefore disagree about whether an answer changed.

Planned changes:

- Normalize spelling and introductory phrases conservatively; expand “fav” to
  “favorite” instead of dropping the concept. Preserve negation and qualifiers.
  Exact matches can be preselected; token/semantic matches require confirmation.
  Duplicate target questions must be resolved before writing.
- Preserve raw imported wording in the sitting snapshot when mapping to an
  existing family question. Matching an identity should not rewrite what an old
  questionnaire actually asked. Ambiguous parsing offers format/orientation
  correction and keeps the source text; never silently truncate multiline answers.
- Define one comparison rule for page, grid, books and iOS. Prefer conservative
  case/whitespace normalization and the previous answered sitting for that
  question, labeled with its actual date/age. Missing answers are not changes;
  call the result “Different answer” rather than implying a verified life change.
- Use shared language-neutral fixtures for Go, TypeScript and Swift, or return
  comparison metadata from the server. Pure TypeScript logic is not shared Swift
  logic. Test punctuation-bearing answers, accents, skipped years and same-day
  sittings explicitly.

## 6. Focused UX and integration polish

- **Phone entry:** Enter should insert a newline; use a visible Next button and
  optionally Cmd/Ctrl+Enter. Keep IME/dictation safe (composition already has a
  guard), controls reachable above the keyboard, and screen-reader focus stable.
  Default previous-answer hints off for new users to avoid influencing answers;
  preserve an existing explicit preference. Use “previous,” not always “last year.”
- **Reading/printing:** distinguish not planned, unanswered and skipped with
  accessible labels. Keep all answer text readable; printing may use multiple
  pages rather than shrinking a long interview into a one-page promise.
- **Grouped results:** folding is already implemented, but `SearchMilestones`
  limits answer hits before the frontend groups them. One interview can consume
  most of that budget. Authorize/filter first, group before card limits, and show
  matching versus total counts. Do not fill a filtered preview with unrelated
  answers. Preserve resume entry points for sittings with zero answers.
- **Bounded history:** `GetPersonInterviews` and previous-answer bundles are
  currently unbounded. Add windows/cursors if needed as history grows, keeping
  the first useful phone view to one request. Do not fetch one detail request per
  answer or require every lifetime answer before drawing the screen.
- **Access:** preserve linked-person read-only access and per-person comparison
  checks. Add explicit milestone-only link cases: portraits/attachments must
  independently pass photo access, not merely milestone access. Verify bundles,
  cards and previews without assuming an image URL grants access. This is a
  targeted verification item, not a claim of an image-serving bypass.
- **Nudges:** retain the built birthday timing and existing dismissal mechanism.
  Verify large-family behavior; cap interview prompts and link to the family
  overview rather than showing one demand per child. Alternate annual cadence
  remains optional future work.

## iOS and API handoff

Before implementing parity, verify the current iOS repository; the web roadmap
alone cannot establish its status. Prioritize conduct/resume, single-interview
reading, the phone Questions view, past entry, then question editing. Paste
import can remain web-only initially.

Document the corrected save/version, replay, date and snapshot contracts in
[Mobile API](../mobile-api.md). Preserve the established procedure names and
`Version`/`Revision` distinctions. Regenerate `frontend/server.ts` through the
local startup flow in [README](../../README.md), not by hand.

Unknown JSON fields being ignored is not sufficient compatibility. Verify how
released iOS builds render the interview category and handle answer edits,
locked dates/context, attachments and conflicts. Add `/interview/` universal-link
claims only when a released app handles those routes; until then the web page
must remain reachable. Do not claim parity based on matching screen layouts.

## Later options

Group interviews with several siblings, named question lists, OCR of paper
questionnaires, alternate reminder cadence, separate answer notes, interviewer
attribution and aggregate answer lists remain optional. None is required for
the corrective work above. Do not add unused group-mode abstractions now.
