package linear

// NewVertical constructs a vertical linear policy.
func NewVertical(gap float32) *Policy {
	return New(Config{Axis: Vertical, Gap: gap})
}
