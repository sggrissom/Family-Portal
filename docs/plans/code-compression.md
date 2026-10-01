# Code compression and cleanup

Status: phases 1–6 done.

Based on a structural review of main at `0c3e690` on 2026-09-30, with deeper
inspection of growth, milestones, imports, permissions, photos, and workers.
Recheck the relevant code before implementing each phase.

## Goal

Make the system simple and highly understandable. Measure progress by how many
places someone must understand and change to modify one behavior, and how many
intermediate states they must reason about. Fewer lines are useful only when
they reflect less duplicated knowledge or less coordination.

Preserve the current direct architecture: one Go application, typed BoltDB
storage, generated RPC bindings, and concrete frontend components. Prefer
ordinary functions and data. Do not introduce a generic repository, form
schema, validation framework, event bus, or universal worker framework.

Work in focused PRs. Preserve visible behavior except for explicitly described
bug fixes. Moving code to smaller files alone is not completion.

## Sequence

| Phase | Scope                         | Intended result                                                          | Effort      |
| ----- | ----------------------------- | ------------------------------------------------------------------------ | ----------- |
| 1     | Milestone storage and imports | One implementation of index maintenance; imported records are searchable | Small       |
| 2     | Backend entry rules           | Concrete date parsing and one set of field rules per feature             | Small       |
| 3     | Milestone and checkup saves   | One transaction per user save; less browser recovery logic               | Medium      |
| 4     | Add/edit entry forms          | Shared date and measurement rules with appropriate page layouts          | Medium      |
| 5     | Membership authority          | One documented source of authority, with a deliberate migration          | Medium–high |
| 6     | Photo listing and search      | Independently readable browse and search algorithms                      | Medium      |

Phases 1–4 are the first implementation scope. Phase 1 is independently useful;
phase 4 can reuse phase 2's clarified rules and phase 3's atomic operations.
Phases 5 and 6 are separate follow-up work and do not block entry cleanup.

## 1. Give storage invariants one implementation

Relevant code: `backend/import.go:importMilestones` and
`backend/milestone.go:updateMilestoneIndices`, `AddMilestoneTx`, and
`UpdateMilestoneTx`.

Normal milestone creation updates the person, family, and text-search indexes.
Import manually updates only the first two. Semantic embedding jobs do not
populate the missing text-search index.

- [x] Route imported milestones through the existing index helper.
- [x] Add a regression test that imports a milestone and finds it through text
      search without a vision daemon, for both JSON and bundle entry points where
      practical. Verify an unrelated family cannot find it.
- [x] Repair already-imported records with a new, named, one-time search-index
      rebuild. The existing startup migration has already run on deployed databases;
      changing its body or relying on restart will not repair those databases.
- [x] Check nearby creation/update/import paths for duplicated write-and-index
      knowledge. Extract only concrete per-record helpers that actual callers need.

Keep import's matching, duplicate handling, and import-specific validation in
the import flow. Do not force imports through RPC handlers. Keep transaction
ownership with the operation's caller and enqueue background work after commit.

Done: `writeMilestone`/`writeGrowthData` are the only record writers (add,
update, import, person merge); link rows use one delete helper each; startup
runs `2026-0930-rebuild-milestone-search`, replacing the 2025 populate step.

Acceptance: imported and manually created milestones have equivalent index
coverage; edits and deletion maintain search correctness; the repair can safely
run on an existing database without altering record content or IDs.

## 2. Use concrete backend entry rules

Relevant code: `backend/growth.go` and `backend/milestone.go`.

`MilestoneDateRequest` has four getters implemented on two request types solely
to parse date fields. Growth duplicates the date algorithm and constructs an
add request inside the update path to reuse it. Add/update validators also
repeat each feature's field rules.

- [x] Replace the getter interface and request-to-request conversion with one
      date resolver using concrete arguments or a small internal input struct.
- [x] Share growth field validation between add and update, leaving their
      distinct ID checks at the endpoints.
- [x] Share milestone description/category validation between add and update.
- [x] Keep the existing wire request fields, JSON names, serialized records,
      error ordering, and user-facing errors unless a separate behavior fix is
      explicitly justified.

Done: `resolveEntryDate` (backend/entry_date.go) is the one date resolver for
growth, milestones, and milestone photo suggestions; `validateMeasurementFields`
and `validateMilestoneFields` hold each feature's field rules. Photo dates use it
too (`calculatePhotoDate`), leaving only EXIF "auto" photo-specific.

Acceptance: existing date and validation tests pass for add and update, with
coverage for missing dates, invalid dates, missing/negative years, months outside
0–11, default months, and birthday-relative calculations. Preserve current
`today` and `AddDate` behavior in this refactor. Changes to time zones, leap-day
policy, or date precision belong in separate behavior changes.

This does not implement [approximate dates](approximate-dates.md), nor require a
general date representation designed for that future feature.

## 3. Make a user save one transaction

### Checkups

Relevant code: `frontend/pages/growth/add-growth.tsx:saveCheckup` and
`backend/growth.go`.

Saving height and weight currently makes separate requests. The browser tracks
partial successes, clears saved fields after failure, and constructs a recovery
message. Move that coordination into one database transaction.

- [x] Add a concrete `AddCheckup` RPC accepting one person, one date/age input,
      and one or both measurements. Final field names can follow existing conventions.
- [x] Validate every supplied measurement and authorization before committing;
      resolve the shared date once and return the created records/IDs.
- [x] Keep `AddGrowthData` working for existing clients. Share the concrete
      measurement rules and storage writes, not a generic batch-operation framework.
- [x] Switch the add screen to the atomic RPC and delete partial-save bookkeeping.

Acceptance: height only, weight only, and both succeed; an empty checkup fails;
an invalid second measurement leaves neither saved; foreign-family writes fail;
the existing single-measurement API continues to work. The receipt/navigation
still works for one or two created records.

Atomicity does not make transport retries idempotent. Do not introduce automatic
create retries on an ambiguous network failure or claim this change prevents
duplicates after a lost response. Any retry protocol needs a separate decision.

### Milestones and tags

Relevant code: `frontend/pages/milestones/add-milestone.tsx`,
`edit-milestone.tsx`, and `backend/milestone.go`.

Both screens save the milestone and then call `UpdateMilestoneTags`; neither
checks that second request's returned error before navigating away.

- [x] Include optional tag IDs in add/update requests and save the milestone,
      photo associations, and tags in the same transaction.
- [x] Preserve omitted/null tag IDs as unchanged on update; an explicit empty
      array clears tags. Omitted tags on creation produce no tags.
- [x] Retain `UpdateMilestoneTags` for existing clients and reuse its concrete
      authorization and tag-association rules without calling the RPC handler.
- [x] Remove the second request from each browser save path.
- [x] Queue embedding work only after a successful commit.

Acceptance: omitted, empty, and populated tags behave correctly; invalid or
unauthorized associations cause no partial changes; existing clients' requests
still work; the browser reports failure without navigating away as if saved.

Done: `AddCheckup` takes optional `height`/`weight` values against one resolved
date; `AddMilestone`/`UpdateMilestone` take `tagIds` and share
`setMilestonePhotos`/`setMilestoneTags` with `UpdateMilestoneTags`. The edit
milestone screen still has its own date/validation state; phase 4 replaces it.

Follow [the mobile API contract](../mobile-api.md). Regenerate
`frontend/server.ts` through the normal development tooling; do not hand-edit it.

## 4. Reuse entry rules across add and edit

Relevant code: `frontend/lib/when.ts`, `checkup.ts`, `weightFormat.ts`,
`frontend/components/WhenControl.tsx`, `frontend/pages/growth/GrowthForm.tsx`,
and the milestone add/edit screens.

Newer add screens use shared date controls and measurement helpers; older edit
screens independently manage date/age input and conversions. Parsing has already
diverged: `Number()` in the checkup helper rejects `12abc`, while `parseFloat()`
in the edit form accepts its numeric prefix.

- [x] Use `WhenControl` and its request/validation helpers for milestone and
      growth editing, with initialization from the existing record.
- [x] Share concrete per-measurement parsing/conversion rules. Do not force a
      single-measurement editor to construct a whole checkup just to validate it.
- [x] Make numeric acceptance consistent and explicitly test the chosen behavior
      for malformed input, non-finite values, negative values, and compound units.
- [x] Preserve units the existing editor supports, including kilograms; do not
      narrow support to the newer checkup screen's available unit buttons.
- [x] Keep distinct page layouts and submit operations where workflows differ.
      Remove superseded state, validation, and handlers after migrating callers.

Done: `heightValue`/`weightValue` (lib/checkup.ts) are the one parser for each
measurement; `parseAmount` accepts plain decimals only. `HeightField`,
`WeightField`, `CategoryChips`, and `TagPicker` render both add and edit. The
edit screens use `WhenControl`; their preview panels and radio-button state are
gone.

Acceptance: add/edit agree on date and measurement rules; existing records open
and save without unwanted unit/date changes; feet/inches and pounds/ounces work;
double submission is guarded; save errors leave entered data available.
Check the changed add/edit flows in a browser as well as through helper tests.

## 5. Resolve membership authority separately

Relevant code: `backend/access.go`, `membership.go`, `membership_procs.go`,
`users.go:GetFamilyUserIds`, and `app.go` membership backfill.

Membership rows coexist with primary-family fallback authority.
`CanAccessFamily` can fall through from an insufficient membership role to
primary-family admin access, while `userRoleIn` returns an existing row's role.
Family-user enumeration combines membership and the primary-family index.

- [x] Enumerate all writers, fallbacks, and legacy records before changing policy.
- [x] Decide and document whether primary-family membership is inherently admin
      or whether membership rows determine roles and `User.FamilyId` only selects a
      default household. Do not silently choose during a mechanical refactor.
- [x] Build a permission matrix covering primary/secondary membership, missing
      rows, lower roles, leave/removal, owner reassignment, and account deletion.
- [x] Implement any backfill/invariant repair before removing runtime fallbacks.
      Preserve valid access while ensuring removed membership cannot regain it.
- [x] Update `docs/permissions.md`, migration comments, and fixtures to match the
      selected policy; some descriptions still reflect earlier rollout stages.

Decided 2026-09-30: membership rows determine access and role; `User.FamilyId`
only selects the default household. Fallbacks removed from `CanAccessFamily`,
`userRoleIn`, `familiesVisibleTo`, `GetFamilyUserIds` (and the unread
`users_by_family` index), `joinFamilyByInviteTx`, and the chat socket, after the
`2026-0930-ensure-primary-memberships` backfill. Matrix:
`backend/membership_matrix_test.go`.

Acceptance: the matrix passes across RPC and relevant HTTP/WebSocket surfaces;
family links remain read-only and scope-specific; primary family cannot become
a dangling/default authority accidentally. This phase changes authorization
semantics and must have its own reviewed migration and regression tests.

## 6. Separate photo browse and search algorithms

Relevant code: `backend/photos.go:ListFamilyPhotos`, `photo_index.go`, and
`photo_search.go`.

The endpoint combines chronological paging, ranked search, two cursor formats,
filtering, similarity collapsing, and response enrichment.

- [x] Keep the public endpoint and wire format stable.
- [x] Give chronological browsing and ranked search separate concrete functions.
- [x] Share visibility/filter predicates and response assembly where their
      meaning is identical. Keep the intentional differences visible.
- [x] Keep daemon calls and database transaction lifetimes explicit during the
      extraction. Do not add caching or change ranking as part of this refactor.

Done: `ListFamilyPhotos` builds one `photoListing` (window, streams, filter)
and calls `browsePhotos` (photo_index.go) or `searchPhotos` (photo_search.go);
response assembly stays in the endpoint.

Acceptance: existing paging/search/isolation tests pass; check person and tag
filters, inclusive date bounds, location visibility, unavailable vision fallback,
similar-photo covers under filters, and cursor continuation without duplicates
or skipped records. Preserve the legacy `personId` request field and current
limit behavior for existing clients.

## Validation and completion

For each implementation PR, describe which duplicated rule or intermediate
state was removed, which behavior intentionally changed, and what remains.
Use existing tests plus focused regressions for the changed behavior. Run the
repository's applicable checks and CI gates; avoid tests that merely mirror a
new helper's implementation.

The review baseline passed 193 frontend tests and TypeScript checking. Backend
tests were not run during that review because the environment lacked Go; this
is not evidence of a backend failure or a waiver for implementation validation.

Leave explicit application wiring and useful worker lifecycle/backlog helpers
in place. Stop extracting when a reader must jump through more layers to follow
an operation. Completion means fewer sources of truth and fewer possible partial
states, with the application's behavior still recognizable.
