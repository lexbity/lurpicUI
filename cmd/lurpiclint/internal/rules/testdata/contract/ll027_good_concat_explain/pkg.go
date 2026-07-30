package ll027_good_concat

import (
	"codeburg.org/lexbit/lurpicui/signal"
)

type Action struct {
	Key string
}

type Widget struct {
	Activated signal.Signal[Action]
}

func (w *Widget) handleConcat(label string) {
	w.Activated.Emit(Action{Key: "prefix_" + label}) //lurpiclint:ignore LL027 -- legitimate string key construction
}
