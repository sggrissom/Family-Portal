package backend

import (
	"sync"
	"time"
)

// attemptLog is the counters and recent-attempt list a worker shows on its
// admin page.
type attemptLog[T any] struct {
	mu sync.Mutex
	attemptStats[T]
}

type attemptStats[T any] struct {
	succeeded   int
	failed      int
	lastSuccess time.Time
	lastError   string
	lastErrorAt time.Time
	recent      []T
}

func (l *attemptLog[T]) record(at time.Time, ok bool, reason string, attempt T, keep int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if ok {
		l.succeeded++
		l.lastSuccess = at
	} else {
		l.failed++
		l.lastError = reason
		l.lastErrorAt = at
	}

	l.recent = append(l.recent, attempt)
	if len(l.recent) > keep {
		l.recent = l.recent[len(l.recent)-keep:]
	}
}

// stats copies the log with the newest attempt first.
func (l *attemptLog[T]) stats() attemptStats[T] {
	l.mu.Lock()
	defer l.mu.Unlock()

	recent := make([]T, 0, len(l.recent))
	for i := len(l.recent) - 1; i >= 0; i-- {
		recent = append(recent, l.recent[i])
	}
	stats := l.attemptStats
	stats.recent = recent
	return stats
}
