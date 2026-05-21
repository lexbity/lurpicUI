package layout

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestArrangeVerticalFlow(t *testing.T) {
	bounds := gfx.RectFromXYWH(0, 0, 120, 80)
	rects := ArrangeVerticalFlow(bounds, 0, 8, []gfx.Size{
		{W: 120, H: 20},
		{W: 40, H: 16},
	}, false)
	if got, want := rects[0].Min.Y, float32(0); got != want {
		t.Fatalf("label y = %v, want %v", got, want)
	}
	if got, want := rects[1].Min.X, float32(40); got != want {
		t.Fatalf("control x = %v, want %v", got, want)
	}
}
