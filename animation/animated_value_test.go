package animation

import (
	"reflect"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
)

var (
	_ Interpolatable[gfx.Color]      = gfx.Color{}
	_ Interpolatable[gfx.Point]      = gfx.Point{}
	_ Interpolatable[gfx.Rect]       = gfx.Rect{}
	_ Interpolatable[gfx.Size]       = gfx.Size{}
	_ Interpolatable[Float32]        = Float32(0)
	_ Interpolatable[theme.Fill]     = theme.Fill{}
	_ Interpolatable[theme.Material] = theme.Material{}
)

func TestLerpMethods(t *testing.T) {
	if got := (gfx.Color{R: 1, G: 0, B: 0, A: 1}).Lerp(gfx.Color{R: 0, G: 1, B: 0, A: 1}, 0.5); got.R != 0.5 || got.G != 0.5 {
		t.Fatalf("color midpoint = %#v", got)
	}
	if got := (gfx.Point{X: 2, Y: 4}).Lerp(gfx.Point{X: 6, Y: 10}, 0.5); got != (gfx.Point{X: 4, Y: 7}) {
		t.Fatalf("point midpoint = %#v", got)
	}
	if got := (gfx.Rect{Min: gfx.Point{X: 0, Y: 0}, Max: gfx.Point{X: 10, Y: 10}}).Lerp(gfx.Rect{Min: gfx.Point{X: 10, Y: 20}, Max: gfx.Point{X: 30, Y: 40}}, 0.5); got.Min != (gfx.Point{X: 5, Y: 10}) || got.Max != (gfx.Point{X: 20, Y: 25}) {
		t.Fatalf("rect midpoint = %#v", got)
	}
	if got := (gfx.Size{W: 4, H: 8}).Lerp(gfx.Size{W: 10, H: 20}, 0.5); got != (gfx.Size{W: 7, H: 14}) {
		t.Fatalf("size midpoint = %#v", got)
	}
	if got := Float32(2).Lerp(Float32(6), 0.5); got != 4 {
		t.Fatalf("float midpoint = %v", got)
	}
}

func TestAnimatedValueLifecycle(t *testing.T) {
	src := Float32(10)
	av := NewAnimatedValue(func() Float32 { return src }, TransitionSpec{Duration: time.Second}, nil)
	if av.Current() != 10 || av.Target() != 10 || av.Progress() != 1 || av.IsAnimating() {
		t.Fatalf("initial state = current:%v target:%v progress:%v animating:%v", av.Current(), av.Target(), av.Progress(), av.IsAnimating())
	}

	src = 20
	if av.Target() != 10 {
		t.Fatalf("target should stay snapped until tick, got %v", av.Target())
	}
	if changed := av.Tick(0); changed {
		t.Fatalf("zero dt should not change current")
	}
	if av.Target() != 20 || !av.IsAnimating() || av.Current() != 10 {
		t.Fatalf("after retarget = current:%v target:%v animating:%v", av.Current(), av.Target(), av.IsAnimating())
	}
	if !av.Tick(500 * time.Millisecond) {
		t.Fatalf("tick should change current")
	}
	if av.Current() != 15 {
		t.Fatalf("halfway current = %v", av.Current())
	}

	src = 30
	if av.Tick(0) {
		t.Fatalf("retarget at zero dt should not move current")
	}
	if av.Current() != 15 || av.Target() != 30 {
		t.Fatalf("retarget preserved current=%v target=%v", av.Current(), av.Target())
	}
	if !av.Tick(500 * time.Millisecond) {
		t.Fatalf("retarget tick should change current")
	}
	if av.Current() <= 15 || av.Current() >= 30 {
		t.Fatalf("retarget current = %v", av.Current())
	}

	av.SnapToTarget()
	if av.Current() != 30 || av.Target() != 30 || av.IsAnimating() {
		t.Fatalf("snap state = current:%v target:%v animating:%v", av.Current(), av.Target(), av.IsAnimating())
	}
}

func TestAnimatedValueZeroDuration(t *testing.T) {
	src := Float32(0)
	av := NewAnimatedValue(func() Float32 { return src }, TransitionSpec{Duration: 0}, nil)
	src = 5
	if !av.Tick(0) {
		t.Fatalf("zero-duration transition should snap")
	}
	if av.Current() != 5 || av.Target() != 5 || av.IsAnimating() {
		t.Fatalf("zero-duration state = current:%v target:%v animating:%v", av.Current(), av.Target(), av.IsAnimating())
	}
	if av.Tick(0) {
		t.Fatalf("second tick should be false")
	}
}

func TestAnimatedValueDelayAndSpec(t *testing.T) {
	src := Float32(0)
	av := NewAnimatedValue(func() Float32 { return src }, TransitionSpec{Duration: 300 * time.Millisecond, Delay: 200 * time.Millisecond}, nil)
	src = 10
	if av.Tick(100 * time.Millisecond) {
		t.Fatalf("delay should block change")
	}
	if av.Current() != 0 || av.Target() != 10 {
		t.Fatalf("delay state = current:%v target:%v", av.Current(), av.Target())
	}
	if av.Tick(100 * time.Millisecond) {
		t.Fatalf("delay boundary should still be blocked")
	}
	if !av.Tick(50 * time.Millisecond) {
		t.Fatalf("post-delay tick should change current")
	}

	av.SetSpec(TransitionSpec{Duration: 0})
	src = 20
	if !av.Tick(0) {
		t.Fatalf("new spec should apply to future transition")
	}
	if av.Current() != 20 || av.Target() != 20 || av.IsAnimating() {
		t.Fatalf("updated spec state = current:%v target:%v animating:%v", av.Current(), av.Target(), av.IsAnimating())
	}
}

func TestAnimatedValueStableWithUnchangedSource(t *testing.T) {
	src := Float32(1)
	av := NewAnimatedValue(func() Float32 { return src }, TransitionSpec{Duration: time.Second}, nil)
	for i := 0; i < 10000; i++ {
		if av.Tick(16 * time.Millisecond) {
			t.Fatalf("unchanged source should not tick dirty at iteration %d", i)
		}
	}
}

func TestAnimatedValueMaterialAndColor(t *testing.T) {
	colorSrc := gfx.Color{R: 1, G: 0, B: 0, A: 1}
	colorAv := NewAnimatedValue(func() gfx.Color { return colorSrc }, TransitionSpec{Duration: time.Second}, nil)
	colorSrc = gfx.Color{R: 0, G: 1, B: 0, A: 1}
	if !colorAv.Tick(500 * time.Millisecond) {
		t.Fatalf("color tick should change")
	}
	if colorAv.Current().R != 0.5 || colorAv.Current().G != 0.5 {
		t.Fatalf("color current = %#v", colorAv.Current())
	}

	matSrc := theme.SolidMaterial(gfx.Color{R: 1, A: 1}, gfx.Color{}, 0)
	matAv := NewAnimatedValue(func() theme.Material { return matSrc }, TransitionSpec{Duration: time.Second}, nil)
	matSrc = theme.SolidMaterial(gfx.Color{G: 1, A: 1}, gfx.Color{}, 0)
	if !matAv.Tick(500 * time.Millisecond) {
		t.Fatalf("material tick should change")
	}
	if reflect.DeepEqual(matAv.Current(), matAv.Target()) {
		t.Fatalf("material should still be mid-transition")
	}
}
