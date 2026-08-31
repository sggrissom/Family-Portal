# Roadmap

Where the app is going next. Short term is work on what already exists; the
rest are new features, roughly in the order they'd be worth building.

## Short term

### UI pass for the current feature set

The interface grew one feature at a time and shows it. Activities (seasons,
competitions, routines, results) landed last and never got folded into the
overall navigation and visual language the way growth, milestones, and photos
were. Go through the app as a whole and make it look like one product:

- Navigation that reflects all the top-level areas, not just the original ones.
- Consistent page structure — headers, empty states, and action placement that
  match across people, growth, milestones, photos, activities, and the timeline.
- Empty and first-run states everywhere. A brand new family currently sees a lot
  of blank pages with no path forward.
- Mobile layout check on every page, not just the ones built mobile-first.

### Landing page rework

The landing page describes a different app than the one that exists. It
advertises a shared calendar (not built) and group chat, photos, and a "family
space", while saying nothing about growth charts, milestones, activities, the
family timeline, or sibling comparison — which are the actual reasons to use it.

Rewrite it around what's really here:

- Growth tracking and side-by-side sibling
  comparison.
- Milestones with tags and search.
- Photos with automatic resizing, EXIF dates, tagging, and face recognition
  that suggests who's in a picture.
- Activities — seasons, competitions, routines, and per-event results.
- Family timeline pulling all of it into one chronological view.
- Real-time family chat.
- Private and self-hosted: one Go binary, one database file, no third-party
  services, no ads, no algorithm.

Drop the calendar claim until the calendar actually ships. Screenshots of the
real thing would carry more weight than the current icon cards.

## Future features

Each of these is a new domain entity, which in this codebase means: a vbolt
bucket plus the indexes to query it, a versioned `vpack` pack function, family
scoping with a cross-family isolation test, `RegisterProc` handlers (the
TypeScript client generates itself), and coverage in export and import so a
family can still take their data with them.

### Shared calendar

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

### Meal tracking with kid ratings

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

### Location sharing

Show where family members are on a map, with optional notifications for arriving
at or leaving known places like home, school, or work.

This one is different from everything above in ways that matter:
- It needs the mobile app to report position in the background, so it is
  primarily mobile app work, not backend work.
- It generates continuous writes rather than occasional ones, so retention has
  to be decided up front. Keeping current position plus a short trail is very
  different from keeping history forever.
- It is the most sensitive data the app would ever hold, and the kids being
  tracked are not the ones who chose to install it. Per-person visibility
  controls and a clear, obvious indicator that sharing is on are requirements,
  not polish.

Given the mobile dependency and the privacy weight, this should come last.

## Not planned

Things worth naming so they don't get accidentally re-litigated:

- No ads, no third-party analytics, no data leaving the host.
- No dependency the app can't start without. New features hold to the same rule:
  one binary, one database file, a directory of images.
