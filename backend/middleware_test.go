package backend

import (
	"context"
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

func TestAuthMiddleware(t *testing.T) {
	testDBPath := "test_middleware_auth.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	// Set the global database for the middleware functions
	appDb = db

	var testUser User

	// Create a test user
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq := CreateAccountRequest{
			Name:            "Test User",
			Email:           "middleware@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		testUser = AddUserTx(tx, userReq, hash)
		vbolt.TxCommit(tx)
	})

	// Generate a valid JWT token for the user
	validToken, err := generateAuthJwt(testUser, httptest.NewRecorder())
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Test handler that checks if user is in context
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r)
		if !ok {
			http.Error(w, "No user in context", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("User ID: " + string(rune(user.Id))))
	}

	wrappedHandler := AuthMiddleware(testHandler)

	t.Run("Valid token in cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.AddCookie(&http.Cookie{
			Name:  "authToken",
			Value: validToken,
		})
		recorder := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}
	})

	t.Run("Valid token in Authorization header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		recorder := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}
	})

	t.Run("No token provided", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		recorder := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", recorder.Code)
		}
	})

	t.Run("Invalid token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.AddCookie(&http.Cookie{
			Name:  "authToken",
			Value: "invalid.token.here",
		})
		recorder := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", recorder.Code)
		}
	})

	t.Run("Expired token", func(t *testing.T) {
		// Create an expired token
		expiredClaims := &Claims{
			Username: testUser.Email,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
		}
		expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
		expiredTokenString, _ := expiredToken.SignedString(jwtKey)

		req := httptest.NewRequest("GET", "/protected", nil)
		req.AddCookie(&http.Cookie{
			Name:  "authToken",
			Value: expiredTokenString,
		})
		recorder := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", recorder.Code)
		}
	})
}

func TestAuthenticateRequest(t *testing.T) {
	testDBPath := "test_authenticate_request.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	// Set the global database
	appDb = db

	var testUser User

	// Create a test user
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq := CreateAccountRequest{
			Name:            "Auth Test User",
			Email:           "auth@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		testUser = AddUserTx(tx, userReq, hash)
		vbolt.TxCommit(tx)
	})

	// Generate a valid JWT token
	validToken, err := generateAuthJwt(testUser, httptest.NewRecorder())
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	t.Run("Valid token authentication", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  "authToken",
			Value: validToken,
		})

		user, err := AuthenticateRequest(req)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if user.Id != testUser.Id {
			t.Errorf("Expected user ID %d, got %d", testUser.Id, user.Id)
		}

		if user.Email != testUser.Email {
			t.Errorf("Expected email %s, got %s", testUser.Email, user.Email)
		}
	})

	t.Run("No token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)

		_, err := AuthenticateRequest(req)
		if err == nil {
			t.Error("Expected error for missing token")
		}

		expectedError := "no auth token found"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
		}
	})

	t.Run("Token for non-existent user", func(t *testing.T) {
		// Create token for non-existent user
		nonExistentClaims := &Claims{
			Username: "nonexistent@example.com",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}
		nonExistentToken := jwt.NewWithClaims(jwt.SigningMethodHS256, nonExistentClaims)
		nonExistentTokenString, _ := nonExistentToken.SignedString(jwtKey)

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  "authToken",
			Value: nonExistentTokenString,
		})

		_, err := AuthenticateRequest(req)
		if err == nil {
			t.Error("Expected error for non-existent user")
		}

		expectedError := "user not found"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
		}
	})
}

func TestExtractToken(t *testing.T) {
	testCases := []struct {
		name           string
		setupRequest   func(*http.Request)
		expectedToken  string
		expectedResult bool
	}{
		{
			name: "Token from cookie",
			setupRequest: func(req *http.Request) {
				req.AddCookie(&http.Cookie{
					Name:  "authToken",
					Value: "cookie-token-123",
				})
			},
			expectedToken:  "cookie-token-123",
			expectedResult: true,
		},
		{
			name: "Token from Authorization header",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer header-token-456")
			},
			expectedToken:  "header-token-456",
			expectedResult: true,
		},
		{
			name: "Cookie takes precedence over header",
			setupRequest: func(req *http.Request) {
				req.AddCookie(&http.Cookie{
					Name:  "authToken",
					Value: "cookie-token",
				})
				req.Header.Set("Authorization", "Bearer header-token")
			},
			expectedToken:  "cookie-token",
			expectedResult: true,
		},
		{
			name: "Empty cookie, valid header",
			setupRequest: func(req *http.Request) {
				req.AddCookie(&http.Cookie{
					Name:  "authToken",
					Value: "",
				})
				req.Header.Set("Authorization", "Bearer header-token")
			},
			expectedToken:  "header-token",
			expectedResult: true,
		},
		{
			name: "Invalid Authorization header format",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "InvalidFormat token")
			},
			expectedToken:  "",
			expectedResult: false,
		},
		{
			name: "No token anywhere",
			setupRequest: func(req *http.Request) {
				// No cookies or headers set
			},
			expectedToken:  "",
			expectedResult: false,
		},
		{
			name: "Case insensitive Bearer",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "bearer lowercase-token")
			},
			expectedToken:  "lowercase-token",
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			tc.setupRequest(req)

			token := extractToken(req)

			if tc.expectedResult {
				if token != tc.expectedToken {
					t.Errorf("Expected token '%s', got '%s'", tc.expectedToken, token)
				}
			} else {
				if token != "" {
					t.Errorf("Expected empty token, got '%s'", token)
				}
			}
		})
	}
}

func TestGetUserFromContext(t *testing.T) {
	testUser := User{
		Id:       123,
		Name:     "Context Test User",
		Email:    "context@example.com",
		FamilyId: 1,
	}

	t.Run("User in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		ctx := context.WithValue(req.Context(), UserContextKey, testUser)
		req = req.WithContext(ctx)

		user, ok := GetUserFromContext(req)
		if !ok {
			t.Error("Expected user to be found in context")
		}

		if user.Id != testUser.Id {
			t.Errorf("Expected user ID %d, got %d", testUser.Id, user.Id)
		}

		if user.Email != testUser.Email {
			t.Errorf("Expected email %s, got %s", testUser.Email, user.Email)
		}
	})

	t.Run("No user in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)

		_, ok := GetUserFromContext(req)
		if ok {
			t.Error("Expected no user to be found in context")
		}
	})

	t.Run("Wrong type in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		ctx := context.WithValue(req.Context(), UserContextKey, "not-a-user")
		req = req.WithContext(ctx)

		_, ok := GetUserFromContext(req)
		if ok {
			t.Error("Expected no user to be found with wrong type in context")
		}
	})
}

func TestRequireAdmin(t *testing.T) {
	testDBPath := "test_require_admin.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	// Set the global database
	appDb = db

	var adminUser, regularUser User

	// Create users
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		// Create admin user (ID = 1)
		adminReq := CreateAccountRequest{
			Name:            "Admin User",
			Email:           "admin@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(adminReq.Password), bcrypt.DefaultCost)
		adminUser = AddUserTx(tx, adminReq, hash)

		// Ensure admin user has ID = 1 (admin check)
		if adminUser.Id != 1 {
			// Manually set ID to 1 for admin test
			adminUser.Id = 1
			vbolt.Write(tx, UsersBkt, 1, &adminUser)
		}

		// Create regular user
		regularReq := CreateAccountRequest{
			Name:            "Regular User",
			Email:           "regular@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash2, _ := bcrypt.GenerateFromPassword([]byte(regularReq.Password), bcrypt.DefaultCost)
		regularUser = AddUserTx(tx, regularReq, hash2)
		vbolt.TxCommit(tx)
	})

	// Generate tokens
	adminToken, err := generateAuthJwt(adminUser, httptest.NewRecorder())
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}

	regularToken, err := generateAuthJwt(regularUser, httptest.NewRecorder())
	if err != nil {
		t.Fatalf("Failed to generate regular token: %v", err)
	}

	// Test handler
	adminHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Admin access granted"))
	}

	wrappedAdminHandler := RequireAdmin(adminHandler)

	t.Run("Admin user access", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin", nil)
		req.AddCookie(&http.Cookie{
			Name:  "authToken",
			Value: adminToken,
		})
		recorder := httptest.NewRecorder()

		wrappedAdminHandler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200 for admin, got %d", recorder.Code)
		}

		expectedBody := "Admin access granted"
		if !strings.Contains(recorder.Body.String(), expectedBody) {
			t.Errorf("Expected body to contain '%s', got '%s'", expectedBody, recorder.Body.String())
		}
	})

	t.Run("Regular user denied", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin", nil)
		req.AddCookie(&http.Cookie{
			Name:  "authToken",
			Value: regularToken,
		})
		recorder := httptest.NewRecorder()

		wrappedAdminHandler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusForbidden {
			t.Errorf("Expected status 403 for regular user, got %d", recorder.Code)
		}
	})

	t.Run("Unauthenticated user denied", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin", nil)
		recorder := httptest.NewRecorder()

		wrappedAdminHandler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for unauthenticated user, got %d", recorder.Code)
		}
	})
}
