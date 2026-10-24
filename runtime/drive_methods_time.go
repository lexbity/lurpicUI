package runtime

import (
	"encoding/json"
	"time"

	"codeburg.org/lexbit/lurpicui/drive/proto"
	drive "codeburg.org/lexbit/lurpicui/drive/server"
)

// timeModeParams is the time.mode payload.
type timeModeParams struct {
	Mode string `json:"mode"` // "wall" | "virtual"
}

// driveTimeMode implements time.mode (FR-5): switches Wall↔Virtual with the
// `time` grant and the mid-gesture guard (QP-5).
func driveTimeMode(rt *Runtime, cmd drive.Command) driveOutcome {
	epoch := func() uint64 { return rt.driveSink.Epoch() }
	var p timeModeParams
	if len(cmd.Params) > 0 {
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "time.mode params: " + err.Error()}, epoch())
		}
	}
	switch p.Mode {
	case "wall":
		rt.clock.SetWall()
	case "virtual":
		if err := rt.clock.SetVirtual(); err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrTimeMode, Detail: err.Error()}, epoch())
		}
	default:
		return errorOutcome(&proto.Error{
			Code:   proto.ErrInternal,
			Detail: `time.mode mode must be "wall" or "virtual"`,
		}, epoch())
	}
	payload, err := json.Marshal(map[string]string{"mode": p.Mode})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, epoch())
	}
	return payloadOutcome(payload, epoch())
}

// timeAdvanceParams is the time.advance payload.
type timeAdvanceParams struct {
	D float64 `json:"d"` // seconds; MUST be > 0
}

// driveTimeAdvance implements time.advance (FR-5): defers the reply to after
// exactly one complete frame has executed on the advanced clock (QP-5).
func driveTimeAdvance(rt *Runtime, cmd drive.Command) driveOutcome {
	epoch := func() uint64 { return rt.driveSink.Epoch() }
	if rt.clock.Mode() != ClockVirtual {
		return errorOutcome(&proto.Error{
			Code:   proto.ErrTimeMode,
			Detail: "time.advance requires virtual mode (call time.mode first)",
		}, epoch())
	}
	var p timeAdvanceParams
	if len(cmd.Params) > 0 {
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "time.advance params: " + err.Error()}, epoch())
		}
	}
	if p.D <= 0 {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "time.advance d must be > 0"}, epoch())
	}
	rt.driveSink.pendingAdvances = append(rt.driveSink.pendingAdvances, pendingAdvance{
		d:   time.Duration(p.D * float64(time.Second)),
		cmd: cmd,
	})
	return driveOutcome{deferred: true}
}

// frameRunStep is one step (or then-read) of a frame transaction (§3.5).
type frameRunStep struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// frameRunAdvance is the advance policy: exactly one of frames / until.
type frameRunAdvance struct {
	Frames *int           `json:"frames,omitempty"`
	Until  *frameRunUntil `json:"until,omitempty"`
}

// frameRunUntil is the composable idle predicate; Max is MANDATORY (§3.5).
type frameRunUntil struct {
	Quiescent   *frameRunQuiescent `json:"quiescent,omitempty"`
	JobsDrained bool               `json:"jobsDrained,omitempty"`
	Max         int                `json:"max"`
}

type frameRunQuiescent struct {
	Frames int `json:"frames"`
}

type frameRunParams struct {
	Steps   []frameRunStep   `json:"steps"`
	Advance *frameRunAdvance `json:"advance,omitempty"`
	Then    []frameRunStep   `json:"then,omitempty"`
}

type frameRunErrorPayload struct {
	Applied *int   `json:"applied"`
	Frames  int    `json:"frames"`
	Detail  string `json:"detail,omitempty"`
}

// driveFrameRun implements frame.run (FR-17): steps apply one per cut, the
// advance policy runs bounded frames, `then` reads at the final cut. Virtual
// mode only — Wall mode's cuts are vsync-driven and cannot be stepped
// synchronously.
func driveFrameRun(rt *Runtime, cmd drive.Command) driveOutcome {
	epoch := func() uint64 { return rt.driveSink.Epoch() }
	if rt.clock.Mode() != ClockVirtual {
		return errorOutcome(&proto.Error{
			Code:   proto.ErrTimeMode,
			Detail: "frame.run requires virtual mode (call time.mode first)",
		}, epoch())
	}
	var p frameRunParams
	if len(cmd.Params) > 0 {
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: "frame.run params: " + err.Error()}, epoch())
		}
	}
	if p.Advance != nil {
		if p.Advance.Frames != nil && p.Advance.Until != nil {
			return errorOutcome(&proto.Error{
				Code:   proto.ErrInternal,
				Detail: "frame.run advance: frames and until are mutually exclusive",
			}, epoch())
		}
		if p.Advance.Until != nil {
			if p.Advance.Until.Max <= 0 {
				return errorOutcome(&proto.Error{
					Code:   proto.ErrInternal,
					Detail: "frame.run advance.until requires a positive max",
				}, epoch())
			}
			if p.Advance.Until.Quiescent != nil && p.Advance.Until.Quiescent.Frames <= 0 {
				return errorOutcome(&proto.Error{
					Code:   proto.ErrInternal,
					Detail: "frame.run advance.until.quiescent requires a positive frames count",
				}, epoch())
			}
			if p.Advance.Until.Quiescent == nil && !p.Advance.Until.JobsDrained {
				return errorOutcome(&proto.Error{
					Code:   proto.ErrInternal,
					Detail: "frame.run advance.until requires at least one predicate",
				}, epoch())
			}
		}
		if p.Advance.Frames != nil && *p.Advance.Frames < 0 {
			return errorOutcome(&proto.Error{
				Code:   proto.ErrInternal,
				Detail: "frame.run advance.frames must be >= 0",
			}, epoch())
		}
	}

	// Validate every step's method and grant before executing anything:
	// grant denials must not half-apply a transaction (FR-3).
	for _, step := range append(append([]frameRunStep{}, p.Steps...), p.Then...) {
		grant, known := drive.MethodGrant(step.Method)
		if !known {
			return errorOutcome(&proto.Error{
				Code:   proto.ErrInternal,
				Detail: "unknown method in frame.run: " + step.Method,
			}, epoch())
		}
		if grant != "" && !cmd.Grants.Allows(grant) {
			return errorOutcome(&proto.Error{
				Code:   proto.ErrGrant,
				Detail: grant + " (frame.run step " + step.Method + ")",
			}, epoch())
		}
	}

	framesRun := 0
	applied := -1
	for i, step := range p.Steps {
		res, perr := rt.driveExecStep(cmd, step)
		if perr != nil {
			return frameRunAbort(rt, applied, framesRun, perr)
		}
		_ = res
		applied = i
		// A plain step executes at a cut; running one frame crosses to the
		// next cut for the following step. Steps that already ran their own
		// frame (time.advance) crossed it themselves.
		if step.Method != "time.advance" {
			rt.runFrame(rt.clock.Now(), true)
			framesRun++
		} else {
			framesRun++
		}
	}

	if p.Advance != nil {
		switch {
		case p.Advance.Frames != nil:
			for i := 0; i < *p.Advance.Frames; i++ {
				rt.runFrame(rt.clock.Advance(rt.framePeriod()), true)
				framesRun++
			}
		case p.Advance.Until != nil:
			idle := rt.runUntilIdle(p.Advance.Until, &framesRun)
			if !idle {
				return errorOutcome(&proto.Error{
					Code: proto.ErrTimeMode,
					Detail: jsonDetail("advance budget exhausted before the idle predicate was satisfied",
						applied, framesRun),
				}, epoch())
			}
		}
	}

	thenResults := make([]json.RawMessage, 0, len(p.Then))
	for _, step := range p.Then {
		res, perr := rt.driveExecStep(cmd, step)
		if perr != nil {
			return frameRunAbort(rt, applied, framesRun, perr)
		}
		thenResults = append(thenResults, res)
	}

	payload, err := json.Marshal(map[string]any{
		"applied": applied,
		"frames":  framesRun,
		"then":    thenResults,
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, epoch())
	}
	return payloadOutcome(payload, rt.frameNumber)
}

// driveExecStep executes one frame.run step at the current cut and returns
// its payload. Grant and existence were pre-validated.
func (rt *Runtime) driveExecStep(cmd drive.Command, step frameRunStep) (json.RawMessage, *proto.Error) {
	if step.Method == "time.advance" {
		// As a step, an advance runs its frame inline (we are at a cut, not
		// inside a frame), so the deferral machinery does not apply.
		var p timeAdvanceParams
		if len(step.Params) > 0 {
			if err := json.Unmarshal(step.Params, &p); err != nil || p.D <= 0 {
				return nil, &proto.Error{Code: proto.ErrInternal, Detail: "time.advance step params invalid"}
			}
		} else {
			return nil, &proto.Error{Code: proto.ErrInternal, Detail: "time.advance step requires d"}
		}
		now := rt.clock.Advance(time.Duration(p.D * float64(time.Second)))
		rt.runFrame(now, true)
		payload, err := json.Marshal(map[string]uint64{"frame": rt.frameNumber})
		if err != nil {
			return nil, &proto.Error{Code: proto.ErrInternal, Detail: err.Error()}
		}
		return payload, nil
	}
	handler, ok := driveMethods[step.Method]
	if !ok {
		return nil, &proto.Error{Code: proto.ErrInternal, Detail: "unknown method: " + step.Method}
	}
	out := handler(rt, drive.Command{
		ID:     cmd.ID,
		Method: step.Method,
		Params: step.Params,
		Grants: cmd.Grants,
		Reply:  make(chan drive.Result, 1),
	})
	if out.deferred {
		return nil, &proto.Error{Code: proto.ErrInternal, Detail: "method cannot be deferred inside frame.run"}
	}
	if out.result.Err != nil {
		return nil, out.result.Err
	}
	return out.result.Payload, nil
}

// runUntilIdle runs frames (each advancing the clock by one nominal period)
// until the composable idle predicate holds, bounded by max. Predicates are
// evaluated at cuts only (§3.5).
func (rt *Runtime) runUntilIdle(u *frameRunUntil, framesRun *int) bool {
	needQuiet := 0
	if u.Quiescent != nil {
		needQuiet = u.Quiescent.Frames
	}
	quiet := 0
	for i := 0; i < u.Max; i++ {
		rt.runFrame(rt.clock.Advance(rt.framePeriod()), true)
		*framesRun++
		stats := rt.LastFrameStats()
		if stats.DirtyFacets == 0 && stats.ArrangeCount == 0 {
			quiet++
		} else {
			quiet = 0
		}
		if needQuiet > 0 && quiet < needQuiet {
			continue
		}
		if u.JobsDrained && rt.jobPool != nil && rt.jobPool.Pending() > 0 {
			continue
		}
		return true
	}
	return false
}

// framePeriod is the nominal virtual-frame duration used by frame.run's
// advance policies (1/targetFPS): deterministic by construction.
func (rt *Runtime) framePeriod() time.Duration {
	return time.Second / time.Duration(rt.config.TargetFPS)
}

func frameRunAbort(rt *Runtime, applied, framesRun int, perr *proto.Error) driveOutcome {
	payload, err := json.Marshal(frameRunErrorPayload{
		Applied: &applied,
		Frames:  framesRun,
		Detail:  perr.Detail,
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, rt.driveSink.Epoch())
	}
	return errorOutcome(&proto.Error{Code: perr.Code, Detail: string(payload)}, rt.driveSink.Epoch())
}

func jsonDetail(msg string, applied, frames int) string {
	b, err := json.Marshal(map[string]int{"applied": applied, "frames": frames})
	if err != nil {
		return msg
	}
	return msg + " " + string(b)
}
