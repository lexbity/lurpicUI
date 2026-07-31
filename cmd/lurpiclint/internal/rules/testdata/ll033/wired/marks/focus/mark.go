package mark

type FocusMark struct{}

func (m *FocusMark) Focusable() bool {
	return true
}

func NewFocusMark() *FocusMark {
	return &FocusMark{}
}
