package store

import "sync/atomic"

// Version is a monotonically increasing counter.
type Version uint64

// VersionSource is a per-store version counter.
type VersionSource struct {
	current atomic.Uint64
}

// Increment bumps the version and returns the new value.
func (v *VersionSource) Increment() Version {
	return Version(v.current.Add(1))
}

// Current returns the current version without incrementing.
func (v *VersionSource) Current() Version {
	return Version(v.current.Load())
}
