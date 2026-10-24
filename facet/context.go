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
}
