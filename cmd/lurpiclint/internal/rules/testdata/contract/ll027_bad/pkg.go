package ll027_bad

import (
	f "fmt"

	"codeburg.org/lexbit/lurpicui/signal"
)

type Action struct {
	Key string
}

type Widget struct {
	Activated signal.Signal[Action]
}

// bad case 1: fmt.Sprintf inside a struct-literal argument (the zoom pattern).
func (w *Widget) handleZoom(pct int) {
	w.Activated.Emit(Action{Key: f.Sprintf("zoom:%.0f", float64(pct))})
}

// bad case 2: fmt.Errorf as an argument (formatted error → string).
func (w *Widget) handleErr(msg string) {
	w.Activated.Emit(Action{Key: f.Errorf("err: %s", msg).Error()})
}

// bad case 3: string concatenation in Emit argument.
func (w *Widget) handleConcat(label string) {
	w.Activated.Emit(Action{Key: "prefix_" + label})
}

// bad case 4: Sprintf with renamed import (proves isFmtImport handles aliases).
func (w *Widget) handleAliasedSprintf(val float64) {
	w.Activated.Emit(Action{Key: f.Sprintf("val:%.2f", val)})
}
