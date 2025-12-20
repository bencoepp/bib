// Package version provides build-time version information for bib.
package version

import (
	"fmt"
	"runtime"
	"time"
)

// Build-time variables. These are set via ldflags during build:
//
//	go build -ldflags "-X bib/internal/version.Version=1.0.0 \
//	                   -X bib/internal/version.Commit=abc123 \
//	                   -X bib/internal/version.BuildTime=2024-01-01T00:00:00Z"
var (
	// Version is the semantic version of the application.
	Version = "0.1.0-dev"

	// Commit is the git commit hash.
	Commit = "unknown"

	// BuildTime is the RFC3339 timestamp of when the binary was built.
	BuildTime = ""

	// DevMode indicates if this is a development build.
	// Set to "false" via ldflags for release builds:
	//   -X bib/internal/version.DevMode=false
	// Development builds (default) have this set to "true".
	DevMode = "true"
)

// Info contains version information.
type Info struct {
	Version   string    `json:"version"`
	Commit    string    `json:"commit"`
	BuildTime time.Time `json:"build_time"`
	GoVersion string    `json:"go_version"`
	OS        string    `json:"os"`
	Arch      string    `json:"arch"`
	DevMode   bool      `json:"dev_mode"`
}

// IsDev returns true if this is a development build.
// Development builds allow features like gRPC reflection that are
// disabled in release builds for security reasons.
func IsDev() bool {
	return DevMode == "true"
}

// Get returns the version information.
func Get() Info {
	var buildTime time.Time
	if BuildTime != "" {
		if t, err := time.Parse(time.RFC3339, BuildTime); err == nil {
			buildTime = t
		}
	}

	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildTime: buildTime,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		DevMode:   IsDev(),
	}
}

// String returns a human-readable version string.
func (i Info) String() string {
	if i.Commit != "unknown" && len(i.Commit) > 7 {
		return fmt.Sprintf("%s (%s)", i.Version, i.Commit[:7])
	}
	return i.Version
}

// Full returns a detailed version string.
func (i Info) Full() string {
	buildTimeStr := "unknown"
	if !i.BuildTime.IsZero() {
		buildTimeStr = i.BuildTime.Format(time.RFC3339)
	}
	return fmt.Sprintf("Version: %s\nCommit: %s\nBuild Time: %s\nGo Version: %s\nOS/Arch: %s/%s",
		i.Version, i.Commit, buildTimeStr, i.GoVersion, i.OS, i.Arch)
}
