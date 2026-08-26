package backend

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type testWorker struct {
	workerLifecycle
	jobQueue  chan int
	processed atomic.Int32
	block     chan struct{}
}

func newTestWorker(queueSize int) *testWorker {
	return &testWorker{
		jobQueue: make(chan int, queueSize),
		block:    make(chan struct{}),
	}
}

func (w *testWorker) Start() {
	quit, done, ok := w.start()
	if !ok {
		return
	}
	go func() {
		defer close(done)
		for {
			select {
			case job := <-w.jobQueue:
				w.process(job)
			case <-quit:
				drainQueue(w.drainContext(), w.jobQueue, w.process)
				return
			}
		}
	}()
}

func (w *testWorker) process(int) {
	<-w.block
	w.processed.Add(1)
}

func TestStopDoesNotBlockOnABusyWorker(t *testing.T) {
	worker := newTestWorker(4)
	worker.Start()

	worker.jobQueue <- 1
	waitFor(t, func() bool { return len(worker.jobQueue) == 0 })

	stopped := make(chan bool, 1)
	go func() { stopped <- worker.stopAndWait(shortCtx(t, 200*time.Millisecond), false) }()

	select {
	case clean := <-stopped:
		if clean {
			t.Error("stopAndWait reported a clean stop while a job was still running")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stopAndWait blocked behind a busy worker")
	}

	close(worker.block)
}

func TestStopWithDrainFinishesQueuedJobs(t *testing.T) {
	worker := newTestWorker(4)
	close(worker.block)
	worker.Start()

	for i := 0; i < 3; i++ {
		worker.jobQueue <- i
	}

	if !worker.stopAndWait(shortCtx(t, 2*time.Second), true) {
		t.Fatal("worker did not stop within the drain budget")
	}
	if got := worker.processed.Load(); got != 3 {
		t.Errorf("processed = %d, want 3; a drain must not abandon accepted work", got)
	}
	if len(worker.jobQueue) != 0 {
		t.Errorf("queue still holds %d jobs after a drain", len(worker.jobQueue))
	}
}

func TestStoppingAWorkerTwiceIsSafe(t *testing.T) {
	worker := newTestWorker(1)
	close(worker.block)
	worker.Start()

	if !worker.stopAndWait(shortCtx(t, time.Second), false) {
		t.Fatal("first stop did not complete")
	}
	if !worker.stopAndWait(shortCtx(t, time.Second), false) {
		t.Fatal("second stop did not complete")
	}
}

func TestStoppingAWorkerThatNeverStartedIsSafe(t *testing.T) {
	worker := newTestWorker(1)
	if !worker.stopAndWait(shortCtx(t, time.Second), true) {
		t.Fatal("stopping an unstarted worker should report clean immediately")
	}
}

func TestAWorkerCanBeRestarted(t *testing.T) {
	worker := newTestWorker(2)
	close(worker.block)
	worker.Start()
	worker.stopAndWait(shortCtx(t, time.Second), false)

	worker.Start()
	if !worker.isRunning() {
		t.Fatal("worker did not restart")
	}
	worker.jobQueue <- 1
	waitFor(t, func() bool { return worker.processed.Load() >= 1 })
	worker.stopAndWait(shortCtx(t, time.Second), false)
}

func TestDrainQueueHonorsItsDeadline(t *testing.T) {
	queue := make(chan int, 3)
	for i := 0; i < 3; i++ {
		queue <- i
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if drained := drainQueue(ctx, queue, func(int) {}); drained != 0 {
		t.Errorf("drained = %d, want 0 from an expired context", drained)
	}
	if len(queue) != 3 {
		t.Errorf("queue = %d, want 3; an expired drain must not consume jobs", len(queue))
	}
}

func TestDrainQueueWithoutAContextDrainsNothing(t *testing.T) {
	queue := make(chan int, 2)
	queue <- 1

	if drained := drainQueue(nil, queue, func(int) {}); drained != 0 {
		t.Errorf("drained = %d, want 0; a nil context means an immediate stop", drained)
	}
}

func shortCtx(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition did not hold within 2s")
}
