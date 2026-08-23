package backend

import (
	"family/cfg"
	"path/filepath"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

// The admin panel was organised by subsystem, so noticing a problem meant
// visiting six pages and knowing what normal looked like on each. For a site
// with one operator who checks in occasionally that is backwards: the landing
// page should answer "is anything wrong" without a click, and the subsystem
// pages should be where you go *after* it says yes.
//
// Everything here is already computed somewhere. What was missing was one place
// that asks all of it at once and stays quiet when the answer is "nothing".

func RegisterAdminHealthMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetSystemHealth)
}

// ConfigProblem is a ConfigIssue on the wire. ConfigIssue is an error type with
// unexported-by-convention semantics; this is the flat pair the browser needs.
type ConfigProblem struct {
	Setting string `json:"setting"`
	Detail  string `json:"detail"`
}

// LogProblems summarises what the log files say about the last day.
type LogProblems struct {
	// WindowHours is the window everything below was counted over, stated
	// rather than assumed — the numbers mean nothing without it.
	WindowHours int `json:"windowHours"`
	Errors      int `json:"errors"`
	// RecentErrors is the tail of them, newest first. Their reference codes
	// are the join key into the log viewer's lookup.
	RecentErrors []PublicLogEntry `json:"recentErrors"`
	// Requests4xx and Requests5xx come from the HTTP timing lines. A 5xx is
	// the app failing; a 4xx in bulk usually means something client-side is
	// retrying against an endpoint that will never accept it.
	Requests4xx int `json:"requests4xx"`
	Requests5xx int `json:"requests5xx"`
	// Unavailable is set when there are no log files at all, which is itself
	// worth saying: it means either a fresh deploy or a logger writing nowhere.
	Unavailable bool `json:"unavailable"`
}

// PhotoProblems counts the photo pipeline's two failure modes.
type PhotoProblems struct {
	Failed int `json:"failed"`
	// Stuck is rows still marked Processing with nothing attending them. The
	// worker sets that status when it picks a job up, so a row sitting in it an
	// hour later was interrupted and nothing will move it on its own.
	Stuck          int `json:"stuck"`
	AnalysisFailed int `json:"analysisFailed"`
	// WorkerStopped is separate from a queue depth: a backlog with a running
	// worker is patience, a backlog without one is a problem.
	WorkerStopped bool `json:"workerStopped"`
	QueueLength   int  `json:"queueLength"`
}

// PushProblems is the subset of PushWorkerStats worth escalating to the
// landing page. The push page itself has the rest.
type PushProblems struct {
	Failed      int       `json:"failed"`
	LastError   string    `json:"lastError"`
	LastErrorAt time.Time `json:"lastErrorAt"`
}

type SystemHealthResponse struct {
	// Healthy is false when anything below is worth looking at. A green page
	// has to mean something, so this is the one field the UI branches on.
	Healthy bool `json:"healthy"`
	// ReleaseBuild says how to read ConfigIssues: a release build refuses to
	// start with any, so seeing them here means the environment changed under
	// a running process. A local build logs them and carries on, and a
	// development machine legitimately has no APNs key.
	ReleaseBuild bool            `json:"releaseBuild"`
	ConfigIssues []ConfigProblem `json:"configIssues"`
	Logs         LogProblems     `json:"logs"`
	Photos       PhotoProblems   `json:"photos"`
	Push         PushProblems    `json:"push"`
}

// healthLogWindow is how far back the landing page looks. A day is the span
// over which "did something break" is still a question about now.
const healthLogWindow = 24 * time.Hour

// healthRecentErrors is how many error entries to carry to the page. This is a
// "go look at this" list, not a log viewer.
const healthRecentErrors = 5

// stuckPhotoAge is how long a row may sit in Processing before it counts as
// stranded rather than in progress.
const stuckPhotoAge = time.Hour

// GetSystemHealth answers "is anything wrong" in one call.
//
// It re-runs the startup configuration check live rather than reporting what
// was true at boot, so an edited .env and a restart are reflected — and so is
// an .env that changed *without* a restart, which is the case worth catching.
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
		resp.Push.LastError == ""

	return
}

// collectLogProblems reads the last day out of the log files.
//
// Files untouched since the cutoff are skipped entirely rather than scanned and
// filtered: after §2.1 a day's traffic is one file, so on a normal load this
// reads one file and stats the rest.
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

	// Files exist but none has been written to today. Nothing is wrong with
	// the app; nothing is being logged either, which is worth not hiding.
	if scanned == 0 {
		problems.Unavailable = true
		return problems
	}

	for _, entry := range recent.newestFirst() {
		problems.RecentErrors = append(problems.RecentErrors, convertToPublicLogEntry(entry))
	}
	return problems
}

// collectPhotoProblems counts what the photo pipeline has left behind.
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
