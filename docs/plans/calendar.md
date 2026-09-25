# Shared calendar

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
