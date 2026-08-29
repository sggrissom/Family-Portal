package backend

import (
	"net/http"
	"strings"
	"time"
)

const (
	defaultReadTimeout = 30 * time.Second

	defaultWriteTimeout = 2 * time.Minute

	uploadReadTimeout = 10 * time.Minute

	importReadTimeout = 30 * time.Minute

	downloadWriteTimeout = 30 * time.Minute
)

func requestDeadlines(r *http.Request) (read, write time.Duration) {
	if isWebSocketRequest(r) {
		return 0, 0
	}

	switch r.URL.Path {
	case "/api/upload-photo":
		return uploadReadTimeout, defaultWriteTimeout
	case "/api/import-bundle":
		return importReadTimeout, defaultWriteTimeout
	case "/api/export-bundle", SnapshotPath:
		return defaultReadTimeout, downloadWriteTimeout
	}

	if strings.HasPrefix(r.URL.Path, "/api/photo/") {
		return defaultReadTimeout, downloadWriteTimeout
	}

	return defaultReadTimeout, defaultWriteTimeout
}

type RequestTimeoutWrapper struct {
	next http.Handler
}

func NewRequestTimeoutWrapper(next http.Handler) *RequestTimeoutWrapper {
	return &RequestTimeoutWrapper{next: next}
}

func (tw *RequestTimeoutWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	read, write := requestDeadlines(r)
	controller := http.NewResponseController(w)

	now := time.Now()
	if read == 0 {
		_ = controller.SetReadDeadline(time.Time{})
	} else {
		_ = controller.SetReadDeadline(now.Add(read))
	}
	if write == 0 {
		_ = controller.SetWriteDeadline(time.Time{})
	} else {
		_ = controller.SetWriteDeadline(now.Add(write))
	}

	tw.next.ServeHTTP(w, r)
}
