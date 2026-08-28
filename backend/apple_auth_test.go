package backend

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"family/cfg"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.hasen.dev/vbolt"
)

/* ---------- fixtures ---------- */

const testAppleKid = "test-apple-kid"

func writeTestApplePrivateKey(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdsa key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}

	path := filepath.Join(t.TempDir(), "AuthKey.p8")
	encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return path
}

// startAppleKeyServer publishes one RSA key as a JWKS and points the identity
// token verifier at it, so no test reaches appleid.apple.com.
func startAppleKeyServer(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"keys": []appleJWK{{
				Kty: "RSA",
				Kid: testAppleKid,
				Alg: "RS256",
				N:   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
				E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
			}},
		})
	}))
	t.Cleanup(server.Close)

	originalEndpoint := appleKeysEndpoint
	appleKeysEndpoint = server.URL
	appleKeys.reset()
	t.Cleanup(func() {
		appleKeysEndpoint = originalEndpoint
		appleKeys.reset()
	})

	return key
}

func signAppleIDToken(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = testAppleKid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign identity token: %v", err)
	}
	return signed
}

func appleClaims(aud string) jwt.MapClaims {
	return jwt.MapClaims{
		"iss":              appleIssuer,
		"sub":              "001234.abcdef.0000",
		"aud":              aud,
		"iat":              time.Now().Add(-time.Minute).Unix(),
		"exp":              time.Now().Add(10 * time.Minute).Unix(),
		"email":            "person@example.com",
		"email_verified":   "true",
		"is_private_email": false,
	}
}

// configureAppleForTest installs a complete web credential and an iOS audience,
// restoring whatever the process had when the test finishes.
func configureAppleForTest(t *testing.T) {
	t.Helper()

	original := map[string]string{}
	for _, name := range []string{"APPLE_CLIENT_ID", "APPLE_TEAM_ID", "APPLE_KEY_ID", "APPLE_KEY_PATH", "APPLE_IOS_CLIENT_ID", "SITE_ROOT"} {
		original[name] = os.Getenv(name)
	}
	originalWeb := appleWebConfig
	originalIOS := appleIOSClientID

	t.Cleanup(func() {
		for name, value := range original {
			if value == "" {
				os.Unsetenv(name)
			} else {
				os.Setenv(name, value)
			}
		}
		appleWebConfig = originalWeb
		appleIOSClientID = originalIOS
	})

	os.Setenv("APPLE_CLIENT_ID", "app.familyrecord.web")
	os.Setenv("APPLE_TEAM_ID", "ABCDE12345")
	os.Setenv("APPLE_KEY_ID", "KEY1234567")
	os.Setenv("APPLE_KEY_PATH", writeTestApplePrivateKey(t))
	os.Setenv("APPLE_IOS_CLIENT_ID", "app.familyrecord.ios")
	os.Setenv("SITE_ROOT", "https://example.com")

	if err := SetupAppleOAuth(); err != nil {
		t.Fatalf("SetupAppleOAuth: %v", err)
	}
}

func appleTestDB(t *testing.T) *vbolt.DB {
	t.Helper()

	db := vbolt.Open(t.TempDir() + "/apple.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })
	appDb = db
	jwtKey = []byte("apple-test-secret-key-at-least-32-chars")
	return db
}

/* ---------- setup ---------- */

func TestSetupAppleOAuth(t *testing.T) {
	keyPath := writeTestApplePrivateKey(t)

	env := []string{"APPLE_CLIENT_ID", "APPLE_TEAM_ID", "APPLE_KEY_ID", "APPLE_KEY_PATH", "APPLE_IOS_CLIENT_ID", "SITE_ROOT"}
	original := map[string]string{}
	for _, name := range env {
		original[name] = os.Getenv(name)
	}
	t.Cleanup(func() {
		for name, value := range original {
			if value == "" {
				os.Unsetenv(name)
			} else {
				os.Setenv(name, value)
			}
		}
		appleWebConfig = nil
		appleIOSClientID = ""
	})

	clearAppleEnv := func() {
		for _, name := range env {
			os.Unsetenv(name)
		}
	}

	t.Run("CompleteConfiguration", func(t *testing.T) {
		clearAppleEnv()
		os.Setenv("APPLE_CLIENT_ID", "app.familyrecord.web")
		os.Setenv("APPLE_TEAM_ID", "ABCDE12345")
		os.Setenv("APPLE_KEY_ID", "KEY1234567")
		os.Setenv("APPLE_KEY_PATH", keyPath)
		os.Setenv("SITE_ROOT", "https://example.com")

		if err := SetupAppleOAuth(); err != nil {
			t.Fatalf("expected successful setup, got %v", err)
		}
		if appleWebConfig == nil {
			t.Fatal("web configuration was not set")
		}
		if appleWebConfig.RedirectURL != "https://example.com/api/apple/callback" {
			t.Errorf("redirect URL = %q", appleWebConfig.RedirectURL)
		}
		if appleWebConfig.Key == nil {
			t.Error("signing key was not loaded")
		}
	})

	t.Run("DefaultSiteRoot", func(t *testing.T) {
		clearAppleEnv()
		os.Setenv("APPLE_CLIENT_ID", "app.familyrecord.web")
		os.Setenv("APPLE_TEAM_ID", "ABCDE12345")
		os.Setenv("APPLE_KEY_ID", "KEY1234567")
		os.Setenv("APPLE_KEY_PATH", keyPath)

		if err := SetupAppleOAuth(); err != nil {
			t.Fatalf("expected successful setup, got %v", err)
		}
		if appleWebConfig.RedirectURL != "http://localhost:8666/api/apple/callback" {
			t.Errorf("redirect URL = %q", appleWebConfig.RedirectURL)
		}
	})

	t.Run("NothingSetIsNotAnHalfConfiguration", func(t *testing.T) {
		clearAppleEnv()

		err := SetupAppleOAuth()
		if err == nil {
			t.Fatal("expected an error when nothing is configured")
		}
		if !strings.Contains(err.Error(), "not configured") {
			t.Errorf("error = %v, want the not-configured message", err)
		}
		if appleWebConfig != nil {
			t.Error("web configuration should stay nil")
		}
	})

	t.Run("PartialConfigurationNamesWhatIsMissing", func(t *testing.T) {
		clearAppleEnv()
		os.Setenv("APPLE_CLIENT_ID", "app.familyrecord.web")

		err := SetupAppleOAuth()
		if err == nil {
			t.Fatal("expected an error for a partial configuration")
		}
		for _, want := range []string{"APPLE_TEAM_ID", "APPLE_KEY_ID", "APPLE_KEY_PATH"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %v does not name %s", err, want)
			}
		}
	})

	t.Run("UnusableKey", func(t *testing.T) {
		clearAppleEnv()
		bad := filepath.Join(t.TempDir(), "bad.p8")
		os.WriteFile(bad, []byte("not a pem file"), 0o600)

		os.Setenv("APPLE_CLIENT_ID", "app.familyrecord.web")
		os.Setenv("APPLE_TEAM_ID", "ABCDE12345")
		os.Setenv("APPLE_KEY_ID", "KEY1234567")
		os.Setenv("APPLE_KEY_PATH", bad)

		if err := SetupAppleOAuth(); err == nil {
			t.Fatal("expected an error for an unparseable key")
		}
	})

	t.Run("NativeAudienceWithoutWebCredential", func(t *testing.T) {
		clearAppleEnv()
		os.Setenv("APPLE_IOS_CLIENT_ID", "app.familyrecord.ios")

		// The web flow stays off, but the native audience is still registered
		// so the companion app can sign in.
		if err := SetupAppleOAuth(); err == nil {
			t.Fatal("expected an error: the web credential is incomplete")
		}
		if appleIOSClientID != "app.familyrecord.ios" {
			t.Errorf("iOS client id = %q", appleIOSClientID)
		}
		if got := appleAudiences(); len(got) != 1 || got[0] != "app.familyrecord.ios" {
			t.Errorf("audiences = %v, want only the iOS client id", got)
		}
	})
}

/* ---------- client secret ---------- */

func TestAppleClientSecretClaims(t *testing.T) {
	configureAppleForTest(t)

	now := time.Now()
	secret, err := appleWebConfig.clientSecret(now)
	if err != nil {
		t.Fatalf("clientSecret: %v", err)
	}

	parsed, _, err := jwt.NewParser().ParseUnverified(secret, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("parse client secret: %v", err)
	}

	if parsed.Method.Alg() != "ES256" {
		t.Errorf("alg = %q, want ES256", parsed.Method.Alg())
	}
	if kid, _ := parsed.Header["kid"].(string); kid != "KEY1234567" {
		t.Errorf("kid header = %q", kid)
	}

	claims := parsed.Claims.(jwt.MapClaims)
	if claims["iss"] != "ABCDE12345" {
		t.Errorf("iss = %v, want the team id", claims["iss"])
	}
	if claims["sub"] != "app.familyrecord.web" {
		t.Errorf("sub = %v, want the services id", claims["sub"])
	}
	if claims["aud"] != appleIssuer {
		t.Errorf("aud = %v, want %s", claims["aud"], appleIssuer)
	}
	if exp, ok := claims["exp"].(float64); !ok || int64(exp) != now.Add(appleClientSecretLifetime).Unix() {
		t.Errorf("exp = %v, want %d", claims["exp"], now.Add(appleClientSecretLifetime).Unix())
	}
}

/* ---------- identity token verification ---------- */

func TestVerifyAppleIDToken(t *testing.T) {
	configureAppleForTest(t)
	key := startAppleKeyServer(t)

	t.Run("WebAudience", func(t *testing.T) {
		info, err := verifyAppleIDToken(signAppleIDToken(t, key, appleClaims("app.familyrecord.web")))
		if err != nil {
			t.Fatalf("verifyAppleIDToken: %v", err)
		}
		if info.Email != "person@example.com" {
			t.Errorf("email = %q", info.Email)
		}
		if !info.EmailVerified {
			t.Error("email_verified sent as the string \"true\" should read as true")
		}
		if info.IsPrivateEmail {
			t.Error("is_private_email sent as false should read as false")
		}
		if info.Sub != "001234.abcdef.0000" {
			t.Errorf("sub = %q", info.Sub)
		}
	})

	t.Run("NativeAudience", func(t *testing.T) {
		if _, err := verifyAppleIDToken(signAppleIDToken(t, key, appleClaims("app.familyrecord.ios"))); err != nil {
			t.Fatalf("verifyAppleIDToken: %v", err)
		}
	})

	t.Run("ForeignAudienceIsRejected", func(t *testing.T) {
		_, err := verifyAppleIDToken(signAppleIDToken(t, key, appleClaims("com.someone.else")))
		if err == nil {
			t.Fatal("a token minted for another relying party must not authenticate anyone")
		}
		if !strings.Contains(err.Error(), "audience") {
			t.Errorf("error = %v, want an audience complaint", err)
		}
	})

	t.Run("ForeignIssuerIsRejected", func(t *testing.T) {
		claims := appleClaims("app.familyrecord.web")
		claims["iss"] = "https://accounts.google.com"
		if _, err := verifyAppleIDToken(signAppleIDToken(t, key, claims)); err == nil {
			t.Fatal("expected an issuer rejection")
		}
	})

	t.Run("ExpiredIsRejected", func(t *testing.T) {
		claims := appleClaims("app.familyrecord.web")
		claims["exp"] = time.Now().Add(-time.Minute).Unix()
		if _, err := verifyAppleIDToken(signAppleIDToken(t, key, claims)); err == nil {
			t.Fatal("expected an expiry rejection")
		}
	})

	t.Run("MissingExpiryIsRejected", func(t *testing.T) {
		claims := appleClaims("app.familyrecord.web")
		delete(claims, "exp")
		if _, err := verifyAppleIDToken(signAppleIDToken(t, key, claims)); err == nil {
			t.Fatal("a token with no expiry never stops working; it must be refused")
		}
	})

	t.Run("UnsignedTokenIsRejected", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodNone, appleClaims("app.familyrecord.web"))
		token.Header["kid"] = testAppleKid
		signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
		if err != nil {
			t.Fatalf("sign none: %v", err)
		}
		if _, err := verifyAppleIDToken(signed); err == nil {
			t.Fatal("alg=none must not be accepted")
		}
	})

	t.Run("UnknownKidIsRejected", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, appleClaims("app.familyrecord.web"))
		token.Header["kid"] = "some-other-kid"
		signed, err := token.SignedString(key)
		if err != nil {
			t.Fatalf("sign token: %v", err)
		}
		if _, err := verifyAppleIDToken(signed); err == nil {
			t.Fatal("expected a rejection for an unknown key id")
		}
	})

	t.Run("WrongSigningKeyIsRejected", func(t *testing.T) {
		other, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("generate rsa key: %v", err)
		}
		if _, err := verifyAppleIDToken(signAppleIDToken(t, other, appleClaims("app.familyrecord.web"))); err == nil {
			t.Fatal("a token signed by anything but Apple must be refused")
		}
	})
}

func TestVerifyAppleIDTokenWithoutConfiguration(t *testing.T) {
	originalWeb, originalIOS := appleWebConfig, appleIOSClientID
	appleWebConfig, appleIOSClientID = nil, ""
	t.Cleanup(func() { appleWebConfig, appleIOSClientID = originalWeb, originalIOS })

	if _, err := verifyAppleIDToken("anything"); err == nil {
		t.Fatal("with no audience configured every token is unverifiable")
	}
}

func TestAppleFlexBool(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{`true`, true},
		{`false`, false},
		{`"true"`, true},
		{`"false"`, false},
	}

	for _, tc := range cases {
		var got appleFlexBool
		if err := json.Unmarshal([]byte(tc.raw), &got); err != nil {
			t.Fatalf("unmarshal %s: %v", tc.raw, err)
		}
		if bool(got) != tc.want {
			t.Errorf("%s decoded to %v, want %v", tc.raw, bool(got), tc.want)
		}
	}

	var got appleFlexBool
	if err := json.Unmarshal([]byte(`42`), &got); err == nil {
		t.Error("a number is neither shape Apple sends and should fail")
	}
}

/* ---------- browser flow ---------- */

func TestAppleLoginHandlerRedirect(t *testing.T) {
	configureAppleForTest(t)

	rec := httptest.NewRecorder()
	appleLoginHandler(rec, httptest.NewRequest(http.MethodGet, "/api/login/apple", nil))

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}

	target, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	if target.Scheme+"://"+target.Host+target.Path != appleAuthorizeURL {
		t.Errorf("redirect target = %q, want %s", target, appleAuthorizeURL)
	}

	query := target.Query()
	for field, want := range map[string]string{
		"client_id":     "app.familyrecord.web",
		"redirect_uri":  "https://example.com/api/apple/callback",
		"response_type": "code id_token",
		"response_mode": "form_post",
		"scope":         "name email",
	} {
		if got := query.Get(field); got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}

	state, nonce := query.Get("state"), query.Get("nonce")
	if state == "" || nonce == "" {
		t.Fatal("state and nonce must both be present")
	}
	if state == nonce {
		t.Error("state and nonce must be independent values")
	}

	var stateCookie *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == appleStateCookieName {
			stateCookie = cookie
		}
	}
	if stateCookie == nil {
		t.Fatal("no state cookie was set")
	}
	if stateCookie.Value != state+"."+nonce {
		t.Errorf("cookie = %q, want the state and nonce sent to Apple", stateCookie.Value)
	}
	if !stateCookie.HttpOnly || !stateCookie.Secure {
		t.Error("state cookie must be HttpOnly and Secure")
	}
	if stateCookie.SameSite != http.SameSiteNoneMode {
		t.Error("Apple posts the callback cross-site, so a Lax cookie would never arrive")
	}
}

func TestAppleLoginHandlerWithoutConfiguration(t *testing.T) {
	original := appleWebConfig
	appleWebConfig = nil
	t.Cleanup(func() { appleWebConfig = original })

	rec := httptest.NewRecorder()
	appleLoginHandler(rec, httptest.NewRequest(http.MethodGet, "/api/login/apple", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func appleCallbackRequest(form url.Values, cookieValue string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/apple/callback", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookieValue != "" {
		req.AddCookie(&http.Cookie{Name: appleStateCookieName, Value: cookieValue})
	}
	return req
}

func TestAppleCallbackRejections(t *testing.T) {
	configureAppleForTest(t)

	t.Run("GetIsNotAllowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		appleCallbackHandler(rec, httptest.NewRequest(http.MethodGet, "/api/apple/callback", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("MissingStateCookie", func(t *testing.T) {
		rec := httptest.NewRecorder()
		appleCallbackHandler(rec, appleCallbackRequest(url.Values{"state": {"abc"}, "code": {"xyz"}}, ""))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("MismatchedState", func(t *testing.T) {
		rec := httptest.NewRecorder()
		appleCallbackHandler(rec, appleCallbackRequest(url.Values{"state": {"attacker"}, "code": {"xyz"}}, "expected.nonce123"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
		if !strings.Contains(rec.Body.String(), "state") {
			t.Errorf("body = %q, want a state complaint", rec.Body.String())
		}
	})

	t.Run("MissingCode", func(t *testing.T) {
		rec := httptest.NewRecorder()
		appleCallbackHandler(rec, appleCallbackRequest(url.Values{"state": {"expected"}}, "expected.nonce123"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("UserCancelledAtApple", func(t *testing.T) {
		rec := httptest.NewRecorder()
		appleCallbackHandler(rec, appleCallbackRequest(url.Values{"error": {"user_cancelled_authorize"}}, "expected.nonce123"))
		if rec.Code != http.StatusFound {
			t.Fatalf("status = %d, want a redirect back to the login page", rec.Code)
		}
		if got := rec.Header().Get("Location"); got != "/login" {
			t.Errorf("Location = %q, want /login", got)
		}
	})
}

func TestAppleCallbackSignsInAndCreatesAnAccount(t *testing.T) {
	configureAppleForTest(t)
	appleTestDB(t)
	key := startAppleKeyServer(t)

	claims := appleClaims("app.familyrecord.web")
	claims["email"] = "newcomer@example.com"
	claims["nonce"] = "nonce123"
	idToken := signAppleIDToken(t, key, claims)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse token request: %v", err)
		}
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %q", r.FormValue("grant_type"))
		}
		if r.FormValue("code") != "the-code" {
			t.Errorf("code = %q", r.FormValue("code"))
		}
		if r.FormValue("client_id") != "app.familyrecord.web" {
			t.Errorf("client_id = %q", r.FormValue("client_id"))
		}
		if r.FormValue("client_secret") == "" {
			t.Error("client_secret was not sent")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id_token":%q}`, idToken)
	}))
	defer tokenServer.Close()

	originalEndpoint := appleTokenEndpoint
	appleTokenEndpoint = tokenServer.URL
	defer func() { appleTokenEndpoint = originalEndpoint }()

	form := url.Values{
		"state": {"expected"},
		"code":  {"the-code"},
		"user":  {`{"name":{"firstName":"Dana","lastName":"Reed"},"email":"newcomer@example.com"}`},
	}

	rec := httptest.NewRecorder()
	appleCallbackHandler(rec, appleCallbackRequest(form, "expected.nonce123"))

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d (%s), want %d", rec.Code, rec.Body.String(), http.StatusFound)
	}
	if got := rec.Header().Get("Location"); got != "/dashboard" {
		t.Errorf("Location = %q, want /dashboard", got)
	}

	var authCookie bool
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "authToken" && cookie.Value != "" {
			authCookie = true
		}
		if cookie.Name == appleStateCookieName && cookie.Value != "" {
			t.Error("the state cookie must be cleared once it has been spent")
		}
	}
	if !authCookie {
		t.Error("no session cookie was issued")
	}

	var user User
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		user = GetUser(tx, GetUserId(tx, "newcomer@example.com"))
	})
	if user.Id == 0 {
		t.Fatal("no account was created")
	}
	if user.Name != "Dana Reed" {
		t.Errorf("name = %q, want the name Apple released on first authorization", user.Name)
	}
	if !user.EmailVerified {
		t.Error("Apple asserted the address, so it should not need a confirmation round trip")
	}
}

func TestAppleCallbackRejectsAReplayedNonce(t *testing.T) {
	configureAppleForTest(t)
	appleTestDB(t)
	key := startAppleKeyServer(t)

	claims := appleClaims("app.familyrecord.web")
	claims["nonce"] = "some-other-login"
	idToken := signAppleIDToken(t, key, claims)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"id_token":%q}`, idToken)
	}))
	defer tokenServer.Close()

	originalEndpoint := appleTokenEndpoint
	appleTokenEndpoint = tokenServer.URL
	defer func() { appleTokenEndpoint = originalEndpoint }()

	rec := httptest.NewRecorder()
	appleCallbackHandler(rec, appleCallbackRequest(url.Values{"state": {"expected"}, "code": {"the-code"}}, "expected.nonce123"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d for a token minted for a different login", rec.Code, http.StatusBadRequest)
	}
}

func TestExchangeAppleCodeFailures(t *testing.T) {
	configureAppleForTest(t)

	t.Run("HTTPError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid_grant"}`))
		}))
		defer server.Close()

		original := appleTokenEndpoint
		appleTokenEndpoint = server.URL
		defer func() { appleTokenEndpoint = original }()

		if _, err := exchangeAppleCode("bad"); err == nil {
			t.Fatal("expected an error for a rejected exchange")
		}
	})

	t.Run("NoIdentityToken", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"access_token":"a"}`))
		}))
		defer server.Close()

		original := appleTokenEndpoint
		appleTokenEndpoint = server.URL
		defer func() { appleTokenEndpoint = original }()

		if _, err := exchangeAppleCode("code"); err == nil {
			t.Fatal("a response with no id_token authenticates nobody")
		}
	})
}

/* ---------- native flow ---------- */

func appleTokenLoginRequest(body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/login/apple/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	appleTokenLoginHandler(rec, req)
	return rec
}

func decodeLoginResponse(t *testing.T, rec *httptest.ResponseRecorder) LoginResponse {
	t.Helper()
	var resp LoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return resp
}

func TestAppleTokenLoginHandler(t *testing.T) {
	configureAppleForTest(t)
	appleTestDB(t)
	key := startAppleKeyServer(t)

	t.Run("GetIsNotAllowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		appleTokenLoginHandler(rec, httptest.NewRequest(http.MethodGet, "/api/login/apple/token", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("MalformedBody", func(t *testing.T) {
		resp := decodeLoginResponse(t, appleTokenLoginRequest(`{`))
		if resp.Success || resp.Error == "" {
			t.Fatalf("response = %+v, want a failure", resp)
		}
	})

	t.Run("MissingToken", func(t *testing.T) {
		resp := decodeLoginResponse(t, appleTokenLoginRequest(`{"idToken":""}`))
		if resp.Success {
			t.Fatal("an empty identity token must not authenticate anyone")
		}
	})

	t.Run("GarbageToken", func(t *testing.T) {
		resp := decodeLoginResponse(t, appleTokenLoginRequest(`{"idToken":"not.a.jwt"}`))
		if resp.Success {
			t.Fatal("an unverifiable token must not authenticate anyone")
		}
		if resp.Error != "Invalid Apple token" {
			t.Errorf("error = %q", resp.Error)
		}
	})

	t.Run("CreatesAnAccountWithTheNameTheAppForwards", func(t *testing.T) {
		claims := appleClaims("app.familyrecord.ios")
		claims["email"] = "native@example.com"
		body, _ := json.Marshal(AppleTokenLoginRequest{
			IDToken: signAppleIDToken(t, key, claims),
			Name:    "Ada Lovelace",
		})

		resp := decodeLoginResponse(t, appleTokenLoginRequest(string(body)))
		if !resp.Success {
			t.Fatalf("response = %+v, want success", resp)
		}
		if resp.Token == "" {
			t.Error("no session token was issued")
		}
		if resp.Auth.Email != "native@example.com" {
			t.Errorf("auth email = %q", resp.Auth.Email)
		}
		if resp.Auth.Name != "Ada Lovelace" {
			t.Errorf("auth name = %q", resp.Auth.Name)
		}
		if !resp.Auth.EmailVerified {
			t.Error("Apple asserted the address")
		}
	})

	t.Run("SecondSignInReusesTheAccount", func(t *testing.T) {
		claims := appleClaims("app.familyrecord.ios")
		claims["email"] = "native@example.com"
		// Apple sends no name after the first authorization, so the app has
		// none to forward. The stored name must survive.
		body, _ := json.Marshal(AppleTokenLoginRequest{IDToken: signAppleIDToken(t, key, claims)})

		resp := decodeLoginResponse(t, appleTokenLoginRequest(string(body)))
		if !resp.Success {
			t.Fatalf("response = %+v, want success", resp)
		}
		if resp.Auth.Name != "Ada Lovelace" {
			t.Errorf("auth name = %q, want the name kept from the first sign-in", resp.Auth.Name)
		}

		var count int
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			if GetUserId(tx, "native@example.com") > 0 {
				count = 1
			}
		})
		if count != 1 {
			t.Error("the second sign-in should not have created a second account")
		}
	})

	t.Run("SignsInToAnAccountCreatedWithAPassword", func(t *testing.T) {
		vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
			AddUserTx(tx, CreateAccountRequest{Name: "Existing Person", Email: "existing@example.com"}, []byte{})
			vbolt.TxCommit(tx)
		})

		claims := appleClaims("app.familyrecord.ios")
		claims["email"] = "existing@example.com"
		body, _ := json.Marshal(AppleTokenLoginRequest{IDToken: signAppleIDToken(t, key, claims)})

		resp := decodeLoginResponse(t, appleTokenLoginRequest(string(body)))
		if !resp.Success {
			t.Fatalf("response = %+v, want success", resp)
		}
		if resp.Auth.Name != "Existing Person" {
			t.Errorf("auth name = %q, want the existing account", resp.Auth.Name)
		}
	})
}

func TestUpsertAppleUserWithoutAnEmail(t *testing.T) {
	appleTestDB(t)

	_, err := upsertAppleUser(
		httptest.NewRequest(http.MethodPost, "/api/login/apple/token", nil),
		&AppleTokenInfo{Sub: "001234.abcdef.0000"},
		"Someone",
	)
	if err == nil {
		t.Fatal("an account is keyed by email; a token without one cannot resolve to a user")
	}
}

func TestAppleDisplayName(t *testing.T) {
	cases := []struct {
		name           string
		given          string
		email          string
		isPrivateEmail bool
		want           string
	}{
		{"UsesTheNameApplePassed", "Dana Reed", "dana@example.com", false, "Dana Reed"},
		{"TrimsWhitespace", "  Dana Reed  ", "dana@example.com", false, "Dana Reed"},
		{"FallsBackToTheLocalPart", "", "dana@example.com", false, "dana"},
		{"IgnoresARelayLocalPart", "", "a1b2c3d4@privaterelay.appleid.com", true, appleFallbackName},
		{"HandlesAnEmptyLocalPart", "", "@example.com", false, appleFallbackName},
		{"HandlesAWhitespaceOnlyName", " ", "dana@example.com", false, "dana"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := appleDisplayName(tc.given, tc.email, tc.isPrivateEmail); got != tc.want {
				t.Errorf("appleDisplayName(%q, %q, %v) = %q, want %q", tc.given, tc.email, tc.isPrivateEmail, got, tc.want)
			}
		})
	}
}

/* ---------- provider discovery ---------- */

func TestAuthProvidersHandler(t *testing.T) {
	configureAppleForTest(t)

	rec := httptest.NewRecorder()
	authProvidersHandler(rec, httptest.NewRequest(http.MethodGet, "/api/auth/providers", nil))

	var providers AuthProviders
	if err := json.Unmarshal(rec.Body.Bytes(), &providers); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if !providers.Apple {
		t.Error("Apple is fully configured and should be advertised")
	}

	appleWebConfig = nil
	rec = httptest.NewRecorder()
	authProvidersHandler(rec, httptest.NewRequest(http.MethodGet, "/api/auth/providers", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &providers); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if providers.Apple {
		t.Error("an unconfigured Apple credential must not be advertised as a button")
	}
}
