# Chores and allowance

## Chores and shared to-do list

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

## Family bank — allowance and kid finance

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
