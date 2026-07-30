package ll024_bad

import (
	"codeburg.org/lexbit/lurpicui/store"
)

type Widget struct {
	Value *store.ValueStore[int]
}

// NewWidget accepts a caller-supplied store AND manufactures one — fires LL024.
func NewWidget(v *store.ValueStore[int]) *Widget {
	return &Widget{Value: store.NewValueStore(42)}
}
