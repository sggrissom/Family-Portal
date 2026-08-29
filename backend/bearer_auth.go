package backend

import (
	"net/http"
	"strings"
)

type BearerTokenWrapper struct {
	next http.Handler
}

func NewBearerTokenWrapper(next http.Handler) *BearerTokenWrapper {
	return &BearerTokenWrapper{next: next}
}

const AuthTokenHeader = "x-auth-token"

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

func (bw *BearerTokenWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(AuthTokenHeader) == "" {
		if token := bearerToken(r); token != "" {
			outer := *r
			outer.Header = r.Header.Clone()
			outer.Header.Set(AuthTokenHeader, token)
			r = &outer
		}
	}
	bw.next.ServeHTTP(w, r)
}
