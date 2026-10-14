# Not planned: household management

Family Record is a record of a family, not a tool for running one. These
proposals manage the household day to day, which would turn the app into a
different product. They are not being built. They're kept here, out of the
roadmap, so the thinking isn't lost if that decision is ever deliberately
reopened.

## Shared calendar

Dance schedules, appointments, practices, school events, birthdays — the
recurring logistics that currently live in someone's head or a separate app.

Scope:
- Events with a title, start/end, all-day flag, location, notes, and the family
  members they apply to.
- Recurrence, at least weekly and monthly. This is the part that gets expensive
  to retrofit, so decide the recurrence model before writing the bucket.
- Birthdays derived automatically from existing person birth dates rather than
  entered twice.
- Reminders through the existing push worker.
- Read-only iCal feed per family so the built-in phone calendar can subscribe.
  A subscription URL is far less work than two-way sync with Google or Apple
  Calendar and covers the actual need: seeing family events next to everything
  else on your phone.

Open question: whether writes ever need to flow back from the phone calendar,
or whether a subscribe-only feed plus adding events in the app is enough. Start
subscribe-only.

## Chores and allowance

### Chores and shared to-do list

The kids have rotating weekly chores. The app should track whose turn it is for
what, on what day, instead of anyone having to remember the rotation.

Scope:
- Chore definitions: name, description, which days it happens, and the rotation
  of people it cycles through.
- Automatic assignment — given the rotation and the date, the app knows whose
  turn it is. No manual reassignment each week.
- Completion marking, with history, so "did anyone actually do it" is
  answerable.
- One-off tasks alongside recurring chores, so the same screen works as a plain
  family to-do list.
- Optional reminders via push.

Open questions: whether a missed chore rolls over or is just skipped, and
whether a parent needs to approve completion or the kid marking it done is
enough. Approval matters if this feeds the money tracking below.

### Family bank — allowance and kid finance

Tie into chores. Some chores earn money; kids also get gifted money at Christmas
and birthdays. Track what each kid has and what they've spent.

Scope:
- A balance per kid, built from a ledger of entries rather than a stored number.
- Entry sources: chore completion, recurring allowance, gifts, and manual
  adjustments.
- Spending entries — a parent marks that some of it was spent, with a note on
  what for.
- Per-kid history so a kid can see where their money came from and where it
  went.

This is bookkeeping, not banking: no real money moves, so it just has to be
honest and easy to correct. Store amounts as integer cents; never floats.

Open questions: whether kids can log their own spending or only parents can, and
whether to support goals ("saving for X") — probably worth it, since that's most
of why a kid wants to see the balance at all.

## Meal tracking with kid ratings

Track what gets cooked and what everyone actually thought of it, then use the
history to decide what to make.

Scope:
- Meals composed of parts — entree, sides, dessert — each rated on its own. A
  kid can like the chicken and hate the green beans, and averaging that into one
  score loses the useful information.
- Optional ingredients and method per component, so this doubles as a recipe
  box.
- A cooked-meal record: date plus which components were served.
- Ratings per person per component, 1-5, entered after the meal.
- Photos, reusing the existing photo pipeline.

The payoff is the queries, so build with those in mind:
- What does a specific picky kid actually like?
- What does everyone like that hasn't been made in a while?
- What's never been rated, so it's untested?
- What is nobody's favorite and can quietly retire?

Open question: whether ratings are per-cooking or per-component-overall. Per
cooking is more honest (the same dish varies) and can still be averaged up.

## Location sharing

Show where family members are on a map, with optional notifications for arriving
at or leaving known places like home, school, or work.

This one is different from the others in ways that matter:
- It needs the mobile app to report position in the background, so it is
  primarily mobile app work, not backend work.
- It generates continuous writes rather than occasional ones, so retention has
  to be decided up front. Keeping current position plus a short trail is very
  different from keeping history forever.
- It is the most sensitive data the app would ever hold, and the kids being
  tracked are not the ones who chose to install it. Per-person visibility
  controls and a clear, obvious indicator that sharing is on are requirements,
  not polish.
