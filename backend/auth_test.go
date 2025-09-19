package backend

import (
	"family/cfg"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.hasen.dev/vbolt"
)

func TestSetupGoogleOAuth(t *testing.T) {
	// Save original env vars
	originalClientID := os.Getenv("GOOGLE_CLIENT_ID")
	originalClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	originalSiteRoot := os.Getenv("SITE_ROOT")

	// Clean up after test
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

	// Set the global database for the auth functions
	appDb = db

	// Create a test user
	user := User{
		Id:       1,
		Name:     "Test User",
		Email:    "test@example.com",
		FamilyId: 1,
		Creation: time.Now(),
		LastLogin: time.Now(),
	}

	// Create a test response recorder
	recorder := httptest.NewRecorder()

	// Test JWT generation
	token, err := generateAuthJwt(user, recorder)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if token == "" {
		t.Error("Expected non-empty token")
	}

	// Check that cookie was set
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

	// Verify token can be parsed and contains correct claims
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

	// Verify expiration is set
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

	// Set the global database for the auth functions
	appDb = db

	// Create a test user in the database
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

	// Check that cookie was set
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

	// Set the global database for the auth functions
	appDb = db

	recorder := httptest.NewRecorder()

	// Try to authenticate non-existent user
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

	// Check response
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	// Check that cookie was cleared
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

	// Check that expiration is in the past
	if authCookie.Expires.After(time.Now()) {
		t.Error("Expected cookie expiration to be in the past")
	}
}

func TestGenerateStateString(t *testing.T) {
	// Test that generateToken produces different results
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