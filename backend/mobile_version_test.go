package backend

import (
	"encoding/json"
	"family/cfg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func TestPlatformID(t *testing.T) {
	testCases := []struct {
		name     string
		platform string
		expected int
	}{
		{name: "iOS platform", platform: "ios", expected: 1},
		{name: "Android platform", platform: "android", expected: 2},
		{name: "Unknown platform", platform: "web", expected: 0},
		{name: "Empty platform", platform: "", expected: 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := platformId(tc.platform)
			if result != tc.expected {
				t.Errorf("Expected platformId(%q) to be %d, got %d", tc.platform, tc.expected, result)
			}
		})
	}
}

func TestCompareSemver(t *testing.T) {
	testCases := []struct {
		name     string
		a        string
		b        string
		expected int
	}{
		{name: "equal versions", a: "1.2.3", b: "1.2.3", expected: 0},
		{name: "major version less", a: "1.2.3", b: "2.0.0", expected: -1},
		{name: "major version greater", a: "3.0.0", b: "2.9.9", expected: 1},
		{name: "minor version less", a: "1.2.3", b: "1.3.0", expected: -1},
		{name: "minor version greater", a: "1.4.0", b: "1.3.9", expected: 1},
		{name: "patch version less", a: "1.2.3", b: "1.2.4", expected: -1},
		{name: "patch version greater", a: "1.2.5", b: "1.2.4", expected: 1},
		{name: "large numeric component", a: "10000000000.0.0", b: "9999999999.9.9", expected: 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := compareSemver(tc.a, tc.b)
			if result != tc.expected {
				t.Errorf("Expected compareSemver(%q, %q) to be %d, got %d", tc.a, tc.b, tc.expected, result)
			}
		})
	}
}

func TestIsValidSemver(t *testing.T) {
	testCases := []struct {
		name     string
		version  string
		expected bool
	}{
		{name: "valid semver", version: "1.2.3", expected: true},
		{name: "zero semver", version: "0.0.0", expected: true},
		{name: "missing patch", version: "1.2", expected: false},
		{name: "too many parts", version: "1.2.3.4", expected: false},
		{name: "non-numeric part", version: "1.a.3", expected: false},
		{name: "prefixed with v", version: "v1.2.3", expected: false},
		{name: "prerelease metadata", version: "1.2.3-beta.1", expected: false},
		{name: "build metadata", version: "1.2.3+42", expected: false},
		{name: "leading zero", version: "1.02.3", expected: false},
		{name: "negative component", version: "1.-2.3", expected: false},
		{name: "positive sign", version: "1.+2.3", expected: false},
		{name: "numeric overflow", version: "18446744073709551616.0.0", expected: false},
		{name: "empty string", version: "", expected: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isValidSemver(tc.version)
			if result != tc.expected {
				t.Errorf("Expected isValidSemver(%q) to be %t, got %t", tc.version, tc.expected, result)
			}
		})
	}
}

func TestValidateMobileVersionRange(t *testing.T) {
	testCases := []struct {
		name           string
		minimumVersion string
		latestVersion  string
		wantError      bool
	}{
		{name: "ordered versions", minimumVersion: "1.2.0", latestVersion: "1.3.0"},
		{name: "equal versions", minimumVersion: "1.2.0", latestVersion: "1.2.0"},
		{name: "minimum omitted", latestVersion: "1.3.0"},
		{name: "latest omitted", minimumVersion: "1.2.0"},
		{name: "minimum exceeds latest", minimumVersion: "2.0.0", latestVersion: "1.9.9", wantError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMobileVersionRange(tc.minimumVersion, tc.latestVersion)
			if (err != nil) != tc.wantError {
				t.Fatalf("validateMobileVersionRange(%q, %q) error = %v, wantError = %t", tc.minimumVersion, tc.latestVersion, err, tc.wantError)
			}
		})
	}
}

func TestEvaluateMobileVersion(t *testing.T) {
	configured := MobileVersionConfig{
		Id:             1,
		Platform:       "ios",
		MinimumVersion: "2.1.0",
		LatestVersion:  "2.3.0",
		UpdateUrl:      "https://apps.apple.com/app/id123456789",
		UpdateMessage:  "A newer version is available.",
	}
	testCases := []struct {
		name       string
		appVersion string
		config     MobileVersionConfig
		wantStatus string
	}{
		{name: "missing configuration permits app", appVersion: "1.0.0", wantStatus: "ok"},
		{name: "current version is ok", appVersion: "2.3.0", config: configured, wantStatus: "ok"},
		{name: "newer version is ok", appVersion: "3.0.0", config: configured, wantStatus: "ok"},
		{name: "older supported version gets optional update", appVersion: "2.2.0", config: configured, wantStatus: "update_available"},
		{name: "minimum version remains supported", appVersion: "2.1.0", config: configured, wantStatus: "update_available"},
		{name: "downgrade below minimum requires update", appVersion: "2.0.9", config: configured, wantStatus: "update_required"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluateMobileVersion(tc.appVersion, tc.config)
			if got.Status != tc.wantStatus {
				t.Fatalf("evaluateMobileVersion(%q) status = %q, want %q", tc.appVersion, got.Status, tc.wantStatus)
			}
			if tc.config.Id != 0 {
				if got.MinimumVersion != tc.config.MinimumVersion || got.LatestVersion != tc.config.LatestVersion {
					t.Errorf("response version policy = %q/%q, want %q/%q", got.MinimumVersion, got.LatestVersion, tc.config.MinimumVersion, tc.config.LatestVersion)
				}
				if got.UpdateUrl != tc.config.UpdateUrl || got.UpdateMessage != tc.config.UpdateMessage {
					t.Errorf("response update guidance = %q/%q, want %q/%q", got.UpdateUrl, got.UpdateMessage, tc.config.UpdateUrl, tc.config.UpdateMessage)
				}
			}
		})
	}
}

func TestMobileVersionPolicyHandler(t *testing.T) {
	db := vbolt.Open(t.TempDir() + "/mobile-version.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { db.Close() })

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		config := MobileVersionConfig{
			Id:             platformId("ios"),
			Platform:       "ios",
			MinimumVersion: "2.0.0",
			LatestVersion:  "2.1.0",
			UpdateUrl:      "https://apps.apple.com/app/id123456789",
		}
		vbolt.Write(tx, MobileVersionBkt, config.Id, &config)
		vbolt.TxCommit(tx)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/mobile-version?platform=ios&appVersion=1.9.0", nil)
	recorder := httptest.NewRecorder()
	mobileVersionPolicyHandler(db).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=300" {
		t.Errorf("Cache-Control = %q, want public cache policy", got)
	}
	var response CheckMobileVersionResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "update_required" || response.MinimumVersion != "2.0.0" {
		t.Errorf("response = %+v, want mandatory update to 2.0.0", response)
	}
}

func TestMobileVersionPolicyHandlerRejectsInvalidRequest(t *testing.T) {
	db := vbolt.Open(t.TempDir() + "/mobile-version.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { db.Close() })

	for _, target := range []string{
		"/api/mobile-version?platform=web&appVersion=1.0.0",
		"/api/mobile-version?platform=ios&appVersion=1.0",
	} {
		recorder := httptest.NewRecorder()
		mobileVersionPolicyHandler(db).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("%s status = %d, want %d", target, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestValidateStoreUrl(t *testing.T) {
	testCases := []struct {
		name     string
		platform string
		url      string
		wantErr  bool
	}{
		{name: "App Store listing", platform: "ios", url: "https://apps.apple.com/us/app/family-portal/id123456789"},
		{name: "legacy iTunes host", platform: "ios", url: "https://itunes.apple.com/app/id123456789"},
		{name: "TestFlight invite", platform: "ios", url: "https://testflight.apple.com/join/abcdef"},
		{name: "Play Store listing", platform: "android", url: "https://play.google.com/store/apps/details?id=app.familyrecord"},
		{name: "host casing ignored", platform: "ios", url: "https://APPS.Apple.COM/app/id123456789"},
		{name: "plain http rejected", platform: "ios", url: "http://apps.apple.com/app/id123456789", wantErr: true},
		{name: "arbitrary host rejected", platform: "ios", url: "https://example.com/app", wantErr: true},
		{name: "lookalike host rejected", platform: "ios", url: "https://apps.apple.com.evil.test/app", wantErr: true},
		{name: "credentials rejected", platform: "ios", url: "https://apps.apple.com@evil.test/app", wantErr: true},
		{name: "javascript scheme rejected", platform: "ios", url: "javascript:alert(1)", wantErr: true},
		{name: "wrong platform's store rejected", platform: "ios", url: "https://play.google.com/store/apps/details?id=app.familyrecord", wantErr: true},
		{name: "unknown platform rejected", platform: "web", url: "https://apps.apple.com/app/id123456789", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStoreUrl(tc.platform, tc.url)
			if tc.wantErr && err == nil {
				t.Errorf("validateStoreUrl(%q, %q) = nil, want an error", tc.platform, tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validateStoreUrl(%q, %q) = %v, want nil", tc.platform, tc.url, err)
			}
		})
	}
}

func TestValidateUpdateMessage(t *testing.T) {
	testCases := []struct {
		name    string
		message string
		wantErr bool
	}{
		{name: "empty is allowed", message: ""},
		{name: "one line of prose", message: "This version is no longer supported. Please update."},
		{name: "accented text", message: "Une nouvelle version est disponible."},
		{name: "newline rejected", message: "Update now.\nOr else.", wantErr: true},
		{name: "carriage return rejected", message: "Update now.\rOr else.", wantErr: true},
		{name: "delete character rejected", message: "Update now.\x7f", wantErr: true},
		{name: "embedded link rejected", message: "Download from https://evil.test/app", wantErr: true},
		{name: "over length rejected", message: strings.Repeat("a", maxUpdateMessageLength+1), wantErr: true},
		{name: "at length allowed", message: strings.Repeat("a", maxUpdateMessageLength)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateUpdateMessage(tc.message)
			if tc.wantErr && err == nil {
				t.Errorf("validateUpdateMessage(%q) = nil, want an error", tc.message)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validateUpdateMessage(%q) = %v, want nil", tc.message, err)
			}
		})
	}
}

// Rows written before the guidance rules existed are still in the bucket, and
// this endpoint answers before anybody signs in — so the last check happens on
// the way out, not only on the way in.
func TestEvaluateMobileVersionWithholdsUnsafeGuidance(t *testing.T) {
	config := MobileVersionConfig{
		Id:             platformId("ios"),
		Platform:       "ios",
		MinimumVersion: "2.0.0",
		UpdateUrl:      "https://phish.test/family-portal",
		UpdateMessage:  "Sign in here:\nhttps://phish.test",
	}

	got := evaluateMobileVersion("1.0.0", config)

	if got.Status != "update_required" {
		t.Errorf("status = %q, want update_required — the policy itself still applies", got.Status)
	}
	if got.UpdateUrl != "" {
		t.Errorf("UpdateUrl = %q, want it withheld", got.UpdateUrl)
	}
	if got.UpdateMessage != "" {
		t.Errorf("UpdateMessage = %q, want it withheld", got.UpdateMessage)
	}
}

// mobileVersionFixture is an admin (user 1, which is what the admin procs check
// for) and an ordinary account in the same database.
type mobileVersionFixture struct {
	db      *vbolt.DB
	admin   User
	regular User
}

func setupMobileVersionFixture(t *testing.T) mobileVersionFixture {
	t.Helper()

	db := vbolt.Open(t.TempDir() + "/mobile-version-procs.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })
	appDb = db
	jwtKey = []byte("mobile-version-test-secret-key-at-least-32")

	fx := mobileVersionFixture{db: db}
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		fx.admin = AddUserTx(tx, CreateAccountRequest{Name: "Admin", Email: "admin@example.com"}, []byte("hash"))
		fx.regular = AddUserTx(tx, CreateAccountRequest{Name: "Parent", Email: "parent@example.com"}, []byte("hash"))
		vbolt.TxCommit(tx)
	})
	if fx.admin.Id != 1 {
		t.Fatalf("admin user id = %d, want 1 — the admin procs authorize on that id", fx.admin.Id)
	}
	return fx
}

// as runs fn in a write transaction as the given user. The write procs commit
// their own transaction, so fn calls exactly one of them.
func (fx mobileVersionFixture) as(t *testing.T, user User, fn func(ctx *vbeam.Context)) {
	t.Helper()

	token, err := generateJwtTokenString(user)
	if err != nil {
		t.Fatalf("generateJwtTokenString() error = %v", err)
	}
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		fn(&vbeam.Context{Tx: tx, Token: token})
	})
}

func (fx mobileVersionFixture) platform(t *testing.T, name string) AdminMobileVersionPlatform {
	t.Helper()

	var found AdminMobileVersionPlatform
	fx.as(t, fx.admin, func(ctx *vbeam.Context) {
		resp, err := AdminGetMobileVersions(ctx, Empty{})
		if err != nil {
			t.Fatalf("AdminGetMobileVersions() error = %v", err)
		}
		if len(resp.Platforms) != 2 {
			t.Fatalf("platforms = %d, want both ios and android so neither can be forgotten", len(resp.Platforms))
		}
		for _, entry := range resp.Platforms {
			if entry.Platform == name {
				found = entry
			}
		}
	})
	if found.Platform != name {
		t.Fatalf("platform %q missing from AdminGetMobileVersions", name)
	}
	return found
}

func TestAdminSetMobileVersionStoresPolicy(t *testing.T) {
	fx := setupMobileVersionFixture(t)

	fx.as(t, fx.admin, func(ctx *vbeam.Context) {
		_, err := AdminSetMobileVersion(ctx, AdminSetMobileVersionRequest{
			Platform:       "ios",
			MinimumVersion: " 1.2.0 ",
			LatestVersion:  "1.4.0",
			UpdateUrl:      " https://apps.apple.com/us/app/family-portal/id123456789 ",
			UpdateMessage:  "  Please update to keep using Family Portal.  ",
		})
		if err != nil {
			t.Fatalf("AdminSetMobileVersion() error = %v", err)
		}
	})

	ios := fx.platform(t, "ios")
	if !ios.Configured {
		t.Error("Configured = false after a successful save")
	}
	if ios.MinimumVersion != "1.2.0" || ios.LatestVersion != "1.4.0" {
		t.Errorf("versions = %q/%q, want them trimmed to 1.2.0/1.4.0", ios.MinimumVersion, ios.LatestVersion)
	}
	if ios.UpdateUrl != "https://apps.apple.com/us/app/family-portal/id123456789" {
		t.Errorf("UpdateUrl = %q, want it trimmed", ios.UpdateUrl)
	}
	if ios.UpdateMessage != "Please update to keep using Family Portal." {
		t.Errorf("UpdateMessage = %q, want it trimmed", ios.UpdateMessage)
	}
	if len(ios.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none for a policy that just passed validation", ios.Warnings)
	}

	android := fx.platform(t, "android")
	if android.Configured {
		t.Error("android Configured = true, want false — only ios was set")
	}
}

// Everything below is what an unauthenticated client would be handed and act on,
// so a typo or a compromised admin session must not get past the save.
func TestAdminSetMobileVersionRejectsUnsafeGuidance(t *testing.T) {
	testCases := []struct {
		name string
		req  AdminSetMobileVersionRequest
	}{
		{
			name: "URL outside the store allowlist",
			req:  AdminSetMobileVersionRequest{Platform: "ios", MinimumVersion: "1.0.0", UpdateUrl: "https://phish.test/family-portal"},
		},
		{
			name: "plain http store URL",
			req:  AdminSetMobileVersionRequest{Platform: "ios", MinimumVersion: "1.0.0", UpdateUrl: "http://apps.apple.com/app/id123456789"},
		},
		{
			name: "the other platform's store",
			req:  AdminSetMobileVersionRequest{Platform: "ios", MinimumVersion: "1.0.0", UpdateUrl: "https://play.google.com/store/apps/details?id=app.familyrecord"},
		},
		{
			name: "forced update with nowhere to go",
			req:  AdminSetMobileVersionRequest{Platform: "ios", MinimumVersion: "1.0.0"},
		},
		{
			name: "message carrying its own link",
			req: AdminSetMobileVersionRequest{
				Platform:      "ios",
				LatestVersion: "1.4.0",
				UpdateUrl:     "https://apps.apple.com/app/id123456789",
				UpdateMessage: "Update at https://phish.test",
			},
		},
		{
			name: "multi-line message",
			req: AdminSetMobileVersionRequest{
				Platform:      "ios",
				LatestVersion: "1.4.0",
				UpdateUrl:     "https://apps.apple.com/app/id123456789",
				UpdateMessage: "Update now.\nAccount suspended.",
			},
		},
		{
			name: "minimum above latest",
			req: AdminSetMobileVersionRequest{
				Platform:       "ios",
				MinimumVersion: "2.0.0",
				LatestVersion:  "1.4.0",
				UpdateUrl:      "https://apps.apple.com/app/id123456789",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fx := setupMobileVersionFixture(t)

			fx.as(t, fx.admin, func(ctx *vbeam.Context) {
				if _, err := AdminSetMobileVersion(ctx, tc.req); err == nil {
					t.Fatal("AdminSetMobileVersion() error = nil, want a validation error")
				}
			})

			if ios := fx.platform(t, "ios"); ios.Configured {
				t.Error("the rejected policy was stored anyway")
			}
		})
	}
}

// A row written before these rules existed still evaluates, but its guidance is
// withheld — the admin page is where that becomes visible.
func TestAdminGetMobileVersionsWarnsAboutStoredGuidance(t *testing.T) {
	fx := setupMobileVersionFixture(t)

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		legacy := MobileVersionConfig{
			Id:             platformId("ios"),
			Platform:       "ios",
			MinimumVersion: "1.0.0",
			UpdateUrl:      "https://example.com/download",
			UpdateMessage:  "Reactivate your account:\nhttps://phish.test",
		}
		vbolt.Write(tx, MobileVersionBkt, legacy.Id, &legacy)
		vbolt.TxCommit(tx)
	})

	ios := fx.platform(t, "ios")
	if len(ios.Warnings) != 2 {
		t.Fatalf("Warnings = %v, want one for the URL and one for the message", ios.Warnings)
	}
	if ios.UpdateUrl != "https://example.com/download" {
		t.Errorf("UpdateUrl = %q, want the stored value shown to the operator who has to fix it", ios.UpdateUrl)
	}
	if len(ios.AllowedHosts) == 0 {
		t.Error("AllowedHosts is empty, so the page cannot say what a valid URL looks like")
	}
}

// A version policy decides whether an install keeps working, so neither reading
// nor writing it is open to an ordinary account.
func TestMobileVersionAdminProcsRequireAdmin(t *testing.T) {
	fx := setupMobileVersionFixture(t)

	procs := map[string]func(*vbeam.Context) error{
		"AdminGetMobileVersions": func(ctx *vbeam.Context) error {
			_, err := AdminGetMobileVersions(ctx, Empty{})
			return err
		},
		"AdminSetMobileVersion": func(ctx *vbeam.Context) error {
			_, err := AdminSetMobileVersion(ctx, AdminSetMobileVersionRequest{
				Platform:       "ios",
				MinimumVersion: "9.0.0",
				UpdateUrl:      "https://apps.apple.com/app/id123456789",
			})
			return err
		},
	}

	for name, call := range procs {
		t.Run(name+" denies a non-admin", func(t *testing.T) {
			fx.as(t, fx.regular, func(ctx *vbeam.Context) {
				if err := call(ctx); err == nil {
					t.Fatal("error = nil, want an authorization error")
				}
			})
		})
		t.Run(name+" denies an anonymous caller", func(t *testing.T) {
			vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
				if err := call(&vbeam.Context{Tx: tx}); err == nil {
					t.Fatal("error = nil, want an authentication error")
				}
			})
		})
	}
}
