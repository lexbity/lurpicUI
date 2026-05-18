package gesture

import (
	"fmt"
	"sync/atomic"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

// RecognizerID identifies one gesture recognizer instance.
type RecognizerID uint32

// TouchID identifies one normalized touch contact.
type TouchID uint32

// Touch normalizes pointer and touch contacts.
type Touch struct {
	ID        TouchID
	Position  gfx.Point
	PrevPos   gfx.Point
	StartPos  gfx.Point
	Force     float32
	Timestamp time.Duration
}

// InputEvent carries input metadata shared across recognizers.
type InputEvent struct {
	Modifiers platform.ModifierKeys
	Timestamp time.Duration
}

// RecognizerState describes the current recognizer lifecycle.
type RecognizerState uint8

const (
	RecognizerPossible RecognizerState = iota
	RecognizerBegan
	RecognizerChanged
	RecognizerEnded
	RecognizerCancelled
	RecognizerFailed
)

func (s RecognizerState) String() string {
	switch s {
	case RecognizerPossible:
		return "Possible"
	case RecognizerBegan:
		return "Began"
	case RecognizerChanged:
		return "Changed"
	case RecognizerEnded:
		return "Ended"
	case RecognizerCancelled:
		return "Cancelled"
	case RecognizerFailed:
		return "Failed"
	default:
		return fmt.Sprintf("RecognizerState(%d)", uint8(s))
	}
}

// RecognizerDelegate controls competition between recognizers.
type RecognizerDelegate struct {
	ShouldRequireFailureOf            []RecognizerID
	ShouldRecognizeSimultaneouslyWith func(other Recognizer) bool
	Priority                          int
}

// Recognizer is implemented by gesture recognizers.
type Recognizer interface {
	ID() RecognizerID
	State() RecognizerState
	Delegate() RecognizerDelegate
	Reset()
	TouchesBegan(touches []Touch, event InputEvent)
	TouchesMoved(touches []Touch, event InputEvent)
	TouchesEnded(touches []Touch, event InputEvent)
	TouchesCancelled(touches []Touch, event InputEvent)
}

// GestureRole attaches recognizers to a facet.
type GestureRole struct {
	Recognizers []Recognizer
}

// StateMachine provides a reusable recognizer state tracker.
type StateMachine struct {
	name  string
	state RecognizerState
}

// NewStateMachine constructs a state machine with the supplied recognizer name.
func NewStateMachine(name string) *StateMachine {
	return &StateMachine{name: name, state: RecognizerPossible}
}

// State returns the current state.
func (m *StateMachine) State() RecognizerState {
	if m == nil {
		return RecognizerPossible
	}
	return m.state
}

// Reset returns the state machine to Possible.
func (m *StateMachine) Reset() {
	if m == nil {
		return
	}
	m.state = RecognizerPossible
}

// Transition advances the state machine, panicking on invalid transitions.
func (m *StateMachine) Transition(next RecognizerState) {
	if m == nil {
		return
	}
	current := m.state
	if next == current {
		if next == RecognizerCancelled || next == RecognizerFailed {
			m.state = RecognizerPossible
		}
		return
	}
	if !validTransition(current, next) {
		panic(fmt.Sprintf("gesture: invalid transition for %s: %s -> %s", m.name, current, next))
	}
	switch next {
	case RecognizerCancelled, RecognizerFailed:
		m.state = next
		m.state = RecognizerPossible
	default:
		m.state = next
	}
}

func validTransition(current, next RecognizerState) bool {
	switch current {
	case RecognizerPossible:
		return next == RecognizerPossible || next == RecognizerBegan || next == RecognizerChanged || next == RecognizerEnded || next == RecognizerCancelled || next == RecognizerFailed
	case RecognizerBegan, RecognizerChanged:
		return next == RecognizerBegan || next == RecognizerChanged || next == RecognizerEnded || next == RecognizerCancelled || next == RecognizerFailed
	case RecognizerEnded, RecognizerCancelled, RecognizerFailed:
		return next == RecognizerPossible || next == RecognizerCancelled || next == RecognizerFailed
	default:
		return false
	}
}

// NewRecognizerID generates a new recognizer ID.
func NewRecognizerID() RecognizerID {
	return RecognizerID(recognizerIDSource.Add(1))
}

var recognizerIDSource atomic.Uint32

// NamedStateMachine returns a state machine name suitable for panic messages.
func NamedStateMachine(name string) *StateMachine {
	return NewStateMachine(name)
}
