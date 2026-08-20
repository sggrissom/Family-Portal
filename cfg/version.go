package cfg

// Version is the application's version, and the only place it is written down.
//
// Anything that needs to name the running version — the startup log line, the
// admin diagnostics view, a release tag — reads it from here. It is a plain
// constant rather than something derived from git, because a binary has to be
// able to say what it is without a repository next to it.
//
// This is the *server's* version. The iOS companion app's version policy lives
// in backend/mobile_version.go and is a different number about a different
// artifact; the two are deliberately unrelated.
const Version = "1.0.0"

// Commit and BuildTime are stamped in by the linker at build time; see the
// build-go target in the Makefile. They are variables rather than constants for
// exactly that reason, and they carry these values in any build that skipped
// the stamping — a `go run`, a `go test`, a developer's `go build`.
var (
	Commit    = "unknown"
	BuildTime = "unknown"
)

// BuildInfo is the full description of a running binary: what version it
// claims, which commit it came from, and when it was linked.
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
	Release   bool   `json:"release"`
}

func Build() BuildInfo {
	return BuildInfo{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
		Release:   IsRelease,
	}
}
