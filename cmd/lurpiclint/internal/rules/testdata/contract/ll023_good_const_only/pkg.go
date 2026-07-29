package ll023_good_const_only

import (
	"codeburg.org/lexbit/lurpicui/marks"
)

type Overlay struct {
	Open marks.Binding[bool]
}

func newOverlay() *Overlay {
	o := &Overlay{}
	o.Open = marks.Const(false)
	return o
}

func (o *Overlay) handleToggle() {
	o.Open = marks.Const(!o.Open.Get())
}
