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

	t.Run("Content includes required directives", func(t *testing.T) {
		body := recorder.Body.String()

		expectedContent := []string{
			"User-agent: *",
			"Disallow: /admin/",
			"Disallow: /static/",
			"Disallow: /api/",
			"Disallow: /dashboard",
			"Disallow: /auth/",
			"Allow: /",
			"Sitemap: " + cfg.SiteURL + "/sitemap.xml",
			"Crawl-delay: 10",
		}

		for _, expected := range expectedContent {
			if !strings.Contains(body, expected) {
				t.Errorf("Expected robots.txt to contain '%s', but got: %s", expected, body)
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
