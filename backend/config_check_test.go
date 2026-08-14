package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// configEnvVars is every variable the checks read. Tests set all of them so a
// developer's own environment cannot change the outcome.
var configEnvVars = []string{
	"SITE_ROOT",
	"GOOGLE_CLIENT_ID",
	"GOOGLE_CLIENT_SECRET",
	"MAIL_FROM",
	"EMAIL",
	"APP_PASSWORD",
	"SMTP_HOST",
	"SMTP_PORT",
	"GEMINI_API_KEY",
	"APNS_TEAM_ID",
	"APNS_KEY_ID",
	"APNS_BUNDLE_ID",
	"APNS_KEY_PATH",
	"APNS_ENVIRONMENT",
}

// validConfigEnv is a configuration with no issues: every required setting
// present and the optional APNs group left entirely unset.
func validConfigEnv() map[string]string {
	env := map[string]string{}
	for _, name := range configEnvVars {
		env[name] = ""
	}
	env["SITE_ROOT"] = "https://familyrecord.app"
	env["GOOGLE_CLIENT_ID"] = "client-id"
	env["GOOGLE_CLIENT_SECRET"] = "client-secret"
	env["MAIL_FROM"] = "noreply@familyrecord.app"
	env["GEMINI_API_KEY"] = "gemini-key"
	return env
}

// applyEnv sets the given environment for the duration of the test.
func applyEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, name := range configEnvVars {
		t.Setenv(name, env[name])
	}
}

// storageDirs returns a scratch database path and static directory that both
// pass the writability check.
func storageDirs(t *testing.T) (dbPath, staticDir string) {
	t.Helper()
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	staticDir = filepath.Join(root, "static")
	for _, dir := range []string{dataDir, staticDir} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("Mkdir(%s) error = %v", dir, err)
		}
	}
	return filepath.Join(dataDir, "db.bolt"), staticDir
}

// settingsWithIssues reports the Setting field of every issue found.
func settingsWithIssues(issues []ConfigIssue) string {
	settings := make([]string, 0, len(issues))
	for _, issue := range issues {
		settings = append(settings, issue.Setting)
	}
	return strings.Join(settings, ", ")
}

func hasIssue(issues []ConfigIssue, setting string) bool {
	for _, issue := range issues {
		if strings.Contains(issue.Setting, setting) {
			return true
		}
	}
	return false
}

func TestCheckProductionConfigAcceptsCompleteConfig(t *testing.T) {
	applyEnv(t, validConfigEnv())
	dbPath, staticDir := storageDirs(t)

	if issues := CheckProductionConfig(dbPath, staticDir); len(issues) > 0 {
		t.Fatalf("CheckProductionConfig() reported %d issues (%s), want none", len(issues), settingsWithIssues(issues))
	}
}

func TestCheckProductionConfigRequiresEverySetting(t *testing.T) {
	required := []string{
		"SITE_ROOT",
		"GOOGLE_CLIENT_ID",
		"GOOGLE_CLIENT_SECRET",
		"MAIL_FROM",
		"GEMINI_API_KEY",
	}

	for _, name := range required {
		t.Run("missing "+name, func(t *testing.T) {
			env := validConfigEnv()
			env[name] = ""
			applyEnv(t, env)
			dbPath, staticDir := storageDirs(t)

			issues := CheckProductionConfig(dbPath, staticDir)
			if !hasIssue(issues, name) {
				t.Fatalf("CheckProductionConfig() with %s unset reported %q, want an issue for %s", name, settingsWithIssues(issues), name)
			}
		})
	}
}

func TestCheckProductionConfigReportsEveryProblemAtOnce(t *testing.T) {
	env := validConfigEnv()
	env["SITE_ROOT"] = ""
	env["GOOGLE_CLIENT_ID"] = ""
	env["GEMINI_API_KEY"] = ""
	applyEnv(t, env)
	dbPath, staticDir := storageDirs(t)

	issues := CheckProductionConfig(dbPath, staticDir)
	if len(issues) != 3 {
		t.Fatalf("CheckProductionConfig() reported %d issues (%s), want 3", len(issues), settingsWithIssues(issues))
	}
}

func TestCheckProductionConfigValidatesSiteRoot(t *testing.T) {
	tests := []struct {
		name      string
		siteRoot  string
		wantIssue bool
	}{
		{name: "https origin", siteRoot: "https://familyrecord.app", wantIssue: false},
		{name: "trailing slash", siteRoot: "https://familyrecord.app/", wantIssue: true},
		{name: "with path", siteRoot: "https://familyrecord.app/app", wantIssue: true},
		{name: "plain http", siteRoot: "http://familyrecord.app", wantIssue: true},
		{name: "host only", siteRoot: "familyrecord.app", wantIssue: true},
		{name: "not a url", siteRoot: "https://exa mple.com", wantIssue: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := validConfigEnv()
			env["SITE_ROOT"] = tt.siteRoot
			applyEnv(t, env)
			dbPath, staticDir := storageDirs(t)

			issues := CheckProductionConfig(dbPath, staticDir)
			if got := hasIssue(issues, "SITE_ROOT"); got != tt.wantIssue {
				t.Fatalf("CheckProductionConfig() SITE_ROOT=%q issue = %v, want %v", tt.siteRoot, got, tt.wantIssue)
			}
		})
	}
}

func TestCheckProductionConfigAcceptsSMTPCredentialsInsteadOfMailFrom(t *testing.T) {
	env := validConfigEnv()
	env["MAIL_FROM"] = ""
	env["EMAIL"] = "portal@example.com"
	env["APP_PASSWORD"] = "app-password"
	applyEnv(t, env)
	dbPath, staticDir := storageDirs(t)

	issues := CheckProductionConfig(dbPath, staticDir)
	if hasIssue(issues, "MAIL_FROM") {
		t.Fatalf("CheckProductionConfig() rejected EMAIL/APP_PASSWORD delivery: %s", settingsWithIssues(issues))
	}
}

func TestCheckProductionConfigRejectsUnparseableMailFrom(t *testing.T) {
	env := validConfigEnv()
	env["MAIL_FROM"] = "not an address"
	applyEnv(t, env)
	dbPath, staticDir := storageDirs(t)

	if issues := CheckProductionConfig(dbPath, staticDir); !hasIssue(issues, "MAIL_FROM") {
		t.Fatalf("CheckProductionConfig() accepted an invalid MAIL_FROM: %s", settingsWithIssues(issues))
	}
}

func TestCheckProductionConfigTreatsAPNsAsAllOrNothing(t *testing.T) {
	t.Run("unset is fine", func(t *testing.T) {
		applyEnv(t, validConfigEnv())
		dbPath, staticDir := storageDirs(t)

		if issues := CheckProductionConfig(dbPath, staticDir); len(issues) > 0 {
			t.Fatalf("CheckProductionConfig() rejected an unconfigured APNs: %s", settingsWithIssues(issues))
		}
	})

	t.Run("partially set is not", func(t *testing.T) {
		env := validConfigEnv()
		env["APNS_TEAM_ID"] = "TEAMID1234"
		env["APNS_BUNDLE_ID"] = "app.familyrecord.ios"
		applyEnv(t, env)
		dbPath, staticDir := storageDirs(t)

		issues := CheckProductionConfig(dbPath, staticDir)
		if !hasIssue(issues, "APNS_KEY_ID") || !hasIssue(issues, "APNS_KEY_PATH") || !hasIssue(issues, "APNS_ENVIRONMENT") {
			t.Fatalf("CheckProductionConfig() did not report the missing APNs settings: %s", settingsWithIssues(issues))
		}
	})

	t.Run("invalid environment", func(t *testing.T) {
		env := validConfigEnv()
		env["APNS_TEAM_ID"] = "TEAMID1234"
		env["APNS_KEY_ID"] = "KEYID12345"
		env["APNS_BUNDLE_ID"] = "app.familyrecord.ios"
		env["APNS_KEY_PATH"] = filepath.Join(t.TempDir(), "AuthKey.p8")
		env["APNS_ENVIRONMENT"] = "staging"
		applyEnv(t, env)
		dbPath, staticDir := storageDirs(t)

		if issues := CheckProductionConfig(dbPath, staticDir); !hasIssue(issues, "APNS_ENVIRONMENT") {
			t.Fatalf("CheckProductionConfig() accepted APNS_ENVIRONMENT=staging: %s", settingsWithIssues(issues))
		}
	})

	t.Run("unreadable signing key", func(t *testing.T) {
		env := validConfigEnv()
		env["APNS_TEAM_ID"] = "TEAMID1234"
		env["APNS_KEY_ID"] = "KEYID12345"
		env["APNS_BUNDLE_ID"] = "app.familyrecord.ios"
		env["APNS_KEY_PATH"] = filepath.Join(t.TempDir(), "missing.p8")
		env["APNS_ENVIRONMENT"] = "production"
		applyEnv(t, env)
		dbPath, staticDir := storageDirs(t)

		if issues := CheckProductionConfig(dbPath, staticDir); !hasIssue(issues, "APNS_KEY_PATH") {
			t.Fatalf("CheckProductionConfig() accepted a missing APNs key file: %s", settingsWithIssues(issues))
		}
	})
}

func TestCheckProductionConfigValidatesStoragePaths(t *testing.T) {
	t.Run("missing static directory", func(t *testing.T) {
		applyEnv(t, validConfigEnv())
		dbPath, _ := storageDirs(t)

		issues := CheckProductionConfig(dbPath, filepath.Join(t.TempDir(), "absent"))
		if !hasIssue(issues, "StaticDir") {
			t.Fatalf("CheckProductionConfig() accepted a missing static directory: %s", settingsWithIssues(issues))
		}
	})

	t.Run("missing database directory", func(t *testing.T) {
		applyEnv(t, validConfigEnv())
		_, staticDir := storageDirs(t)

		issues := CheckProductionConfig(filepath.Join(t.TempDir(), "absent", "db.bolt"), staticDir)
		if !hasIssue(issues, "DBPath") {
			t.Fatalf("CheckProductionConfig() accepted a missing database directory: %s", settingsWithIssues(issues))
		}
	})

	t.Run("static path is a file", func(t *testing.T) {
		applyEnv(t, validConfigEnv())
		dbPath, _ := storageDirs(t)
		notADir := filepath.Join(t.TempDir(), "static")
		if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		issues := CheckProductionConfig(dbPath, notADir)
		if !hasIssue(issues, "StaticDir") {
			t.Fatalf("CheckProductionConfig() accepted a file as the static directory: %s", settingsWithIssues(issues))
		}
	})

	t.Run("unwritable static directory", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root bypasses directory permissions")
		}
		applyEnv(t, validConfigEnv())
		dbPath, _ := storageDirs(t)
		readOnly := filepath.Join(t.TempDir(), "static")
		if err := os.Mkdir(readOnly, 0o555); err != nil {
			t.Fatalf("Mkdir() error = %v", err)
		}

		issues := CheckProductionConfig(dbPath, readOnly)
		if !hasIssue(issues, "StaticDir") {
			t.Fatalf("CheckProductionConfig() accepted an unwritable static directory: %s", settingsWithIssues(issues))
		}
	})
}

func TestCheckProductionConfigLeavesNoProbeFilesBehind(t *testing.T) {
	applyEnv(t, validConfigEnv())
	dbPath, staticDir := storageDirs(t)

	CheckProductionConfig(dbPath, staticDir)

	entries, err := os.ReadDir(staticDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("CheckProductionConfig() left %d files in the static directory", len(entries))
	}
}
