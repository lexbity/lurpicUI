package gfx

import "math"

type Color struct {
	R, G, B, A float32
}

func ColorFromRGBA8(r, g, b, a uint8) Color {
	alpha := float32(a) / 255
	return Color{
		R: float32(r) / 255 * alpha,
		G: float32(g) / 255 * alpha,
		B: float32(b) / 255 * alpha,
		A: alpha,
	}
}

func ColorFromHex(hex uint32) Color {
	r := uint8((hex >> 24) & 0xFF)
	g := uint8((hex >> 16) & 0xFF)
	b := uint8((hex >> 8) & 0xFF)
	a := uint8(hex & 0xFF)
	return ColorFromRGBA8(r, g, b, a)
}

func (c Color) WithAlpha(a float32) Color {
	c.A = a
	return c
}

func (c Color) ToRGBA8() (r, g, b, a uint8) {
	a = float32ToByte(c.A)
	if a == 0 {
		return 0, 0, 0, 0
	}

	alpha := float32(a) / 255
	r = float32ToByte(clamp01(c.R / alpha))
	g = float32ToByte(clamp01(c.G / alpha))
	b = float32ToByte(clamp01(c.B / alpha))
	return r, g, b, a
}

func (c Color) Premultiply() Color {
	return Color{
		R: c.R * c.A,
		G: c.G * c.A,
		B: c.B * c.A,
		A: c.A,
	}
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

func float32ToByte(v float32) uint8 {
	return uint8(math.Round(float64(clamp01(v) * 255)))
}
