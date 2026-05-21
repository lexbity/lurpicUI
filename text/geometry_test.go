package text

import "testing"

type testRect struct {
	Min testPoint
	Max testPoint
}

type testPoint struct {
	X float32
	Y float32
}

func (r testRect) Width() float32  { return r.Max.X - r.Min.X }
func (r testRect) Height() float32 { return r.Max.Y - r.Min.Y }

func TestWidthAndHeightHandleNil(t *testing.T) {
	if got := Width(nil); got != 0 {
		t.Fatalf("Width(nil) = %v, want 0", got)
	}
	if got := Height(nil); got != 0 {
		t.Fatalf("Height(nil) = %v, want 0", got)
	}
}

func TestMaxWidthAndHeightIgnoreNilLayouts(t *testing.T) {
	a := &TextLayout{Bounds: RectFromXYWH(0, 0, 10, 4)}
	b := &TextLayout{Bounds: RectFromXYWH(0, 0, 3, 7)}
	if got := MaxWidth(nil, a, b); got != 10 {
		t.Fatalf("MaxWidth = %v, want 10", got)
	}
	if got := MaxHeight(nil, a, b); got != 7 {
		t.Fatalf("MaxHeight = %v, want 7", got)
	}
}

func TestCenterHelpersWorkWithGfxRects(t *testing.T) {
	bounds := testRect{Min: testPoint{X: 10, Y: 20}, Max: testPoint{X: 90, Y: 60}}
	if got := CenterY(bounds, 10); got != 35 {
		t.Fatalf("CenterY = %v, want 35", got)
	}

	centered := CenterRect(bounds, 20, 10)
	if centered.Min.X != 40 || centered.Min.Y != 35 {
		t.Fatalf("CenterRect = %#v, want min (40,35)", centered)
	}
	if centered.Width() != 20 || centered.Height() != 10 {
		t.Fatalf("CenterRect size = (%v,%v), want (20,10)", centered.Width(), centered.Height())
	}

	aligned := AlignRectY(testRect{Min: testPoint{X: 2, Y: 4}, Max: testPoint{X: 12, Y: 10}}, 20, 30)
	if aligned.Min.Y != 32 || aligned.Max.Y != 38 {
		t.Fatalf("AlignRectY = %#v, want vertical span 32..38", aligned)
	}
}
