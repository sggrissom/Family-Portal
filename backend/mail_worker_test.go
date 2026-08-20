package backend

import (
	"errors"
	"net/textproto"
	"sync"
	"testing"
	"time"
)

// mailRecorder captures delivery attempts and lets a test decide what each one
// returns.
type mailRecorder struct {
	mu       sync.Mutex
	jobs     []MailJob
	results  []error
	attempts chan int
}

// newMailRecorder installs a stub deliverer. results supplies the error for
// each successive attempt; attempts past the end of the slice succeed.
func newMailRecorder(t *testing.T, results ...error) *mailRecorder {
	t.Helper()

	recorder := &mailRecorder{
		results:  results,
		attempts: make(chan int, 16),
	}

	original := mailDeliverer
	mailDeliverer = func(job MailJob) error {
		recorder.mu.Lock()
		recorder.jobs = append(recorder.jobs, job)
		index := len(recorder.jobs) - 1
		recorder.mu.Unlock()

		recorder.attempts <- index + 1

		if index < len(recorder.results) {
			return recorder.results[index]
		}
		return nil
	}
	t.Cleanup(func() { mailDeliverer = original })

	return recorder
}

func (r *mailRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.jobs)
}

// waitForAttempts blocks until n delivery attempts have been made, failing the
// test rather than hanging if they never arrive.
func (r *mailRecorder) waitForAttempts(t *testing.T, n int) {
	t.Helper()

	for i := 0; i < n; i++ {
		select {
		case <-r.attempts:
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for delivery attempt %d of %d (saw %d)", i+1, n, r.count())
		}
	}
}

// useTestMailWorker installs a running worker with short retry delays so the
// backoff does not dominate the test runtime.
func useTestMailWorker(t *testing.T, queueSize int) *MailWorker {
	t.Helper()

	originalWorker := globalMailWorker
	originalDelays := mailRetryDelays
	mailRetryDelays = []time.Duration{time.Millisecond, time.Millisecond}

	worker := &MailWorker{
		jobQueue: make(chan MailJob, queueSize),
	}
	globalMailWorker = worker
	worker.Start()

	t.Cleanup(func() {
		worker.Stop()
		globalMailWorker = originalWorker
		mailRetryDelays = originalDelays
	})

	return worker
}

func TestQueueMailSendsInlineWithoutWorker(t *testing.T) {
	recorder := newMailRecorder(t)

	original := globalMailWorker
	globalMailWorker = nil
	t.Cleanup(func() { globalMailWorker = original })

	if err := QueueMail(MailJob{To: "user@example.com", Subject: "Hi", Kind: "test"}); err != nil {
		t.Fatalf("QueueMail() error = %v", err)
	}

	// No worker means the send already happened by the time QueueMail returned.
	if recorder.count() != 1 {
		t.Fatalf("delivery attempts = %d, want 1", recorder.count())
	}
	if recorder.jobs[0].To != "user@example.com" {
		t.Errorf("To = %q, want user@example.com", recorder.jobs[0].To)
	}
}

func TestQueueMailDeliversInBackground(t *testing.T) {
	recorder := newMailRecorder(t)
	useTestMailWorker(t, 4)

	if err := QueueMail(MailJob{To: "user@example.com", Subject: "Hi", Kind: "test"}); err != nil {
		t.Fatalf("QueueMail() error = %v", err)
	}

	recorder.waitForAttempts(t, 1)
	if recorder.count() != 1 {
		t.Fatalf("delivery attempts = %d, want 1", recorder.count())
	}
}

func TestMailWorkerRetriesTransientFailures(t *testing.T) {
	// A 451 is the server saying "not now", so the message is worth resending.
	transient := &textproto.Error{Code: 451, Msg: "try again later"}
	recorder := newMailRecorder(t, transient, transient)
	useTestMailWorker(t, 4)

	if err := QueueMail(MailJob{To: "user@example.com", Subject: "Hi", Kind: "test"}); err != nil {
		t.Fatalf("QueueMail() error = %v", err)
	}

	recorder.waitForAttempts(t, 3)
	if got := recorder.count(); got != 3 {
		t.Fatalf("delivery attempts = %d, want 3 (two failures then a success)", got)
	}
}

func TestMailWorkerGivesUpAfterMaxAttempts(t *testing.T) {
	transient := &textproto.Error{Code: 421, Msg: "service unavailable"}
	recorder := newMailRecorder(t, transient, transient, transient, transient)
	useTestMailWorker(t, 4)

	if err := QueueMail(MailJob{To: "user@example.com", Subject: "Hi", Kind: "test"}); err != nil {
		t.Fatalf("QueueMail() error = %v", err)
	}

	recorder.waitForAttempts(t, mailMaxAttempts)

	// Give a fourth attempt a chance to happen so the cap is really tested.
	time.Sleep(50 * time.Millisecond)
	if got := recorder.count(); got != mailMaxAttempts {
		t.Fatalf("delivery attempts = %d, want %d", got, mailMaxAttempts)
	}
}

func TestMailWorkerDoesNotRetryPermanentFailures(t *testing.T) {
	rejected := &textproto.Error{Code: 550, Msg: "no such user"}
	recorder := newMailRecorder(t, rejected, rejected, rejected)
	useTestMailWorker(t, 4)

	if err := QueueMail(MailJob{To: "nobody@example.com", Subject: "Hi", Kind: "test"}); err != nil {
		t.Fatalf("QueueMail() error = %v", err)
	}

	recorder.waitForAttempts(t, 1)

	time.Sleep(50 * time.Millisecond)
	if got := recorder.count(); got != 1 {
		t.Fatalf("delivery attempts = %d, want 1; a rejected recipient must not be retried", got)
	}
}

func TestMailWorkerDoesNotRetryUnconfiguredMailer(t *testing.T) {
	recorder := newMailRecorder(t, ErrMailNotConfigured, ErrMailNotConfigured)
	useTestMailWorker(t, 4)

	if err := QueueMail(MailJob{To: "user@example.com", Subject: "Hi", Kind: "test"}); err != nil {
		t.Fatalf("QueueMail() error = %v", err)
	}

	recorder.waitForAttempts(t, 1)

	time.Sleep(50 * time.Millisecond)
	if got := recorder.count(); got != 1 {
		t.Fatalf("delivery attempts = %d, want 1; missing configuration cannot resolve itself", got)
	}
}

func TestQueueMailReportsAFullQueue(t *testing.T) {
	originalWorker := globalMailWorker

	// A worker that is never started leaves the queue unattended, which is the
	// only reliable way to fill it.
	globalMailWorker = &MailWorker{
		jobQueue: make(chan MailJob, 1),
	}
	t.Cleanup(func() { globalMailWorker = originalWorker })

	if err := QueueMail(MailJob{To: "user@example.com", Kind: "test"}); err != nil {
		t.Fatalf("QueueMail() error = %v on the first message", err)
	}
	if err := QueueMail(MailJob{To: "user@example.com", Kind: "test"}); err == nil {
		t.Error("QueueMail() accepted a message with no room in the queue")
	}
}

func TestIsPermanentMailError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"rejected recipient", &textproto.Error{Code: 550, Msg: "no such user"}, true},
		{"message refused", &textproto.Error{Code: 552, Msg: "too large"}, true},
		{"greylisted", &textproto.Error{Code: 451, Msg: "try again"}, false},
		{"service unavailable", &textproto.Error{Code: 421, Msg: "shutting down"}, false},
		{"connection failure", errors.New("dial tcp: connection refused"), false},
		{"wrapped rejection", errors.Join(errors.New("send mail"), &textproto.Error{Code: 553, Msg: "bad address"}), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPermanentMailError(tt.err); got != tt.want {
				t.Errorf("isPermanentMailError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeliverNowLogsWhenMailIsNotConfigured(t *testing.T) {
	t.Setenv("MAIL_FROM", "")
	t.Setenv("EMAIL", "")
	t.Setenv("APP_PASSWORD", "")

	// Local builds fall back to logging so link-bearing flows stay testable
	// without a mail server; this must not surface as a failure.
	if err := deliverNow(MailJob{To: "user@example.com", Subject: "Hi", Body: "body", Kind: "test"}); err != nil {
		t.Fatalf("deliverNow() error = %v, want nil in a local build", err)
	}
}
