package gesture

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
)

// PinchRecognizer recognizes two-touch scaling gestures.
type PinchRecognizer struct {
	Signal signal.Signal[PinchSignal]
	recognizerDelegateHolder

	id            RecognizerID
	sm            *StateMachine
	active        bool
	startDistance float32
	lastScale     float32
	startCenter   gfx.Point
	lastCenter    gfx.Point
	pending       *PinchSignal
}

func (r *PinchRecognizer) ID() RecognizerID { return r.ensureID() }

func (r *PinchRecognizer) State() RecognizerState {
	if r == nil || r.sm == nil {
		return RecognizerPossible
	}
	return r.sm.State()
}

func (r *PinchRecognizer) Delegate() RecognizerDelegate {
	if r == nil {
		return RecognizerDelegate{}
	}
	return r.recognizerDelegateHolder.Delegate
}

func (r *PinchRecognizer) Reset() {
	if r == nil {
		return
	}
	if r.sm == nil {
		r.sm = NewStateMachine("PinchRecognizer")
	}
	r.sm.Reset()
	r.active = false
	r.pending = nil
	r.startDistance = 0
	r.lastScale = 1
}

func (r *PinchRecognizer) TouchesBegan(touches []Touch, event InputEvent) {
	if r == nil || len(touches) != 2 {
		return
	}
	if r.sm == nil {
		r.sm = NewStateMachine("PinchRecognizer")
	}
	r.active = true
	r.startDistance = max(distance(touches[0].Position, touches[1].Position), 1)
	r.lastScale = 1
	r.startCenter = midpoint(touches[0].Position, touches[1].Position)
	r.lastCenter = r.startCenter
	r.pending = &PinchSignal{
		State:  RecognizerBegan,
		Scale:  1,
		Delta:  0,
		Center: r.startCenter,
	}
	r.sm.Transition(RecognizerBegan)
}

func (r *PinchRecognizer) TouchesMoved(touches []Touch, event InputEvent) {
	if r == nil || !r.active {
		return
	}
	if len(touches) != 2 {
		r.pending = &PinchSignal{State: RecognizerCancelled, Scale: r.lastScale, Center: r.lastCenter}
		r.active = false
		r.sm.Transition(RecognizerCancelled)
		return
	}
	currentDistance := distance(touches[0].Position, touches[1].Position)
	if currentDistance <= 0 {
		currentDistance = 1
	}
	scale := currentDistance / r.startDistance
	center := midpoint(touches[0].Position, touches[1].Position)
	r.pending = &PinchSignal{
		State:    RecognizerChanged,
		Scale:    scale,
		Delta:    scale - r.lastScale,
		Center:   center,
		Velocity: 0,
	}
	r.lastScale = scale
	r.lastCenter = center
	r.sm.Transition(RecognizerChanged)
}

func (r *PinchRecognizer) TouchesEnded(touches []Touch, event InputEvent) {
	if r == nil || !r.active {
		return
	}
	if len(touches) != 2 {
		r.pending = &PinchSignal{State: RecognizerCancelled, Scale: r.lastScale, Center: r.lastCenter}
		r.active = false
		r.sm.Transition(RecognizerCancelled)
		return
	}
	currentDistance := distance(touches[0].Position, touches[1].Position)
	if currentDistance <= 0 {
		currentDistance = 1
	}
	scale := currentDistance / r.startDistance
	center := midpoint(touches[0].Position, touches[1].Position)
	r.pending = &PinchSignal{
		State:    RecognizerEnded,
		Scale:    scale,
		Delta:    scale - r.lastScale,
		Center:   center,
		Velocity: 0,
	}
	r.active = false
	r.lastScale = scale
	r.lastCenter = center
	r.sm.Transition(RecognizerEnded)
}

func (r *PinchRecognizer) TouchesCancelled(touches []Touch, event InputEvent) {
	if r == nil {
		return
	}
	r.pending = &PinchSignal{State: RecognizerCancelled, Scale: r.lastScale, Center: r.lastCenter}
	r.active = false
	if r.sm == nil {
		r.sm = NewStateMachine("PinchRecognizer")
	}
	r.sm.Transition(RecognizerCancelled)
}

func (r *PinchRecognizer) QueueSignals(q SignalQueue, touches []Touch, event InputEvent) {
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

func (r *PinchRecognizer) ScrollGesture(e platform.EventScroll, event InputEvent) {
	if r == nil {
		return
	}
	if e.Modifiers&platform.ModControl == 0 {
		return
	}
	if r.sm == nil {
		r.sm = NewStateMachine("PinchRecognizer")
	}
	scaleDelta := 1 + e.DeltaY*0.1
	if scaleDelta <= 0 {
		scaleDelta = 0.1
	}
	if !r.active {
		r.active = true
		r.lastScale = 1
		r.startDistance = 1
		r.startCenter = e.Position
		r.lastCenter = e.Position
		r.pending = &PinchSignal{
			State:    RecognizerBegan,
			Scale:    scaleDelta,
			Delta:    scaleDelta - 1,
			Center:   e.Position,
			Velocity: 0,
		}
		r.lastScale = scaleDelta
		r.sm.Transition(RecognizerBegan)
		return
	}
	r.pending = &PinchSignal{
		State:    RecognizerChanged,
		Scale:    r.lastScale * scaleDelta,
		Delta:    scaleDelta - 1,
		Center:   e.Position,
		Velocity: 0,
	}
	r.lastScale = r.pending.Scale
	r.lastCenter = e.Position
	r.sm.Transition(RecognizerChanged)
}

func (r *PinchRecognizer) ensureID() RecognizerID {
	if r == nil {
		return 0
	}
	if r.id == 0 {
		r.id = NewRecognizerID()
	}
	if r.sm == nil {
		r.sm = NewStateMachine("PinchRecognizer")
	}
	return r.id
}
