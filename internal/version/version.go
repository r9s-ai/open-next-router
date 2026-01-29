package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the current version of open-next-router
	// This will be set via ldflags during build:
	// go build -ldflags "-X github.com/r9s-ai/open-next-router/internal/version.Version=v1.2.3"
	Version = "dev"

	// Commit is the git commit hash
	// Set via: -X github.com/r9s-ai/open-next-router/internal/version.Commit=abc123
	Commit = "unknown"

	// BuildDate is the build date in RFC3339 format
	// Set via: -X github.com/r9s-ai/open-next-router/internal/version.BuildDate=2026-01-29T11:24:55Z
	BuildDate = "unknown"
)

// Info holds version information
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

// Get returns the version information
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a formatted version string
func (i Info) String() string {
	return fmt.Sprintf(
		"open-next-router %s\ncommit: %s\nbuilt at: %s\ngo version: %s\nplatform: %s",
		i.Version,
		i.Commit,
		i.BuildDate,
		i.GoVersion,
		i.Platform,
	)
}

// Short returns a short version string
func Short() string {
	if Commit != "unknown" && len(Commit) > 7 {
		return fmt.Sprintf("%s (%s)", Version, Commit[:7])
	}
	return Version
}
