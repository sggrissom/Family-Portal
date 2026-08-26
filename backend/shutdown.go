package backend

import (
	"context"
	"time"
)

const (
	chatShutdownTimeout = 3 * time.Second

	workerShutdownTimeout = 10 * time.Second
)

func ShutdownWorkers(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, workerShutdownTimeout)
	defer cancel()

	clean := true

	if !stopPhotoWorkerAndDrain(ctx) {
		LogWarn(LogCategoryWorker, "Photo worker still had queued photos at shutdown", map[string]interface{}{
			"queued": GetQueueLength(),
		})
		clean = false
	}
	if !stopMailWorkerAndDrain(ctx) {
		LogWarn(LogCategoryWorker, "Mail worker still had queued messages at shutdown", map[string]interface{}{
			"queued": GetMailQueueLength(),
		})
		clean = false
	}
	if !stopPushWorkerAndDrain(ctx) {
		LogWarn(LogCategoryWorker, "Push worker still had queued notifications at shutdown", nil)
		clean = false
	}
	if !StopAnalysisWorker(ctx) {
		LogWarn(LogCategoryWorker, "Face analysis worker did not stop before the deadline", nil)
		clean = false
	}

	LogInfo(LogCategoryWorker, "Background workers stopped", map[string]interface{}{
		"clean": clean,
	})
	return clean
}

func ShutdownChatConnections(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, chatShutdownTimeout)
	defer cancel()
	return ShutdownChatHub(ctx)
}
