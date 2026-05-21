package gfx

import "math"

type Transform struct {
	A, B, C, D, TX, TY float32
}

func Identity() Transform {
	return Transform{
		A: 1, D: 1,
	}
}

func Translation(dx, dy float32) Transform {
	return Transform{
		A: 1, D: 1,
		TX: dx,
		TY: dy,
	}
}

func Scale(sx, sy float32) Transform {
	return Transform{
		A: sx,
		D: sy,
	}
}

func Rotation(radians float32) Transform {
	s := float32(math.Sin(float64(radians)))
	c := float32(math.Cos(float64(radians)))
	return Transform{
		A: c,
		B: -s,
		C: s,
		D: c,
	}
}

func (t Transform) Multiply(other Transform) Transform {
	return Transform{
		A:  t.A*other.A + t.B*other.C,
		B:  t.A*other.B + t.B*other.D,
		C:  t.C*other.A + t.D*other.C,
		D:  t.C*other.B + t.D*other.D,
		TX: t.A*other.TX + t.B*other.TY + t.TX,
		TY: t.C*other.TX + t.D*other.TY + t.TY,
	}
}

func (t Transform) TransformPoint(p Point) Point {
	return Point{
		X: t.A*p.X + t.B*p.Y + t.TX,
		Y: t.C*p.X + t.D*p.Y + t.TY,
	}
}

func (t Transform) TransformRect(r Rect) Rect {
	if r.IsEmpty() {
		return r
	}

	pts := [4]Point{
		{X: r.Min.X, Y: r.Min.Y},
		{X: r.Max.X, Y: r.Min.Y},
		{X: r.Min.X, Y: r.Max.Y},
		{X: r.Max.X, Y: r.Max.Y},
	}

	min := t.TransformPoint(pts[0])
	max := min
	for i := 1; i < len(pts); i++ {
		p := t.TransformPoint(pts[i])
		if p.X < min.X {
			min.X = p.X
		}
		if p.Y < min.Y {
			min.Y = p.Y
		}
		if p.X > max.X {
			max.X = p.X
		}
		if p.Y > max.Y {
			max.Y = p.Y
		}
	}

	return Rect{Min: min, Max: max}
}

func (t Transform) Inverse() (Transform, bool) {
	det := t.A*t.D - t.B*t.C
	if det == 0 {
		return Transform{}, false
	}

	invDet := 1 / det
	inv := Transform{
		A:  t.D * invDet,
		B:  -t.B * invDet,
		C:  -t.C * invDet,
		D:  t.A * invDet,
		TX: (t.B*t.TY - t.D*t.TX) * invDet,
		TY: (t.C*t.TX - t.A*t.TY) * invDet,
	}
	return inv, true
}

func (t Transform) IsIdentity() bool {
	return t == Identity()
}
