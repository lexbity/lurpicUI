package ll036_good

import "codeburg.org/lexbit/lurpicui/marks"

type gridHost struct {
	GridRows    marks.Binding[int]
	GridColumns marks.Binding[int]
}

func newGoodGrid() *gridHost {
	g := &gridHost{}
	// Small counts are fine: content-sized intrinsic tracks by default.
	g.GridRows = marks.Const(3)
	g.GridColumns = marks.Const(3)
	return g
}