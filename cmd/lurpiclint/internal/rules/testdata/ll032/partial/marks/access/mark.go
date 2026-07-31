package mark

type PartialMark struct{}

func (m *PartialMark) AccessibilityRole() string {
	return "group"
}

func NewPartialMark() *PartialMark {
	return &PartialMark{}
}
