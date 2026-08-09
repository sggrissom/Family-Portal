package backend

import (
	"crypto/subtle"
	"errors"
	"family/cfg"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

// SnapshotPath is the route that streams a consistent copy of the database.
const SnapshotPath = "/internal/snapshot"

const minimumBackupTokenLength = 32

// resolveBackupToken reads the shared secret that guards the snapshot endpoint.
// Release builds must configure one: an endpoint that silently 404s because the
// token was never set is a backup that silently never runs.
func resolveBackupToken() (string, error) {
	token := os.Getenv("BACKUP_TOKEN")
	if token == "" {
		if cfg.IsRelease {
			return "", errors.New("BACKUP_TOKEN must be set in release builds")
		}
		return "", nil
	}

	if len(token) < minimumBackupTokenLength {
		return "", fmt.Errorf("BACKUP_TOKEN must be at least %d characters long", minimumBackupTokenLength)
	}

	return token, nil
}

// presentedBackupToken extracts the bearer credential from a request. Anything
// that is not a well-formed `Authorization: Bearer <token>` header yields the
// empty string, which never matches a configured token.
func presentedBackupToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return header[len(prefix):]
}

// backupTokenAuthorized compares the presented credential in constant time.
// An unconfigured token authorizes nobody, including callers that present an
// empty bearer value.
func backupTokenAuthorized(configured string, r *http.Request) bool {
	if configured == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(configured), []byte(presentedBackupToken(r))) == 1
}

// SnapshotHandler streams a point-in-time copy of the bolt database.
//
// bolt holds an exclusive flock on the database file, so no outside process can
// read it while the server runs, and copying the live file risks capturing a
// torn meta page mid-commit. tx.WriteTo inside a read transaction is the only
// no-downtime way to get a consistent copy, and it has to run in this process.
//
// Requests without a valid token get 404 rather than 401 so the endpoint is not
// discoverable. Caddy proxies the whole public domain to localhost, so every
// request arrives from 127.0.0.1 and a loopback check would buy nothing.
func SnapshotHandler(db *vbolt.DB, token string) http.HandlerFunc {
	// A snapshot holds a read tx for its whole duration, and bolt cannot
	// reclaim freed pages while one is open. Two overlapping snapshots are how
	// the file doubles, so the second caller is turned away rather than queued.
	var inFlight sync.Mutex

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")

		if !backupTokenAuthorized(token, r) {
			http.NotFound(w, r)
			return
		}

		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}

		if !inFlight.TryLock() {
			http.Error(w, "snapshot already in progress", http.StatusConflict)
			return
		}
		defer inFlight.Unlock()

		tx, err := db.Begin(false)
		if err != nil {
			LogErrorSimple(LogCategorySystem, "Database snapshot could not open a read transaction", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "snapshot unavailable", http.StatusServiceUnavailable)
			return
		}
		defer func() { _ = tx.Rollback() }()

		expected := tx.Size()
		start := time.Now()
		LogInfo(LogCategorySystem, "Database snapshot starting", map[string]interface{}{
			"expectedBytes": expected,
		})

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.FormatInt(expected, 10))
		w.Header().Set("Content-Disposition", `attachment; filename="db.bolt"`)

		written, err := tx.WriteTo(w)
		duration := time.Since(start)
		if err != nil {
			// Headers are already out, so the only way to signal failure is to
			// stop short of the declared Content-Length; net/http then drops
			// the connection and the client sees a truncated response.
			LogErrorSimple(LogCategorySystem, "Database snapshot failed", map[string]interface{}{
				"error":        err.Error(),
				"writtenBytes": written,
				"durationMs":   duration.Milliseconds(),
			})
			return
		}

		LogInfo(LogCategorySystem, "Database snapshot complete", map[string]interface{}{
			"bytes":      written,
			"durationMs": duration.Milliseconds(),
		})
	}
}

// RegisterBackupHandlers wires the snapshot endpoint. Release builds abort
// startup when BACKUP_TOKEN is unusable, the same treatment JWT_SECRET_KEY gets.
func RegisterBackupHandlers(app *vbeam.Application) {
	token, err := resolveBackupToken()
	if err != nil {
		log.Fatal(err)
	}
	if token == "" {
		log.Printf("BACKUP_TOKEN not set; %s is disabled.", SnapshotPath)
	}

	app.HandleFunc("GET "+SnapshotPath, SnapshotHandler(app.DB, token))
}
