package backend

import (
	"family/cfg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegisterSEOHandlers(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()
	RegisterSEOHandlers(app)

	// Test that handlers are registered (we can't directly test registration,
	// but we can test that the routes work)
	t.Run("Handlers registered successfully", func(t *testing.T) {
		// This test mainly ensures no panic occurs during registration
		// The actual functionality is tested in individual handler tests
		// Note: manifest.json is now served as a static file, not via handler
	})
}

func TestRobotsHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/robots.txt", nil)
	recorder := httptest.NewRecorder()

	robotsHandler(recorder, req)

	t.Run("Status code", func(t *testing.T) {
		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}
	})

	t.Run("Content-Type header", func(t *testing.T) {
		expected := "text/plain"
		actual := recorder.Header().Get("Content-Type")
		if actual != expected {
			t.Errorf("Expected Content-Type '%s', got '%s'", expected, actual)
		}
	})

	t.Run("Cache-Control header", func(t *testing.T) {
		expected := "public, max-age=86400"
		actual := recorder.Header().Get("Cache-Control")
		if actual != expected {
			t.Errorf("Expected Cache-Control '%s', got '%s'", expected, actual)
		}
	})

	// robots.txt denies by default. The old assertions checked that a handful
	// of private paths were listed, which said nothing about the ones nobody
	// had thought of — and every route in this application except seven is
	// somebody's family.
	t.Run("Denies everything by default", func(t *testing.T) {
		body := recorder.Body.String()

		required := []string{
			"User-agent: *",
			"Disallow: /",
			"Sitemap: " + cfg.SiteURL + "/sitemap.xml",
			"Crawl-delay: 10",
		}
		for _, expected := range required {
			if !strings.Contains(body, expected) {
				t.Errorf("robots.txt is missing %q:\n%s", expected, body)
			}
		}

		// A bare "Allow: /" would undo the blanket disallow for every path.
		// The root is allowed as "Allow: /$", which matches only the landing
		// page.
		for _, line := range strings.Split(body, "\n") {
			if strings.TrimSpace(line) == "Allow: /" {
				t.Errorf("robots.txt re-allows the whole site:\n%s", body)
			}
		}
	})

	t.Run("Allows exactly the public pages", func(t *testing.T) {
		body := recorder.Body.String()

		for _, public := range []string{"/$", "/login", "/create-account", "/forgot-password", "/privacy", "/terms", "/support"} {
			if !strings.Contains(body, "Allow: "+public) {
				t.Errorf("robots.txt does not allow the public page %q:\n%s", public, body)
			}
		}

		// Nothing behind authentication may be allowed back in.
		for _, private := range []string{"/dashboard", "/settings", "/photos", "/profile", "/chat", "/admin"} {
			if strings.Contains(body, "Allow: "+private) {
				t.Errorf("robots.txt allows the private path %q:\n%s", private, body)
			}
		}
	})
}

func TestSitemapHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/sitemap.xml", nil)
	recorder := httptest.NewRecorder()

	sitemapHandler(recorder, req)

	t.Run("Status code", func(t *testing.T) {
		if recorder.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", recorder.Code)
		}
	})

	t.Run("Content-Type header", func(t *testing.T) {
		expected := "application/xml"
		actual := recorder.Header().Get("Content-Type")
		if actual != expected {
			t.Errorf("Expected Content-Type '%s', got '%s'", expected, actual)
		}
	})

	t.Run("Cache-Control header", func(t *testing.T) {
		expected := "public, max-age=86400"
		actual := recorder.Header().Get("Cache-Control")
		if actual != expected {
			t.Errorf("Expected Cache-Control '%s', got '%s'", expected, actual)
		}
	})

	t.Run("Valid XML structure", func(t *testing.T) {
		body := recorder.Body.String()

		expectedContent := []string{
			`<?xml version="1.0" encoding="UTF-8"?>`,
			`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`,
			`<url>`,
			`<loc>` + cfg.SiteURL + `/</loc>`,
			`<changefreq>weekly</changefreq>`,
			`<priority>1.0</priority>`,
			`<loc>` + cfg.SiteURL + `/login</loc>`,
			`<loc>` + cfg.SiteURL + `/create-account</loc>`,
			// The policy pages are the only ones a stranger has any reason to
			// find, and a privacy page nobody can reach is not a privacy page.
			`<loc>` + cfg.SiteURL + `/privacy</loc>`,
			`<loc>` + cfg.SiteURL + `/terms</loc>`,
			`<loc>` + cfg.SiteURL + `/support</loc>`,
			`</urlset>`,
		}

		for _, expected := range expectedContent {
			if !strings.Contains(body, expected) {
				t.Errorf("Expected sitemap.xml to contain '%s', but got: %s", expected, body)
			}
		}
	})

	t.Run("Contains current date", func(t *testing.T) {
		body := recorder.Body.String()

		// Check that lastmod contains a date in YYYY-MM-DD format
		if !strings.Contains(body, "<lastmod>") {
			t.Error("Expected sitemap.xml to contain lastmod tags")
		}

		// Check that the date format is reasonable (contains dashes)
		if !strings.Contains(body, "-") {
			t.Error("Expected sitemap.xml to contain properly formatted dates")
		}
	})
}

// TestManifestHandler removed - manifest.json is now served as a static file from frontend/manifest.json

func TestSEOHandlersIntegration(t *testing.T) {
	// Test that all SEO handlers work when registered with a real vbeam app
	app, cleanup := setupTestApp(t)
	defer cleanup()
	RegisterSEOHandlers(app)

	tests := []struct {
		name                string
		path                string
		expectedStatus      int
		expectedContentType string
	}{
		{"Robots.txt", "/robots.txt", http.StatusOK, "text/plain"},
		{"Sitemap.xml", "/sitemap.xml", http.StatusOK, "application/xml"},
		// Manifest.json is now served as a static file, not via handler
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			recorder := httptest.NewRecorder()

			app.ServeHTTP(recorder, req)

			if recorder.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d for %s", tt.expectedStatus, recorder.Code, tt.path)
			}

			contentType := recorder.Header().Get("Content-Type")
			if contentType != tt.expectedContentType {
				t.Errorf("Expected Content-Type '%s', got '%s' for %s", tt.expectedContentType, contentType, tt.path)
			}

			// Verify all responses have cache headers
			cacheControl := recorder.Header().Get("Cache-Control")
			if cacheControl != "public, max-age=86400" {
				t.Errorf("Expected proper cache headers for %s", tt.path)
			}
		})
	}
}
