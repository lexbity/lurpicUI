package ll011_good

import "codeburg.org/lexbit/lurpicui/job"

type GoodFacet struct {
	facet.Facet
}

func newGood() *GoodFacet {
	f := &GoodFacet{}
	f.Facet = facet.NewFacet()

	// job.Schedule is allowlisted — not a raw goroutine.
	job.Schedule(func() {
		_ = 42
	})

	return f
}
