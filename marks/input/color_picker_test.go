package input

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

func TestColorPickerSetColorSyncsHSV(t *testing.T) {
	picker := NewColorPicker("Palette")
	picker.SetColor(gfx.ColorFromRGBA8(255, 0, 0, 255))

	if !nearFloat64(picker.Hue, 0, 0.001) {
		t.Fatalf("Hue = %.6f, want 0", picker.Hue)
	}
	if !nearFloat32(picker.Saturation, 1, 0.001) {
		t.Fatalf("Saturation = %.6f, want 1", picker.Saturation)
	}
	if !nearFloat32(picker.Value, 1, 0.001) {
		t.Fatalf("Value = %.6f, want 1", picker.Value)
	}
	if got := picker.CurrentColor(); got != (gfx.ColorFromRGBA8(255, 0, 0, 255)) {
		t.Fatalf("CurrentColor = %#v, want red", got)
	}
}

func TestColorPickerPointerSelectsWheelAndTriangle(t *testing.T) {
	picker := NewColorPicker("Palette")
	var emitted []gfx.Color
	picker.ColorChanged.Subscribe(func(c gfx.Color) {
		emitted = append(emitted, c)
	})

	arrangeBounds := gfx.RectFromXYWH(0, 0, 200, 200)
	picker.layoutRole.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, arrangeBounds)

	wheelPoint := gfx.Point{X: 100, Y: 20}
	if handled := picker.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: wheelPoint, Button: platform.PointerLeft}); !handled {
		t.Fatal("expected wheel press to be handled")
	}
	if len(emitted) != 1 {
		t.Fatalf("emitted count = %d, want 1", len(emitted))
	}
	if !nearFloat64(picker.Hue, 3*math.Pi/2, 0.05) {
		t.Fatalf("Hue = %.6f, want near 3*pi/2 after wheel press", picker.Hue)
	}
	if !nearFloat32(picker.Saturation, 1, 0.001) || !nearFloat32(picker.Value, 1, 0.001) {
		t.Fatalf("wheel press should not change SV: s=%.3f v=%.3f", picker.Saturation, picker.Value)
	}

	whiteVertex := picker.cachedTriangleVerts[1]
	if handled := picker.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: whiteVertex, Button: platform.PointerLeft}); !handled {
		t.Fatal("expected triangle press to be handled")
	}
	if len(emitted) != 2 {
		t.Fatalf("emitted count = %d, want 2", len(emitted))
	}
	if picker.Value < 0.9 {
		t.Fatalf("Value = %.6f, want near 1 at white vertex", picker.Value)
	}
	if picker.Saturation > 0.1 {
		t.Fatalf("Saturation = %.6f, want near 0 at white vertex", picker.Saturation)
	}
}

func TestColorPickerBuildCommandsProducesGeometry(t *testing.T) {
	picker := NewColorPicker("Palette")
	picker.layoutRole.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(0, 0, 200, 200))
	cmds := picker.buildCommands(picker.layoutRole.ArrangedBounds, nil)
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
