# Family archive

A family archive is a family's own copy of its source-of-truth data, for
taking it away and restoring it into a family here or on another site. It is
not disaster recovery: whole-site snapshots do that (`docs/restore.md`, ADR
0001). Terms are in `GLOSSARY.md`.

The code lives in `backend/archive*.go`, one file per group of entities, each
with its export and import side by side. `archive.go` classifies every bucket.

## The two exports

| download | what it is | how it's made |
| --- | --- | --- |
| **Records** | one JSON file: every person, moment, measurement, relation, tag, activity, interview, book, place, face decision and transcript, and the photo and video records, with no media files | `ExportData` (the settings page saves `jsonData`) |
| **Everything archive** | a zip: `data.json` (the same records) plus `photos/<original>` and `videos/<date>_<id>_<name>` | `GET /api/export-bundle?mode=everything` |

`?mode=records` returns the records as a zip holding only `data.json`. The old
mode names, `data_only` and `with_photos`, are still accepted.
`GetArchiveExportSizes` estimates both downloads for the settings page.

Only videos that have finished processing (or failed but kept their original)
are exported. A photo is exported once it's ready.

## Format versions

Every export carries `formatVersion` (`archiveFormatVersion`, now **1**).

- A file without `formatVersion` was exported before versioning. It's read as
  version 1. A future birthday in one of those files is taken to be a
  pregnancy's due date, since they don't carry the pregnancy flag, and the
  import says so.
- Version 1 keeps the capitalised `Id`, `FamilyId` and `PersonId` keys on
  people and measurements. Everything else is camelCase, except photos and
  videos, which use snake_case.
- A file with a newer version than the site knows is refused with a message
  saying so. Nothing is written.

The reader (`readArchiveRecords`) converts whatever it's given into the
current shape, so the pipeline only ever sees the current version. A new
version adds a reader case. It doesn't change the pipeline.

## What is carried, derived and excluded

`archiveBuckets` in `backend/archive.go` declares every vbolt bucket.
`TestEveryBucketIsClassifiedForTheArchive` fails on a bucket that isn't
declared, so a new feature can't fall out of the archive without someone
noticing. `TestArchiveSeedFillsEveryArchivedBucket` fails if the round-trip
seed leaves an archived bucket empty.

### Carried

Everything a person entered or decided:

- **people**, with profile photo and crop, pregnancy status, relationship and
  birth details
- **measurements**, with the unit they were entered in
- **relations** and **tags**
- **moments**, with their note, tags and photos
- **photos**, with title, description, dates, original filename, the people
  in them (including whether a person was auto-tagged) and their tags
- **faces**: positions, and the confirmed, corrected and rejected decisions,
  plus the photo's analysis version
- **videos** and their files, with **transcripts** as a person left them
  (edited or removed included)
- **places**
- **activities**, with their seasons, entries and rosters, events,
  performances, results and photos
- **interviews**, their questions and the interview list settings (cadence,
  dismissed questions)
- **books**
- the **family's name**

Every record keeps when it was added (`createdAt`), and so does every link
between records (a moment's tags and photos, a roster member, an event's or
performance's photos). An archive from before a field was carried imports
with that field stamped at import time.

### Derived (rebuilt after an import)

Search indexes, moment and photo embeddings, vision analysis output
(`photo_features`), transcript embeddings (`speech_vectors`), sampled video
frames and face descriptors. These are recomputed from the carried data:

- moment and interview-answer embeddings are queued when the import commits
- imported photos and videos go through the normal processing queue, which
  rebuilds variants, posters, embeddings and features

Analysis that runs afterwards doesn't overwrite decisions. Imported faces keep
their analysis version, so the face sweep skips those photos. Imported
transcripts are marked done or removed, so the video worker doesn't transcribe
them again.

### Excluded

| what | why |
| --- | --- |
| accounts, passwords, tokens, invite codes, push devices, notification preferences | account state. The archive moves to sites where those accounts don't exist |
| memberships (`family_membership`) | account state. Whoever imports is a member of the target family already |
| the family's **video setting** | account state. Only an admin turns video on, so an import mustn't. Restored families get video when an admin turns it on |
| chat | conversation between accounts, not family records |
| family links, person-family links, the family page, recap links | sharing state, set up again on the target site |
| tag suggestions and their bookkeeping | regenerated as photos are analysed |
| appearance sync mutations and tombstones | bookkeeping for offline clients |
| in-flight video uploads and archive imports | not finished media or family data |
| media deletions, log review, mobile version, seed runs | site operations |

Who recorded an entry is carried by name (`recordedBy`). It's reattached when
exactly one member of the importing family has that name. Otherwise the entry
shows no recorder.

## Importing

Both files go through one pipeline (`importFamilyArchive`) with the same
options: include moments, include activities, which people to import (by
person or by family in the file), a preview of who will be matched, and a dry
run that reports the counts without writing anything.

- **A file**, a zip or a Records `.json`, is uploaded.
  `BeginArchiveImport` `{familyId?, filename, size}` checks the size (40 GiB at
  most), the family's storage quota and the disk floor, and returns a signed
  upload URL. A filename ending in `.json` is read as a Records file; anything
  else as a zip. Any zip with a `data.json` works, including the
  `?mode=records` download. The `import` rate rule applies here. The browser
  PUTs the file straight to the media store. The upload URL lasts 12 hours,
  and the PUT gets that long to finish. `CompleteArchiveImport`
  `{importId, options}` checks that the upload arrived and starts a background
  job. `GetArchiveImportStatus` `{importId}` reports `uploading`, `queued`,
  `staging`, `importing`, `placing`, `done` or `failed`, with progress counts
  and, when done, the same report `ImportData` returns.
- **Pasted records:** `ImportData` with the text in `jsonData`. It's an
  ordinary request: 30s to read it, a 1 MiB body limit and the `import` rate
  rule. The import page uses it only for JSON pasted into the page.

The job runs in three steps, so the database write lock is held only briefly:

1. **Stage**, outside any transaction. The target family's photos and videos
   that have no content hash yet are hashed first, so the import recognises
   them before the startup backfill does. Every photo and video the import
   will consider is then unpacked into `import-staging/import-*/` and hashed.
   Before each file the disk floor is checked. A file there's no room for is
   only hashed, so it can still be linked to a copy the family already has,
   and the result says how many weren't imported. A dry run hashes without
   unpacking.
2. **Import**, in one write transaction. Every record is written, or none
   is. A failure here, or while staging, leaves the family unchanged and
   deletes the staging directory. The commit also records the files still to
   be placed, and the job becomes `placing`.
3. **Place**, after the commit. Staged photos are renamed into the static
   directory and queued for processing. Videos are copied into the media store
   and queued; their disk check adds up every video's room across the import.
   A file that can't be moved is logged and left to the stuck-photo or
   stuck-video recovery. The records are already committed.

On shutdown, archive imports are stopped first, at their next step, while the
photo and video workers still run. A job stopped before its commit runs again
on the next start. One stopped, or crashed, while `placing` finishes placing
its files on the next start.

The upload is deleted once its import has committed and placed its files. A
preview, a dry run or a failed import keeps it, so the same upload can be
imported or retried. A kept upload counts toward the family's storage quota
until it's deleted. The media janitor expires an upload left unused for 24
hours, whether it never finished uploading or its job ended without using it
up, and deletes it.

### The staging directory

`import-staging/` sits beside the static directory, so a staged photo moves
into place with a rename rather than a copy:

| | path |
| --- | --- |
| local | `.serve/import-staging/` |
| production | `/srv/apps/family/shared/import-staging/` |

It's created when an import first stages. Each import gets its own
`import-*` directory, removed when the import finishes or fails. At startup,
every directory except those of jobs still placing their files is removed,
so a crash doesn't leave staged files behind. It isn't in the snapshots.

## Merge rules

An import always merges into the chosen family. There is one merge behaviour
(ADR 0001):

- **People** match on the same name, birthday and gender among the family's
  own people (not ones shared in through a link). A matched person is linked,
  and details the family is missing, such as birth details, are filled in from
  the archive. Everyone else is created.
- **Photos and videos** already in the family, by the SHA-256 of the
  original, are linked rather than created again. An Everything archive
  imported after a Records file therefore only adds the media and its links.
- Each kind of record has its own duplicate check, so importing the same
  archive twice changes nothing:

| record | already there when |
| --- | --- |
| measurement | same person, type and day, and the same value within 0.01 in imperial units |
| moment | same person, date and description |
| tag, place | same name, ignoring case |
| relation | the edge exists, in either direction |
| interview | same person and day. Only answers it doesn't have are added |
| activity, season, entry, event | same name, ignoring case, within its parent |
| performance | same event, entry, time, round and opponent |
| book | same subject, preset, title and start date |

- The family name is set only while the family still has the default name
  its creator was given.
- A Records file carries no media. Photos and videos whose files aren't
  present are skipped, along with the links that point at them, and the result
  says how many were skipped. Importing the Everything archive later fills
  them in.

An import never reads or writes another family's data: the target family is
resolved from the importer's own memberships, and people are matched only
among that family's own people.

## Verifying a restore

The round-trip test, `TestFamilyArchiveRoundTrip` in `backend/archive_test.go`,
does this in code. It seeds a family with a row in every archived bucket,
exports the Everything archive, imports it into a fresh family, compares the
two with ids replaced by content keys (a person's name and birthday, a photo's
hash, and so on), then imports again and checks that nothing changed. The
comparison masks nothing. A field the archive leaves out on purpose is listed
in `archiveExcludedFields` or `archiveIgnored`, with the reason.

To check a real restore by hand, as in the 2026-11-04 drill:

1. Export the Everything archive from the source family.
2. Import it into an empty family on the target site (a fresh local site
   works). Check that the result reports no errors, and read the warnings.
3. Export the Everything archive from the restored family.
4. Compare the two `data.json` files with ids ignored. Counts per section
   should match, and so should every person, moment, measurement and photo.
   Only the export date should differ.
5. Import the same archive again and check that the result reports only merged
   people and skipped records.
