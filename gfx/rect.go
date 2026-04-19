package gfx

func RectFromXYWH(x, y, w, h float32) Rect {
	return Rect{
		Min: Point{X: x, Y: y},
		Max: Point{X: x + w, Y: y + h},
	}
}

func (r Rect) Width() float32 {
	return r.Max.X - r.Min.X
}

func (r Rect) Height() float32 {
	return r.Max.Y - r.Min.Y
}

func (r Rect) IsEmpty() bool {
	return r.Width() <= 0 || r.Height() <= 0
}

func (r Rect) Contains(p Point) bool {
	if r.IsEmpty() {
		return false
	}
	return p.X >= r.Min.X && p.X <= r.Max.X && p.Y >= r.Min.Y && p.Y <= r.Max.Y
}

func (r Rect) Intersects(other Rect) bool {
	if r.IsEmpty() || other.IsEmpty() {
		return false
	}
	return !(other.Max.X < r.Min.X || other.Min.X > r.Max.X || other.Max.Y < r.Min.Y || other.Min.Y > r.Max.Y)
}

func (r Rect) Union(other Rect) Rect {
	if r.IsEmpty() {
		return other
	}
	if other.IsEmpty() {
		return r
	}
	if other.Min.X < r.Min.X {
		r.Min.X = other.Min.X
	}
	if other.Min.Y < r.Min.Y {
		r.Min.Y = other.Min.Y
	}
	if other.Max.X > r.Max.X {
		r.Max.X = other.Max.X
	}
	if other.Max.Y > r.Max.Y {
		r.Max.Y = other.Max.Y
	}
	return r
}

func (r Rect) Inset(dx, dy float32) Rect {
	return Rect{
		Min: Point{X: r.Min.X + dx, Y: r.Min.Y + dy},
		Max: Point{X: r.Max.X - dx, Y: r.Max.Y - dy},
	}
}

func (r Rect) Offset(dx, dy float32) Rect {
	return Rect{
		Min: Point{X: r.Min.X + dx, Y: r.Min.Y + dy},
		Max: Point{X: r.Max.X + dx, Y: r.Max.Y + dy},
	}
}

