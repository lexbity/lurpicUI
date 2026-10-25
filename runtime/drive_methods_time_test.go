package runtime

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
)

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
	if resultCode(res.Err) != string(proto.ErrInvalidParams) {
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
	if resultCode(res.Err) != string(proto.ErrTimeMode) {
		t.Fatalf("err = %+v, want time.mode", res.Err)
	}
	if rt.Clock().Mode() != ClockWall {
		t.Fatal("mode must remain wall after refusal")
	}
}

func TestDrive_TimeModeGrantDenied(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	beforeDenied := rt.DriveStats().Load().GrantDenied
	res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, drive.Grants{Read: true})
	if resultCode(res.Err) != string(proto.ErrGrant) {
		t.Fatalf("err = %+v, want grant.denied", res.Err)
	}
	if denied := rt.DriveStats().Load().GrantDenied; denied != beforeDenied+1 {
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
	if resultCode(res.Err) != string(proto.ErrTimeMode) {
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

	// Manual advance: submit, then one RunOneFrameAt executes the wake
	// frame (drain defers the reply) and the post-frame advance. Admission
	// claims the next contiguous ledger ID.
	client := testClientFor(t, rt)
	client.next++
	cmd := drive.Command{
		Session:    client.sess.ID,
		Generation: client.sess.Generation(),
		ID:         client.next,
		Method:     "time.advance",
		Params:     json.RawMessage(`{"d":0.5}`),
		Grants:     allGrants(),
		Reply:      make(chan drive.Result, 1),
	}
	if err := rt.DrivePort().Submit(cmd); err != nil {
		t.Fatalf("submit: %v", err)
	}
	rt.RunOneFrameAt(rt.Clock().Now())
	select {
	case res := <-cmd.Reply:
		if res.Err != nil {
			t.Fatalf("advance: %v", res.Err)
		}
		var payload struct {
			Frame string `json:"frame"`
		}
		if err := json.Unmarshal(res.Payload, &payload); err != nil {
			t.Fatalf("payload: %v", err)
		}
		if payload.Frame != strconv.FormatUint(rt.frameNumber, 10) {
			t.Fatalf("reply frame = %s, want the just-submitted frame %d", payload.Frame, rt.frameNumber)
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
	if res.Err == nil || !strings.Contains(errDetail(res.Err), "must be > 0") {
		t.Fatalf("err = %+v", res.Err)
	}
}

func TestDrive_FrameRunRequiresVirtual(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	res := driveCall(t, rt, "frame.run", map[string]any{"steps": []any{}}, allGrants())
	if resultCode(res.Err) != string(proto.ErrTimeMode) {
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
		t.Fatalf("frame.run: %v (%s)", res.Err, errDetail(res.Err))
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
			code:   proto.ErrInvalidParams,
			detail: "positive max",
		},
		{
			name:   "frames and until both",
			params: map[string]any{"advance": map[string]any{"frames": 1, "until": map[string]any{"max": 5}}},
			code:   proto.ErrInvalidParams,
			detail: "mutually exclusive",
		},
		{
			name:   "unknown step method",
			params: map[string]any{"steps": []map[string]any{{"method": "no.such"}}},
			code:   proto.ErrInvalidParams,
			detail: "unknown method",
		},
		{
			name:   "until without predicates",
			params: map[string]any{"advance": map[string]any{"until": map[string]any{"max": 5}}},
			code:   proto.ErrInvalidParams,
			detail: "at least one predicate",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := driveCall(t, rt, "frame.run", tc.params, allGrants())
			if resultCode(res.Err) != string(tc.code) {
				t.Fatalf("err = %+v, want %s", res.Err, tc.code)
			}
			if !strings.Contains(errDetail(res.Err), tc.detail) {
				t.Fatalf("detail %q must contain %q", errDetail(res.Err), tc.detail)
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
	if resultCode(res.Err) != string(proto.ErrGrant) {
		t.Fatalf("err = %+v, want grant.denied", res.Err)
	}
	if !strings.Contains(errDetail(res.Err), "sys.ping") {
		t.Fatalf("detail %q must name the denied step", errDetail(res.Err))
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
	if resultCode(res.Err) != string(proto.ErrTimeMode) {
		t.Fatalf("err = %+v, want time.mode budget error", res.Err)
	}
	if !strings.Contains(errDetail(res.Err), "applied") {
		t.Fatalf("detail %q must carry the applied/frames report", errDetail(res.Err))
	}
}
