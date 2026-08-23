package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"go.hasen.dev/vbeam"
)

// The admin panel stops at the process boundary. It knows its own uptime, its
// own queues, and its own log file. It knows nothing about the disk it is
// filling or the traffic Caddy is seeing — and the second of those is the
// interesting one, because it is measured at the proxy, independently of the
// app's own logging, and answers "is the site actually erroring for people",
// a question the panel could not ask at all.
//
// metrics-server already collects all of it on a 30-second loop. This consumes
// it. The types below mirror its JSON exactly (Rust field names, serialized
// as-is), so the two can be compared side by side when either changes.

// metricsAppName is the app whose slice of the snapshot this panel cares about.
// metrics-server reports every app under /srv/apps.
const metricsAppName = "family"

// metricsCacheTTL matches metrics-server's own collection interval. Polling
// faster than the source updates gains nothing but load on both processes.
const metricsCacheTTL = 30 * time.Second

// metricsFetchTimeout is short on purpose. This runs inside an admin page load,
// against a service on loopback; if it is not answering promptly the right
// outcome is a hidden card, not a hung dashboard.
const metricsFetchTimeout = 3 * time.Second

// diskWarnPct is where free space stops being somebody else's problem. The
// shared disk is 20 GB, which backupctl's retention policy already exists to
// protect; leaving it this late still leaves room to act.
const diskWarnPct = 85.0

type HostLoadAvg struct {
	One     float64 `json:"one"`
	Five    float64 `json:"five"`
	Fifteen float64 `json:"fifteen"`
}

type HostMemory struct {
	TotalKb     uint64  `json:"total_kb"`
	AvailableKb uint64  `json:"available_kb"`
	UsedKb      uint64  `json:"used_kb"`
	UsedPct     float64 `json:"used_pct"`
}

type HostCPU struct {
	UserPct   float64 `json:"user_pct"`
	SystemPct float64 `json:"system_pct"`
	IdlePct   float64 `json:"idle_pct"`
	// IowaitPct is the one worth having: a box that is slow because it is
	// waiting on the disk looks idle by every other measure.
	IowaitPct float64 `json:"iowait_pct"`
}

type HostDisk struct {
	TotalKb uint64  `json:"total_kb"`
	UsedKb  uint64  `json:"used_kb"`
	FreeKb  uint64  `json:"free_kb"`
	UsedPct float64 `json:"used_pct"`
}

type HostSystem struct {
	LoadAvg HostLoadAvg `json:"load_avg"`
	Memory  HostMemory  `json:"memory"`
	CPU     HostCPU     `json:"cpu"`
	Disk    HostDisk    `json:"disk"`
}

// HostTraffic is what Caddy's access log says, over metrics-server's window.
// This is real traffic measured at the proxy, independent of anything this
// application logs about itself.
type HostTraffic struct {
	WindowSeconds  uint64  `json:"window_seconds"`
	RequestsTotal  uint64  `json:"requests_total"`
	RequestsPerMin float64 `json:"requests_per_min"`
	Error4xx       uint64  `json:"error_4xx"`
	Error5xx       uint64  `json:"error_5xx"`
	ErrorPct       float64 `json:"error_pct"`
}

type HostApp struct {
	Name    string      `json:"name"`
	DiskKb  uint64      `json:"disk_kb"`
	Traffic HostTraffic `json:"traffic"`
}

type hostSnapshot struct {
	CollectedAt time.Time  `json:"collected_at"`
	System      HostSystem `json:"system"`
	Apps        []HostApp  `json:"apps"`
}

// HostMetricsResponse is the panel's view of the snapshot: the system block and
// this app's slice of it.
type HostMetricsResponse struct {
	// Configured is false when METRICS_URL is unset, which is a legitimate
	// state — the card is simply hidden.
	Configured bool `json:"configured"`
	// Available is false when the fetch failed. A metrics service that is down
	// must not take the admin panel with it, so this is reported rather than
	// returned as an error.
	Available bool   `json:"available"`
	Error     string `json:"error"`

	CollectedAt time.Time  `json:"collectedAt"`
	System      HostSystem `json:"system"`
	App         HostApp    `json:"app"`
}

var hostMetrics = struct {
	mu        sync.Mutex
	fetchedAt time.Time
	cached    HostMetricsResponse
}{}

func RegisterHostMetricsMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetHostMetrics)
}

// MetricsConfigured reports whether this deployment consumes metrics-server.
func MetricsConfigured() bool {
	return os.Getenv("METRICS_URL") != ""
}

// GetHostMetrics returns the cached host snapshot, refreshing it when stale.
func GetHostMetrics(ctx *vbeam.Context, req Empty) (resp HostMetricsResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}
	return fetchHostMetrics(), nil
}

// fetchHostMetrics is the cached read. It never returns an error: an absent or
// failing metrics service is a state to report, not a failure to propagate —
// the same instinct the /admin fetch already has about diagnostics.
func fetchHostMetrics() HostMetricsResponse {
	if !MetricsConfigured() {
		return HostMetricsResponse{}
	}

	hostMetrics.mu.Lock()
	defer hostMetrics.mu.Unlock()

	if time.Since(hostMetrics.fetchedAt) < metricsCacheTTL && hostMetrics.cached.Configured {
		return hostMetrics.cached
	}

	resp := HostMetricsResponse{Configured: true}
	snapshot, err := requestHostSnapshot()
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Available = true
		resp.CollectedAt = snapshot.CollectedAt
		resp.System = snapshot.System
		for _, app := range snapshot.Apps {
			if app.Name == metricsAppName {
				resp.App = app
				break
			}
		}
	}

	// Cache failures too, so a service that is down is asked about every 30
	// seconds rather than on every page load.
	hostMetrics.fetchedAt = time.Now()
	hostMetrics.cached = resp
	return resp
}

func requestHostSnapshot() (hostSnapshot, error) {
	var snapshot hostSnapshot

	ctx, cancel := context.WithTimeout(context.Background(), metricsFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, os.Getenv("METRICS_URL"), nil)
	if err != nil {
		return snapshot, fmt.Errorf("METRICS_URL is not usable: %w", err)
	}
	if key := os.Getenv("METRICS_API_KEY"); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return snapshot, fmt.Errorf("metrics service did not answer: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		// 401 is the one worth naming: it means the key is wrong, not that
		// the service is down, and those need different fixes.
		if res.StatusCode == http.StatusUnauthorized {
			return snapshot, fmt.Errorf("metrics service rejected METRICS_API_KEY")
		}
		return snapshot, fmt.Errorf("metrics service answered %d", res.StatusCode)
	}

	if err := json.NewDecoder(res.Body).Decode(&snapshot); err != nil {
		return snapshot, fmt.Errorf("metrics service response was not readable: %w", err)
	}
	return snapshot, nil
}
