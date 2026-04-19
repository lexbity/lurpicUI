package facet

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/theme"
)

// AttachContext carries the narrow set of services exposed during Attach.
type AttachContext struct {
	Runtime RuntimeServices
	Theme   theme.Context
}

// RuntimeServices is the narrow subset of runtime capabilities facets may see.
type RuntimeServices interface {
	Schedule(j job.AnyJob)
	CancelJob(id job.JobID)
	Invalidate(id FacetID, flags DirtyFlags, source string)
}

// geometryAnchor reserves the gfx dependency for later geometry-aware phases.
type geometryAnchor struct {
	Bounds    gfx.Rect
	Transform gfx.Transform
}
