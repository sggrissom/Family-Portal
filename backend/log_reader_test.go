package backend

import (
	"family/cfg"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

// writeLog puts a log file in a scratch directory and returns its path.
func writeLog(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "family_record.log")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func scanAll(t *testing.T, contents string) []logEntry {
	t.Helper()
	var entries []logEntry
	if err := scanLogFile(writeLog(t, contents), func(entry logEntry) bool {
		entries = append(entries, entry)
		return true
	}); err != nil {
		t.Fatalf("scanLogFile() error = %v", err)
	}
	return entries
}

// TestScanLogFileParsesEveryShape covers the three shapes a log file actually
// holds. There used to be two copies of this loop with different parse
// strategies; this pins the one they were consolidated onto.
func TestScanLogFileParsesEveryShape(t *testing.T) {
	entries := scanAll(t, `2026/08/22 10:00:00 {"timestamp":"2026-08-22T10:00:00Z","level":"ERROR","category":"AUTH","message":"Login failed","data":{"requestId":"abc123"}}
2026/08/22 10:00:01 200 POST /rpc/SendMessage ⎯⎯⎯ 12759µs [12602µs]
2026/08/22 10:00:02 Photo worker failed to decode something
`)

	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	if entries[0].Level != logLevelError || entries[0].Category != logCategoryAuth {
		t.Errorf("JSON entry parsed as %s/%s, want ERROR/AUTH", entries[0].Level, entries[0].Category)
	}
	if entries[0].Message != "Login failed" {
		t.Errorf("JSON entry message = %q", entries[0].Message)
	}

	timing := entries[1]
	if timing.Duration == nil || *timing.Duration != 12759 {
		t.Errorf("timing entry duration = %v, want 12759", timing.Duration)
	}
	if timing.HTTPMethod != "POST" || timing.HTTPPath != "/rpc/SendMessage" {
		t.Errorf("timing entry endpoint = %s %s", timing.HTTPMethod, timing.HTTPPath)
	}
	if timing.HTTPStatus == nil || *timing.HTTPStatus != 200 {
		t.Errorf("timing entry status = %v, want 200", timing.HTTPStatus)
	}

	// "FAILED" in the text is how a plain log.Printf gets a level at all.
	if entries[2].Level != logLevelError {
		t.Errorf("plain entry level = %s, want ERROR", entries[2].Level)
	}
	if entries[2].Category != logCategoryPhoto {
		t.Errorf("plain entry category = %s, want PHOTO", entries[2].Category)
	}
}

// TestScanLogFileAttachesStackTraces: continuation lines belong to the entry
// above them, and must not be mistaken for entries of their own.
func TestScanLogFileAttachesStackTraces(t *testing.T) {
	entries := scanAll(t, `2026/08/22 10:00:00 panic: something went wrong
goroutine 1 [running]:
	main.main()
		/srv/app/main.go:12 +0x20
2026/08/22 10:00:05 Recovered
`)

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 — continuation lines should not become entries", len(entries))
	}
	if entries[0].StackTrace == "" {
		t.Error("first entry has no stack trace attached")
	}
	if entries[1].StackTrace != "" {
		t.Errorf("second entry picked up a stack trace: %q", entries[1].StackTrace)
	}
}

// TestScanLogFileStopsWhenVisitorDoes proves early exit works, which is what
// lets a caller read a bounded prefix of a large file.
func TestScanLogFileStopsWhenVisitorDoes(t *testing.T) {
	seen := 0
	err := scanLogFile(writeLog(t, `2026/08/22 10:00:00 one
2026/08/22 10:00:01 two
2026/08/22 10:00:02 three
`), func(logEntry) bool {
		seen++
		return seen < 2
	})
	if err != nil {
		t.Fatalf("scanLogFile() error = %v", err)
	}
	if seen != 2 {
		t.Errorf("visited %d entries after asking to stop at 2", seen)
	}
}

// TestPerfAccumulatorSeesTimingLines is the regression the consolidation fixes.
// GetLogStats used a scanner that never tried parseTimingLogLine, so every
// duration in the file was invisible and the latency percentiles it presented
// were computed over an always-empty set.
func TestPerfAccumulatorSeesTimingLines(t *testing.T) {
	perf := newPerfAccumulator()
	err := scanLogFile(writeLog(t, `2026/08/22 10:00:00 200 GET /rpc/GetPeople ⎯⎯⎯ 1000µs
2026/08/22 10:00:01 200 GET /rpc/GetPeople ⎯⎯⎯ 3000µs
2026/08/22 10:00:02 500 POST /rpc/AddPhoto ⎯⎯⎯ 9000µs
2026/08/22 10:00:03 Not a request at all
`), func(entry logEntry) bool {
		perf.add(entry)
		return true
	})
	if err != nil {
		t.Fatalf("scanLogFile() error = %v", err)
	}

	stats := perf.result()
	if stats.TotalRequests != 3 {
		t.Fatalf("TotalRequests = %d, want 3", stats.TotalRequests)
	}
	if stats.MedianResponse != 3000 {
		t.Errorf("MedianResponse = %d, want 3000", stats.MedianResponse)
	}

	get := stats.EndpointStats["GET /rpc/GetPeople"]
	if get.Count != 2 || get.AverageResponse != 2000 || get.MinResponse != 1000 || get.MaxResponse != 3000 {
		t.Errorf("GET /rpc/GetPeople = %+v", get)
	}
	if get.ErrorRate != 0 {
		t.Errorf("GET error rate = %v, want 0", get.ErrorRate)
	}

	post := stats.EndpointStats["POST /rpc/AddPhoto"]
	if post.ErrorRate != 100 {
		t.Errorf("POST error rate = %v, want 100 (its only request was a 500)", post.ErrorRate)
	}

	// Slowest first, so the table reads the way its heading claims.
	if len(stats.SlowestEndpoints) != 2 || stats.SlowestEndpoints[0].Path != "/rpc/AddPhoto" {
		t.Errorf("SlowestEndpoints = %+v", stats.SlowestEndpoints)
	}
}

func TestEntryRingKeepsTheNewest(t *testing.T) {
	ring := newEntryRing(3)

	if got := ring.newestFirst(); len(got) != 0 {
		t.Errorf("empty ring returned %d entries", len(got))
	}

	for _, message := range []string{"a", "b", "c", "d", "e"} {
		ring.add(logEntry{Message: message})
	}

	got := ring.newestFirst()
	want := []string{"e", "d", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Message != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i].Message, want[i])
		}
	}
}

func TestLogFilePathRejectsEscapes(t *testing.T) {
	for _, name := range []string{"", "../etc/passwd", "logs/../config.json", `..\windows`, "sub/dir.log"} {
		if _, err := logFilePath(name); err == nil {
			t.Errorf("logFilePath(%q) was accepted", name)
		}
	}

	if _, err := logFilePath("family_record.log"); err != nil {
		t.Errorf("logFilePath rejected an ordinary name: %v", err)
	}
}

// withLogDir puts files into cfg.LogDir for the procs that read the whole
// directory, and removes exactly what it created. cfg.LogDir is a compile-time
// constant, so there is nowhere else for them to look.
func withLogDir(t *testing.T, files map[string]string) {
	t.Helper()

	createdDir := false
	if _, err := os.Stat(cfg.LogDir); os.IsNotExist(err) {
		if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", cfg.LogDir, err)
		}
		createdDir = true
	}

	for name, contents := range files {
		path := filepath.Join(cfg.LogDir, name)
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("%s already exists; refusing to overwrite it", path)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", path, err)
		}
	}

	t.Cleanup(func() {
		for name := range files {
			_ = os.Remove(filepath.Join(cfg.LogDir, name))
		}
		if createdDir {
			_ = os.Remove(cfg.LogDir)
		}
	})
}

// adminContext returns a read transaction and token for user 1.
func adminContext(t *testing.T, db *vbolt.DB) string {
	t.Helper()
	appDb = db

	var admin User
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		req := CreateAccountRequest{
			Name:            "Admin User",
			Email:           "admin@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		admin = AddUserTx(tx, req, hash)
		admin.Id = 1
		vbolt.Write(tx, UsersBkt, 1, &admin)
		vbolt.TxCommit(tx)
	})

	token, _ := generateAuthJwt(admin, httptest.NewRecorder())
	return token
}

func logTestDB(t *testing.T, name string) *vbolt.DB {
	t.Helper()
	db := vbolt.Open(name)
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() {
		db.Close()
		_ = os.Remove(name)
	})
	return db
}

// TestLookupLogReferenceFindsTheEntryAndItsContext covers the workflow the
// error design already assumed existed: ProcError mints an id, logs the real
// cause against it, and shows the user "Reference: <id>". Until now there was
// no way to look one up without an SSH session.
func TestLookupLogReferenceFindsTheEntryAndItsContext(t *testing.T) {
	withLogDir(t, map[string]string{
		"reftest_family_record.log": `2026/08/22 10:00:00 {"timestamp":"2026-08-22T10:00:00Z","level":"INFO","category":"API","message":"Before one"}
2026/08/22 10:00:01 {"timestamp":"2026-08-22T10:00:01Z","level":"INFO","category":"API","message":"Before two"}
2026/08/22 10:00:02 {"timestamp":"2026-08-22T10:00:02Z","level":"ERROR","category":"SYSTEM","message":"Unexpected procedure error","data":{"requestId":"a1b2c3d4e5f6","error":"bolt: tx closed"}}
2026/08/22 10:00:03 {"timestamp":"2026-08-22T10:00:03Z","level":"INFO","category":"API","message":"After one"}
`,
	})

	token := adminContext(t, logTestDB(t, "test_log_reference.db"))

	t.Run("bare code", func(t *testing.T) {
		var resp LookupLogReferenceResponse
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			var err error
			resp, err = LookupLogReference(&vbeam.Context{Tx: tx, Token: token},
				LookupLogReferenceRequest{Reference: "a1b2c3d4e5f6"})
			if err != nil {
				t.Fatalf("LookupLogReference() error = %v", err)
			}
		})

		if !resp.Found {
			t.Fatalf("reference not found; searched %v", resp.FilesSearched)
		}
		if resp.File != "reftest_family_record.log" {
			t.Errorf("File = %q", resp.File)
		}
		// The cause, not the sentence the user saw — that is the whole point.
		if data, ok := resp.Entry.Data.(map[string]interface{}); !ok || data["error"] != "bolt: tx closed" {
			t.Errorf("Entry.Data = %#v, want the logged cause", resp.Entry.Data)
		}
		// Context in file order, so it reads like the log does.
		if len(resp.Before) != 2 || resp.Before[0].Message != "Before one" || resp.Before[1].Message != "Before two" {
			t.Errorf("Before = %+v, want the two preceding entries in file order", resp.Before)
		}
		if len(resp.After) != 1 || resp.After[0].Message != "After one" {
			t.Errorf("After = %+v", resp.After)
		}
	})

	t.Run("whole pasted sentence", func(t *testing.T) {
		// What someone actually sends you is the message, not the bare code.
		pasted := "Something went wrong on our end. " + ReferencePrefix + "a1b2c3d4e5f6"
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			resp, err := LookupLogReference(&vbeam.Context{Tx: tx, Token: token},
				LookupLogReferenceRequest{Reference: pasted})
			if err != nil {
				t.Fatalf("LookupLogReference() error = %v", err)
			}
			if !resp.Found {
				t.Error("a pasted reference sentence was not resolved to its code")
			}
		})
	})

	t.Run("unknown code says where it looked", func(t *testing.T) {
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			resp, err := LookupLogReference(&vbeam.Context{Tx: tx, Token: token},
				LookupLogReferenceRequest{Reference: "ffffffffffff"})
			if err != nil {
				t.Fatalf("LookupLogReference() error = %v", err)
			}
			if resp.Found {
				t.Error("found a reference that is not in any file")
			}
			if len(resp.FilesSearched) == 0 {
				t.Error("a miss should still say which files were read")
			}
		})
	})

	t.Run("empty reference is refused", func(t *testing.T) {
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			if _, err := LookupLogReference(&vbeam.Context{Tx: tx, Token: token},
				LookupLogReferenceRequest{Reference: "   "}); err == nil {
				t.Error("an empty reference was accepted")
			}
		})
	})

	t.Run("non-admin is refused", func(t *testing.T) {
		regular, _ := generateAuthJwt(User{Id: 2, Email: "regular@example.com"}, httptest.NewRecorder())
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			if _, err := LookupLogReference(&vbeam.Context{Tx: tx, Token: regular},
				LookupLogReferenceRequest{Reference: "a1b2c3d4e5f6"}); err != ErrAdminRequired {
				t.Errorf("Expected ErrAdminRequired, got %v", err)
			}
		})
	})
}

// TestGetLogContentSearchesEveryFile: an empty filename means the whole
// directory, because the code someone mailed you is not necessarily in today's.
func TestGetLogContentSearchesEveryFile(t *testing.T) {
	withLogDir(t, map[string]string{
		"searchtest_a.log": `2026/08/20 10:00:00 {"timestamp":"2026-08-20T10:00:00Z","level":"ERROR","category":"PHOTO","message":"Old failure","data":{"requestId":"aaaaaaaaaaaa"}}
`,
		"searchtest_b.log": `2026/08/22 10:00:00 {"timestamp":"2026-08-22T10:00:00Z","level":"INFO","category":"API","message":"Nothing to see"}
2026/08/22 10:00:01 {"timestamp":"2026-08-22T10:00:01Z","level":"ERROR","category":"PHOTO","message":"New failure","data":{"requestId":"bbbbbbbbbbbb"}}
`,
	})

	token := adminContext(t, logTestDB(t, "test_log_search.db"))

	t.Run("search spans files", func(t *testing.T) {
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			resp, err := GetLogContent(&vbeam.Context{Tx: tx, Token: token},
				GetLogContentRequest{Search: "failure"})
			if err != nil {
				t.Fatalf("GetLogContent() error = %v", err)
			}
			if resp.TotalLines != 2 {
				t.Errorf("TotalLines = %d, want 2 (one match in each file)", resp.TotalLines)
			}
			// Newest first is the default now: you open the log because
			// something is wrong right now.
			if len(resp.Entries) > 0 && resp.Entries[0].Message != "New failure" {
				t.Errorf("first entry = %q, want the most recent", resp.Entries[0].Message)
			}
		})
	})

	t.Run("search matches the structured payload", func(t *testing.T) {
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			resp, err := GetLogContent(&vbeam.Context{Tx: tx, Token: token},
				GetLogContentRequest{Search: "bbbbbbbbbbbb"})
			if err != nil {
				t.Fatalf("GetLogContent() error = %v", err)
			}
			if resp.TotalLines != 1 {
				t.Fatalf("TotalLines = %d, want 1", resp.TotalLines)
			}
			if resp.Entries[0].Message != "New failure" {
				t.Errorf("matched %q", resp.Entries[0].Message)
			}
		})
	})

	t.Run("search combines with the level filter", func(t *testing.T) {
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			resp, err := GetLogContent(&vbeam.Context{Tx: tx, Token: token},
				GetLogContentRequest{Search: "failure", Level: "INFO"})
			if err != nil {
				t.Fatalf("GetLogContent() error = %v", err)
			}
			if resp.TotalLines != 0 {
				t.Errorf("TotalLines = %d, want 0 — no INFO entry says \"failure\"", resp.TotalLines)
			}
		})
	})

	t.Run("a named file is not a directory search", func(t *testing.T) {
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			resp, err := GetLogContent(&vbeam.Context{Tx: tx, Token: token},
				GetLogContentRequest{Filename: "searchtest_a.log", Search: "failure"})
			if err != nil {
				t.Fatalf("GetLogContent() error = %v", err)
			}
			if resp.TotalLines != 1 {
				t.Errorf("TotalLines = %d, want 1", resp.TotalLines)
			}
		})
	})
}

// TestGetLogContentSinceHours: the viewer is almost always opened because
// something is wrong now, so "the last N hours" has to be one field.
func TestGetLogContentSinceHours(t *testing.T) {
	now := time.Now().UTC()
	recent := now.Add(-2 * time.Hour).Format(time.RFC3339)
	old := now.Add(-72 * time.Hour).Format(time.RFC3339)

	withLogDir(t, map[string]string{
		"sincetest.log": `2026/08/22 10:00:00 {"timestamp":"` + old + `","level":"ERROR","category":"SYSTEM","message":"Three days ago"}
2026/08/22 10:00:01 {"timestamp":"` + recent + `","level":"ERROR","category":"SYSTEM","message":"Two hours ago"}
`,
	})

	token := adminContext(t, logTestDB(t, "test_log_since.db"))

	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		resp, err := GetLogContent(&vbeam.Context{Tx: tx, Token: token},
			GetLogContentRequest{Filename: "sincetest.log", Level: "ERROR", SinceHours: 24})
		if err != nil {
			t.Fatalf("GetLogContent() error = %v", err)
		}
		if resp.TotalLines != 1 {
			t.Fatalf("TotalLines = %d, want 1", resp.TotalLines)
		}
		if resp.Entries[0].Message != "Two hours ago" {
			t.Errorf("kept %q", resp.Entries[0].Message)
		}
	})
}
