package model

import (
	"runtime/debug"
	"time"
)

// BuildMetadata contains build-time information.
type BuildMetadata struct {
	Version   string
	Commit    string
	BuildTime time.Time
	GoVersion string
}

// DefaultBuildMetadata returns build metadata from debug info.
func DefaultBuildMetadata() BuildMetadata {
	meta := BuildMetadata{
		Version:   "0.1.0",
		BuildTime: time.Now(),
		GoVersion: "unknown",
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		meta.GoVersion = info.GoVersion
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				meta.Commit = setting.Value
			case "vcs.time":
				if t, err := time.Parse(time.RFC3339, setting.Value); err == nil {
					meta.BuildTime = t
				}
			}
		}
	}

	return meta
}
