package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

/* ---------- fixtures ---------- */

// appleFormRecorder stands in for one of Apple's form-post endpoints and keeps
// every request it was sent, so a test can assert on what was spent where.
type appleFormRecorder struct {
	mu       sync.Mutex
	requests []url.Values
	status   int
	body     string
}

func startAppleFormEndpoint(t *testing.T, endpoint *string, status int, body string) *appleFormRecorder {
	t.Helper()

	recorder := &appleFormRecorder{status: status, body: body}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		recorder.mu.Lock()
		recorder.requests = append(recorder.requests, r.PostForm)
		recorder.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(recorder.status)
		fmt.Fprint(w, recorder.body)
	}))
	t.Cleanup(server.Close)

	original := *endpoint
	*endpoint = server.URL
	t.Cleanup(func() { *endpoint = original })

	return recorder
}

func (rec *appleFormRecorder) calls() []url.Values {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return append([]url.Values(nil), rec.requests...)
}

func storedAppleToken(t *testing.T, userId int) AppleRefreshToken {
	t.Helper()

	var record AppleRefreshToken
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		record = GetAppleRefreshToken(tx, userId)
	})
	return record
}

/* ---------- capture at sign-in ---------- */

func TestAppleTokenLoginStoresARevocableRefreshToken(t *testing.T) {
	configureAppleForTest(t)
	appleTestDB(t)
	key := startAppleKeyServer(t)

	tokens := startAppleFormEndpoint(t, &appleTokenEndpoint, http.StatusOK,
		`{"id_token":"unused","refresh_token":"apple-refresh-token"}`)

	claims := appleClaims("app.familyrecord.ios")
	claims["email"] = "native@example.com"
	body, _ := json.Marshal(AppleTokenLoginRequest{
		IDToken:           signAppleIDToken(t, key, claims),
		Name:              "Ada Lovelace",
		AuthorizationCode: "the-native-code",
	})

	resp := decodeLoginResponse(t, appleTokenLoginRequest(string(body)))
	if !resp.Success {
		t.Fatalf("response = %+v, want success", resp)
	}

	calls := tokens.calls()
	if len(calls) != 1 {
		t.Fatalf("token endpoint calls = %d, want 1", len(calls))
	}
	if got := calls[0].Get("code"); got != "the-native-code" {
		t.Errorf("code = %q", got)
	}
	if got := calls[0].Get("client_id"); got != "app.familyrecord.ios" {
		t.Errorf("client_id = %q, want the bundle id the code was issued to", got)
	}
	if got := calls[0].Get("redirect_uri"); got != "" {
		t.Errorf("redirect_uri = %q, want none for a native authorization", got)
	}
	if got := calls[0].Get("client_secret"); got == "" {
		t.Error("client_secret was not sent")
	}

	record := storedAppleToken(t, resp.Auth.Id)
	if record.Token != "apple-refresh-token" {
		t.Errorf("stored token = %q", record.Token)
	}
	if record.ClientId != "app.familyrecord.ios" {
		t.Errorf("stored client id = %q, want the client Apple will accept the token back from", record.ClientId)
	}
	if record.UserId != resp.Auth.Id {
		t.Errorf("stored user id = %d, want %d", record.UserId, resp.Auth.Id)
	}
}

func TestAppleTokenLoginSucceedsWithoutAnAuthorizationCode(t *testing.T) {
	configureAppleForTest(t)
	appleTestDB(t)
	key := startAppleKeyServer(t)

	tokens := startAppleFormEndpoint(t, &appleTokenEndpoint, http.StatusOK, `{}`)

	claims := appleClaims("app.familyrecord.ios")
	claims["email"] = "older-build@example.com"
	body, _ := json.Marshal(AppleTokenLoginRequest{IDToken: signAppleIDToken(t, key, claims)})

	resp := decodeLoginResponse(t, appleTokenLoginRequest(string(body)))
	if !resp.Success {
		t.Fatalf("response = %+v, want an app that sends no code to still sign in", resp)
	}
	if len(tokens.calls()) != 0 {
		t.Error("there is no code to exchange, so Apple should not have been called")
	}
	if record := storedAppleToken(t, resp.Auth.Id); record.Token != "" {
		t.Errorf("stored token = %q, want none", record.Token)
	}
}

func TestAppleTokenLoginSurvivesAFailedExchange(t *testing.T) {
	configureAppleForTest(t)
	appleTestDB(t)
	key := startAppleKeyServer(t)

	startAppleFormEndpoint(t, &appleTokenEndpoint, http.StatusBadRequest, `{"error":"invalid_grant"}`)

	claims := appleClaims("app.familyrecord.ios")
	claims["email"] = "unlucky@example.com"
	body, _ := json.Marshal(AppleTokenLoginRequest{
		IDToken:           signAppleIDToken(t, key, claims),
		AuthorizationCode: "spent-already",
	})

	resp := decodeLoginResponse(t, appleTokenLoginRequest(string(body)))
	if !resp.Success {
		t.Fatalf("response = %+v: the identity token already proved who this is, so a refused exchange must not block sign-in", resp)
	}
	if record := storedAppleToken(t, resp.Auth.Id); record.Token != "" {
		t.Errorf("stored token = %q, want none after a refused exchange", record.Token)
	}
}

func TestAppleCallbackStoresTheRefreshTokenFromTheWebFlow(t *testing.T) {
	configureAppleForTest(t)
	appleTestDB(t)
	key := startAppleKeyServer(t)

	claims := appleClaims("app.familyrecord.web")
	claims["email"] = "browser@example.com"
	claims["nonce"] = "nonce123"
	idToken := signAppleIDToken(t, key, claims)

	tokens := startAppleFormEndpoint(t, &appleTokenEndpoint, http.StatusOK,
		fmt.Sprintf(`{"id_token":%q,"refresh_token":"web-refresh-token"}`, idToken))

	rec := httptest.NewRecorder()
	appleCallbackHandler(rec, appleCallbackRequest(
		url.Values{"state": {"expected"}, "code": {"the-code"}}, "expected.nonce123"))

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d (%s), want %d", rec.Code, rec.Body.String(), http.StatusFound)
	}

	calls := tokens.calls()
	if len(calls) != 1 {
		t.Fatalf("token endpoint calls = %d, want 1", len(calls))
	}
	if got := calls[0].Get("redirect_uri"); got != appleWebConfig.RedirectURL {
		t.Errorf("redirect_uri = %q, want the one the browser was sent to", got)
	}

	var userId int
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		userId = GetUserId(tx, "browser@example.com")
	})
	record := storedAppleToken(t, userId)
	if record.Token != "web-refresh-token" {
		t.Errorf("stored token = %q", record.Token)
	}
	if record.ClientId != "app.familyrecord.web" {
		t.Errorf("stored client id = %q, want the services id", record.ClientId)
	}
}

/* ---------- revocation at deletion ---------- */

func seedAppleToken(t *testing.T, userId int, clientId, token string) {
	t.Helper()

	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		storeAppleRefreshTokenTx(tx, userId, clientId, token, time.Now())
		vbolt.TxCommit(tx)
	})
}

func TestDeleteAccountRevokesAppleTokens(t *testing.T) {
	configureAppleForTest(t)
	fx := setupDeletionFixture(t)
	seedAppleToken(t, fx.owner.Id, "app.familyrecord.ios", "apple-refresh-token")

	revokes := startAppleFormEndpoint(t, &appleRevokeEndpoint, http.StatusOK, ``)

	recorder := deleteAccountRequest(t, fx.ownerAuth,
		`{"password":"password123","confirmEmail":"owner@example.com"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	calls := revokes.calls()
	if len(calls) != 1 {
		t.Fatalf("revoke calls = %d, want 1", len(calls))
	}
	if got := calls[0].Get("token"); got != "apple-refresh-token" {
		t.Errorf("token = %q", got)
	}
	if got := calls[0].Get("token_type_hint"); got != "refresh_token" {
		t.Errorf("token_type_hint = %q", got)
	}
	if got := calls[0].Get("client_id"); got != "app.familyrecord.ios" {
		t.Errorf("client_id = %q, want the client the token was minted for", got)
	}
	if got := calls[0].Get("client_secret"); got == "" {
		t.Error("client_secret was not sent")
	}

	if record := storedAppleToken(t, fx.owner.Id); record.Token != "" {
		t.Error("the stored Apple token survived the deletion")
	}
	if got := countRows(t, fx.db, AppleRefreshTokenBkt); got != 0 {
		t.Errorf("apple refresh tokens remaining = %d, want 0", got)
	}
}

func TestDeleteAccountProceedsWhenAppleRefusesTheRevoke(t *testing.T) {
	configureAppleForTest(t)
	fx := setupDeletionFixture(t)
	seedAppleToken(t, fx.owner.Id, "app.familyrecord.ios", "apple-refresh-token")

	revokes := startAppleFormEndpoint(t, &appleRevokeEndpoint, http.StatusBadRequest,
		`{"error":"invalid_client"}`)

	recorder := deleteAccountRequest(t, fx.ownerAuth,
		`{"password":"password123","confirmEmail":"owner@example.com"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: Apple requires that deletion not be blocked", recorder.Code, http.StatusOK)
	}
	if len(revokes.calls()) != 1 {
		t.Errorf("revoke calls = %d, want the attempt to have been made", len(revokes.calls()))
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if GetUser(tx, fx.owner.Id).Id != 0 {
			t.Error("the account survived a failed revoke")
		}
	})
}

func TestDeleteAccountWithoutAnAppleTokenOnFile(t *testing.T) {
	configureAppleForTest(t)
	fx := setupDeletionFixture(t)

	revokes := startAppleFormEndpoint(t, &appleRevokeEndpoint, http.StatusOK, ``)

	recorder := deleteAccountRequest(t, fx.ownerAuth,
		`{"password":"password123","confirmEmail":"owner@example.com"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if len(revokes.calls()) != 0 {
		t.Error("a password account has nothing to revoke, so Apple should not have been called")
	}
}

func TestRevokeAppleRefreshTokenRequiresConfiguration(t *testing.T) {
	originalWeb := appleWebConfig
	appleWebConfig = nil
	t.Cleanup(func() { appleWebConfig = originalWeb })

	err := revokeAppleRefreshToken(AppleRefreshToken{Token: "orphaned-token"})
	if err == nil {
		t.Fatal("a token cannot be revoked without the key that signs the client secret")
	}
}

func TestRevokeAppleRefreshTokenFallsBackToTheWebClient(t *testing.T) {
	configureAppleForTest(t)

	revokes := startAppleFormEndpoint(t, &appleRevokeEndpoint, http.StatusOK, ``)

	// A row written before the client id was recorded alongside the token.
	if err := revokeAppleRefreshToken(AppleRefreshToken{Token: "legacy-token"}); err != nil {
		t.Fatalf("revokeAppleRefreshToken: %v", err)
	}

	calls := revokes.calls()
	if len(calls) != 1 {
		t.Fatalf("revoke calls = %d, want 1", len(calls))
	}
	if got := calls[0].Get("client_id"); got != "app.familyrecord.web" {
		t.Errorf("client_id = %q, want the services id", got)
	}
}
