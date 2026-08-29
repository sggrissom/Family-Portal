package backend

import (
	"family/cfg"
	"runtime"
	"time"

	"go.hasen.dev/vbeam"
)

var startedAt = time.Now()

type DiagnosticsResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
	Release   bool   `json:"release"`

	GoVersion string `json:"goVersion"`

	StartedAt     time.Time `json:"startedAt"`
	UptimeSeconds int       `json:"uptimeSeconds"`

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
