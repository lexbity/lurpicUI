package ll016_bad

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/store"
)

type MutatingFacet struct {
	facet.Facet
	layout  facet.LayoutRole
	counter *store.ValueStore
}

func newMutating() *MutatingFacet {
	m := &MutatingFacet{
		counter: store.NewValueStore(0),
	}

	m.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			m.counter.Set(1) // LL016: mutation in OnMeasure
			return facet.MeasureResult{Size: c.MaxSize}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			m.counter.Set(2) // LL016: mutation in OnArrange
		},
	}

	return m
}
