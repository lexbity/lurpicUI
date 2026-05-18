package linear

// NewHorizontal constructs a horizontal linear policy.
func NewHorizontal(gap float32) *Policy {
	return New(Config{Axis: Horizontal, Gap: gap})
}
