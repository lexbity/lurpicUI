package theme

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestFillLerp_solidIdentityAndMidpoint(t *testing.T) {
	left := Fill{Type: FillSolid, Color: gfx.ColorFromRGBA8(255, 0, 0, 255), Opacity: 1}
	right := Fill{Type: FillSolid, Color: gfx.ColorFromRGBA8(0, 0, 255, 255), Opacity: 1}

	if got := left.Lerp(right, 0); !reflect.DeepEqual(got, left) {
		t.Fatalf("t=0 should return left, got %#v", got)
	}
	if got := left.Lerp(right, 1); !reflect.DeepEqual(got, right) {
		t.Fatalf("t=1 should return right, got %#v", got)
	}

	mid := left.Lerp(right, 0.5)
	expect := Fill{Type: FillSolid, Color: gfx.ColorFromRGBA8(128, 0, 128, 255), Opacity: 1}
	if !colorClose(mid.Color, expect.Color, 1) {
		t.Fatalf("unexpected midpoint color: got %#v want %#v", mid.Color, expect.Color)
	}
}

func TestFillLerp_mismatch_usesFillNoneRule(t *testing.T) {
	left := Fill{Type: FillSolid, Color: gfx.ColorFromRGBA8(255, 0, 0, 255)}
	right := Fill{
		Type: FillGradient,
		Gradient: Gradient{
			Type: GradientLinear,
			Stops: []GradientStop{
				{Position: 0, Color: gfx.ColorFromRGBA8(0, 0, 0, 255)},
				{Position: 1, Color: gfx.ColorFromRGBA8(255, 255, 255, 255)},
			},
		},
	}
	if got := left.Lerp(right, 0.7); !reflect.DeepEqual(got, right) {
		t.Fatalf("mismatch should choose right at t>=0.5, got %#v", got)
	}
}

func TestFillLerp_gradientInterpolatesStops(t *testing.T) {
	left := Fill{
		Type: FillGradient,
		Gradient: Gradient{
			Type:   GradientLinear,
			Start:  gfx.Point{X: 0, Y: 0},
			End:    gfx.Point{X: 1, Y: 0},
			Center: gfx.Point{X: 0, Y: 0},
			Radius: 1,
			Stops: []GradientStop{
				{Position: 0, Color: gfx.ColorFromRGBA8(255, 0, 0, 255)},
				{Position: 1, Color: gfx.ColorFromRGBA8(0, 0, 255, 255)},
			},
		},
	}
	right := Fill{
		Type: FillGradient,
		Gradient: Gradient{
			Type:   GradientLinear,
			Start:  gfx.Point{X: 0, Y: 1},
			End:    gfx.Point{X: 1, Y: 1},
			Center: gfx.Point{X: 1, Y: 1},
			Radius: 3,
			Stops: []GradientStop{
				{Position: 0.5, Color: gfx.ColorFromRGBA8(0, 255, 0, 255)},
				{Position: 1, Color: gfx.ColorFromRGBA8(255, 255, 0, 255)},
			},
		},
	}

	got := left.Lerp(right, 0.5)
	if got.Gradient.Radius != 2 {
		t.Fatalf("unexpected gradient radius: %v", got.Gradient.Radius)
	}
	if got.Gradient.Stops[0].Position != 0.25 {
		t.Fatalf("unexpected first stop position: %v", got.Gradient.Stops[0].Position)
	}
	if !colorClose(got.Gradient.Stops[0].Color, gfx.ColorFromRGBA8(128, 128, 0, 255), 2) {
		t.Fatalf("unexpected first stop color: %#v", got.Gradient.Stops[0].Color)
	}
}

func TestFillLerp_textureCrossFade(t *testing.T) {
	left := Fill{
		Type:    FillTexture,
		Texture: TextureFill{Ref: 1, Repeat: RepeatClamp},
		Opacity: 1,
	}
	right := Fill{
		Type:    FillTexture,
		Texture: TextureFill{Ref: 2, Repeat: RepeatTile},
		Opacity: 1,
	}
	got := left.Lerp(right, 0.5)
	if got.Opacity != 0.5 {
		t.Fatalf("expected cross-fade opacity 0.5, got %v", got.Opacity)
	}
}

func TestFillLerp_identityAcrossTypes(t *testing.T) {
	fills := []Fill{
		{Type: FillNone},
		{Type: FillSolid, Color: gfx.ColorFromRGBA8(4, 5, 6, 255), Opacity: 1},
		{
			Type: FillGradient,
			Gradient: Gradient{
				Type:  GradientLinear,
				Stops: []GradientStop{{Position: 0, Color: gfx.ColorFromRGBA8(1, 2, 3, 255)}},
			},
		},
		{Type: FillTexture, Texture: TextureFill{Ref: 7}, Opacity: 0.75},
		{Type: FillPattern, Pattern: PatternFill{Ref: 9, Scale: 2}},
	}
	for i, f := range fills {
		if got := f.Lerp(f, 0.37); !reflect.DeepEqual(got, f) {
			t.Fatalf("fill %d identity failed: got %#v want %#v", i, got, f)
		}
	}
}

func TestMaterialLerp_paddingAndOpacity(t *testing.T) {
	left := Material{
		Fills:   []Fill{{Type: FillSolid, Color: gfx.ColorFromRGBA8(255, 0, 0, 255), Opacity: 1}},
		Opacity: 1,
	}
	right := Material{
		Fills: []Fill{
			{Type: FillSolid, Color: gfx.ColorFromRGBA8(0, 0, 255, 255), Opacity: 1},
			{Type: FillSolid, Color: gfx.ColorFromRGBA8(0, 255, 0, 255), Opacity: 1},
		},
		Opacity: 0,
	}
	got := left.Lerp(right, 0.5)
	if len(got.Fills) != 2 {
		t.Fatalf("expected padded fill count 2, got %d", len(got.Fills))
	}
	if got.Opacity != 0.5 {
		t.Fatalf("unexpected opacity: %v", got.Opacity)
	}
}

func TestMaterialLerp_identity(t *testing.T) {
	m := Material{
		Fills: []Fill{
			{Type: FillSolid, Color: gfx.ColorFromRGBA8(8, 9, 10, 255), Opacity: 1},
		},
		Strokes: []MaterialStroke{
			{
				Paint:      Fill{Type: FillSolid, Color: gfx.ColorFromRGBA8(1, 2, 3, 255), Opacity: 1},
				Width:      2,
				BlurRadius: 4,
				Offset:     gfx.Point{X: 1, Y: 2},
			},
		},
		Opacity: 0.8,
	}
	if got := m.Lerp(m, 0.42); !reflect.DeepEqual(got, m) {
		t.Fatalf("identity failed: got %#v want %#v", got, m)
	}
}

func TestMaterialRegistry_defineGetAndMustGet(t *testing.T) {
	reg := NewMaterialRegistry()
	a := FromToken(gfx.ColorFromRGBA8(1, 2, 3, 255))
	b := FromToken(gfx.ColorFromRGBA8(4, 5, 6, 255))

	reg.Define("card", a)
	if got, ok := reg.Get("card"); !ok || !reflect.DeepEqual(got, a) {
		t.Fatalf("unexpected registry get: %#v %v", got, ok)
	}

	reg.Define("card", b)
	if got, ok := reg.Get("card"); !ok || !reflect.DeepEqual(got, b) {
		t.Fatalf("registry should replace existing entry: %#v %v", got, ok)
	}

	if got, ok := reg.Get("missing"); ok || !reflect.DeepEqual(got, Material{}) {
		t.Fatalf("missing lookup should return zero,false, got %#v %v", got, ok)
	}

	assertPanicContains(t, func() { _ = reg.MustGet("missing") }, "missing")
	if got := reg.MustGet("card"); !reflect.DeepEqual(got, b) {
		t.Fatalf("MustGet returned wrong material: %#v", got)
	}
}

func TestValidateMaterial_limits(t *testing.T) {
	if err := ValidateMaterial(Material{Fills: []Fill{{}}, Strokes: []MaterialStroke{{}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateMaterial(Material{Fills: []Fill{{}, {}}, Strokes: []MaterialStroke{{}, {}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateMaterial(Material{Fills: []Fill{{}, {}, {}}, Strokes: []MaterialStroke{{}, {}, {}}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateMaterial(Material{Fills: []Fill{{}, {}, {}, {}}}); err == nil {
		t.Fatal("expected fill limit error")
	} else if !strings.Contains(err.Error(), "4") {
		t.Fatalf("error should mention count, got %v", err)
	}
	if err := ValidateMaterial(Material{Strokes: []MaterialStroke{{}, {}, {}, {}}}); err == nil {
		t.Fatal("expected stroke limit error")
	}
}

func TestElevationTokens_stub_replaced(t *testing.T) {
	e := DefaultTokens().Elevation
	if e.Level0.Width != 0 || e.Level0.BlurRadius != 0 {
		t.Fatalf("level0 should be zero, got %#v", e.Level0)
	}
	if !(e.Level1.BlurRadius < e.Level2.BlurRadius &&
		e.Level2.BlurRadius < e.Level3.BlurRadius &&
		e.Level3.BlurRadius < e.Level4.BlurRadius) {
		t.Fatalf("blur radii not increasing: %#v", e)
	}
	if !(e.Level1.Offset.Y > 0 && e.Level2.Offset.Y > 0 && e.Level3.Offset.Y > 0 && e.Level4.Offset.Y > 0) {
		t.Fatalf("elevation offsets should be downward: %#v", e)
	}
}

func assertPanicContains(t *testing.T, fn func(), want string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic containing %q", want)
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T", r)
		}
		if !strings.Contains(msg, want) {
			t.Fatalf("panic %q does not contain %q", msg, want)
		}
	}()
	fn()
}

func colorClose(a, b gfx.Color, tolerance uint8) bool {
	ar, ag, ab, aa := a.ToRGBA8()
	br, bg, bb, ba := b.ToRGBA8()
	return absByteDiff(ar, br) <= tolerance &&
		absByteDiff(ag, bg) <= tolerance &&
		absByteDiff(ab, bb) <= tolerance &&
		absByteDiff(aa, ba) <= tolerance
}

func absByteDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

func TestFillLerp_randomIdentity(t *testing.T) {
	f := Fill{Type: FillSolid, Color: gfx.ColorFromRGBA8(10, 20, 30, 255), Opacity: 1}
	for i := 0; i < 1000; i++ {
		if got := f.Lerp(f, float32(i)/999); !reflect.DeepEqual(got, f) {
			t.Fatalf("identity failed at %d: %#v", i, got)
		}
	}
}

func TestMaterialLerp_endpointIdentity(t *testing.T) {
	a := Material{Fills: []Fill{{Type: FillSolid, Color: gfx.ColorFromRGBA8(10, 10, 10, 255), Opacity: 1}}, Opacity: 1}
	b := Material{Fills: []Fill{{Type: FillSolid, Color: gfx.ColorFromRGBA8(20, 20, 20, 255), Opacity: 1}}, Opacity: 0}
	if got := a.Lerp(b, 0); !reflect.DeepEqual(got, a) {
		t.Fatalf("t=0 should return a, got %#v", got)
	}
	if got := a.Lerp(b, 1); !reflect.DeepEqual(got, b) {
		t.Fatalf("t=1 should return b, got %#v", got)
	}
}

func TestFillLerp_textureIdentity(t *testing.T) {
	f := Fill{Type: FillTexture, Texture: TextureFill{Ref: 3, Repeat: RepeatMirror}, Opacity: 0.8}
	if got := f.Lerp(f, 0.5); !reflect.DeepEqual(got, f) {
		t.Fatalf("texture identity failed: got %#v want %#v", got, f)
	}
}

func TestMaterialRegistry_nilSafe(t *testing.T) {
	var reg *MaterialRegistry
	reg.Define("x", Material{})
	if got, ok := reg.Get("x"); ok || !reflect.DeepEqual(got, Material{}) {
		t.Fatalf("nil registry get should be zero,false")
	}
}

func TestValidateMaterial_errorMessageCounts(t *testing.T) {
	err := ValidateMaterial(Material{
		Fills: []Fill{{}, {}, {}, {}},
	})
	if err == nil || !strings.Contains(err.Error(), "4") {
		t.Fatalf("expected count in error, got %v", err)
	}
}

func TestElevationTokens_offsetsPositive(t *testing.T) {
	e := DarkTokens().Elevation
	if e.Level1.Offset.Y <= 0 || e.Level4.Offset.Y <= 0 {
		t.Fatalf("dark elevations should have positive offsets: %#v", e)
	}
}

func TestFillLerp_opacityLinearForSameTexture(t *testing.T) {
	left := Fill{Type: FillTexture, Texture: TextureFill{Ref: 1}, Opacity: 0.2}
	right := Fill{Type: FillTexture, Texture: TextureFill{Ref: 1}, Opacity: 0.8}
	got := left.Lerp(right, 0.5)
	if math.Abs(float64(got.Opacity-0.5)) > 1e-6 {
		t.Fatalf("expected linear opacity blend for same texture, got %v", got.Opacity)
	}
}
