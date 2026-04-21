package gfx

// Lerp linearly interpolates each color channel.
func (c Color) Lerp(other Color, t float32) Color {
	t = clamp01(t)
	if t == 0 {
		return c
	}
	if t == 1 {
		return other
	}
	return Color{
		R: c.R + (other.R-c.R)*t,
		G: c.G + (other.G-c.G)*t,
		B: c.B + (other.B-c.B)*t,
		A: c.A + (other.A-c.A)*t,
	}
}

// Lerp linearly interpolates the point components.
func (p Point) Lerp(other Point, t float32) Point {
	t = clamp01(t)
	if t == 0 {
		return p
	}
	if t == 1 {
		return other
	}
	return Point{
		X: p.X + (other.X-p.X)*t,
		Y: p.Y + (other.Y-p.Y)*t,
	}
}

// Lerp linearly interpolates the rectangle corners.
func (r Rect) Lerp(other Rect, t float32) Rect {
	t = clamp01(t)
	if t == 0 {
		return r
	}
	if t == 1 {
		return other
	}
	return Rect{
		Min: r.Min.Lerp(other.Min, t),
		Max: r.Max.Lerp(other.Max, t),
	}
}

// Lerp linearly interpolates the size components.
func (s Size) Lerp(other Size, t float32) Size {
	t = clamp01(t)
	if t == 0 {
		return s
	}
	if t == 1 {
		return other
	}
	return Size{
		W: s.W + (other.W-s.W)*t,
		H: s.H + (other.H-s.H)*t,
	}
}

// Lerp linearly interpolates transform components.
func (t Transform) Lerp(other Transform, alpha float32) Transform {
	alpha = clamp01(alpha)
	if alpha == 0 {
		return t
	}
	if alpha == 1 {
		return other
	}
	return Transform{
		A:  t.A + (other.A-t.A)*alpha,
		B:  t.B + (other.B-t.B)*alpha,
		C:  t.C + (other.C-t.C)*alpha,
		D:  t.D + (other.D-t.D)*alpha,
		TX: t.TX + (other.TX-t.TX)*alpha,
		TY: t.TY + (other.TY-t.TY)*alpha,
	}
}
