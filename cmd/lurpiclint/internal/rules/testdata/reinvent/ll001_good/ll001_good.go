package ll001_good

type GoodFacet struct {
	facet.Facet
	layout facet.LayoutRole
}

func newGood() *GoodFacet {
	f := &GoodFacet{}
	f.Facet = facet.NewFacet()
	// Zero-valued LayoutRole — no OnMeasure or OnArrange set, so LL001
	// should not fire.
	f.AddRole(&f.layout)
	return f
}

// Non-LayoutRole composite literals should not trigger LL001.
type unrelated struct {
	x int
}

func newUnrelated() unrelated {
	return unrelated{x: 1}
}
