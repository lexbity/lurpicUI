package projection

import (
	"math"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/voiceux"
)

// LayerKind identifies one deterministic projection layer.
type LayerKind string

const (
	LayerRect     LayerKind = "rect"
	LayerPoint    LayerKind = "point"
	LayerPoints   LayerKind = "points"
	LayerPolyline LayerKind = "polyline"
)

// Layer is the reusable projection primitive used by Voice UX tests.
type Layer struct {
	Name    string
	Kind    LayerKind
	Rect    gfx.Rect
	Points  []gfx.Point
	Point   gfx.Point
	Value   float32
	Label   string
	Visible bool
	Color   gfx.Color
	Radius  float32
}

// MeterSnapshot is the deterministic projection of the meter facet.
type MeterSnapshot struct {
	Bounds gfx.Rect
	Layers []Layer
}

// MeterFromParams projects audio params into a deterministic layer set.
func MeterFromParams(bounds gfx.Rect, params voiceux.AudioParamsView) MeterSnapshot {
	layers := []Layer{
		{Name: "background", Kind: LayerRect, Rect: bounds, Visible: true, Color: gfx.ColorFromHex(0x11151BFF)},
		{Name: "rms", Kind: LayerRect, Rect: fillFromBottom(bounds, clamp01(params.RMS)), Value: clamp01(params.RMS), Visible: true, Color: gfx.ColorFromHex(0x4DD0FFFF)},
		{Name: "peak", Kind: LayerRect, Rect: thinMarker(bounds, clamp01(params.Peak)), Value: clamp01(params.Peak), Visible: true, Color: gfx.ColorFromHex(0xFFB74DFF)},
		{Name: "energy", Kind: LayerRect, Rect: fillFromBottom(bounds, clamp01(params.Energy)), Value: clamp01(params.Energy), Visible: true, Color: gfx.ColorFromHex(0x81C784FF)},
		{Name: "pitch", Kind: LayerRect, Rect: fillFromBottom(bounds, pitchNorm(params.PitchHz)), Value: pitchNorm(params.PitchHz), Visible: true, Color: gfx.ColorFromHex(0xCE93D8FF)},
		{Name: "mouth", Kind: LayerRect, Rect: fillFromBottom(bounds, clamp01(params.MouthOpen)), Value: clamp01(params.MouthOpen), Visible: true, Color: gfx.ColorFromHex(0xFF8A65FF)},
		{Name: "speaking", Kind: LayerRect, Rect: fillFromBottom(bounds, clamp01(params.SpeakingConf)), Value: clamp01(params.SpeakingConf), Visible: true, Color: gfx.ColorFromHex(0x29B6F6FF)},
		{Name: "vowel", Kind: LayerRect, Rect: fillFromBottom(bounds, clamp01(params.VowelConf)), Value: clamp01(params.VowelConf), Visible: true, Color: gfx.ColorFromHex(0x26A69AFF)},
	}
	if params.Clipping {
		layers = append(layers, Layer{Name: "clipping_badge", Kind: LayerRect, Rect: badgeRect(bounds, 0), Visible: true, Color: gfx.ColorFromHex(0xEF5350FF), Label: "clip"})
	}
	if params.Dropout {
		layers = append(layers, Layer{Name: "dropout_badge", Kind: LayerRect, Rect: badgeRect(bounds, 1), Visible: true, Color: gfx.ColorFromHex(0x90A4AEFF), Label: "drop"})
	}
	return MeterSnapshot{Bounds: bounds, Layers: layers}
}

// AppendCommands appends commands for all visible layers.
func (s MeterSnapshot) AppendCommands(list *gfx.CommandList) {
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
			list.Add(gfx.DrawPolyline{Points: layer.Points, Brush: gfx.SolidBrush(layer.Color), Closed: false})
		}
	}
}

func fillFromBottom(bounds gfx.Rect, value float32) gfx.Rect {
	value = clamp01(value)
	height := bounds.Height() * value
	return gfx.RectFromXYWH(bounds.Min.X, bounds.Max.Y-height, bounds.Width(), height)
}

func thinMarker(bounds gfx.Rect, value float32) gfx.Rect {
	value = clamp01(value)
	y := bounds.Max.Y - bounds.Height()*value
	return gfx.RectFromXYWH(bounds.Min.X, y-1, bounds.Width(), 2)
}

func badgeRect(bounds gfx.Rect, idx int) gfx.Rect {
	size := float32(math.Min(float64(bounds.Width()), float64(bounds.Height()))) * 0.12
	if size < 8 {
		size = 8
	}
	x := bounds.Max.X - size - float32(idx)*((size*0.9)+2)
	y := bounds.Min.Y + 2
	return gfx.RectFromXYWH(x, y, size, size)
}

func pitchNorm(hz float32) float32 {
	if hz <= 0 {
		return 0
	}
	return clamp01((hz - 60) / (600 - 60))
}

func radiusOr(v, fallback float32) float32 {
	if v > 0 {
		return v
	}
	return fallback
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
