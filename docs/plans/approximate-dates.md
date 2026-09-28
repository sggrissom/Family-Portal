# Approximate dates and ages

Status: planned direction; not implemented. Build manual support first.
AI assistance is a later experiment, not a dependency or provider commitment.

## Goal

An old photo or remembered milestone should be useful even when nobody knows
its exact date. Preserve what the family knows without inventing precision.
This particularly supports same-age comparisons between children, parents,
and grandparents.

Initial scope is photo dates, followed by remembered milestones. Measurements
keep their existing exact-date requirements; do not infer clinical measurements
or silently broaden every date field. Partial birth dates are a separate
decision, but comparison must tolerate an unknown birthday.

## Entry and representation

Allow:

- an exact date
- a known month and year, or year only
- an approximate date such as "around 1970"
- an explicit range such as "between 1968 and 1971"
- a person's approximate age, such as "about age 8" or "age 6–9"
- an unknown date

Distinguish precision from uncertainty: "sometime in 1970" is not the same
claim as "around 1970." For an approximate point, show and allow editing the
range used for matching; do not apply an invisible universal tolerance.

Keep the original assertion as well as derived bounds. An age assertion names
its anchor person and depends on that person's birth date; it must survive
correction of the birthday without turning into an unrelated fixed date.
If the birthday is unknown, keep the age assertion and leave calendar dates
unresolved. A photo with several people uses an explicitly chosen anchor.

Preserve:

- the entered date/age expression, precision, and uncertainty
- earliest/latest plausible bounds where derivable
- the source: family recollection, inscription, original metadata, or AI suggestion
- an optional supporting note and the person who accepted or changed it

The final schema needs design before implementation. Do not squeeze this into
one timestamp plus a display label. Any internal sorting key is not the
recorded date and must never be exposed as one.

A scan's creation/upload date is not evidence of the original photo date.
Keep occurrence dates separate from file metadata and creation timestamps.
Do not reinterpret existing exact dates during migration.

## How it appears throughout the app

| Surface | Behavior |
| --- | --- |
| Photo or milestone detail | Show the date/age label, uncertainty, source, and supporting note. Make refinement easy. |
| History | Label approximate periods honestly. Broad ranges must not appear to have a known day or exact order relative to other records. Keep undated records reachable. |
| Date filters | Include overlapping ranges with an approximate-match label; explain the matching behavior. |
| Same age | Label estimated ages and distinguish possible overlap from a precise match. Do not let a broad range masquerade as the nearest exact-age record. |
| Future family-at-a-date views | Mark an overlapping estimate as "Possibly from this period." |
| On this day | Use known anniversary dates; a year-only or estimated day must not produce a false anniversary. |
| Export/import and sharing | Preserve precision, bounds, source, and notes where appropriate to the audience. Never flatten an estimate into an exact date. |

Keep exact matches and possible matches distinguishable. Decide and test the
ranking rules for wide ranges before enabling approximate Same age results.
When a birthday is missing, omit calendar-derived ages rather than guessing.
Any future partial-birthday support must propagate that uncertainty too.

Refining an estimate must update derived age/date matches. Preserve the
distinction between the family's assertion and a computed interpretation.

## Optional "Help estimate"

The manual workflow is complete on its own. A later, explicitly invoked helper
may propose a range from a selected photo and information the family supplies.

Illustrative flow:

1. The user identifies the person: "This is my mom when she was young."
2. They may add context: her birth year, a known school year, or "before they
   moved in 1970." Show which existing facts would be used.
3. The helper suggests an age/date range and a brief explanation of the clues
   and limitations. Example: "Estimated age 6–9; approximately 1968–1971."
   These are illustrative values, not an accuracy promise.
4. Offer Use this estimate, Adjust, and Leave undated.
5. Only an explicit acceptance writes the proposed estimate to the record.
   Keep it labeled AI-suggested even after acceptance.

Do not infer identity from the image. The user supplies the person.
Appearance, clothing, and image quality can be misleading; the helper can say
there is not enough evidence. Do not manufacture narrow ranges or numeric
confidence scores. User-provided constraints must be honored, and conflicting
evidence should trigger a question rather than a silent override.
Never overwrite existing metadata or family-supplied facts automatically.

Avoid ambient analysis, automatic backfills, or processing the whole library
for this feature. It operates only on selected records at the user's request.

No provider is selected. Evaluate a small, consented sample of historical
photos with known dates before deciding whether the estimates are useful.
Check range coverage, misleading precision, correction effort, latency, and
cost. Broad age estimates may be the honest limit of the feature.

If an external service is considered, it is an explicit exception to local
processing: disclose what photo and contextual information will be sent and
obtain an affirmative choice before sending it. Do not send unrelated records.
Provider retention and data handling need evaluation before integration.
A local model is an option only if its quality and operating cost justify it.

The app must work normally with the helper absent, disabled, or unavailable.
Removing an integration later must leave accepted records readable and editable.

## Delivery sequence

1. Define date/age semantics, serialization, migration, and mobile contracts.
2. Ship manual photo entry/editing and honest detail labels.
3. Add History/filter behavior, unknown-date retrieval, and export/import support.
4. Add Same age matching with visible uncertainty and shared backend rules.
5. Extend the representation to remembered milestones.
6. Evaluate optional AI estimation; build only if it saves useful effort.

Validation should include year-only and unknown dates, overlapping broad
ranges, scans with recent metadata, changing an anchor's birthday, missing
birthdays, multi-person photos, rejected AI suggestions, and export/import
round trips. Existing exact-date behavior must remain intact.
