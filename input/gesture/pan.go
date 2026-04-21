package gesture

import (
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
)

// PanRecognizer recognizes drag-like movement.
type PanRecognizer struct {
	MinimumTranslation float32
	MinimumTouches     int
	MaximumTouches     int
	Signal             signal.Signal[PanSignal]
	recognizerDelegateHolder

	id         RecognizerID
	sm         *StateMachine
	tracking   bool
	began      bool
	startPos   gfx.Point
	lastPos    gfx.Point
	startTime  time.Duration
	lastTime   time.Duration
	totalDelta gfx.Point
	velocity   gfx.Point
	pending    *PanSignal
}

func (r *PanRecognizer) ID() RecognizerID { return r.ensureID() }

func (r *PanRecognizer) State() RecognizerState {
	if r == nil || r.sm == nil {
		return RecognizerPossible
	}
	return r.sm.State()
}

func (r *PanRecognizer) Delegate() RecognizerDelegate {
	if r == nil {
		return RecognizerDelegate{}
	}
	return r.recognizerDelegateHolder.Delegate
}

func (r *PanRecognizer) Reset() {
	if r == nil {
		return
	}
	if r.sm == nil {
		r.sm = NewStateMachine("PanRecognizer")
	}
	r.sm.Reset()
	r.tracking = false
	r.began = false
	r.totalDelta = gfx.Point{}
	r.velocity = gfx.Point{}
	r.pending = nil
}

func (r *PanRecognizer) TouchesBegan(touches []Touch, event InputEvent) {
	if r == nil || len(touches) == 0 {
		return
	}
	if r.sm == nil {
		r.sm = NewStateMachine("PanRecognizer")
	}
	r.tracking = true
	r.began = false
	r.startPos = touches[0].Position
	r.lastPos = touches[0].Position
	r.startTime = event.Timestamp
	r.lastTime = event.Timestamp
	r.totalDelta = gfx.Point{}
	r.velocity = gfx.Point{}
}

func (r *PanRecognizer) TouchesMoved(touches []Touch, event InputEvent) {
	if r == nil || !r.tracking || len(touches) == 0 {
		return
	}
	current := touches[0].Position
	delta := gfx.Point{X: current.X - r.lastPos.X, Y: current.Y - r.lastPos.Y}
	total := gfx.Point{X: current.X - r.startPos.X, Y: current.Y - r.startPos.Y}
	r.updateVelocity(delta, event.Timestamp)
	r.totalDelta = total
	r.lastPos = current
	r.lastTime = event.Timestamp
	if !r.began {
		if magnitude(total) < r.minimumTranslation() {
			return
		}
		r.began = true
		r.pending = &PanSignal{
			State:      RecognizerBegan,
			Position:   current,
			Delta:      delta,
			TotalDelta: total,
			Velocity:   r.velocity,
		}
		r.sm.Transition(RecognizerBegan)
		return
	}
	r.pending = &PanSignal{
		State:      RecognizerChanged,
		Position:   current,
		Delta:      delta,
		TotalDelta: total,
		Velocity:   r.velocity,
	}
	r.sm.Transition(RecognizerChanged)
}

func (r *PanRecognizer) TouchesEnded(touches []Touch, event InputEvent) {
	if r == nil || !r.tracking || len(touches) == 0 {
		return
	}
	current := touches[0].Position
	delta := gfx.Point{X: current.X - r.lastPos.X, Y: current.Y - r.lastPos.Y}
	total := gfx.Point{X: current.X - r.startPos.X, Y: current.Y - r.startPos.Y}
	r.updateVelocity(delta, event.Timestamp)
	r.totalDelta = total
	r.lastPos = current
	r.lastTime = event.Timestamp
	if !r.began && magnitude(total) < r.minimumTranslation() {
		r.pending = nil
		r.tracking = false
		r.sm.Transition(RecognizerFailed)
		return
	}
	r.pending = &PanSignal{
		State:      RecognizerEnded,
		Position:   current,
		Delta:      delta,
		TotalDelta: total,
		Velocity:   r.velocity,
	}
	r.tracking = false
	r.began = false
	r.sm.Transition(RecognizerEnded)
}

func (r *PanRecognizer) TouchesCancelled(touches []Touch, event InputEvent) {
	if r == nil {
		return
	}
	r.pending = nil
	r.tracking = false
	r.began = false
	if r.sm == nil {
		r.sm = NewStateMachine("PanRecognizer")
	}
	r.sm.Transition(RecognizerCancelled)
}

func (r *PanRecognizer) QueueSignals(q SignalQueue, touches []Touch, event InputEvent) {
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

func (r *PanRecognizer) ensureID() RecognizerID {
	if r == nil {
		return 0
	}
	if r.id == 0 {
		r.id = NewRecognizerID()
	}
	if r.sm == nil {
		r.sm = NewStateMachine("PanRecognizer")
	}
	return r.id
}

func (r *PanRecognizer) minimumTranslation() float32 {
	if r.MinimumTranslation > 0 {
		return r.MinimumTranslation
	}
	return 10
}

func (r *PanRecognizer) updateVelocity(delta gfx.Point, now time.Duration) {
	if r.lastTime == 0 {
		return
	}
	dt := float32(now-r.lastTime) / float32(time.Second)
	if dt <= 0 {
		return
	}
	instant := gfx.Point{X: delta.X / dt, Y: delta.Y / dt}
	alpha := emaAlpha(time.Duration(float64(now-r.lastTime)), 100*time.Millisecond)
	r.velocity.X = r.velocity.X*(1-alpha) + instant.X*alpha
	r.velocity.Y = r.velocity.Y*(1-alpha) + instant.Y*alpha
}
