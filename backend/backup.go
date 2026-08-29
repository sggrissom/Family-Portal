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

const SnapshotPath = "/internal/snapshot"

const minimumBackupTokenLength = 32

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

func presentedBackupToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return header[len(prefix):]
}

func backupTokenAuthorized(configured string, r *http.Request) bool {
	if configured == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(configured), []byte(presentedBackupToken(r))) == 1
}

func SnapshotHandler(db *vbolt.DB, token string) http.HandlerFunc {
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

		// WriteTo inside a read transaction is the only consistent snapshot: bolt
		// would otherwise stream a meta page torn mid-commit.
		written, err := tx.WriteTo(w)
		duration := time.Since(start)
		if err != nil {
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
