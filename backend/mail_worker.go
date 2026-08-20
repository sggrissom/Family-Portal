package backend

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Outbound mail is transactional and low volume, so a failed send is worth a
// few attempts: a relay that is reloading or greylisting recovers in seconds,
// and the alternative is a user who never receives the reset link they asked
// for. Attempts are spaced apart rather than immediate because an instant retry
// tends to meet the same condition that caused the failure.
const mailMaxAttempts = 3

var mailRetryDelays = []time.Duration{15 * time.Second, 60 * time.Second}

// MailWorker delivers queued messages on a background goroutine so an
// unresponsive relay costs a message its latency rather than costing a user
// their HTTP request.
type MailWorker struct {
	workerLifecycle
	jobQueue chan MailJob
}

var globalMailWorker *MailWorker

// mailDeliverer performs one delivery attempt. It is a variable so tests can
// exercise queueing and retry behaviour without an SMTP server.
var mailDeliverer = deliverNow

// InitializeMailWorker starts the background mail worker. Unlike the push
// worker there is no configuration to validate here: whether mail can be sent
// at all depends on environment that resolveMailSettings reads per send, and a
// relay that is down at startup may well be up by the first message.
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

// QueueMail hands a message to the background worker, or sends it inline when
// no worker is running. The fallback is what keeps tests and one-off tooling
// working without a worker goroutine, and it is safe there precisely because
// those callers are not serving a request.
func QueueMail(job MailJob) error {
	if globalMailWorker == nil {
		return mailDeliverer(job)
	}

	select {
	case globalMailWorker.jobQueue <- job:
		return nil
	default:
		// Deliberately not falling back to a synchronous send: a full queue
		// means deliveries are already backing up, so sending inline would
		// hand the caller the stall the queue exists to absorb.
		LogErrorSimple(LogCategoryWorker, "Mail queue is full; message dropped", map[string]interface{}{
			"kind": job.Kind,
		})
		return fmt.Errorf("mail queue is full")
	}
}

// Start begins the background worker goroutine.
func (mw *MailWorker) Start() {
	quit, done, ok := mw.start()
	if !ok {
		return
	}

	go mw.processJobs(quit, done)
}

// Stop signals the worker to exit, abandoning anything still queued.
func (mw *MailWorker) Stop() {
	mw.stopImmediately()
	LogInfo(LogCategoryWorker, "Mail worker stopped")
}

// StopAndDrain stops the worker and makes one last attempt at each message
// already queued. Password reset is the only way a locked-out user gets back in
// without me, so a reset accepted a moment before a deploy is worth the attempt
// — but only one, since the process is on its way out and a backoff would just
// burn the shutdown budget.
func (mw *MailWorker) StopAndDrain(ctx context.Context) bool {
	return mw.stopAndWait(ctx, true)
}

// processJobs is the main worker loop.
func (mw *MailWorker) processJobs(quit <-chan struct{}, done chan struct{}) {
	defer close(done)
	for {
		select {
		case job := <-mw.jobQueue:
			// A stop signal can also arrive mid-delivery, during a retry
			// backoff. Returning here is what makes it take effect; looping
			// would leave the goroutine running with the signal consumed.
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

// deliverFinal makes a single attempt at a message during shutdown. It never
// retries: the point of draining is to not lose a queued reset link, not to
// keep the process alive through a backoff.
func (mw *MailWorker) deliverFinal(job MailJob) {
	if err := mailDeliverer(job); err != nil {
		LogErrorSimple(LogCategoryWorker, "Mail delivery failed during shutdown", map[string]interface{}{
			"kind":  job.Kind,
			"error": err.Error(),
		})
	}
}

// deliver sends one job, retrying transient failures. It reports false when the
// worker was stopped before the job was resolved, which is the caller's signal
// to shut the loop down.
//
// Retries run on this goroutine, so a message being retried holds up the ones
// behind it; at the volume of mail this application sends that is a fair trade
// for not needing a durable queue, but it is the reason the backoff is measured
// in seconds rather than minutes.
func (mw *MailWorker) deliver(job MailJob, quit <-chan struct{}) bool {
	for attempt := 1; attempt <= mailMaxAttempts; attempt++ {
		err := mailDeliverer(job)
		if err == nil {
			LogInfo(LogCategoryWorker, "Mail sent", map[string]interface{}{
				"kind":    job.Kind,
				"attempt": attempt,
			})
			return true
		}

		// A rejected recipient or an unconfigured mailer will fail identically
		// on every attempt, so stop rather than spending the backoff on it.
		permanent := isPermanentMailError(err) || errors.Is(err, ErrMailNotConfigured)
		if permanent || attempt == mailMaxAttempts {
			LogErrorSimple(LogCategoryWorker, "Mail delivery failed", map[string]interface{}{
				"kind":      job.Kind,
				"attempts":  attempt,
				"permanent": permanent,
				"error":     err.Error(),
			})
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
			return false
		}
	}

	return true
}

// wait sleeps between attempts, reporting false if the worker was stopped
// first. Waiting on the stop channel keeps shutdown from blocking for the
// length of a backoff.
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

// GetMailQueueLength returns the current number of messages awaiting delivery.
func GetMailQueueLength() int {
	if globalMailWorker == nil {
		return 0
	}
	return len(globalMailWorker.jobQueue)
}

// StopMailWorker gracefully shuts down the global mail worker.
func StopMailWorker() {
	if globalMailWorker != nil {
		globalMailWorker.Stop()
	}
}

// stopMailWorkerAndDrain is the shutdown path's entry point.
func stopMailWorkerAndDrain(ctx context.Context) bool {
	if globalMailWorker == nil {
		return true
	}
	return globalMailWorker.StopAndDrain(ctx)
}
