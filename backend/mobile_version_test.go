package backend

import "testing"

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
		UpdateUrl:      "https://example.com/app",
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
