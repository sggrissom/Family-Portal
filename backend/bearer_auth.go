package backend

import (
	"net/http"
	"strings"
)

// Two things read the caller's JWT, and until this wrapper existed they did not
// read the same header.
//
// `/api/*` handlers go through AuthenticateRequest, which accepts the authToken
// cookie or `Authorization: Bearer`. Procedures go through vbeam, whose
// MakeContext accepts the authToken cookie or `x-auth-token` — and nothing
// else. A native client sending a bearer token was therefore authenticated on
// login, logout, refresh, upload and download, and anonymous on every one of
// the hundred-odd procedures that carry the actual application.
//
// The iOS app works today only because URLSession replays the authToken cookie
// out of its shared cookie jar. That is a lot of weight for a store the app
// does not own: it is cleared by the system, it is per-process, and the cookie
// is Secure, so anything that reaches the server over plaintext silently loses
// its session. A bearer token in the Authorization header is the credential a
// native client actually manages, and this makes procedures accept it.
//
// The translation is a header rename and nothing more. Both paths end in
// GetAuthUser / AuthenticateRequest parsing the same signed JWT with the same
// key, so this widens which header carries the credential, not what counts as
// one.
type BearerTokenWrapper struct {
	next http.Handler
}

func NewBearerTokenWrapper(next http.Handler) *BearerTokenWrapper {
	return &BearerTokenWrapper{next: next}
}

// AuthTokenHeader is the header vbeam's MakeContext reads.
const AuthTokenHeader = "x-auth-token"

// bearerToken returns the token from an `Authorization: Bearer <token>` header,
// or an empty string. The scheme is matched case-insensitively, as RFC 7235
// requires.
func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

func (bw *BearerTokenWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// An explicit x-auth-token wins: a client that sets it has said which
	// credential it means, and overwriting that would be surprising.
	if r.Header.Get(AuthTokenHeader) == "" {
		if token := bearerToken(r); token != "" {
			// The request is shallow-copied rather than mutated, because the
			// header map belongs to the server's read of the wire and other
			// wrappers hold the same pointer.
			outer := *r
			outer.Header = r.Header.Clone()
			outer.Header.Set(AuthTokenHeader, token)
			r = &outer
		}
	}
	bw.next.ServeHTTP(w, r)
}
