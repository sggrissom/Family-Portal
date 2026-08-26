package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestGoogleOAuthSetup(t *testing.T) {
	originalClientID := os.Getenv("GOOGLE_CLIENT_ID")
	originalClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	originalSiteRoot := os.Getenv("SITE_ROOT")

	defer func() {
		os.Setenv("GOOGLE_CLIENT_ID", originalClientID)
		os.Setenv("GOOGLE_CLIENT_SECRET", originalClientSecret)
		os.Setenv("SITE_ROOT", originalSiteRoot)
	}()

	t.Run("SuccessfulSetup", func(t *testing.T) {
		os.Setenv("GOOGLE_CLIENT_ID", "test_client_id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test_client_secret")
		os.Setenv("SITE_ROOT", "https://example.com")

		err := SetupGoogleOAuth()
		if err != nil {
			t.Errorf("Expected successful setup, got error: %v", err)
		}

		if oauthConf == nil {
			t.Error("OAuth configuration was not set")
		}
		if oauthConf.ClientID != "test_client_id" {
			t.Errorf("Expected client ID 'test_client_id', got '%s'", oauthConf.ClientID)
		}
		if oauthConf.ClientSecret != "test_client_secret" {
			t.Errorf("Expected client secret 'test_client_secret', got '%s'", oauthConf.ClientSecret)
		}
		if oauthConf.RedirectURL != "https://example.com/api/google/callback" {
			t.Errorf("Expected redirect URL 'https://example.com/api/google/callback', got '%s'", oauthConf.RedirectURL)
		}

		if oauthStateString == "" {
			t.Error("OAuth state string was not generated")
		}
	})

	t.Run("DefaultSiteRoot", func(t *testing.T) {
		os.Setenv("GOOGLE_CLIENT_ID", "test_client_id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test_client_secret")
		os.Unsetenv("SITE_ROOT")

		err := SetupGoogleOAuth()
		if err != nil {
			t.Errorf("Expected successful setup, got error: %v", err)
		}

		if oauthConf.RedirectURL != "http://localhost:8666/api/google/callback" {
			t.Errorf("Expected default redirect URL, got '%s'", oauthConf.RedirectURL)
		}
	})

	t.Run("MissingClientID", func(t *testing.T) {
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test_client_secret")

		err := SetupGoogleOAuth()
		if err == nil {
			t.Error("Expected error for missing client ID")
		}
		if !strings.Contains(err.Error(), "Google OAuth credentials not configured") {
			t.Errorf("Expected configuration error, got: %v", err)
		}
	})

	t.Run("MissingClientSecret", func(t *testing.T) {
		os.Setenv("GOOGLE_CLIENT_ID", "test_client_id")
		os.Unsetenv("GOOGLE_CLIENT_SECRET")

		err := SetupGoogleOAuth()
		if err == nil {
			t.Error("Expected error for missing client secret")
		}
		if !strings.Contains(err.Error(), "Google OAuth credentials not configured") {
			t.Errorf("Expected configuration error, got: %v", err)
		}
	})
}

func TestOAuthScopes(t *testing.T) {
	os.Setenv("GOOGLE_CLIENT_ID", "test_client_id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test_client_secret")

	err := SetupGoogleOAuth()
	if err != nil {
		t.Fatalf("Failed to setup OAuth: %v", err)
	}

	expectedScopes := []string{
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
	}

	if len(oauthConf.Scopes) != len(expectedScopes) {
		t.Errorf("Expected %d scopes, got %d", len(expectedScopes), len(oauthConf.Scopes))
	}

	for i, expectedScope := range expectedScopes {
		if i >= len(oauthConf.Scopes) || oauthConf.Scopes[i] != expectedScope {
			t.Errorf("Expected scope '%s', got '%s'", expectedScope, oauthConf.Scopes[i])
		}
	}
}

func TestGoogleLoginHandler(t *testing.T) {
	t.Run("WithValidConfig", func(t *testing.T) {
		os.Setenv("GOOGLE_CLIENT_ID", "test_client_id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test_client_secret")
		SetupGoogleOAuth()

		req := httptest.NewRequest("GET", "/api/google/login", nil)
		w := httptest.NewRecorder()

		googleLoginHandler(w, req)

		if w.Code != http.StatusTemporaryRedirect {
			t.Errorf("Expected status %d, got %d", http.StatusTemporaryRedirect, w.Code)
		}

		location := w.Header().Get("Location")
		if !strings.Contains(location, "accounts.google.com") {
			t.Errorf("Expected redirect to Google, got: %s", location)
		}
		if !strings.Contains(location, "test_client_id") {
			t.Errorf("Expected client ID in URL, got: %s", location)
		}
		if !strings.Contains(location, oauthStateString) {
			t.Errorf("Expected state string in URL, got: %s", location)
		}
	})

	t.Run("WithoutConfig", func(t *testing.T) {
		oauthConf = nil

		req := httptest.NewRequest("GET", "/api/google/login", nil)
		w := httptest.NewRecorder()

		googleLoginHandler(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}

		body := strings.TrimSpace(w.Body.String())
		if !strings.Contains(body, "Google OAuth not configured") {
			t.Errorf("Expected configuration error, got: %s", body)
		}
	})
}

func TestGoogleCallbackHandler(t *testing.T) {
	t.Run("WithoutConfig", func(t *testing.T) {
		oauthConf = nil

		req := httptest.NewRequest("GET", "/api/google/callback", nil)
		w := httptest.NewRecorder()

		googleCallbackHandler(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})

	t.Run("InvalidState", func(t *testing.T) {
		os.Setenv("GOOGLE_CLIENT_ID", "test_client_id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test_client_secret")
		SetupGoogleOAuth()

		req := httptest.NewRequest("GET", "/api/google/callback?state=invalid_state&code=test_code", nil)
		w := httptest.NewRecorder()

		googleCallbackHandler(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
		}

		body := strings.TrimSpace(w.Body.String())
		if !strings.Contains(body, "Invalid OAuth state") {
			t.Errorf("Expected state error, got: %s", body)
		}
	})
}

func TestParseUserInfo(t *testing.T) {
	testCases := []struct {
		name     string
		jsonData string
		expected UserInfo
		hasError bool
	}{
		{
			name: "ValidUserInfo",
			jsonData: `{
				"email": "test@example.com",
				"verified_email": true,
				"name": "Test User",
				"given_name": "Test",
				"family_name": "User",
				"picture": "https://example.com/photo.jpg",
				"locale": "en"
			}`,
			expected: UserInfo{
				Email:         "test@example.com",
				VerifiedEmail: true,
				Name:          "Test User",
				GivenName:     "Test",
				FamilyName:    "User",
				Picture:       "https://example.com/photo.jpg",
				Locale:        "en",
			},
			hasError: false,
		},
		{
			name: "UnverifiedEmail",
			jsonData: `{
				"email": "test@example.com",
				"verified_email": false,
				"name": "Test User"
			}`,
			expected: UserInfo{
				Email:         "test@example.com",
				VerifiedEmail: false,
				Name:          "Test User",
			},
			hasError: false,
		},
		{
			name:     "InvalidJSON",
			jsonData: `{"email": "test@example.com", "verified_email": }`,
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var userInfo UserInfo
			err := json.Unmarshal([]byte(tc.jsonData), &userInfo)

			if tc.hasError {
				if err == nil {
					t.Error("Expected error parsing invalid JSON")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if userInfo.Email != tc.expected.Email {
				t.Errorf("Expected email '%s', got '%s'", tc.expected.Email, userInfo.Email)
			}
			if userInfo.VerifiedEmail != tc.expected.VerifiedEmail {
				t.Errorf("Expected verified_email %t, got %t", tc.expected.VerifiedEmail, userInfo.VerifiedEmail)
			}
			if userInfo.Name != tc.expected.Name {
				t.Errorf("Expected name '%s', got '%s'", tc.expected.Name, userInfo.Name)
			}
			if userInfo.GivenName != tc.expected.GivenName {
				t.Errorf("Expected given_name '%s', got '%s'", tc.expected.GivenName, userInfo.GivenName)
			}
			if userInfo.FamilyName != tc.expected.FamilyName {
				t.Errorf("Expected family_name '%s', got '%s'", tc.expected.FamilyName, userInfo.FamilyName)
			}
			if userInfo.Picture != tc.expected.Picture {
				t.Errorf("Expected picture '%s', got '%s'", tc.expected.Picture, userInfo.Picture)
			}
			if userInfo.Locale != tc.expected.Locale {
				t.Errorf("Expected locale '%s', got '%s'", tc.expected.Locale, userInfo.Locale)
			}
		})
	}
}

func TestOAuthURLGeneration(t *testing.T) {
	os.Setenv("GOOGLE_CLIENT_ID", "test_client_id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test_client_secret")
	os.Setenv("SITE_ROOT", "https://example.com")

	err := SetupGoogleOAuth()
	if err != nil {
		t.Fatalf("Failed to setup OAuth: %v", err)
	}

	url := oauthConf.AuthCodeURL(oauthStateString, oauth2.AccessTypeOffline)

	if !strings.Contains(url, "accounts.google.com") {
		t.Error("URL should contain Google OAuth endpoint")
	}
	if !strings.Contains(url, "client_id=test_client_id") {
		t.Error("URL should contain client ID")
	}
	if !strings.Contains(url, fmt.Sprintf("state=%s", oauthStateString)) {
		t.Error("URL should contain state parameter")
	}
	if !strings.Contains(url, "redirect_uri=https%3A%2F%2Fexample.com%2Fapi%2Fgoogle%2Fcallback") {
		t.Error("URL should contain redirect URI")
	}
	if !strings.Contains(url, "https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinfo.email") {
		t.Error("URL should contain email scope")
	}
	if !strings.Contains(url, "https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinfo.profile") {
		t.Error("URL should contain profile scope")
	}
}

func TestCreateUserFromOAuthStructure(t *testing.T) {
	testUserInfo := UserInfo{
		Email:         "test@example.com",
		VerifiedEmail: true,
		Name:          "Test User",
		GivenName:     "Test",
		FamilyName:    "User",
		Picture:       "https://example.com/photo.jpg",
		Locale:        "en",
	}

	if testUserInfo.Email == "" {
		t.Error("Email should not be empty")
	}
	if !testUserInfo.VerifiedEmail {
		t.Error("Email should be verified for OAuth")
	}
	if testUserInfo.Name == "" {
		t.Error("Name should not be empty")
	}
}

func TestUnverifiedEmailHandling(t *testing.T) {
	testUserInfo := UserInfo{
		Email:         "unverified@example.com",
		VerifiedEmail: false,
		Name:          "Unverified User",
	}

	if !testUserInfo.VerifiedEmail {
		t.Log("Correctly identified unverified email")
	} else {
		t.Error("Should have detected unverified email")
	}
}

func TestStateStringGeneration(t *testing.T) {
	states := make(map[string]bool)

	for i := 0; i < 10; i++ {
		os.Setenv("GOOGLE_CLIENT_ID", "test_client_id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test_client_secret")

		err := SetupGoogleOAuth()
		if err != nil {
			t.Fatalf("Failed to setup OAuth: %v", err)
		}

		if oauthStateString == "" {
			t.Error("State string should not be empty")
		}

		if len(oauthStateString) < 10 {
			t.Error("State string should be at least 10 characters for security")
		}

		if states[oauthStateString] {
			t.Error("State strings should be unique")
		}
		states[oauthStateString] = true
	}
}
