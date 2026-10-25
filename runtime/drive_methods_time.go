package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/job"
)

// timeModeParams is the time.mode payload.
type timeModeParams struct {
	Mode string `json:"mode"` // "wall" | "virtual"
}

// driveTimeMode implements time.mode (FR-5): switches Wall↔Virtual with the
// `time` grant and the mid-gesture guard (QP-5).
func driveTimeMode(rt *Runtime, cmd drive.Command) driveOutcome {
	var p timeModeParams
	if err := decodeStrictDriveParams(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "time.mode requires {mode}"})
	}
	switch p.Mode {
	case "wall":
		if rt.clock.Mode() == ClockWall {
			if rt.drivePort != nil && rt.drivePort.ownsTime(cmd) {
				rt.drivePort.releaseTimeOwner(false)
			}
			return payloadOutcome([]byte(`{"mode":"wall","frames":"0"}`))
		}
		if !rt.drivePort.ownsTime(cmd) {
			return errorOutcome(&proto.Error{Code: proto.ErrTimeBusy, Detail: "session does not own virtual time"})
		}
		if rt.clock.GestureActive() {
			return errorOutcome(&proto.Error{Code: proto.ErrGestureActive, Detail: "mode transition refused during a gesture"})
		}
		if rt.renderPipeline == nil || !rt.isSurfaceReady() {
			return errorOutcome(&proto.Error{Code: proto.ErrRenderUnavailable, Detail: "mode transition requires a ready surface"})
		}
		rt.clock.SetWall()
		rt.notifyControlChanged()
	case "virtual":
		if rt.clock.Mode() == ClockVirtual {
			if perr := rt.drivePort.acquireTimeOwner(cmd); perr != nil {
				return errorOutcome(perr)
			}
			return payloadOutcome([]byte(`{"mode":"virtual","frames":"0"}`))
		}
		if rt.clock.GestureActive() {
			return errorOutcome(&proto.Error{Code: proto.ErrGestureActive, Detail: "mode transition refused during a gesture"})
		}
		if rt.renderPipeline == nil || !rt.isSurfaceReady() {
			return errorOutcome(&proto.Error{Code: proto.ErrRenderUnavailable, Detail: "mode transition requires a ready surface"})
		}
		if perr := rt.drivePort.acquireTimeOwner(cmd); perr != nil {
			return errorOutcome(perr)
		}
		if err := rt.clock.SetVirtual(); err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrTimeMode, Detail: err.Error()})
		}
		rt.notifyControlChanged()
	default:
		return errorOutcome(&proto.Error{
			Code:   proto.ErrInvalidParams,
			Detail: `time.mode mode must be "wall" or "virtual"`,
		})
	}
	return deferredMode(p.Mode)
}

// timeAdvanceParams is the time.advance payload.
type timeAdvanceParams struct {
	DurationNS string `json:"duration_ns"`
}

// driveTimeAdvance implements time.advance (FR-5): defers the reply to after
// exactly one complete frame has executed on the advanced clock (QP-5).
func driveTimeAdvance(rt *Runtime, cmd drive.Command) driveOutcome {
	if rt.clock.Mode() != ClockVirtual {
		return errorOutcome(&proto.Error{
			Code:   proto.ErrTimeMode,
			Detail: "time.advance requires virtual mode (call time.mode first)",
		})
	}
	if rt.renderPipeline == nil || !rt.isSurfaceReady() {
		return errorOutcome(&proto.Error{Code: proto.ErrRenderUnavailable, Detail: "time.advance requires a ready surface"})
	}
	var p timeAdvanceParams
	if err := decodeStrictDriveParams(cmd.Params, &p); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "time.advance requires duration_ns"})
	}
	ns, err := strconv.ParseUint(p.DurationNS, 10, 64)
	if err != nil || ns == 0 || ns > 1_000_000_000 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "duration_ns must be in [1,1000000000]"})
	}
	duration := time.Duration(ns)
	if !rt.clock.CanAdvance(duration) {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "duration_ns overflows engine time"})
	}
	if rt.drivePort != nil && !rt.drivePort.canQueueVirtualDuration(duration) {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "queued duration_ns overflows engine time"})
	}
	if !rt.drivePort.ownsTime(cmd) {
		if perr := rt.drivePort.acquireTimeOwner(cmd); perr != nil {
			return errorOutcome(perr)
		}
	}
	return deferredFrame(duration)
}

// frameRunStep is a typed scheduler step. S3 supports GUI-backed commands;
// input steps become available with their real scheduler implementation.
type frameRunStep struct {
	Method      string          `json:"method"`
	Params      json.RawMessage `json:"params"`
	BindGesture string          `json:"bind_gesture,omitempty"`
	UseGesture  string          `json:"use_gesture,omitempty"`
}

type frameRunAdvance struct {
	Frames *int           `json:"frames,omitempty"`
	Until  *frameRunUntil `json:"until,omitempty"`
}

type frameRunUntil struct {
	QuiescentFrames int  `json:"quiescent_frames,omitempty"`
	JobsDrained     bool `json:"jobs_drained,omitempty"`
	MaxFrames       int  `json:"max_frames"`
}

type frameRunParams struct {
	Steps          []frameRunStep   `json:"steps"`
	StepDurationNS string           `json:"step_duration_ns"`
	Advance        *frameRunAdvance `json:"advance,omitempty"`
	MaxFrames      int              `json:"max_frames"`
	Then           []frameRunStep   `json:"then,omitempty"`
}

type framePlan struct {
	cmd                drive.Command
	entry              *drive.Entry
	params             frameRunParams
	stepDuration       time.Duration
	stepIndex          int
	frames             int
	tailFrames         int
	tailStarted        bool
	quietFrames        int
	stepActive         bool
	waitingJobs        bool
	scope              *commandJobScope
	failure            *proto.Error
	thenResults        []json.RawMessage
	waiters            []drive.Command
	gestureBindings    map[string]string
	gestureReleased    map[string]bool
	inputAtomsComplete bool
}

func driveFrameRun(rt *Runtime, cmd drive.Command) driveOutcome {
	if !cmd.Grants.Time {
		return errorOutcome(&proto.Error{Code: proto.ErrGrant, Detail: "time (frame.run)"})
	}
	if rt.clock.Mode() != ClockVirtual {
		return errorOutcome(&proto.Error{Code: proto.ErrTimeMode, Detail: "frame.run requires virtual mode"})
	}
	if rt.renderPipeline == nil || !rt.isSurfaceReady() {
		return errorOutcome(&proto.Error{Code: proto.ErrRenderUnavailable, Detail: "frame.run requires a ready surface"})
	}
	if rt.drivePort == nil || rt.drivePort.framePlan != nil || len(rt.drivePort.pendingCommandActivations) != 0 || len(rt.drivePort.pendingCommandFrames) != 0 || len(rt.drivePort.waitingCommandJobs) != 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInputBusy, Detail: "frame plan requires an idle actuation scheduler"})
	}
	var params frameRunParams
	if err := decodeStrictDriveParams(cmd.Params, &params); err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "frame.run params are malformed"})
	}
	if params.MaxFrames < 1 || params.MaxFrames > 4096 || len(params.Steps) > 256 || len(params.Then) > 64 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "frame.run limits exceeded or max_frames is missing"})
	}
	durationNS, err := strconv.ParseInt(params.StepDurationNS, 10, 64)
	if err != nil || durationNS <= 0 || durationNS > 1_000_000_000 {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "step_duration_ns must be in [1,1000000000]"})
	}
	const maxInt64 = int64(1<<63 - 1)
	if durationNS > maxInt64/int64(params.MaxFrames) || !rt.clock.CanAdvance(time.Duration(durationNS*int64(params.MaxFrames))) {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "frame plan duration overflows engine time"})
	}
	if params.Advance != nil {
		if (params.Advance.Frames == nil) == (params.Advance.Until == nil) {
			return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "advance requires exactly one of frames or until"})
		}
		if params.Advance.Frames != nil && (*params.Advance.Frames < 0 || *params.Advance.Frames > params.MaxFrames) {
			return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "advance.frames is outside the total frame budget"})
		}
		if params.Advance.Until != nil {
			u := params.Advance.Until
			if u.MaxFrames < 1 || u.MaxFrames > params.MaxFrames || (u.QuiescentFrames <= 0 && !u.JobsDrained) {
				return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "advance.until requires a bounded predicate"})
			}
		}
	}
	if len(params.Steps) == 0 && params.Advance == nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "frame.run requires a command step or bounded advance"})
	}
	if rt.drivePort == nil || rt.drivePort.framePlan != nil || rt.driveInputStep != nil || len(rt.drivePort.pendingCommandActivations) != 0 || len(rt.drivePort.pendingCommandFrames) != 0 || len(rt.drivePort.waitingCommandJobs) != 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInputBusy, Detail: "frame plan requires an idle actuation scheduler"})
	}
	minimumFrames := 0
	bindings := make(map[string]bool)
	released := make(map[string]bool)
	for index, step := range params.Steps {
		switch {
		case step.Method == driveMethodCommandExecute:
			if step.BindGesture != "" || step.UseGesture != "" {
				return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "gesture metadata is valid only on input steps"})
			}
			if !cmd.Grants.Command {
				return errorOutcome(&proto.Error{Code: proto.ErrGrant, Detail: proto.GrantCommand + " (frame.run step cmd.execute)"})
			}
			out := driveCmdExecute(rt, drive.Command{Method: step.Method, Params: step.Params, Grants: cmd.Grants})
			if out.result.Err != nil || out.activation == nil {
				if out.result.Err != nil {
					var perr *proto.Error
					if errors.As(out.result.Err, &perr) {
						return errorOutcome(perr)
					}
				}
				return errorOutcome(&proto.Error{Code: proto.ErrCommandUnavailable})
			}
			minimumFrames++
		case strings.HasPrefix(step.Method, "input."):
			if !cmd.Grants.Input {
				return errorOutcome(&proto.Error{Code: proto.ErrGrant, Detail: proto.GrantInput + " (frame.run step " + step.Method + ")"})
			}
			handler, ok := driveMethods[step.Method]
			if !ok {
				return errorOutcome(&proto.Error{Code: proto.ErrUnknownMethod, Detail: "frame.run input method is unavailable"})
			}
			if step.UseGesture != "" {
				if step.Method != "input.move" && step.Method != driveMethodInputRelease {
					return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "use_gesture is only valid on input.move/input.release"})
				}
				if !bindings[step.UseGesture] || released[step.UseGesture] {
					return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "use_gesture must refer to an earlier unreleased press"})
				}
				if step.Method == driveMethodInputRelease {
					released[step.UseGesture] = true
				}
			} else if step.Method == driveMethodInputRelease && len(bindings) != 0 {
				return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.release requires use_gesture"})
			}
			if step.BindGesture != "" {
				if step.Method != "input.press" || bindings[step.BindGesture] || len(bindings) >= 16 {
					return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "bind_gesture must uniquely name input.press (maximum 16)"})
				}
				bindings[step.BindGesture] = true
			}
			if step.Method == "input.press" && step.BindGesture == "" {
				return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.press requires bind_gesture inside frame.run"})
			}
			if step.Method == driveMethodInputRelease && step.UseGesture == "" {
				return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.release requires use_gesture inside frame.run"})
			}
			validationParams := step.Params
			if step.UseGesture != "" {
				var params inputPointParams
				if err := decodeStrictDriveParams(step.Params, &params); err != nil {
					return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "gesture input params are malformed"})
				}
				params.Gesture = "plan-validation"
				encoded, err := json.Marshal(params)
				if err != nil {
					return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "gesture input params are malformed"})
				}
				validationParams = encoded
			}
			// Handler validation is side-effect-free with respect to application state;
			// its immutable atom list is discarded and rebuilt at the execution cut.
			if step.Method == driveMethodInputRelease {
				var params inputPointParams
				if err := decodeStrictDriveParams(step.Params, &params); err != nil {
					return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.release params are malformed"})
				}
				if (params.X == nil) != (params.Y == nil) {
					return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "release coordinates require both x and y"})
				}
				if params.Target != nil || params.X != nil {
					if _, _, perr := params.resolve(rt); perr != nil {
						return errorOutcome(perr)
					}
				}
				minimumFrames++
			} else if step.Method == "input.move" && step.UseGesture != "" {
				// A plan-local held gesture does not exist until its press executes.
				// Validate the point DTO directly; ownership is checked when scheduled.
				var params inputPointParams
				if err := decodeStrictDriveParams(step.Params, &params); err != nil {
					return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "input.move params are malformed"})
				}
				if _, _, perr := params.resolve(rt); perr != nil {
					return errorOutcome(perr)
				}
				minimumFrames++
			} else {
				out := handler(rt, drive.Command{Method: step.Method, Params: validationParams, Grants: cmd.Grants, Session: cmd.Session, Generation: cmd.Generation})
				if out.result.Err != nil || out.inputStep == nil {
					var perr *proto.Error
					if out.result.Err != nil && errors.As(out.result.Err, &perr) {
						return errorOutcome(perr)
					}
					return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "frame.run input step did not produce an atom sequence"})
				}
				minimumFrames += len(out.inputStep.events)
			}
			if step.BindGesture != "" && (index == len(params.Steps)-1 || !gestureReleasedLater(params.Steps, index, step.BindGesture)) {
				return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "every bound gesture must be released later in the plan"})
			}
		default:
			return errorOutcome(&proto.Error{Code: proto.ErrUnknownMethod, Detail: "frame.run supports GUI-backed command and input steps only"})
		}
	}
	for name := range bindings {
		if !released[name] {
			return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "every bound gesture must be released before plan completion"})
		}
	}
	if params.Advance != nil {
		if params.Advance.Frames != nil {
			minimumFrames += *params.Advance.Frames
		} else {
			minimumFrames += params.Advance.Until.MaxFrames
		}
	}
	if minimumFrames > params.MaxFrames {
		return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "max_frames is smaller than the declared plan budget"})
	}
	for _, read := range params.Then {
		spec, ok := registrySpec(read.Method)
		if !ok || spec.Mutating || read.Method == "op.status" {
			return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "frame.run then accepts registered read methods only"})
		}
		if spec.Grant == proto.GrantRead && !cmd.Grants.Read {
			return errorOutcome(&proto.Error{Code: proto.ErrGrant, Detail: "read (frame.run then " + read.Method + ")"})
		}
		if _, ok := driveMethods[read.Method]; !ok {
			return errorOutcome(&proto.Error{Code: proto.ErrUnknownMethod, Detail: "frame.run then method is not implemented"})
		}
	}
	plan := &framePlan{params: params, stepDuration: time.Duration(durationNS), gestureBindings: make(map[string]string), gestureReleased: make(map[string]bool)}
	if params.Advance != nil && len(params.Steps) == 0 {
		if params.Advance.Frames != nil && *params.Advance.Frames == 0 {
			return errorOutcome(&proto.Error{Code: proto.ErrInvalidParams, Detail: "a frame plan with no command steps requires at least one frame"})
		}
		plan.tailStarted = true
		if params.Advance.Frames != nil {
			plan.tailFrames = *params.Advance.Frames - 1
		}
	}
	if !rt.drivePort.ownsTime(cmd) {
		if perr := rt.drivePort.acquireTimeOwner(cmd); perr != nil {
			return errorOutcome(perr)
		}
	}
	return driveOutcome{deferred: true, frameToken: &pendingFrameToken{plan: plan, duration: plan.stepDuration}}
}

func gestureReleasedLater(steps []frameRunStep, pressIndex int, name string) bool {
	for index := pressIndex + 1; index < len(steps); index++ {
		if steps[index].Method == driveMethodInputRelease && steps[index].UseGesture == name {
			return true
		}
	}
	return false
}

func (rt *Runtime) driveActivatePendingPlanStep() {
	p := rt.drivePort
	if p == nil || p.currentFramePlan == nil {
		return
	}
	plan := p.currentFramePlan
	if plan.failure != nil || plan.waitingJobs || plan.stepIndex >= len(plan.params.Steps) || plan.stepActive {
		return
	}
	step := plan.params.Steps[plan.stepIndex]
	if strings.HasPrefix(step.Method, "input.") {
		return // input atoms are installed before phase 2 by the frame scheduler.
	}
	activation := driveCmdExecute(rt, drive.Command{Method: step.Method, Params: step.Params, Grants: plan.cmd.Grants})
	if activation.result.Err != nil || activation.activation == nil {
		plan.failure = &proto.Error{Code: proto.ErrCommandUnavailable, Detail: "command became unavailable before frame plan activation"}
		return
	}
	plan.scope = &commandJobScope{operationID: plan.entry.OpID, jobs: make(map[job.JobID]struct{})}
	plan.scope.active.Store(true)
	rt.currentDriveJobScope = plan.scope
	invoked := false
	var panicValue any
	func() {
		defer func() { panicValue = recover() }()
		registry := rt.CommandRegistry()
		if registry != nil {
			invoked = registry.ExecuteVersion(activation.activation.name, activation.activation.version)
		}
	}()
	rt.currentDriveJobScope = nil
	if panicValue != nil {
		plan.failure = &proto.Error{Code: proto.ErrInternalPanic, Detail: "frame plan command panicked; effect is unknown_partial"}
		plan.scope.active.Store(false)
		return
	}
	if !invoked {
		plan.failure = &proto.Error{Code: proto.ErrCommandUnavailable, Detail: "command became unavailable before frame plan activation"}
		plan.scope.active.Store(false)
		return
	}
	if plan.scope.failure != nil {
		plan.failure = plan.scope.failure
	}
	plan.stepActive = true
}

func (rt *Runtime) drivePreparePlanInputStep(plan *framePlan) {
	if plan == nil || plan.failure != nil || plan.stepActive || plan.stepIndex >= len(plan.params.Steps) {
		return
	}
	step := plan.params.Steps[plan.stepIndex]
	if !strings.HasPrefix(step.Method, "input.") {
		return
	}
	params := append(json.RawMessage(nil), step.Params...)
	if step.UseGesture != "" {
		handle := plan.gestureBindings[step.UseGesture]
		if handle == "" || plan.gestureReleased[step.UseGesture] {
			plan.failure = &proto.Error{Code: proto.ErrInputPreempted, Detail: "frame plan gesture binding is unavailable"}
			return
		}
		var decoded inputPointParams
		if err := decodeStrictDriveParams(params, &decoded); err != nil {
			plan.failure = &proto.Error{Code: proto.ErrInvalidParams, Detail: "frame plan gesture params are malformed"}
			return
		}
		decoded.Gesture = handle
		encoded, err := json.Marshal(decoded)
		if err != nil {
			plan.failure = &proto.Error{Code: proto.ErrInternalPanic, Detail: "frame plan gesture params could not be encoded"}
			return
		}
		params = encoded
	}
	out := driveMethods[step.Method](rt, drive.Command{Session: plan.cmd.Session, Generation: plan.cmd.Generation, OpID: plan.cmd.OpID, Method: step.Method, Params: params, Grants: plan.cmd.Grants})
	if out.result.Err != nil || out.inputStep == nil {
		var perr *proto.Error
		if out.result.Err != nil && errors.As(out.result.Err, &perr) {
			plan.failure = perr
		} else {
			plan.failure = &proto.Error{Code: proto.ErrInvalidParams, Detail: "frame plan input step could not be scheduled"}
		}
		return
	}
	inputStep := out.inputStep
	inputStep.plan = plan
	inputStep.cmd = plan.cmd
	if step.BindGesture != "" {
		if !inputStep.held || inputStep.gesture == "" {
			plan.failure = &proto.Error{Code: proto.ErrInternalPanic, Detail: "input.press did not create its declared gesture handle"}
			return
		}
		plan.gestureBindings[step.BindGesture] = inputStep.gesture
	}
	if step.Method == driveMethodInputRelease && step.UseGesture != "" {
		plan.gestureReleased[step.UseGesture] = true
	}
	plan.inputAtomsComplete = false
	plan.stepActive = true
	rt.beginInputStep(inputStep)
}

func (rt *Runtime) driveCompletePlanFrame(plan *framePlan, completion RenderCompletion) {
	p := rt.drivePort
	if p == nil || p.framePlan != plan {
		return
	}
	plan.frames++
	if completion.Err != nil || !completion.Ready {
		detail := "frame plan render acknowledgment failed"
		if completion.Err != nil {
			detail = completion.Err.Error()
		}
		rt.finishFramePlan(plan, &proto.Error{Code: proto.ErrRenderFailed, Detail: detail})
		return
	}
	if plan.failure != nil {
		rt.finishFramePlan(plan, plan.failure)
		return
	}
	if plan.stepIndex < len(plan.params.Steps) {
		if strings.HasPrefix(plan.params.Steps[plan.stepIndex].Method, "input.") {
			if !plan.inputAtomsComplete {
				if !rt.queuePlanFrame(plan) {
					return
				}
				return
			}
			plan.stepIndex++
			plan.stepActive = false
			plan.inputAtomsComplete = false
			if plan.stepIndex < len(plan.params.Steps) {
				if !rt.queuePlanFrame(plan) {
					return
				}
				return
			}
		} else {
			if plan.scope != nil && plan.scope.failure != nil {
				rt.finishFramePlan(plan, plan.scope.failure)
				return
			}
			if plan.scope != nil && len(plan.scope.jobs) > 0 {
				plan.waitingJobs = true
				if !rt.queuePlanFrame(plan) {
					return
				}
				return
			}
			plan.stepIndex++
			plan.stepActive = false
			plan.waitingJobs = false
			if plan.scope != nil {
				plan.scope.active.Store(false)
			}
			plan.scope = nil
			if plan.stepIndex < len(plan.params.Steps) {
				if !rt.queuePlanFrame(plan) {
					return
				}
				return
			}
		}
	}
	if plan.params.Advance != nil {
		if until := plan.params.Advance.Until; until != nil {
			if !plan.tailStarted {
				plan.tailStarted = true
				if !rt.queuePlanFrame(plan) {
					return
				}
				return
			}
			plan.tailFrames++
			stats := rt.LastFrameStats()
			if stats.DirtyFacets == 0 && stats.ArrangeCount == 0 {
				plan.quietFrames++
			} else {
				plan.quietFrames = 0
			}
			quiet := until.QuiescentFrames == 0 || plan.quietFrames >= until.QuiescentFrames
			jobs := !until.JobsDrained || rt.jobPool == nil || rt.jobPool.Pending() == 0
			if !quiet || !jobs {
				if plan.tailFrames >= until.MaxFrames {
					rt.finishFramePlan(plan, plan.budgetError("until predicate exhausted its frame budget"))
					return
				}
				if !rt.queuePlanFrame(plan) {
					return
				}
				return
			}
		} else {
			if !plan.tailStarted {
				plan.tailStarted = true
				plan.tailFrames = *plan.params.Advance.Frames
			}
			if plan.tailFrames > 0 {
				plan.tailFrames--
				if !rt.queuePlanFrame(plan) {
					return
				}
				return
			}
		}
	}
	for _, read := range plan.params.Then {
		out := driveMethods[read.Method](rt, drive.Command{Session: plan.cmd.Session, Generation: plan.cmd.Generation, Method: read.Method, Params: read.Params, Grants: plan.cmd.Grants})
		if out.deferred || out.result.Err != nil {
			var perr *proto.Error
			if out.result.Err != nil {
				errors.As(out.result.Err, &perr)
			}
			if perr == nil {
				perr = &proto.Error{Code: proto.ErrInternalPanic, Detail: "then read unexpectedly deferred"}
			}
			rt.finishFramePlan(plan, perr)
			return
		}
		plan.thenResults = append(plan.thenResults, out.result.Payload)
	}
	payload, err := json.Marshal(map[string]any{
		"state": "completed", "applied_steps": plan.stepIndex,
		"frames": strconv.Itoa(plan.frames), "then": plan.thenResults,
		"render_status": "ready", "ready_frame": p.Frame(),
	})
	if err != nil {
		rt.finishFramePlan(plan, &proto.Error{Code: proto.ErrInternalPanic, Detail: "frame plan result encoding failed"})
		return
	}
	rt.finishFramePlan(plan, nil, payload)
}

func (p *framePlan) budgetError(detail string) *proto.Error {
	data, err := json.Marshal(struct {
		AppliedSteps int    `json:"applied_steps"`
		Frames       string `json:"frames"`
		Detail       string `json:"detail"`
	}{AppliedSteps: p.stepIndex, Frames: strconv.Itoa(p.frames), Detail: detail})
	if err != nil {
		return &proto.Error{Code: proto.ErrInternalPanic, Detail: "frame plan progress encoding failed"}
	}
	return &proto.Error{Code: proto.ErrTimeMode, Detail: string(data)}
}

func (rt *Runtime) queuePlanFrame(plan *framePlan) bool {
	if plan.frames >= plan.params.MaxFrames {
		rt.finishFramePlan(plan, plan.budgetError("total max_frames exhausted"))
		return false
	}
	frontier := rt.jobPool.Frontier()
	rt.drivePort.frameTokens = append(rt.drivePort.frameTokens, pendingFrameToken{
		cmd: plan.cmd, entry: plan.entry, duration: plan.stepDuration, plan: plan,
		frontier: frontier, frontierSet: true,
	})
	select {
	case rt.drivePort.wake <- struct{}{}:
	default:
	}
	return true
}

func (rt *Runtime) finishFramePlan(plan *framePlan, failure *proto.Error, payloads ...json.RawMessage) {
	if rt.drivePort == nil || rt.drivePort.framePlan != plan {
		return
	}
	if failure != nil && rt.driveInputStep != nil && rt.driveInputStep.plan == plan {
		rt.preemptInputStep()
	}
	if plan.scope != nil {
		if failure != nil {
			rt.drivePort.cancelCommandJobs(plan.scope)
		} else {
			plan.scope.active.Store(false)
		}
	}
	rt.drivePort.framePlan = nil
	if failure != nil {
		result := drive.Result{Err: failure, Frame: rt.drivePort.Frame(), Cut: rt.drivePort.Cut()}
		rt.drivePort.completeClaimed(plan.entry, plan.cmd, result)
		for _, waiter := range plan.waiters {
			select {
			case waiter.Reply <- result:
			default:
			}
		}
		return
	}
	result := drive.Result{Payload: payloads[0], Frame: rt.drivePort.Frame(), Cut: rt.drivePort.Cut()}
	rt.drivePort.completeClaimed(plan.entry, plan.cmd, result)
	for _, waiter := range plan.waiters {
		select {
		case waiter.Reply <- result:
		default:
		}
	}
}

func (p *drivePort) canQueueVirtualDuration(d time.Duration) bool {
	if p == nil || d <= 0 {
		return false
	}
	projected := p.rt.clock.Now()
	for _, token := range p.frameTokens {
		if token.duration <= 0 {
			continue
		}
		current := projected.UnixNano()
		if current > int64(^uint64(0)>>1)-int64(token.duration) {
			return false
		}
		projected = projected.Add(token.duration)
	}
	current := projected.UnixNano()
	return current <= int64(^uint64(0)>>1)-int64(d)
}

func decodeStrictDriveParams(data []byte, out any) error {
	if len(data) == 0 {
		return errors.New("empty params")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return errors.New("trailing JSON value")
	}
	return nil
}
