package backend

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestBacklogWorkerDedupesAndRetriesInOrder(t *testing.T) {
	retryPauseMin, retryPauseMax = time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() { retryPauseMin, retryPauseMax = 5*time.Second, 5*time.Minute })

	var mu sync.Mutex
	var seen []int
	failures := 2
	done := make(chan struct{})
	w := newBacklogWorker("test worker", func(job int) error {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, job)
		if job == 1 && failures > 0 {
			failures--
			return errRetryLater
		}
		if job == 3 {
			close(done)
		}
		return nil
	})
	w.add(1, 2, 2, 1)
	w.run(func() []int { return []int{3} })

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not finish")
	}
	w.stopWait(context.Background())

	mu.Lock()
	defer mu.Unlock()
	want := []int{1, 1, 1, 2, 3}
	if len(seen) != len(want) {
		t.Fatalf("processed %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("processed %v, want %v", seen, want)
		}
	}
}
