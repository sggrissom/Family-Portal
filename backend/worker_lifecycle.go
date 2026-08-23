package backend

import (
	"context"
	"sync"
	"time"
)

// workerLifecycle is the start/stop half of a background worker: the loop
// goroutine, the channel that tells it to finish, and the channel the shutdown
// path waits on. The four workers differ only in what they do with a job, so
// this is the part they share.
//
// Two details here are deliberate, and both were bugs before.
//
// The quit channel is *closed* rather than written to. The previous form sent
// on an unbuffered channel, which meant stopping a worker blocked until its
// loop came back around to select — behind a slow photo resize, or forever if
// the loop had already exited. A close is non-blocking and idempotent from the
// caller's side, so shutdown can signal every worker and then wait once.
//
// Each start allocates fresh channels, because a closed channel cannot be
// reopened and a worker may legitimately be stopped and started again.
// stopWaitTimeout bounds a plain Stop. It is long enough for any single job
// this application runs — the slowest is a photo resize — and short enough that
// a wedged worker cannot hang a test binary.
const stopWaitTimeout = 30 * time.Second

type workerLifecycle struct {
	mu       sync.Mutex
	running  bool
	quit     chan struct{}
	done     chan struct{}
	drainCtx context.Context
}

// start marks the worker running and hands its loop the two channels it needs.
// ok is false when the worker was already running, in which case the caller
// must not spawn a second loop.
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

// stop signals the loop to finish and returns the channel that closes when it
// has. A nil drainCtx tells the loop to abandon whatever is still queued; a
// non-nil one tells it to finish that work until the context expires.
func (l *workerLifecycle) stop(drainCtx context.Context) <-chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.running {
		// Nothing to wait for. A closed channel keeps every caller on the same
		// "wait for done" shape whether or not the worker was ever started.
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	l.running = false
	l.drainCtx = drainCtx
	close(l.quit)
	return l.done
}

// stopping returns a channel that closes when this run of the worker is asked
// to stop. A producer blocked on a full queue selects on it so that stopping the
// worker releases the producer instead of parking it on a channel nothing will
// read again.
//
// When the worker is not running the returned channel is already closed, so the
// caller takes the "give up" branch immediately rather than blocking on a nil
// channel forever.
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

// drainContext is what a loop consults after quit fires. Nil means exit now.
func (l *workerLifecycle) drainContext() context.Context {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.drainCtx
}

// stopImmediately stops the worker without draining and waits for its loop to
// exit. Stop must actually mean stopped: callers — tests especially — go on to
// close the database the loop is writing to, and returning while a job is still
// in flight turns that into a panic. The wait is bounded so a wedged job can
// still not hold a caller forever.
func (l *workerLifecycle) stopImmediately() bool {
	ctx, cancel := context.WithTimeout(context.Background(), stopWaitTimeout)
	defer cancel()
	return l.stopAndWait(ctx, false)
}

// stopAndWait stops the worker and waits for its loop to exit, giving up when
// ctx expires. It reports whether the loop finished in time — a false means the
// process is about to exit with work still in flight, which is worth a log line
// at the call site.
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

// drainQueue runs process over the jobs already sitting in queue, stopping when
// the queue empties or ctx expires. A nil ctx drains nothing: the caller asked
// for an immediate stop.
//
// It deliberately never blocks on the queue. Anything still arriving is from a
// producer that outlived the HTTP server, and waiting for it would turn a
// bounded shutdown into an unbounded one.
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
