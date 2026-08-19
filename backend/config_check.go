package backend

import (
	"family/cfg"
	"fmt"
	"log"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Configuration that is only checked at the moment it is used fails in a user's
// face, hours after a deploy, in whichever corner of the app happened to need
// it: an unset SITE_ROOT points Google's OAuth redirect at localhost, a
// half-configured APNs keypair rejects every device registration, and a static
// directory the service user cannot write turns every upload into a 500. The
// checks below run once at startup so a release build refuses to serve rather
// than serving something broken.
//
// Two policies are in play. Settings the site cannot work without are required
// outright in release builds. Settings that belong to an optional subsystem —
// APNs today — are all-or-nothing: configuring none of them is fine, but
// configuring some of them is a mistake worth stopping for, because a partially
// configured subsystem looks enabled and behaves like it is broken.

// ConfigIssue is one problem found in the startup configuration.
type ConfigIssue struct {
	// Setting names the environment variable or path at fault.
	Setting string
	// Detail says what is wrong with it, without echoing secret values.
	Detail string
}

func (i ConfigIssue) Error() string {
	return i.Setting + ": " + i.Detail
}

// apnsEnvVars are the variables that must be set together for push to work.
// APNS_ENVIRONMENT is included because push registration compares against it
// (see validateRegisterPushDeviceRequest); without it every device is refused.
var apnsEnvVars = []string{
	"APNS_TEAM_ID",
	"APNS_KEY_ID",
	"APNS_BUNDLE_ID",
	"APNS_KEY_PATH",
	"APNS_ENVIRONMENT",
}

// CheckProductionConfig reports everything about the current environment that
// would make a release build unfit to serve. dbPath and staticDir are passed in
// rather than read from cfg so tests can point them at a scratch directory.
//
// It never returns a partial answer: callers get the full list so one restart
// surfaces every problem instead of one per deploy.
func CheckProductionConfig(dbPath, staticDir string) []ConfigIssue {
	var issues []ConfigIssue
	issues = append(issues, checkSiteRoot()...)
	issues = append(issues, checkGoogleOAuth()...)
	issues = append(issues, checkMail()...)
	issues = append(issues, checkAIProvider()...)
	issues = append(issues, checkBackupToken()...)
	issues = append(issues, checkAPNs()...)
	issues = append(issues, checkStoragePaths(dbPath, staticDir)...)
	return issues
}

// checkSiteRoot validates the public origin. Several call sites fall back to
// cfg.SiteURL or plain localhost when it is unset, and the OAuth redirect is the
// one that matters: a release build without SITE_ROOT sends users returning from
// Google to http://localhost:8666.
func checkSiteRoot() []ConfigIssue {
	raw := os.Getenv("SITE_ROOT")
	if raw == "" {
		return []ConfigIssue{{Setting: "SITE_ROOT", Detail: "must be set to the public origin, e.g. " + cfg.SiteURL}}
	}

	if strings.HasSuffix(raw, "/") {
		return []ConfigIssue{{Setting: "SITE_ROOT", Detail: "must not end in a slash; URLs are built by appending paths to it"}}
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return []ConfigIssue{{Setting: "SITE_ROOT", Detail: "is not a valid URL"}}
	}
	if parsed.Host == "" {
		return []ConfigIssue{{Setting: "SITE_ROOT", Detail: "must include a scheme and host, e.g. " + cfg.SiteURL}}
	}
	if parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return []ConfigIssue{{Setting: "SITE_ROOT", Detail: "must be an origin only, with no path, query, or fragment"}}
	}
	if parsed.Scheme != "https" {
		return []ConfigIssue{{Setting: "SITE_ROOT", Detail: "must use https so cookies and OAuth redirects are not sent in the clear"}}
	}

	return nil
}

// checkGoogleOAuth requires both halves of the web credential. The login page
// offers "Sign in with Google" unconditionally, so a missing credential is a
// dead button rather than a hidden feature. GOOGLE_IOS_CLIENT_ID stays optional
// — it belongs to the companion app, which is not part of 1.0.
func checkGoogleOAuth() []ConfigIssue {
	var issues []ConfigIssue
	for _, name := range []string{"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET"} {
		if os.Getenv(name) == "" {
			issues = append(issues, ConfigIssue{Setting: name, Detail: "must be set; the login page offers Google sign-in"})
		}
	}
	return issues
}

// checkMail requires a usable outbound path. Password reset is the only way a
// locked-out user gets back in without me, and the backup staleness alert is the
// only way I learn backups stopped, so unconfigured mail is not a degraded
// feature — it is two of this release's guarantees quietly gone.
func checkMail() []ConfigIssue {
	settings, err := resolveMailSettings()
	if err != nil {
		return []ConfigIssue{{
			Setting: "MAIL_FROM",
			Detail:  "must be set (or EMAIL and APP_PASSWORD together); password reset and backup alerts need outbound mail",
		}}
	}
	if _, err := mail.ParseAddress(settings.From); err != nil {
		return []ConfigIssue{{Setting: "MAIL_FROM", Detail: "is not a valid email address"}}
	}
	return nil
}

// checkAIProvider requires the Gemini key. The import page exposes AI-assisted
// import to every user in 1.0, so an unset key is a visible feature that errors
// on use. AI_MODEL is optional and falls back to GetDefaultAIModel.
func checkAIProvider() []ConfigIssue {
	if os.Getenv("GEMINI_API_KEY") == "" {
		return []ConfigIssue{{Setting: "GEMINI_API_KEY", Detail: "must be set; AI-assisted import is offered in the UI"}}
	}
	return nil
}

// checkBackupToken requires the snapshot credential. The endpoint the nightly
// backup pulls from authorizes nobody until this is set, and RegisterBackupHandlers
// stops a release build that lacks it — but it stops it after this report has
// already been printed, which is how an operator ends up fixing five settings,
// restarting, and meeting a sixth. Checking it here puts it in the same list.
func checkBackupToken() []ConfigIssue {
	token := os.Getenv("BACKUP_TOKEN")
	if token == "" {
		return []ConfigIssue{{
			Setting: "BACKUP_TOKEN",
			Detail:  "must be set; the snapshot endpoint the nightly backup reads authorizes nobody without it",
		}}
	}
	if len(token) < minimumBackupTokenLength {
		return []ConfigIssue{{
			Setting: "BACKUP_TOKEN",
			Detail:  fmt.Sprintf("must be at least %d characters long", minimumBackupTokenLength),
		}}
	}
	return nil
}

// checkAPNs is the all-or-nothing case. Push belongs to the companion app, so an
// entirely unconfigured APNs is expected in 1.0; a partially configured one is
// not, and it fails in the least visible place possible — a device registration
// the user never sees succeed or fail.
func checkAPNs() []ConfigIssue {
	var configured, missing []string
	for _, name := range apnsEnvVars {
		if os.Getenv(name) == "" {
			missing = append(missing, name)
		} else {
			configured = append(configured, name)
		}
	}

	if len(configured) == 0 {
		return nil
	}

	if len(missing) > 0 {
		return []ConfigIssue{{
			Setting: strings.Join(missing, ", "),
			Detail:  "must be set because APNs is partially configured (" + strings.Join(configured, ", ") + " present)",
		}}
	}

	var issues []ConfigIssue
	if env := os.Getenv("APNS_ENVIRONMENT"); env != "sandbox" && env != "production" {
		issues = append(issues, ConfigIssue{Setting: "APNS_ENVIRONMENT", Detail: `must be "sandbox" or "production"`})
	}
	// loadAPNsConfig reads and parses the signing key, which is the half of this
	// configuration that a typo in a path or a wrong key format breaks.
	if _, err := loadAPNsConfig(); err != nil {
		issues = append(issues, ConfigIssue{Setting: "APNS_KEY_PATH", Detail: "signing key is unusable: " + err.Error()})
	}
	return issues
}

// checkStoragePaths confirms the process can actually write where it stores
// things. The paths are compile-time constants, so what is being checked is the
// deployed filesystem: directory present, owned by a user that can write it.
func checkStoragePaths(dbPath, staticDir string) []ConfigIssue {
	var issues []ConfigIssue

	// bolt creates the database file but not the directory holding it.
	if issue := checkWritableDir("DBPath", filepath.Dir(dbPath)); issue != nil {
		issues = append(issues, *issue)
	}
	if issue := checkWritableDir("StaticDir", staticDir); issue != nil {
		issues = append(issues, *issue)
	}
	return issues
}

// checkWritableDir verifies a directory exists and accepts a new file. Checking
// the mode bits would answer a different question than the one that matters,
// which is whether this process, as the user it runs as, can create a file.
func checkWritableDir(setting, dir string) *ConfigIssue {
	if dir == "" {
		return &ConfigIssue{Setting: setting, Detail: "is empty"}
	}

	clean := filepath.Clean(dir)
	info, err := os.Stat(clean)
	if err != nil {
		return &ConfigIssue{Setting: setting, Detail: fmt.Sprintf("%s is not accessible: %v", clean, err)}
	}
	if !info.IsDir() {
		return &ConfigIssue{Setting: setting, Detail: clean + " is not a directory"}
	}

	probe, err := os.CreateTemp(clean, ".configcheck-*")
	if err != nil {
		return &ConfigIssue{Setting: setting, Detail: fmt.Sprintf("%s is not writable: %v", clean, err)}
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return nil
}

// EnforceProductionConfig checks the startup configuration and decides what to
// do about it. Release builds stop — a process that keeps running past this
// point serves the broken behavior instead of reporting it. Local builds log the
// same list and continue, because a development machine legitimately has no APNs
// key or Gemini quota.
func EnforceProductionConfig(dbPath, staticDir string) {
	issues := CheckProductionConfig(dbPath, staticDir)
	if len(issues) == 0 {
		return
	}

	report := FormatConfigIssues(issues)
	if cfg.IsRelease {
		log.Fatalf("configuration is not usable for a release build:\n%s", report)
	}
	log.Printf("configuration would fail a release build (ignored in local builds):\n%s", report)
}

// FormatConfigIssues renders issues one per line for a startup log.
func FormatConfigIssues(issues []ConfigIssue) string {
	lines := make([]string, 0, len(issues))
	for _, issue := range issues {
		lines = append(lines, "  - "+issue.Error())
	}
	return strings.Join(lines, "\n")
}
