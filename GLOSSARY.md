# Family Portal

A private place for a family to record their children's growth, moments, photos, interviews and activities, and to share chosen parts of it.

## Language

### Recording

**Moment**:
Something that happened to a person on a day, told in words and optionally photos. Code, routes and stored data call it a milestone.
_Avoid_: Milestone (in anything people read), event, entry

### Archive

**Family archive**:
A family's own complete copy of its recorded content, which the family can take away and restore into a family here or on another site.
_Avoid_: Backup, export (for the whole thing), dump

**Records**:
A family archive with no media: every person, moment, measurement, activity, interview and decision, as one small file. It is complete without media and can be imported on its own.
_Avoid_: Data only, metadata, JSON export

**Everything archive**:
A family archive that holds the records plus every photo and video file.
_Avoid_: Bundle, full backup, with-photos export

**Media**:
The photo and video files themselves, as opposed to the records that describe them.
_Avoid_: Assets, attachments

**Source-of-truth data**:
Content a person entered or decided (a moment, a photo, a confirmed face, a transcript edit) that cannot be recomputed. A family archive must carry all of it.
_Avoid_: Primary data, user data

**Derived data**:
Content the site can recompute from source-of-truth data, such as search indexes, embeddings and analysis results. A family archive leaves it out, and the site rebuilds it after a restore.
_Avoid_: Cache, metadata

**Snapshot**:
A whole-site database copy taken for disaster recovery. It is not a family archive.
_Avoid_: Backup bundle, export
