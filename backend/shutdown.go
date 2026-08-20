package backend

import (
	"context"
	"time"
)

// Shutdown budget.
//
// The whole stop sequence has to fit inside systemd's TimeoutStopSec for the
// unit, or the kill arrives mid-drain and the drain was pointless. The stages
// below are sequential, so the numbers add: chat close, then the HTTP drain
// (app.go), then the workers.
const (
	// chatShutdownTimeout is how long the hub gets to send its close frames and
	// see the clients unregister. Closing a socket is fast; this is a ceiling
	// for a peer that has stopped reading, not an expected duration.
	chatShutdownTimeout = 3 * time.Second

	// workerShutdownTimeout is shared by every worker that drains, in the order
	// ShutdownWorkers runs them. Photo processing is the one that can plausibly
	// use it.
	workerShutdownTimeout = 10 * time.Second
)

// ShutdownWorkers stops the background workers, in the order of how much a
// user notices their work going missing.
//
// Photo processing goes first and gets the largest share of the budget: a
// queued job holds the upload's bytes in memory and its row already reads
// "processing", so abandoning it strands a photo nothing retries. Mail is next,
// because a queued password reset is somebody's way back into their account.
// Push is best-effort but cheap — one HTTPS call each. Face analysis is stopped
// without draining at all; see StopAnalysisWorker for why.
//
// Every stage shares one deadline, so a worker that will not finish cannot
// spend the next worker's budget. Reports whether everything stopped cleanly.
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

// ShutdownChatConnections closes the open chat sockets with a Going Away frame.
// It must run before the HTTP server drains, because an upgraded connection is
// hijacked and http.Server.Shutdown neither tracks nor waits for those.
func ShutdownChatConnections(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, chatShutdownTimeout)
	defer cancel()
	return ShutdownChatHub(ctx)
}
