package mark

//nolint:LL033 // deliberate opt-out
type FocusMark struct{}

func (m *FocusMark) Focusable() bool {
	return true
}

func NewFocusMark() *FocusMark {
	return &FocusMark{}
}
