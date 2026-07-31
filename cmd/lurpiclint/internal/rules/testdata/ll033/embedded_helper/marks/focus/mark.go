package mark

// focusHelper carries a Focusable() that is NOT the marks.Focusable capability
// — it is an unrelated configuration helper.
type focusHelper struct{}

func (h *focusHelper) Focusable() bool {
	return false
}

// FocusMark embeds focusHelper, so Focusable() is promoted rather than
// declared directly on the mark.  LL033 must not treat the promoted method as
// the Focusable capability (false-positive guard).
type FocusMark struct {
	focusHelper
}

func NewFocusMark() *FocusMark {
	return &FocusMark{}
}
