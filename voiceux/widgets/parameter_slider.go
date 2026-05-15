package widgets

// ParameterSlider is a reusable binding helper for effect parameters.
type ParameterSlider struct {
	Label string
	Unit  string
	Min   float32
	Max   float32
	Step  float32
	Value float32
}

// Normalized returns the slider position in 0..1.
func (s ParameterSlider) Normalized() float32 {
	if s.Max <= s.Min {
		return 0
	}
	if s.Value <= s.Min {
		return 0
	}
	if s.Value >= s.Max {
		return 1
	}
	return (s.Value - s.Min) / (s.Max - s.Min)
}

// SetValue clamps and stores one slider value.
func (s *ParameterSlider) SetValue(v float32) {
	if s == nil {
		return
	}
	if v < s.Min {
		v = s.Min
	}
	if v > s.Max {
		v = s.Max
	}
	s.Value = v
}

// Snap rounds the value to the nearest step.
func (s *ParameterSlider) Snap() {
	if s == nil || s.Step <= 0 {
		return
	}
	steps := (s.Value - s.Min) / s.Step
	s.Value = s.Min + float32(int(steps+0.5))*s.Step
	if s.Value < s.Min {
		s.Value = s.Min
	}
	if s.Value > s.Max {
		s.Value = s.Max
	}
}
