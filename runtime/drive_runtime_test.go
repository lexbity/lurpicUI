package runtime

import (
	"encoding/json"
	goruntime "runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/render"
)

func mustDriveRuntime(t testing.TB) *Runtime {
	t.Helper()
	root := buildBenchmarkTree()
	cfg := testConfig(t)
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Drive.Enabled = true
	testSessionTable(t, &cfg)
	rt, err := New(cfg, nil, &testWindow{width: 800, height: 600}, &backendFixture{}, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(func() { rt.Shutdown() })
	return rt
}

func drivePingCmd(rt *Runtime, t testing.TB) drive.Result {
	t.Helper()
	return driveCall(t, rt, "sys.ping", nil, allGrants())
}

func TestDrive_DisabledByDefaultNoSink(t *testing.T) {
	rt := mustRuntimeTree(t, buildBenchmarkTree())
	defer rt.Shutdown()
	if rt.DrivePort() != nil {
		t.Fatal("DrivePort must be nil when disabled")
	}
	if rt.DriveStats() != nil {
		t.Fatal("DriveStats must be nil when disabled")
	}
	if got := rt.driveDrain(); got != 0 {
		t.Fatalf("driveDrain = %d when disabled, want 0", got)
	}
	rt.RunOneFrame()
	stats := rt.LastFrameStats()
	if stats.DriveCommandsDrained != 0 || stats.DriveResponsesSent != 0 {
		t.Fatalf("disabled drive counters = %+v", stats)
	}
}

func TestDrive_CommandExecutesAtCut(t *testing.T) {
	rt := mustDriveRuntime(t)
	rt.RunOneFrame() // frame 1: establishes epoch 1

	res := drivePingCmd(rt, t)

	if res.Err != nil {
		t.Fatalf("ping error: %+v", res.Err)
	}
	var payload struct {
		Pong  bool   `json:"pong"`
		Cut   string `json:"cut"`
		Frame string `json:"frame"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil || !payload.Pong {
		t.Fatalf("payload = %s (%v)", res.Payload, err)
	}
	// The pong observes the sealed cut of the last completed frame (frame
	// 1); the command executes at the boundary before frame 2.
	if res.Frame != "1" || res.Cut != "1" || payload.Frame != "1" {
		t.Fatalf("cut/frame = %s/%s (payload %s), want the sealed cut after frame 1", res.Cut, res.Frame, payload.Frame)
	}
	stats := rt.LastFrameStats()
	if stats.DriveCommandsDrained != 1 {
		t.Fatalf("DriveCommandsDrained = %d, want 1", stats.DriveCommandsDrained)
	}
	if stats.FrameNumber != 2 {
		t.Fatalf("FrameNumber = %d, want 2", stats.FrameNumber)
	}
}

func TestDrive_MultipleCommandsInOrder(t *testing.T) {
	rt := mustDriveRuntime(t)
	rt.RunOneFrame()
	const n = 4
	client := testClientFor(t, rt)
	cmds := make([]drive.Command, n)
	for i := range cmds {
		client.next++
		cmds[i] = drive.Command{
			Trusted:    true,
			Session:    client.sess.ID,
			Generation: client.sess.Generation(),
			ID:         client.next,
			Method:     "sys.ping",
			Grants:     allGrants(),
			Reply:      make(chan drive.Result, 1),
		}
		if err := rt.DrivePort().Submit(cmds[i]); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	rt.RunOneFrame()
	for i, cmd := range cmds {
		res := <-cmd.Reply
		if res.Err != nil || res.Frame != "1" {
			t.Fatalf("cmd %d: %+v (frame %s, want the sealed cut \"1\")", i, res.Err, res.Frame)
		}
	}
	if got := rt.LastFrameStats().DriveCommandsDrained; got != n {
		t.Fatalf("DriveCommandsDrained = %d, want %d", got, n)
	}
}

func TestDrive_UnknownMethodTypedError(t *testing.T) {
	rt := mustDriveRuntime(t)
	rt.RunOneFrame()
	res := driveCall(t, rt, "sys.nope", nil, allGrants())
	if resultCode(res.Err) != string(proto.ErrUnknownMethod) {
		t.Fatalf("result err = %+v, want protocol.unknown_method", res.Err)
	}
	if !strings.Contains(errDetail(res.Err), "sys.nope") {
		t.Fatalf("detail %q must name the method", errDetail(res.Err))
	}
}

func TestDriveVersionedCommandUsesGUIRegistryDispatcher(t *testing.T) {
	root := buildBenchmarkTree()
	cfg := testConfig(t)
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Drive.Enabled = true
	testSessionTable(t, &cfg)
	var activations int
	registry := NewCommandRegistry()
	registry.Register(CommandEntry{
		ID: "fixture.activate", Version: 7, Title: "Activate fixture",
		Execute: func() { activations++ },
	})
	cfg.CommandRegistry = registry
	rt, err := New(cfg, nil, &testWindow{width: 800, height: 600}, &backendFixture{}, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rt.Shutdown() })
	rt.RunOneFrame()

	wrong := driveCall(t, rt, "cmd.execute", map[string]any{"name": "fixture.activate", "version": 6}, allGrants())
	if resultCode(wrong.Err) != string(proto.ErrCommandUnavailable) || activations != 0 {
		t.Fatalf("wrong version = %+v, activations=%d", wrong.Err, activations)
	}
	result := driveCall(t, rt, "cmd.execute", map[string]any{"name": "fixture.activate", "version": 7}, allGrants())
	if result.Err != nil || activations != 1 {
		t.Fatalf("versioned command = %+v, activations=%d", result.Err, activations)
	}
	var payload struct {
		OperationID  string `json:"operation_id"`
		State        string `json:"state"`
		Executed     string `json:"executed"`
		Version      string `json:"version"`
		AppliedFrame string `json:"applied_frame"`
		RenderStatus string `json:"render_status"`
	}
	if err := json.Unmarshal(result.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.OperationID == "" || payload.State != "completed" || payload.Executed != "fixture.activate" || payload.Version != "7" || payload.AppliedFrame == "" || payload.RenderStatus != "ready" {
		t.Fatalf("command result = %+v", payload)
	}
}

func TestCommandListOnlyExposesGUIBackedEntries(t *testing.T) {
	registry := NewCommandRegistry()
	registry.Register(CommandEntry{ID: "fixture.routed", Version: 1, Title: "Routed", Execute: func() {}})
	registry.Register(CommandEntry{ID: "fixture.unrouted", Version: 1, Title: "Unrouted"})
	registry.Register(CommandEntry{ID: "fixture.hidden", Version: 1, Title: "Hidden", Hidden: true, Execute: func() {}})
	listed := registry.List()
	if len(listed) != 1 || listed[0].ID != "fixture.routed" || listed[0].Version != 1 {
		t.Fatalf("cmd.list entries = %+v, want only the visible GUI-backed route", listed)
	}
}

type delayedCommandBackend struct {
	backendFixture
	delay   atomic.Bool
	started chan struct{}
	release chan struct{}
}

func (b *delayedCommandBackend) Submit(frame *render.Frame) error {
	if b.delay.Load() {
		b.started <- struct{}{}
		<-b.release
	}
	return b.backendFixture.Submit(frame)
}

func TestCommandWaitsForRenderAcknowledgment(t *testing.T) {
	root := buildBenchmarkTree()
	cfg := testConfig(t)
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Drive.Enabled = true
	table := testSessionTable(t, &cfg)
	var activations atomic.Int32
	registry := NewCommandRegistry()
	registry.Register(CommandEntry{ID: "fixture.activate", Version: 1, Title: "Activate fixture", Execute: func() { activations.Add(1) }})
	cfg.CommandRegistry = registry
	backend := &delayedCommandBackend{started: make(chan struct{}, 1), release: make(chan struct{}, 1)}
	rt, err := New(cfg, nil, &testWindow{width: 800, height: 600}, backend, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rt.Shutdown() })
	rt.RunOneFrame()
	backend.delay.Store(true)
	sess, _, err := table.Create(allGrants())
	if err != nil {
		t.Fatal(err)
	}
	cmd := drive.Command{
		Trusted: true, Session: sess.ID, Generation: sess.Generation(), ID: 1,
		OpID: table.InstanceID() + "/" + sess.ID + "/1", Method: "cmd.execute",
		Params: json.RawMessage(`{"name":"fixture.activate","version":1}`),
		Grants: allGrants(), Reply: make(chan drive.Result, 1), Accepted: make(chan drive.Receipt, 1),
	}
	if err := rt.DrivePort().Submit(cmd); err != nil {
		t.Fatal(err)
	}
	rt.driveDrain()
	<-cmd.Accepted
	if !rt.renderPipeline.TryReserveFrame() {
		t.Fatal("could not reserve render credit for test frame")
	}
	rt.runFrame(time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC), false, true, false)
	select {
	case <-backend.started:
	case <-time.After(5 * time.Second):
		t.Fatal("render worker did not receive the command's frame")
	}
	if activations.Load() != 1 {
		t.Fatalf("activation count = %d, want 1 before render acknowledgment", activations.Load())
	}
	select {
	case result := <-cmd.Reply:
		t.Fatalf("operation completed before ready surface: %+v", result)
	default:
	}
	statusParams, err := json.Marshal(map[string]string{"operation_id": cmd.OpID})
	if err != nil {
		t.Fatal(err)
	}
	status := drive.Command{
		Trusted: true, Session: sess.ID, Generation: sess.Generation(), ID: 2,
		OpID: table.InstanceID() + "/" + sess.ID + "/2", Method: "op.status",
		Params: statusParams, Grants: allGrants(), Reply: make(chan drive.Result, 1),
	}
	if err := rt.DrivePort().Submit(status); err != nil {
		t.Fatal(err)
	}
	rt.driveDrain()
	statusResult := <-status.Reply
	var pendingState struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(statusResult.Payload, &pendingState); err != nil || pendingState.State != "running" {
		t.Fatalf("pending op.status=%s err=%v, want running", statusResult.Payload, err)
	}
	retry := drive.Command{
		Trusted: true, Session: sess.ID, Generation: sess.Generation(), ID: 1,
		OpID: cmd.OpID, Method: cmd.Method, Params: cmd.Params, Grants: allGrants(),
		Reply: make(chan drive.Result, 1), Accepted: make(chan drive.Receipt, 1),
	}
	if err := rt.DrivePort().Submit(retry); err != nil {
		t.Fatal(err)
	}
	rt.driveDrain()
	select {
	case receipt := <-retry.Accepted:
		if receipt.OperationID != cmd.OpID {
			t.Fatalf("retry operation id=%q, want %q", receipt.OperationID, cmd.OpID)
		}
	default:
		t.Fatal("retry of the retained running ID did not return its original receipt")
	}
	backend.release <- struct{}{}
	var completion RenderCompletion
	select {
	case completion = <-rt.renderPipeline.Completions():
	case <-time.After(5 * time.Second):
		t.Fatal("render acknowledgment was not published")
	}
	rt.driveCompleteCommandFrame(rt.drivePort, completion)
	result := <-cmd.Reply
	if result.Err != nil {
		t.Fatalf("operation result: %v", result.Err)
	}
	var outcome struct {
		RenderStatus string `json:"render_status"`
		AppliedFrame string `json:"applied_frame"`
	}
	if err := json.Unmarshal(result.Payload, &outcome); err != nil || outcome.RenderStatus != "ready" || outcome.AppliedFrame == "" {
		t.Fatalf("rendered outcome=%s err=%v", result.Payload, err)
	}
	retryResult := <-retry.Reply
	if retryResult.Err != nil || string(retryResult.Payload) != string(result.Payload) {
		t.Fatalf("running retry outcome=%+v, original=%+v", retryResult, result)
	}
}

func TestDisconnectCancelsQueuedCommand(t *testing.T) {
	root := buildBenchmarkTree()
	cfg := testConfig(t)
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Drive.Enabled = true
	table := testSessionTable(t, &cfg)
	var activations int
	registry := NewCommandRegistry()
	registry.Register(CommandEntry{ID: "fixture.activate", Version: 1, Title: "Activate fixture", Execute: func() { activations++ }})
	cfg.CommandRegistry = registry
	rt, err := New(cfg, nil, &testWindow{width: 800, height: 600}, &backendFixture{}, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rt.Shutdown() })
	rt.RunOneFrame()
	sess, _, err := table.Create(allGrants())
	if err != nil {
		t.Fatal(err)
	}
	gen := sess.Generation()
	id := uint64(1)
	cmd := drive.Command{
		Trusted: true, Session: sess.ID, Generation: gen, ID: id,
		OpID: table.InstanceID() + "/" + sess.ID + "/1", Method: "cmd.execute",
		Params: json.RawMessage(`{"name":"fixture.activate","version":1}`),
		Grants: allGrants(), Reply: make(chan drive.Result, 1), Accepted: make(chan drive.Receipt, 1),
	}
	if err := rt.DrivePort().Submit(cmd); err != nil {
		t.Fatal(err)
	}
	rt.driveDrain()
	select {
	case <-cmd.Accepted:
	default:
		t.Fatal("command was not accepted before its activation boundary")
	}
	if !sess.DetachGeneration(gen, time.Now()) {
		t.Fatal("failed to detach the controlling generation")
	}
	rt.DrivePort().SignalControl(drive.ControlSignal{Kind: drive.SignalConnectionLost, Session: sess.ID, Generation: gen})
	rt.RunOneFrameAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if activations != 0 {
		t.Fatalf("queued command activated after disconnect: %d", activations)
	}
	select {
	case result := <-cmd.Reply:
		if resultCode(result.Err) != string(proto.ErrOperationCancelled) {
			t.Fatalf("cancellation result = %+v", result)
		}
	default:
		t.Fatal("accepted command did not terminalize at disconnect boundary")
	}
	newGeneration, err := sess.Attach(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	statusParams, err := json.Marshal(map[string]string{"operation_id": table.InstanceID() + "/" + sess.ID + "/1"})
	if err != nil {
		t.Fatal(err)
	}
	status := drive.Command{
		Trusted: true, Session: sess.ID, Generation: newGeneration, ID: 2,
		OpID: table.InstanceID() + "/" + sess.ID + "/2", Method: "op.status",
		Params: statusParams, Grants: allGrants(), Reply: make(chan drive.Result, 1),
	}
	if err := rt.DrivePort().Submit(status); err != nil {
		t.Fatal(err)
	}
	rt.RunOneFrameAt(time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC))
	statusResult := <-status.Reply
	if statusResult.Err != nil {
		t.Fatalf("resumed op.status: %v", statusResult.Err)
	}
	var statusPayload struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(statusResult.Payload, &statusPayload); err != nil || statusPayload.State != "cancelled" {
		t.Fatalf("resumed op.status=%s err=%v, want cancelled", statusResult.Payload, err)
	}
}

func TestDisconnectFencesCommandJobCommit(t *testing.T) {
	root := buildBenchmarkTree()
	cfg := testConfig(t)
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Drive.Enabled = true
	table := testSessionTable(t, &cfg)
	started := make(chan struct{})
	release := make(chan struct{})
	var commits atomic.Int32
	var rt *Runtime
	registry := NewCommandRegistry()
	registry.Register(CommandEntry{ID: "fixture.job", Version: 1, Title: "Schedule owned job", Execute: func() {
		rt.Schedule(job.BindJob(uint64(root.Base().ID()), job.Job[int, int]{
			ID: 7801,
			MemoryDecl: job.JobMemoryDecl{
				MaxInputBytes: 16, MaxOutputBytes: 16,
			},
			Work: func(snap job.Snapshot[int], _ *job.CancelToken) (int, error) {
				close(started)
				<-release
				return snap.Data + 1, nil
			},
			Snapshot: job.NewSnapshot(1),
		}, func(int) { commits.Add(1) }))
	}})
	cfg.CommandRegistry = registry
	var err error
	rt, err = New(cfg, nil, &testWindow{width: 800, height: 600}, &backendFixture{}, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		rt.Shutdown()
	})
	rt.RunOneFrame()
	sess, _, err := table.Create(allGrants())
	if err != nil {
		t.Fatal(err)
	}
	gen := sess.Generation()
	cmd := drive.Command{
		Trusted: true, Session: sess.ID, Generation: gen, ID: 1,
		OpID: table.InstanceID() + "/" + sess.ID + "/1", Method: "cmd.execute",
		Params: json.RawMessage(`{"name":"fixture.job","version":1}`), Grants: allGrants(),
		Reply: make(chan drive.Result, 1), Accepted: make(chan drive.Receipt, 1),
	}
	if err := rt.DrivePort().Submit(cmd); err != nil {
		t.Fatal(err)
	}
	rt.driveDrain()
	<-cmd.Accepted
	rt.RunOneFrameAt(time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC))
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("operation-owned job did not start")
	}
	select {
	case result := <-cmd.Reply:
		t.Fatalf("command completed before its owned job: %+v", result)
	default:
	}
	close(release)
	waitForCommandJobResult(t, rt.jobPool, 7801)
	if !sess.DetachGeneration(gen, time.Now()) {
		t.Fatal("failed to detach command owner")
	}
	rt.DrivePort().SignalControl(drive.ControlSignal{Kind: drive.SignalConnectionLost, Session: sess.ID, Generation: gen})
	rt.driveDrain()
	select {
	case result := <-cmd.Reply:
		if resultCode(result.Err) != string(proto.ErrOperationCancelled) {
			t.Fatalf("disconnect result=%+v, want cancelled", result.Err)
		}
	default:
		t.Fatal("disconnect did not terminalize the job-owning operation")
	}
	rt.RunOneFrameAt(time.Date(2026, 1, 1, 0, 0, 2, 0, time.UTC))
	if got := commits.Load(); got != 0 {
		t.Fatalf("late operation-owned job committed %d times after disconnect", got)
	}
	if len(rt.drivePort.commandJobScopes) != 0 {
		t.Fatalf("operation job scope retained %d late results", len(rt.drivePort.commandJobScopes))
	}
}

func TestCommandAwaitsOwnedJobCommitAndReadyFrame(t *testing.T) {
	root := buildBenchmarkTree()
	cfg := testConfig(t)
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Drive.Enabled = true
	table := testSessionTable(t, &cfg)
	var committed atomic.Int32
	var rt *Runtime
	registry := NewCommandRegistry()
	registry.Register(CommandEntry{ID: "fixture.job", Version: 1, Title: "Schedule owned job", Execute: func() {
		rt.Schedule(job.BindJob(uint64(root.Base().ID()), job.Job[int, int]{
			ID:         7802,
			Snapshot:   job.NewSnapshot(4),
			MemoryDecl: job.JobMemoryDecl{MaxInputBytes: 16, MaxOutputBytes: 16},
			Work: func(snap job.Snapshot[int], _ *job.CancelToken) (int, error) {
				return snap.Data * 2, nil
			},
		}, func(value int) { committed.Store(int32(value)) }))
	}})
	cfg.CommandRegistry = registry
	var err error
	rt, err = New(cfg, nil, &testWindow{width: 800, height: 600}, &backendFixture{}, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rt.Shutdown() })
	rt.RunOneFrame()
	sess, _, err := table.Create(allGrants())
	if err != nil {
		t.Fatal(err)
	}
	cmd := drive.Command{
		Trusted: true, Session: sess.ID, Generation: sess.Generation(), ID: 1,
		OpID: table.InstanceID() + "/" + sess.ID + "/1", Method: "cmd.execute",
		Params: json.RawMessage(`{"name":"fixture.job","version":1}`), Grants: allGrants(),
		Reply: make(chan drive.Result, 1), Accepted: make(chan drive.Receipt, 1),
	}
	if err := rt.DrivePort().Submit(cmd); err != nil {
		t.Fatal(err)
	}
	rt.driveDrain()
	<-cmd.Accepted
	for frame := 1; frame <= 16; frame++ {
		rt.RunOneFrameAt(time.Date(2026, 1, 1, 0, 0, frame+1, 0, time.UTC))
		select {
		case result := <-cmd.Reply:
			if result.Err != nil {
				t.Fatalf("owned command result: %v", result.Err)
			}
			var payload struct {
				State         string `json:"state"`
				OwnedJobs     string `json:"owned_jobs"`
				JobsCompleted string `json:"jobs_completed"`
				RenderStatus  string `json:"render_status"`
			}
			if err := json.Unmarshal(result.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.State != "completed" || payload.OwnedJobs != "1" || payload.JobsCompleted != "1" || payload.RenderStatus != "ready" || committed.Load() != 8 {
				t.Fatalf("owned command payload=%+v committed=%d", payload, committed.Load())
			}
			return
		default:
		}
	}
	t.Fatal("command did not wait for its owned job commit and ready frame")
}

func waitForCommandJobResult(t *testing.T, pool *job.Pool, id job.JobID) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if pool.Ready(id) {
			return
		}
		goruntime.Gosched()
	}
	t.Fatal("command job result did not enter the pool's result queue")
}

func TestCommandCallbackPanicIsTerminalAndNotRetried(t *testing.T) {
	root := buildBenchmarkTree()
	cfg := testConfig(t)
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Drive.Enabled = true
	testSessionTable(t, &cfg)
	var activations int
	registry := NewCommandRegistry()
	registry.Register(CommandEntry{ID: "fixture.panic", Version: 1, Title: "Panic fixture", Execute: func() {
		activations++
		panic("redacted fixture panic")
	}})
	cfg.CommandRegistry = registry
	rt, err := New(cfg, nil, &testWindow{width: 800, height: 600}, &backendFixture{}, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rt.Shutdown() })
	rt.RunOneFrame()
	params := map[string]any{"name": "fixture.panic", "version": 1}
	first := driveCall(t, rt, "cmd.execute", params, allGrants())
	if resultCode(first.Err) != string(proto.ErrInternalPanic) || activations != 1 {
		t.Fatalf("first panic outcome=%+v activations=%d", first.Err, activations)
	}
	client := testClientFor(t, rt)
	commandParams, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	retry := drive.Command{
		Trusted: true, Session: client.sess.ID, Generation: client.sess.Generation(), ID: 1,
		OpID: client.sess.InstanceID + "/" + client.sess.ID + "/1", Method: "cmd.execute",
		Params: commandParams, Grants: allGrants(), Reply: make(chan drive.Result, 1),
	}
	if err := rt.DrivePort().Submit(retry); err != nil {
		t.Fatal(err)
	}
	rt.RunOneFrameAt(time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC))
	second := <-retry.Reply
	if resultCode(second.Err) != string(proto.ErrInternalPanic) || activations != 1 {
		t.Fatalf("retry outcome=%+v activations=%d; claimed panic must never dispatch twice", second.Err, activations)
	}
}

func TestDrive_HandlerPanicBecomesTypedError(t *testing.T) {
	rt := mustDriveRuntime(t)
	rt.RunOneFrame()
	// Inject a panicking handler for the duration of the test; the dispatch
	// recover must convert it to a typed internal.panic result and reply
	// exactly once (QP-16, NFR-7).
	driveMethods["sys.panic-test"] = func(*Runtime, drive.Command) driveOutcome {
		panic("boom")
	}
	defer delete(driveMethods, "sys.panic-test")
	// The dispatch recover wraps handler execution (driveExecute); the
	// panicking handler is invoked directly on the runtime thread.
	cmd := drive.Command{
		Method: "sys.panic-test",
		Grants: allGrants(),
		Reply:  make(chan drive.Result, 1),
	}
	out := rt.driveExecute(cmd)
	res := out.result
	if resultCode(res.Err) != string(proto.ErrInternalPanic) {
		t.Fatalf("result err = %+v, want internal.panic", res.Err)
	}
	if strings.Contains(errDetail(res.Err), "boom") {
		t.Fatalf("wire error exposed the panic value: %q", errDetail(res.Err))
	}
	// Exactly-once reply.
	select {
	case r := <-cmd.Reply:
		t.Fatalf("duplicate reply: %+v", r)
	default:
	}
}

func TestDrive_InfoMethodListMatchesTable(t *testing.T) {
	// sys.info derives its list from proto.Registry (the single source of
	// truth); the runtime's own dispatch table must be a subset — the
	// server handles sub.* locally, everything else executes here.
	for name := range driveMethods {
		found := false
		for _, m := range proto.MethodNames() {
			if m == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("runtime method %q is not in proto.Registry", name)
		}
	}
}

func TestDrive_SysInfoReportsCounters(t *testing.T) {
	rt := mustDriveRuntime(t)
	rt.RunOneFrame()
	res := driveCall(t, rt, "sys.info", nil, allGrants())
	if res.Err != nil {
		t.Fatalf("sys.info error: %+v", res.Err)
	}
	var payload struct {
		V       int      `json:"v"`
		Frame   string   `json:"frame"`
		Methods []string `json:"methods"`
		Drive   struct {
			CommandsDrained uint64 `json:"commands_drained"`
		} `json:"drive"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload.V != proto.ProtocolMajor {
		t.Fatalf("v = %d", payload.V)
	}
	if payload.Frame != "1" {
		t.Fatalf("frame = %s, want the sealed cut after frame 1", payload.Frame)
	}
	if payload.Drive.CommandsDrained != 1 {
		t.Fatalf("commandsDrained = %d, want 1", payload.Drive.CommandsDrained)
	}
}

// TestDrive_IdleDrainZeroAlloc asserts NFR-4: with a sink installed but zero
// pending commands, the drain point allocates nothing.
func TestDrive_IdleDrainZeroAlloc(t *testing.T) {
	rt := mustDriveRuntime(t)
	rt.RunOneFrame()
	allocs := testing.AllocsPerRun(100, func() {
		if got := rt.driveDrain(); got != 0 {
			t.Fatalf("driveDrain = %d, want 0 when idle", got)
		}
	})
	if allocs != 0 {
		t.Fatalf("idle drain allocated %v times per run, want 0", allocs)
	}
}

// BenchmarkFrameDriveIdle measures the drain point with the sink installed
// and no clients (G1.5): must report 0 allocs/op.
func BenchmarkFrameDriveIdle(b *testing.B) {
	root := buildBenchmarkTree()
	cfg := testConfig(b)
	cfg.LayerRegistry = testLayerRegistry(b)
	cfg.Drive.Enabled = true
	testSessionTable(b, &cfg)
	rt, err := New(cfg, nil, &testWindow{width: 800, height: 600}, &backendFixture{}, root)
	if err != nil {
		b.Fatalf("new runtime: %v", err)
	}
	defer rt.Shutdown()
	rt.RunOneFrame()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rt.driveDrain()
	}
}
