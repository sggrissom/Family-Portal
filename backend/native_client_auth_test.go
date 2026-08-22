package backend

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

// A native client holds its credentials itself: an access token it puts in an
// Authorization header, and a refresh token it keeps in the Keychain. Neither
// of those reached the server before this change — procedures read only the
// authToken cookie or x-auth-token, and refresh and logout read only the
// refreshToken cookie — so the iOS app authenticated entirely by way of
// URLSession replaying cookies out of a jar the app does not own.
//
// The tests below are about that seam and nothing else. What counts as a valid
// credential is unchanged and covered elsewhere.

func TestBearerTokenReachesProcedureDispatch(t *testing.T) {
	_, user, token := accountTestUser(t, "bearer@example.com", "correct-horse")

	req := httptest.NewRequest(http.MethodPost, "/rpc/GetAuthContext", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+token)

	// The wrapper's whole job is to make MakeContext see the token, so that is
	// what the assertion is about — not a round trip through a real proc, which
	// would test vbeam rather than this.
	rec := httptest.NewRecorder()
	var seen string
	NewBearerTokenWrapper(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get(AuthTokenHeader)
	})).ServeHTTP(rec, req)

	if seen != token {
		t.Fatalf("%s = %q, want the bearer token", AuthTokenHeader, seen)
	}

	// And the token that arrived is the one that resolves to this user.
	app := vbeam.Application{DB: appDb}
	forwarded := req.Clone(req.Context())
	forwarded.Header.Set(AuthTokenHeader, seen)
	ctx := vbeam.MakeContext(&app, forwarded)
	defer vbeam.CloseContext(&ctx)

	got, err := GetAuthUser(&ctx)
	if err != nil {
		t.Fatalf("GetAuthUser: %v", err)
	}
	if got.Id != user.Id {
		t.Errorf("authenticated user = %d, want %d", got.Id, user.Id)
	}
}

func TestBearerTokenWrapperLeavesOtherRequestsAlone(t *testing.T) {
	cases := []struct {
		name       string
		authHeader string
		existing   string
		want       string
	}{
		{name: "no authorization header", want: ""},
		{name: "not a bearer scheme", authHeader: "Basic dXNlcjpwYXNz", want: ""},
		{name: "bearer with no token", authHeader: "Bearer ", want: ""},
		// RFC 7235 makes the scheme case-insensitive, and clients do vary.
		{name: "lowercase scheme", authHeader: "bearer abc123", want: "abc123"},
		// An explicit x-auth-token is the caller saying which credential it
		// means; the wrapper must not overrule it.
		{name: "explicit header wins", authHeader: "Bearer abc123", existing: "xyz789", want: "xyz789"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/rpc/Whatever", strings.NewReader("{}"))
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			if tc.existing != "" {
				req.Header.Set(AuthTokenHeader, tc.existing)
			}

			var seen string
			NewBearerTokenWrapper(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				seen = r.Header.Get(AuthTokenHeader)
			})).ServeHTTP(httptest.NewRecorder(), req)

			if seen != tc.want {
				t.Errorf("%s = %q, want %q", AuthTokenHeader, seen, tc.want)
			}
		})
	}
}

// nativeSession creates a user and a refresh token for them, returning the raw
// token string as a native client would have captured it at login.
func nativeSession(t *testing.T, email string) (*vbolt.DB, User, string) {
	t.Helper()
	db, user, _ := accountTestUser(t, email, "correct-horse")

	var refreshToken string
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		var err error
		_, refreshToken, err = CreateRefreshToken(tx, user.Id, refreshTokenLifetime)
		if err != nil {
			t.Fatalf("create refresh token: %v", err)
		}
		vbolt.TxCommit(tx)
	})
	return db, user, refreshToken
}

func postJSON(t *testing.T, handler http.HandlerFunc, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(encoded))
	rec := httptest.NewRecorder()
	handler(rec, req)

	var decoded map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &decoded)
	}
	return rec, decoded
}

func TestRefreshAcceptsTokenInTheBody(t *testing.T) {
	db, _, refreshToken := nativeSession(t, "native-refresh@example.com")

	rec, body := postJSON(t, refreshTokenHandler, "/api/refresh", RefreshRequest{RefreshToken: refreshToken})
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh returned %d: %s", rec.Code, rec.Body.String())
	}
	if body["success"] != true {
		t.Fatalf("expected success, got %v", body)
	}
	if body["token"] == "" || body["token"] == nil {
		t.Error("expected a new access token")
	}

	// A client with no cookie jar cannot read the rotated successor out of
	// Set-Cookie, so it has to come back in the body or the session dies at the
	// next refresh.
	rotated, ok := body["refreshToken"].(string)
	if !ok || rotated == "" {
		t.Fatal("expected the rotated refresh token in the response body")
	}
	if rotated == refreshToken {
		t.Error("refresh returned the same token; rotation did not happen")
	}

	// And the successor is the one that works from here.
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, ok := ValidateRefreshToken(tx, rotated); !ok {
			t.Error("rotated token does not validate")
		}
	})
}

func TestRefreshWithdrawsTheRotatedTokenFromBrowsers(t *testing.T) {
	_, _, refreshToken := nativeSession(t, "cookie-refresh@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/refresh", strings.NewReader("{}"))
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: refreshToken})
	rec := httptest.NewRecorder()
	refreshTokenHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("refresh returned %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	// The cookie is HttpOnly so that script cannot reach the refresh token.
	// Echoing it into the response body would undo exactly that.
	if _, present := body["refreshToken"]; present {
		t.Error("a cookie-based refresh must not put the refresh token in the response body")
	}

	var rotatedCookie string
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refreshToken" {
			rotatedCookie = c.Value
		}
	}
	if rotatedCookie == "" || rotatedCookie == refreshToken {
		t.Error("expected a rotated refreshToken cookie")
	}
}

func TestRefreshPrefersTheCookieOverTheBody(t *testing.T) {
	_, _, cookieToken := nativeSession(t, "both@example.com")

	// A stale value in a body must not be able to displace the session the
	// browser is actually holding.
	encoded, _ := json.Marshal(RefreshRequest{RefreshToken: "stale-token-from-somewhere"})
	req := httptest.NewRequest(http.MethodPost, "/api/refresh", bytes.NewReader(encoded))
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: cookieToken})
	rec := httptest.NewRecorder()
	refreshTokenHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("refresh returned %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRefreshWithNoTokenAnywhereIsUnauthorized(t *testing.T) {
	nativeSession(t, "empty@example.com")

	rec, _ := postJSON(t, refreshTokenHandler, "/api/refresh", RefreshRequest{})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRefreshRejectsAMalformedBody(t *testing.T) {
	nativeSession(t, "malformed@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/refresh", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	refreshTokenHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a malformed body, got %d", rec.Code)
	}
}

func TestLogoutRevokesARefreshTokenNamedInTheBody(t *testing.T) {
	db, _, refreshToken := nativeSession(t, "native-logout@example.com")

	rec, _ := postJSON(t, logoutHandler, "/api/logout", LogoutRequest{RefreshToken: refreshToken})
	if rec.Code != http.StatusOK {
		t.Fatalf("logout returned %d: %s", rec.Code, rec.Body.String())
	}

	// Without this the phone cleared its own storage and left a working refresh
	// token in the database for the rest of its thirty days.
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, ok := ValidateRefreshToken(tx, refreshToken); ok {
			t.Error("refresh token still validates after logout")
		}
	})
}
