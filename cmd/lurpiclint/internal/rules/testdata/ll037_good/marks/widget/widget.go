package widget

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/store"
)

// Widget inside marks/ may use AddBinding for a mark-internal binding.
type Widget struct {
	marks.Core
	Label marks.Binding[string]
}

func New(s *store.ValueStore[string]) *Widget {
	w := &Widget{
		Facet: facet.NewFacet(),
		Label: marks.FromStore(s, 0),
	}
	w.RegisterRoles(w)
	return w
}
