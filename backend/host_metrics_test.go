package backend

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// snapshotJSON is a metrics-server response in its real shape — Rust field
// names, serialized as-is. Kept literal rather than built from the Go structs,
// so a rename on either side shows up as a test failure.
const snapshotJSON = `{
  "collected_at": "2026-08-22T10:00:00Z",
  "system": {
    "load_avg": {"one": 0.4, "five": 0.3, "fifteen": 0.2},
    "memory": {"total_kb": 2000000, "available_kb": 800000, "used_kb": 1200000, "used_pct": 60.0},
    "cpu": {"user_pct": 12.0, "system_pct": 3.0, "idle_pct": 84.0, "iowait_pct": 1.0},
    "disk": {"total_kb": 20000000, "used_kb": 18000000, "free_kb": 2000000, "used_pct": 90.0}
  },
  "apps": [
    {"name": "chess", "disk_kb": 1000, "traffic": {"window_seconds": 900, "requests_total": 5, "requests_per_min": 0.3, "error_4xx": 0, "error_5xx": 0, "error_pct": 0.0}},
    {"name": "family", "disk_kb": 4200000, "traffic": {"window_seconds": 900, "requests_total": 120, "requests_per_min": 8.0, "error_4xx": 3, "error_5xx": 2, "error_pct": 4.17}}
  ]
}`

// resetHostMetricsCache clears the cache so each test fetches for itself.
func resetHostMetricsCache(t *testing.T) {
	t.Helper()
	hostMetrics.mu.Lock()
	hostMetrics.fetchedAt = time.Time{}
	hostMetrics.cached = HostMetricsResponse{}
	hostMetrics.mu.Unlock()
	t.Cleanup(func() {
		hostMetrics.mu.Lock()
		hostMetrics.fetchedAt = time.Time{}
		hostMetrics.cached = HostMetricsResponse{}
		hostMetrics.mu.Unlock()
	})
}

func TestFetchHostMetricsReadsTheRealShape(t *testing.T) {
	var requests int
	var sawAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		sawAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(snapshotJSON))
	}))
	defer server.Close()

	resetHostMetricsCache(t)
	t.Setenv("METRICS_URL", server.URL+"/metrics")
	t.Setenv("METRICS_API_KEY", "a-key")

	resp := fetchHostMetrics()

	if !resp.Configured || !resp.Available {
		t.Fatalf("Configured=%v Available=%v Error=%q", resp.Configured, resp.Available, resp.Error)
	}
	if sawAuth != "Bearer a-key" {
		t.Errorf("Authorization = %q, want the bearer key", sawAuth)
	}

	// The app's own slice, not the first one in the list.
	if resp.App.Name != metricsAppName {
		t.Errorf("App.Name = %q, want %q", resp.App.Name, metricsAppName)
	}
	if resp.App.Traffic.Error5xx != 2 || resp.App.Traffic.Error4xx != 3 {
		t.Errorf("traffic errors = %d/%d, want 3/2", resp.App.Traffic.Error4xx, resp.App.Traffic.Error5xx)
	}
	if resp.System.Disk.UsedPct != 90.0 {
		t.Errorf("Disk.UsedPct = %v, want 90", resp.System.Disk.UsedPct)
	}
	if resp.System.CPU.IowaitPct != 1.0 {
		t.Errorf("Cpu.IowaitPct = %v, want 1 — a box slow on disk looks idle by every other measure", resp.System.CPU.IowaitPct)
	}

	// Cached: the collector only updates every 30s, so polling faster gains
	// nothing but load on both processes.
	fetchHostMetrics()
	if requests != 1 {
		t.Errorf("made %d requests for two calls; the second should have been cached", requests)
	}
}

// TestFetchHostMetricsDegradesQuietly — a metrics service that is down must not
// take the admin panel with it.
func TestFetchHostMetricsDegradesQuietly(t *testing.T) {
	t.Run("unauthorized names the key", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		resetHostMetricsCache(t)
		t.Setenv("METRICS_URL", server.URL+"/metrics")
		t.Setenv("METRICS_API_KEY", "wrong")

		resp := fetchHostMetrics()
		if resp.Available {
			t.Error("Available = true after a 401")
		}
		// A wrong key and a dead service need different fixes, so they get
		// different messages.
		if resp.Error != "metrics service rejected METRICS_API_KEY" {
			t.Errorf("Error = %q", resp.Error)
		}
	})

	t.Run("unreachable is reported, not returned", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		url := server.URL + "/metrics"
		server.Close() // nothing is listening now

		resetHostMetricsCache(t)
		t.Setenv("METRICS_URL", url)
		t.Setenv("METRICS_API_KEY", "a-key")

		resp := fetchHostMetrics()
		if resp.Available || resp.Error == "" {
			t.Errorf("Available=%v Error=%q, want an unavailable card with a reason", resp.Available, resp.Error)
		}
		if !resp.Configured {
			t.Error("Configured = false; the deployment is configured, the service is just down")
		}
	})

	t.Run("unconfigured hides the card", func(t *testing.T) {
		resetHostMetricsCache(t)
		t.Setenv("METRICS_URL", "")
		t.Setenv("METRICS_API_KEY", "")

		resp := fetchHostMetrics()
		if resp.Configured || resp.Available || resp.Error != "" {
			t.Errorf("unconfigured metrics reported %+v, want an empty response", resp)
		}
	})
}
