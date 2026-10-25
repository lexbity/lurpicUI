package runtime

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/platform"
)

func TestDrive_TimeModeTransitionsOwnExactlyOneFrame(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	before := rt.frameNumber
	res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants())
	if res.Err != nil {
		t.Fatalf("time.mode virtual: %v", res.Err)
	}
	var transition struct {
		Mode       string `json:"mode"`
		Frame      string `json:"frame"`
		ReadyFrame string `json:"ready_frame"`
		Status     string `json:"render_status"`
	}
	if err := json.Unmarshal(res.Payload, &transition); err != nil {
		t.Fatal(err)
	}
	if transition.Mode != "virtual" || transition.Frame == "" || transition.Frame != transition.ReadyFrame || transition.Status != "ready" {
		t.Fatalf("virtual transition did not identify its ready surface: %+v (%s)", transition, res.Payload)
	}
	if rt.Clock().Mode() != ClockVirtual || rt.frameNumber != before+1 {
		t.Fatalf("virtual transition mode/frame = %s/%d, want virtual/%d", rt.Clock().Mode(), rt.frameNumber, before+1)
	}
	before = rt.frameNumber
	res = driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants())
	if res.Err != nil || rt.frameNumber != before {
		t.Fatalf("idempotent transition result=%v frames=%d want no frame", res.Err, rt.frameNumber-before)
	}
	res = driveCall(t, rt, "time.mode", map[string]string{"mode": "wall"}, allGrants())
	if res.Err != nil || rt.Clock().Mode() != ClockWall || rt.frameNumber != before+1 {
		t.Fatalf("wall transition result=%v mode=%s frames=%d", res.Err, rt.Clock().Mode(), rt.frameNumber-before)
	}
	if res = driveCall(t, rt, "time.mode", map[string]string{"mode": "quantum"}, allGrants()); resultCode(res.Err) != string(proto.ErrInvalidParams) {
		t.Fatalf("unknown mode error = %+v", res.Err)
	}
}

func TestVirtualEntryTransitionFrameReady(t *testing.T) {
	prepared, err := PrepareClock(ClockConfig{InitialMode: ClockVirtual, EpochNS: 1_800_000_000_000_000_000})
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Drive.Enabled = true
	testSessionTable(t, &cfg)
	cfg.PreparedClock = prepared
	rt, err := New(cfg, nil, &testWindow{width: 800, height: 600}, &backendFixture{}, buildBenchmarkTree())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- rt.Run() }()
	defer func() { rt.Shutdown(); <-done }()
	deadline := time.Now().Add(3 * time.Second)
	for rt.DrivePort().Observe().Frame == "0" {
		if time.Now().After(deadline) {
			t.Fatal("virtual startup did not finish its ready bootstrap frame")
		}
		time.Sleep(time.Millisecond)
	}
	if rt.DrivePort().Observe().Frame != "1" {
		t.Fatalf("bootstrap frame=%s, want one parked virtual frame", rt.DrivePort().Observe().Frame)
	}
	frame := rt.DrivePort().Observe().Frame
	time.Sleep(10 * time.Millisecond)
	if got := rt.DrivePort().Observe().Frame; got != frame {
		t.Fatalf("parked bootstrap produced extra frame: %s -> %s", frame, got)
	}
}

func TestRenderCreditDoesNotBlockControl(t *testing.T) {
	rtRoot := buildBenchmarkTree()
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Drive.Enabled = true
	testSessionTable(t, &cfg)
	backend := &delayedCommandBackend{
		started: make(chan struct{}, 1),
		release: make(chan struct{}, 1),
	}
	rt, err := New(cfg, nil, &testWindow{width: 800, height: 600}, backend, rtRoot)
	if err != nil {
		t.Fatal(err)
	}
	client := testClientFor(t, rt)
	done := make(chan error, 1)
	go func() { done <- rt.Run() }()
	defer func() {
		select {
		case backend.release <- struct{}{}:
		default:
		}
		rt.Shutdown()
		<-done
	}()
	transition := enqueueDriveCommand(t, client, "time.mode", map[string]string{"mode": "virtual"})
	select {
	case result := <-transition.Reply:
		if result.Err != nil {
			t.Fatalf("virtual transition failed: %v", result.Err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("virtual transition did not complete")
	}
	backend.delay.Store(true)
	advance := enqueueDriveCommand(t, client, "time.advance", map[string]string{"duration_ns": "1000000"})
	select {
	case <-backend.started:
	case <-time.After(3 * time.Second):
		t.Fatal("virtual transition did not reach the render worker")
	}
	ping := enqueueDriveCommand(t, client, "sys.ping", map[string]any{})
	select {
	case result := <-ping.Reply:
		if result.Err != nil {
			t.Fatalf("control ping failed while render was blocked: %v", result.Err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("runtime control service waited for render completion")
	}
	select {
	case result := <-advance.Reply:
		t.Fatalf("advance completed before its render acknowledgment: %+v", result)
	default:
	}
	backend.release <- struct{}{}
	select {
	case result := <-advance.Reply:
		if result.Err != nil {
			t.Fatalf("advance result: %v", result.Err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("advance did not complete after render acknowledgment")
	}
}

func TestDrive_TimeModeRefusesMidGesture(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	rt.Clock().SetGestureGuard(func() bool { return true })
	res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants())
	if resultCode(res.Err) != string(proto.ErrGestureActive) {
		t.Fatalf("err = %+v, want gesture-active", res.Err)
	}
	if rt.Clock().Mode() != ClockWall {
		t.Fatal("mode changed after refused transition")
	}
}

func TestDrive_TimeModeGrantDenied(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	before := rt.DriveStats().Load().GrantDenied
	res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, drive.Grants{Read: true})
	if resultCode(res.Err) != string(proto.ErrGrant) {
		t.Fatalf("err = %+v, want grant.denied", res.Err)
	}
	if got := rt.DriveStats().Load().GrantDenied; got != before+1 {
		t.Fatalf("grant-denied counter = %d, want %d", got, before+1)
	}
	if rt.Clock().Mode() != ClockWall {
		t.Fatal("denied command changed mode")
	}
}

func TestDrive_AdvanceRequiresVirtual(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	res := driveCall(t, rt, "time.advance", map[string]string{"duration_ns": "16666667"}, allGrants())
	if resultCode(res.Err) != string(proto.ErrTimeMode) {
		t.Fatalf("err = %+v, want time.mode", res.Err)
	}
}

func TestAdvanceExactlyOneLiveAndHeadless(t *testing.T) {
	t.Run("headless", func(t *testing.T) {
		rt := mustDriveRuntime(t)
		defer rt.Shutdown()
		if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); res.Err != nil {
			t.Fatal(res.Err)
		}
		before := rt.frameNumber
		res := driveCall(t, rt, "time.advance", map[string]string{"duration_ns": "500000000"}, allGrants())
		if res.Err != nil {
			t.Fatal(res.Err)
		}
		if got := rt.frameNumber - before; got != 1 {
			t.Fatalf("headless advance frames=%d want 1", got)
		}
		var payload struct {
			Frame  string `json:"frame"`
			Ready  string `json:"ready_frame"`
			Status string `json:"render_status"`
		}
		if err := json.Unmarshal(res.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Frame != strconv.FormatUint(rt.frameNumber, 10) || payload.Ready != payload.Frame || payload.Status != "ready" {
			t.Fatalf("advance result = %+v", payload)
		}
	})
	t.Run("live", func(t *testing.T) {
		rt := mustDriveRuntime(t)
		client := testClientFor(t, rt)
		done := make(chan error, 1)
		go func() { done <- rt.Run() }()
		defer func() { rt.Shutdown(); <-done }()
		// The first wall frame is driven by the runtime loop. The mode request
		// owns its explicit transition frame and then parks.
		frameNumber := func() uint64 {
			value, _ := strconv.ParseUint(rt.DrivePort().Observe().Frame, 10, 64)
			return value
		}
		waitFor := func(pred func() bool) bool {
			deadline := time.After(3 * time.Second)
			for {
				if pred() {
					return true
				}
				select {
				case <-deadline:
					return false
				case <-time.After(time.Millisecond):
				}
			}
		}
		if !waitFor(func() bool { return frameNumber() > 0 }) {
			t.Fatal("live loop did not submit bootstrap wall frame")
		}
		call := func(method string, params any) drive.Result {
			raw, err := json.Marshal(params)
			if err != nil {
				t.Fatal(err)
			}
			client.next++
			cmd := drive.Command{Session: client.sess.ID, Generation: client.sess.Generation(), ID: client.next, OpID: rt.config.Drive.Sessions.InstanceID() + "/" + client.sess.ID + "/" + strconv.FormatUint(client.next, 10), Method: method, Params: raw, Grants: client.sess.Grants, Reply: make(chan drive.Result, 1)}
			if err := rt.DrivePort().Submit(cmd); err != nil {
				t.Fatal(err)
			}
			select {
			case result := <-cmd.Reply:
				return result
			case <-time.After(3 * time.Second):
				t.Fatalf("live %s did not complete", method)
			}
			return drive.Result{}
		}
		res := call("time.mode", map[string]string{"mode": "virtual"})
		if res.Err != nil {
			t.Fatal(res.Err)
		}
		before := frameNumber()
		res = call("time.advance", map[string]string{"duration_ns": "500000000"})
		if res.Err != nil {
			t.Fatal(res.Err)
		}
		if got := frameNumber() - before; got != 1 {
			t.Fatalf("live advance frames=%d want 1", got)
		}
	})
}

func TestParkedReadDoesNotRender(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); res.Err != nil {
		t.Fatal(res.Err)
	}
	before := rt.frameNumber
	res := driveCall(t, rt, "sys.ping", nil, allGrants())
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if rt.frameNumber != before {
		t.Fatalf("parked read rendered frame %d -> %d", before, rt.frameNumber)
	}
}

func TestDrive_AdvanceRejectsDurationBoundsAndOverflow(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); res.Err != nil {
		t.Fatal(res.Err)
	}
	for _, value := range []string{"0", "1000000001", "-1", "18446744073709551616", "1.5"} {
		res := driveCall(t, rt, "time.advance", map[string]string{"duration_ns": value}, allGrants())
		if resultCode(res.Err) != string(proto.ErrInvalidParams) {
			t.Errorf("duration %q error=%+v, want invalid_params", value, res.Err)
		}
	}
}

func TestFramePlanBudgetAndThenReads(t *testing.T) {
	var activations atomic.Int32
	rt := mustDriveRuntimeWithCommand(t, func() { activations.Add(1) })
	defer rt.Shutdown()
	if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); res.Err != nil {
		t.Fatal(res.Err)
	}
	before := rt.frameNumber
	res := driveCall(t, rt, "frame.run", map[string]any{
		"steps": []map[string]any{
			{"method": "cmd.execute", "params": map[string]any{"name": "fixture.plan", "version": 1}},
			{"method": "cmd.execute", "params": map[string]any{"name": "fixture.plan", "version": 1}},
		},
		"step_duration_ns": "16666667",
		"advance":          map[string]any{"frames": 2},
		"max_frames":       4,
		"then":             []map[string]any{{"method": "sys.ping", "params": map[string]any{}}},
	}, allGrants())
	if res.Err != nil {
		t.Fatalf("frame.run: %v (%s)", res.Err, errDetail(res.Err))
	}
	if got := activations.Load(); got != 2 {
		t.Fatalf("GUI command activations=%d want 2", got)
	}
	if got := rt.frameNumber - before; got != 4 {
		t.Fatalf("plan frames=%d want 4", got)
	}
	var payload struct {
		State   string            `json:"state"`
		Applied int               `json:"applied_steps"`
		Frames  string            `json:"frames"`
		Then    []json.RawMessage `json:"then"`
		Ready   string            `json:"ready_frame"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.State != "completed" || payload.Applied != 2 || payload.Frames != "4" || len(payload.Then) != 1 || payload.Ready != strconv.FormatUint(rt.frameNumber, 10) {
		t.Fatalf("plan result=%+v payload=%s", payload, res.Payload)
	}
}

func TestPlanDisconnectStopsNextStep(t *testing.T) {
	var activations atomic.Int32
	rt := mustDriveRuntimeWithCommand(t, func() { activations.Add(1) })
	defer rt.Shutdown()
	client := testClientFor(t, rt)
	if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); res.Err != nil {
		t.Fatal(res.Err)
	}
	plan := enqueueDriveCommand(t, client, "frame.run", map[string]any{
		"steps": []map[string]any{
			{"method": "cmd.execute", "params": map[string]any{"name": "fixture.plan", "version": 1}},
			{"method": "cmd.execute", "params": map[string]any{"name": "fixture.plan", "version": 1}},
		},
		"step_duration_ns": "1000000",
		"max_frames":       2,
	})
	// The first top-level turn admits the plan and consumes its first frame.
	rt.RunOneFrameAt(time.Time{})
	if got := activations.Load(); got != 1 {
		t.Fatalf("first plan frame activations=%d want 1", got)
	}
	rt.DrivePort().SignalControl(drive.ControlSignal{Session: client.sess.ID, Generation: client.sess.Generation()})
	rt.RunOneFrameAt(time.Time{})
	select {
	case result := <-plan.Reply:
		if resultCode(result.Err) != string(proto.ErrOperationCancelled) {
			t.Fatalf("plan terminal error=%+v, want operation.cancelled", result.Err)
		}
	case <-time.After(time.Second):
		t.Fatal("disconnected plan did not terminalize at the next safe boundary")
	}
	if got := activations.Load(); got != 1 {
		t.Fatalf("disconnect allowed another plan step: activations=%d want 1", got)
	}
}

func TestFramePlanGrantUnionAndBounds(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, drive.Grants{Time: true}); res.Err != nil {
		t.Fatal(res.Err)
	}
	params := map[string]any{"steps": []map[string]any{{"method": "cmd.execute", "params": map[string]any{"name": "fixture.missing", "version": 1}}}, "step_duration_ns": "1", "max_frames": 1}
	if res := driveCall(t, rt, "frame.run", params, drive.Grants{Time: true}); resultCode(res.Err) != string(proto.ErrGrant) {
		t.Fatalf("missing command grant err=%+v", res.Err)
	}
	params = map[string]any{"steps": []map[string]any{{"method": "cmd.execute", "params": map[string]any{"name": "fixture.missing", "version": 1}}}, "step_duration_ns": "1", "max_frames": 1}
	res := driveCall(t, rt, "frame.run", params, allGrants())
	if resultCode(res.Err) != string(proto.ErrCommandUnavailable) {
		t.Fatalf("missing command err=%+v", res.Err)
	}
	inputRT, _ := newInputRuntime(t, &recordQueue{})
	if mode := driveCall(t, inputRT, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); mode.Err != nil {
		t.Fatal(mode.Err)
	}
	params = map[string]any{"steps": []map[string]any{{"method": "input.click", "params": map[string]any{"x": 1, "y": 1}}}, "step_duration_ns": "1", "max_frames": 3}
	res = driveCall(t, inputRT, "frame.run", params, allGrants())
	if res.Err != nil {
		t.Fatalf("input click frame plan err=%+v (%s)", res.Err, errDetail(res.Err))
	}
	params = map[string]any{"steps": []map[string]any{{"method": "cmd.execute", "params": map[string]any{"name": "x", "version": 1}}}, "step_duration_ns": "1", "max_frames": 0}
	res = driveCall(t, rt, "frame.run", params, allGrants())
	if resultCode(res.Err) != string(proto.ErrInvalidParams) {
		t.Fatalf("missing max_frames err=%+v", res.Err)
	}
}

func TestFramePlanBoundGestureUsesCanonicalInputAtoms(t *testing.T) {
	q := &recordQueue{}
	rt, _ := newInputRuntime(t, q)
	defer rt.Shutdown()
	if result := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); result.Err != nil {
		t.Fatal(result.Err)
	}
	before := rt.frameNumber
	result := driveCall(t, rt, "frame.run", map[string]any{
		"steps": []map[string]any{
			{"method": "input.press", "bind_gesture": "drag", "params": map[string]any{"x": 10, "y": 10}},
			{"method": "input.move", "use_gesture": "drag", "params": map[string]any{"x": 20, "y": 10}},
			{"method": "input.release", "use_gesture": "drag", "params": map[string]any{}},
		},
		"step_duration_ns": "1000000",
		"max_frames":       4,
	}, allGrants())
	if result.Err != nil {
		t.Fatalf("frame.run gesture plan: %v (%s)", result.Err, errDetail(result.Err))
	}
	if got := rt.frameNumber - before; got != 4 {
		t.Fatalf("gesture plan frames=%d want four canonical atoms", got)
	}
	var kinds []platform.PointerEventKind
	for _, event := range q.seenDriveEvents() {
		if pointer, ok := event.(platform.EventPointer); ok && pointer.Source == platform.SourceDrive {
			kinds = append(kinds, pointer.Kind)
		}
	}
	want := []platform.PointerEventKind{platform.PointerMove, platform.PointerPress, platform.PointerMove, platform.PointerRelease}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("gesture atom kinds=%v want %v", kinds, want)
	}
	if rt.driveInputStep != nil {
		t.Fatal("successful frame plan left its gesture held")
	}
}

func TestFramePlanUntilQuiescentIsBounded(t *testing.T) {
	rt := mustDriveRuntime(t)
	defer rt.Shutdown()
	if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); res.Err != nil {
		t.Fatal(res.Err)
	}
	before := rt.frameNumber
	res := driveCall(t, rt, "frame.run", map[string]any{
		"steps": []any{}, "step_duration_ns": "1000000",
		"advance":    map[string]any{"until": map[string]any{"quiescent_frames": 2, "jobs_drained": true, "max_frames": 4}},
		"max_frames": 4,
	}, allGrants())
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if got := rt.frameNumber - before; got < 2 || got > 4 {
		t.Fatalf("until frames=%d want 2..4", got)
	}
}

func TestFramePlanTotalBudgetReportsProgress(t *testing.T) {
	var activations atomic.Int32
	rt := mustDriveRuntimeWithCommand(t, func() { activations.Add(1) })
	defer rt.Shutdown()
	if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); res.Err != nil {
		t.Fatal(res.Err)
	}
	res := driveCall(t, rt, "frame.run", map[string]any{
		"steps":            []map[string]any{{"method": "cmd.execute", "params": map[string]any{"name": "fixture.plan", "version": 1}}, {"method": "cmd.execute", "params": map[string]any{"name": "fixture.plan", "version": 1}}},
		"step_duration_ns": "1", "max_frames": 1,
	}, allGrants())
	if resultCode(res.Err) != string(proto.ErrInvalidParams) || !strings.Contains(errDetail(res.Err), "declared plan budget") {
		t.Fatalf("budget result=%+v detail=%s", res.Err, errDetail(res.Err))
	}
	if got := activations.Load(); got != 0 {
		t.Fatalf("invalid plan activations=%d want 0", got)
	}
}

func mustDriveRuntimeWithCommand(t *testing.T, execute func()) *Runtime {
	t.Helper()
	root := buildBenchmarkTree()
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Drive.Enabled = true
	testSessionTable(t, &cfg)
	registry := NewCommandRegistry()
	registry.Register(CommandEntry{ID: "fixture.plan", Version: 1, Title: "Plan fixture", Execute: execute})
	cfg.CommandRegistry = registry
	rt, err := New(cfg, nil, &testWindow{width: 800, height: 600}, &backendFixture{}, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(func() { rt.Shutdown() })
	return rt
}

func TestVirtualJobCompletionOrderIndependent(t *testing.T) {
	for _, releaseOrder := range [][]int{{1, 2}, {2, 1}} {
		t.Run(strconv.Itoa(releaseOrder[0])+"-then-"+strconv.Itoa(releaseOrder[1]), func(t *testing.T) {
			var rt *Runtime
			started := make(chan int, 2)
			release := map[int]chan struct{}{1: make(chan struct{}), 2: make(chan struct{})}
			var mu sync.Mutex
			var committed []int
			rt = mustDriveRuntimeWithCommand(t, func() {
				for i := 1; i <= 2; i++ {
					value := i
					rt.Schedule(job.BindJob(uint64(rt.root.Base().ID()), job.Job[int, int]{
						ID: job.JobID(9200 + value), Snapshot: job.NewSnapshot(value),
						Work: func(snap job.Snapshot[int], _ *job.CancelToken) (int, error) {
							started <- snap.Data
							<-release[snap.Data]
							return snap.Data, nil
						},
					}, func(result int) { mu.Lock(); committed = append(committed, result); mu.Unlock() }))
				}
			})
			defer rt.Shutdown()
			client := testClientFor(t, rt)
			if res := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); res.Err != nil {
				t.Fatal(res.Err)
			}
			command := enqueueDriveCommand(t, client, "cmd.execute", map[string]any{"name": "fixture.plan", "version": 1})
			if res := driveCall(t, rt, "time.advance", map[string]string{"duration_ns": "1000000"}, allGrants()); res.Err != nil {
				t.Fatal(res.Err)
			}
			seen := map[int]bool{}
			for len(seen) < 2 {
				seen[<-started] = true
			}
			for _, value := range releaseOrder {
				close(release[value])
			}
			frontier := rt.jobPool.Frontier()
			deadline := time.Now().Add(time.Second)
			for !rt.jobPool.ReadyThrough(frontier) {
				if time.Now().After(deadline) {
					t.Fatal("pure job frontier did not complete")
				}
				time.Sleep(time.Millisecond)
			}
			if res := driveCall(t, rt, "time.advance", map[string]string{"duration_ns": "1000000"}, allGrants()); res.Err != nil {
				t.Fatal(res.Err)
			}
			select {
			case res := <-command.Reply:
				if res.Err != nil {
					t.Fatalf("command result: %v", res.Err)
				}
			case <-time.After(time.Second):
				t.Fatal("command did not complete after its owned jobs")
			}
			mu.Lock()
			got := append([]int(nil), committed...)
			mu.Unlock()
			if len(got) != 2 || got[0] != 1 || got[1] != 2 {
				t.Fatalf("commit order=%v, want submission order [1 2]", got)
			}
		})
	}
}

func enqueueDriveCommand(t testing.TB, client *driveTestClient, method string, params any) drive.Command {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	client.next++
	cmd := drive.Command{Trusted: true, Session: client.sess.ID, Generation: client.sess.Generation(), ID: client.next, OpID: client.rt.config.Drive.Sessions.InstanceID() + "/" + client.sess.ID + "/" + strconv.FormatUint(client.next, 10), Method: method, Params: raw, Grants: client.sess.Grants, Reply: make(chan drive.Result, 1)}
	if err := client.rt.DrivePort().Submit(cmd); err != nil {
		t.Fatal(err)
	}
	return cmd
}
