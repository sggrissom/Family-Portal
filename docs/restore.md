# Restoring Family Portal from a backup

Follow this top to bottom. It assumes nothing except a shell on a box and access
to the restic repository password.

**What a backup contains:** the database and the photo originals. Nothing else.
Derived photo variants (thumbnails, WebP/AVIF) are deliberately not backed up —
[§5](#5-photo-variants-come-back-missing-and-that-is-fine) covers what that means
in practice. The app's `shared/.env` is **not** in the archive either; see
[§3](#3-put-back-the-secrets-not-in-the-archive).

Verified end to end on 2026-08-09 against the archive `backupctl run family`
produced on 2026-08-08 — see [§8](#8-drill-log).

---

## 0. Before you touch anything

If you are restoring over a live install, take a snapshot of what is there now.
A restore that turns out to be the wrong snapshot is recoverable; a restore that
overwrote the only good copy is not.

```bash
sudo systemctl stop app@family
sudo cp -a /srv/apps/family/shared/data/db.bolt /root/db.bolt.before-restore
```

## 1. Get the archive onto disk

`backupctl fetch` is not implemented yet (`tiny-server-helper/back-plan.md` §6),
so today this is raw restic on the VPS:

```bash
export $(grep -v '^#' /etc/tiny-server-helper/backup.env | xargs -d '\n')

restic snapshots --tag app=family              # pick one; `latest` is the newest
restic restore latest --tag app=family --target /root/restore
```

Without `/etc/tiny-server-helper/backup.env` you need the repository URL and the
repository password by hand (`RESTIC_REPOSITORY`, `RESTIC_PASSWORD`). **There is
no recovery path if the password is lost** — the archives are encrypted with it.

restic restores full absolute paths, so the two things you want land at:

| what | where in `/root/restore` |
| --- | --- |
| database | `var/lib/tiny-server-helper/backup/stage/family/db.bolt` |
| photo originals | `srv/apps/family/shared/static/photos/*_original.*` |

The database sits under the staging path because that is where `backupctl`
wrote the snapshot it fetched from `/internal/snapshot` before handing it to
restic. It is a normal bolt file; the path is cosmetic.

## 2. Put the files where the app expects them

```bash
sudo systemctl stop app@family

sudo install -o apps -g apps -m 644 \
  /root/restore/var/lib/tiny-server-helper/backup/stage/family/db.bolt \
  /srv/apps/family/shared/data/db.bolt

sudo mkdir -p /srv/apps/family/shared/static/photos
sudo cp -a /root/restore/srv/apps/family/shared/static/photos/. \
           /srv/apps/family/shared/static/photos/
sudo chown -R apps:apps /srv/apps/family/shared/static
```

Everything under `shared/` is owned by `apps:apps`. The service runs as that
user and will fail to open a database it cannot write.

## 3. Put back the secrets — not in the archive

`/srv/apps/family/shared/.env` is not backed up, and the service will not start
without it. On a fresh box you must recreate it, mode 600, owned by `apps`:

| variable | what happens without it |
| --- | --- |
| `PORT` | service does not serve where Caddy expects |
| `JWT_SECRET_KEY` | startup uses a generated one; **every existing session and refresh token is invalidated**, so everyone is logged out |
| `BACKUP_TOKEN` | **release builds refuse to start** (`backend/backup.go`), ≥32 chars |
| `SITE_ROOT`, `MAIL_FROM` | links in outbound mail point at the wrong place |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` / `GOOGLE_IOS_CLIENT_ID` | Google sign-in unavailable; re-issue from Google Cloud console |
| `GEMINI_API_KEY` | AI import unavailable; re-issue from the Gemini console |

None of these are stored in the database, so restoring the database does not
bring any of them back.

## 4. Start it and confirm

```bash
sudo systemctl start app@family
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:$PORT/readyz   # 200
```

`/readyz` checks that the database opens and the static directory is readable,
which is exactly the pair this restore just replaced.

Then check the data is actually there, not just that the process is alive:

```bash
# from a checkout, on a COPY — bolt takes an exclusive flock, so this cannot
# read the live file while the service is running
scp vps:/srv/apps/family/shared/data/db.bolt /tmp/check.bolt
go run ./cmd/verifydb -db /tmp/check.bolt
```

`verifydb` prints a row count per bucket and fails if the database has no users
or no people. With a copy of the photo tree alongside it, `-static <dir>` also
checks that every image row has its original on disk — the only photo file the
backup carries, and therefore the only one whose absence is unrecoverable.

Finally, log in and look at a family: people, growth entries, milestones, tags,
chat history, and photos should all be present.

While you are logged in, press **Verify backup path** on `/admin`. It sends the
application the same request `backupctl` sends — loopback, bearer token, whole
body read — and says whether a complete snapshot came back. That is worth doing
after a restore in particular, because `BACKUP_TOKEN` was just retyped by hand
into a fresh `.env`, and a token the running process does not hold fails as a
404 that says nothing about why. The check names both causes of that 404.

It shares the endpoint's budget of ten requests an hour with the nightly backup,
so a result stands for ten minutes before another check will actually run.

## 5. Photo variants come back missing, and that is fine

The archive holds `*_original.*` only. Thumbnails and the WebP/AVIF variants are
regenerable from the originals, so backing them up would multiply the archive
for no recovery value.

**Nothing breaks in the meantime.** `servePhotoHandler` falls back to the
original when a requested variant is not on disk (`backend/photos.go:949-966`),
so every photo still renders immediately after a restore — just at full size,
so a photo grid downloads far more than it should.

To regenerate, sign in as the admin user (user id 1) and use **Reprocess** on
`/admin/photos`. It reads each original and rewrites the full variant set
(`backend/admin.go:192`). It only touches photos with no modern-format variant
on disk, so it is safe to re-run and it skips work already done.

## 6. Migrations re-run, and that is safe

`OpenDB` runs `vbolt.ApplyDBProcess` for each migration, keyed by name in the
database itself (`app.go:104`). Restoring an archive taken *before* a migration
landed means that migration replays on first boot.

This was tested, not assumed: forcing all three migrations to replay against the
restored production data produced byte-identical row counts in every bucket. See
[§8](#8-drill-log).

**Expect the database file to grow on that first boot** — a 2.5 MB restored file
became 20.9 MB after replaying the milestone search index rebuild. That is bolt
pre-allocating in 16 MiB chunks, not data duplication; the next snapshot is
small again. It is the same reason production's `db.bolt` is 19 MB on disk while
its snapshot is 2.5 MB.

## 7. Rehearsing a restore without touching production

A local build resolves `.serve/db.bolt` and `.serve/static/` relative to the
working directory (`cfg/local.go`), so a scratch directory is a complete test
rig — no need to point anything at `/srv`.

```bash
go build -o /tmp/drill/restoredrill ./cmd/restoredrill

mkdir -p /tmp/drill/.serve/static
cp  /root/restore/var/lib/tiny-server-helper/backup/stage/family/db.bolt /tmp/drill/.serve/db.bolt
cp -r /root/restore/srv/apps/family/shared/static/photos /tmp/drill/.serve/static/photos

cd /tmp/drill && ./restoredrill -replay-migrations \
  /readyz /static/photos/<some>_original.jpg
```

`restoredrill` boots the real application against that tree, probes the paths
you name, and exits non-zero if any of them fail. `-replay-migrations` forgets
the recorded migrations first, which is the only way to exercise the
older-archive case — a same-day restore skips every migration because the
records came back with the snapshot.

## 8. Drill log

**2026-08-09** — restored the 2026-08-08 archive (restic snapshot `1ca432a3`,
71.0 MiB, 40 files) into a scratch tree and booted the app against it.

- `db.bolt` 2,547,712 bytes, 27 originals.
- Counts: 3 users, 2 families, 8 people, 8 person_family, 3 family_membership,
  272 growth_data, 861 milestones, 2 tags, 3 photo_tags, 26 images,
  38 photo_person, 24 chat_messages, 1 milestone_photo, 2 milestone_tags.
- 26/26 image rows had their original on disk. One extra original
  (`ea1fcdff…_original.jpeg`) has no image row — an unfinished delete, harmless,
  costs one file of archive space.
- `/readyz` 200; a photo served from `/static` at 156,010 bytes.
- With `-replay-migrations`, all three migrations re-ran and every count was
  unchanged.

Re-run this after any change to a `Pack*` function or to the migration list in
`OpenDB` — those are the two things that can make an old archive unreadable by
new code.
