package backend

import (
	"net/http/httptest"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

// TestRequeueStuckPhotos: rows stranded in Processing are currently
// unrecoverable without editing the database, and §1.3's old reprocess
// implementation is how they got created.
func TestRequeueStuckPhotos(t *testing.T) {
	if globalPhotoWorker != nil {
		globalPhotoWorker.Stop()
	}
	globalPhotoWorker = nil

	db := logTestDB(t, "test_requeue_stuck.db")
	token := adminContext(t, db)
	now := time.Now()

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		images := []Image{
			// Stranded: claiming to be in progress since three hours ago.
			{Id: 1, FamilyId: 1, Status: 1, CreatedAt: now.Add(-3 * time.Hour), FilePath: "photos/stuck.jpg", MimeType: "image/jpeg"},
			// Genuinely in progress right now.
			{Id: 2, FamilyId: 1, Status: 1, CreatedAt: now, FilePath: "photos/live.jpg", MimeType: "image/jpeg"},
			// Failed and done are not this button's business.
			{Id: 3, FamilyId: 1, Status: 2, CreatedAt: now.Add(-3 * time.Hour), FilePath: "photos/failed.jpg"},
			{Id: 4, FamilyId: 1, Status: 0, CreatedAt: now.Add(-3 * time.Hour), FilePath: "photos/done.jpg"},
		}
		for _, image := range images {
			vbolt.Write(tx, ImagesBkt, image.Id, &image)
		}
		vbolt.TxCommit(tx)
	})

	t.Run("refuses when the worker is not running", func(t *testing.T) {
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			if _, err := RequeueStuckPhotos(&vbeam.Context{Tx: tx, Token: token}, RequeueStuckPhotosRequest{}); err != ErrPhotoWorkerUnavailable {
				t.Errorf("Expected ErrPhotoWorkerUnavailable, got %v", err)
			}
		})
	})

	t.Run("queues only the stranded rows", func(t *testing.T) {
		InitializePhotoWorker(10, db)
		defer func() {
			// The backlog is fed from a goroutine. Let it finish before
			// tearing the worker down, or the teardown races the feeder.
			waitForBacklogFeeders()
			globalPhotoWorker.Stop()
			globalPhotoWorker = nil
		}()

		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			resp, err := RequeueStuckPhotos(&vbeam.Context{Tx: tx, Token: token}, RequeueStuckPhotosRequest{})
			if err != nil {
				t.Fatalf("RequeueStuckPhotos() error = %v", err)
			}
			if resp.Queued != 1 {
				t.Errorf("Queued = %d, want 1 — only the row stranded for over an hour", resp.Queued)
			}
		})
	})

	t.Run("non-admin is refused", func(t *testing.T) {
		regular, _ := generateAuthJwt(User{Id: 2, Email: "regular@example.com"}, httptest.NewRecorder())
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			if _, err := RequeueStuckPhotos(&vbeam.Context{Tx: tx, Token: regular}, RequeueStuckPhotosRequest{}); err != ErrAdminRequired {
				t.Errorf("Expected ErrAdminRequired, got %v", err)
			}
		})
	})
}

// TestRevokeUserSessions: one button for a lost phone, or a session that
// outlived a JWT secret change.
func TestRevokeUserSessions(t *testing.T) {
	db := logTestDB(t, "test_revoke_sessions.db")
	token := adminContext(t, db)

	// Two logins for the target, and one for a bystander who must be untouched.
	var targetId, bystanderId int
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		target := User{Id: 2, Name: "Target", Email: "target@example.com", Creation: time.Now(), LastLogin: time.Now()}
		vbolt.Write(tx, UsersBkt, target.Id, &target)
		targetId = target.Id

		bystander := User{Id: 3, Name: "Bystander", Email: "bystander@example.com", Creation: time.Now(), LastLogin: time.Now()}
		vbolt.Write(tx, UsersBkt, bystander.Id, &bystander)
		bystanderId = bystander.Id

		for i := 0; i < 2; i++ {
			if _, _, err := CreateRefreshToken(tx, targetId, time.Hour); err != nil {
				t.Fatalf("CreateRefreshToken() error = %v", err)
			}
		}
		if _, _, err := CreateRefreshToken(tx, bystanderId, time.Hour); err != nil {
			t.Fatalf("CreateRefreshToken() error = %v", err)
		}
		vbolt.TxCommit(tx)
	})

	t.Run("revokes every session for that user only", func(t *testing.T) {
		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			resp, err := RevokeUserSessions(&vbeam.Context{Tx: tx, Token: token}, RevokeUserSessionsRequest{UserId: targetId})
			if err != nil {
				t.Fatalf("RevokeUserSessions() error = %v", err)
			}
			if resp.Revoked != 2 {
				t.Errorf("Revoked = %d, want 2", resp.Revoked)
			}
		})

		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			var remaining []int
			vbolt.ReadTermTargets(tx, RefreshTokenByUserIndex, targetId, &remaining, vbolt.Window{})
			if len(remaining) != 0 {
				t.Errorf("target still has %d tokens", len(remaining))
			}

			var bystanderTokens []int
			vbolt.ReadTermTargets(tx, RefreshTokenByUserIndex, bystanderId, &bystanderTokens, vbolt.Window{})
			if len(bystanderTokens) != 1 {
				t.Errorf("bystander has %d tokens, want 1 — revoking one user must not touch another", len(bystanderTokens))
			}
		})
	})

	t.Run("unknown user is refused", func(t *testing.T) {
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			if _, err := RevokeUserSessions(&vbeam.Context{Tx: tx, Token: token}, RevokeUserSessionsRequest{UserId: 9999}); err != ErrUserNotFound {
				t.Errorf("Expected ErrUserNotFound, got %v", err)
			}
			if _, err := RevokeUserSessions(&vbeam.Context{Tx: tx, Token: token}, RevokeUserSessionsRequest{UserId: 0}); err != ErrUserNotFound {
				t.Errorf("Expected ErrUserNotFound for id 0, got %v", err)
			}
		})
	})

	t.Run("non-admin is refused", func(t *testing.T) {
		regular, _ := generateAuthJwt(User{Id: 3, Email: "bystander@example.com"}, httptest.NewRecorder())
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			if _, err := RevokeUserSessions(&vbeam.Context{Tx: tx, Token: regular}, RevokeUserSessionsRequest{UserId: targetId}); err != ErrAdminRequired {
				t.Errorf("Expected ErrAdminRequired, got %v", err)
			}
		})
	})
}

// TestPhotoWorkerRecordsAttempts — photo processing failure is the most common
// real problem this site has and was the least visible: the worker's only
// observable state was a queue length and a boolean.
func TestPhotoWorkerRecordsAttempts(t *testing.T) {
	if globalPhotoWorker != nil {
		globalPhotoWorker.Stop()
	}
	globalPhotoWorker = nil

	db := logTestDB(t, "test_worker_attempts.db")
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		image := Image{Id: 1, FamilyId: 1, Status: 0, FilePath: "photos/no-such-file.jpg", MimeType: "image/jpeg"}
		vbolt.Write(tx, ImagesBkt, image.Id, &image)
		vbolt.TxCommit(tx)
	})

	InitializePhotoWorker(5, db)
	defer func() {
		globalPhotoWorker.Stop()
		globalPhotoWorker = nil
	}()

	// A reprocess job whose original is not on disk: fails at a known step.
	globalPhotoWorker.processPhotoJob(PhotoProcessingJob{
		ImageId:   1,
		FamilyId:  1,
		FilePath:  "photos/no-such-file.jpg",
		MimeType:  "image/jpeg",
		Reprocess: true,
	})

	stats := GetProcessingStats()
	if stats.Failed != 1 {
		t.Errorf("Failed = %d, want 1", stats.Failed)
	}
	if stats.LastError == "" {
		t.Error("LastError is empty after a failure")
	}
	if len(stats.RecentAttempts) != 1 {
		t.Fatalf("RecentAttempts has %d entries, want 1", len(stats.RecentAttempts))
	}

	attempt := stats.RecentAttempts[0]
	if attempt.Success || attempt.ImageId != 1 || !attempt.Reprocess {
		t.Errorf("attempt = %+v", attempt)
	}
	if attempt.Reason == "" {
		t.Error("attempt has no reason; the whole point is saying why")
	}
}
