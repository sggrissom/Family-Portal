package backend

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const mailMaxAttempts = 3

var mailRetryDelays = []time.Duration{15 * time.Second, 60 * time.Second}

type MailWorker struct {
	workerLifecycle
	jobQueue chan MailJob
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
	if err := mailDeliverer(job); err != nil {
		LogErrorSimple(LogCategoryWorker, "Mail delivery failed during shutdown", map[string]interface{}{
			"kind":  job.Kind,
			"error": err.Error(),
		})
	}
}

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
