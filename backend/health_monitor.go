package backend

import (
	"context"
	"family/cfg"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.hasen.dev/vbolt"
)

const healthCheckInterval = 15 * time.Minute

// While something stays broken, repeat at most this often.
const healthReminderInterval = 24 * time.Hour

// Covers failures the process survives; a dead process cannot report itself, so
// an external check on /healthz covers the rest.
type healthMonitor struct {
	workerLifecycle
	db *vbolt.DB

	mu          sync.Mutex
	lastHealthy bool
	lastAlertAt time.Time
	started     bool
}

var globalHealthMonitor *healthMonitor

func InitializeHealthMonitor(db *vbolt.DB) {
	if globalHealthMonitor != nil {
		LogInfo(LogCategoryWorker, "Health monitor already initialized, skipping")
		return
	}

	globalHealthMonitor = &healthMonitor{db: db, lastHealthy: true}
	globalHealthMonitor.start()

	LogInfo(LogCategoryWorker, "Health monitor started", map[string]interface{}{
		"intervalMinutes": int(healthCheckInterval / time.Minute),
	})
}

func (hm *healthMonitor) start() {
	quit, done, ok := hm.workerLifecycle.start()
	if !ok {
		return
	}
	go hm.run(quit, done)
}

func (hm *healthMonitor) run(quit <-chan struct{}, done chan struct{}) {
	defer close(done)

	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-quit:
			return
		case <-ticker.C:
			hm.check()
		}
	}
}

func (hm *healthMonitor) check() {
	var health SystemHealthResponse
	vbolt.WithReadTx(hm.db, func(tx *vbolt.Tx) {
		health = collectSystemHealth(tx)
	})

	subject, body, send := hm.evaluate(health, time.Now())
	if !send {
		return
	}

	to := hm.recipient()
	if to == "" {
		LogWarn(LogCategoryWorker, "Health alert has no recipient; the admin account has no email address", nil)
		return
	}

	if err := QueueMail(MailJob{To: to, Subject: subject, Body: body, Kind: "health-alert"}); err != nil {
		LogErrorSimple(LogCategoryWorker, "Could not queue a health alert", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

// Split from check so the transition rules are testable without a database.
func (hm *healthMonitor) evaluate(health SystemHealthResponse, now time.Time) (subject, body string, send bool) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	// A baseline, so a restart does not alert about a pre-existing problem.
	if !hm.started {
		hm.started = true
		hm.lastHealthy = health.Healthy
		if !health.Healthy {
			hm.lastAlertAt = now
			return "Family Portal: problems detected at startup", describeHealth(health), true
		}
		return "", "", false
	}

	if health.Healthy {
		recovered := !hm.lastHealthy
		hm.lastHealthy = true
		hm.lastAlertAt = time.Time{}
		if recovered {
			return "Family Portal: recovered", "Every health check is passing again.", true
		}
		return "", "", false
	}

	newlyBroken := hm.lastHealthy
	hm.lastHealthy = false
	if newlyBroken || now.Sub(hm.lastAlertAt) >= healthReminderInterval {
		hm.lastAlertAt = now
		subject = "Family Portal: problems detected"
		if !newlyBroken {
			subject = "Family Portal: problems continuing"
		}
		return subject, describeHealth(health), true
	}
	return "", "", false
}

func (hm *healthMonitor) recipient() string {
	var email string
	vbolt.WithReadTx(hm.db, func(tx *vbolt.Tx) {
		email = GetUser(tx, AdminUserId).Email
	})
	return email
}

func describeHealth(health SystemHealthResponse) string {
	var lines []string
	add := func(format string, args ...interface{}) {
		lines = append(lines, "- "+fmt.Sprintf(format, args...))
	}

	for _, issue := range health.ConfigIssues {
		add("Config: %s %s", issue.Setting, issue.Detail)
	}
	if health.Logs.Unavailable {
		add("Logs: could not be read")
	}
	if health.Logs.Errors > 0 {
		add("Logs: %d error(s) in the last %dh", health.Logs.Errors, health.Logs.WindowHours)
	}
	if health.Logs.Requests5xx > 0 {
		add("Logs: %d request(s) returned 5xx", health.Logs.Requests5xx)
	}
	if health.Photos.WorkerStopped {
		add("Photos: the processing worker is not running")
	}
	if health.Photos.Failed > 0 {
		add("Photos: %d failed to process", health.Photos.Failed)
	}
	if health.Photos.Stuck > 0 {
		add("Photos: %d stuck processing for over an hour", health.Photos.Stuck)
	}
	if health.Photos.AnalysisFailed > 0 {
		add("Photos: %d failed face analysis", health.Photos.AnalysisFailed)
	}
	if health.Push.LastError != "" {
		add("Push: %s", health.Push.LastError)
	}
	if health.Mail.LastError != "" {
		add("Mail: %s", health.Mail.LastError)
	}
	if health.Host.DiskLow {
		add("Host: disk is %.1f%% full", health.Host.DiskUsedPct)
	}
	if health.Host.Proxy5xx > 0 {
		add("Host: the proxy saw %d 5xx response(s) in the last %ds",
			health.Host.Proxy5xx, health.Host.WindowSeconds)
	}
	if backupTrouble(health.Backups) {
		switch {
		case !health.Backups.Registered:
			add("Backups: this app is not registered for backup")
		case health.Backups.NeverRun:
			add("Backups: registered but never completed")
		default:
			add("Backups: last success was %s", health.Backups.LastSuccess.Format(time.RFC1123))
		}
	}

	if len(lines) == 0 {
		add("The health check reported a problem but listed no detail.")
	}

	return fmt.Sprintf("The health check found:\n\n%s\n\nFull detail: %s/admin\n",
		strings.Join(lines, "\n"), cfg.SiteURL)
}

func StopHealthMonitor(ctx context.Context) bool {
	if globalHealthMonitor == nil {
		return true
	}
	return globalHealthMonitor.stopAndWait(ctx, false)
}
