package backend

import (
	"context"
	"sync"
	"time"
)

const stopWaitTimeout = 30 * time.Second

type workerLifecycle struct {
	mu       sync.Mutex
	running  bool
	quit     chan struct{}
	done     chan struct{}
	drainCtx context.Context
}

func (l *workerLifecycle) start() (quit <-chan struct{}, done chan struct{}, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running {
		return nil, nil, false
	}
	l.running = true
	l.quit = make(chan struct{})
	l.done = make(chan struct{})
	l.drainCtx = nil
	return l.quit, l.done, true
}

func (l *workerLifecycle) isRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

func (l *workerLifecycle) stop(drainCtx context.Context) <-chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.running {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	l.running = false
	l.drainCtx = drainCtx
	close(l.quit)
	return l.done
}

func (l *workerLifecycle) stopping() <-chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.running {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return l.quit
}

func (l *workerLifecycle) drainContext() context.Context {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.drainCtx
}

func (l *workerLifecycle) stopImmediately() bool {
	ctx, cancel := context.WithTimeout(context.Background(), stopWaitTimeout)
	defer cancel()
	return l.stopAndWait(ctx, false)
}

func (l *workerLifecycle) stopAndWait(ctx context.Context, drain bool) bool {
	var drainCtx context.Context
	if drain {
		drainCtx = ctx
	}
	done := l.stop(drainCtx)
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}

func drainQueue[T any](ctx context.Context, queue <-chan T, process func(T)) int {
	if ctx == nil {
		return 0
	}
	drained := 0
	for {
		if ctx.Err() != nil {
			return drained
		}
		select {
		case job := <-queue:
			process(job)
			drained++
		default:
			return drained
		}
	}
}
