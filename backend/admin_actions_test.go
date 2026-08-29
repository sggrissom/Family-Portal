package backend

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

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
			{Id: 1, FamilyId: 1, Status: 1, CreatedAt: now.Add(-3 * time.Hour), FilePath: "photos/stuck.jpg", MimeType: "image/jpeg"},
			{Id: 2, FamilyId: 1, Status: 1, CreatedAt: now, FilePath: "photos/live.jpg", MimeType: "image/jpeg"},
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

func TestRevokeUserSessions(t *testing.T) {
	db := logTestDB(t, "test_revoke_sessions.db")
	token := adminContext(t, db)

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

func TestVerifyBackupPath(t *testing.T) {
	const token = "a-token-that-is-long-enough-to-be-real"

	t.Run("a complete snapshot passes", func(t *testing.T) {
		body := []byte("bolt file contents")
		var sawAuth string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sawAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			_, _ = w.Write(body)
		}))
		defer server.Close()

		result := runBackupVerification(server.URL, token)

		if !result.OK {
			t.Fatalf("OK = false, want true: %s", result.Detail)
		}
		if sawAuth != "Bearer "+token {
			t.Errorf("Authorization = %q, want the bearer token backupctl sends", sawAuth)
		}
		if result.ReceivedBytes != int64(len(body)) || result.DeclaredBytes != int64(len(body)) {
			t.Errorf("declared %d, received %d, want %d of each", result.DeclaredBytes, result.ReceivedBytes, len(body))
		}
	})

	t.Run("a truncated stream fails", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "4096")
			_, _ = w.Write([]byte("only this much"))
		}))
		defer server.Close()

		result := runBackupVerification(server.URL, token)

		if result.OK {
			t.Fatal("OK = true for a stream that stopped short of its declared length")
		}
		if result.ReceivedBytes >= result.DeclaredBytes {
			t.Errorf("received %d of a declared %d, want fewer", result.ReceivedBytes, result.DeclaredBytes)
		}
	})

	t.Run("a snapshot with no declared length fails", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("first"))
			w.(http.Flusher).Flush()
			_, _ = w.Write([]byte("second"))
		}))
		defer server.Close()

		result := runBackupVerification(server.URL, token)

		if result.OK {
			t.Fatal("OK = true for a response that declared no length")
		}
		if !strings.Contains(result.Detail, "Content-Length") {
			t.Errorf("Detail = %q, want it to name the missing header", result.Detail)
		}
	})

	t.Run("404 names both of its causes", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer server.Close()

		result := runBackupVerification(server.URL, token)

		if result.OK || result.Status != http.StatusNotFound {
			t.Fatalf("OK = %v, Status = %d, want false and 404", result.OK, result.Status)
		}
		if !strings.Contains(result.Detail, "BACKUP_TOKEN") || !strings.Contains(result.Detail, "rate-limited") {
			t.Errorf("Detail = %q, want the stale-token and spent-budget causes both named", result.Detail)
		}
	})

	t.Run("409 is inconclusive rather than a failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "snapshot already in progress", http.StatusConflict)
		}))
		defer server.Close()

		result := runBackupVerification(server.URL, token)

		if result.OK || result.Status != http.StatusConflict {
			t.Fatalf("OK = %v, Status = %d, want false and 409", result.OK, result.Status)
		}
		if !strings.Contains(result.Detail, "proves nothing") {
			t.Errorf("Detail = %q, want it to say the check was inconclusive", result.Detail)
		}
	})

	t.Run("an unset token is reported without a request", func(t *testing.T) {
		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
		}))
		defer server.Close()

		result := runBackupVerification(server.URL, "")

		if result.OK {
			t.Fatal("OK = true with no token configured")
		}
		if requests != 0 {
			t.Errorf("sent %d requests with no token to send", requests)
		}
	})

	t.Run("a dead endpoint is a result, not an error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		url := server.URL
		server.Close()

		result := runBackupVerification(url, token)

		if result.OK || result.Detail == "" {
			t.Fatalf("OK = %v, Detail = %q, want false and a sentence", result.OK, result.Detail)
		}
	})
}

func TestVerifyBackupPathCooldown(t *testing.T) {
	db := logTestDB(t, "test_verify_backup.db")
	adminToken := adminContext(t, db)

	seeded := VerifyBackupPathResponse{
		OK:        true,
		Detail:    "a complete snapshot came back over the path the nightly backup uses.",
		Status:    http.StatusOK,
		CheckedAt: time.Now(),
	}
	backupVerify.mu.Lock()
	backupVerify.last = seeded
	backupVerify.mu.Unlock()
	t.Cleanup(func() {
		backupVerify.mu.Lock()
		backupVerify.last = VerifyBackupPathResponse{}
		backupVerify.mu.Unlock()
	})

	t.Run("replays the last result within the cooldown", func(t *testing.T) {
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			resp, err := VerifyBackupPath(&vbeam.Context{Tx: tx, Token: adminToken}, VerifyBackupPathRequest{})
			if err != nil {
				t.Fatalf("VerifyBackupPath() error = %v", err)
			}
			if !resp.Cached {
				t.Error("Cached = false; a replayed result that does not say so is the most misleading thing on the page")
			}
			if !resp.OK || resp.CheckedAt != seeded.CheckedAt {
				t.Errorf("resp = %+v, want the seeded result replayed", resp)
			}
		})
	})

	t.Run("non-admin is refused", func(t *testing.T) {
		regular, _ := generateAuthJwt(User{Id: 2, Email: "regular@example.com"}, httptest.NewRecorder())
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			if _, err := VerifyBackupPath(&vbeam.Context{Tx: tx, Token: regular}, VerifyBackupPathRequest{}); err != ErrAdminRequired {
				t.Errorf("Expected ErrAdminRequired, got %v", err)
			}
		})
	})
}

func TestVerifyBackupPathAgainstTheRealEndpoint(t *testing.T) {
	const token = "a-backup-token-of-at-least-32-characters"

	db := logTestDB(t, "test_snapshot_endpoint.db")

	t.Setenv("RATE_LIMIT_DISABLED", "")

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+SnapshotPath, SnapshotHandler(db, token))
	server := httptest.NewServer(NewRequestIDWrapper(NewRateLimitWrapper(NewRequestTimeoutWrapper(mux))))
	defer server.Close()

	t.Run("a real snapshot passes", func(t *testing.T) {
		result := runBackupVerification(server.URL, token)

		if !result.OK {
			t.Fatalf("OK = false against the real endpoint: %s", result.Detail)
		}
		if result.ReceivedBytes == 0 || result.ReceivedBytes != result.DeclaredBytes {
			t.Errorf("received %d of a declared %d bytes", result.ReceivedBytes, result.DeclaredBytes)
		}
	})

	t.Run("a token the endpoint does not hold is refused", func(t *testing.T) {
		result := runBackupVerification(server.URL, "a-different-token-of-at-least-32-chars")

		if result.OK || result.Status != http.StatusNotFound {
			t.Fatalf("OK = %v, Status = %d, want false and 404", result.OK, result.Status)
		}
	})

	t.Run("a spent budget looks exactly like a bad token", func(t *testing.T) {
		var last VerifyBackupPathResponse
		for i := 0; i < 12; i++ {
			last = runBackupVerification(server.URL, token)
			if !last.OK {
				break
			}
		}

		if last.OK {
			t.Fatal("twelve snapshots in a row all succeeded; the snapshot rate limit is not being applied")
		}
		if last.Status != http.StatusNotFound {
			t.Errorf("Status = %d, want 404 — a rate-limited caller is turned away with the same answer as an unauthorized one", last.Status)
		}
	})
}
