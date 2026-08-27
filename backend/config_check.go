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

type ConfigIssue struct {
	Setting string
	Detail  string
}

func (i ConfigIssue) Error() string {
	return i.Setting + ": " + i.Detail
}

var apnsEnvVars = []string{
	"APNS_TEAM_ID",
	"APNS_KEY_ID",
	"APNS_BUNDLE_ID",
	"APNS_KEY_PATH",
	"APNS_ENVIRONMENT",
}

func CheckProductionConfig(dbPath, staticDir, logDir string) []ConfigIssue {
	var issues []ConfigIssue
	issues = append(issues, checkSiteRoot()...)
	issues = append(issues, checkGoogleOAuth()...)
	issues = append(issues, checkMail()...)
	issues = append(issues, checkBackupToken()...)
	issues = append(issues, checkAPNs()...)
	issues = append(issues, checkIOSAppID()...)
	issues = append(issues, checkMetrics()...)
	issues = append(issues, checkStoragePaths(dbPath, staticDir, logDir)...)
	return issues
}

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

func checkGoogleOAuth() []ConfigIssue {
	var issues []ConfigIssue
	for _, name := range []string{"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET"} {
		if os.Getenv(name) == "" {
			issues = append(issues, ConfigIssue{Setting: name, Detail: "must be set; the login page offers Google sign-in"})
		}
	}
	return issues
}

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
	if _, err := loadAPNsConfig(); err != nil {
		issues = append(issues, ConfigIssue{Setting: "APNS_KEY_PATH", Detail: "signing key is unusable: " + err.Error()})
	}
	return issues
}

var metricsEnvVars = []string{
	"METRICS_URL",
	"METRICS_API_KEY",
}

func checkMetrics() []ConfigIssue {
	var configured, missing []string
	for _, name := range metricsEnvVars {
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
			Detail:  "must be set because host metrics are partially configured (" + strings.Join(configured, ", ") + " present)",
		}}
	}

	raw := os.Getenv("METRICS_URL")
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return []ConfigIssue{{Setting: "METRICS_URL", Detail: "must be a full URL, e.g. http://127.0.0.1:9110/metrics"}}
	}
	if host := parsed.Hostname(); host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return []ConfigIssue{{
			Setting: "METRICS_URL",
			Detail:  "should point at the internal port on loopback; metrics-server runs on this box and binds 127.0.0.1",
		}}
	}

	return nil
}

func checkIOSAppID() []ConfigIssue {
	appID := IOSAppID()
	if appID == "" {
		return nil
	}
	if !iosAppIDPattern.MatchString(appID) {
		return []ConfigIssue{{
			Setting: "IOS_APP_ID",
			Detail:  "must be <TeamID>.<BundleID>, e.g. ABCDE12345.app.familyrecord.ios; universal links are disabled while it is malformed",
		}}
	}
	return nil
}

func checkStoragePaths(dbPath, staticDir, logDir string) []ConfigIssue {
	var issues []ConfigIssue

	if issue := checkWritableDir("DBPath", filepath.Dir(dbPath)); issue != nil {
		issues = append(issues, *issue)
	}
	if issue := checkWritableDir("StaticDir", staticDir); issue != nil {
		issues = append(issues, *issue)
	}
	if issue := checkWritableDir("LogDir", logDir); issue != nil {
		issues = append(issues, *issue)
	}
	return issues
}

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

func EnforceProductionConfig(dbPath, staticDir, logDir string) {
	issues := CheckProductionConfig(dbPath, staticDir, logDir)
	if len(issues) == 0 {
		return
	}

	report := FormatConfigIssues(issues)
	if cfg.IsRelease {
		log.Fatalf("configuration is not usable for a release build:\n%s", report)
	}
	log.Printf("configuration would fail a release build (ignored in local builds):\n%s", report)
}

func FormatConfigIssues(issues []ConfigIssue) string {
	lines := make([]string, 0, len(issues))
	for _, issue := range issues {
		lines = append(lines, "  - "+issue.Error())
	}
	return strings.Join(lines, "\n")
}
