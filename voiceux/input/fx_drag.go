package input

import "codeburg.org/lexbit/voicedsp"

// FXDragState tracks drag-reorder state for an effect chain.
type FXDragState struct {
	Effect voicedsp.EffectID
	From   int
	To     int
}

// Reorder returns a new slot order with one index moved to another slot.
func Reorder[T any](items []T, from, to int) []T {
	if from < 0 || from >= len(items) || to < 0 || to >= len(items) || from == to {
		out := append([]T(nil), items...)
		return out
	}
	out := append([]T(nil), items...)
	item := out[from]
	out = append(out[:from], out[from+1:]...)
	if to >= len(out) {
		out = append(out, item)
		return out
	}
	out = append(out[:to], append([]T{item}, out[to:]...)...)
	return out
}
