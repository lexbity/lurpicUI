package gesture

import (
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
)

// SwipeRecognizer recognizes fast directional flicks.
type SwipeRecognizer struct {
	Direction          SwipeDirectionMask
	MinimumVelocity    float32
	MinimumTranslation float32
	Signal             signal.Signal[SwipeSignal]
	recognizerDelegateHolder

	id        RecognizerID
	sm        *StateMachine
	tracking  bool
	startPos  gfx.Point
	lastPos   gfx.Point
	startTime time.Duration
	lastTime  time.Duration
	pending   *SwipeSignal
}

func (r *SwipeRecognizer) ID() RecognizerID { return r.ensureID() }

func (r *SwipeRecognizer) State() RecognizerState {
	if r == nil || r.sm == nil {
		return RecognizerPossible
	}
	return r.sm.State()
}

func (r *SwipeRecognizer) Delegate() RecognizerDelegate {
	if r == nil {
		return RecognizerDelegate{}
	}
	return r.recognizerDelegateHolder.Delegate
}

func (r *SwipeRecognizer) Reset() {
	if r == nil {
		return
	}
	if r.sm == nil {
		r.sm = NewStateMachine("SwipeRecognizer")
	}
	r.sm.Reset()
	r.tracking = false
	r.pending = nil
}

func (r *SwipeRecognizer) TouchesBegan(touches []Touch, event InputEvent) {
	if r == nil || len(touches) == 0 {
		return
	}
	if r.sm == nil {
		r.sm = NewStateMachine("SwipeRecognizer")
	}
	r.tracking = true
	r.startPos = touches[0].Position
	r.lastPos = touches[0].Position
	r.startTime = event.Timestamp
	r.lastTime = event.Timestamp
}

func (r *SwipeRecognizer) TouchesMoved(touches []Touch, event InputEvent) {
	if r == nil || !r.tracking || len(touches) == 0 {
		return
	}
	r.lastPos = touches[0].Position
	r.lastTime = event.Timestamp
}

func (r *SwipeRecognizer) TouchesEnded(touches []Touch, event InputEvent) {
	if r == nil || !r.tracking || len(touches) == 0 {
		return
	}
	current := touches[0].Position
	delta := gfx.Point{X: current.X - r.startPos.X, Y: current.Y - r.startPos.Y}
	dist := magnitude(delta)
	elapsed := event.Timestamp - r.startTime
	velocity := float32(0)
	if elapsed > 0 {
		velocity = dist / float32(elapsed.Seconds())
	}
	dir, ok := swipeDirection(delta)
	if !ok || dist < r.minimumTranslation() || velocity < r.minimumVelocity() || !r.directionAllowed(dir) {
		r.pending = nil
		r.tracking = false
		r.sm.Transition(RecognizerFailed)
		return
	}
	r.pending = &SwipeSignal{Direction: dir, Velocity: velocity}
	r.tracking = false
	r.sm.Transition(RecognizerEnded)
}

func (r *SwipeRecognizer) TouchesCancelled(touches []Touch, event InputEvent) {
	if r == nil {
		return
	}
	r.pending = nil
	r.tracking = false
	if r.sm == nil {
		r.sm = NewStateMachine("SwipeRecognizer")
	}
	r.sm.Transition(RecognizerCancelled)
}

func (r *SwipeRecognizer) QueueSignals(q SignalQueue, touches []Touch, event InputEvent) {
	if r == nil || q == nil || r.pending == nil {
		return
	}
	sig := *r.pending
	r.pending = nil
	q.Enqueue(func() { r.Signal.Emit(sig) })
	if r.sm != nil {
		r.sm.Reset()
	}
}

func (r *SwipeRecognizer) ensureID() RecognizerID {
	if r == nil {
		return 0
	}
	if r.id == 0 {
		r.id = NewRecognizerID()
	}
	if r.sm == nil {
		r.sm = NewStateMachine("SwipeRecognizer")
	}
	return r.id
}

func (r *SwipeRecognizer) minimumTranslation() float32 {
	if r.MinimumTranslation > 0 {
		return r.MinimumTranslation
	}
	return 50
}

func (r *SwipeRecognizer) minimumVelocity() float32 {
	if r.MinimumVelocity > 0 {
		return r.MinimumVelocity
	}
	return 200
}

func (r *SwipeRecognizer) directionAllowed(dir SwipeDirection) bool {
	mask := r.Direction
	if mask == 0 {
		mask = SwipeMaskAny
	}
	switch dir {
	case SwipeLeft:
		return mask&SwipeMaskLeft != 0
	case SwipeRight:
		return mask&SwipeMaskRight != 0
	case SwipeUp:
		return mask&SwipeMaskUp != 0
	case SwipeDown:
		return mask&SwipeMaskDown != 0
	default:
		return false
	}
}
