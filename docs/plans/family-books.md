# Family books: composed views of the family record

Status: proposal for discussion and implementation planning, not a release commitment.
Date: 2026-10-01

## 1. Purpose

Family Portal already captures records and helps people explore them. Books turn those records into something worth sitting down and reading: a child's first year, a birthday retrospective, a family yearbook, or a collection of Halloweens across childhood.

The original inspiration is a baby book: photos alongside the parent's own words, firsts, growth, and ordinary moments that become precious later. Success means the existing records produce that experience without requiring a parent to enter everything again or spend an evening designing pages.

“Book” describes a coherent collection with a beginning, chapters, and an ending. Its primary form is a beautiful webpage that also works on a phone. Export, sharing, and physical printing are possible ways to deliver it, not requirements that should dictate the first reading experience.

## 2. Product model

A book combines five independent choices:

| Choice | Question | Examples |
| --- | --- | --- |
| Subject | Whose story? | One person, selected siblings, the household |
| Scope | What part of their lives? | First year, calendar year, age seven, lifetime, custom dates, recurring occasion |
| Content | Which material belongs? | Photos, milestones, quotes, artwork, growth, activities, parent-written passages |
| Structure | How should it unfold? | Chronological chapters, chapters by person, themes, a photo-led story |
| Presentation | How should it feel? | Spacious keepsake, compact recap, photo album; later print layout |

Presets choose sensible defaults across these dimensions. They should not become separate implementations with incompatible editors. Add capabilities only as actual presets need them; do not build a generic report designer or arbitrary template language.

Keep three concepts distinct in code and UI:

- **Source records:** the existing facts, photos, and text.
- **A saved book:** chosen records, section order, cover, and book-specific writing.
- **A published edition:** an explicitly approved snapshot for outside sharing or export.

Suggested user-facing name: **Books**, with “Create a book” and a description such as “Turn your family's memories into a story.” Revisit the name after seeing a prototype; “Collections” may sound more like photo organization than the intended experience.

## 3. Useful presets

| Preset | Default composition | Proposed stage |
| --- | --- | --- |
| First year | Birth introduction, month chapters, firsts, photos, quotes/notes, optional growth summary, first-birthday ending | Initial feature |
| A year of one person | Opening portrait, seasonal or monthly chapters, memorable moments, activities, growth if applicable | Initial feature |
| Family yearbook | Shared moments through the year, short sections for each selected person, closing family photo | Initial feature |
| Custom period | Same building blocks with explicit people and dates | Initial feature |
| A life / our family story | Years or eras, expandable chapters, selected highlights rather than every record | Later |
| Recurring tradition | Halloween, birthdays, Christmas, first day of school, grouped by year | Later |
| Event or season | Vacation, reunion, dance season; a deliberately small story | Later |
| At the same age | Siblings or generations at comparable ages, clearly labeled actual ages | Later |
| Short family update | A few selected moments and a personal introduction | Later; overlaps existing sharing proposal |

The first preset built should be First year. The other initial presets should follow only once its reading quality is convincing.

“Last year” must resolve to an explicit range: previous calendar year, past twelve months, or previous year of age. Display the selected dates rather than burying this distinction. Save concrete boundaries so a saved book does not shift as time passes. First year means birth up to the first birthday; treat an optional birthday celebration chapter as an explicit boundary exception. Age seven means seventh birthday up to eighth birthday. Show inclusive dates in the UI and use consistent half-open ranges internally.

## 4. What a book should feel like

An example first-year book might read as follows:

1. A large cover portrait, title, and date range.
2. “Welcome to our family”: an optional parent-written introduction with an early photo and recorded birth information, if available.
3. “The early months”: selected photos interleaved with actual notes and milestones.
4. A remembered first, given enough visual space to feel important.
5. Later chapters with smiles, ordinary days, quotes, and family moments.
6. “How you grew”: a small optional chart and a few dated measurements.
7. A closing photo and an optional letter from a parent.

Do not render this as twelve identical database cards. Vary the rhythm using a small set of designed layouts: hero image, text with image, photo grid, prominent quote, milestone group, and compact chart. Preserve the author's actual words by default. A quiet month can be combined with neighboring months; an absent record does not imply that an event never happened.

The reader should have readable typography, generous photo sizes, captions, chapter navigation, and a clear sense of progression. Use responsive scrolling first. Page-turn effects and fixed two-page spreads are optional future experiments. No editing controls, source IDs, or data-management chrome in the main reading surface.

Large histories need chapter summaries and progressive loading rather than downloading every image at once. Provide keyboard navigation, semantic headings, reduced-motion support, accessible chart descriptions, and alternatives to drag-only editing.

## 5. Creation and editing flow

1. Choose a preset from the Books area or start from a person's page.
2. Pick people and dates. Default to a balanced amount of content; offer “Brief,” “Balanced,” and “Detailed.”
3. Assemble and preview a draft immediately. Show the content mix and omissions in the editor, not in the keepsake itself.
4. Refine: replace the cover, swap photos, remove or pin items, reorder sections, and write an introduction or captions.
5. Save and read. Sharing and exports are separate explicit actions when implemented.

Use progressive disclosure. Most users should only need preset, people, and dates. Advanced controls can expose included categories, chapter grouping, photo density, and optional growth or activity sections. A later custom builder can start with an empty book and these same blocks.

Edits should be local to the book unless the user explicitly chooses “Edit original record.” A caption written for the book should not silently replace the photo's caption elsewhere. Offer a route to add a genuine missing memory to the main record, then include it in the book. An introduction or closing letter may remain book-only.

For several people, distinguish “any selected person” from “all selected people in the same photo.” Default to any. Household-wide untagged photos should be candidates for manual inclusion, not silently assumed to depict a specific child. Membership in a household does not prove historical presence at an event.

## 6. Assembly rules

Initial assembly should be deterministic and useful without AI or new analysis infrastructure.

- Collect permitted candidates using occurrence dates, selected people, and content categories. Upload dates are not event dates.
- Prefer explicit record-to-photo associations. Keep a milestone and its attached photo together.
- Distribute photos across the time range. Avoid filling a yearbook with one prolific upload day.
- Reduce exact duplicates using existing identity/hash information where available. Reuse existing similarity or quality signals if reliable, but do not require them.
- Preserve pinned choices first, then allocate suggested slots across chapters. Never randomly change suggestions on refresh.
- Give selected people reasonable representation in a family book without inventing missing content or implying equal source coverage.
- Avoid repeating a shared event separately for every sibling. Allow an intentional repeat, such as a cover photo, when selected by the editor.
- Keep original wording. Use factual headings and summaries only where supported: “Recorded first steps — May 4” is different from asserting an inferred first.
- Growth summaries use actual observations and dates. Do not label the nearest measurement “at birth” or “at one year,” or extrapolate into an unrecorded endpoint.
- Hide empty decorative sections. Offer private editor prompts such as “Add an introduction” without turning the book into an incomplete checklist.
- Record why an item was suggested, enough for an editor explanation such as “Photo attached to this milestone.”

Length controls set content budgets, not rigid promises about printed pages. The editor can inspect excluded candidates and override selection. A rich photo library may still need manual selection of emotionally important photos; a quality score cannot determine a family's favorite moment.

### Dates and uncertainty

Keep uncertain dates honest. Once approximate dates are supported, use interval overlap to identify possible candidates and preserve labels such as “Summer 1998.” Borderline matches need review; undated items belong in an editor tray until placed. Never manufacture a precise age from an uncertain birth or event date. This feature should work with today's exact dates without waiting for the approximate-date project.

Recurring traditions should begin with user-selected records, tags, or reviewed suggestions. Date proximity alone cannot establish that a photo is a Halloween celebration or school event.

## 7. Saved books and changing source data

Recommended initial behavior: save the selected references and ordering. New records do not silently join a saved book. Existing source corrections appear when the private book is read, except where a book-specific caption overrides them.

“Refresh suggestions” explicitly proposes additions and replacements. It must preserve removals, pinned items, manual ordering, and parent-written text. Start with an additions tray; full regeneration should be a separate action with a preview and a recoverable prior saved version. Optimistic version checks should prevent one parent's save from silently overwriting another's.

Private books remain source-linked. A missing or newly forbidden record disappears from the readable content and is flagged in the authorized editor. Previously computed summaries must also be recomputed or removed. Never let cached text or charts bypass current permissions.

Later, publishing or exporting creates an edition containing resolved approved content. Ordinary source edits do not silently rewrite that edition. Define deletion behavior before shipping snapshots: recommended behavior is to revoke affected online editions when a source is deleted or access is withdrawn, requiring review and republishing. Previously downloaded exports cannot be recalled. Stable editions are not a reason to keep serving content after a privacy change.

## 8. Initial scope and later options

| Capability | Recommendation | Relative effort |
| --- | --- | --- |
| Responsive book reader and a small set of attractive blocks | Essential first slice | Medium |
| First-year assembly, saved draft, photo swaps, removals, text | Essential first slice | Medium |
| Personal year, family year, custom dates | Complete initial feature after the first slice works | Medium |
| Detailed theme/font/layout controls | Later; start with one excellent design | Small–medium |
| Authenticated household reading | Initial feature, respecting existing rights | Medium |
| Revocable outside share links and snapshots | Separate follow-up milestone | Medium–large |
| Browser print stylesheet / Save as PDF | Early follow-up | Small–medium |
| Repeatable server-generated PDF | Later if demand justifies deployment cost | Medium–large |
| Offline HTML package with media | Later for a durable digital keepsake | Medium |
| Professionally printed photo book | Later investigation | Large |
| Native iOS editor/reader | Later parity; preserve a reusable API contract | Medium–large |
| AI-assisted selection or prose | Optional future experiment | Medium–large |

These are comparative estimates, not durations or release commitments. Sharing adds substantial permission and media-lifecycle work. A printable webpage is not equivalent to a professionally typeset photo book.

## 9. Sharing and export design

Start private. Reading permission must be checked against the underlying records and existing household/linked-family scopes. For the initial release, limiting saved-book access to eligible members of the owning household is simpler than promising external linked-household access. Even within the household, enforce existing category/person permissions.

An outside share flow should show the exact edition a recipient will see, allow omission of sensitive categories and exact dates, and publish only that approved result. Do not publish growth/health data by default. Only users authorized to publish the included sources may create an outside edition; access through another household's link does not automatically grant republishing rights.

Use revocable, unguessable links, optional expiry, no indexing, and a dedicated media path restricted to the edition. Every page and media request must honor revocation; an unguessable static filename alone is insufficient. Keep private API responses and source identifiers out of the public payload. Avoid third-party analytics and token-bearing logs/referrers. Link previews are optional and need approved preview content because messaging services may retain it. Noindex and link secrecy are not recipient identity checks.

PDF and offline exports should render the same resolved edition and order as the reader. Browser printing can provide an early useful option with intentional page breaks, hidden navigation, loaded images, and unclipped captions/charts. Later print production requires paper dimensions, margins, image-resolution checks, font handling, bleed where applicable, and real printer proofing. Video, if added to the source record, needs a deliberate static poster/caption fallback for print.

## 10. Fit with the current repository

Reviewed on 2026-10-01: the main-branch README, roadmap, record-ideas plan, and sharing plan. This is a product/architecture proposal grounded in those documents, not a source-level implementation audit. Verify concrete storage and handler names before coding.

The documented stack is Go with vbeam procedures, BoltDB/vbolt storage and versioned vpack serialization, plus a Preact/vlens frontend. Keep the feature within those conventions. Do not introduce another database, a separate rendering service, or a required AI dependency for the initial feature.

Suggested minimum stored shape, adapted to existing conventions:

- `Book`: owning family, title, preset, resolved scope, cover reference, ordered sections, selection settings, excluded source references, created/updated metadata, schema version, edit revision.
- `BookSection`: stable local identifier, kind, optional heading/text, ordered items and relevant layout settings.
- `BookItem`: stable local identifier, source kind and ID when applicable, optional caption/crop override, pinned state; authored text blocks need no source reference.
- `BookEdition` (only when sharing/export requires it): book revision, resolved content, explicit asset manifest, publication policy, creation time. Share grants can reference editions separately so revocation does not destroy the private draft.

Use a small fixed set of block kinds, not a deeply nested document engine. Support sections and ordered blocks without arbitrary nesting. Keep logical content separate from pixel positions so responsive web and print can arrange the same story differently.

Implement ordinary domain functions for candidate gathering, selection, and assembly. Fetch permitted records, assemble a logical document, and render it. Reuse date, chart, media, and permission helpers. Keep permission checks on the server. Introduce background jobs only for proven expensive work such as export rendering, following the existing worker conventions.

The roadmap requires every new entity to include storage/indexes, versioned serialization, registered procedures, family isolation tests, export/import coverage, and person merge/delete behavior. Books need all of these. Import must remap source IDs; unresolved references must be surfaced rather than accidentally pointing at unrelated records. Include book-only writing and configuration in full-family export from the start.

### Relationship to existing proposals

- Year-in-review and print/PDF ideas in `record-ideas.md` become presets/output options of this feature.
- Suggested event collections can supply candidates or a starting chapter; a photo grouping is not itself a finished book.
- Same-age montages can become a block or preset later.
- Christmas-card updates can potentially reuse the book reader, approved-edition model, and share infrastructure. Do not add a second composer before evaluating that overlap.
- A public family summary remains a distinct purpose even if it shares publication infrastructure.
- Birth details, interviews, video, approximate dates, and other future record types should enrich books when available. None are prerequisites.

Keep existing proposal files intact while discussing this plan. Cross-link and consolidate overlapping ideas only when the direction is accepted.

## 11. Implementation sequence and acceptance gates

### Phase 0: prove the reading experience

Build a disposable or fixture-backed First year reader with the proposed block types. Use rich, sparse, and uneven sample records. Review phone and desktop versions together with the person who would actually make the baby books.

Acceptance: it feels enjoyable to read; original notes are prominent; sparse data still looks intentional; the design does not depend on a perfect photo for every month. Choose the visual direction before building extensive controls.

### Phase 1: one complete private book

Implement First year creation, authorized data assembly, persistence, cover/photo swaps, remove/pin/reorder controls, book-only writing, and private reading. Include the entity lifecycle and full-family import/export work.

Acceptance: a parent can create a satisfying draft from existing records in a few minutes, improve it without re-entering records, leave, and reopen the saved result. New uploads do not rearrange it. Corrected/deleted sources behave as documented. No cross-family data appears in candidate lists or reading responses.

### Phase 2: generalize only what the next presets need

Add personal year, family year, and custom range; Brief/Balanced/Detailed controls; additions review; explicit content-category selection; and a Books index. Reuse the same saved shape and reader.

Acceptance: shared moments are not duplicated per child, coverage is reasonably distributed, date meanings are explicit, and refreshing suggestions preserves user decisions.

### Phase 3: choose the next delivery need

Choose sharing or print based on actual use after private books work. If sharing comes first, build edition snapshots, preview, publication authorization, restricted assets, revocation, and deletion handling together. If printing comes first, ship an honest browser-print option before promising production-quality PDFs.

Acceptance: shared output matches the approved preview; revocation blocks both pages and media; private records are not accessible through public routes. Printed output has readable text, loaded images, sane breaks, and no cut-off content.

### Phase 4: expand from observed demand

Add traditions, whole-history books, same-age comparisons, additional styles, offline packages, native parity, or optional AI assistance individually. AI may suggest choices or draft text for review, but must never invent events, developmental firsts, dates, or personal feelings. The feature remains fully usable without it.

## 12. Targeted verification

- Selection/date tests: first birthday, leap-day births under the app's age convention, timezone boundaries, source date vs upload date, future/undated data, and sparse chapters.
- Curation tests: pins/removals/order survive refresh; shared photos deduplicate; captions stay book-local; deterministic suggestions do not shuffle.
- Lifecycle tests: source edits/deletes, permission withdrawal, person merge, concurrent saves, export/import with remapped IDs, and family deletion.
- Permission tests: unauthorized candidates, cached charts/text, cross-family access, linked-household republishing, and later share-media revocation.
- Reader/editor checks: mobile widths, long captions, portrait/landscape images, keyboard operation, large histories, and loading failures.
- Later print checks: multipage output, font/image readiness, long paragraphs, charts, and very low-resolution photos.

## 13. Decisions to revisit with a prototype

The defaults above let work begin without settling every future possibility. The highest-value questions are:

1. Does the first-year draft feel more like the physical baby books when organized by months, themes, or a blend? Start with chronological chapters plus themed highlights.
2. How much selection should happen automatically? Start with an editable balanced draft and make omissions inspectable.
3. How much writing belongs only in a book? Support introductions/captions immediately; encourage actual memories to also be saved as source records.
4. What is the next real use after private reading: sending Grandma a link or printing a keepsake? Let that determine Phase 3.
5. Should a completed private book eventually be frozen too? Start source-linked; introduce an explicit “Save an edition” action when snapshot semantics are implemented.

The initial success criterion is simple: open a child's book and want to keep reading. More settings, output formats, and clever summaries matter only when they improve that experience.

## References

- https://github.com/sggrissom/Family-Portal/blob/main/README.md
- https://github.com/sggrissom/Family-Portal/blob/main/docs/plan.md
- https://github.com/sggrissom/Family-Portal/blob/main/docs/plans/record-ideas.md
- https://github.com/sggrissom/Family-Portal/blob/main/docs/plans/sharing.md
