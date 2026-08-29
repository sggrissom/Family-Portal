package cfg

const Version = "1.0.0"

var (
	Commit    = "unknown"
	BuildTime = "unknown"
)

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
