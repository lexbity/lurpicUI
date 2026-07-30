package ll024_good_set

import (
	"codeburg.org/lexbit/lurpicui/store"
)

type Widget struct {
	Value *store.ValueStore[int]
}

// NewWidget takes a caller store and uses .Set() — no fire.
func NewWidget(v *store.ValueStore[int]) *Widget {
	w := &Widget{Value: v}
	w.Value.Set(42)
	return w
}
