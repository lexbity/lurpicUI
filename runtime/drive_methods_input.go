package runtime

import (
	"encoding/json"
	"fmt"
	"math"
	"unicode/utf8"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/locate"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

const (
	driveDeviceTouch              = "touch"
	drivePrimaryWindow            = "primary"
	drivePrimaryWindowUnavailable = "only the primary input window is available"
)

func driveRectCenter(bounds gfx.Rect) gfx.Point {
	return gfx.Point{X: bounds.Min.X + bounds.Width()*0.5, Y: bounds.Min.Y + bounds.Height()*0.5}
}

// inputPointParams is the shared payload of the point verbs
// (click/press/release/move/scroll): window coordinates — the same space the
// platform bridge delivers (QP-10: identical pipeline from the very first
// switch) — or a target locator resolving to the arranged center.
type inputPointParams struct {
	X         *float32        `json:"x,omitempty"`
	Y         *float32        `json:"y,omitempty"`
	Button    string          `json:"button,omitempty"`
	Modifiers []string        `json:"modifiers,omitempty"`
	Gesture   string          `json:"gesture_handle,omitempty"`
	Device    string          `json:"device,omitempty"`
	Window    string          `json:"window,omitempty"`
	Target    *locate.Locator `json:"target,omitempty"`
}

// decode decodes the params and resolves the target locator (locators
// remove coordinates from the common path; raw coords are the explicit
// fallback).
func (p *inputPointParams) decode(rt *Runtime, raw json.RawMessage) (gfx.Point, platform.PointerButton, *proto.Error) {
	if err := decodeStrictDriveParams(raw, p); err != nil {
		return gfx.Point{}, 0, &proto.Error{Code: proto.ErrInvalidParams, Detail: "input params are malformed"}
	}
	return p.resolve(rt)
}

func (p *inputPointParams) resolve(rt *Runtime) (gfx.Point, platform.PointerButton, *proto.Error) {
	if p.Window != "" && p.Window != drivePrimaryWindow {
		return gfx.Point{}, 0, &proto.Error{Code: proto.ErrTargetUnavailable, Detail: drivePrimaryWindowUnavailable}
	}
	btn, berr := pointerButtonOf(p.Button)
	if berr != nil {
		return gfx.Point{}, 0, berr
	}
	if p.Target != nil {
		if p.X != nil || p.Y != nil {
			return gfx.Point{}, 0, &proto.Error{Code: proto.ErrInvalidParams, Detail: "target and coordinates are mutually exclusive"}
		}
		m, _, lerr := locate.Resolve(rt.root, *p.Target, rt.driveDescribe, rt.driveHitTest, rt.visibleChildren)
		if lerr != nil {
			return gfx.Point{}, 0, driveLocateError(lerr)
		}
		if lr := m.Base.LayoutRole(); lr != nil {
			b := lr.ArrangedBounds
			if !b.IsEmpty() {
				return gfx.Point{X: b.Min.X + b.Width()*0.5, Y: b.Min.Y + b.Height()*0.5}, btn, nil
			}
		}
		return gfx.Point{}, 0, &proto.Error{Code: proto.ErrInvalidParams, Detail: "target has no arranged bounds (no frame projected?)"}
	}
	if p.X == nil || p.Y == nil || math.IsNaN(float64(*p.X)) || math.IsNaN(float64(*p.Y)) || math.IsInf(float64(*p.X), 0) || math.IsInf(float64(*p.Y), 0) {
		return gfx.Point{}, 0, &proto.Error{Code: proto.ErrInvalidParams, Detail: "coordinates require finite x and y"}
	}
	return gfx.Point{X: *p.X, Y: *p.Y}, btn, nil
}

func (rt *Runtime) selectDriveInputDevice(requested string) (string, *proto.Error) {
	switch requested {
	case "":
		if rt.inputSystem != nil && rt.inputSystem.CurrentInputModality() == facet.InputModalityTouch {
			return driveDeviceTouch, nil
		}
		return "mouse", nil
	case "mouse", driveDeviceTouch:
		return requested, nil
	default:
		return "", &proto.Error{Code: proto.ErrInvalidParams, Detail: "device must be mouse or touch"}
	}
}

func (rt *Runtime) driveTouchID() (uint64, *proto.Error) {
	if rt.nextDriveTouchID >= (uint64(1)<<63)-1 {
		return 0, &proto.Error{Code: proto.ErrResourceLimit, Detail: "drive touch sequence IDs exhausted"}
	}
	rt.nextDriveTouchID++
	return uint64(1)<<63 | rt.nextDriveTouchID, nil
}

func drivePointerAtoms(device string, touchID uint64, point gfx.Point, button platform.PointerButton, modifiers platform.ModifierKeys, kind platform.PointerEventKind) []platform.Event {
	if device == driveDeviceTouch {
		phase := platform.TouchMove
		switch kind {
		case platform.PointerPress:
			phase = platform.TouchDown
		case platform.PointerRelease, platform.PointerCancel:
			phase = platform.TouchUp
		}
		return []platform.Event{platform.TouchEvent{SequenceID: touchID, Phase: phase, X: point.X, Y: point.Y, Source: platform.SourceDrive}}
	}
	return []platform.Event{platform.EventPointer{Kind: kind, Position: point, Button: button, Modifiers: modifiers, Source: platform.SourceDrive}}
}

func bindInputTarget(rt *Runtime, step *driveInputStep, locator *locate.Locator, followPoint bool) *proto.Error {
	if locator == nil || step == nil {
		return nil
	}
	mark, _, err := locate.Resolve(rt.root, *locator, rt.driveDescribe, rt.driveHitTest, rt.visibleChildren)
	if err != nil {
		return driveLocateError(err)
	}
	step.targetLocator = locator
	step.targetFacet = mark.Base.ID()
	step.targetFollowPoint = followPoint
	return nil
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
		return 0, &proto.Error{Code: proto.ErrInvalidParams, Detail: "button must be left, middle, or right"}
	}
}

func modifierMask(names []string) (platform.ModifierKeys, *proto.Error) {
	var mask platform.ModifierKeys
	for _, name := range names {
		var bit platform.ModifierKeys
		switch name {
		case "shift":
			bit = platform.ModShift
		case "control":
			bit = platform.ModControl
		case "alt":
			bit = platform.ModAlt
		case "super":
			bit = platform.ModSuper
		default:
			return 0, &proto.Error{Code: proto.ErrInvalidParams, Detail: "modifier must be shift, control, alt, or super"}
		}
		if mask&bit != 0 {
			return 0, &proto.Error{Code: proto.ErrInvalidParams, Detail: "duplicate modifier"}
		}
		mask |= bit
	}
	return mask, nil
}

// driveInputQueue returns the app event queue (QP-10: drive injection rides
// the same queue the platform bridge feeds).
func (rt *Runtime) driveInputQueue() platform.EventQueue {
	if rt.platformApp == nil {
		return nil
	}
	return rt.platformApp.Events()
}

// runInputStep creates an operation-owned event sequence. The top-level
// scheduler emits one event at each ordinary or explicitly authorized frame;
// a handler never recursively enters the frame pipeline.
func (rt *Runtime) runInputStep(cmd drive.Command, evs []platform.Event) driveOutcome {
	if rt.driveInputQueue() == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrTargetUnavailable, Detail: "platform input queue is unavailable"})
	}
	if len(evs) == 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input sequence is empty"})
	}
	return driveOutcome{deferred: true, inputStep: &driveInputStep{
		sessionID:  cmd.Session,
		generation: cmd.Generation, events: append([]platform.Event(nil), evs...),
	}}
}

// driveInputClick implements input.click: press + release at one point
// (the release fires activation handlers — the same shape a human click
// produces).
func driveInputClick(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputPointParams
	pt, btn, perr := p.decode(rt, cmd.Params)
	if perr != nil {
		return errorOutcome(perr)
	}
	mods, err := modifierMask(p.Modifiers)
	if err != nil {
		return errorOutcome(err)
	}
	device, perr := rt.selectDriveInputDevice(p.Device)
	if perr != nil {
		return errorOutcome(perr)
	}
	if device == driveDeviceTouch && (btn != platform.PointerLeft || mods != 0) {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "touch input supports only the primary contact without mouse modifiers"})
	}
	var touchID uint64
	if device == driveDeviceTouch {
		touchID, perr = rt.driveTouchID()
		if perr != nil {
			return errorOutcome(perr)
		}
		events := append(drivePointerAtoms(device, touchID, pt, btn, mods, platform.PointerPress), drivePointerAtoms(device, touchID, pt, btn, mods, platform.PointerRelease)...)
		out := rt.runInputStep(cmd, events)
		if out.inputStep != nil {
			out.inputStep.device = device
			out.inputStep.touchID = touchID
			out.inputStep.modifiers = mods
			if err := bindInputTarget(rt, out.inputStep, p.Target, true); err != nil {
				return errorOutcome(err)
			}
		}
		return out
	}
	events := append(drivePointerAtoms(device, 0, pt, btn, mods, platform.PointerMove), drivePointerAtoms(device, 0, pt, btn, mods, platform.PointerPress)...)
	events = append(events, drivePointerAtoms(device, 0, pt, btn, mods, platform.PointerRelease)...)
	out := rt.runInputStep(cmd, events)
	if out.inputStep != nil {
		out.inputStep.device = device
		out.inputStep.modifiers = mods
		if err := bindInputTarget(rt, out.inputStep, p.Target, true); err != nil {
			return errorOutcome(err)
		}
	}
	return out
}

func driveInputPress(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputPointParams
	pt, btn, perr := p.decode(rt, cmd.Params)
	if perr != nil {
		return errorOutcome(perr)
	}
	mods, err := modifierMask(p.Modifiers)
	if err != nil {
		return errorOutcome(err)
	}
	device, perr := rt.selectDriveInputDevice(p.Device)
	if perr != nil {
		return errorOutcome(perr)
	}
	if device == driveDeviceTouch && (btn != platform.PointerLeft || mods != 0) {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "touch input supports only the primary contact without mouse modifiers"})
	}
	var touchID uint64
	if device == driveDeviceTouch {
		touchID, perr = rt.driveTouchID()
		if perr != nil {
			return errorOutcome(perr)
		}
	}
	events := append(drivePointerAtoms(device, touchID, pt, btn, mods, platform.PointerMove), drivePointerAtoms(device, touchID, pt, btn, mods, platform.PointerPress)...)
	out := rt.runInputStep(cmd, events)
	if out.inputStep != nil {
		if rt.nextGestureID == ^uint64(0) {
			return errorOutcome(&proto.Error{Code: proto.ErrResourceLimit, Detail: "held gesture identifiers exhausted"})
		}
		rt.nextGestureID++
		out.inputStep.held = true
		out.inputStep.gesture = fmt.Sprintf("g/%s/%d/%d", cmd.Session, cmd.Generation, rt.nextGestureID)
		out.inputStep.position = pt
		out.inputStep.button = btn
		out.inputStep.modifiers = mods
		out.inputStep.device = device
		out.inputStep.touchID = touchID
		if err := bindInputTarget(rt, out.inputStep, p.Target, true); err != nil {
			return errorOutcome(err)
		}
	}
	return out
}

func driveInputRelease(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputPointParams
	if err := decodeStrictDriveParams(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.release params require a gesture handle"})
	}
	if p.Window != "" && p.Window != drivePrimaryWindow {
		return errorOutcome(&proto.Error{Code: proto.ErrTargetUnavailable, Detail: drivePrimaryWindowUnavailable})
	}
	held, perr := rt.ownedGesture(cmd, p.Gesture)
	if perr != nil {
		return errorOutcome(perr)
	}
	if p.Device != "" && p.Device != held.device {
		return errorOutcome(&proto.Error{Code: proto.ErrInputPreempted, Detail: "gesture device does not match its owner"})
	}
	pt := held.position
	target := held.targetLocator
	if p.Target != nil || p.X != nil || p.Y != nil {
		var err *proto.Error
		pt, _, err = p.decode(rt, cmd.Params)
		if err != nil {
			return errorOutcome(err)
		}
	}
	events := drivePointerAtoms(held.device, held.touchID, pt, held.button, held.modifiers, platform.PointerRelease)
	out := rt.runInputStep(cmd, events)
	if out.inputStep != nil {
		out.inputStep.release = true
		out.inputStep.gesture = held.gesture
		out.inputStep.device = held.device
		out.inputStep.touchID = held.touchID
		out.inputStep.modifiers = held.modifiers
		if err := bindInputTarget(rt, out.inputStep, target, true); err != nil {
			return errorOutcome(err)
		}
	}
	return out
}

func driveInputMove(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputPointParams
	pt, _, perr := p.decode(rt, cmd.Params)
	if perr != nil {
		return errorOutcome(perr)
	}
	var held *driveInputStep
	if rt.driveInputStep != nil && rt.driveInputStep.held {
		held, perr = rt.ownedGesture(cmd, p.Gesture)
		if perr != nil {
			return errorOutcome(perr)
		}
	} else if p.Gesture != "" {
		return errorOutcome(&proto.Error{Code: proto.ErrInputPreempted, Detail: "gesture handle is not active"})
	}
	if held != nil && p.Device != "" && p.Device != held.device {
		return errorOutcome(&proto.Error{Code: proto.ErrInputPreempted, Detail: "gesture device does not match its owner"})
	}
	device, perr := rt.selectDriveInputDevice(p.Device)
	if perr != nil {
		return errorOutcome(perr)
	}
	mods, perr := modifierMask(p.Modifiers)
	if perr != nil {
		return errorOutcome(perr)
	}
	if device == driveDeviceTouch && mods != 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "touch move cannot carry mouse modifiers"})
	}
	if held == nil && device == driveDeviceTouch {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "touch move requires an owned held gesture"})
	}
	events := drivePointerAtoms(device, 0, pt, platform.PointerNone, mods, platform.PointerMove)
	if held != nil {
		device = held.device
		events = drivePointerAtoms(device, held.touchID, pt, held.button, mods, platform.PointerMove)
	}
	out := rt.runInputStep(cmd, events)
	if out.inputStep != nil {
		out.inputStep.preserveHeld = held
		out.inputStep.position = pt
		out.inputStep.modifiers = mods
		out.inputStep.device = device
		if held != nil {
			out.inputStep.touchID = held.touchID
			out.inputStep.targetLocator = held.targetLocator
			out.inputStep.targetFacet = held.targetFacet
		}
	}
	return out
}

type inputLocationParams struct {
	X      *float32        `json:"x,omitempty"`
	Y      *float32        `json:"y,omitempty"`
	Target *locate.Locator `json:"target,omitempty"`
}

func (p inputLocationParams) resolve(rt *Runtime) (gfx.Point, *locate.Locator, facet.FacetID, *proto.Error) {
	if p.Target != nil {
		if p.X != nil || p.Y != nil {
			return gfx.Point{}, nil, 0, &proto.Error{Code: proto.ErrInvalidParams, Detail: "target and coordinates are mutually exclusive"}
		}
		mark, _, err := locate.Resolve(rt.root, *p.Target, rt.driveDescribe, rt.driveHitTest, rt.visibleChildren)
		if err != nil {
			return gfx.Point{}, nil, 0, driveLocateError(err)
		}
		layout := mark.Base.LayoutRole()
		if layout == nil || layout.ArrangedBounds.IsEmpty() {
			return gfx.Point{}, nil, 0, &proto.Error{Code: proto.ErrTargetUnavailable, Detail: "drag target has no arranged bounds"}
		}
		locator := *p.Target
		return driveRectCenter(layout.ArrangedBounds), &locator, mark.Base.ID(), nil
	}
	if p.X == nil || p.Y == nil || math.IsNaN(float64(*p.X)) || math.IsNaN(float64(*p.Y)) || math.IsInf(float64(*p.X), 0) || math.IsInf(float64(*p.Y), 0) {
		return gfx.Point{}, nil, 0, &proto.Error{Code: proto.ErrInvalidParams, Detail: "location requires finite x/y or target"}
	}
	return gfx.Point{X: *p.X, Y: *p.Y}, nil, 0, nil
}

// inputDragParams uses a oneof location for each endpoint.
type inputDragParams struct {
	From      inputLocationParams `json:"from"`
	To        inputLocationParams `json:"to"`
	MoveCount int                 `json:"move_count,omitempty"`
	Button    string              `json:"button,omitempty"`
	Device    string              `json:"device,omitempty"`
	Window    string              `json:"window,omitempty"`
	Modifiers []string            `json:"modifiers,omitempty"`
}

// driveInputDrag implements input.drag: press, interpolated moves, release —
// the exact event grammar of testkit's DriveDrag, so a drive drag and a
// human drag are indistinguishable downstream.
func driveInputDrag(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputDragParams
	if len(cmd.Params) == 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.drag requires from and to locations"})
	}
	if err := decodeStrictDriveParams(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.drag params are malformed"})
	}
	if p.Window != "" && p.Window != drivePrimaryWindow {
		return errorOutcome(&proto.Error{Code: proto.ErrTargetUnavailable, Detail: drivePrimaryWindowUnavailable})
	}
	btn, berr := pointerButtonOf(p.Button)
	if berr != nil {
		return errorOutcome(berr)
	}
	from, fromTarget, fromFacet, perr := p.From.resolve(rt)
	if perr != nil {
		return errorOutcome(perr)
	}
	to, _, _, perr := p.To.resolve(rt)
	if perr != nil {
		return errorOutcome(perr)
	}
	device, perr := rt.selectDriveInputDevice(p.Device)
	if perr != nil {
		return errorOutcome(perr)
	}
	modifiers, perr := modifierMask(p.Modifiers)
	if perr != nil {
		return errorOutcome(perr)
	}
	if device == driveDeviceTouch && (btn != platform.PointerLeft || modifiers != 0) {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "touch input supports only the primary contact without mouse modifiers"})
	}
	var touchID uint64
	if device == driveDeviceTouch {
		touchID, perr = rt.driveTouchID()
		if perr != nil {
			return errorOutcome(perr)
		}
	}
	// The default grammar mirrors testkit.Drag exactly (5 interpolated moves
	// at t=i/6, release at the target) so drag equivalence measures the input
	// pipeline, not a difference between the two drivers' event scripts.
	steps := p.MoveCount
	if steps < 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "move_count must be in [1,128]"})
	}
	if steps == 0 {
		steps = 5
	}
	if steps > 128 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.drag steps must be <= 128"})
	}
	evs := make([]platform.Event, 0, steps+3)
	atom := func(kind platform.PointerEventKind, point gfx.Point) {
		evs = append(evs, drivePointerAtoms(device, touchID, point, btn, modifiers, kind)...)
	}
	if device == "mouse" {
		atom(platform.PointerMove, from)
	}
	atom(platform.PointerPress, from)
	for i := 1; i <= steps; i++ {
		t := float32(i) / float32(steps+1)
		atom(platform.PointerMove, gfx.Point{
			X: from.X + (to.X-from.X)*t,
			Y: from.Y + (to.Y-from.Y)*t,
		})
	}
	atom(platform.PointerRelease, to)
	out := rt.runInputStep(cmd, evs)
	if out.inputStep != nil {
		out.inputStep.device = device
		out.inputStep.touchID = touchID
		out.inputStep.position = to
		out.inputStep.targetLocator = fromTarget
		out.inputStep.targetFacet = fromFacet
		out.inputStep.targetFollowPoint = false
	}
	return out
}

// inputTypeParams is the type payload.
type inputTypeParams struct {
	Text   string     `json:"text,omitempty"`
	Events []imeEvent `json:"events,omitempty"`
	Window string     `json:"window,omitempty"`
}

type imeEvent struct {
	Kind      string `json:"kind"`
	Text      string `json:"text"`
	CursorPos int    `json:"cursor_pos,omitempty"`
}

// driveInputType implements input.type (FR-14, IME fidelity G5.4): when the
// focused mark enables IME, text flows through EventText — the same route a
// real keyboard takes on every platform backend. Without an IME-enabled
// focused mark the verb refuses with a typed error: synthesizing key codes
// per rune would diverge from the real keyboard path.
func driveInputType(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputTypeParams
	if len(cmd.Params) == 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.type requires text"})
	}
	if err := decodeStrictDriveParams(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.type params are malformed"})
	}
	if p.Window != "" && p.Window != drivePrimaryWindow {
		return errorOutcome(&proto.Error{Code: proto.ErrTargetUnavailable, Detail: drivePrimaryWindowUnavailable})
	}
	if len(p.Events) == 0 && p.Text == "" {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.type requires text or IME events"})
	}
	if len(p.Events) != 0 && p.Text != "" {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.type accepts text or IME events, not both"})
	}
	if len(p.Events) > 64 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.type supports at most 64 IME events"})
	}
	total := len(p.Text)
	for _, event := range p.Events {
		total += len(event.Text)
	}
	if total > 16*1024 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.type data exceeds 16 KiB"})
	}
	if !utf8.ValidString(p.Text) {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.type text is invalid UTF-8"})
	}
	if rt.focusManager == nil || rt.focusManager.FocusedImpl() == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.type requires a focused text mark"})
	}
	focused := rt.focusManager.FocusedImpl()
	tr := focused.Base().TextRole()
	if tr == nil || !tr.IMEEnabled {
		return errorOutcome(&proto.Error{
			Code:   proto.ErrTargetUnavailable,
			Detail: "input.type requires an IME-enabled focused text mark (synthetic key-code routes diverge from real keyboards)",
		})
	}
	events := make([]platform.Event, 0, max(1, len(p.Events)))
	if len(p.Events) == 0 {
		events = append(events, platform.EventText{Text: p.Text, Source: platform.SourceDrive})
	} else {
		for _, event := range p.Events {
			if event.Text == "" || !utf8.ValidString(event.Text) {
				return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "IME event text must be valid and non-empty"})
			}
			switch event.Kind {
			case "commit":
				events = append(events, platform.EventIMECommit{Text: event.Text, Source: platform.SourceDrive})
			case "compose":
				if event.CursorPos < 0 || event.CursorPos > utf8.RuneCountInString(event.Text) {
					return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "IME cursor_pos is outside text"})
				}
				events = append(events, platform.EventIMECompose{Text: event.Text, CursorPos: event.CursorPos, Source: platform.SourceDrive})
			default:
				return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "IME event kind must be commit or compose"})
			}
		}
	}
	return rt.runInputStep(cmd, events)
}

// inputScrollParams is the scroll payload.
type inputScrollParams struct {
	X         *float32        `json:"x,omitempty"`
	Y         *float32        `json:"y,omitempty"`
	Target    *locate.Locator `json:"target,omitempty"`
	Window    string          `json:"window,omitempty"`
	DeltaX    float32         `json:"delta_x"`
	DeltaY    float32         `json:"delta_y"`
	Precise   bool            `json:"precise,omitempty"`
	Modifiers []string        `json:"modifiers,omitempty"`
}

// driveInputScroll implements input.scroll.
func driveInputScroll(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputScrollParams
	if err := decodeStrictDriveParams(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.scroll params are malformed"})
	}
	point := inputPointParams{X: p.X, Y: p.Y, Target: p.Target, Window: p.Window}
	pt, _, perr := point.resolve(rt)
	if perr != nil {
		return errorOutcome(perr)
	}
	if p.DeltaX == 0 && p.DeltaY == 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "scroll delta cannot be zero"})
	}
	if math.IsNaN(float64(p.DeltaX)) || math.IsNaN(float64(p.DeltaY)) || math.IsInf(float64(p.DeltaX), 0) || math.IsInf(float64(p.DeltaY), 0) {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "scroll deltas must be finite"})
	}
	mods, err := modifierMask(p.Modifiers)
	if err != nil {
		return errorOutcome(err)
	}
	return rt.runInputStep(cmd, []platform.Event{
		platform.EventScroll{
			Position: pt, DeltaX: p.DeltaX, DeltaY: p.DeltaY,
			Precise: p.Precise, Modifiers: mods, Source: platform.SourceDrive,
		},
	})
}

// inputFocusParams is the focus payload.
type inputFocusParams struct {
	Target  locate.Locator `json:"target"`
	MaxHops int            `json:"max_hops,omitempty"`
	Shift   bool           `json:"shift,omitempty"`
	Window  string         `json:"window,omitempty"`
}

// driveInputFocus implements input.focus (QP-11 orthogonality): drives the
// FocusRole without a pointer — a driver may move focus without owning
// input.
func driveInputFocus(rt *Runtime, cmd drive.Command) driveOutcome {
	var p inputFocusParams
	if len(cmd.Params) == 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.focus requires target"})
	}
	if err := decodeStrictDriveParams(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.focus params are malformed"})
	}
	if p.Window != "" && p.Window != drivePrimaryWindow {
		return errorOutcome(&proto.Error{Code: proto.ErrTargetUnavailable, Detail: drivePrimaryWindowUnavailable})
	}
	m, _, lerr := locate.Resolve(rt.root, p.Target, rt.driveDescribe, rt.driveHitTest, rt.visibleChildren)
	if lerr != nil {
		return errorOutcome(driveLocateError(lerr))
	}
	if rt.focusManager == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "no focus manager"})
	}
	role := m.Base.FocusRole()
	if role == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrTargetNotFocusable, Detail: "target has no focus role"})
	}
	hops := p.MaxHops
	if hops == 0 {
		hops = 256
	}
	if hops < 1 || hops > 256 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "max_hops must be in [1,256]"})
	}
	modifiers := platform.ModifierKeys(0)
	if p.Shift {
		modifiers = platform.ModShift
	}
	events := make([]platform.Event, 0, hops*2)
	for i := 0; i < hops; i++ {
		events = append(events,
			platform.EventKey{Kind: platform.KeyPress, Key: platform.KeyTab, Modifiers: modifiers, Source: platform.SourceDrive},
			platform.EventKey{Kind: platform.KeyRelease, Key: platform.KeyTab, Modifiers: modifiers, Source: platform.SourceDrive},
		)
	}
	step := &driveInputStep{sessionID: cmd.Session, generation: cmd.Generation, events: events, focusTarget: m.Base.ID()}
	return driveOutcome{deferred: true, inputStep: step}
}
