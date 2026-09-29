package backend

import (
	"context"
	"sync"
)

// backlogWorker runs one job at a time from an unbounded backlog. New jobs
// and startup sweeps both go through the backlog, so nothing is dropped when
// a burst of uploads arrives.
type backlogWorker[T comparable] struct {
	workerLifecycle
	name    string
	process func(T)

	mu      sync.Mutex
	backlog []T
	queued  map[T]bool
	wake    chan struct{}
}

func newBacklogWorker[T comparable](name string, process func(T)) *backlogWorker[T] {
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
		for {
			select {
			case <-quit:
				LogInfo(LogCategoryWorker, w.name+" stopped", map[string]interface{}{"abandoned": w.length()})
				return
			default:
			}
			if job, ok := w.pop(); ok {
				w.process(job)
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
