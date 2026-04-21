package animation

import (
	"math"
	"reflect"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestTimeline_basicPlayback(t *testing.T) {
	ResetTimelineRegistryForTest()
	tl := NewTimeline(nil, TimelineConfig{Duration: 2})
	if got := tl.T.Get(); got != 0 {
		t.Fatalf("initial T = %v", got)
	}
	if got := tl.State.Get(); got != PlaybackStopped {
		t.Fatalf("initial state = %v", got)
	}

	tl.Play()
	if got := tl.State.Get(); got != PlaybackPlaying {
		t.Fatalf("play state = %v", got)
	}
	dispatchTimelines(time.Second)
	if got := tl.T.Get(); math.Abs(got-1) > 0.001 {
		t.Fatalf("played T = %v", got)
	}
	tl.Pause()
	dispatchTimelines(time.Second)
	if got := tl.T.Get(); math.Abs(got-1) > 0.001 {
		t.Fatalf("paused T changed = %v", got)
	}
	tl.Stop()
	if got := tl.State.Get(); got != PlaybackStopped {
		t.Fatalf("stop state = %v", got)
	}
	if got := tl.T.Get(); got != 0 {
		t.Fatalf("stop T = %v", got)
	}

	tl.Seek(1.5)
	if got := tl.T.Get(); math.Abs(got-1.5) > 0.001 {
		t.Fatalf("seek T = %v", got)
	}
}

func TestTimeline_loopNoneStopsAtDuration(t *testing.T) {
	ResetTimelineRegistryForTest()
	tl := NewTimeline(nil, TimelineConfig{Duration: 1, Loop: LoopNone})
	tl.Play()
	dispatchTimelines(1500 * time.Millisecond)
	if got := tl.T.Get(); math.Abs(got-1) > 0.001 {
		t.Fatalf("loop none T = %v", got)
	}
	if got := tl.State.Get(); got != PlaybackStopped {
		t.Fatalf("loop none state = %v", got)
	}
	dispatchTimelines(time.Second)
	if got := tl.T.Get(); math.Abs(got-1) > 0.001 {
		t.Fatalf("loop none advanced after stop = %v", got)
	}
}

func TestTimeline_loopRepeatWraps(t *testing.T) {
	ResetTimelineRegistryForTest()
	tl := NewTimeline(nil, TimelineConfig{Duration: 1, Loop: LoopRepeat})
	tl.Play()
	dispatchTimelines(1500 * time.Millisecond)
	if got := tl.T.Get(); math.Abs(got-0.5) > 0.001 {
		t.Fatalf("loop repeat T = %v", got)
	}
	dispatchTimelines(5 * time.Second)
	if got := tl.T.Get(); got < 0 || got > 1 {
		t.Fatalf("loop repeat out of bounds = %v", got)
	}
}

func TestTimeline_loopPingPongReverses(t *testing.T) {
	ResetTimelineRegistryForTest()
	tl := NewTimeline(nil, TimelineConfig{Duration: 1, Loop: LoopPingPong})
	tl.Play()
	dispatchTimelines(1500 * time.Millisecond)
	if got := tl.T.Get(); math.Abs(got-0.5) > 0.001 {
		t.Fatalf("pingpong T = %v", got)
	}
	dispatchTimelines(1500 * time.Millisecond)
	if got := tl.T.Get(); got < 0 || got > 1 {
		t.Fatalf("pingpong out of bounds = %v", got)
	}
}

func TestTimeline_speedControlsRate(t *testing.T) {
	ResetTimelineRegistryForTest()
	tl := NewTimeline(nil, TimelineConfig{Duration: 2, Speed: 2})
	tl.Play()
	dispatchTimelines(500 * time.Millisecond)
	if got := tl.T.Get(); math.Abs(got-1) > 0.001 {
		t.Fatalf("speed 2 T = %v", got)
	}
	tl.SetSpeed(0)
	dispatchTimelines(time.Second)
	if got := tl.T.Get(); math.Abs(got-1) > 0.001 {
		t.Fatalf("speed 0 T changed = %v", got)
	}
}

func TestKeyframeSequenceEvaluate(t *testing.T) {
	reg := DefaultEasingRegistry()
	seq := NewKeyframeSequence([]Keyframe[Float32]{
		{T: 0, Value: 0, Easing: "ease-out"},
		{T: 1, Value: 10, Easing: "linear"},
		{T: 2, Value: 20, Easing: "linear"},
	}, reg)

	if got := seq.Evaluate(-1); got != 0 {
		t.Fatalf("before first = %v", got)
	}
	if got := seq.Evaluate(2.5); got != 20 {
		t.Fatalf("after last = %v", got)
	}
	if got := seq.Evaluate(1); got != 10 {
		t.Fatalf("on keyframe = %v", got)
	}
	midLinear := NewKeyframeSequence([]Keyframe[Float32]{
		{T: 0, Value: 0, Easing: "linear"},
		{T: 1, Value: 10, Easing: "linear"},
	}, reg)
	if got := midLinear.Evaluate(0.5); math.Abs(float64(got-5)) > 0.001 {
		t.Fatalf("linear midpoint = %v", got)
	}
	eased := seq.Evaluate(0.75)
	if eased <= 7.5 {
		t.Fatalf("ease-out should be ahead of linear, got %v", eased)
	}
}

func TestKeyframeSequenceEmptyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	_ = NewKeyframeSequence[Float32](nil, nil).Evaluate(0)
}

func TestAsSourceAndAnimatedValueIntegration(t *testing.T) {
	ResetTimelineRegistryForTest()
	tl := NewTimeline(nil, TimelineConfig{Duration: 2, Loop: LoopNone})
	seq := NewKeyframeSequence([]Keyframe[Float32]{
		{T: 0, Value: 0, Easing: "linear"},
		{T: 1, Value: 10, Easing: "linear"},
		{T: 2, Value: 20, Easing: "linear"},
	}, nil)
	av := NewAnimatedValue(seq.AsSource(tl), TransitionSpec{Duration: 0}, nil)
	av.SnapToTarget()
	tl.Play()
	dispatchTimelines(500 * time.Millisecond)
	if !av.Tick(0) {
		t.Fatalf("expected zero-duration tick to snap")
	}
	if got := av.Current(); math.Abs(float64(got-5)) > 0.001 {
		t.Fatalf("current = %v", got)
	}
	dispatchTimelines(500 * time.Millisecond)
	if !av.Tick(0) {
		t.Fatalf("expected second snap")
	}
	if got := av.Current(); math.Abs(float64(got-10)) > 0.001 {
		t.Fatalf("current = %v", got)
	}
}

func TestTimelineStoreSignalsQueued(t *testing.T) {
	ResetTimelineRegistryForTest()
	var queued []func()
	store.SetSignalQueueHook(func(fn func()) {
		queued = append(queued, fn)
	})
	defer store.SetSignalQueueHook(nil)

	tl := NewTimeline(nil, TimelineConfig{Duration: 1})
	tl.Play()
	dispatchTimelines(100 * time.Millisecond)
	if len(queued) == 0 {
		t.Fatalf("expected queued store signal")
	}
	for _, fn := range queued {
		fn()
	}
}

func TestMultipleTimelinesIndependent(t *testing.T) {
	ResetTimelineRegistryForTest()
	a := NewTimeline(nil, TimelineConfig{Duration: 2})
	b := NewTimeline(nil, TimelineConfig{Duration: 2})
	a.Play()
	b.Play()
	a.SetSpeed(1)
	b.SetSpeed(2)
	dispatchTimelines(500 * time.Millisecond)
	if math.Abs(a.T.Get()-0.5) > 0.001 {
		t.Fatalf("timeline a = %v", a.T.Get())
	}
	if math.Abs(b.T.Get()-1) > 0.001 {
		t.Fatalf("timeline b = %v", b.T.Get())
	}
}

func TestThemeMaterialIntegration(t *testing.T) {
	ResetTimelineRegistryForTest()
	tl := NewTimeline(nil, TimelineConfig{Duration: 3, Loop: LoopNone})
	seq := NewKeyframeSequence([]Keyframe[theme.Material]{
		{T: 0, Value: theme.FromToken(gfx.ColorFromRGBA8(10, 20, 30, 255)), Easing: "linear"},
		{T: 1.5, Value: theme.FromToken(gfx.ColorFromRGBA8(60, 80, 100, 255)), Easing: "ease-out"},
		{T: 3, Value: theme.FromToken(gfx.ColorFromRGBA8(120, 140, 160, 255)), Easing: "linear"},
	}, DefaultEasingRegistry())
	av := NewAnimatedValue(seq.AsSource(tl), TransitionSpec{Duration: 0}, nil)
	av.SnapToTarget()
	tl.Play()
	for i := 0; i < 91; i++ {
		dispatchTimelines(33 * time.Millisecond)
		if !av.Tick(0) {
			t.Fatalf("expected zero-duration snap at frame %d", i)
		}
		if av.IsAnimating() {
			t.Fatalf("expected zero-duration animated value to stay snapped")
		}
		if got, want := av.Current(), seq.Evaluate(tl.T.Get()); !reflect.DeepEqual(got, want) {
			t.Fatalf("frame %d material mismatch", i)
		}
	}
	if got := tl.State.Get(); got != PlaybackStopped {
		t.Fatalf("expected stopped at end, got %v", got)
	}
}

func TestTimelineDisposeUnregisters(t *testing.T) {
	ResetTimelineRegistryForTest()
	tl := NewTimeline(nil, TimelineConfig{Duration: 1})
	tl.Play()
	dispatchTimelines(time.Second)
	if got := tl.T.Get(); math.Abs(got-1) > 0.001 {
		t.Fatalf("before dispose T = %v", got)
	}
	tl.Dispose()
	dispatchTimelines(time.Second)
	if got := tl.T.Get(); math.Abs(got-1) > 0.001 {
		t.Fatalf("after dispose T changed = %v", got)
	}
}
