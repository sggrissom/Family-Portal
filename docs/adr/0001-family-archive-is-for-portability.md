# The family archive is for portability, not disaster recovery

The family archive is a family's own copy of its source-of-truth data, for taking that data away and restoring it into a family here or on another site. Disaster recovery is done with whole-site snapshots (`/internal/snapshot` pushed to restic, see `docs/restore.md`), not with archives. Because of that split, the archive merges into an existing family in one way only: matched people get their missing details filled in, and importing the same archive twice changes nothing. It leaves out derived data, which the site rebuilds after an import. It also excludes account and sharing state: chat (written by accounts that may not exist on the target site), the family page, recap links, family links, memberships and notification preferences.

## Considered Options

- **Make the archive the restore path.** Rejected. Snapshots already cover every bucket and are tested by the restore drill. An archive that also had to restore the whole site would need to carry accounts, tokens and sharing state across sites.
- **Split media across several zips, each carrying all the records, or records plus separate media packs.** Rejected in favour of two exports: Records (small, complete without media) and one Everything archive. The import-size limit is handled on the upload side, not by making the family hold several files.
- **Several merge strategies (`create_all`, `skip_duplicates`).** Removed. `create_all` breaks idempotent re-import, and `skip_duplicates` left a matched person's records with no one to attach to.
