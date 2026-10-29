# Usability backlog

Findings from a hands-on pass over the local web app (main at 0f90d72, Oct
2026): a fresh account on an empty database, an invitee joining by link, a
second fresh account, then the seed accounts. None of it is large on its own;
there's just a lot of it. No priority order — pick anything.

The browser window during testing was stuck at 621×207, so the notes about
fixed bars covering controls may be partly an artifact of that. Check them on a
real landscape phone before fixing.

## Bugs

- [x] **B1 — Future birthday accepted.** Add family member with birthday
  2027-03-01 and "Baby isn't born yet" unticked saves, and the profile reads
  "Daughter · Newborn · born March 1, 2027".
- [x] **B2 — Implausible measurements accepted.** 610 lb for a 9-year-old saves
  with no warning and shows "~99.9th %ile". No check against age or the
  previous value.
- [x] **B3 — No kg on Add measurement.** The weight unit picker offers only lb
  and lb/oz. Height offers cm, and Edit offers kg, so metric users can't add a
  weight in kg.
- [x] **B4 — "By age…" accepts an age older than the person.** "Age 9" on a
  6-year-old's moment saved it on Feb 14, 2029, and it now tops History and
  their Story. There's no preview of the resolved date.
- [x] **B5 — Ordinal suffix.** `ordinalSuffix` in
  `frontend/lib/growthPercentiles.ts` only special-cases 1, 2 and 3, so 21–23,
  31–33 … 91–93 read "~91th %ile". (11–13 must stay "th".)
- [x] **B6 — Season end before start accepted.** Start 2026-09-01, end
  2026-06-01 saves and files the season under Past seasons.
- [x] **B7 — Game outcome can contradict the score.** "Won" with 1–3 saves and
  displays "Won 1–3". Derive the outcome from the score, or flag a mismatch.
- [x] **B8 — Scroll position carries over on route change.** Starting an
  interview opens the ask page still scrolled down from the long setup page, so
  the question is off-screen. Other route changes may do the same.
- [x] **B9 — Typo.** "Not yet at a event" on a team card with no results.
- [ ] **B10 — Blank page on first load (unconfirmed).** Twice a page rendered
  fully blank on a cold load (`/` and `/profile/3`) and was fine on reload. No
  console errors were captured.

## Family setup

- [ ] **Joining by invite creates a duplicate person.** Dana added "Marco
  Rivera" with the kids linked; Marco then joined by invite link and got a
  second, unlinked "Marco Rivera". Offer "Are you one of these?" from the
  family's adults without an account and link the account instead. The join
  form also says "We'll add you as the first adult in this family" when the
  family already has adults.
- [ ] **Merge People is hard to use safely.** Both pickers list two identical
  "Marco Rivera" entries, and the confirm step reads "Source: Marco Rivera →
  Target: Marco Rivera". Merging the wrong way deletes the person holding the
  relationships. Show birthday, relationship count and "has an account" next to
  each name in the pickers and on the confirm step. (It did correctly move the
  account to the kept person.)
- [ ] **Kids added before the spouse make the spouse a step-parent.** Sign up,
  add two children, then add a wife: she shows as "stepmother (implied)". The
  co-parent checkbox only appears when adding a child to someone who already
  has a partner. After saving a partner, ask "Is Jordan also the parent of Ada
  and Ben?" with the kids pre-checked. This order is probably the most common
  onboarding path.

## Sign-up and people

- [x] Birthday is required at sign-up but not marked as such; Family Code says
  "(Optional)" and Birthday says nothing, so the first submit fails on a field
  below the fold.
- [x] Password rules ("at least 8 characters") only appear after a failed
  submit.
- [x] A brand-new account is greeted with "Welcome back, Dana Rivera!" seconds
  after sign-up.
- [x] Gender wording and defaults differ: sign-up says "Prefer not to say"
  (default); Add family member says "Unknown" and defaults to Male.
- [x] The moment placeholder is gendered ("Wrote her name for the first time")
  regardless of who it's for.
- [x] The unborn baby can't be picked in the + sheet, so there's no way to log
  an ultrasound photo or pregnancy moment, even though the model supports it
  (seed data has a "Twenty-week scan" photo).

## Timelines

- [x] History and Story are mostly "X turned N" rows. With six people (one born
  1960) the 2024 section is birthdays plus one photo, and an adult's Story is
  40 birthday rows and nothing else. Hide or collapse birthdays, at least for
  adults.
- [x] Measurement detail lists every relative with "No height data recorded
  yet." Drop empty relatives or summarise them in one line.
- [x] A game result doesn't appear in the child's Story; it shows "vs Red Foxes
  / Blue Hawks" but not the outcome or score.

## Forms and defaults

- [x] The + sheet remembers the last person (`last-person-id`) even outside
  that person's context. Add photos from the Photos tab arrived with Leo
  preselected, so tapping Sofia and Leo tagged only Sofia. Fine for one-person
  forms, risky for multi-tag photos.
- [x] Validation reports one field at a time (height, then weight on the next
  submit).
- [x] "abc" as a height says "Enter a height above zero" — right outcome, odd
  message.
- [x] Photo upload: "Retry" on a non-image file can never succeed, and Done
  stays blocked until the bad file is removed. Done could skip it.
- [x] Add photos accepts `image/*` only, though video is enabled in local
  config. *(Intended: video is also a per-family switch the site admin turns
  on in Admin → Users; `mediaAccept` already takes video once it's on.)*
- [x] The book year dropdown runs 2026 → 1960 (the oldest family birth year)
  for everyone, including a 6-year-old.
- [x] Team form shows dance fields (Format, Style) on a Sport program, and the
  Players list includes the unborn baby and grandma, adults first.
- [x] Event form has "Against" separate from the event name, so "vs Red Foxes"
  gets typed twice. Pre-fill one from the other.

## Multi-family accounts

- [ ] Adding a person never says which family it goes into. The view-only
  sitter added "Sitter Test"; it went silently into their own household, but
  the relationship dropdown listed only the other family's people, and the
  dashboard mixes both families' people with no labels.
- [ ] Chat is per-family and unlabeled. Grandma lands in an empty chat for the
  two-person grandparents' family and can't reach the Whitfield thread, with
  nothing saying whose chat it is. Label the family in the add-person form,
  chat header and dashboard groups.
- [ ] Same age opens on "Everyone at 38 years 6 months" (the parents' age). A
  child's age, or the youngest child's, would be a more useful default.

## Layout and copy

- [x] The email-confirm banner and bottom nav are fixed and tall; in a short
  window they covered the Add Family Member submit button. *(Not reproducible
  at 844×390 or 667×375 — an artifact of the 207px-tall test window.)*
- [x] The account dropdown can't clear the fixed bottom nav in a short window,
  so Log out was unreachable by click.
- [x] Photo grouping merges different photos: three distinct uploads from 2023,
  2024 and 2025 showed as one card ("1 photo · +2 similar", dated 2/14/2025,
  naming one person).
- [x] A book with nothing in range shows "CONTENTS ❦" and an empty body, with
  no "nothing recorded in 2024" note.
- [x] The Books page links "Sample books: rich · sparse · uneven · family year".
  These read as dev fixtures and aren't DEBUG-gated
  (`frontend/pages/books/books.tsx`).
- [x] Faces and Suggestions links show in the Photos header even when those
  services aren't running; both pages then say there's nothing to review. Gate
  them on availability.
- [x] Interviews card says "No interviews yet" next to "Continue · 2 of 46".
- [x] Search snippet says "1 of 46 answers" for an interview with two answers,
  and lists the interview under MOMENTS.
- [x] Chat shows newest messages on top and doesn't linkify URLs.

## Checked and fine

- Permissions: view-only got a clear "View only" page on edit, and the
  contribute/admin splits behaved as the seed summary describes.
- Chat escapes HTML correctly.
- Admin being user id 1 is intentional for now.
