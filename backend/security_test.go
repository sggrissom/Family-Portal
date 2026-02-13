package backend

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.hasen.dev/vbeam"
)


func TestIsWebSocketRequest(t *testing.T) {
	tests := []struct {
		name     string
		conn     string
		upgrade  string
		expected bool
	}{
		{name: "standard websocket headers", conn: "Upgrade", upgrade: "websocket", expected: true},
		{name: "case insensitive and trimmed upgrade", conn: "keep-alive, UpGrAdE", upgrade: "  WebSocket  ", expected: true},
		{name: "missing connection upgrade token", conn: "keep-alive", upgrade: "websocket", expected: false},
		{name: "wrong upgrade type", conn: "Upgrade", upgrade: "h2c", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws/chat", nil)
			req.Header.Set("Connection", tt.conn)
			req.Header.Set("Upgrade", tt.upgrade)

			actual := isWebSocketRequest(req)
			if actual != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestNewSecurityWrapper(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()
	wrapper := NewSecurityWrapper(app)

	if wrapper == nil {
		t.Error("Expected NewSecurityWrapper to return a non-nil wrapper")
	}

	if wrapper.app != app {
		t.Error("Expected wrapper to contain the original app")
	}
}

func TestAddSecurityHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()

	addSecurityHeaders(recorder)

	headers := recorder.Header()

	t.Run("X-Content-Type-Options", func(t *testing.T) {
		expected := "nosniff"
		actual := headers.Get("X-Content-Type-Options")
		if actual != expected {
			t.Errorf("Expected X-Content-Type-Options '%s', got '%s'", expected, actual)
		}
	})

	t.Run("X-Frame-Options", func(t *testing.T) {
		expected := "DENY"
		actual := headers.Get("X-Frame-Options")
		if actual != expected {
			t.Errorf("Expected X-Frame-Options '%s', got '%s'", expected, actual)
		}
	})

	t.Run("X-XSS-Protection", func(t *testing.T) {
		expected := "1; mode=block"
		actual := headers.Get("X-XSS-Protection")
		if actual != expected {
			t.Errorf("Expected X-XSS-Protection '%s', got '%s'", expected, actual)
		}
	})

	t.Run("Referrer-Policy", func(t *testing.T) {
		expected := "strict-origin-when-cross-origin"
		actual := headers.Get("Referrer-Policy")
		if actual != expected {
			t.Errorf("Expected Referrer-Policy '%s', got '%s'", expected, actual)
		}
	})

	t.Run("Permissions-Policy", func(t *testing.T) {
		expected := "geolocation=(), microphone=(), camera=()"
		actual := headers.Get("Permissions-Policy")
		if actual != expected {
			t.Errorf("Expected Permissions-Policy '%s', got '%s'", expected, actual)
		}
	})

	t.Run("Content-Security-Policy", func(t *testing.T) {
		actual := headers.Get("Content-Security-Policy")

		// Check that CSP contains expected directives
		expectedDirectives := []string{
			"default-src 'self'",
			"script-src 'self' 'unsafe-inline'",
			"style-src 'self' 'unsafe-inline'",
			"img-src 'self' data: blob:",
			"font-src 'self'",
			"connect-src 'self'",
			"frame-ancestors 'none'",
		}

		for _, directive := range expectedDirectives {
			if !contains(actual, directive) {
				t.Errorf("Expected CSP to contain '%s', but got: %s", directive, actual)
			}
		}
	})
}

func TestSecurityWrapperServeHTTP(t *testing.T) {
	// Create a simple test handler that writes a response
	testApp, cleanup := setupTestApp(t)
	defer cleanup()
	testApp.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	wrapper := NewSecurityWrapper(testApp)

	// Test request
	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	wrapper.ServeHTTP(recorder, req)

	t.Run("Response status and body", func(t *testing.T) {
		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}

		body := recorder.Body.String()
		if body != "test response" {
			t.Errorf("Expected body 'test response', got '%s'", body)
		}
	})

	t.Run("Security headers are applied", func(t *testing.T) {
		headers := recorder.Header()

		// Verify key security headers are present
		if headers.Get("X-Content-Type-Options") != "nosniff" {
			t.Error("Expected X-Content-Type-Options header to be set")
		}

		if headers.Get("X-Frame-Options") != "DENY" {
			t.Error("Expected X-Frame-Options header to be set")
		}

		if headers.Get("Content-Security-Policy") == "" {
			t.Error("Expected Content-Security-Policy header to be set")
		}
	})
}

func TestSecurityWrapperWithDifferentRoutes(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expectedBody   string
		expectedStatus int
		setupRoute     func(*vbeam.Application)
	}{
		{
			"Home route",
			"/home",
			"home",
			http.StatusOK,
			func(app *vbeam.Application) {
				app.HandleFunc("/home", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("home"))
				})
			},
		},
		{
			"API route",
			"/api/test",
			`{"status": "ok"}`,
			http.StatusOK,
			func(app *vbeam.Application) {
				app.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"status": "ok"}`))
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh app for each test case to avoid route conflicts
			testApp, cleanup := setupTestApp(t)
			defer cleanup()

			// Setup the specific route for this test
			tt.setupRoute(testApp)

			wrapper := NewSecurityWrapper(testApp)

			req := httptest.NewRequest("GET", tt.path, nil)
			recorder := httptest.NewRecorder()

			wrapper.ServeHTTP(recorder, req)

			if recorder.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, recorder.Code)
			}

			body := recorder.Body.String()
			if body != tt.expectedBody {
				t.Errorf("Expected body '%s', got '%s'", tt.expectedBody, body)
			}

			// Verify security headers are always present
			headers := recorder.Header()
			if headers.Get("X-Frame-Options") != "DENY" {
				t.Error("Expected X-Frame-Options header to be set on all routes")
			}
		})
	}
}

func TestSecurityHeadersWithCustomHeaders(t *testing.T) {
	// Test that security headers don't interfere with custom application headers
	testApp, cleanup := setupTestApp(t)
	defer cleanup()
	testApp.HandleFunc("/custom", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Custom-Header", "custom-value")
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("custom response"))
	})

	wrapper := NewSecurityWrapper(testApp)

	req := httptest.NewRequest("GET", "/custom", nil)
	recorder := httptest.NewRecorder()

	wrapper.ServeHTTP(recorder, req)

	headers := recorder.Header()

	// Verify custom headers are preserved
	if headers.Get("Custom-Header") != "custom-value" {
		t.Error("Expected custom header to be preserved")
	}

	if headers.Get("Content-Type") != "text/plain" {
		t.Error("Expected Content-Type header to be preserved")
	}

	// Verify security headers are still added
	if headers.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("Expected security headers to be added alongside custom headers")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
