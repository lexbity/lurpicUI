package animation

// Interpolatable is implemented by any value that can be smoothly interpolated.
type Interpolatable[T any] interface {
	Lerp(other T, t float32) T
}

// Float32 is a convenience wrapper that satisfies Interpolatable.
type Float32 float32

// Lerp linearly interpolates two Float32 values.
func (f Float32) Lerp(other Float32, t float32) Float32 {
	if t <= 0 {
		return f
	}
	if t >= 1 {
		return other
	}
	return f + Float32(t)*(other-f)
}
