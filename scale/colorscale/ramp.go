package colorscale

import (
	"math"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// InterpolationSpace selects the color space for ramp interpolation.
type InterpolationSpace uint8

const (
	// InterpolationOKLab uses perceptually-uniform OKLab interpolation (default).
	InterpolationOKLab InterpolationSpace = iota
	// InterpolationLinearSRGB uses linear sRGB interpolation (faster).
	InterpolationLinearSRGB
)

// ColorStop defines a color at a position t in [0, 1] along a ramp.
type ColorStop struct {
	T     float64
	Color gfx.Color
}

// Ramp is a sequence of color stops defining a continuous gradient.
type Ramp []ColorStop

// At evaluates the ramp at position t ∈ [0, 1] using the given interpolation
// space. t is clamped to [0, 1] before evaluation.
func (r Ramp) At(t float64, space InterpolationSpace) gfx.Color {
	if len(r) == 0 {
		return gfx.Color{}
	}
	if t <= r[0].T {
		return r[0].Color
	}
	if t >= r[len(r)-1].T {
		return r[len(r)-1].Color
	}
	for i := 0; i < len(r)-1; i++ {
		if math.IsNaN(t) {
			return r[0].Color
		}
		if t >= r[i].T && t < r[i+1].T {
			localT := (t - r[i].T) / (r[i+1].T - r[i].T)
			return interpolateColor(r[i].Color, r[i+1].Color, localT, space)
		}
	}
	return r[len(r)-1].Color
}

func interpolateColor(a, b gfx.Color, t float64, space InterpolationSpace) gfx.Color {
	switch space {
	case InterpolationLinearSRGB:
		return interpolateLinearSRGB(a, b, t)
	default:
		return interpolateOKLab(a, b, t)
	}
}

func interpolateOKLab(a, b gfx.Color, t float64) gfx.Color {
	l1, a1, b1 := SRGBToOKLab(a)
	l2, a2, b2 := SRGBToOKLab(b)
	return OKLabToSRGB(
		l1+(l2-l1)*t,
		a1+(a2-a1)*t,
		b1+(b2-b1)*t,
	)
}

func interpolateLinearSRGB(a, b gfx.Color, t float64) gfx.Color {
	// Un-premultiply to get straight sRGB
	var ar, ag, ab, br, bg, bb float64
	if a.A > 0 {
		invA := 1 / float64(a.A)
		ar = float64(a.R) * invA
		ag = float64(a.G) * invA
		ab = float64(a.B) * invA
	}
	if b.A > 0 {
		invA := 1 / float64(b.A)
		br = float64(b.R) * invA
		bg = float64(b.G) * invA
		bb = float64(b.B) * invA
	}
	// Degamma to linear
	ar, ag, ab = SRGBToLinear(ar), SRGBToLinear(ag), SRGBToLinear(ab)
	br, bg, bb = SRGBToLinear(br), SRGBToLinear(bg), SRGBToLinear(bb)
	// Interpolate in linear space
	r := ar + (br-ar)*t
	g := ag + (bg-ag)*t
	bb = ab + (bb-ab)*t
	// Gamma encode to sRGB
	rs := clamp01(LinearToSRGB(r))
	gs := clamp01(LinearToSRGB(g))
	bs := clamp01(LinearToSRGB(bb))
	// Premultiply with alpha blend from both endpoints
	alpha := float64(a.A) + (float64(b.A)-float64(a.A))*t
	return gfx.Color{
		R: float32(rs * clamp01(alpha)),
		G: float32(gs * clamp01(alpha)),
		B: float32(bs * clamp01(alpha)),
		A: float32(clamp01(alpha)),
	}
}

// Built-in ramps.
var (
	// RampViridis is a green-purple-yellow perceptually-uniform colormap.
	RampViridis = Ramp{
		{T: 0.00, Color: viridis0},
		{T: 0.25, Color: viridis25},
		{T: 0.50, Color: viridis50},
		{T: 0.75, Color: viridis75},
		{T: 1.00, Color: viridis100},
	}

	// RampInferno is a dark-to-bright perceptually-uniform colormap.
	RampInferno = Ramp{
		{T: 0.00, Color: inferno0},
		{T: 0.25, Color: inferno25},
		{T: 0.50, Color: inferno50},
		{T: 0.75, Color: inferno75},
		{T: 1.00, Color: inferno100},
	}

	// RampGrayscale is a simple black-to-white ramp.
	RampGrayscale = Ramp{
		{T: 0.00, Color: gfx.Color{R: 0, G: 0, B: 0, A: 1}},
		{T: 1.00, Color: gfx.Color{R: 1, G: 1, B: 1, A: 1}},
	}

	// Diverging ramp halves for blue-white-red.
	RampBlueWhiteRedLow  = Ramp{{T: 0, Color: divergingBlueLow}, {T: 1, Color: whiteColor}}
	RampBlueWhiteRedHigh = Ramp{{T: 0, Color: whiteColor}, {T: 1, Color: divergingRedHigh}}

	// Diverging ramp halves for purple-white-green.
	RampPurpleWhiteGreenLow  = Ramp{{T: 0, Color: divergingPurpleLow}, {T: 1, Color: whiteColor}}
	RampPurpleWhiteGreenHigh = Ramp{{T: 0, Color: whiteColor}, {T: 1, Color: divergingGreenHigh}}
)

var (
	whiteColor = gfx.Color{R: 1, G: 1, B: 1, A: 1}

	divergingBlueLow   = gfx.ColorFromHex(0x4575b4ff)
	divergingRedHigh   = gfx.ColorFromHex(0xd73027ff)
	divergingPurpleLow = gfx.ColorFromHex(0x762a83ff)
	divergingGreenHigh = gfx.ColorFromHex(0x1a9641ff)

	viridis0   = gfx.ColorFromHex(0x440154ff)
	viridis25  = gfx.ColorFromHex(0x3b528bff)
	viridis50  = gfx.ColorFromHex(0x21918dff)
	viridis75  = gfx.ColorFromHex(0x5ec962ff)
	viridis100 = gfx.ColorFromHex(0xfde725ff)
	inferno0   = gfx.ColorFromHex(0x000004ff)
	inferno25  = gfx.ColorFromHex(0x420a68ff)
	inferno50  = gfx.ColorFromHex(0x932667ff)
	inferno75  = gfx.ColorFromHex(0xdd513aff)
	inferno100 = gfx.ColorFromHex(0xfcffa4ff)
)
