package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const RequestIDHeader = "X-Request-Id"

type requestIDContextKey struct{}

func NewRequestID() string {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "000000000000"
	}
	return hex.EncodeToString(buf[:])
}

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

func RequestID(r *http.Request) string {
	if r == nil {
		return ""
	}
	id, _ := r.Context().Value(requestIDContextKey{}).(string)
	return id
}
