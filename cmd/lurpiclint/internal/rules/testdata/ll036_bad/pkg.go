package ll036_bad

import "codeburg.org/lexbit/lurpicui/marks"

type gridHost struct {
	GridRows    marks.Binding[int]
	GridColumns marks.Binding[int]
}

func newBadGrid() *gridHost {
	g := &gridHost{}
	// A 301-row catalog sliced into near-zero flex bands (the A-8 class):
	// counts do not size tracks.
	g.GridRows = marks.Const(301)
	g.GridColumns = marks.Const(24)
	return g
}