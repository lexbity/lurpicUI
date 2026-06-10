package theme

import (
	"math"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// MaterialCommands converts a theme material into drawable commands for a path.
func MaterialCommands(path gfx.Path, material Material) []gfx.Command {
	if Transparent(material) {
		return nil
	}

	cmds := make([]gfx.Command, 0, len(material.Fills)+len(material.Strokes))
	materialOpacity := clamp01(material.Opacity)
	for _, fill := range material.Fills {
		switch fill.Type {
		case FillSolid:
			if fill.Color.A <= 0 || fill.Opacity <= 0 {
				continue
			}
			cmds = append(cmds, gfx.FillPath{
				Path:  path,
				Brush: gfx.SolidBrush(scaleColor(fill.Color, materialOpacity*fill.Opacity)),
			})
		case FillGradient:
			if fill.Opacity <= 0 || fill.Gradient.Type != GradientLinear || len(fill.Gradient.Stops) == 0 {
				continue
			}
			stops := make([]gfx.GradientStop, len(fill.Gradient.Stops))
			for i, stop := range fill.Gradient.Stops {
				stops[i] = gfx.GradientStop{Offset: stop.Position, Color: scaleColor(stop.Color, materialOpacity*fill.Opacity)}
			}
			// Gradient Start/End are in [0,1] normalized space relative to the
			// path bounding box. Map them to absolute pixel coordinates so the
			// software renderer's dot-product projection is correct.
			bbox := pathBounds(path)
			start := resolveGradientPoint(fill.Gradient.Start, bbox)
			end := resolveGradientPoint(fill.Gradient.End, bbox)
			cmds = append(cmds, gfx.FillPath{
				Path:  path,
				Brush: gfx.LinearGradientBrush(start, end, stops),
			})
		case FillTexture:
			if fill.Opacity <= 0 {
				continue
			}
			texData, ok := GetTexture(fill.Texture.Ref)
			if ok && texData.Image != nil {
				bounds := pathBounds(path)
				cmds = append(cmds, gfx.DrawImage{
					Image:    texData.Image,
					DestRect: bounds,
					SrcRect:  gfx.Rect{Max: gfx.Point{X: float32(texData.Image.Bounds().Dx()), Y: float32(texData.Image.Bounds().Dy())}},
					Sampling: gfx.SamplingBilinear,
					Opacity:  materialOpacity * fill.Opacity,
				})
			}
		}
	}

	for _, stroke := range material.Strokes {
		if stroke.Width <= 0 || stroke.Paint.Type != FillSolid || stroke.Paint.Color.A <= 0 || stroke.Paint.Opacity <= 0 {
			continue
		}

		// Map shadows, glows, embossing, and highlights to analytical soft shadows
		if stroke.BlurRadius > 0 || stroke.Offset != (gfx.Point{}) {
			offset := stroke.Offset
			if GlobalLighting.Enabled {
				rad := float64(GlobalLighting.Angle) * math.Pi / 180.0
				lx := float32(math.Cos(rad))
				ly := float32(math.Sin(rad))

				isHighlight := isColorLight(stroke.Paint.Color)
				mag := float32(math.Sqrt(float64(offset.X*offset.X + offset.Y*offset.Y)))
				if mag == 0 {
					mag = stroke.BlurRadius * 0.5
					if mag <= 0 {
						mag = 2
					}
				}

				if isHighlight {
					// Highlight is cast towards the light source
					offset = gfx.Point{X: lx * mag, Y: ly * mag}
				} else {
					// Shadow is cast away from the light source
					offset = gfx.Point{X: -lx * mag, Y: -ly * mag}
				}
			}

			cmds = append(cmds, gfx.DrawBlurredShadow{
				Path:       path,
				Color:      scaleColor(stroke.Paint.Color, materialOpacity*stroke.Paint.Opacity),
				BlurRadius: stroke.BlurRadius,
				Offset:     offset,
				Inner:      stroke.Inner,
			})
		} else {
			cmds = append(cmds, gfx.StrokePath{
				Path:  path,
				Brush: gfx.SolidBrush(scaleColor(stroke.Paint.Color, materialOpacity*stroke.Paint.Opacity)),
				Stroke: gfx.StrokeStyle{
					Width:      stroke.Width,
					Cap:        convertLineCap(stroke.Cap),
					Join:       convertLineJoin(stroke.Join),
					MiterLimit: 10,
					Dash:       append([]float32(nil), stroke.Dash...),
					DashOffset: stroke.DashOffset,
				},
			})
		}
	}
	return cmds
}

// IsTransparentMaterial reports whether a material would render any visible output.
func IsTransparentMaterial(material Material) bool {
	return Transparent(material)
}

// MaterialColor returns the first visible color from a material, including opacity.
func MaterialColor(material Material) gfx.Color {
	return Color(material)
}

func convertLineCap(cap StrokeCap) gfx.LineCap {
	switch cap {
	case CapRound:
		return gfx.LineCapRound
	case CapSquare:
		return gfx.LineCapSquare
	default:
		return gfx.LineCapButt
	}
}

func convertLineJoin(join StrokeJoin) gfx.LineJoin {
	switch join {
	case JoinRound:
		return gfx.LineJoinRound
	case JoinBevel:
		return gfx.LineJoinBevel
	default:
		return gfx.LineJoinMiter
	}
}

// pathPointCount returns the number of semantically meaningful Pts entries for
// each PathVerb. PathMoveTo and PathLineTo use only Pts[0]; PathQuadTo uses
// Pts[0..1]; PathCubicTo uses all three. The remaining entries are zero-valued
// and must not be included in any bounding-box calculation.
func pathPointCount(verb gfx.PathVerb) int {
	switch verb {
	case gfx.PathMoveTo, gfx.PathLineTo:
		return 1
	case gfx.PathQuadTo:
		return 2
	case gfx.PathCubicTo:
		return 3
	default:
		return 0
	}
}

func pathBounds(path gfx.Path) gfx.Rect {
	if len(path.Segments) == 0 {
		return gfx.Rect{}
	}
	minX := float32(math.MaxFloat32)
	minY := float32(math.MaxFloat32)
	maxX := float32(-math.MaxFloat32)
	maxY := float32(-math.MaxFloat32)
	for _, seg := range path.Segments {
		n := pathPointCount(seg.Verb)
		for i := 0; i < n; i++ {
			p := seg.Pts[i]
			if p.X < minX {
				minX = p.X
			}
			if p.Y < minY {
				minY = p.Y
			}
			if p.X > maxX {
				maxX = p.X
			}
			if p.Y > maxY {
				maxY = p.Y
			}
		}
	}
	if minX > maxX || minY > maxY {
		return gfx.Rect{}
	}
	return gfx.Rect{Min: gfx.Point{X: minX, Y: minY}, Max: gfx.Point{X: maxX, Y: maxY}}
}

// resolveGradientPoint maps a normalized [0,1] gradient coordinate to an
// absolute pixel position within the path bounding box. This allows theme
// recipes to express gradients in a coordinate-system-agnostic way without
// knowledge of the component's screen position or size.
//
// For example, Start:{X:0,Y:0}, End:{X:0,Y:1} produces a vertical
// top-to-bottom gradient that always spans the full path height, regardless
// of where the path sits on screen.
func resolveGradientPoint(pt gfx.Point, bbox gfx.Rect) gfx.Point {
	return gfx.Point{
		X: bbox.Min.X + pt.X*(bbox.Max.X-bbox.Min.X),
		Y: bbox.Min.Y + pt.Y*(bbox.Max.Y-bbox.Min.Y),
	}
}

func isColorLight(c gfx.Color) bool {
	// standard luminance formula
	lum := 0.299*float32(c.R) + 0.587*float32(c.G) + 0.114*float32(c.B)
	return lum > 128
}
