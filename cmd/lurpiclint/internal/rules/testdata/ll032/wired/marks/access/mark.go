package mark

type AccessMark struct{}

func (m *AccessMark) AccessibilityRole() string {
	return "group"
}

func (m *AccessMark) AccessibleName() string {
	return "name"
}

func NewAccessMark() *AccessMark {
	return &AccessMark{}
}
