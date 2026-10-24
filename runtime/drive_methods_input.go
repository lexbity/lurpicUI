package runtime

import (
	"encoding/json"
	"fmt"

	"codeburg.org/lexbit/lurpicui/drive/locate"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	drive "codeburg.org/lexbit/lurpicui/drive/server"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

// inputPointParams is the shared payload of the point verbs
// (click/press/release/move/scroll): window coordinates — the same space the
// platform bridge delivers (QP-10: identical pipeline from the very first
// switch) — or a target locator resolving to the arranged center.
type inputPointParams struct {
	X      float32         `json:"x,omitempty"`
	Y      float32         `json:"y,omitempty"`
	Button string          `json:"button,omitempty"`
	Target *locate.Locator `json:"target,omitempty"`
}

// decode decodes the params and resolves the target locator (locators
// remove coordinates from the common path; raw coords are the explicit
// fallback).
func (p *inputPointParams) decode(rt *Runtime, raw json.RawMessage) (gfx.Point, platform.PointerButton, *proto.Error) {
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, p); err != nil {
			return gfx.Point{}, 0, &proto.Error{Code: proto.ErrInternal, Detail: "input params: " + err.Error()}
		}
	}
	btn, berr := pointerButtonOf(p.Button)
	if berr != nil {
		return gfx.Point{}, 0, berr
	}
	if p.Target != nil {
		m, _, lerr := locate.Resolve(rt.root, *p.Target, rt.driveDescribe, rt.driveHitTest)
		if lerr != nil {
			return gfx.Point{}, 0, driveLocateError(lerr)
		}
		if lr := m.Base.LayoutRole(); lr != nil {
			b := lr.ArrangedBounds
			if !b.IsEmpty() {
				return gfx.Point{X: b.Min.X + b.Width()*0.5, Y: b.Min.Y + b.Height()*0.5}, btn, nil
			}
		}
		return gfx.Point{}, 0, &proto.Error{Code: proto.ErrInternal, Detail: "target has no arranged bounds (no frame projected?)"}
	}
	return gfx.Point{X: p.X, Y: p.Y}, btn, nil
}

// pointerButtonOf decodes the button name (default left).
func pointerButtonOf(name string) (platform.PointerButton, *proto.Error) {
	switch name {
	case "", "left":
		return platform.PointerLeft, nil
	case "middle":
		return platform.PointerMiddle, nil
	case "right":
		return platform.PointerRight, nil
	default:
		return 0, &proto.Error{Code: proto.ErrInternal, Detail: "button must be left, middle, or right"}
	}
}

// driveInputQueue returns the app event queue (QP-10: drive injection rides
// the same queue the platform bridge feeds).
func (rt *Runtime) driveInputQueue() platform.EventQueue {
	if rt.platformApp == nil {
		return nil
	}
	return rt.platformApp.Events()
}

// runInputStep enqueues the step's events and runs frames until the input
// phase has consumed them (or preemption severed the step). Returns a typed
// input.preempted error when preempted (QP-11, FR-15).
func (rt *Runtime) runInputStep(cmd drive.Command, evs []platform.Event) driveOutcome {
	queue := rt.driveInputQueue()
	if queue == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "no platform event queue"}, rt.driveSink.Epoch())
	}
	step := &driveInputStep{commandID: cmd.ID, clientID: "drive", pending: len(evs)}
	rt.beginInputStep(step)
	queue.EnqueueDrive(evs...)
	defer rt.endInputStep(step)

	// Frames until consumed: each frame drains the queue at its head; the
	// step ends when every synthesized event has been routed. Bounded so a
	// wedged pipeline replies instead of hanging the driver.
	now := rt.clock.Now()
	for i := 0; i < 128; i++ {
		rt.RunOneFrameAt(now)
		now = now.Add(rt.framePeriod())
		if step.pending <= 0 || step.preempted {
			break
		}
	}
	if step.preempted {
		return errorOutcome(&proto.Error{
			Code:   proto.ErrPreempted,
			Detail: "input step severed by a platform event (ownership returned to human)",
		}, rt.driveSink.Epoch())
	}
	if step.pending > 0 {
		return errorOutcome(&proto.Error{
			Code:   proto.ErrInternal,
			Detail: fmt.Sprintf("input step did not settle: %d events unconsumed", step.pending),
		}, rt.driveSink.Epoch())
	}
	return payloadOutcome(json.RawMessage(`{"consumed":true}`), rt.driveSink.Epoch())
}

// driveInputClick implements input.click: press + release at one point
// (the release fires activation handlers — the same shape a human click
// produces).
func driveInputClick(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputPointParams
	pt, btn, perr := p.decode(rt, cmd.Params)
	if perr != nil {
		return errorOutcome(perr, rt.driveSink.Epoch())
	}
	return rt.runInputStep(cmd, []platform.Event{
		platform.EventPointer{Kind: platform.PointerPress, Position: pt, Button: btn, Source: platform.SourceDrive},
		platform.EventPointer{Kind: platform.PointerRelease, Position: pt, Button: btn, Source: platform.SourceDrive},
	})
}

func driveInputPress(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputPointParams
	pt, btn, perr := p.decode(rt, cmd.Params)
	if perr != nil {
		return errorOutcome(perr, rt.driveSink.Epoch())
	}
	return rt.runInputStep(cmd, []platform.Event{
		platform.EventPointer{Kind: platform.PointerPress, Position: pt, Button: btn, Source: platform.SourceDrive},
	})
}

func driveInputRelease(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputPointParams
	pt, btn, perr := p.decode(rt, cmd.Params)
	if perr != nil {
		return errorOutcome(perr, rt.driveSink.Epoch())
	}
	return rt.runInputStep(cmd, []platform.Event{
		platform.EventPointer{Kind: platform.PointerRelease, Position: pt, Button: btn, Source: platform.SourceDrive},
	})
}

func driveInputMove(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputPointParams
	pt, _, perr := p.decode(rt, cmd.Params)
	if perr != nil {
		return errorOutcome(perr, rt.driveSink.Epoch())
	}
	return rt.runInputStep(cmd, []platform.Event{
		platform.EventPointer{Kind: platform.PointerMove, Position: pt, Source: platform.SourceDrive},
	})
}

// inputDragParams is the drag payload: from/to points (no target form — a
// drag's endpoints are geometry by nature).
type inputDragParams struct {
	FromX  float32 `json:"from_x"`
	FromY  float32 `json:"from_y"`
	ToX    float32 `json:"to_x"`
	ToY    float32 `json:"to_y"`
	Steps  int     `json:"steps,omitempty"`
	Button string  `json:"button,omitempty"`
}

// driveInputDrag implements input.drag: press, interpolated moves, release —
// the exact event grammar of testkit's DriveDrag, so a drive drag and a
// human drag are indistinguishable downstream.
func driveInputDrag(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputDragParams
	if len(cmd.Params) == 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "input.drag requires from_x/from_y/to_x/to_y"}, rt.driveSink.Epoch())
	}
	if err := json.Unmarshal(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "input.drag params: " + err.Error()}, rt.driveSink.Epoch())
	}
	btn, berr := pointerButtonOf(p.Button)
	if berr != nil {
		return errorOutcome(berr, rt.driveSink.Epoch())
	}
	// The default grammar mirrors testkit.Drag exactly (5 interpolated moves
	// at t=i/6, release at the target) so drag equivalence measures the input
	// pipeline, not a difference between the two drivers' event scripts.
	steps := p.Steps
	if steps <= 0 {
		steps = 5
	}
	if steps > 128 {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "input.drag steps must be <= 128"}, rt.driveSink.Epoch())
	}
	evs := make([]platform.Event, 0, steps+2)
	evs = append(evs, platform.EventPointer{
		Kind: platform.PointerPress, Button: btn,
		Position: gfx.Point{X: p.FromX, Y: p.FromY}, Source: platform.SourceDrive,
	})
	for i := 1; i <= steps; i++ {
		t := float32(i) / float32(steps+1)
		evs = append(evs, platform.EventPointer{
			Kind: platform.PointerMove, Button: btn,
			Position: gfx.Point{
				X: p.FromX + (p.ToX-p.FromX)*t,
				Y: p.FromY + (p.ToY-p.FromY)*t,
			},
			Source: platform.SourceDrive,
		})
	}
	evs = append(evs, platform.EventPointer{
		Kind: platform.PointerRelease, Button: btn,
		Position: gfx.Point{X: p.ToX, Y: p.ToY}, Source: platform.SourceDrive,
	})
	return rt.runInputStep(cmd, evs)
}

// inputTypeParams is the type payload.
type inputTypeParams struct {
	Text string `json:"text"`
}

// driveInputType implements input.type (FR-14, IME fidelity G5.4): when the
// focused mark enables IME, text flows through EventText — the same route a
// real keyboard takes on every platform backend. Without an IME-enabled
// focused mark the verb refuses with a typed error: synthesizing key codes
// per rune would diverge from the real keyboard path.
func driveInputType(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputTypeParams
	if len(cmd.Params) == 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "input.type requires text"}, rt.driveSink.Epoch())
	}
	if err := json.Unmarshal(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "input.type params: " + err.Error()}, rt.driveSink.Epoch())
	}
	if p.Text == "" {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "input.type text must be non-empty"}, rt.driveSink.Epoch())
	}
	if rt.focusManager == nil || rt.focusManager.FocusedImpl() == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "input.type requires a focused text mark"}, rt.driveSink.Epoch())
	}
	focused := rt.focusManager.FocusedImpl()
	tr := focused.Base().TextRole()
	if tr == nil || !tr.IMEEnabled {
		return errorOutcome(&proto.Error{
			Code:   proto.ErrInternal,
			Detail: "input.type requires an IME-enabled focused text mark (synthetic key-code routes diverge from real keyboards)",
		}, rt.driveSink.Epoch())
	}
	return rt.runInputStep(cmd, []platform.Event{
		platform.EventText{Text: p.Text, Source: platform.SourceDrive},
	})
}

// inputScrollParams is the scroll payload.
type inputScrollParams struct {
	inputPointParams
	DeltaX  float32 `json:"delta_x"`
	DeltaY  float32 `json:"delta_y"`
	Precise bool    `json:"precise,omitempty"`
}

// driveInputScroll implements input.scroll.
func driveInputScroll(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputScrollParams
	pt, _, perr := p.decode(rt, cmd.Params)
	if perr != nil {
		return errorOutcome(perr, rt.driveSink.Epoch())
	}
	return rt.runInputStep(cmd, []platform.Event{
		platform.EventScroll{
			Position: pt, DeltaX: p.DeltaX, DeltaY: p.DeltaY,
			Precise: p.Precise, Source: platform.SourceDrive,
		},
	})
}

// inputFocusParams is the focus payload.
type inputFocusParams struct {
	Target locate.Locator `json:"target"`
}

// driveInputFocus implements input.focus (QP-11 orthogonality): drives the
// FocusRole without a pointer — a driver may move focus without owning
// input.
func driveInputFocus(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputFocusParams
	if len(cmd.Params) == 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "input.focus requires target"}, rt.driveSink.Epoch())
	}
	if err := json.Unmarshal(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "input.focus params: " + err.Error()}, rt.driveSink.Epoch())
	}
	m, _, lerr := locate.Resolve(rt.root, p.Target, rt.driveDescribe, rt.driveHitTest)
	if lerr != nil {
		return errorOutcome(driveLocateError(lerr), rt.driveSink.Epoch())
	}
	if rt.focusManager == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "no focus manager"}, rt.driveSink.Epoch())
	}
	if !rt.focusManager.SetFocus(m.Impl) {
		return errorOutcome(&proto.Error{
			Code:   proto.ErrInternal,
			Detail: "focus refused (the mark is not focusable or focus rules rejected it)",
		}, rt.driveSink.Epoch())
	}
	return payloadOutcome(json.RawMessage(`{"focused":true}`), rt.driveSink.Epoch())
}
