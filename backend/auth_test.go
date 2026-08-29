package backend

import (
	"encoding/json"
	"family/cfg"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

func TestResolveJWTSecret(t *testing.T) {
	t.Run("rejects missing release secret", func(t *testing.T) {
		if !cfg.IsRelease {
			t.Skip("release-only assertion")
		}
		t.Setenv("JWT_SECRET_KEY", "")

		if _, err := resolveJWTSecret(); err == nil {
			t.Fatal("resolveJWTSecret() accepted an empty release secret")
		}
	})

	t.Run("rejects weak configured secret", func(t *testing.T) {
		t.Setenv("JWT_SECRET_KEY", strings.Repeat("x", minimumJWTSecretLength-1))

		if _, err := resolveJWTSecret(); err == nil {
			t.Fatal("resolveJWTSecret() accepted a weak secret")
		}
	})

	t.Run("accepts strong configured secret", func(t *testing.T) {
		want := strings.Repeat("x", minimumJWTSecretLength)
		t.Setenv("JWT_SECRET_KEY", want)

		got, err := resolveJWTSecret()
		if err != nil {
			t.Fatalf("resolveJWTSecret() error = %v", err)
		}
		if got != want {
			t.Errorf("resolveJWTSecret() = %q, want configured secret", got)
		}
	})
}

func TestSetupGoogleOAuth(t *testing.T) {
	originalClientID := os.Getenv("GOOGLE_CLIENT_ID")
	originalClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	originalSiteRoot := os.Getenv("SITE_ROOT")

	defer func() {
		os.Setenv("GOOGLE_CLIENT_ID", originalClientID)
		os.Setenv("GOOGLE_CLIENT_SECRET", originalClientSecret)
		os.Setenv("SITE_ROOT", originalSiteRoot)
	}()

	t.Run("Valid OAuth setup", func(t *testing.T) {
		os.Setenv("GOOGLE_CLIENT_ID", "test_client_id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test_client_secret")
		os.Setenv("SITE_ROOT", "https://example.com")

		err := SetupGoogleOAuth()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if oauthConf == nil {
			t.Error("Expected oauthConf to be initialized")
		}

		if oauthConf.ClientID != "test_client_id" {
			t.Errorf("Expected ClientID 'test_client_id', got '%s'", oauthConf.ClientID)
		}

		if oauthConf.RedirectURL != "https://example.com/api/google/callback" {
			t.Errorf("Expected RedirectURL 'https://example.com/api/google/callback', got '%s'", oauthConf.RedirectURL)
		}
	})

	t.Run("Missing client ID", func(t *testing.T) {
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test_client_secret")

		err := SetupGoogleOAuth()
		if err == nil {
			t.Error("Expected error for missing client ID")
		}

		expectedError := "Google OAuth credentials not configured"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
		}
	})

	t.Run("Missing client secret", func(t *testing.T) {
		os.Setenv("GOOGLE_CLIENT_ID", "test_client_id")
		os.Unsetenv("GOOGLE_CLIENT_SECRET")

		err := SetupGoogleOAuth()
		if err == nil {
			t.Error("Expected error for missing client secret")
		}
	})

	t.Run("Default site root", func(t *testing.T) {
		os.Setenv("GOOGLE_CLIENT_ID", "test_client_id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test_client_secret")
		os.Unsetenv("SITE_ROOT")

		err := SetupGoogleOAuth()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		expectedRedirect := "http://localhost:8666/api/google/callback"
		if oauthConf.RedirectURL != expectedRedirect {
			t.Errorf("Expected RedirectURL '%s', got '%s'", expectedRedirect, oauthConf.RedirectURL)
		}
	})
}

func TestGenerateAuthJwt(t *testing.T) {
	testDBPath := "test_auth.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	appDb = db

	user := User{
		Id:        1,
		Name:      "Test User",
		Email:     "test@example.com",
		FamilyId:  1,
		Creation:  time.Now(),
		LastLogin: time.Now(),
	}

	recorder := httptest.NewRecorder()

	token, err := generateAuthJwt(user, recorder)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if token == "" {
		t.Error("Expected non-empty token")
	}

	cookies := recorder.Result().Cookies()
	var authCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "authToken" {
			authCookie = cookie
			break
		}
	}

	if authCookie == nil {
		t.Error("Expected authToken cookie to be set")
	}

	if authCookie.Value != token {
		t.Errorf("Expected cookie value '%s', got '%s'", token, authCookie.Value)
	}

	if !authCookie.HttpOnly {
		t.Error("Expected cookie to be HttpOnly")
	}

	parsedToken, err := jwt.ParseWithClaims(token, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil {
		t.Fatalf("Expected to parse token, got error %v", err)
	}

	if !parsedToken.Valid {
		t.Error("Expected token to be valid")
	}

	claims, ok := parsedToken.Claims.(*Claims)
	if !ok {
		t.Error("Expected claims to be *Claims")
	}

	if claims.Username != "test@example.com" {
		t.Errorf("Expected username 'test@example.com', got %v", claims.Username)
	}

	if claims.ExpiresAt == nil {
		t.Error("Expected expiration to be set")
	}
}

func TestAuthenticateForUser(t *testing.T) {
	testDBPath := "test_auth_user.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	appDb = db

	var user User
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user = User{
			Id:        1,
			Name:      "Test User",
			Email:     "test@example.com",
			FamilyId:  1,
			Creation:  time.Now(),
			LastLogin: time.Now(),
		}
		user.Id = vbolt.NextIntId(tx, UsersBkt)
		vbolt.Write(tx, UsersBkt, user.Id, &user)
		vbolt.TxCommit(tx)
	})

	recorder := httptest.NewRecorder()

	err := authenticateForUser(user.Id, recorder)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	cookies := recorder.Result().Cookies()
	var authCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "authToken" {
			authCookie = cookie
			break
		}
	}

	if authCookie == nil {
		t.Error("Expected authToken cookie to be set")
	}

	if authCookie.Value == "" {
		t.Error("Expected non-empty cookie value")
	}
}

func TestAuthenticateForUserNotFound(t *testing.T) {
	testDBPath := "test_auth_not_found.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	appDb = db

	recorder := httptest.NewRecorder()

	err := authenticateForUser(999, recorder)
	if err == nil {
		t.Error("Expected error for non-existent user")
	}

	expectedError := "user not found"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestLogoutHandler(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/logout", nil)
	recorder := httptest.NewRecorder()

	logoutHandler(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	cookies := recorder.Result().Cookies()
	var authCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "authToken" {
			authCookie = cookie
			break
		}
	}

	if authCookie == nil {
		t.Error("Expected authToken cookie to be set for clearing")
	}

	if authCookie.Value != "" {
		t.Errorf("Expected empty cookie value, got '%s'", authCookie.Value)
	}

	if authCookie.Expires.After(time.Now()) {
		t.Error("Expected cookie expiration to be in the past")
	}
}

func TestLogoutHandlerDeactivatesCurrentDeviceToken(t *testing.T) {
	db := vbolt.Open(t.TempDir() + "/logout.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })
	appDb = db
	jwtKey = []byte("logout-test-secret-key")

	user := User{Id: 42, Name: "Test User", Email: "test@example.com", Creation: time.Now()}
	deviceToken := strings.Repeat("a", apnsDeviceTokenHexLength)
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		vbolt.Write(tx, UsersBkt, user.Id, &user)
		vbolt.Write(tx, EmailBkt, user.Email, &user.Id)
		if _, err := upsertPushDeviceToken(tx, user.Id, RegisterPushDeviceRequest{
			Token: deviceToken, Platform: "ios", Environment: "sandbox", BundleId: "com.example.family",
		}); err != nil {
			t.Fatalf("register push device: %v", err)
		}
		vbolt.TxCommit(tx)
	})

	authToken, err := generateJwtTokenString(user)
	if err != nil {
		t.Fatalf("generate auth token: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/logout", strings.NewReader(`{"deviceToken":"`+deviceToken+`"}`))
	req.AddCookie(&http.Cookie{Name: "authToken", Value: authToken})
	recorder := httptest.NewRecorder()

	logoutHandler(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		device := GetPushDeviceTokenByToken(tx, deviceToken)
		if device.IsActive {
			t.Error("device token remained active after logout")
		}
	})
}

func TestLogoutHandlerDoesNotDeactivateAnotherUsersDevice(t *testing.T) {
	db := vbolt.Open(t.TempDir() + "/logout.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })
	appDb = db
	jwtKey = []byte("logout-test-secret-key")

	user := User{Id: 42, Name: "Test User", Email: "test@example.com", Creation: time.Now()}
	deviceToken := strings.Repeat("b", apnsDeviceTokenHexLength)
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		vbolt.Write(tx, UsersBkt, user.Id, &user)
		vbolt.Write(tx, EmailBkt, user.Email, &user.Id)
		if _, err := upsertPushDeviceToken(tx, 99, RegisterPushDeviceRequest{
			Token: deviceToken, Platform: "ios", Environment: "sandbox", BundleId: "com.example.family",
		}); err != nil {
			t.Fatalf("register push device: %v", err)
		}
		vbolt.TxCommit(tx)
	})

	authToken, err := generateJwtTokenString(user)
	if err != nil {
		t.Fatalf("generate auth token: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/logout", strings.NewReader(`{"deviceToken":"`+deviceToken+`"}`))
	req.AddCookie(&http.Cookie{Name: "authToken", Value: authToken})
	recorder := httptest.NewRecorder()

	logoutHandler(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		device := GetPushDeviceTokenByToken(tx, deviceToken)
		if !device.IsActive {
			t.Error("logout deactivated another user's device token")
		}
	})
}

func TestLogoutHandlerRequiresPost(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/logout", nil)
	recorder := httptest.NewRecorder()

	logoutHandler(recorder, req)

	if !strings.Contains(recorder.Body.String(), "logout call must be POST") {
		t.Errorf("Expected error to mention POST requirement, got: %s", recorder.Body.String())
	}
}

func TestGenerateStateString(t *testing.T) {
	token1, err1 := generateToken(32)
	token2, err2 := generateToken(32)

	if err1 != nil || err2 != nil {
		t.Fatalf("Expected no errors, got %v, %v", err1, err2)
	}

	if token1 == token2 {
		t.Error("Expected different tokens, got same value")
	}

	if len(token1) == 0 || len(token2) == 0 {
		t.Error("Expected non-empty tokens")
	}
}

func loginTestUser(t *testing.T, db *vbolt.DB, email, password string) User {
	t.Helper()

	var user User
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		user = User{
			Id:        vbolt.NextIntId(tx, UsersBkt),
			Name:      "Login Test",
			Email:     email,
			FamilyId:  1,
			Creation:  time.Now(),
			LastLogin: time.Now(),
		}
		vbolt.Write(tx, UsersBkt, user.Id, &user)
		vbolt.Write(tx, EmailBkt, user.Email, &user.Id)

		hash := []byte{}
		if password != "" {
			generated, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				t.Fatalf("bcrypt.GenerateFromPassword() error = %v", err)
			}
			hash = generated
		}
		vbolt.Write(tx, PasswdBkt, user.Id, &hash)
		vbolt.TxCommit(tx)
	})
	return user
}

func postLogin(t *testing.T, email, password string) (*httptest.ResponseRecorder, LoginResponse) {
	t.Helper()

	body := strings.NewReader(`{"email":"` + email + `","password":"` + password + `"}`)
	req := httptest.NewRequest("POST", "/api/login", body)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	loginHandler(recorder, req)

	var resp LoginResponse
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	return recorder, resp
}

func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	testDBPath := "test_login_enumeration.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()
	appDb = db

	loginTestUser(t, db, "member@example.com", "correct-horse-battery")
	loginTestUser(t, db, "google-only@example.com", "")

	cases := []struct {
		name     string
		email    string
		password string
	}{
		{name: "wrong password", email: "member@example.com", password: "wrong-password"},
		{name: "unknown address", email: "stranger@example.com", password: "wrong-password"},
		{name: "google-only account", email: "google-only@example.com", password: "wrong-password"},
	}

	var firstStatus int
	for i, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			recorder, resp := postLogin(t, tt.email, tt.password)

			if resp.Success {
				t.Fatal("login succeeded with a bad password")
			}
			if resp.Error != invalidCredentialsMessage {
				t.Errorf("error = %q, want %q", resp.Error, invalidCredentialsMessage)
			}
			if i == 0 {
				firstStatus = recorder.Code
			} else if recorder.Code != firstStatus {
				t.Errorf("status = %d, want %d to match the other failures", recorder.Code, firstStatus)
			}
		})
	}
}

func TestLoginTimingDoesNotRevealAccounts(t *testing.T) {
	testDBPath := "test_login_timing.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()
	appDb = db

	loginTestUser(t, db, "member@example.com", "correct-horse-battery")

	compareAgainstDecoyPassword("warmup")

	start := time.Now()
	postLogin(t, "member@example.com", "wrong-password")
	knownAccount := time.Since(start)

	start = time.Now()
	postLogin(t, "stranger@example.com", "wrong-password")
	unknownAccount := time.Since(start)

	if unknownAccount < knownAccount/2 {
		t.Errorf("unknown address answered in %v against %v for a known one", unknownAccount, knownAccount)
	}
}
