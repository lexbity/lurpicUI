package ll016_good

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/store"
)

type ReadOnlyFacet struct {
	facet.Facet
	layout facet.LayoutRole
	state  *store.ValueStore
}

func newReadOnly() *ReadOnlyFacet {
	r := &ReadOnlyFacet{
		state: store.NewValueStore(0),
	}

	r.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			v := r.state.Get() // OK: read-only
			_ = v
			return facet.MeasureResult{Size: c.MaxSize}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			v := r.state.Version() // OK: read-only
			_ = v
		},
	}

	return r
}
