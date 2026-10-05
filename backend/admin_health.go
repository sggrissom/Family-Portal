package backend

import (
	"family/cfg"
	"path/filepath"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterAdminHealthMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetSystemHealth)
	vbeam.RegisterProc(app, ReviewLogFailures)
}

type ConfigProblem struct {
	Setting string `json:"setting"`
	Detail  string `json:"detail"`
}

type LogProblems struct {
	LatestFailureAt time.Time        `json:"latestFailureAt"`
	ReviewedThrough time.Time        `json:"reviewedThrough"`
	WindowHours     int              `json:"windowHours"`
	Errors          int              `json:"errors"`
	RecentErrors    []PublicLogEntry `json:"recentErrors"`
	Requests4xx     int              `json:"requests4xx"`
	Requests5xx     int              `json:"requests5xx"`
	Unavailable     bool             `json:"unavailable"`
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

type MailProblems struct {
	Failed      int       `json:"failed"`
	LastError   string    `json:"lastError"`
	LastErrorAt time.Time `json:"lastErrorAt"`
	QueueLength int       `json:"queueLength"`
}

type HostProblems struct {
	Available     bool    `json:"available"`
	DiskUsedPct   float64 `json:"diskUsedPct"`
	DiskLow       bool    `json:"diskLow"`
	Proxy5xx      int     `json:"proxy5xx"`
	Proxy4xx      int     `json:"proxy4xx"`
	WindowSeconds int     `json:"windowSeconds"`
}

type BackupProblems struct {
	Available   bool      `json:"available"`
	Registered  bool      `json:"registered"`
	NeverRun    bool      `json:"neverRun"`
	Stale       bool      `json:"stale"`
	LastSuccess time.Time `json:"lastSuccess"`
	SizeKb      int       `json:"sizeKb"`
}

type SystemHealthResponse struct {
	Healthy      bool            `json:"healthy"`
	ReleaseBuild bool            `json:"releaseBuild"`
	ConfigIssues []ConfigProblem `json:"configIssues"`
	Logs         LogProblems     `json:"logs"`
	Photos       PhotoProblems   `json:"photos"`
	Push         PushProblems    `json:"push"`
	Mail         MailProblems    `json:"mail"`
	Host         HostProblems    `json:"host"`
	Backups      BackupProblems  `json:"backups"`
}

const healthLogWindow = 24 * time.Hour

const healthRecentErrors = 5

const stuckPhotoAge = time.Hour

func GetSystemHealth(ctx *vbeam.Context, req Empty) (resp SystemHealthResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}
	return collectSystemHealth(ctx.Tx), nil
}

// One persistent watermark keeps the logs intact while acknowledging old failures.
// Store RFC3339Nano to preserve the exact boundary across restarts.
type LogReview struct {
	Through string
}

func PackLogReview(self *LogReview, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.String(&self.Through, buf)
}

var LogReviewBkt = vbolt.Bucket(&cfg.Info, "log_review", vpack.FInt, PackLogReview)

func reviewedLogsThrough(tx *vbolt.Tx) time.Time {
	var review LogReview
	vbolt.Read(tx, LogReviewBkt, 1, &review)
	through, _ := time.Parse(time.RFC3339Nano, review.Through)
	return through
}

type ReviewLogFailuresRequest struct {
	Through time.Time `json:"through"`
}

func ReviewLogFailures(ctx *vbeam.Context, req ReviewLogFailuresRequest) (resp SystemHealthResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}
	if req.Through.IsZero() || req.Through.After(time.Now()) {
		return resp, NewAppError(ErrCodeValidation, "Choose an existing log failure to review")
	}
	vbeam.UseWriteTx(ctx)
	if req.Through.After(reviewedLogsThrough(ctx.Tx)) {
		review := LogReview{Through: req.Through.UTC().Format(time.RFC3339Nano)}
		vbolt.Write(ctx.Tx, LogReviewBkt, 1, &review)
	}
	db := ctx.Tx.DB()
	vbolt.TxCommit(ctx.Tx)
	ctx.Tx = vbolt.ReadTx(db)
	LogInfo(LogCategoryAdmin, "Admin reviewed log failures", map[string]interface{}{
		"through": req.Through,
	})
	return collectSystemHealth(ctx.Tx), nil
}

// Shared with the health monitor so an alert and the admin panel cannot disagree
// about what "healthy" means.
func collectSystemHealth(tx *vbolt.Tx) (resp SystemHealthResponse) {
	resp.ReleaseBuild = cfg.IsRelease
	resp.ConfigIssues = []ConfigProblem{}
	for _, issue := range CheckProductionConfig(cfg.DBPath, cfg.StaticDir, cfg.LogDir) {
		resp.ConfigIssues = append(resp.ConfigIssues, ConfigProblem{
			Setting: issue.Setting,
			Detail:  issue.Detail,
		})
	}

	resp.Logs = collectLogProblems(reviewedLogsThrough(tx))
	resp.Photos = collectPhotoProblems(tx)

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
		if !isStagingSite() {
			resp.Backups = backupProblems(host.App.Backups)
		}
	}

	push := GetPushWorkerStats()
	resp.Push = PushProblems{
		Failed:      push.Failed,
		LastError:   push.LastError,
		LastErrorAt: push.LastErrorAt,
	}

	mail := GetMailWorkerStats()
	resp.Mail = MailProblems{
		Failed:      mail.Failed,
		LastError:   mail.LastError,
		LastErrorAt: mail.LastErrorAt,
		QueueLength: mail.QueueLength,
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
		resp.Mail.LastError == "" &&
		!resp.Host.DiskLow &&
		resp.Host.Proxy5xx == 0 &&
		!backupTrouble(resp.Backups)

	return
}

// A backup nobody is watching is backupctl's own stated failure mode, so the
// three states worth surfacing are "not registered at all", "registered and
// never once succeeded", and "succeeded, but not lately".
func backupProblems(backups HostBackups) BackupProblems {
	problems := BackupProblems{
		Available:   true,
		Registered:  backups.Registered,
		LastSuccess: backups.LastSuccess,
		SizeKb:      int(backups.SizeKb),
	}
	if !backups.Registered {
		return problems
	}
	if backups.LastSuccess.IsZero() {
		problems.NeverRun = true
		return problems
	}
	problems.Stale = time.Since(backups.LastSuccess) > backupStaleAge
	return problems
}

func backupTrouble(problems BackupProblems) bool {
	return problems.Available && (!problems.Registered || problems.NeverRun || problems.Stale)
}

func collectLogProblems(reviewedThrough time.Time) LogProblems {
	problems := LogProblems{
		WindowHours:     int(healthLogWindow / time.Hour),
		ReviewedThrough: reviewedThrough,
		RecentErrors:    []PublicLogEntry{},
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
			// Also exclude AUTH_ERROR entries written before 401s became warnings.
			data, _ := entry.Data.(map[string]interface{})
			failure := entry.Timestamp.After(reviewedThrough)
			if entry.Level == logLevelError && data["code"] != string(ErrCodeAuth) && failure {
				problems.Errors++
				if entry.Timestamp.After(problems.LatestFailureAt) {
					problems.LatestFailureAt = entry.Timestamp
				}
				recent.add(entry)
			}
			if entry.HTTPStatus != nil {
				switch {
				case *entry.HTTPStatus >= 500 && failure:
					if entry.Timestamp.After(problems.LatestFailureAt) {
						problems.LatestFailureAt = entry.Timestamp
					}
					problems.Requests5xx++
				case *entry.HTTPStatus >= 400 && *entry.HTTPStatus < 500:
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
