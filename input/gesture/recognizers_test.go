package gesture

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

type captureQueue struct {
	items []func()
}

func (q *captureQueue) Enqueue(fn func()) {
	if fn != nil {
		q.items = append(q.items, fn)
	}
}

func (q *captureQueue) Drain() {
	for _, fn := range q.items {
		fn()
	}
	q.items = q.items[:0]
}

func TestTapRecognizer_singleTap(t *testing.T) {
	var rec TapRecognizer
	var got []TapSignal
	rec.Signal.Subscribe(func(sig TapSignal) { got = append(got, sig) })
	q := &captureQueue{}

	rec.TouchesBegan([]Touch{{Position: gfx.Point{X: 10, Y: 20}}}, InputEvent{})
	rec.TouchesEnded([]Touch{{Position: gfx.Point{X: 10, Y: 20}}}, InputEvent{})
	rec.QueueSignals(q, nil, InputEvent{})
	q.Drain()

	if len(got) != 1 || got[0].TapCount != 1 {
		t.Fatalf("got %#v", got)
	}
}

func TestTapRecognizer_movementFails(t *testing.T) {
	var rec TapRecognizer
	var got []TapSignal
	rec.Signal.Subscribe(func(sig TapSignal) { got = append(got, sig) })
	q := &captureQueue{}

	rec.TouchesBegan([]Touch{{Position: gfx.Point{X: 0, Y: 0}}}, InputEvent{})
	rec.TouchesMoved([]Touch{{Position: gfx.Point{X: 100, Y: 0}}}, InputEvent{})
	rec.TouchesEnded([]Touch{{Position: gfx.Point{X: 100, Y: 0}}}, InputEvent{})
	rec.QueueSignals(q, nil, InputEvent{})
	q.Drain()

	if len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestTapRecognizer_doubleTap(t *testing.T) {
	var rec TapRecognizer
	rec.NumberOfTapsRequired = 2
	rec.MaxTapInterval = 300 * time.Millisecond
	var got []TapSignal
	rec.Signal.Subscribe(func(sig TapSignal) { got = append(got, sig) })
	q := &captureQueue{}

	rec.TouchesBegan([]Touch{{Position: gfx.Point{X: 1, Y: 1}}}, InputEvent{Timestamp: 0})
	rec.TouchesEnded([]Touch{{Position: gfx.Point{X: 1, Y: 1}}}, InputEvent{Timestamp: 10 * time.Millisecond})
	rec.QueueSignals(q, nil, InputEvent{})
	q.Drain()
	rec.TouchesBegan([]Touch{{Position: gfx.Point{X: 1, Y: 1}}}, InputEvent{Timestamp: 100 * time.Millisecond})
	rec.TouchesEnded([]Touch{{Position: gfx.Point{X: 1, Y: 1}}}, InputEvent{Timestamp: 120 * time.Millisecond})
	rec.QueueSignals(q, nil, InputEvent{})
	q.Drain()

	if len(got) != 1 || got[0].TapCount != 2 {
		t.Fatalf("got %#v", got)
	}
}

func TestLongPressRecognizer_lifecycle(t *testing.T) {
	var rec LongPressRecognizer
	var got []LongPressSignal
	rec.MinimumPressDuration = 100 * time.Millisecond
	rec.Signal.Subscribe(func(sig LongPressSignal) { got = append(got, sig) })
	q := &captureQueue{}

	rec.TouchesBegan([]Touch{{Position: gfx.Point{X: 2, Y: 3}}}, InputEvent{Timestamp: 0})
	if rec.Tick(50 * time.Millisecond) {
		t.Fatalf("tick should not trigger yet")
	}
	if !rec.Tick(60 * time.Millisecond) {
		t.Fatalf("tick should trigger began")
	}
	rec.QueueSignals(q, nil, InputEvent{})
	q.Drain()
	rec.TouchesMoved([]Touch{{Position: gfx.Point{X: 3, Y: 4}}}, InputEvent{Timestamp: 120 * time.Millisecond})
	rec.QueueSignals(q, nil, InputEvent{})
	q.Drain()
	rec.TouchesEnded([]Touch{{Position: gfx.Point{X: 3, Y: 4}}}, InputEvent{Timestamp: 150 * time.Millisecond})
	rec.QueueSignals(q, nil, InputEvent{})
	q.Drain()

	if len(got) == 0 || got[0].State != RecognizerBegan {
		t.Fatalf("got %#v", got)
	}
}

func TestPanRecognizer_emitsMoveAndEnd(t *testing.T) {
	var rec PanRecognizer
	rec.MinimumTranslation = 10
	var got []PanSignal
	rec.Signal.Subscribe(func(sig PanSignal) { got = append(got, sig) })
	q := &captureQueue{}

	rec.TouchesBegan([]Touch{{Position: gfx.Point{X: 0, Y: 0}}}, InputEvent{Timestamp: 0})
	rec.TouchesMoved([]Touch{{Position: gfx.Point{X: 2, Y: 2}}}, InputEvent{Timestamp: 10 * time.Millisecond})
	rec.QueueSignals(q, nil, InputEvent{})
	q.Drain()
	rec.TouchesMoved([]Touch{{Position: gfx.Point{X: 20, Y: 0}}}, InputEvent{Timestamp: 20 * time.Millisecond})
	rec.QueueSignals(q, nil, InputEvent{})
	q.Drain()
	rec.TouchesEnded([]Touch{{Position: gfx.Point{X: 25, Y: 0}}}, InputEvent{Timestamp: 30 * time.Millisecond})
	rec.QueueSignals(q, nil, InputEvent{})
	q.Drain()

	if len(got) < 2 || got[0].State != RecognizerBegan || got[len(got)-1].State != RecognizerEnded {
		t.Fatalf("got %#v", got)
	}
}

func TestSwipeRecognizer_rightSwipe(t *testing.T) {
	var rec SwipeRecognizer
	var got []SwipeSignal
	rec.Signal.Subscribe(func(sig SwipeSignal) { got = append(got, sig) })
	q := &captureQueue{}

	rec.TouchesBegan([]Touch{{Position: gfx.Point{X: 0, Y: 0}}}, InputEvent{Timestamp: 0})
	rec.TouchesEnded([]Touch{{Position: gfx.Point{X: 100, Y: 0}}}, InputEvent{Timestamp: 100 * time.Millisecond})
	rec.QueueSignals(q, nil, InputEvent{})
	q.Drain()

	if len(got) != 1 || got[0].Direction != SwipeRight {
		t.Fatalf("got %#v", got)
	}
}

func TestPinchRecognizer_twoTouchAndScroll(t *testing.T) {
	var rec PinchRecognizer
	var got []PinchSignal
	rec.Signal.Subscribe(func(sig PinchSignal) { got = append(got, sig) })
	q := &captureQueue{}

	rec.TouchesBegan([]Touch{
		{Position: gfx.Point{X: 0, Y: 0}},
		{Position: gfx.Point{X: 10, Y: 0}},
	}, InputEvent{})
	rec.QueueSignals(q, nil, InputEvent{})
	q.Drain()
	rec.TouchesMoved([]Touch{
		{Position: gfx.Point{X: 0, Y: 0}},
		{Position: gfx.Point{X: 20, Y: 0}},
	}, InputEvent{})
	rec.QueueSignals(q, nil, InputEvent{})
	q.Drain()
	rec.TouchesEnded([]Touch{{Position: gfx.Point{X: 0, Y: 0}}}, InputEvent{})
	rec.QueueSignals(q, nil, InputEvent{})
	q.Drain()

	if len(got) == 0 || got[0].Scale != 1 {
		t.Fatalf("got %#v", got)
	}

	tree, root, _ := buildGestureTree()
	scroll := NewGesturePipeline()
	scroll.SetPlatformAdaptation(PlatformAdaptationDesktop)
	pr := &PinchRecognizer{}
	pr.Signal.Subscribe(func(sig PinchSignal) { got = append(got, sig) })
	scroll.Register(root.ID(), GestureRole{Recognizers: []Recognizer{pr}})
	scroll.Process(platform.EventScroll{
		Position:  gfx.Point{X: 5, Y: 5},
		DeltaY:    1,
		Modifiers: platform.ModControl,
	}, root.ID(), tree)
	scroll.DrainQueuedSignals()

	if len(got) < 2 || got[len(got)-1].Scale <= 1 {
		t.Fatalf("scroll pinch got %#v", got)
	}
}
