package ll023_ext_bad

import (
	"codeburg.org/lexbit/lurpicui/store"
)

type Widget struct {
	Value *store.ValueStore[int]
}

// NewWidget accepts a caller-supplied store — correct.
func NewWidget(v *store.ValueStore[int]) *Widget {
	return &Widget{Value: v}
}

// handleToggle reassigns the caller-supplied store field — fires LL023.
func (w *Widget) handleToggle() {
	w.Value = store.NewValueStore(99)
}
