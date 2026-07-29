package ll023_good_init

import (
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/store"
)

type Widget struct {
	Label marks.Binding[string]
}

func newWidget(s *store.ValueStore[string]) *Widget {
	w := &Widget{}
	w.Label = marks.Const("default")
	w.Label = marks.FromStore(s, 0)
	return w
}
