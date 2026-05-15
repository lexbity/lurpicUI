package projection

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/voiceux"
)

// VowelPoint identifies a plotted vowel-space point.
type VowelPoint struct {
	Name       string
	Point      gfx.Point
	Confidence float32
	Selected   bool
	Visible    bool
}

// VowelSpaceSnapshot is the deterministic projection for vowel-space facets.
type VowelSpaceSnapshot struct {
	Bounds gfx.Rect
	Layers []Layer
	Points []VowelPoint
}

// VowelSpaceFromState projects formant data and calibration state to layers.
func VowelSpaceFromState(bounds gfx.Rect, params voiceux.AudioParamsView, cal voiceux.CalibrationStateView) VowelSpaceSnapshot {
	inner := bounds.Inset(bounds.Width()*0.08, bounds.Height()*0.08)
	if inner.Width() < 1 || inner.Height() < 1 {
		inner = bounds
	}
	centerY := inner.Min.Y + inner.Height()/2
	centerX := inner.Min.X + inner.Width()/2
	axes := []Layer{
		{
			Name:    "axis_x",
			Kind:    LayerPolyline,
			Points:  []gfx.Point{{X: inner.Min.X, Y: centerY}, {X: inner.Max.X, Y: centerY}},
			Visible: true,
			Color:   gfx.ColorFromHex(0x607D8BFF),
		},
		{
			Name:    "axis_y",
			Kind:    LayerPolyline,
			Points:  []gfx.Point{{X: centerX, Y: inner.Min.Y}, {X: centerX, Y: inner.Max.Y}},
			Visible: true,
			Color:   gfx.ColorFromHex(0x607D8BFF),
		},
	}
	livePoint := VowelPoint{
		Name:       "live",
		Point:      mapFormants(inner, params.F1Hz, params.F2Hz),
		Confidence: clamp01(params.FormantConf),
		Selected:   true,
		Visible:    true,
	}
	points := []VowelPoint{livePoint}
	if cal.Phase != "" {
		points = append(points, VowelPoint{
			Name:       "calibration",
			Point:      livePoint.Point,
			Confidence: clamp01(params.VowelConf),
			Selected:   false,
			Visible:    true,
		})
	}
	layers := append([]Layer{{Name: "background", Kind: LayerRect, Rect: bounds, Visible: true, Color: gfx.ColorFromHex(0x0F1318FF)}}, axes...)
	layers = append(layers, Layer{
		Name:    "live_point",
		Kind:    LayerPoint,
		Point:   livePoint.Point,
		Value:   livePoint.Confidence,
		Label:   params.Vowel.String(),
		Visible: true,
		Color:   gfx.ColorFromHex(0xFFCC80FF),
		Radius:  5,
	})
	if params.VowelConf > 0.6 {
		layers = append(layers, Layer{
			Name:    "stable_point",
			Kind:    LayerPoint,
			Point:   livePoint.Point,
			Value:   params.VowelConf,
			Label:   params.Vowel.String(),
			Visible: true,
			Color:   gfx.ColorFromHex(0x81C784FF),
			Radius:  7,
		})
	}
	if params.VowelWeights.A+params.VowelWeights.E+params.VowelWeights.I+params.VowelWeights.O+params.VowelWeights.U > 0 {
		layers = append(layers, Layer{
			Name: "weights",
			Kind: LayerPoints,
			Points: []gfx.Point{
				{X: inner.Min.X, Y: inner.Max.Y},
				{X: inner.Min.X + inner.Width()*0.25, Y: inner.Max.Y - inner.Height()*0.2},
				{X: inner.Min.X + inner.Width()*0.5, Y: inner.Max.Y - inner.Height()*0.35},
				{X: inner.Min.X + inner.Width()*0.75, Y: inner.Max.Y - inner.Height()*0.2},
				{X: inner.Max.X, Y: inner.Max.Y},
			},
			Visible: true,
			Color:   gfx.ColorFromHex(0x90CAF9FF),
			Radius:  2,
		})
	}
	return VowelSpaceSnapshot{Bounds: bounds, Layers: layers, Points: points}
}

// AppendCommands appends the layer commands to a command list.
func (s VowelSpaceSnapshot) AppendCommands(list *gfx.CommandList) {
	if list == nil {
		return
	}
	for _, layer := range s.Layers {
		if !layer.Visible {
			continue
		}
		switch layer.Kind {
		case LayerRect:
			list.Add(gfx.FillRect{Rect: layer.Rect, Brush: gfx.SolidBrush(layer.Color)})
		case LayerPoint:
			list.Add(gfx.DrawPoints{Points: []gfx.Point{layer.Point}, Radius: radiusOr(layer.Radius, 4), Brush: gfx.SolidBrush(layer.Color)})
		case LayerPoints:
			list.Add(gfx.DrawPoints{Points: layer.Points, Radius: radiusOr(layer.Radius, 3), Brush: gfx.SolidBrush(layer.Color)})
		case LayerPolyline:
			list.Add(gfx.DrawPolyline{Points: layer.Points, Brush: gfx.SolidBrush(layer.Color)})
		}
	}
}

// MapFormants maps F1/F2 to the drawing bounds.
func MapFormants(bounds gfx.Rect, f1, f2 float32) gfx.Point {
	return mapFormants(bounds, f1, f2)
}

func mapFormants(bounds gfx.Rect, f1, f2 float32) gfx.Point {
	x := clamp01((f2 - 500) / (3000 - 500))
	y := clamp01((f1 - 150) / (1000 - 150))
	return gfx.Point{
		X: bounds.Min.X + bounds.Width()*x,
		Y: bounds.Max.Y - bounds.Height()*y,
	}
}

func normalizeScalar(min, max, v float32) float32 {
	if max <= min {
		return 0
	}
	return clamp01((v - min) / (max - min))
}
