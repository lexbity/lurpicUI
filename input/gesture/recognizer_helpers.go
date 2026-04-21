package gesture

import (
	"math"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func radiusExceeded(start, current gfx.Point, radius float32) bool {
	if radius <= 0 {
		radius = 10
	}
	return distanceSquared(start, current) > radius*radius
}

func distanceSquared(a, b gfx.Point) float32 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return dx*dx + dy*dy
}

func distance(a, b gfx.Point) float32 {
	return float32(math.Sqrt(float64(distanceSquared(a, b))))
}

func midpoint(a, b gfx.Point) gfx.Point {
	return gfx.Point{X: (a.X + b.X) * 0.5, Y: (a.Y + b.Y) * 0.5}
}

func magnitude(p gfx.Point) float32 {
	return float32(math.Sqrt(float64(p.X*p.X + p.Y*p.Y)))
}

func emaAlpha(dt, window time.Duration) float32 {
	if window <= 0 || dt <= 0 {
		return 1
	}
	ratio := float64(dt) / float64(window)
	return float32(1 - math.Exp(-ratio))
}

func swipeDirection(delta gfx.Point) (SwipeDirection, bool) {
	ax := float32(math.Abs(float64(delta.X)))
	ay := float32(math.Abs(float64(delta.Y)))
	if ax == 0 && ay == 0 {
		return 0, false
	}
	if ax >= ay {
		if ay > ax*0.5 {
			return 0, false
		}
		if delta.X < 0 {
			return SwipeLeft, true
		}
		return SwipeRight, true
	}
	if ax > ay*0.5 {
		return 0, false
	}
	if delta.Y < 0 {
		return SwipeUp, true
	}
	return SwipeDown, true
}

func max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
