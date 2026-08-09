// Package backend tests for the database snapshot endpoint.
// Tests: token authorization, streamed bytes open as a bolt DB, concurrency refusal.
package backend

import (
	"family/cfg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

const testBackupToken = "0123456789abcdef0123456789abcdef"

// snapshotTestDB opens a scratch database holding one known milestone so a
// restored snapshot has something to read back.
func snapshotTestDB(t *testing.T) (*vbolt.DB, Milestone) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "snapshot_source.db")
	db := vbolt.Open(dbPath)
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })

	milestone := Milestone{
		Id:            1,
		FamilyId:      7,
		PersonId:      3,
		Description:   "first steps",
		Category:      "development",
		MilestoneDate: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		CreatedAt:     time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
	}
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		vbolt.Write(tx, MilestoneBkt, milestone.Id, &milestone)
		vbolt.TxCommit(tx)
	})

	return db, milestone
}

func snapshotRequest(token string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, SnapshotPath, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

func TestResolveBackupToken(t *testing.T) {
	t.Run("rejects missing release token", func(t *testing.T) {
		if !cfg.IsRelease {
			t.Skip("release-only assertion")
		}
		t.Setenv("BACKUP_TOKEN", "")

		if _, err := resolveBackupToken(); err == nil {
			t.Fatal("resolveBackupToken() accepted an empty release token")
		}
	})

	t.Run("disables endpoint in local builds", func(t *testing.T) {
		if cfg.IsRelease {
			t.Skip("local-only assertion")
		}
		t.Setenv("BACKUP_TOKEN", "")

		token, err := resolveBackupToken()
		if err != nil {
			t.Fatalf("resolveBackupToken() error = %v", err)
		}
		if token != "" {
			t.Errorf("resolveBackupToken() = %q, want empty token", token)
		}
	})

	t.Run("rejects weak configured token", func(t *testing.T) {
		t.Setenv("BACKUP_TOKEN", strings.Repeat("x", minimumBackupTokenLength-1))

		if _, err := resolveBackupToken(); err == nil {
			t.Fatal("resolveBackupToken() accepted a weak token")
		}
	})

	t.Run("accepts strong configured token", func(t *testing.T) {
		want := strings.Repeat("x", minimumBackupTokenLength)
		t.Setenv("BACKUP_TOKEN", want)

		got, err := resolveBackupToken()
		if err != nil {
			t.Fatalf("resolveBackupToken() error = %v", err)
		}
		if got != want {
			t.Errorf("resolveBackupToken() = %q, want configured token", got)
		}
	})
}

func TestSnapshotHandlerRestoresIntoScratchDatabase(t *testing.T) {
	db, want := snapshotTestDB(t)

	response := httptest.NewRecorder()
	SnapshotHandler(db, testBackupToken).ServeHTTP(response, snapshotRequest(testBackupToken))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", contentType)
	}
	if cacheControl := response.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cacheControl)
	}

	body := response.Body.Bytes()
	if len(body) == 0 {
		t.Fatal("snapshot body is empty")
	}
	declared, err := strconv.Atoi(response.Header().Get("Content-Length"))
	if err != nil {
		t.Fatalf("Content-Length = %q, want an integer", response.Header().Get("Content-Length"))
	}
	if declared != len(body) {
		t.Errorf("Content-Length = %d, want %d streamed bytes", declared, len(body))
	}

	// The streamed bytes must open cleanly as a bolt database and still hold
	// the row written before the snapshot.
	restoredPath := filepath.Join(t.TempDir(), "restored.db")
	if err := os.WriteFile(restoredPath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	restored := vbolt.Open(restoredPath)
	defer restored.Close()

	var got Milestone
	vbolt.WithReadTx(restored, func(tx *vbolt.Tx) {
		vbolt.Read(tx, MilestoneBkt, want.Id, &got)
	})

	if got.Id != want.Id || got.Description != want.Description || got.FamilyId != want.FamilyId {
		t.Errorf("restored milestone = %+v, want %+v", got, want)
	}
}

func TestSnapshotHandlerRejectsUnauthorizedCallers(t *testing.T) {
	db, _ := snapshotTestDB(t)

	tests := []struct {
		name       string
		configured string
		request    *http.Request
	}{
		{"wrong token", testBackupToken, snapshotRequest("wrong-token-wrong-token-wrong-tok")},
		{"no authorization header", testBackupToken, snapshotRequest("")},
		{"unconfigured token", "", snapshotRequest(testBackupToken)},
		{"unconfigured token with empty bearer", "", snapshotRequest("")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			SnapshotHandler(db, test.configured).ServeHTTP(response, test.request)

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
			}
			if response.Body.Len() > 0 && strings.Contains(response.Body.String(), "snapshot") {
				t.Errorf("body %q leaks the existence of the endpoint", response.Body.String())
			}
		})
	}
}

// blockingRecorder pauses inside the first Write so a second request can be
// served while the first snapshot still holds the read transaction.
type blockingRecorder struct {
	*httptest.ResponseRecorder
	writing chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockingRecorder) Write(p []byte) (int, error) {
	b.once.Do(func() {
		close(b.writing)
		<-b.release
	})
	return b.ResponseRecorder.Write(p)
}

func TestSnapshotHandlerRefusesConcurrentSnapshots(t *testing.T) {
	db, _ := snapshotTestDB(t)
	handler := SnapshotHandler(db, testBackupToken)

	blocked := &blockingRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		writing:          make(chan struct{}),
		release:          make(chan struct{}),
	}
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		handler.ServeHTTP(blocked, snapshotRequest(testBackupToken))
	}()

	select {
	case <-blocked.writing:
	case <-time.After(5 * time.Second):
		t.Fatal("first snapshot never started streaming")
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, snapshotRequest(testBackupToken))
	if second.Code != http.StatusConflict {
		t.Errorf("concurrent status = %d, want %d", second.Code, http.StatusConflict)
	}

	close(blocked.release)
	<-firstDone

	if blocked.Code != http.StatusOK {
		t.Fatalf("first snapshot status = %d, want %d", blocked.Code, http.StatusOK)
	}

	// The lock is released once the first snapshot finishes.
	third := httptest.NewRecorder()
	handler.ServeHTTP(third, snapshotRequest(testBackupToken))
	if third.Code != http.StatusOK {
		t.Errorf("status after release = %d, want %d", third.Code, http.StatusOK)
	}
}

func TestSnapshotHandlerWithoutDatabase(t *testing.T) {
	response := httptest.NewRecorder()
	SnapshotHandler(nil, testBackupToken).ServeHTTP(response, snapshotRequest(testBackupToken))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}
