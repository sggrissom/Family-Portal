package backend

import (
	"context"
	"errors"
	"family/cfg"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// MediaDeletion is a stored object the app no longer references. Recording it
// in the same transaction that drops the reference means a failed or
// interrupted delete is retried instead of leaving the object stored forever.
type MediaDeletion struct {
	Id        int
	Key       string
	Attempts  int
	LastError string
	CreatedAt time.Time
}

func PackMediaDeletion(self *MediaDeletion, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.String(&self.Key, buf)
	vpack.Int(&self.Attempts, buf)
	vpack.String(&self.LastError, buf)
	vpack.Time(&self.CreatedAt, buf)
}

var MediaDeletionBkt = vbolt.Bucket(&cfg.Info, "media_deletions", vpack.FInt, PackMediaDeletion)

func queueMediaDeletionTx(tx *vbolt.Tx, key string) {
	if key == "" {
		return
	}
	d := MediaDeletion{Id: vbolt.NextIntId(tx, MediaDeletionBkt), Key: key, CreatedAt: time.Now()}
	vbolt.Write(tx, MediaDeletionBkt, d.Id, &d)
}

func queueVideoObjectDeletionsTx(tx *vbolt.Tx, image Image) {
	if !image.IsVideo() {
		return
	}
	queueMediaDeletionTx(tx, image.OriginalKey)
	queueMediaDeletionTx(tx, image.PlaybackKey)
}

const (
	mediaJanitorInterval = 10 * time.Minute

	// Completed upload sessions are kept a while after their URL dies so a
	// late PUT to the source key is found and removed.
	completedUploadRetention = 24 * time.Hour
)

type mediaJanitor struct {
	workerLifecycle
	db   *vbolt.DB
	kick chan struct{}
}

var globalMediaJanitor *mediaJanitor

func InitializeVideoUploads(db *vbolt.DB) {
	if s, ok := activeMediaStore().(*localMediaStore); ok {
		s.gate = videoUploadGate(db)
	}
	if globalMediaJanitor != nil || activeMediaStore() == nil {
		return
	}
	globalMediaJanitor = &mediaJanitor{db: db, kick: make(chan struct{}, 1)}
	quit, done, ok := globalMediaJanitor.start()
	if ok {
		go globalMediaJanitor.run(quit, done)
	}
}

func kickMediaJanitor() {
	if j := globalMediaJanitor; j != nil {
		select {
		case j.kick <- struct{}{}:
		default:
		}
	}
}

func StopMediaJanitor(ctx context.Context) bool {
	if globalMediaJanitor == nil {
		return true
	}
	return globalMediaJanitor.stopAndWait(ctx, false)
}

func (j *mediaJanitor) run(quit <-chan struct{}, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(mediaJanitorInterval)
	defer ticker.Stop()

	sweepMedia(j.db, activeMediaStore(), time.Now())
	for {
		select {
		case <-quit:
			return
		case <-ticker.C:
		case <-j.kick:
		}
		sweepMedia(j.db, activeMediaStore(), time.Now())
	}
}

type mediaSweepResult struct {
	Expired       int
	SessionsFreed int
	LateRemoved   int
	Deleted       int
	DeleteFailed  int
}

func sweepMedia(db *vbolt.DB, store MediaStore, now time.Time) (result mediaSweepResult) {
	if store == nil {
		return
	}
	ctx := context.Background()

	var uploads []VideoUpload
	var deletions []MediaDeletion
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, VideoUploadBkt, func(_ int, u VideoUpload) bool {
			uploads = append(uploads, u)
			return true
		})
		vbolt.IterateAll(tx, MediaDeletionBkt, func(_ int, d MediaDeletion) bool {
			deletions = append(deletions, d)
			return true
		})
	})

	closed := now.Add(-videoCompletionGrace)
	for _, u := range uploads {
		switch u.State {
		case VideoUploadPending:
			if u.ExpiresAt.Before(closed) {
				if expireVideoUpload(db, u.Id) {
					result.Expired++
					u.State = VideoUploadAbandoned
				}
			}
		}

		switch u.State {
		case VideoUploadAbandoned:
			if err := store.Delete(ctx, u.Key); err != nil {
				logMediaDeleteFailure(u.Key, err)
				result.DeleteFailed++
				continue
			}
			forgetVideoUpload(db, u.Id)
			result.SessionsFreed++
		case VideoUploadCompleted:
			if !u.ExpiresAt.Before(closed) || stillProcessingFrom(db, u) {
				continue
			}
			if _, err := store.Stat(ctx, u.Key); err == nil {
				result.LateRemoved++
			}
			if err := store.Delete(ctx, u.Key); err != nil {
				logMediaDeleteFailure(u.Key, err)
				result.DeleteFailed++
				continue
			}
			if u.ExpiresAt.Before(now.Add(-completedUploadRetention)) {
				forgetVideoUpload(db, u.Id)
				result.SessionsFreed++
			}
		}
	}

	for _, d := range deletions {
		err := store.Delete(ctx, d.Key)
		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			if err == nil {
				vbolt.Delete(tx, MediaDeletionBkt, d.Id)
			} else {
				d.Attempts++
				d.LastError = err.Error()
				vbolt.Write(tx, MediaDeletionBkt, d.Id, &d)
			}
			vbolt.TxCommit(tx)
		})
		if err == nil {
			result.Deleted++
		} else {
			logMediaDeleteFailure(d.Key, err)
			result.DeleteFailed++
		}
	}

	if result != (mediaSweepResult{}) {
		LogInfo(LogCategoryWorker, "Media sweep finished", map[string]interface{}{
			"expired": result.Expired, "sessionsFreed": result.SessionsFreed,
			"lateRemoved": result.LateRemoved, "deleted": result.Deleted, "deleteFailed": result.DeleteFailed,
		})
	}
	return
}

func expireVideoUpload(db *vbolt.DB, id int) (expired bool) {
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		u := getVideoUpload(tx, id)
		if u.Id == 0 || u.State != VideoUploadPending {
			return
		}
		u.State = VideoUploadAbandoned
		writeVideoUpload(tx, &u)
		vbolt.TxCommit(tx)
		expired = true
	})
	return
}

func forgetVideoUpload(db *vbolt.DB, id int) {
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		deleteVideoUploadTx(tx, id)
		vbolt.TxCommit(tx)
	})
}

// The worker moves an accepted upload's source to its final key; until then
// the source is the only copy and must stay.
func stillProcessingFrom(db *vbolt.DB, u VideoUpload) (busy bool) {
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		image := GetImageById(tx, u.ImageId)
		busy = image.Id != 0 && image.OriginalKey == u.Key
	})
	return
}

func logMediaDeleteFailure(key string, err error) {
	if errors.Is(err, ErrMediaNotFound) {
		return
	}
	LogErrorSimple(LogCategoryWorker, "Could not delete a stored media object; will retry", map[string]interface{}{
		"key": key, "error": err.Error(),
	})
}
