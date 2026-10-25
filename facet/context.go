package facet

import (
	"time"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/job"
)

// AssetServices exposes runtime asset access to facets.
type AssetServices struct {
	Manager assets.Manager
}

// StoreServices exposes runtime stores to facets.
type StoreServices struct {
	AssetRegistry *assets.AssetRegistryStore
}

// AttachContext carries the narrow set of services exposed during Attach.
type AttachContext struct {
	Runtime RuntimeServices
	Theme   any
	Assets  AssetServices
	Stores  StoreServices
}

// RuntimeServices is the narrow subset of runtime capabilities facets may see.
type RuntimeServices interface {
	Schedule(j job.AnyJob)
	CancelJob(id job.JobID)
	Invalidate(id FacetID, flags DirtyFlags, source string)
	// Now returns the engine time (QP-5: the runtime clock). Marks that time
	// behavior (a toast expiry, a cadence) read this — never wall time — so
	// Virtual-mode replay is deterministic.
	Now() time.Time
	// ControlSnapshot returns immutable runtime interaction state. It is not
	// application state and is never registered as a store export.
	ControlSnapshot() ControlSnapshot
	// WatchControl observes runtime-owned interaction changes. The callback is
	// invoked on the runtime thread with an immutable snapshot; releasing the
	// returned function removes the observation.
	WatchControl(id FacetID, callback func(ControlSnapshot)) func()
}

// ControlSnapshot is the readonly projection of runtime input and time
// ownership. Session identifiers are opaque attribution, not credentials.
type ControlSnapshot struct {
	InputOwner       ControlOwner
	InputSession     string
	InputGeneration  uint64
	HeldGestures     uint32
	TimeMode         ControlTimeMode
	TimeOwnerSession string
	TimeGeneration   uint64
}

type ControlOwner uint8

const (
	ControlOwnerHuman ControlOwner = iota
	ControlOwnerClient
)

type ControlTimeMode uint8

const (
	ControlTimeWall ControlTimeMode = iota
	ControlTimeVirtual
)

func (s ControlSnapshot) ClientOwnsInput() bool { return s.InputOwner == ControlOwnerClient }

// DefaultControlSnapshot is the state of an unattached/no-drive runtime.
func DefaultControlSnapshot() ControlSnapshot {
	return ControlSnapshot{InputOwner: ControlOwnerHuman, TimeMode: ControlTimeWall}
}

// WatchNoControl is the inert implementation used by non-runtime service
// fixtures. Production runtimes install the live watcher implementation.
func WatchNoControl(_ FacetID, callback func(ControlSnapshot)) func() {
	if callback != nil {
		callback(DefaultControlSnapshot())
	}
	return func() {}
}
