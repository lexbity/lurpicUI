package gesture

import (
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
)

// TapRecognizer recognizes discrete tap sequences.
type TapRecognizer struct {
	NumberOfTapsRequired    int
	NumberOfTouchesRequired int
	MaxMovementRadius       float32
	MaxTapInterval          time.Duration
	Signal                  signal.Signal[TapSignal]
	recognizerDelegateHolder

	id         RecognizerID
	sm         *StateMachine
	tracking   bool
	startPos   gfx.Point
	lastPos    gfx.Point
	startTime  time.Duration
	lastTapEnd time.Duration
	tapCount   int
	pending    *TapSignal
}

// LongPressRecognizer recognizes presses held beyond a minimum duration.
type LongPressRecognizer struct {
	MinimumPressDuration time.Duration
	MaxMovementRadius    float32
	Signal               signal.Signal[LongPressSignal]
	recognizerDelegateHolder

	id        RecognizerID
	sm        *StateMachine
	tracking  bool
	began     bool
	pressPos  gfx.Point
	pressTime time.Duration
	heldTime  time.Duration
	pending   *LongPressSignal
}

// ID returns the recognizer identity.
func (r *TapRecognizer) ID() RecognizerID { return r.ensureID() }

func (r *TapRecognizer) State() RecognizerState {
	if r == nil || r.sm == nil {
		return RecognizerPossible
	}
	return r.sm.State()
}

func (r *TapRecognizer) Delegate() RecognizerDelegate {
	if r == nil {
		return RecognizerDelegate{}
	}
	return r.recognizerDelegateHolder.Delegate
}

func (r *TapRecognizer) Reset() {
	if r == nil {
		return
	}
	if r.sm == nil {
		r.sm = NewStateMachine("TapRecognizer")
	}
	r.sm.Reset()
	r.tracking = false
	r.tapCount = 0
	r.pending = nil
}

func (r *TapRecognizer) TouchesBegan(touches []Touch, event InputEvent) {
	if r == nil || !r.acceptTouchCount(len(touches)) {
		return
	}
	if r.sm == nil {
		r.sm = NewStateMachine("TapRecognizer")
	}
	if r.tapCount > 0 && r.MaxTapInterval > 0 && event.Timestamp-r.lastTapEnd > r.MaxTapInterval {
		r.queuePendingTap()
		r.tapCount = 0
	}
	r.tracking = true
	r.startPos = touches[0].Position
	r.lastPos = touches[0].Position
	r.startTime = event.Timestamp
}

func (r *TapRecognizer) TouchesMoved(touches []Touch, event InputEvent) {
	if r == nil || !r.tracking || len(touches) == 0 {
		return
	}
	current := touches[0].Position
	if radiusExceeded(r.startPos, current, r.maxMovementRadius()) {
		r.pending = nil
		r.tapCount = 0
		r.tracking = false
		r.sm.Transition(RecognizerFailed)
		return
	}
	r.lastPos = current
}

func (r *TapRecognizer) TouchesEnded(touches []Touch, event InputEvent) {
	if r == nil || !r.tracking || len(touches) == 0 {
		return
	}
	current := touches[0].Position
	if radiusExceeded(r.startPos, current, r.maxMovementRadius()) {
		r.pending = nil
		r.tapCount = 0
		r.tracking = false
		r.sm.Transition(RecognizerFailed)
		return
	}
	r.tapCount++
	r.lastTapEnd = event.Timestamp
	r.lastPos = current
	r.tracking = false
	if r.requiredTaps() <= 1 || r.tapCount >= r.requiredTaps() {
		r.pending = &TapSignal{
			Position:  current,
			TapCount:  r.tapCount,
			Modifiers: event.Modifiers,
		}
		r.sm.Transition(RecognizerEnded)
		r.tapCount = 0
		return
	}
	r.sm.Transition(RecognizerPossible)
}

func (r *TapRecognizer) TouchesCancelled(touches []Touch, event InputEvent) {
	if r == nil {
		return
	}
	r.pending = nil
	r.tapCount = 0
	r.tracking = false
	if r.sm == nil {
		r.sm = NewStateMachine("TapRecognizer")
	}
	r.sm.Transition(RecognizerCancelled)
}

// QueueSignals queues the tap signal after a completed sequence.
func (r *TapRecognizer) QueueSignals(q SignalQueue, touches []Touch, event InputEvent) {
	if r == nil || q == nil {
		return
	}
	if r.pending == nil {
		return
	}
	sig := *r.pending
	r.pending = nil
	q.Enqueue(func() { r.Signal.Emit(sig) })
	if r.sm != nil {
		r.sm.Reset()
	}
}

func (r *TapRecognizer) ensureID() RecognizerID {
	if r == nil {
		return 0
	}
	if r.id == 0 {
		r.id = NewRecognizerID()
	}
	if r.sm == nil {
		r.sm = NewStateMachine("TapRecognizer")
	}
	return r.id
}

func (r *TapRecognizer) acceptTouchCount(count int) bool {
	want := r.requiredTapsTouches()
	return want == 0 || count == want
}

func (r *TapRecognizer) requiredTaps() int {
	if r.NumberOfTapsRequired <= 0 {
		return 1
	}
	return r.NumberOfTapsRequired
}

func (r *TapRecognizer) requiredTapsTouches() int {
	if r.NumberOfTouchesRequired <= 0 {
		return 1
	}
	return r.NumberOfTouchesRequired
}

func (r *TapRecognizer) maxMovementRadius() float32 {
	if r.MaxMovementRadius > 0 {
		return r.MaxMovementRadius
	}
	return 10
}

func (r *TapRecognizer) queuePendingTap() {
	if r.pending != nil {
		return
	}
	r.pending = &TapSignal{
		Position:  r.lastPos,
		TapCount:  1,
		Modifiers: 0,
	}
	r.sm.Transition(RecognizerEnded)
}

func (r *LongPressRecognizer) ID() RecognizerID { return r.ensureID() }

func (r *LongPressRecognizer) State() RecognizerState {
	if r == nil || r.sm == nil {
		return RecognizerPossible
	}
	return r.sm.State()
}

func (r *LongPressRecognizer) Delegate() RecognizerDelegate {
	if r == nil {
		return RecognizerDelegate{}
	}
	return r.recognizerDelegateHolder.Delegate
}

func (r *LongPressRecognizer) Reset() {
	if r == nil {
		return
	}
	if r.sm == nil {
		r.sm = NewStateMachine("LongPressRecognizer")
	}
	r.sm.Reset()
	r.tracking = false
	r.began = false
	r.heldTime = 0
	r.pending = nil
}

func (r *LongPressRecognizer) TouchesBegan(touches []Touch, event InputEvent) {
	if r == nil || len(touches) == 0 {
		return
	}
	if r.sm == nil {
		r.sm = NewStateMachine("LongPressRecognizer")
	}
	r.tracking = true
	r.began = false
	r.pressPos = touches[0].Position
	r.pressTime = event.Timestamp
	r.heldTime = 0
}

func (r *LongPressRecognizer) TouchesMoved(touches []Touch, event InputEvent) {
	if r == nil || !r.tracking || len(touches) == 0 {
		return
	}
	current := touches[0].Position
	if radiusExceeded(r.pressPos, current, r.maxMovementRadius()) {
		r.pending = nil
		r.tracking = false
		r.began = false
		r.sm.Transition(RecognizerFailed)
		return
	}
	if r.began {
		r.pending = &LongPressSignal{State: RecognizerChanged, Position: current}
		r.sm.Transition(RecognizerChanged)
	}
}

func (r *LongPressRecognizer) TouchesEnded(touches []Touch, event InputEvent) {
	if r == nil || !r.tracking || len(touches) == 0 {
		return
	}
	current := touches[0].Position
	if radiusExceeded(r.pressPos, current, r.maxMovementRadius()) {
		r.pending = nil
		r.tracking = false
		r.began = false
		r.sm.Transition(RecognizerFailed)
		return
	}
	if !r.began {
		if r.minimumDuration() == 0 || event.Timestamp-r.pressTime >= r.minimumDuration() {
			r.pending = &LongPressSignal{State: RecognizerBegan, Position: current}
			r.sm.Transition(RecognizerBegan)
			r.began = true
		} else {
			r.pending = nil
			r.tracking = false
			r.sm.Transition(RecognizerFailed)
			return
		}
	}
	r.pending = appendLongPressEnd(r.pending, current)
	r.tracking = false
	r.began = false
	r.sm.Transition(RecognizerEnded)
}

func (r *LongPressRecognizer) TouchesCancelled(touches []Touch, event InputEvent) {
	if r == nil {
		return
	}
	r.pending = nil
	r.tracking = false
	r.began = false
	if r.sm == nil {
		r.sm = NewStateMachine("LongPressRecognizer")
	}
	r.sm.Transition(RecognizerCancelled)
}

// Tick advances the long-press timer and may queue a Began signal.
func (r *LongPressRecognizer) Tick(dt time.Duration) bool {
	if r == nil || !r.tracking || r.began {
		return false
	}
	r.heldTime += dt
	if r.heldTime < r.minimumDuration() {
		return false
	}
	r.began = true
	r.pending = &LongPressSignal{State: RecognizerBegan, Position: r.pressPos}
	r.sm.Transition(RecognizerBegan)
	return true
}

func (r *LongPressRecognizer) QueueSignals(q SignalQueue, touches []Touch, event InputEvent) {
	if r == nil || q == nil || r.pending == nil {
		return
	}
	sig := *r.pending
	r.pending = nil
	q.Enqueue(func() { r.Signal.Emit(sig) })
	if sig.State == RecognizerEnded || sig.State == RecognizerCancelled || sig.State == RecognizerFailed {
		r.Reset()
	}
}

func (r *LongPressRecognizer) ensureID() RecognizerID {
	if r == nil {
		return 0
	}
	if r.id == 0 {
		r.id = NewRecognizerID()
	}
	if r.sm == nil {
		r.sm = NewStateMachine("LongPressRecognizer")
	}
	return r.id
}

func (r *LongPressRecognizer) maxMovementRadius() float32 {
	if r.MaxMovementRadius > 0 {
		return r.MaxMovementRadius
	}
	return 10
}

func (r *LongPressRecognizer) minimumDuration() time.Duration {
	if r.MinimumPressDuration > 0 {
		return r.MinimumPressDuration
	}
	return 500 * time.Millisecond
}

func appendLongPressEnd(sig *LongPressSignal, pos gfx.Point) *LongPressSignal {
	if sig == nil {
		return &LongPressSignal{State: RecognizerEnded, Position: pos}
	}
	sig.State = RecognizerEnded
	sig.Position = pos
	return sig
}
