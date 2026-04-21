package theme

import (
	"fmt"
	"math"
	"reflect"
	"sync"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// Fill describes how the interior of a mark is painted.
type Fill struct {
	Type     FillType
	Color    gfx.Color
	Gradient Gradient
	Texture  TextureFill
	Pattern  PatternFill
	Opacity  float32
}

// FillType identifies the active fill variant.
type FillType uint8

const (
	FillNone FillType = iota
	FillSolid
	FillGradient
	FillTexture
	FillPattern
)

// Gradient describes a color ramp.
type Gradient struct {
	Type   GradientType
	Start  gfx.Point
	End    gfx.Point
	Center gfx.Point
	Radius float32
	Stops  []GradientStop
}

// GradientType identifies the active gradient variant.
type GradientType uint8

const (
	GradientLinear GradientType = iota
	GradientRadial
	GradientAngular
)

// GradientStop defines one gradient stop.
type GradientStop struct {
	Position float32
	Color    gfx.Color
}

// TextureFill describes a texture reference.
type TextureFill struct {
	Ref       TextureRef
	Transform gfx.Transform
	Repeat    RepeatMode
}

// RepeatMode controls texture tiling.
type RepeatMode uint8

const (
	RepeatClamp RepeatMode = iota
	RepeatTile
	RepeatMirror
)

// PatternFill describes a pattern reference.
type PatternFill struct {
	Ref       PatternRef
	Transform gfx.Transform
	Scale     float32
}

// TextureRef is an opaque texture handle.
type TextureRef uint32

// PatternRef is an opaque pattern handle.
type PatternRef uint32

// StrokeCap controls stroke end caps.
type StrokeCap uint8

const (
	CapButt StrokeCap = iota
	CapRound
	CapSquare
)

// StrokeJoin controls stroke joins.
type StrokeJoin uint8

const (
	JoinMiter StrokeJoin = iota
	JoinRound
	JoinBevel
)

// MaterialStroke describes the edge and optional shadow/glow of a mark.
type MaterialStroke struct {
	Paint      Fill
	Width      float32
	Cap        StrokeCap
	Join       StrokeJoin
	Dash       []float32
	DashOffset float32
	BlurRadius float32
	Offset     gfx.Point
	Inner      bool
}

// Material is a composite visual description.
type Material struct {
	Fills   []Fill
	Strokes []MaterialStroke
	Opacity float32
}

// SolidMaterial returns a simple fill/stroke material.
func SolidMaterial(fill gfx.Color, stroke gfx.Color, strokeWidth float32) Material {
	m := Material{
		Fills: []Fill{{
			Type:    FillSolid,
			Color:   fill,
			Opacity: 1,
		}},
		Opacity: 1,
	}
	if strokeWidth > 0 {
		m.Strokes = []MaterialStroke{{
			Paint: Fill{
				Type:    FillSolid,
				Color:   stroke,
				Opacity: 1,
			},
			Width: strokeWidth,
			Cap:   CapButt,
			Join:  JoinMiter,
		}}
	}
	return m
}

// FromToken returns a solid material from a token color.
func FromToken(color gfx.Color) Material {
	return Material{
		Fills: []Fill{{
			Type:    FillSolid,
			Color:   color,
			Opacity: 1,
		}},
		Opacity: 1,
	}
}

// ValidateMaterial enforces the declared soft limits.
func ValidateMaterial(m Material) error {
	if len(m.Fills) > 3 {
		return fmt.Errorf("theme: material has %d fills; maximum is 3", len(m.Fills))
	}
	if len(m.Strokes) > 3 {
		return fmt.Errorf("theme: material has %d strokes; maximum is 3", len(m.Strokes))
	}
	return nil
}

// Lerp returns a blended fill.
func (f Fill) Lerp(other Fill, t float32) Fill {
	if reflect.DeepEqual(f, other) {
		return f
	}
	t = clamp01(t)
	if t == 0 {
		return f
	}
	if t == 1 {
		return other
	}
	if f.Type != other.Type {
		if t >= 0.5 {
			return other
		}
		return f
	}

	out := f
	out.Opacity = lerpFloat32(f.Opacity, other.Opacity, t)
	switch f.Type {
	case FillNone:
		if t >= 0.5 {
			return other
		}
		return f
	case FillSolid:
		out.Color = lerpColor(f.Color, other.Color, t)
	case FillGradient:
		if f.Gradient.Type != other.Gradient.Type || len(f.Gradient.Stops) != len(other.Gradient.Stops) {
			if t >= 0.5 {
				return other
			}
			return f
		}
		out.Gradient.Type = f.Gradient.Type
		out.Gradient.Start = lerpPoint(f.Gradient.Start, other.Gradient.Start, t)
		out.Gradient.End = lerpPoint(f.Gradient.End, other.Gradient.End, t)
		out.Gradient.Center = lerpPoint(f.Gradient.Center, other.Gradient.Center, t)
		out.Gradient.Radius = lerpFloat32(f.Gradient.Radius, other.Gradient.Radius, t)
		out.Gradient.Stops = make([]GradientStop, len(f.Gradient.Stops))
		for i := range out.Gradient.Stops {
			out.Gradient.Stops[i] = GradientStop{
				Position: lerpFloat32(f.Gradient.Stops[i].Position, other.Gradient.Stops[i].Position, t),
				Color:    lerpColor(f.Gradient.Stops[i].Color, other.Gradient.Stops[i].Color, t),
			}
		}
	case FillTexture:
		out.Texture.Transform = lerpTransform(f.Texture.Transform, other.Texture.Transform, t)
		if f.Texture.Repeat != other.Texture.Repeat {
			if t >= 0.5 {
				out.Texture.Repeat = other.Texture.Repeat
			}
		}
		if f.Texture.Ref != other.Texture.Ref {
			if t >= 0.5 {
				out.Texture.Ref = other.Texture.Ref
			}
			out.Opacity = out.Opacity * 0.5
		}
	case FillPattern:
		out.Pattern.Transform = lerpTransform(f.Pattern.Transform, other.Pattern.Transform, t)
		out.Pattern.Scale = lerpFloat32(f.Pattern.Scale, other.Pattern.Scale, t)
		if f.Pattern.Ref != other.Pattern.Ref && t >= 0.5 {
			out.Pattern.Ref = other.Pattern.Ref
		}
	default:
		if t >= 0.5 {
			return other
		}
		return f
	}
	return out
}

// Lerp returns a blended stroke.
func (s MaterialStroke) Lerp(other MaterialStroke, t float32) MaterialStroke {
	if reflect.DeepEqual(s, other) {
		return s
	}
	t = clamp01(t)
	if t == 0 {
		return s
	}
	if t == 1 {
		return other
	}
	out := s
	out.Paint = s.Paint.Lerp(other.Paint, t)
	out.Width = lerpFloat32(s.Width, other.Width, t)
	out.DashOffset = lerpFloat32(s.DashOffset, other.DashOffset, t)
	out.BlurRadius = lerpFloat32(s.BlurRadius, other.BlurRadius, t)
	out.Offset = lerpPoint(s.Offset, other.Offset, t)
	out.Inner = other.Inner
	if t < 0.5 {
		out.Cap = s.Cap
		out.Join = s.Join
	} else {
		out.Cap = other.Cap
		out.Join = other.Join
	}
	if len(s.Dash) == 0 && len(other.Dash) == 0 {
		out.Dash = nil
		return out
	}
	maxLen := len(s.Dash)
	if len(other.Dash) > maxLen {
		maxLen = len(other.Dash)
	}
	out.Dash = make([]float32, maxLen)
	for i := 0; i < maxLen; i++ {
		out.Dash[i] = lerpFloat32(indexFloat32(s.Dash, i), indexFloat32(other.Dash, i), t)
	}
	return out
}

// Lerp returns a blended material.
func (m Material) Lerp(other Material, t float32) Material {
	if reflect.DeepEqual(m, other) {
		return m
	}
	t = clamp01(t)
	if t == 0 {
		return m
	}
	if t == 1 {
		return other
	}
	out := Material{
		Opacity: lerpFloat32(m.Opacity, other.Opacity, t),
	}
	maxFills := len(m.Fills)
	if len(other.Fills) > maxFills {
		maxFills = len(other.Fills)
	}
	out.Fills = make([]Fill, maxFills)
	for i := 0; i < maxFills; i++ {
		out.Fills[i] = indexFill(m.Fills, i).Lerp(indexFill(other.Fills, i), t)
	}
	maxStrokes := len(m.Strokes)
	if len(other.Strokes) > maxStrokes {
		maxStrokes = len(other.Strokes)
	}
	out.Strokes = make([]MaterialStroke, maxStrokes)
	for i := 0; i < maxStrokes; i++ {
		out.Strokes[i] = indexStroke(m.Strokes, i).Lerp(indexStroke(other.Strokes, i), t)
	}
	return out
}

// MaterialRegistry stores named materials.
type MaterialRegistry struct {
	mu        sync.RWMutex
	materials map[string]Material
}

// NewMaterialRegistry constructs an empty registry.
func NewMaterialRegistry() *MaterialRegistry {
	return &MaterialRegistry{materials: make(map[string]Material)}
}

// Define registers or replaces a material.
func (r *MaterialRegistry) Define(name string, m Material) {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.materials == nil {
		r.materials = make(map[string]Material)
	}
	r.materials[name] = m
	r.mu.Unlock()
}

// Get returns a material by name.
func (r *MaterialRegistry) Get(name string) (Material, bool) {
	if r == nil {
		return Material{}, false
	}
	r.mu.RLock()
	m, ok := r.materials[name]
	r.mu.RUnlock()
	return m, ok
}

// MustGet returns a material or panics if missing.
func (r *MaterialRegistry) MustGet(name string) Material {
	if m, ok := r.Get(name); ok {
		return m
	}
	panic(fmt.Sprintf("theme: material %q not found", name))
}

func lerpFloat32(a, b, t float32) float32 {
	return a + (b-a)*t
}

func lerpPoint(a, b gfx.Point, t float32) gfx.Point {
	return gfx.Point{
		X: lerpFloat32(a.X, b.X, t),
		Y: lerpFloat32(a.Y, b.Y, t),
	}
}

func lerpTransform(a, b gfx.Transform, t float32) gfx.Transform {
	return gfx.Transform{
		A:  lerpFloat32(a.A, b.A, t),
		B:  lerpFloat32(a.B, b.B, t),
		C:  lerpFloat32(a.C, b.C, t),
		D:  lerpFloat32(a.D, b.D, t),
		TX: lerpFloat32(a.TX, b.TX, t),
		TY: lerpFloat32(a.TY, b.TY, t),
	}
}

func lerpColor(a, b gfx.Color, t float32) gfx.Color {
	return gfx.Color{
		R: lerpFloat32(a.R, b.R, t),
		G: lerpFloat32(a.G, b.G, t),
		B: lerpFloat32(a.B, b.B, t),
		A: lerpFloat32(a.A, b.A, t),
	}
}

func indexFill(in []Fill, i int) Fill {
	if i < len(in) {
		return in[i]
	}
	return Fill{}
}

func indexStroke(in []MaterialStroke, i int) MaterialStroke {
	if i < len(in) {
		return in[i]
	}
	return MaterialStroke{}
}

func indexFloat32(in []float32, i int) float32 {
	if i < len(in) {
		return in[i]
	}
	return 0
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 || math.IsNaN(float64(v)) {
		return 1
	}
	return v
}
