package runtime

import (
	"encoding/json"
	"strconv"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/locate"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

type driveInputStep struct {
	sessionID         string
	generation        uint64
	preempted         bool
	plan              *framePlan
	cmd               drive.Command
	entry             *drive.Entry
	events            []platform.Event
	completed         int
	focusTarget       facet.FacetID
	held              bool
	release           bool
	gesture           string
	position          gfx.Point
	button            platform.PointerButton
	modifiers         platform.ModifierKeys
	device            string
	touchID           uint64
	targetLocator     *locate.Locator
	targetFacet       facet.FacetID
	targetFollowPoint bool
	preserveHeld      *driveInputStep
}

type controlWatcher struct {
	id       uint64
	callback func(facet.ControlSnapshot)
}

func (rt *Runtime) ControlSnapshot() facet.ControlSnapshot {
	s := facet.ControlSnapshot{InputOwner: facet.ControlOwnerHuman, TimeMode: facet.ControlTimeWall}
	if rt.driveInputStep != nil {
		s.InputOwner = facet.ControlOwnerClient
		s.InputSession = rt.driveInputStep.sessionID
		s.InputGeneration = rt.driveInputStep.generation
		if rt.driveInputStep.held {
			s.HeldGestures = 1
		}
	}
	if rt.clock != nil && rt.clock.Mode() == ClockVirtual {
		s.TimeMode = facet.ControlTimeVirtual
	}
	if rt.drivePort != nil {
		s.TimeOwnerSession = rt.drivePort.timeOwnerSession
		s.TimeGeneration = rt.drivePort.timeOwnerGeneration
	}
	return s
}

func (rt *Runtime) WatchControl(id facet.FacetID, callback func(facet.ControlSnapshot)) func() {
	if id == 0 || callback == nil {
		panic("runtime: control observation requires an attached facet identity and callback")
	}
	rt.controlWatchSeq++
	watchID := rt.controlWatchSeq
	rt.controlWatchers[id] = append(rt.controlWatchers[id], controlWatcher{id: watchID, callback: callback})
	callback(rt.ControlSnapshot())
	return func() {
		callbacks := rt.controlWatchers[id]
		for i, candidate := range callbacks {
			if candidate.id == watchID {
				rt.controlWatchers[id] = append(callbacks[:i], callbacks[i+1:]...)
				if len(rt.controlWatchers[id]) == 0 {
					delete(rt.controlWatchers, id)
				}
				return
			}
		}
	}
}

// Go functions cannot be compared except to nil. A subscription therefore
// returns an idempotent closure that removes its exact insertion slot.
func (rt *Runtime) notifyControlChanged() {
	snapshot := rt.ControlSnapshot()
	for id, callbacks := range rt.controlWatchers {
		rt.Invalidate(id, facet.DirtyProjection, "runtime.control")
		for _, watcher := range callbacks {
			watcher.callback(snapshot)
		}
	}
}

func (rt *Runtime) beginInputStep(step *driveInputStep) {
	rt.driveInputStep = step
	rt.notifyControlChanged()
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

func (rt *Runtime) preemptInputStep() {
	rt.preemptInputStepOwned(func(*driveInputStep) bool { return true })
}

func (rt *Runtime) preemptInputStepFor(session string, generation uint64) {
	rt.preemptInputStepOwned(func(step *driveInputStep) bool {
		return step.sessionID == session && step.generation == generation
	})
}

func (rt *Runtime) preemptInputStepOwned(owns func(*driveInputStep) bool) {
	step := rt.driveInputStep
	if step == nil || !owns(step) {
		return
	}
	step.preempted = true
	rt.driveInputStep = nil
	kept := rt.pendingEvents[:0]
	for _, ev := range rt.pendingEvents {
		if eventSource(ev) != platform.SourceDrive {
			kept = append(kept, ev)
		}
	}
	kept = append(kept, inputCancellationEvent(step)...)
	rt.pendingEvents = kept
	if rt.drivePort != nil && step.entry != nil {
		rt.drivePort.completeClaimed(step.entry, step.cmd, drive.Result{Err: &proto.Error{Code: proto.ErrInputPreempted, Detail: "input sequence stopped before its next atom"}, Frame: rt.drivePort.Frame(), Cut: rt.drivePort.Cut()})
	}
	rt.notifyControlChanged()
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

func inputCancellationEvent(step *driveInputStep) []platform.Event {
	if step.device == driveDeviceTouch && step.touchID != 0 {
		return []platform.Event{platform.TouchEvent{SequenceID: step.touchID, Phase: platform.TouchCancel, X: step.position.X, Y: step.position.Y, Source: platform.SourceRuntimeCancel, Attribution: platform.InputAttribution{Session: step.sessionID, Generation: step.generation, Operation: step.cmd.OpID}}}
	}
	return []platform.Event{platform.EventPointer{Kind: platform.PointerCancel, Source: platform.SourceRuntimeCancel, Attribution: platform.InputAttribution{Session: step.sessionID, Generation: step.generation, Operation: step.cmd.OpID}}}
}

func (rt *Runtime) drivePrepareInputFrame() {
	step := rt.driveInputStep
	if step == nil || step.preempted || step.completed >= len(step.events) || rt.drivePort == nil {
		return
	}
	if step.focusTarget != 0 && rt.focusManager != nil && rt.focusManager.Focused() == step.focusTarget {
		rt.finishInputStep(step, nil)
		return
	}
	if step.targetLocator != nil {
		mark, _, err := locate.Resolve(rt.root, *step.targetLocator, rt.driveDescribe, rt.driveHitTest, rt.visibleChildren)
		if err != nil || mark.Base.ID() != step.targetFacet {
			rt.finishInputStep(step, &proto.Error{Code: proto.ErrTargetUnavailable, Detail: "the pinned input target disappeared or changed lifetime"})
			return
		}
		if layout := mark.Base.LayoutRole(); layout == nil || layout.ArrangedBounds.IsEmpty() {
			rt.finishInputStep(step, &proto.Error{Code: proto.ErrTargetUnavailable, Detail: "the pinned input target is no longer arranged"})
			return
		} else if step.targetFollowPoint {
			center := driveRectCenter(layout.ArrangedBounds)
			step.position = center
			event := step.events[step.completed]
			switch current := event.(type) {
			case platform.EventPointer:
				current.Position = center
				step.events[step.completed] = current
			case platform.TouchEvent:
				current.X, current.Y = center.X, center.Y
				step.events[step.completed] = current
			}
		}
	}
	queue := rt.driveInputQueue()
	if queue == nil {
		rt.preemptInputStep()
		return
	}
	event := step.events[step.completed]
	setDriveAttribution(&event, step.cmd)
	queue.EnqueueDrive(event)
}

func setDriveAttribution(event *platform.Event, cmd drive.Command) {
	if event == nil {
		return
	}
	a := platform.InputAttribution{Session: cmd.Session, Generation: cmd.Generation, Operation: cmd.OpID}
	switch e := (*event).(type) {
	case platform.EventPointer:
		e.Attribution = a
		*event = e
	case platform.EventKey:
		e.Attribution = a
		*event = e
	case platform.EventScroll:
		e.Attribution = a
		*event = e
	case platform.EventText:
		e.Attribution = a
		*event = e
	case platform.EventIMECompose:
		e.Attribution = a
		*event = e
	case platform.EventIMECommit:
		e.Attribution = a
		*event = e
	case platform.TouchEvent:
		e.Attribution = a
		*event = e
	}
}

func (rt *Runtime) driveInputAtomApplied(count int) {
	step := rt.driveInputStep
	if step == nil || count == 0 {
		return
	}
	step.completed += count
	if step.plan != nil {
		if step.completed >= len(step.events) {
			step.plan.inputAtomsComplete = true
			if step.held {
				rt.driveInputStep = step
			} else if step.preserveHeld != nil && !step.release {
				step.preserveHeld.position = step.position
				step.preserveHeld.modifiers = step.modifiers
				rt.driveInputStep = step.preserveHeld
			} else {
				rt.driveInputStep = nil
			}
			rt.notifyControlChanged()
		}
		return
	}
	if step.focusTarget != 0 && rt.focusManager != nil && rt.focusManager.Focused() == step.focusTarget {
		rt.finishInputStep(step, nil)
		return
	}
	if step.completed < len(step.events) {
		if rt.frameTimer != nil {
			rt.frameTimer.RequestFrame()
		}
		return
	}
	if step.focusTarget != 0 {
		rt.finishInputStep(step, &proto.Error{Code: proto.ErrTargetNotFocusable, Detail: "Tab traversal did not reach target within max_hops"})
		return
	}
	rt.finishInputStep(step, nil)
}

func (rt *Runtime) finishInputStep(step *driveInputStep, failure *proto.Error) {
	if rt.driveInputStep != step {
		return
	}
	if step.plan != nil && failure != nil {
		if step.completed > 0 {
			rt.pendingEvents = append(rt.pendingEvents, inputCancellationEvent(step)...)
		}
		step.plan.failure = failure
		rt.driveInputStep = nil
		rt.notifyControlChanged()
		return
	}
	if failure != nil && step.completed > 0 {
		rt.pendingEvents = append(rt.pendingEvents, inputCancellationEvent(step)...)
	}
	if failure == nil && step.held {
		rt.driveInputStep = step
	} else if step.preserveHeld != nil && failure == nil && !step.release {
		step.preserveHeld.position = step.position
		step.preserveHeld.modifiers = step.modifiers
		rt.driveInputStep = step.preserveHeld
	} else {
		rt.driveInputStep = nil
	}
	if rt.drivePort != nil && step.entry != nil {
		result := drive.Result{Frame: rt.drivePort.Frame(), Cut: rt.drivePort.Cut()}
		if failure != nil {
			result.Err = failure
		} else if step.held {
			payload, err := json.Marshal(struct {
				ConsumedAtoms int    `json:"consumed_atoms"`
				GestureHandle string `json:"gesture_handle"`
				Device        string `json:"device,omitempty"`
				AtomsPerFrame int    `json:"atoms_per_frame"`
			}{step.completed, step.gesture, step.device, 1})
			if err != nil {
				result.Err = &proto.Error{Code: proto.ErrInternalPanic, Detail: "gesture handle encoding failed"}
			} else {
				result.Payload = payload
			}
		} else if step.focusTarget != 0 {
			payload, err := json.Marshal(map[string]any{"focus_hops": step.completed / 2, "focused": strconv.FormatUint(uint64(step.focusTarget), 10)})
			if err != nil {
				result.Err = &proto.Error{Code: proto.ErrInternalPanic, Detail: "focus progress encoding failed"}
			} else {
				result.Payload = payload
			}
		} else {
			payload, err := json.Marshal(struct {
				ConsumedAtoms int    `json:"consumed_atoms"`
				Device        string `json:"device,omitempty"`
				AtomsPerFrame int    `json:"atoms_per_frame"`
			}{step.completed, step.device, 1})
			if err != nil {
				result.Err = &proto.Error{Code: proto.ErrInternalPanic, Detail: "input progress encoding failed"}
			} else {
				result.Payload = payload
			}
		}
		rt.drivePort.completeClaimed(step.entry, step.cmd, result)
		if step.held && failure == nil {
			step.entry = nil
		}
	}
	rt.notifyControlChanged()
}

func (rt *Runtime) ownedGesture(cmd drive.Command, handle string) (*driveInputStep, *proto.Error) {
	held := rt.driveInputStep
	if held == nil || !held.held || held.sessionID != cmd.Session || held.generation != cmd.Generation {
		return nil, &proto.Error{Code: proto.ErrInputPreempted, Detail: "no held gesture is owned by this session generation"}
	}
	if handle == "" || handle != held.gesture {
		return nil, &proto.Error{Code: proto.ErrInputPreempted, Detail: "gesture handle is invalid or expired"}
	}
	return held, nil
}

func (rt *Runtime) preemptDriveForHuman() {
	if rt.driveInputStep != nil {
		rt.preemptInputStep()
	}
	if rt.drivePort == nil {
		return
	}
	p := rt.drivePort
	reason := &proto.Error{Code: proto.ErrInputPreempted, Detail: "human input took control before the next scheduled step"}
	seenPlans := make(map[*framePlan]struct{})
	for i := 0; i < len(p.frameTokens); {
		token := p.frameTokens[i]
		p.frameTokens = append(p.frameTokens[:i], p.frameTokens[i+1:]...)
		if token.plan != nil {
			if _, seen := seenPlans[token.plan]; !seen {
				seenPlans[token.plan] = struct{}{}
				rt.finishFramePlan(token.plan, reason)
			}
		} else if token.entry != nil {
			p.completeFrameToken(token, drive.Result{Err: reason, Frame: p.Frame(), Cut: p.Cut()})
		}
	}
	for frame, token := range p.pendingFrameTokens {
		if token.plan != nil {
			if _, seen := seenPlans[token.plan]; !seen {
				seenPlans[token.plan] = struct{}{}
				rt.finishFramePlan(token.plan, reason)
			}
		} else if token.entry != nil {
			p.completeFrameToken(token, drive.Result{Err: reason, Frame: strconv.FormatUint(frame, 10), Cut: p.Cut()})
		}
		delete(p.pendingFrameTokens, frame)
	}
	if p.framePlan != nil {
		if _, seen := seenPlans[p.framePlan]; !seen {
			rt.finishFramePlan(p.framePlan, reason)
		}
	}
	for i := 0; i < len(p.pendingCommandActivations); {
		pending := p.pendingCommandActivations[i]
		p.pendingCommandActivations = append(p.pendingCommandActivations[:i], p.pendingCommandActivations[i+1:]...)
		p.finishCommand(pending, drive.Result{Err: reason, Frame: p.Frame(), Cut: p.Cut()})
	}
	for _, pending := range p.waitingCommandJobs {
		p.cancelCommandJobs(pending.jobScope)
		p.finishCommand(pending, drive.Result{Err: reason, Frame: p.Frame(), Cut: p.Cut()})
	}
	p.waitingCommandJobs = make(map[string]pendingCommand)
	p.releaseTimeOwner(true)
}

func isHumanInput(ev platform.Event) bool {
	return eventSource(ev) == platform.SourcePlatform && platform.IsInputEvent(ev)
}

func eventSource(ev platform.Event) platform.EventSource {
	return platform.EventSourceOf(ev)
}
