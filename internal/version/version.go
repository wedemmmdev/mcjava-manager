// Package version exposes build metadata injected at link time.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

const (
	devVersion       = "dev"
	unknownCommit    = "none"
	unknownBuildDate = "unknown"
)

// These variables are overridden with -ldflags, for example:
//
//	-X github.com/wedemmm/mcjava-manager/internal/version.Version=v0.1.0
var (
	Version   = devVersion
	Commit    = unknownCommit
	BuildDate = unknownBuildDate
)

// Short returns the version suitable for the --version flag.
func Short() string {
	if Version == "" {
		return devVersion
	}

	return Version
}

// Full returns detailed build information, including VCS metadata when the
// binary was built from a Git checkout.
func Full() string {
	version := Short()
	commit := Commit
	buildDate := BuildDate

	if info, ok := debug.ReadBuildInfo(); ok {
		if version == devVersion && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}

		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if commit == unknownCommit {
					commit = setting.Value
				}
			case "vcs.time":
				if buildDate == unknownBuildDate {
					buildDate = setting.Value
				}
			}
		}
	}

	if commit == "" {
		commit = unknownCommit
	}
	if buildDate == "" {
		buildDate = unknownBuildDate
	}
	if len(commit) > 12 {
		commit = commit[:12]
	}

	return fmt.Sprintf("%s (commit %s, build %s, %s)", version, commit, buildDate, runtime.Version())
}
