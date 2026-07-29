package ll023_good_init

import (
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/store"
)

type Overlay struct {
	Open marks.Binding[bool]
}

func newOverlay(s *store.ValueStore[bool]) *Overlay {
	o := &Overlay{}
	o.Open = marks.Const(false)
	o.Open = marks.FromStore(s, 0)
	return o
}
