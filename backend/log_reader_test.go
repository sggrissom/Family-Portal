package backend

import (
	"os"
	"path/filepath"
	"testing"
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
