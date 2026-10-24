package runtime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive/proto"
	drive "codeburg.org/lexbit/lurpicui/drive/server"
)

func allGrants() drive.Grants {
	return drive.Grants{Read: true, Input: true, Write: true, Time: true, Admin: true}
}

// driveCall enqueues a command and runs frames until the runtime thread has
// replied (the manual-frame equivalent of the socket round trip).
func driveCall(t *testing.T, rt *Runtime, method string, params any, grants drive.Grants) drive.Result {
	t.Helper()
	var raw json.RawMessage
	if params != nil {
		enc, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
		raw = enc
	}
	cmd := drive.Command{ID: 1, Method: method, Params: raw, Grants: grants, Reply: make(chan drive.Result, 1)}
	rt.DriveSink().Enqueue(cmd)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 128; i++ {
		rt.RunOneFrameAt(base.Add(time.Duration(i) * time.Second))
		select {
		case res := <-cmd.Reply:
			return res
		default:
		}
	}
	t.Fatalf("drive %q did not reply within 128 frames", method)
	return drive.Result{}
}

func TestDrive_TimeModeVirtualAndBack(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants())
	if res.Err != nil {
		t.Fatalf("time.mode virtual: %v", res.Err)
	}
	if rt.Clock().Mode() != ClockVirtual {
		t.Fatal("clock must be virtual")
	}
	res = driveCall(t, rt, "time.mode", map[string]string{"mode": "wall"}, allGrants())
	if res.Err != nil {
		t.Fatalf("time.mode wall: %v", res.Err)
	}
	if rt.Clock().Mode() != ClockWall {
		t.Fatal("clock must be wall")
	}
	// Unknown mode is a typed error, not a panic.
	res = driveCall(t, rt, "time.mode", map[string]string{"mode": "quantum"}, allGrants())
	if res.Err == nil || res.Err.Code != proto.ErrInternal {
		t.Fatalf("err = %+v", res.Err)
	}
}

func TestDrive_TimeModeRefusesMidGesture(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	// Simulate a mid-gesture platform pointer via the guard the input phase
	// installs; the clock consults it on every virtual switch (QP-5).
	rt.Clock().SetGestureGuard(func() bool { return true })
	res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants())
	if res.Err == nil || res.Err.Code != proto.ErrTimeMode {
		t.Fatalf("err = %+v, want time.mode", res.Err)
	}
	if rt.Clock().Mode() != ClockWall {
		t.Fatal("mode must remain wall after refusal")
	}
}

func TestDrive_TimeModeGrantDenied(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	_, _, beforeDenied, _ := rt.DriveStats().Snapshot()
	res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, drive.Grants{Read: true})
	if res.Err == nil || res.Err.Code != proto.ErrGrant {
		t.Fatalf("err = %+v, want grant.denied", res.Err)
	}
	if _, _, denied, _ := rt.DriveStats().Snapshot(); denied != beforeDenied+1 {
		t.Fatalf("grant-denied counter = %d, want %d", denied, beforeDenied+1)
	}
	if rt.Clock().Mode() != ClockWall {
		t.Fatal("denied command must not execute")
	}
}

func TestDrive_AdvanceRequiresVirtual(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	res := driveCall(t, rt, "time.advance", map[string]float64{"d": 0.016}, allGrants())
	if res.Err == nil || res.Err.Code != proto.ErrTimeMode {
		t.Fatalf("err = %+v, want time.mode", res.Err)
	}
}

func TestDrive_AdvanceRunsExactlyOneFrame(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	rt.RunOneFrameAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); res.Err != nil {
		t.Fatalf("time.mode: %v", res.Err)
	}
	beforeFrame := rt.frameNumber
	beforeNow := rt.Clock().Now()

	// Manual advance: enqueue, then one RunOneFrameAt executes the wake
	// frame (drain defers the reply) and the post-frame advance.
	cmd := drive.Command{
		ID:     7,
		Method: "time.advance",
		Params: json.RawMessage(`{"d":0.5}`),
		Grants: allGrants(),
		Reply:  make(chan drive.Result, 1),
	}
	rt.DriveSink().Enqueue(cmd)
	rt.RunOneFrameAt(rt.Clock().Now())
	select {
	case res := <-cmd.Reply:
		if res.Err != nil {
			t.Fatalf("advance: %v", res.Err)
		}
		var payload struct {
			Frame uint64 `json:"frame"`
		}
		if err := json.Unmarshal(res.Payload, &payload); err != nil {
			t.Fatalf("payload: %v", err)
		}
		if payload.Frame != rt.frameNumber {
			t.Fatalf("reply frame = %d, want the just-submitted frame %d", payload.Frame, rt.frameNumber)
		}
	default:
		t.Fatal("advance reply missing after its frame")
	}
	if rt.frameNumber != beforeFrame+2 {
		t.Fatalf("frames run = %d, want 2 (wake + advance)", rt.frameNumber-beforeFrame)
	}
	if got := rt.Clock().Now(); !got.Equal(beforeNow.Add(500 * time.Millisecond)) {
		t.Fatalf("clock after advance = %v, want %v", got, beforeNow.Add(500*time.Millisecond))
	}
}

func TestDrive_AdvanceRejectsNonPositive(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); res.Err != nil {
		t.Fatalf("time.mode: %v", res.Err)
	}
	res := driveCall(t, rt, "time.advance", map[string]float64{"d": 0}, allGrants())
	if res.Err == nil || !strings.Contains(res.Err.Detail, "must be > 0") {
		t.Fatalf("err = %+v", res.Err)
	}
}

func TestDrive_FrameRunRequiresVirtual(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	res := driveCall(t, rt, "frame.run", map[string]any{"steps": []any{}}, allGrants())
	if res.Err == nil || res.Err.Code != proto.ErrTimeMode {
		t.Fatalf("err = %+v, want time.mode", res.Err)
	}
}

func TestDrive_FrameRunStepsAdvanceThen(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); res.Err != nil {
		t.Fatalf("time.mode: %v", res.Err)
	}
	beforeFrame := rt.frameNumber
	res := driveCall(t, rt, "frame.run", map[string]any{
		"steps": []map[string]any{
			{"method": "sys.ping"},
			{"method": "time.advance", "params": map[string]float64{"d": 0.1}},
		},
		"advance": map[string]any{"frames": 3},
		"then":    []map[string]any{{"method": "sys.ping"}},
	}, allGrants())
	if res.Err != nil {
		t.Fatalf("frame.run: %v (%s)", res.Err, res.Err.Detail)
	}
	var payload struct {
		Applied int               `json:"applied"`
		Frames  int               `json:"frames"`
		Then    []json.RawMessage `json:"then"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatalf("payload %s: %v", res.Payload, err)
	}
	if payload.Applied != 1 {
		t.Fatalf("applied = %d, want 1", payload.Applied)
	}
	// step 1 (ping) runs its own frame; step 2 (advance) runs one frame;
	// the advance policy runs 3 → 5 frames total.
	if payload.Frames != 5 {
		t.Fatalf("frames = %d, want 5", payload.Frames)
	}
	// The manual harness path runs one carrying frame (the drain host) plus
	// the transaction's own frames.
	if rt.frameNumber != beforeFrame+6 {
		t.Fatalf("frames run = %d, want 6 (carrying frame + 5)", rt.frameNumber-beforeFrame)
	}
	if len(payload.Then) != 1 {
		t.Fatalf("then results = %d", len(payload.Then))
	}
}

func TestDrive_FrameRunValidation(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); res.Err != nil {
		t.Fatalf("time.mode: %v", res.Err)
	}
	cases := []struct {
		name   string
		params map[string]any
		code   proto.ErrorCode
		detail string
	}{
		{
			name:   "until without max",
			params: map[string]any{"advance": map[string]any{"until": map[string]any{"quiescent": map[string]int{"frames": 2}}}},
			code:   proto.ErrInternal,
			detail: "positive max",
		},
		{
			name:   "frames and until both",
			params: map[string]any{"advance": map[string]any{"frames": 1, "until": map[string]any{"max": 5}}},
			code:   proto.ErrInternal,
			detail: "mutually exclusive",
		},
		{
			name:   "unknown step method",
			params: map[string]any{"steps": []map[string]any{{"method": "no.such"}}},
			code:   proto.ErrInternal,
			detail: "unknown method",
		},
		{
			name:   "until without predicates",
			params: map[string]any{"advance": map[string]any{"until": map[string]any{"max": 5}}},
			code:   proto.ErrInternal,
			detail: "at least one predicate",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := driveCall(t, rt, "frame.run", tc.params, allGrants())
			if res.Err == nil || res.Err.Code != tc.code {
				t.Fatalf("err = %+v, want %s", res.Err, tc.code)
			}
			if !strings.Contains(res.Err.Detail, tc.detail) {
				t.Fatalf("detail %q must contain %q", res.Err.Detail, tc.detail)
			}
		})
	}
}

func TestDrive_FrameRunGrantDeniedPerStep(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, drive.Grants{Time: true}); res.Err != nil {
		t.Fatalf("time.mode: %v", res.Err)
	}
	// A step needing `read` under a connection without it aborts the run
	// before anything executes (FR-3).
	res := driveCall(t, rt, "frame.run", map[string]any{
		"steps": []map[string]any{{"method": "sys.ping"}},
	}, drive.Grants{Time: true})
	if res.Err == nil || res.Err.Code != proto.ErrGrant {
		t.Fatalf("err = %+v, want grant.denied", res.Err)
	}
	if !strings.Contains(res.Err.Detail, "sys.ping") {
		t.Fatalf("detail %q must name the denied step", res.Err.Detail)
	}
}

func TestDrive_FrameRunUntilQuiescent(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); res.Err != nil {
		t.Fatalf("time.mode: %v", res.Err)
	}
	res := driveCall(t, rt, "frame.run", map[string]any{
		"advance": map[string]any{
			"until": map[string]any{
				"quiescent":   map[string]int{"frames": 2},
				"jobsDrained": true,
				"max":         30,
			},
		},
	}, allGrants())
	if res.Err != nil {
		t.Fatalf("frame.run: %v", res.Err)
	}
	var payload struct {
		Frames int `json:"frames"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload.Frames > 30 {
		t.Fatalf("frames = %d exceeds max 30", payload.Frames)
	}
}

func TestDrive_FrameRunUntilBudgetExhaustion(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); res.Err != nil {
		t.Fatalf("time.mode: %v", res.Err)
	}
	// A quiescence demand the tree can never satisfy inside a tiny budget
	// fails loudly with the applied/frames report (§3.5).
	res := driveCall(t, rt, "frame.run", map[string]any{
		"steps": []map[string]any{{"method": "sys.ping"}},
		"advance": map[string]any{
			"until": map[string]any{
				"quiescent": map[string]int{"frames": 2},
				"max":       1,
			},
		},
	}, allGrants())
	if res.Err == nil || res.Err.Code != proto.ErrTimeMode {
		t.Fatalf("err = %+v, want time.mode budget error", res.Err)
	}
	if !strings.Contains(res.Err.Detail, "applied") {
		t.Fatalf("detail %q must carry the applied/frames report", res.Err.Detail)
	}
}
