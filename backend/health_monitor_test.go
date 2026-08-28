package backend

import (
	"strings"
	"testing"
	"time"
)

func healthy() SystemHealthResponse {
	return SystemHealthResponse{Healthy: true}
}

func unhealthy() SystemHealthResponse {
	return SystemHealthResponse{
		Healthy: false,
		Photos:  PhotoProblems{Failed: 2},
	}
}

func TestHealthMonitorSilentWhenHealthyFromTheStart(t *testing.T) {
	hm := &healthMonitor{}
	now := time.Now()

	if _, _, send := hm.evaluate(healthy(), now); send {
		t.Fatal("the first healthy check should not alert")
	}
	if _, _, send := hm.evaluate(healthy(), now.Add(time.Hour)); send {
		t.Fatal("a steady healthy state should not alert")
	}
}

func TestHealthMonitorAlertsOnceOnTransition(t *testing.T) {
	hm := &healthMonitor{}
	now := time.Now()

	hm.evaluate(healthy(), now)

	subject, body, send := hm.evaluate(unhealthy(), now.Add(healthCheckInterval))
	if !send {
		t.Fatal("going unhealthy should alert")
	}
	if !strings.Contains(subject, "problems detected") {
		t.Errorf("unexpected subject: %q", subject)
	}
	if !strings.Contains(body, "2 failed to process") {
		t.Errorf("body should name the problem, got: %q", body)
	}

	if _, _, send := hm.evaluate(unhealthy(), now.Add(2*healthCheckInterval)); send {
		t.Error("a still-broken system should not alert again immediately")
	}
}

func TestHealthMonitorRemindsAfterADay(t *testing.T) {
	hm := &healthMonitor{}
	start := time.Now()

	hm.evaluate(healthy(), start)
	hm.evaluate(unhealthy(), start.Add(time.Minute))

	if _, _, send := hm.evaluate(unhealthy(), start.Add(healthReminderInterval-time.Minute)); send {
		t.Error("should not remind before the interval elapses")
	}

	subject, _, send := hm.evaluate(unhealthy(), start.Add(healthReminderInterval+time.Minute))
	if !send {
		t.Fatal("should remind once the interval elapses")
	}
	if !strings.Contains(subject, "continuing") {
		t.Errorf("a reminder should read as a continuation, got %q", subject)
	}
}

func TestHealthMonitorAlertsOnRecovery(t *testing.T) {
	hm := &healthMonitor{}
	now := time.Now()

	hm.evaluate(healthy(), now)
	hm.evaluate(unhealthy(), now.Add(time.Minute))

	subject, _, send := hm.evaluate(healthy(), now.Add(2*time.Minute))
	if !send {
		t.Fatal("recovery should alert")
	}
	if !strings.Contains(subject, "recovered") {
		t.Errorf("unexpected subject: %q", subject)
	}

	if _, _, send := hm.evaluate(healthy(), now.Add(3*time.Minute)); send {
		t.Error("recovery should alert only once")
	}
}

// Starting up already broken must not be mistaken for a healthy baseline.
func TestHealthMonitorAlertsWhenFirstCheckIsUnhealthy(t *testing.T) {
	hm := &healthMonitor{}

	subject, _, send := hm.evaluate(unhealthy(), time.Now())
	if !send {
		t.Fatal("a first check that is already broken should alert")
	}
	if !strings.Contains(subject, "at startup") {
		t.Errorf("unexpected subject: %q", subject)
	}
}

func TestDescribeHealthCoversEverySignal(t *testing.T) {
	health := SystemHealthResponse{
		ConfigIssues: []ConfigProblem{{Setting: "JWT_SECRET_KEY", Detail: "must be set"}},
		Logs:         LogProblems{Errors: 3, WindowHours: 24, Requests5xx: 7},
		Photos:       PhotoProblems{Failed: 1, Stuck: 2, AnalysisFailed: 3, WorkerStopped: true},
		Push:         PushProblems{LastError: "apns rejected the token"},
		Mail:         MailProblems{LastError: "smtp timeout"},
		Host:         HostProblems{Available: true, DiskLow: true, DiskUsedPct: 91.5, Proxy5xx: 4, WindowSeconds: 300},
		Backups:      BackupProblems{Available: true, Registered: true, LastSuccess: time.Now().Add(-72 * time.Hour), Stale: true},
	}

	body := describeHealth(health)
	for _, want := range []string{
		"JWT_SECRET_KEY",
		"3 error(s)",
		"7 request(s) returned 5xx",
		"processing worker is not running",
		"1 failed to process",
		"2 stuck processing",
		"3 failed face analysis",
		"apns rejected the token",
		"smtp timeout",
		"91.5% full",
		"4 5xx response(s)",
		"Backups: last success was",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n\n%s", want, body)
		}
	}
}

func TestDescribeHealthNeverSendsAnEmptyList(t *testing.T) {
	body := describeHealth(SystemHealthResponse{})
	if !strings.Contains(body, "listed no detail") {
		t.Errorf("expected a fallback line, got: %q", body)
	}
}
