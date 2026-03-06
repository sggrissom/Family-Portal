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
		{name: "missing patch treated as zero", a: "1.2", b: "1.2.1", expected: -1},
		{name: "single number version", a: "2", b: "1.9.9", expected: 1},
		{name: "invalid part treated as zero", a: "1.a.0", b: "1.0.1", expected: -1},
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
