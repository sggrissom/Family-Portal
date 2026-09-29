package backend

import (
	"context"
	"errors"
	"sync"
	"time"
)

// errRetryLater from a job puts it back at the front of the backlog and
// pauses the worker, doubling the pause on each consecutive failure.
var errRetryLater = errors.New("retry later")

var (
	retryPauseMin = 5 * time.Second
	retryPauseMax = 5 * time.Minute
)

// backlogWorker runs one job at a time from a deduplicated backlog, so it
// never holds more than one entry per job (a photo id). New jobs and startup
// sweeps both go through it, and nothing is dropped when uploads arrive in a
// burst.
type backlogWorker[T comparable] struct {
	workerLifecycle
	name    string
	process func(T) error

	mu      sync.Mutex
	backlog []T
	queued  map[T]bool
	wake    chan struct{}
}

func newBacklogWorker[T comparable](name string, process func(T) error) *backlogWorker[T] {
	return &backlogWorker[T]{
		name:    name,
		process: process,
		queued:  map[T]bool{},
		wake:    make(chan struct{}, 1),
	}
}

func (w *backlogWorker[T]) add(jobs ...T) {
	if len(jobs) == 0 {
		return
	}
	w.mu.Lock()
	for _, job := range jobs {
		if !w.queued[job] {
			w.queued[job] = true
			w.backlog = append(w.backlog, job)
		}
	}
	w.mu.Unlock()
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *backlogWorker[T]) pushFront(job T) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.queued[job] {
		w.queued[job] = true
		w.backlog = append([]T{job}, w.backlog...)
	}
}

func (w *backlogWorker[T]) pop() (T, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var zero T
	if len(w.backlog) == 0 {
		return zero, false
	}
	job := w.backlog[0]
	w.backlog = w.backlog[1:]
	delete(w.queued, job)
	return job, true
}

func (w *backlogWorker[T]) length() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.backlog)
}

// run starts the loop; sweep runs first, in the worker's goroutine, and its
// jobs join the backlog.
func (w *backlogWorker[T]) run(sweep func() []T) bool {
	quit, done, ok := w.start()
	if !ok {
		return false
	}
	go func() {
		defer close(done)
		if sweep != nil {
			w.add(sweep()...)
		}
		pause := time.Duration(0)
		for {
			select {
			case <-quit:
				LogInfo(LogCategoryWorker, w.name+" stopped", map[string]interface{}{"abandoned": w.length()})
				return
			default:
			}
			if job, ok := w.pop(); ok {
				if err := w.process(job); errors.Is(err, errRetryLater) {
					w.pushFront(job)
					pause = min(max(pause*2, retryPauseMin), retryPauseMax)
					select {
					case <-quit:
					case <-time.After(pause):
					}
					continue
				}
				pause = 0
				continue
			}
			select {
			case <-quit:
				LogInfo(LogCategoryWorker, w.name+" stopped", map[string]interface{}{"abandoned": w.length()})
				return
			case <-w.wake:
			}
		}
	}()
	return true
}

func (w *backlogWorker[T]) stopWait(ctx context.Context) bool {
	return w.stopAndWait(ctx, false)
}
