package backend

import (
	"family/cfg"
	"runtime"
	"time"

	"go.hasen.dev/vbeam"
)

// startedAt is the process's own start time. Uptime is the single most useful
// number when something is wrong — it says whether the thing you are looking at
// is a fresh deploy, a crash loop, or a process that has been up for a month.
var startedAt = time.Now()

type DiagnosticsResponse struct {
	// Build identifies the running binary. Version comes from cfg.Version;
	// Commit and BuildTime are stamped by the linker.
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
	Release   bool   `json:"release"`

	GoVersion string `json:"goVersion"`

	StartedAt     time.Time `json:"startedAt"`
	UptimeSeconds int       `json:"uptimeSeconds"`

	// Worker state, so "is anything actually running" has an answer that does
	// not require reading a log file over SSH.
	PhotoQueue     int  `json:"photoQueue"`
	PhotoRunning   bool `json:"photoRunning"`
	AnalysisQueue  int  `json:"analysisQueue"`
	AnalysisFaces  bool `json:"analysisFaces"`
	MailQueue      int  `json:"mailQueue"`
	PushConfigured bool `json:"pushConfigured"`
}

func RegisterDiagnosticsMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetDiagnostics)
}

// GetDiagnostics reports what this process is and what it is doing.
//
// Authenticated and admin-only. None of it is secret exactly — a version number
// is not a credential — but a stranger has no reason to learn which commit is
// deployed or how long it has been up, and both are useful to someone looking
// for a known vulnerability in a known build.
//
// Deliberately absent: filesystem paths, environment variable values, and
// anything derived from a configured secret. The diagnostics view answers "what
// is running", not "how is it configured".
func GetDiagnostics(ctx *vbeam.Context, req Empty) (resp DiagnosticsResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	build := cfg.Build()
	processing := GetProcessingStats()
	analysis := GetAnalysisWorkerStats()

	resp = DiagnosticsResponse{
		Version:        build.Version,
		Commit:         build.Commit,
		BuildTime:      build.BuildTime,
		Release:        build.Release,
		GoVersion:      runtime.Version(),
		StartedAt:      startedAt,
		UptimeSeconds:  int(time.Since(startedAt).Seconds()),
		PhotoQueue:     processing.QueueLength,
		PhotoRunning:   processing.IsRunning,
		AnalysisQueue:  analysis.QueueLength,
		AnalysisFaces:  analysis.IsRunning,
		MailQueue:      GetMailQueueLength(),
		PushConfigured: IsPushWorkerEnabled(),
	}
	return
}
