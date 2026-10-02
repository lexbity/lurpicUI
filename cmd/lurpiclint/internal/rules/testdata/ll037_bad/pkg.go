package ll037_bad

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/store"
)

// Widget re-registers a binding field through AddBinding — the RX-1 A-6
// ritual that LL037 exists to kill (RX-2 FR-1).
type Widget struct {
	marks.Core
	Label marks.Binding[string]
}

func newWidget(s *store.ValueStore[string]) *Widget {
	w := &Widget{
		Facet: facet.NewFacet(),
		Label: marks.Const(""),
	}
	w.Label = marks.FromStore(s, 0)
	w.AddBinding(w.Label)
	w.RegisterRoles(w)
	return w
}
