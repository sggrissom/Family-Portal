package backend

import (
	"family/cfg"
	"path/filepath"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterAdminHealthMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetSystemHealth)
}

type ConfigProblem struct {
	Setting string `json:"setting"`
	Detail  string `json:"detail"`
}

type LogProblems struct {
	WindowHours  int              `json:"windowHours"`
	Errors       int              `json:"errors"`
	RecentErrors []PublicLogEntry `json:"recentErrors"`
	Requests4xx  int              `json:"requests4xx"`
	Requests5xx  int              `json:"requests5xx"`
	Unavailable  bool             `json:"unavailable"`
}

type PhotoProblems struct {
	Failed         int  `json:"failed"`
	Stuck          int  `json:"stuck"`
	AnalysisFailed int  `json:"analysisFailed"`
	WorkerStopped  bool `json:"workerStopped"`
	QueueLength    int  `json:"queueLength"`
}

type PushProblems struct {
	Failed      int       `json:"failed"`
	LastError   string    `json:"lastError"`
	LastErrorAt time.Time `json:"lastErrorAt"`
}

type HostProblems struct {
	Available     bool    `json:"available"`
	DiskUsedPct   float64 `json:"diskUsedPct"`
	DiskLow       bool    `json:"diskLow"`
	Proxy5xx      int     `json:"proxy5xx"`
	Proxy4xx      int     `json:"proxy4xx"`
	WindowSeconds int     `json:"windowSeconds"`
}

type SystemHealthResponse struct {
	Healthy      bool            `json:"healthy"`
	ReleaseBuild bool            `json:"releaseBuild"`
	ConfigIssues []ConfigProblem `json:"configIssues"`
	Logs         LogProblems     `json:"logs"`
	Photos       PhotoProblems   `json:"photos"`
	Push         PushProblems    `json:"push"`
	Host         HostProblems    `json:"host"`
}

const healthLogWindow = 24 * time.Hour

const healthRecentErrors = 5

const stuckPhotoAge = time.Hour

func GetSystemHealth(ctx *vbeam.Context, req Empty) (resp SystemHealthResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	resp.ReleaseBuild = cfg.IsRelease
	for _, issue := range CheckProductionConfig(cfg.DBPath, cfg.StaticDir, cfg.LogDir) {
		resp.ConfigIssues = append(resp.ConfigIssues, ConfigProblem{
			Setting: issue.Setting,
			Detail:  issue.Detail,
		})
	}

	resp.Logs = collectLogProblems()
	resp.Photos = collectPhotoProblems(ctx.Tx)

	host := fetchHostMetrics()
	if host.Available {
		resp.Host = HostProblems{
			Available:     true,
			DiskUsedPct:   host.System.Disk.UsedPct,
			DiskLow:       host.System.Disk.UsedPct >= diskWarnPct,
			Proxy5xx:      int(host.App.Traffic.Error5xx),
			Proxy4xx:      int(host.App.Traffic.Error4xx),
			WindowSeconds: int(host.App.Traffic.WindowSeconds),
		}
	}

	push := GetPushWorkerStats()
	resp.Push = PushProblems{
		Failed:      push.Failed,
		LastError:   push.LastError,
		LastErrorAt: push.LastErrorAt,
	}

	resp.Healthy = len(resp.ConfigIssues) == 0 &&
		resp.Logs.Errors == 0 &&
		resp.Logs.Requests5xx == 0 &&
		!resp.Logs.Unavailable &&
		resp.Photos.Failed == 0 &&
		resp.Photos.Stuck == 0 &&
		resp.Photos.AnalysisFailed == 0 &&
		!resp.Photos.WorkerStopped &&
		resp.Push.LastError == "" &&
		!resp.Host.DiskLow &&
		resp.Host.Proxy5xx == 0

	return
}

func collectLogProblems() LogProblems {
	problems := LogProblems{
		WindowHours:  int(healthLogWindow / time.Hour),
		RecentErrors: []PublicLogEntry{},
	}

	files, err := listLogFiles()
	if err != nil || len(files) == 0 {
		problems.Unavailable = true
		return problems
	}

	cutoff := time.Now().Add(-healthLogWindow)
	recent := newEntryRing(healthRecentErrors)
	scanned := 0

	for _, file := range files {
		if file.ModTime.Before(cutoff) {
			continue
		}
		scanned++

		scanErr := scanLogFile(filepath.Join(cfg.LogDir, file.Name), func(entry logEntry) bool {
			if entry.Timestamp.Before(cutoff) {
				return true
			}
			if entry.Level == logLevelError {
				problems.Errors++
				recent.add(entry)
			}
			if entry.HTTPStatus != nil {
				switch {
				case *entry.HTTPStatus >= 500:
					problems.Requests5xx++
				case *entry.HTTPStatus >= 400:
					problems.Requests4xx++
				}
			}
			return true
		})
		if scanErr != nil {
			LogErrorSimple(LogCategoryAdmin, "Could not read a log file for the health summary", map[string]interface{}{
				"file":  file.Name,
				"error": scanErr.Error(),
			})
		}
	}

	if scanned == 0 {
		problems.Unavailable = true
		return problems
	}

	for _, entry := range recent.newestFirst() {
		problems.RecentErrors = append(problems.RecentErrors, convertToPublicLogEntry(entry))
	}
	return problems
}

func collectPhotoProblems(tx *vbolt.Tx) PhotoProblems {
	processing := GetProcessingStats()
	problems := PhotoProblems{
		QueueLength:   processing.QueueLength,
		WorkerStopped: !processing.IsRunning,
	}

	stuckBefore := time.Now().Add(-stuckPhotoAge)
	vbolt.IterateAll(tx, ImagesBkt, func(key int, image Image) bool {
		switch {
		case image.Status == 2:
			problems.Failed++
		case image.Status == 1 && image.CreatedAt.Before(stuckBefore):
			problems.Stuck++
		}
		if image.AnalysisStatus == 3 {
			problems.AnalysisFailed++
		}
		return true
	})

	return problems
}
