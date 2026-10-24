package runtime

import (
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

// OwnerKind names who owns input this event (LDP QP-11).
type OwnerKind uint8

const (
	// OwnerHuman is the default: platform events are accepted.
	OwnerHuman OwnerKind = iota
	// OwnerClient marks a drive input step in flight.
	OwnerClient
)

func (o OwnerKind) String() string {
	switch o {
	case OwnerHuman:
		return "human"
	case OwnerClient:
		return "client"
	default:
		return "OwnerKind(unknown)"
	}
}

// InputOwnership is the app-readable input-ownership state (QP-11): which
// client's events the runtime accepts. The studio status bar binds it to the
// "driving" light; drivers read it via the named store
// runtime.input.ownership. Focus and ownership are orthogonal: a driver may
// move focus (input.focus) without owning input.
type InputOwnership struct {
	Owner    OwnerKind `json:"owner"`
	ClientID string    `json:"client_id,omitempty"`
}

// inputOwnershipName is the named-store address of the ownership store.
const inputOwnershipName = "runtime.input.ownership"

// installInputOwnership creates the ownership store and registers it under
// its stable name. Called from New when the drive seam is enabled.
func (rt *Runtime) installInputOwnership() error {
	rt.inputOwnership = store.NewValueStore(InputOwnership{Owner: OwnerHuman})
	return rt.RegisterNamedStore(inputOwnershipName, rt.inputOwnership)
}

// Ownership returns the input-ownership store (nil when the drive seam is
// disabled). The studio status bar binds its "driving" light through it.
func (rt *Runtime) Ownership() *store.ValueStore[InputOwnership] {
	return rt.inputOwnership
}

// driveInputStep wraps one drive input command's event sequence (QP-11):
// request-scoped ownership — there is no standing grant to revoke. The step
// begins when the verb enqueues its synthesized events and ends when the
// runtime has consumed them (or the step was preempted).
type driveInputStep struct {
	commandID uint64
	clientID  string
	// pending counts the step's synthesized events not yet consumed by the
	// input phase; the step ends when it reaches zero.
	pending int
	// preempted marks the step as severed: the reply reports input.preempted
	// instead of success.
	preempted bool
}

// beginInputStep flips ownership to the client for the step's duration.
func (rt *Runtime) beginInputStep(step *driveInputStep) {
	rt.inputOwnership.Set(InputOwnership{Owner: OwnerClient, ClientID: step.clientID})
	rt.driveInputStep = step
}

// endInputStep clears request-scoped ownership.
func (rt *Runtime) endInputStep(step *driveInputStep) {
	if rt.driveInputStep == step {
		rt.driveInputStep = nil
	}
	rt.inputOwnership.Set(InputOwnership{Owner: OwnerHuman})
}

// preemptInputStep severs an in-flight drive input step (QP-11): the
// remaining synthesized events are voided from the pending set — gesture
// recognizers exercise exactly the abort paths a human yank produces —
// ownership returns to the human, and own.preempted fans out through the
// event consumer (the command reply carries input.preempted to the driving
// connection).
func (rt *Runtime) preemptInputStep() {
	step := rt.driveInputStep
	if step == nil {
		return
	}
	step.preempted = true
	rt.driveInputStep = nil
	// Void the step's remaining synthesized events: they never reach the
	// input pipeline.
	kept := rt.pendingEvents[:0]
	for _, ev := range rt.pendingEvents {
		if eventSource(ev) == platform.SourceDrive {
			step.pending--
			continue
		}
		kept = append(kept, ev)
	}
	rt.pendingEvents = kept
	rt.inputOwnership.Set(InputOwnership{Owner: OwnerHuman})
	rt.driveSink.notifyPreempted(step.commandID, step.clientID)
}

// scanForPreemption is the input-phase observation (QP-11, FR-15): any
// SourcePlatform event observed while a drive input step is in flight
// preempts the step, per event — a scripted drag can be severed mid-gesture
// by a stray human move, and that is the honest outcome.
func (rt *Runtime) scanForPreemption(events []platform.Event) {
	if rt.driveInputStep == nil {
		return
	}
	for _, ev := range events {
		if eventSource(ev) == platform.SourcePlatform {
			rt.preemptInputStep()
			return
		}
	}
}

// eventSource extracts an event's provenance (the input taxonomy: pointer,
// key, scroll, and text events carry Source).
func eventSource(ev platform.Event) platform.EventSource {
	switch e := ev.(type) {
	case platform.EventPointer:
		return e.Source
	case platform.EventKey:
		return e.Source
	case platform.EventScroll:
		return e.Source
	case platform.EventText:
		return e.Source
	default:
		return platform.SourcePlatform
	}
}
