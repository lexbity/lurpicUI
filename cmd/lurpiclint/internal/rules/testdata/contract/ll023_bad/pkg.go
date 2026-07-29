package ll023_bad

import (
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/store"
)

type Overlay struct {
	Open marks.Binding[bool]
}

func newOverlay(s *store.ValueStore[bool]) *Overlay {
	o := &Overlay{}
	o.Open = marks.FromStore(s, 0)
	return o
}

func (o *Overlay) handleToggle() {
	o.Open = marks.Const(!o.Open.Get())
}
