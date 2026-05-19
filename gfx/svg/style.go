package svg

import . "codeburg.org/lexbit/lurpicui/gfx"

func defaultSVGStyleState() svgStyleState {
	return svgStyleState{
		Fill:     SVGPaint{Kind: SVGPaintColor, Color: Color{A: 1}},
		Stroke:   nil,
		Opacity:  1,
		FillRule: SVGFillRuleNonZero,
	}
}

func cloneStroke(s *SVGStroke) *SVGStroke {
	if s == nil {
		return nil
	}
	out := *s
	if s.Dash != nil {
		out.Dash = append([]float32(nil), s.Dash...)
	}
	return &out
}

func applyStyleAttrs(attrs map[string]string, state svgStyleState) (svgStyleState, error) {
	out := state
	if out.Opacity == 0 {
		out.Opacity = 1
	}
	if fill := attrs["fill"]; fill != "" {
		paint, err := parsePaint(fill)
		if err != nil {
			return svgStyleState{}, err
		}
		out.Fill = paint
	}
	if stroke := attrs["stroke"]; stroke != "" {
		paint, err := parsePaint(stroke)
		if err != nil {
			return svgStyleState{}, err
		}
		if paint.Kind == SVGPaintNone {
			out.Stroke = nil
		} else {
			st := out.Stroke
			if st == nil {
				st = &SVGStroke{
					Width:      1,
					Cap:        LineCapButt,
					Join:       LineJoinMiter,
					MiterLimit: 10,
				}
			} else {
				st = cloneStroke(st)
			}
			st.Paint = paint
			out.Stroke = st
		}
	}
	if opacity := attrs["opacity"]; opacity != "" {
		v, err := parseOpacity(opacity)
		if err != nil {
			return svgStyleState{}, err
		}
		out.Opacity *= v
	}
	if fillOpacity := attrs["fill-opacity"]; fillOpacity != "" {
		v, err := parseOpacity(fillOpacity)
		if err != nil {
			return svgStyleState{}, err
		}
		out.Fill.Opacity *= v
	}
	if fillRule := attrs["fill-rule"]; fillRule != "" {
		switch fillRule {
		case "nonzero":
			out.FillRule = SVGFillRuleNonZero
		case "evenodd":
			out.FillRule = SVGFillRuleEvenOdd
		}
	}
	if out.Stroke != nil {
		if strokeOpacity := attrs["stroke-opacity"]; strokeOpacity != "" {
			v, err := parseOpacity(strokeOpacity)
			if err != nil {
				return svgStyleState{}, err
			}
			out.Stroke.Paint.Opacity *= v
		}
		if width, ok, err := parseLength(attrs["stroke-width"]); err != nil {
			return svgStyleState{}, err
		} else if ok {
			out.Stroke.Width = width
		}
		if capValue := attrs["stroke-linecap"]; capValue != "" {
			switch capValue {
			case "butt":
				out.Stroke.Cap = LineCapButt
			case "round":
				out.Stroke.Cap = LineCapRound
			case "square":
				out.Stroke.Cap = LineCapSquare
			}
		}
		if joinValue := attrs["stroke-linejoin"]; joinValue != "" {
			switch joinValue {
			case "miter":
				out.Stroke.Join = LineJoinMiter
			case "round":
				out.Stroke.Join = LineJoinRound
			case "bevel":
				out.Stroke.Join = LineJoinBevel
			}
		}
		if miterLimit, ok, err := parseLength(attrs["stroke-miterlimit"]); err == nil && ok {
			out.Stroke.MiterLimit = miterLimit
		}
		if dashArray := attrs["stroke-dasharray"]; dashArray != "" {
			out.Stroke.Dash = parseFloatList(dashArray)
		}
		if dashOffset, ok, err := parseLength(attrs["stroke-dashoffset"]); err == nil && ok {
			out.Stroke.DashOffset = dashOffset
		}
	}
	if clip := attrs["clip-path"]; clip != "" {
		ref, err := parseURLReference(clip)
		if err != nil {
			return svgStyleState{}, err
		}
		out.ClipPath = ref
	}
	return out, nil
}
