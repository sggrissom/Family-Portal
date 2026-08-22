package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testIOSAppID = "ABCDE12345.app.familyrecord.ios"

// fetchAssociation serves the association file and returns the recorder plus the
// decoded document. The document is decoded generically rather than into
// appSiteAssociation, because what Apple reads is the JSON, and a wrong key
// name would round-trip through the Go type without anyone noticing.
func fetchAssociation(t *testing.T) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, AppSiteAssociationPath, nil)
	rec := httptest.NewRecorder()
	appSiteAssociationHandler(rec, req)

	if rec.Code != http.StatusOK {
		return rec, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("association is not valid JSON: %v", err)
	}
	return rec, doc
}

func TestAppSiteAssociationUnconfigured(t *testing.T) {
	t.Setenv("IOS_APP_ID", "")

	rec, _ := fetchAssociation(t)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 with no IOS_APP_ID, got %d", rec.Code)
	}
}

func TestAppSiteAssociationMalformedIDIsNotServed(t *testing.T) {
	// A wrong appID is cached by Apple and by every device that fetched it, so
	// each of these must produce nothing rather than something plausible.
	for _, appID := range []string{
		"app.familyrecord.ios",               // no team prefix
		"ABCDE1234.app.familyrecord",         // nine-character team id
		"abcde12345.app.familyrecord",        // lowercase team id
		"ABCDE12345.",                        // no bundle id
		"ABCDE12345.app familyrecord",        // space
		"ABCDE12345.app.familyrecord/photos", // a URL pasted where an id was wanted
	} {
		t.Run(appID, func(t *testing.T) {
			t.Setenv("IOS_APP_ID", appID)
			rec, _ := fetchAssociation(t)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("expected 404 for %q, got %d", appID, rec.Code)
			}
		})
	}
}

func TestIOSAppIDTrimsSurroundingWhitespace(t *testing.T) {
	t.Setenv("IOS_APP_ID", "  "+testIOSAppID+"  ")
	if got := IOSAppID(); got != testIOSAppID {
		t.Fatalf("IOSAppID() = %q, want %q", got, testIOSAppID)
	}
	rec, _ := fetchAssociation(t)
	if rec.Code != http.StatusOK {
		t.Fatalf("a padded but valid id should still be served, got %d", rec.Code)
	}
}

func TestAppSiteAssociationHeaders(t *testing.T) {
	t.Setenv("IOS_APP_ID", testIOSAppID)
	rec, _ := fetchAssociation(t)

	// Apple refuses anything that is not application/json, and refuses a
	// redirect outright.
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if got := rec.Header().Get("Cache-Control"); got == "" {
		t.Error("expected a Cache-Control header")
	}
}

func TestAppSiteAssociationShape(t *testing.T) {
	t.Setenv("IOS_APP_ID", testIOSAppID)
	_, doc := fetchAssociation(t)

	applinks, ok := doc["applinks"].(map[string]any)
	if !ok {
		t.Fatal("missing applinks object")
	}
	// The legacy key must be present and empty; iOS treats its absence as a
	// malformed document.
	apps, ok := applinks["apps"].([]any)
	if !ok || len(apps) != 0 {
		t.Errorf("applinks.apps = %v, want an empty array", applinks["apps"])
	}

	details, ok := applinks["details"].([]any)
	if !ok || len(details) != 1 {
		t.Fatalf("expected exactly one details entry, got %v", applinks["details"])
	}
	detail := details[0].(map[string]any)
	appIDs := detail["appIDs"].([]any)
	if len(appIDs) != 1 || appIDs[0] != testIOSAppID {
		t.Errorf("appIDs = %v, want [%s]", appIDs, testIOSAppID)
	}

	creds, ok := doc["webcredentials"].(map[string]any)
	if !ok {
		t.Fatal("missing webcredentials; the app's login cannot offer the saved website password without it")
	}
	credApps := creds["apps"].([]any)
	if len(credApps) != 1 || credApps[0] != testIOSAppID {
		t.Errorf("webcredentials.apps = %v, want [%s]", credApps, testIOSAppID)
	}
}

// associationComponents returns the components list as Apple would read it.
func associationComponents(t *testing.T, doc map[string]any) []map[string]any {
	t.Helper()
	details := doc["applinks"].(map[string]any)["details"].([]any)
	raw := details[0].(map[string]any)["components"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, c := range raw {
		out = append(out, c.(map[string]any))
	}
	return out
}

func TestAppSiteAssociationExclusionsComeFirst(t *testing.T) {
	t.Setenv("IOS_APP_ID", testIOSAppID)
	_, doc := fetchAssociation(t)
	components := associationComponents(t, doc)

	// iOS takes the first matching component. An exclusion listed after an
	// allow rule that also matches never runs, which is how /reset-password
	// ends up opening an app with no reset screen.
	seenAllow := false
	for _, c := range components {
		excluded, _ := c["exclude"].(bool)
		if excluded && seenAllow {
			t.Fatalf("exclusion %q is listed after an allow rule", c["/"])
		}
		if !excluded {
			seenAllow = true
		}
	}
}

func TestAppSiteAssociationExcludesFlowsThatMustStayInTheBrowser(t *testing.T) {
	t.Setenv("IOS_APP_ID", testIOSAppID)
	_, doc := fetchAssociation(t)
	components := associationComponents(t, doc)

	required := map[string]bool{
		// A single-use token from an email, answered by a page the app does not
		// have.
		"/reset-password": false,
		// Includes the Google OAuth callback, which has to finish in the browser
		// session that started it.
		"/api/*": false,
	}
	for _, c := range components {
		path, _ := c["/"].(string)
		if excluded, _ := c["exclude"].(bool); excluded {
			if _, wanted := required[path]; wanted {
				required[path] = true
			}
		}
	}
	for path, found := range required {
		if !found {
			t.Errorf("%s is not excluded; following it on a device would open the app instead of the browser", path)
		}
	}
}

func TestAppSiteAssociationClaimsThePushDestinations(t *testing.T) {
	t.Setenv("IOS_APP_ID", testIOSAppID)
	_, doc := fetchAssociation(t)

	claimed := map[string]bool{}
	for _, c := range associationComponents(t, doc) {
		if excluded, _ := c["exclude"].(bool); excluded {
			continue
		}
		path, _ := c["/"].(string)
		claimed[path] = true
	}

	// Every destination the push worker can send has to be a path the app is
	// allowed to open, or tapping that notification lands in Safari.
	for event, spec := range pushEventSpecs {
		if !claimed[spec.Destination] {
			t.Errorf("push event %q has destination %q, which the association does not claim", event, spec.Destination)
		}
	}
}

func TestCheckIOSAppIDOptionalButValidated(t *testing.T) {
	t.Run("unset is not an issue", func(t *testing.T) {
		t.Setenv("IOS_APP_ID", "")
		if issues := checkIOSAppID(); len(issues) != 0 {
			t.Errorf("expected no issues, got %v", issues)
		}
	})

	t.Run("valid is not an issue", func(t *testing.T) {
		t.Setenv("IOS_APP_ID", testIOSAppID)
		if issues := checkIOSAppID(); len(issues) != 0 {
			t.Errorf("expected no issues, got %v", issues)
		}
	})

	t.Run("malformed is reported at startup", func(t *testing.T) {
		t.Setenv("IOS_APP_ID", "familyrecord")
		issues := checkIOSAppID()
		if len(issues) != 1 || issues[0].Setting != "IOS_APP_ID" {
			t.Fatalf("expected one IOS_APP_ID issue, got %v", issues)
		}
		// The report goes in a startup log; it must not echo the value back.
		if got := issues[0].Detail; got == "" {
			t.Error("expected a detail explaining the expected form")
		}
	})
}

// TestAppSiteAssociationIsReachableThroughTheApplication exercises the route as
// registered rather than the handler in isolation. Apple fetches this path
// exactly once per app install and will not follow a redirect or accept a 404,
// so "the handler is correct but nothing routes to it" is a failure mode worth
// having a test for.
func TestAppSiteAssociationIsReachableThroughTheApplication(t *testing.T) {
	t.Setenv("IOS_APP_ID", testIOSAppID)

	app, cleanup := setupTestApp(t)
	defer cleanup()
	RegisterUniversalLinkHandlers(app)

	req := httptest.NewRequest(http.MethodGet, AppSiteAssociationPath, nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s through the application returned %d", AppSiteAssociationPath, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}
