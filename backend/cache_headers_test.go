package backend

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrivateEndpointsAreNotCached(t *testing.T) {
	paths := []string{
		"/api/export-bundle",
		"/api/import-bundle",
		"/api/delete-account",
		"/api/change-password",
		"/rpc/GetDashboard",
		"/rpc/GetChatMessages",
		"/internal/snapshot",
		"/api/something-invented-next-year",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			addCacheDefaults(recorder, httptest.NewRequest(http.MethodGet, path, nil))

			if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "no-store")
			}
		})
	}
}

func TestStaticAssetsKeepTheirOwnCaching(t *testing.T) {
	for _, path := range []string{"/", "/login", "/main-ABC123.js", "/images/og-image.png", "/manifest.json"} {
		recorder := httptest.NewRecorder()
		addCacheDefaults(recorder, httptest.NewRequest(http.MethodGet, path, nil))

		if got := recorder.Header().Get("Cache-Control"); got != "" {
			t.Errorf("%s: Cache-Control = %q, want it left alone", path, got)
		}
	}
}

func TestPhotoCachingIsPrivateAndRevalidated(t *testing.T) {
	header := photoCacheControl

	if !strings.Contains(header, "private") {
		t.Error("photo caching must be private; a shared cache would hold another family's photos")
	}
	if strings.Contains(header, "immutable") {
		t.Error("the photo URL carries no content hash, so immutable would strand a reprocessed photo")
	}
	if strings.Contains(header, "public") {
		t.Error("photos are not public")
	}
}
