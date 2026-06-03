package colorscale

import (
	"math"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// SRGBToLinear decodes a single sRGB gamma-encoded component (0–1) to
// linear RGB.
func SRGBToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// LinearToSRGB encodes a single linear RGB component (0–1) to sRGB
// gamma-encoded.
func LinearToSRGB(c float64) float64 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return 1.055*math.Pow(c, 1.0/2.4) - 0.055
}

// m1 is the linear sRGB → LMS matrix (OKLab M1).
var m1 = [3][3]float64{
	{0.4122214708, 0.5363325363, 0.0514459929},
	{0.2119034982, 0.6806995451, 0.1073969566},
	{0.0883024619, 0.2817188376, 0.6299787005},
}

// m1Inv is the LMS → linear sRGB matrix (inverse of M1).
var m1Inv = [3][3]float64{
	{4.0767416621, -3.3077115913, 0.2309699292},
	{-1.2684380046, 2.6097574011, -0.3413193965},
	{-0.0041960863, -0.7034186147, 1.7076147010},
}

// m2 is the LMS' → OKLab matrix (OKLab M2).
var m2 = [3][3]float64{
	{0.2104542553, 0.7936177850, -0.0040720468},
	{1.9779984951, -2.4285922050, 0.4505937099},
	{0.0259040371, 0.7827717662, -0.8086757660},
}

// m2Inv is the OKLab → LMS' matrix (inverse of M2).
var m2Inv = [3][3]float64{
	{1.0000000000, 0.3963377774, 0.2158037573},
	{1.0000000000, -0.1055613458, -0.0638541728},
	{1.0000000000, -0.0894841775, -1.2914855480},
}

func matMul(m [3][3]float64, v [3]float64) [3]float64 {
	return [3]float64{
		m[0][0]*v[0] + m[0][1]*v[1] + m[0][2]*v[2],
		m[1][0]*v[0] + m[1][1]*v[1] + m[1][2]*v[2],
		m[2][0]*v[0] + m[2][1]*v[1] + m[2][2]*v[2],
	}
}

// LinearRGBToOKLab converts linear RGB values in [0, 1] to OKLab.
func LinearRGBToOKLab(r, g, b float64) (L, a, bb float64) {
	lms := matMul(m1, [3]float64{r, g, b})
	lms[0] = math.Cbrt(lms[0])
	lms[1] = math.Cbrt(lms[1])
	lms[2] = math.Cbrt(lms[2])
	lab := matMul(m2, lms)
	return lab[0], lab[1], lab[2]
}

// OKLabToLinearRGB converts OKLab to linear RGB values in [0, 1].
func OKLabToLinearRGB(L, a, bb float64) (r, g, b float64) {
	lms := matMul(m2Inv, [3]float64{L, a, bb})
	lms[0] = lms[0] * lms[0] * lms[0] // cube
	lms[1] = lms[1] * lms[1] * lms[1]
	lms[2] = lms[2] * lms[2] * lms[2]
	rgb := matMul(m1Inv, lms)
	return rgb[0], rgb[1], rgb[2]
}

// SRGBToOKLab converts an sRGB color (with premultiplied alpha) to OKLab.
// The alpha is ignored; only the color is converted.
func SRGBToOKLab(col gfx.Color) (L, a, bb float64) {
	// Un-premultiply to get straight sRGB values
	var rs, gs, bs float64
	if col.A > 0 {
		invA := 1 / float64(col.A)
		rs = float64(col.R) * invA
		gs = float64(col.G) * invA
		bs = float64(col.B) * invA
	}
	// Degamma to linear
	rLin := SRGBToLinear(clamp01(rs))
	gLin := SRGBToLinear(clamp01(gs))
	bLin := SRGBToLinear(clamp01(bs))
	return LinearRGBToOKLab(rLin, gLin, bLin)
}

// OKLabToSRGB converts OKLab to an sRGB color with alpha = 1.
func OKLabToSRGB(L, a, bb float64) gfx.Color {
	rLin, gLin, bLin := OKLabToLinearRGB(L, a, bb)
	// Gamma encode
	rs := clamp01(LinearToSRGB(rLin))
	gs := clamp01(LinearToSRGB(gLin))
	bs := clamp01(LinearToSRGB(bLin))
	// Premultiply with alpha = 1
	return gfx.Color{
		R: float32(rs),
		G: float32(gs),
		B: float32(bs),
		A: 1,
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
