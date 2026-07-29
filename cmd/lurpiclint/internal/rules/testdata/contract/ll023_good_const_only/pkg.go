package ll023_good_const_only

import (
	"codeburg.org/lexbit/lurpicui/marks"
)

type Widget struct {
	Label marks.Binding[string]
}

func newWidget() *Widget {
	w := &Widget{}
	w.Label = marks.Const("default")
	return w
}

func (w *Widget) handleToggle() {
	w.Label = marks.Const("toggled")
}
