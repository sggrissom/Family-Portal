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

const metricsAppName = "family"

const metricsCacheTTL = 30 * time.Second

const metricsFetchTimeout = 3 * time.Second

const diskWarnPct = 85.0

// A backup older than this is stale. backupctl's own alert threshold, so the
// panel and the nightly mail agree about when to start worrying.
const backupStaleAge = 48 * time.Hour

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

type HostTraffic struct {
	WindowSeconds  uint64  `json:"window_seconds"`
	RequestsTotal  uint64  `json:"requests_total"`
	RequestsPerMin float64 `json:"requests_per_min"`
	Error4xx       uint64  `json:"error_4xx"`
	Error5xx       uint64  `json:"error_5xx"`
	ErrorPct       float64 `json:"error_pct"`
}

type HostBackups struct {
	Registered  bool      `json:"registered"`
	LastSuccess time.Time `json:"last_success"`
	AgeSeconds  uint64    `json:"age_seconds"`
	SizeKb      uint64    `json:"size_kb"`
}

type HostRelease struct {
	Name       string    `json:"name"`
	Sha        string    `json:"sha"`
	DeployedAt time.Time `json:"deployed_at"`
	Current    bool      `json:"current"`
}

type HostApp struct {
	Name     string        `json:"name"`
	DiskKb   uint64        `json:"disk_kb"`
	Traffic  HostTraffic   `json:"traffic"`
	Backups  HostBackups   `json:"backups"`
	Releases []HostRelease `json:"releases"`
}

type hostSnapshot struct {
	CollectedAt time.Time  `json:"collected_at"`
	System      HostSystem `json:"system"`
	Apps        []HostApp  `json:"apps"`
}

type HostMetricsResponse struct {
	Configured bool   `json:"configured"`
	Available  bool   `json:"available"`
	Error      string `json:"error"`

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

func MetricsConfigured() bool {
	return os.Getenv("METRICS_URL") != ""
}

func GetHostMetrics(ctx *vbeam.Context, req Empty) (resp HostMetricsResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}
	return fetchHostMetrics(), nil
}

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
		if resp.App.Releases == nil {
			resp.App.Releases = []HostRelease{}
		}
	}

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
