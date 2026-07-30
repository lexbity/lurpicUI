package ll023_ext_good

import (
	"codeburg.org/lexbit/lurpicui/store"
)

type Widget struct {
	Data *store.ValueStore[[]string]
}

// NewWidget has no store param — self-owned store.
func NewWidget(items []string) *Widget {
	return &Widget{Data: store.NewValueStore(items)}
}

// handleReset reassigns its OWN store (not caller-supplied) — no fire.
func (w *Widget) handleReset() {
	w.Data = store.NewValueStore([]string{})
}
