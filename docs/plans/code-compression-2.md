# Code compression, second pass

Status: proposed.

Based on a review of main at `1a3bb8c` on 2026-11-03. The first pass
(milestone/growth storage, entry rules, atomic saves, membership authority,
photo listing) is done; this pass covers what accumulated since and what it
didn't reach. Recheck the code before each phase.

Same rules as last time: measure progress by how many places someone must
understand to change one behavior. No generic repository, form schema, worker
framework, or CSS utility system. Concrete functions and data only. Preserve
visible behavior except where a bug fix is called out.

## Sequence

| Phase | Scope                        | Intended result                                         | Effort |
| ----- | ---------------------------- | ------------------------------------------------------- | ------ |
| 1     | Photo files on disk          | One file knows how a photo's files are named            | Small  |
| 2     | Edit photo screen            | Built from the same pieces as add photo                 | Small  |
| 3     | Dead code                    | Unreachable Go functions and unused CSS gone            | Small  |
| 4     | Status numbers               | `Image.Status` / `AnalysisStatus` read as words         | Small  |
| 5     | Photo, mail, push workers    | One copy of the attempt log and stop wiring             | Small  |
| 6     | Bundle-imported photos (bug) | Imported photos get variants, dimensions and analysis   | Small  |
| 7     | Legacy date entry            | One date entry path once approximate dates ship in prod | Medium |

1–5 are independent. 7 is blocked on the approximate-dates rollout.

## 1. Photo file naming has one owner

The rule "variants live beside `FilePath` as `<base>_<size>.<ext>`, except
`large` which has no suffix, and the upload is `<base>_original<ext>`" is
rebuilt by hand in:

- `photos.go`: upload handler, `servePhotoHandler`, `deletePhotoFiles`
- `photo_worker.go`: `saveImageVariants`, `writeImageVariants`
- `photo_maintenance.go`: `isPhotoProcessed`, `cleanupOldVariants`,
  `originalPathIn`, `getOriginalPhotoPath`
- `photo_analysis_worker.go:analysisImagePath`,
  `photo_features.go:featureSourceImage`,
  `photo_embeddings.go:embeddingSourceBytes`
- `export.go`, `import_bundle.go`

The size list exists four times with different contents (`deletePhotoFiles`
includes png, `isPhotoProcessed` only avif/webp, `cleanupOldVariants` only jpg).

- [ ] Add `backend/photo_files.go` with `photoOriginalPath(image)`,
      `photoVariantPath(image, size, ext)`, and the size/format lists derived
      from `image_processing.go`'s `ImageSize` values.
- [ ] Route every caller above through it. The three "best source for analysis"
      functions become one `photoSourcePath(image, preferredSizes...)`.
- [ ] Stop writing the original twice: the upload handler writes it, then
      `saveImageVariants` writes it again from `job.FileData`.
- [ ] Move `MkdirAll` and the original-file write out of `WithWriteTx` in
      `uploadPhotoHandler`; today up to 32MB is written to disk while holding
      Bolt's single write lock. Write first, then the transaction; remove the
      file if the transaction rejects.

Acceptance: `grep -n '_original\|_medium\|_xlarge' backend/*.go` hits only
`photo_files.go` and tests; existing photo, export/import and maintenance tests
pass.

## 2. Edit photo uses the add-photo pieces

The first pass unified milestone and growth add/edit screens. Edit photo was
left behind (`frontend/pages/photos/edit-photo.tsx`):

- Its own date UI: four hand-written radio buttons plus date/age inputs, where
  every other entry screen uses `WhenControl`.
- The tag picker is copied three times (`MilestoneFields.tsx:TagPicker`,
  `add-photo.tsx`, `edit-photo.tsx`), and its CSS (`.tag-picker`, `.tag-pill`,
  `.tag-color-dot`) is defined in three to five style files. Because these are
  global classes, whichever file loaded last wins.
- Older form vocabulary (`form-group`, `radio-option`, hardcoded "Tags",
  "Saving...") instead of `entry-field` and `copy.photos`.

- [ ] Move `TagPicker` to `components/TagPicker.tsx` with a `label` prop; use it
      on all three screens; keep one copy of its CSS beside it.
- [ ] Replace edit photo's date radios with `WhenControl`.
- [ ] Use `entry-form-styles` classes and `copy.photos` strings, matching
      add photo.

## 3. Delete dead code

`deadcode` over `./release ./cmd/...` (with `release,faceanalysis,
visionanalysis` tags) reports 29 unreachable functions, including
`RequireFamilyAccessFrom`, `ActingFamilyFor`, `GetResultById`,
`canAccessResult`, `GetVisibleImages`, `RequireAdmin`, `ValidateRefreshToken`,
`UpdateRefreshTokenLastUsed`, `HashStoredRefreshTokens`, `DefaultLinkScopes`,
`GetImageMimeType`, `normalizeQuestion`, `RelationGroup`,
`ChatHub.GetFamilyConnectionCount`, `RespondConflictError`, `LogDebug`, and the
`Stop`/`StopXWorker`/`GetPushQueueLength` wrappers covered in phase 5.

- [ ] Delete them, and any tests that exist only to exercise them.
      `setupTestApp` is a test helper in a non-test file; move it to a
      `_test.go` file.
- [ ] Remove the 33 CSS classes no `.ts`/`.tsx` references (most in
      `admin/logs-styles.ts`, `books/book-styles.ts`, `activities/*`). Check each
      for names built in template strings first.
- [ ] The 72 `log.Printf` calls bypass the structured log the admin log viewer
      reads (183 `LogInfo/Warn/Error` calls). Convert the ones in request and
      worker paths.

## 4. Name the status numbers

`Image.Status` (0 ready, 1 processing, 2 failed) and `AnalysisStatus` (0–3) are
compared as bare integers about 60 times across 24 backend files and 10 times in
the frontend (`image.Status == 2`, `updatePhotoStatus(id, 2)`).

- [ ] Add `const (PhotoReady = 0; PhotoProcessing = 1; PhotoFailed = 2)` and the
      analysis equivalents beside `Image`; replace the literals. Mirror them in
      one frontend module. No storage or wire change.

## 5. One copy of the worker attempt log

Photo, mail and push workers each have an identical `recordAttempt`
(success/failure counters, last error, trimmed `recent` slice), plus
`Start`/`Stop`/`StopAndDrain`/`StopXWorker`/`stopXWorkerAndDrain` wrappers, of
which `Stop` and `StopXWorker` are never called.

- [ ] Add a small concrete `attemptLog` struct (counts, last success/error,
      recent N) embedded in all three; delete the three `recordAttempt` copies.
- [ ] Delete the unused `Stop`/`StopXWorker` paths; `ShutdownWorkers` calls
      `StopAndDrain` directly.
- [ ] Turn `ShutdownWorkers`' eleven copied `if !stop(ctx) { LogWarn; clean =
      false }` blocks into a list of `{name, stop}` pairs walked in order.

Not a worker framework: each worker keeps its own loop, queue and retry rules.

## 6. Bundle-imported photos are never processed (bug)

`import_bundle.go:importPhotos` writes the original to disk and creates the
`Image` with `Status: 0`, but never queues `PhotoProcessingJob`. Imported photos
have no resized variants, `Width`/`Height` of 0, and are never face- or
vision-analyzed until an admin runs "Reprocess all". The serve handler hides
this by falling back to the original, so every thumbnail of an imported photo
sends the full file.

- [ ] Verify against a real bundle import, then create imported photos as
      processing and queue them after commit, like upload does.

## 7. One date entry path

The `APPROXIMATE_DATES` flag is gone: moment entry and photo edits use only
`RecordWhenControl` + `lib/recordWhen.ts` + `RecordTime` resolved by
`applyRecordTime`, and their legacy request fields are removed. iOS
compatibility was not kept (the app isn't released).

- [x] Delete the flag and the legacy branch on web.
- [x] Remove `inputType/milestoneDate/photoDate/ageYears/ageMonths` from
      `AddMilestone`, `UpdateMilestone` and `UpdatePhoto`.
- [ ] Move growth and interview dates onto `RecordWhenControl`, then delete
      `WhenControl` and `lib/when.ts` (keep `localDateString` wherever it ends
      up).
- [ ] Photo upload still takes `inputType` (`"auto"` reads EXIF) through
      `calculatePhotoDate`; `SuggestMilestonePhotos` still takes `inputType`
      fields. Once those take a `RecordTime`, `resolveEntryDate` is left to
      growth and interviews only.

## Looked at and left alone

- `GetAuthUser` + `ErrAuthFailure` at the top of 140 procs: four explicit lines
  each is the vbeam idiom; a wrapper would hide which procs are public.
- `access.go`: already small, every helper has callers (except the two in
  phase 3).
- Repeated three-line CSS bodies across style files (`list-style: none; margin:
  0; padding: 0` ×10, etc.): cheaper to read in place than to route through
  shared utility classes.
- Feature import/export files (activity, interview, book, video): each owns its
  own data; little shared mechanics beyond zip I/O in two places.
- Large files (`photos.go`, `seed.go`, page components): size alone is not the
  problem; phases 1–2 shrink `photos.go` and `edit-photo.tsx` as a side effect.
