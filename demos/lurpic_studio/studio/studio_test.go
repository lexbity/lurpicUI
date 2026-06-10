package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/text"
)

type stubRuntime struct {
	fonts *text.FontRegistry
}

func (s stubRuntime) Schedule(j job.AnyJob)                                              {}
func (s stubRuntime) CancelJob(id job.JobID)                                             {}
func (s stubRuntime) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {}
func (s stubRuntime) FontRegistry() *text.FontRegistry                                   { return s.fonts }
