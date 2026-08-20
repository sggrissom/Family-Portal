package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// RequestIDHeader carries the correlation id back to the client. Every response
// gets one, not just failures, so a user reporting "the page was slow" can be
// tied to a log line as easily as one reporting an error.
const RequestIDHeader = "X-Request-Id"

type requestIDContextKey struct{}

// NewRequestID returns a fresh correlation id: twelve lowercase hex characters.
//
// Short enough to read out loud or retype from a screenshot, long enough that
// two requests in the same log file will not collide. It is not a secret and
// carries no information about the request — it is a join key, nothing more.
func NewRequestID() string {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// A failing CSPRNG is not a reason to fail the request; a repeated
		// correlation id makes for a worse log, not a broken response.
		return "000000000000"
	}
	return hex.EncodeToString(buf[:])
}

// RequestIDWrapper assigns every request an id and puts it on the response.
type RequestIDWrapper struct {
	next http.Handler
}

func NewRequestIDWrapper(next http.Handler) *RequestIDWrapper {
	return &RequestIDWrapper{next: next}
}

func (rw *RequestIDWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := NewRequestID()
	w.Header().Set(RequestIDHeader, id)
	rw.next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, id)))
}

// RequestID returns the correlation id for a request, or "" if the request did
// not pass through the wrapper — which in practice means a unit test.
func RequestID(r *http.Request) string {
	if r == nil {
		return ""
	}
	id, _ := r.Context().Value(requestIDContextKey{}).(string)
	return id
}
