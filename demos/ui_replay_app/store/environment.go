package store

import (
	"codeburg.org/lexbit/lurpicui/store"
)

// EnvironmentStore holds the current environment configuration.
var EnvironmentStore = store.NewValueStore[EnvironmentState](DefaultEnvironment())

// EnvironmentState represents the replay environment settings.
type EnvironmentState struct {
	Backend  string
	Platform string
	Theme    string
	Density  string
	BuildInfo BuildInfo
}

// BuildInfo contains build metadata.
type BuildInfo struct {
	Version   string
	Commit    string
	GoVersion string
}

// DefaultEnvironment returns the default environment state.
func DefaultEnvironment() EnvironmentState {
	return EnvironmentState{
		Backend:  "software",
		Platform: "linux",
		Theme:    "baseline",
		Density:  "default",
		BuildInfo: BuildInfo{
			Version:   "0.1.0",
			Commit:    "unknown",
			GoVersion: "unknown",
		},
	}
}

// DisplayString returns a human-readable environment summary.
func (e EnvironmentState) DisplayString() string {
	return e.Backend + " / " + e.Platform + " / " + e.Theme + " / " + e.Density
}
