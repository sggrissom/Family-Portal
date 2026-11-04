package backend

import (
	"context"
	"time"
)

const (
	chatShutdownTimeout = 3 * time.Second

	workerShutdownTimeout = 10 * time.Second
)

var workerShutdownOrder = []struct {
	stuck  string
	stop   func(context.Context) bool
	queued func() int
}{
	{"Photo worker still had queued photos at shutdown", func(ctx context.Context) bool {
		return globalPhotoWorker == nil || globalPhotoWorker.StopAndDrain(ctx)
	}, GetQueueLength},
	{"Mail worker still had queued messages at shutdown", func(ctx context.Context) bool {
		return globalMailWorker == nil || globalMailWorker.StopAndDrain(ctx)
	}, GetMailQueueLength},
	{"Push worker still had queued notifications at shutdown", func(ctx context.Context) bool {
		return globalPushWorker == nil || globalPushWorker.StopAndDrain(ctx)
	}, nil},
	{"Health monitor did not stop before the deadline", StopHealthMonitor, nil},
	{"Face analysis worker did not stop before the deadline", StopAnalysisWorker, nil},
	{"Milestone worker did not stop before the deadline", StopMilestoneWorker, nil},
	{"Vision worker did not stop before the deadline", StopVisionWorker, nil},
	{"Video analysis worker did not stop before the deadline", StopVideoAnalysisWorker, nil},
	{"Video worker did not stop before the deadline", StopVideoWorker, nil},
	{"Media janitor did not stop before the deadline", StopMediaJanitor, nil},
	{"Photo features worker did not stop before the deadline", StopFeaturesWorker, nil},
}

func ShutdownWorkers(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, workerShutdownTimeout)
	defer cancel()

	clean := true
	for _, worker := range workerShutdownOrder {
		if worker.stop(ctx) {
			continue
		}
		var data interface{}
		if worker.queued != nil {
			data = map[string]interface{}{"queued": worker.queued()}
		}
		LogWarn(LogCategoryWorker, worker.stuck, data)
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
