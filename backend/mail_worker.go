package backend

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const mailMaxAttempts = 3

const maxRecentMailAttempts = 20

var mailRetryDelays = []time.Duration{15 * time.Second, 60 * time.Second}

type MailAttempt struct {
	Time      time.Time `json:"time"`
	Kind      string    `json:"kind"`
	To        string    `json:"to"`
	Success   bool      `json:"success"`
	Attempts  int       `json:"attempts"`
	Permanent bool      `json:"permanent"`
	Error     string    `json:"error"`
}

type MailWorker struct {
	workerLifecycle
	jobQueue chan MailJob

	statsMu   sync.Mutex
	sent      int
	failed    int
	lastSent  time.Time
	lastError string
	lastErrAt time.Time
	recent    []MailAttempt
}

func (mw *MailWorker) recordAttempt(attempt MailAttempt) {
	mw.statsMu.Lock()
	defer mw.statsMu.Unlock()

	if attempt.Success {
		mw.sent++
		mw.lastSent = attempt.Time
	} else {
		mw.failed++
		mw.lastError = attempt.Error
		mw.lastErrAt = attempt.Time
	}

	mw.recent = append(mw.recent, attempt)
	if len(mw.recent) > maxRecentMailAttempts {
		mw.recent = mw.recent[len(mw.recent)-maxRecentMailAttempts:]
	}
}

var globalMailWorker *MailWorker

var mailDeliverer = deliverNow

func InitializeMailWorker(queueSize int) {
	if globalMailWorker != nil {
		LogInfo(LogCategoryWorker, "Mail worker already initialized, skipping")
		return
	}

	globalMailWorker = &MailWorker{
		jobQueue: make(chan MailJob, queueSize),
	}

	globalMailWorker.Start()
	LogInfo(LogCategoryWorker, "Mail worker started", map[string]interface{}{
		"queueSize": queueSize,
	})
}

func QueueMail(job MailJob) error {
	if globalMailWorker == nil {
		return mailDeliverer(job)
	}

	select {
	case globalMailWorker.jobQueue <- job:
		return nil
	default:
		LogErrorSimple(LogCategoryWorker, "Mail queue is full; message dropped", map[string]interface{}{
			"kind": job.Kind,
		})
		globalMailWorker.recordAttempt(MailAttempt{
			Time:      time.Now(),
			Kind:      job.Kind,
			To:        job.To,
			Permanent: true,
			Error:     "the mail queue was full, so the message was dropped without being sent",
		})
		return fmt.Errorf("mail queue is full")
	}
}

func (mw *MailWorker) Start() {
	quit, done, ok := mw.start()
	if !ok {
		return
	}

	go mw.processJobs(quit, done)
}

func (mw *MailWorker) Stop() {
	mw.stopImmediately()
	LogInfo(LogCategoryWorker, "Mail worker stopped")
}

func (mw *MailWorker) StopAndDrain(ctx context.Context) bool {
	return mw.stopAndWait(ctx, true)
}

func (mw *MailWorker) processJobs(quit <-chan struct{}, done chan struct{}) {
	defer close(done)
	for {
		select {
		case job := <-mw.jobQueue:
			if !mw.deliver(job, quit) {
				drained := drainQueue(mw.drainContext(), mw.jobQueue, mw.deliverFinal)
				LogInfo(LogCategoryWorker, "Mail worker stopped during delivery", map[string]interface{}{
					"drained": drained,
				})
				return
			}
		case <-quit:
			drained := drainQueue(mw.drainContext(), mw.jobQueue, mw.deliverFinal)
			LogInfo(LogCategoryWorker, "Mail worker received stop signal", map[string]interface{}{
				"drained":   drained,
				"abandoned": len(mw.jobQueue),
			})
			return
		}
	}
}

func (mw *MailWorker) deliverFinal(job MailJob) {
	err := mailDeliverer(job)
	if err != nil {
		LogErrorSimple(LogCategoryWorker, "Mail delivery failed during shutdown", map[string]interface{}{
			"kind":  job.Kind,
			"error": err.Error(),
		})
	}
	mw.recordAttempt(mailAttemptFor(job, 1, err))
}

func (mw *MailWorker) deliver(job MailJob, quit <-chan struct{}) bool {
	for attempt := 1; attempt <= mailMaxAttempts; attempt++ {
		err := mailDeliverer(job)
		if err == nil {
			LogInfo(LogCategoryWorker, "Mail sent", map[string]interface{}{
				"kind":    job.Kind,
				"attempt": attempt,
			})
			mw.recordAttempt(mailAttemptFor(job, attempt, nil))
			return true
		}

		permanent := isPermanentMailError(err) || errors.Is(err, ErrMailNotConfigured)
		if permanent || attempt == mailMaxAttempts {
			LogErrorSimple(LogCategoryWorker, "Mail delivery failed", map[string]interface{}{
				"kind":      job.Kind,
				"attempts":  attempt,
				"permanent": permanent,
				"error":     err.Error(),
			})
			failure := mailAttemptFor(job, attempt, err)
			failure.Permanent = permanent
			mw.recordAttempt(failure)
			return true
		}

		LogWarn(LogCategoryWorker, "Mail delivery attempt failed; will retry", map[string]interface{}{
			"kind":    job.Kind,
			"attempt": attempt,
			"error":   err.Error(),
		})

		if !mw.wait(mailRetryDelays[attempt-1], quit) {
			LogInfo(LogCategoryWorker, "Mail worker stopping; abandoning retry", map[string]interface{}{
				"kind": job.Kind,
			})
			abandoned := mailAttemptFor(job, attempt, err)
			abandoned.Error = "the worker stopped before the retry, so this was never sent: " + err.Error()
			mw.recordAttempt(abandoned)
			return false
		}
	}

	return true
}

func (mw *MailWorker) wait(delay time.Duration, quit <-chan struct{}) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-quit:
		return false
	}
}

func mailAttemptFor(job MailJob, attempts int, err error) MailAttempt {
	attempt := MailAttempt{
		Time:     time.Now(),
		Kind:     job.Kind,
		To:       job.To,
		Success:  err == nil,
		Attempts: attempts,
	}
	if err != nil {
		attempt.Error = err.Error()
	}
	return attempt
}

type MailWorkerStats struct {
	QueueLength    int           `json:"queueLength"`
	IsRunning      bool          `json:"isRunning"`
	Sent           int           `json:"sent"`
	Failed         int           `json:"failed"`
	LastSentAt     time.Time     `json:"lastSentAt"`
	LastError      string        `json:"lastError"`
	LastErrorAt    time.Time     `json:"lastErrorAt"`
	RecentAttempts []MailAttempt `json:"recentAttempts"`
}

func GetMailWorkerStats() MailWorkerStats {
	if globalMailWorker == nil {
		return MailWorkerStats{RecentAttempts: []MailAttempt{}}
	}

	mw := globalMailWorker
	mw.statsMu.Lock()
	defer mw.statsMu.Unlock()

	recent := make([]MailAttempt, 0, len(mw.recent))
	for i := len(mw.recent) - 1; i >= 0; i-- {
		recent = append(recent, mw.recent[i])
	}

	return MailWorkerStats{
		QueueLength:    len(mw.jobQueue),
		IsRunning:      mw.isRunning(),
		Sent:           mw.sent,
		Failed:         mw.failed,
		LastSentAt:     mw.lastSent,
		LastError:      mw.lastError,
		LastErrorAt:    mw.lastErrAt,
		RecentAttempts: recent,
	}
}

func GetMailQueueLength() int {
	if globalMailWorker == nil {
		return 0
	}
	return len(globalMailWorker.jobQueue)
}

func StopMailWorker() {
	if globalMailWorker != nil {
		globalMailWorker.Stop()
	}
}

func stopMailWorkerAndDrain(ctx context.Context) bool {
	if globalMailWorker == nil {
		return true
	}
	return globalMailWorker.StopAndDrain(ctx)
}
