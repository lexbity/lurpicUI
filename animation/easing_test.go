package animation

import (
	"math"
	"testing"
	"time"
)

type easingRecordingLogger struct {
	warnings []string
}

func (l *easingRecordingLogger) Warn(msg string, args ...any) {
	l.warnings = append(l.warnings, msg)
}

func TestEasingRegistryBasics(t *testing.T) {
	reg := DefaultEasingRegistry()
	for _, name := range []string{"linear", "standard", "decelerate", "accelerate", "ease-in", "ease-out", "ease-in-out", "spring", "bounce-out", "elastic-out"} {
		if _, ok := reg.Get(name); !ok {
			t.Fatalf("missing easing %q", name)
		}
	}
	want := func(t float32) float32 { return t * 2 }
	reg.Register("linear", want)
	if got := reg.MustGet("linear")(0.5); got != 1 {
		t.Fatalf("overwrite failed: %v", got)
	}
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	_ = reg.MustGet("missing")
}

func TestCubicBezierLinearAndEaseInOut(t *testing.T) {
	linear := CubicBezier(0, 0, 1, 1)
	for i := 0; i <= 20; i++ {
		tVal := float32(i) / 20
		if got := linear(tVal); math.Abs(float64(got-tVal)) > 0.002 {
			t.Fatalf("linear mismatch at %v: %v", tVal, got)
		}
	}

	eio := CubicBezier(0.4, 0, 0.6, 1)
	samples := []struct {
		t, want float32
	}{
		{0.0, 0},
		{0.1, 0.025},
		{0.25, 0.129},
		{0.5, 0.5},
		{0.75, 0.871},
		{0.9, 0.975},
		{1.0, 1},
	}
	for _, sample := range samples {
		got := eio(sample.t)
		if math.Abs(float64(got-sample.want)) > 0.02 {
			t.Fatalf("ease-in-out mismatch at %v: got %v want %v", sample.t, got, sample.want)
		}
	}
}

func TestEasingCurvesShape(t *testing.T) {
	linear := Linear()
	if linear(0.5) != 0.5 {
		t.Fatalf("linear midpoint = %v", linear(0.5))
	}
	easeIn := CubicBezier(0.4, 0, 1, 1)
	if easeIn(0.25) >= 0.25 {
		t.Fatalf("ease-in should start slowly, got %v", easeIn(0.25))
	}
	easeOut := CubicBezier(0, 0, 0.6, 1)
	if easeOut(0.75) <= 0.75 {
		t.Fatalf("ease-out should advance quickly, got %v", easeOut(0.75))
	}
	if dec := CubicBezier(0, 0, 0, 1)(0.2); dec <= 0.35 {
		t.Fatalf("decelerate should be ahead at 0.2, got %v", dec)
	}
	if acc := DefaultEasingRegistry().MustGet("accelerate")(0.8); acc >= 0.65 {
		t.Fatalf("accelerate should lag early, got %v", acc)
	}
}

func TestSpringCurves(t *testing.T) {
	crit := Spring(1.0, 7.0)
	for i := 0; i <= 100; i++ {
		v := crit(float32(i) / 100)
		if v > 1.001 {
			t.Fatalf("critical spring overshot at %d: %v", i, v)
		}
	}

	under := Spring(0.5, 7.0)
	maxV := float32(0)
	for i := 0; i <= 100; i++ {
		v := under(float32(i) / 100)
		if v > maxV {
			maxV = v
		}
	}
	if maxV <= 1.05 {
		t.Fatalf("underdamped spring did not overshoot enough: %v", maxV)
	}

	for _, damping := range []float32{0.3, 0.5, 0.7, 1.0} {
		for _, freq := range []float32{3, 5, 7, 10} {
			v := Spring(damping, freq)(1)
			if v < 0.999 || v > 1.001 {
				t.Fatalf("spring did not settle for damping=%v freq=%v: %v", damping, freq, v)
			}
		}
	}
}

func TestBounceAndElastic(t *testing.T) {
	bounce := BounceOut()
	peak := float32(0)
	dip := false
	for i := 0; i <= 100; i++ {
		v := bounce(float32(i) / 100)
		if v > peak {
			peak = v
		}
		if peak > 0.95 && v < peak {
			dip = true
		}
	}
	if peak < 0.999 {
		t.Fatalf("bounce never reached 1: %v", peak)
	}
	if !dip {
		t.Fatalf("bounce never dipped after a peak")
	}

	elastic := ElasticOut()
	overshot := false
	for i := 0; i <= 100; i++ {
		if v := elastic(float32(i) / 100); v > 1.0 {
			overshot = true
			break
		}
	}
	if !overshot {
		t.Fatalf("elastic never overshot")
	}
}

func TestAnimatedValueEasingAndFallback(t *testing.T) {
	logger := &easingRecordingLogger{}
	SetLogger(logger)
	defer SetLogger(nil)

	src := Float32(0)
	reg := DefaultEasingRegistry()
	avOut := NewAnimatedValue(func() Float32 { return src }, TransitionSpec{Duration: time.Second, Easing: "ease-out"}, reg)
	src = 1
	if !avOut.Tick(250 * time.Millisecond) {
		t.Fatalf("ease-out tick should change")
	}
	if avOut.Progress() <= 0.4 {
		t.Fatalf("ease-out progress should be ahead of linear, got %v", avOut.Progress())
	}

	src = 0
	avIn := NewAnimatedValue(func() Float32 { return src }, TransitionSpec{Duration: time.Second, Easing: "ease-in"}, reg)
	src = 1
	if !avIn.Tick(250 * time.Millisecond) {
		t.Fatalf("ease-in tick should change")
	}
	if avIn.Progress() >= 0.2 {
		t.Fatalf("ease-in progress should lag linear, got %v", avIn.Progress())
	}

	src = 0
	avUnknown := NewAnimatedValue(func() Float32 { return src }, TransitionSpec{Duration: time.Second, Easing: "not-a-real-easing"}, reg)
	src = 1
	if !avUnknown.Tick(500 * time.Millisecond) {
		t.Fatalf("fallback tick should change")
	}
	if avUnknown.Progress() != 0.5 {
		t.Fatalf("fallback should be linear, got %v", avUnknown.Progress())
	}
	if len(logger.warnings) == 0 {
		t.Fatalf("expected warning for unknown easing")
	}
}
