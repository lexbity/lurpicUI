package ll027_good_typed

import (
	"codeburg.org/lexbit/lurpicui/signal"
)

type MyAction struct {
	Key         string
	ZoomPercent int
}

type Widget struct {
	Activated signal.Signal[MyAction]
}

func (w *Widget) handleZoom(pct int) {
	w.Activated.Emit(MyAction{Key: "zoom", ZoomPercent: pct})
}

func (w *Widget) handleToggle() {
	w.Activated.Emit(MyAction{Key: "toggle"})
}
