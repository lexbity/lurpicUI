package svg

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	. "codeburg.org/lexbit/lurpicui/gfx"
)

func parsePathData(data string) (Path, error) {
	var sc pathScanner
	sc.s = strings.TrimSpace(data)
	if sc.s == "" {
		return Path{}, errors.New("svg: empty path data")
	}
	var builder PathBuilder
	var current Point
	var subpathStart Point
	var currentCmd byte
	var lastCubicCtrl Point
	var lastQuadCtrl Point
	var hasCubicCtrl bool
	var hasQuadCtrl bool

	readNumber := func() (float32, bool, error) {
		sc.skipSep()
		if sc.i >= len(sc.s) {
			return 0, false, nil
		}
		start := sc.i
		if sc.s[sc.i] == '+' || sc.s[sc.i] == '-' {
			sc.i++
		}
		digits := 0
		for sc.i < len(sc.s) && isDigit(sc.s[sc.i]) {
			sc.i++
			digits++
		}
		if sc.i < len(sc.s) && sc.s[sc.i] == '.' {
			sc.i++
			for sc.i < len(sc.s) && isDigit(sc.s[sc.i]) {
				sc.i++
				digits++
			}
		}
		if digits == 0 {
			return 0, false, nil
		}
		if sc.i < len(sc.s) && (sc.s[sc.i] == 'e' || sc.s[sc.i] == 'E') {
			j := sc.i + 1
			if j < len(sc.s) && (sc.s[j] == '+' || sc.s[j] == '-') {
				j++
			}
			expDigits := 0
			for j < len(sc.s) && isDigit(sc.s[j]) {
				j++
				expDigits++
			}
			if expDigits > 0 {
				sc.i = j
			}
		}
		v, err := strconv.ParseFloat(sc.s[start:sc.i], 32)
		if err != nil {
			return 0, false, err
		}
		sc.skipSep()
		return float32(v), true, nil
	}

	readPoint := func(relative bool) (Point, bool, error) {
		x, ok, err := readNumber()
		if err != nil || !ok {
			return Point{}, false, err
		}
		y, ok, err := readNumber()
		if err != nil || !ok {
			return Point{}, false, errors.New("svg: expected y coordinate")
		}
		if relative {
			return Point{X: current.X + x, Y: current.Y + y}, true, nil
		}
		return Point{X: x, Y: y}, true, nil
	}

	for {
		sc.skipSep()
		if sc.i >= len(sc.s) {
			break
		}
		if isCommandLetter(sc.s[sc.i]) {
			currentCmd = sc.s[sc.i]
			sc.i++
		}
		if currentCmd == 0 {
			return Path{}, fmt.Errorf("svg: path data missing command near %q", sc.s[sc.i:])
		}
		switch currentCmd {
		case 'M', 'm':
			relative := currentCmd == 'm'
			p, ok, err := readPoint(relative)
			if err != nil {
				return Path{}, err
			}
			if !ok {
				return Path{}, errors.New("svg: move command requires a point")
			}
			builder.MoveTo(p)
			current = p
			subpathStart = p
			hasCubicCtrl = false
			hasQuadCtrl = false
			if currentCmd == 'M' {
				currentCmd = 'L'
			} else {
				currentCmd = 'l'
			}
			for {
				p, ok, err := readPoint(currentCmd == 'l')
				if err != nil {
					return Path{}, err
				}
				if !ok {
					break
				}
				builder.LineTo(p)
				current = p
			}
			currentCmd = 0
		case 'L', 'l':
			relative := currentCmd == 'l'
			for {
				p, ok, err := readPoint(relative)
				if err != nil {
					return Path{}, err
				}
				if !ok {
					break
				}
				builder.LineTo(p)
				current = p
			}
			currentCmd = 0
		case 'H', 'h':
			relative := currentCmd == 'h'
			for {
				x, ok, err := readNumber()
				if err != nil {
					return Path{}, err
				}
				if !ok {
					break
				}
				if relative {
					x += current.X
				}
				current = Point{X: x, Y: current.Y}
				builder.LineTo(current)
			}
			currentCmd = 0
		case 'V', 'v':
			relative := currentCmd == 'v'
			for {
				y, ok, err := readNumber()
				if err != nil {
					return Path{}, err
				}
				if !ok {
					break
				}
				if relative {
					y += current.Y
				}
				current = Point{X: current.X, Y: y}
				builder.LineTo(current)
			}
			currentCmd = 0
		case 'C', 'c':
			relative := currentCmd == 'c'
			for {
				x1, ok, err := readNumber()
				if err != nil || !ok {
					if err != nil {
						return Path{}, err
					}
					break
				}
				y1, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: cubic command requires control point")
				}
				x2, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: cubic command requires second control point")
				}
				y2, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: cubic command requires second control point")
				}
				x, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: cubic command requires destination point")
				}
				y, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: cubic command requires destination point")
				}
				if relative {
					x1 += current.X
					y1 += current.Y
					x2 += current.X
					y2 += current.Y
					x += current.X
					y += current.Y
				}
				builder.CubicTo(Point{X: x1, Y: y1}, Point{X: x2, Y: y2}, Point{X: x, Y: y})
				current = Point{X: x, Y: y}
				lastCubicCtrl = Point{X: x2, Y: y2}
				hasCubicCtrl = true
				hasQuadCtrl = false
			}
			currentCmd = 0
		case 'S', 's':
			relative := currentCmd == 's'
			for {
				x2, ok, err := readNumber()
				if err != nil || !ok {
					if err != nil {
						return Path{}, err
					}
					break
				}
				y2, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: smooth cubic command requires second control point")
				}
				x, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: smooth cubic command requires destination point")
				}
				y, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: smooth cubic command requires destination point")
				}
				c1 := current
				if hasCubicCtrl {
					c1 = Point{X: 2*current.X - lastCubicCtrl.X, Y: 2*current.Y - lastCubicCtrl.Y}
				}
				if relative {
					x2 += current.X
					y2 += current.Y
					x += current.X
					y += current.Y
				}
				builder.CubicTo(c1, Point{X: x2, Y: y2}, Point{X: x, Y: y})
				current = Point{X: x, Y: y}
				lastCubicCtrl = Point{X: x2, Y: y2}
				hasCubicCtrl = true
				hasQuadCtrl = false
			}
			currentCmd = 0
		case 'Q', 'q':
			relative := currentCmd == 'q'
			for {
				x1, ok, err := readNumber()
				if err != nil || !ok {
					if err != nil {
						return Path{}, err
					}
					break
				}
				y1, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: quadratic command requires control point")
				}
				x, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: quadratic command requires destination point")
				}
				y, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: quadratic command requires destination point")
				}
				if relative {
					x1 += current.X
					y1 += current.Y
					x += current.X
					y += current.Y
				}
				builder.QuadTo(Point{X: x1, Y: y1}, Point{X: x, Y: y})
				current = Point{X: x, Y: y}
				lastQuadCtrl = Point{X: x1, Y: y1}
				hasQuadCtrl = true
				hasCubicCtrl = false
			}
			currentCmd = 0
		case 'T', 't':
			relative := currentCmd == 't'
			for {
				x, ok, err := readNumber()
				if err != nil || !ok {
					if err != nil {
						return Path{}, err
					}
					break
				}
				y, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: smooth quadratic command requires destination point")
				}
				ctrl := current
				if hasQuadCtrl {
					ctrl = Point{X: 2*current.X - lastQuadCtrl.X, Y: 2*current.Y - lastQuadCtrl.Y}
				}
				if relative {
					x += current.X
					y += current.Y
				}
				builder.QuadTo(ctrl, Point{X: x, Y: y})
				current = Point{X: x, Y: y}
				lastQuadCtrl = ctrl
				hasQuadCtrl = true
				hasCubicCtrl = false
			}
			currentCmd = 0
		case 'A', 'a':
			relative := currentCmd == 'a'
			for {
				rx, ok, err := readNumber()
				if err != nil || !ok {
					if err != nil {
						return Path{}, err
					}
					break
				}
				ry, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: arc command requires ry")
				}
				rot, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: arc command requires x-axis rotation")
				}
				largeArc, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: arc command requires large-arc flag")
				}
				sweep, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: arc command requires sweep flag")
				}
				x, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: arc command requires destination x")
				}
				y, ok, err := readNumber()
				if err != nil || !ok {
					return Path{}, errors.New("svg: arc command requires destination y")
				}
				if relative {
					x += current.X
					y += current.Y
				}
				segs, err := arcToCubics(current, Point{X: x, Y: y}, rx, ry, rot, largeArc != 0, sweep != 0)
				if err != nil {
					return Path{}, err
				}
				for _, seg := range segs {
					switch seg.Verb {
					case PathMoveTo:
						builder.MoveTo(seg.Pts[0])
					case PathLineTo:
						builder.LineTo(seg.Pts[0])
					case PathQuadTo:
						builder.QuadTo(seg.Pts[0], seg.Pts[1])
					case PathCubicTo:
						builder.CubicTo(seg.Pts[0], seg.Pts[1], seg.Pts[2])
					case PathClose:
						builder.Close()
					}
				}
				current = Point{X: x, Y: y}
				hasCubicCtrl = false
				hasQuadCtrl = false
			}
			currentCmd = 0
		case 'Z', 'z':
			builder.Close()
			current = subpathStart
			currentCmd = 0
			hasCubicCtrl = false
			hasQuadCtrl = false
		default:
			return Path{}, fmt.Errorf("svg: unsupported path command %q", currentCmd)
		}
	}
	return builder.Build(), nil
}

type pathScanner struct {
	s string
	i int
}

func (s *pathScanner) skipSep() {
	for s.i < len(s.s) && isSeparator(s.s[s.i]) {
		s.i++
	}
}

func isCommandLetter(b byte) bool {
	switch b {
	case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v', 'C', 'c', 'S', 's', 'Q', 'q', 'T', 't', 'A', 'a', 'Z', 'z':
		return true
	default:
		return false
	}
}

func arcToCubics(from, to Point, rx, ry, rotation float32, largeArc, sweep bool) ([]PathSegment, error) {
	rx = float32(math.Abs(float64(rx)))
	ry = float32(math.Abs(float64(ry)))
	if rx == 0 || ry == 0 || (from == to) {
		return []PathSegment{{Verb: PathLineTo, Pts: [3]Point{to}}}, nil
	}

	phi := float64(rotation) * math.Pi / 180
	cosphi := math.Cos(phi)
	sinphi := math.Sin(phi)

	x1p := cosphi*float64(from.X-to.X)/2 + sinphi*float64(from.Y-to.Y)/2
	y1p := -sinphi*float64(from.X-to.X)/2 + cosphi*float64(from.Y-to.Y)/2

	rx2 := float64(rx * rx)
	ry2 := float64(ry * ry)
	x1p2 := x1p * x1p
	y1p2 := y1p * y1p

	lambda := x1p2/rx2 + y1p2/ry2
	if lambda > 1 {
		scale := math.Sqrt(lambda)
		rx *= float32(scale)
		ry *= float32(scale)
		rx2 = float64(rx * rx)
		ry2 = float64(ry * ry)
	}

	sign := -1.0
	if largeArc != sweep {
		sign = 1.0
	}
	num := rx2*ry2 - rx2*y1p2 - ry2*x1p2
	den := rx2*y1p2 + ry2*x1p2
	if den == 0 {
		return []PathSegment{{Verb: PathLineTo, Pts: [3]Point{to}}}, nil
	}
	if num < 0 {
		num = 0
	}
	coef := sign * math.Sqrt(num/den)
	cxp := coef * (float64(rx) * y1p / float64(ry))
	cyp := coef * (-float64(ry) * x1p / float64(rx))

	cx := cosphi*cxp - sinphi*cyp + float64(from.X+to.X)/2
	cy := sinphi*cxp + cosphi*cyp + float64(from.Y+to.Y)/2

	ux := (x1p - cxp) / float64(rx)
	uy := (y1p - cyp) / float64(ry)
	vx := (-x1p - cxp) / float64(rx)
	vy := (-y1p - cyp) / float64(ry)
	theta1 := angleBetween(1, 0, ux, uy)
	delta := angleBetween(ux, uy, vx, vy)
	if !sweep && delta > 0 {
		delta -= 2 * math.Pi
	}
	if sweep && delta < 0 {
		delta += 2 * math.Pi
	}

	segments := int(math.Ceil(math.Abs(delta) / (math.Pi / 2)))
	if segments < 1 {
		segments = 1
	}
	step := delta / float64(segments)
	out := make([]PathSegment, 0, segments)
	for i := 0; i < segments; i++ {
		t1 := theta1 + float64(i)*step
		t2 := t1 + step
		out = append(out, arcSegment(cosphi, sinphi, float32(rx), float32(ry), float32(cx), float32(cy), t1, t2))
	}
	return out, nil
}

func angleBetween(ux, uy, vx, vy float64) float64 {
	dot := ux*vx + uy*vy
	det := ux*vy - uy*vx
	return math.Atan2(det, dot)
}

func arcSegment(cosphi, sinphi float64, rx, ry, cx, cy float32, t1, t2 float64) PathSegment {
	delta := t2 - t1
	alpha := 4.0 / 3.0 * math.Tan(delta/4.0)
	p1 := arcPoint(cosphi, sinphi, rx, ry, cx, cy, t1)
	p2 := arcPoint(cosphi, sinphi, rx, ry, cx, cy, t2)
	t1Tan := arcTangent(cosphi, sinphi, rx, ry, t1)
	t2Tan := arcTangent(cosphi, sinphi, rx, ry, t2)
	c1 := Point{X: p1.X + float32(alpha)*t1Tan.X, Y: p1.Y + float32(alpha)*t1Tan.Y}
	c2 := Point{X: p2.X - float32(alpha)*t2Tan.X, Y: p2.Y - float32(alpha)*t2Tan.Y}
	return PathSegment{Verb: PathCubicTo, Pts: [3]Point{c1, c2, p2}}
}

func arcPoint(cosphi, sinphi float64, rx, ry, cx, cy float32, theta float64) Point {
	cosT := math.Cos(theta)
	sinT := math.Sin(theta)
	x := cosphi*float64(rx)*cosT - sinphi*float64(ry)*sinT + float64(cx)
	y := sinphi*float64(rx)*cosT + cosphi*float64(ry)*sinT + float64(cy)
	return Point{X: float32(x), Y: float32(y)}
}

func arcTangent(cosphi, sinphi float64, rx, ry float32, theta float64) Point {
	cosT := math.Cos(theta)
	sinT := math.Sin(theta)
	x := -cosphi*float64(rx)*sinT - sinphi*float64(ry)*cosT
	y := -sinphi*float64(rx)*sinT + cosphi*float64(ry)*cosT
	return Point{X: float32(x), Y: float32(y)}
}

func roundedRectPath(r Rect, rx, ry float32) Path {
	if r.IsEmpty() {
		return Path{}
	}
	k := float32(0.552284749831)
	minX, minY := r.Min.X, r.Min.Y
	maxX, maxY := r.Max.X, r.Max.Y
	if rx <= 0 {
		rx = ry
	}
	if ry <= 0 {
		ry = rx
	}
	if rx > r.Width()/2 {
		rx = r.Width() / 2
	}
	if ry > r.Height()/2 {
		ry = r.Height() / 2
	}
	return NewPath().
		MoveTo(Point{X: minX + rx, Y: minY}).
		LineTo(Point{X: maxX - rx, Y: minY}).
		CubicTo(Point{X: maxX - rx + k*rx, Y: minY}, Point{X: maxX, Y: minY + ry - k*ry}, Point{X: maxX, Y: minY + ry}).
		LineTo(Point{X: maxX, Y: maxY - ry}).
		CubicTo(Point{X: maxX, Y: maxY - ry + k*ry}, Point{X: maxX - rx + k*rx, Y: maxY}, Point{X: maxX - rx, Y: maxY}).
		LineTo(Point{X: minX + rx, Y: maxY}).
		CubicTo(Point{X: minX + rx - k*rx, Y: maxY}, Point{X: minX, Y: maxY - ry + k*ry}, Point{X: minX, Y: maxY - ry}).
		LineTo(Point{X: minX, Y: minY + ry}).
		CubicTo(Point{X: minX, Y: minY + ry - k*ry}, Point{X: minX + rx - k*rx, Y: minY}, Point{X: minX + rx, Y: minY}).
		Close().
		Build()
}

func ellipsePath(center Point, rx, ry float32) Path {
	if rx <= 0 || ry <= 0 {
		return Path{}
	}
	k := float32(0.552284749831)
	return NewPath().
		MoveTo(Point{X: center.X + rx, Y: center.Y}).
		CubicTo(Point{X: center.X + rx, Y: center.Y + k*ry}, Point{X: center.X + k*rx, Y: center.Y + ry}, Point{X: center.X, Y: center.Y + ry}).
		CubicTo(Point{X: center.X - k*rx, Y: center.Y + ry}, Point{X: center.X - rx, Y: center.Y + k*ry}, Point{X: center.X - rx, Y: center.Y}).
		CubicTo(Point{X: center.X - rx, Y: center.Y - k*ry}, Point{X: center.X - k*rx, Y: center.Y - ry}, Point{X: center.X, Y: center.Y - ry}).
		CubicTo(Point{X: center.X + k*rx, Y: center.Y - ry}, Point{X: center.X + rx, Y: center.Y - k*ry}, Point{X: center.X + rx, Y: center.Y}).
		Close().
		Build()
}
