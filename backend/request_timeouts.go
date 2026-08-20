package backend

import (
	"net/http"
	"strings"
	"time"
)

// Per-request read and write deadlines.
//
// Why these are not http.Server.ReadTimeout and WriteTimeout
// ----------------------------------------------------------
// Those two apply to every connection the server accepts, including the ones a
// WebSocket upgrade hijacks. coder/websocket does not clear the deadlines
// net/http set before the hijack, so a global WriteTimeout would sever every
// chat connection the moment it elapsed — mid-conversation, with nothing a user
// could do about it. And a single global read budget cannot be both short
// enough to matter for a JSON call and long enough for a 512 MiB import.
//
// So the server leaves both unset (app.go) and the deadlines are applied here,
// per request, from the budget the route actually needs. http.ResponseController
// reaches through the middleware chain to the underlying connection.
//
// The budgets are sized against the body limits in security.go: a route that
// accepts 512 MiB has to allow for a slow uplink sending 512 MiB, or the limit
// is unreachable in practice and the timeout becomes the real limit.
const (
	// defaultReadTimeout bounds an ordinary request body — an RPC call, a login,
	// a form post. All of them are under a megabyte.
	defaultReadTimeout = 30 * time.Second

	// defaultWriteTimeout bounds the response. It has to exceed the slowest
	// handler, not just the slowest write: net/http writes the response after
	// the handler returns, so a deadline shorter than the handler's own runtime
	// turns a slow success into an empty reply. The slowest is AI import, whose
	// Gemini client gives up at 60 seconds (ai.go).
	defaultWriteTimeout = 2 * time.Minute

	// uploadReadTimeout covers a photo upload: up to 50 MiB from a phone, which
	// at a genuinely bad 200 KB/s of mobile data is a little over four minutes.
	uploadReadTimeout = 10 * time.Minute

	// importReadTimeout covers a full-family archive, ten times the size. The
	// assumption behind it is different: an archive is uploaded from a computer
	// that already has the file on disk, so the floor is broadband rather than
	// mobile data. Sizing this for a phone would mean a read deadline of most of
	// an hour, to cover a case that does not happen.
	importReadTimeout = 30 * time.Minute

	// downloadWriteTimeout covers responses that stream a file back: photo
	// variants, family exports, and the database snapshot the nightly backup
	// pulls.
	downloadWriteTimeout = 30 * time.Minute
)

// requestDeadlines returns the read and write budgets for a request. A zero
// duration means "no deadline", which is what a hijacked WebSocket needs.
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

// RequestTimeoutWrapper applies those deadlines. It sits outside the handlers
// and inside the rate limiter, so a refused flood never reaches it.
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
	// A zero time clears the deadline rather than setting one in the past, which
	// is exactly what the WebSocket case wants. Both calls return
	// ErrNotSupported on a ResponseWriter with no connection behind it —
	// httptest, most notably — and there is nothing to do about that but carry
	// on serving.
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
