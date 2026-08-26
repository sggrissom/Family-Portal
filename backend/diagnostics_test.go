package backend

import (
	"family/cfg"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestTheVersionIsWrittenDownOnce(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(cfg.Version) {
		t.Fatalf("cfg.Version = %q, want a major.minor.patch string", cfg.Version)
	}

	literal := `"` + cfg.Version + `"`
	exempt := map[string]bool{
		"cfg/version.go":              true,
		"backend/mobile_version.go":   true,
		"backend/diagnostics_test.go": true,
	}

	root := ".."
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case "node_modules", ".git", "build", "dist", ".serve":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		relative := strings.TrimPrefix(filepath.ToSlash(path), "../")
		if exempt[relative] || strings.HasSuffix(relative, "_test.go") {
			return nil
		}

		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(source), literal) {
			t.Errorf("%s writes the version down as %s; read cfg.Version instead", relative, literal)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the source tree: %v", err)
	}
}

func TestBuildProvenanceIsStampable(t *testing.T) {
	build := cfg.Build()

	if build.Version != cfg.Version {
		t.Errorf("Build().Version = %q, want cfg.Version %q", build.Version, cfg.Version)
	}
	if build.Release != cfg.IsRelease {
		t.Errorf("Build().Release = %v, want cfg.IsRelease %v", build.Release, cfg.IsRelease)
	}
	if build.Commit == "" || build.BuildTime == "" {
		t.Error("unstamped builds should report a placeholder, not an empty string")
	}
}

func TestDiagnosticsExposeNoConfiguration(t *testing.T) {
	resp := DiagnosticsResponse{
		Version:   cfg.Version,
		Commit:    "abc1234",
		BuildTime: "2026-08-19T00:00:00Z",
		GoVersion: "go1.25.0",
	}

	forbidden := []string{cfg.DBPath, cfg.StaticDir}
	rendered := strings.Join([]string{
		resp.Version, resp.Commit, resp.BuildTime, resp.GoVersion,
	}, " ")

	for _, secret := range forbidden {
		if secret == "" {
			continue
		}
		if strings.Contains(rendered, secret) {
			t.Errorf("diagnostics expose %q", secret)
		}
	}
}
