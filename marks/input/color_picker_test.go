package input

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

func defaultColorPickerStore() *store.ValueStore[gfx.Color] {
	return store.NewValueStore(gfx.Color{R: 1, G: 0, B: 0, A: 1}) // red, opaque
}

func TestColorPickerSetColorSyncsHSV(t *testing.T) {
	picker := NewColorPicker("Palette", defaultColorPickerStore())
	picker.SetColor(gfx.ColorFromRGBA8(255, 0, 0, 255))

	if !nearFloat64(picker.cachedHue, 0, 0.001) {
		t.Fatalf("Hue = %.6f, want 0", picker.cachedHue)
	}
	if !nearFloat32(picker.cachedSaturation, 1, 0.001) {
		t.Fatalf("Saturation = %.6f, want 1", picker.cachedSaturation)
	}
	if !nearFloat32(picker.cachedBrightness, 1, 0.001) {
		t.Fatalf("Brightness = %.6f, want 1", picker.cachedBrightness)
	}
	if got := picker.CurrentColor(); got != (gfx.ColorFromRGBA8(255, 0, 0, 255)) {
		t.Fatalf("CurrentColor = %#v, want red", got)
	}
}

func TestColorPickerPointerSelectsWheelAndTriangle(t *testing.T) {
	picker := NewColorPicker("Palette", defaultColorPickerStore())
	var emitted []gfx.Color
	picker.ColorChanged.Subscribe(func(c gfx.Color) {
		emitted = append(emitted, c)
	})

	arrangeBounds := gfx.RectFromXYWH(0, 0, 200, 200)
	picker.Layout.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, arrangeBounds)

	wheelPoint := gfx.Point{X: 100, Y: 20}
	if handled := picker.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: wheelPoint, Button: platform.PointerLeft}); !handled {
		t.Fatal("expected wheel press to be handled")
	}
	if len(emitted) != 1 {
		t.Fatalf("emitted count = %d, want 1", len(emitted))
	}
	if !nearFloat64(picker.cachedHue, 3*math.Pi/2, 0.05) {
		t.Fatalf("Hue = %.6f, want near 3*pi/2 after wheel press", picker.cachedHue)
	}
	if !nearFloat32(picker.cachedSaturation, 1, 0.001) || !nearFloat32(picker.cachedBrightness, 1, 0.001) {
		t.Fatalf("wheel press should not change SV: s=%.3f v=%.3f", picker.cachedSaturation, picker.cachedBrightness)
	}

	whiteVertex := picker.cachedTriangleVerts[1]
	if handled := picker.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: whiteVertex, Button: platform.PointerLeft}); !handled {
		t.Fatal("expected triangle press to be handled")
	}
	if len(emitted) != 2 {
		t.Fatalf("emitted count = %d, want 2", len(emitted))
	}
	if picker.cachedBrightness < 0.9 {
		t.Fatalf("Brightness = %.6f, want near 1 at white vertex", picker.cachedBrightness)
	}
	if picker.cachedSaturation > 0.1 {
		t.Fatalf("Saturation = %.6f, want near 0 at white vertex", picker.cachedSaturation)
	}
}

func TestColorPickerBuildCommandsProducesGeometry(t *testing.T) {
	picker := NewColorPicker("Palette", defaultColorPickerStore())
	picker.Layout.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(0, 0, 200, 200))
	cmds := picker.buildCommands(picker.Layout.ArrangedBounds, nil)
	if len(cmds) == 0 {
		t.Fatal("expected geometry commands")
	}
}

func nearFloat32(a, b, tol float32) bool {
	if a > b {
		return a-b <= tol
	}
	return b-a <= tol
}

func nearFloat64(a, b, tol float64) bool {
	if a > b {
		return a-b <= tol
	}
	return b-a <= tol
}

func TestColorPickerValueSurvivesDispose(t *testing.T) {
	contracttest.AssertValueSurvivesDispose[gfx.Color](
		t,
		func() *store.ValueStore[gfx.Color] { return store.NewValueStore(gfx.Color{}) },
		func(s *store.ValueStore[gfx.Color]) facet.FacetImpl {
			return NewColorPicker("test", s)
		},
		func(m facet.FacetImpl) {
			m.(*ColorPicker).SetColor(gfx.Color{R: 1, G: 0, B: 0, A: 1})
		},
	)
}
