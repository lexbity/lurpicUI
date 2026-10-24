package runtime

import (
	"encoding/json"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive/proto"
	drive "codeburg.org/lexbit/lurpicui/drive/server"
)

func mustDriveRuntime(t testing.TB) *Runtime {
	t.Helper()
	root := buildBenchmarkTree()
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Drive.Enabled = true
	rt, err := New(cfg, nil, &testWindow{width: 800, height: 600}, &backendFixture{}, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(func() { rt.Shutdown() })
	return rt
}

func drivePingCmd(id uint64) drive.Command {
	return drive.Command{ID: id, Method: "sys.ping", Grants: drive.Grants{Read: true}, Reply: make(chan drive.Result, 1)}
}

func TestDrive_DisabledByDefaultNoSink(t *testing.T) {
	rt := mustRuntimeTree(t, buildBenchmarkTree())
	defer rt.Shutdown()
	if rt.DriveSink() != nil {
		t.Fatal("DriveSink must be nil when disabled")
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

	cmd := drivePingCmd(1)
	rt.DriveSink().Enqueue(cmd)

	rt.RunOneFrame() // frame 2: drains at the cut
	res := <-cmd.Reply
	if res.Err != nil {
		t.Fatalf("ping error: %+v", res.Err)
	}
	var payload struct {
		Pong bool `json:"pong"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil || !payload.Pong {
		t.Fatalf("payload = %s (%v)", res.Payload, err)
	}
	// QP-6: the reply carries the epoch of the cut it observed — the last
	// completed frame (frame 1); the command executes at the boundary before
	// frame 2.
	if res.Frame != 1 {
		t.Fatalf("frame = %d, want 1 (the cut after the first frame)", res.Frame)
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
	cmds := make([]drive.Command, n)
	for i := range cmds {
		cmds[i] = drivePingCmd(uint64(i + 1))
		rt.DriveSink().Enqueue(cmds[i])
	}
	rt.RunOneFrame()
	for i, cmd := range cmds {
		res := <-cmd.Reply
		if res.Err != nil || res.Frame != 1 {
			t.Fatalf("cmd %d: %+v (frame %d, want the cut epoch 1)", i, res.Err, res.Frame)
		}
	}
	if got := rt.LastFrameStats().DriveCommandsDrained; got != n {
		t.Fatalf("DriveCommandsDrained = %d, want %d", got, n)
	}
}

func TestDrive_UnknownMethodTypedError(t *testing.T) {
	rt := mustDriveRuntime(t)
	rt.RunOneFrame()
	cmd := drive.Command{ID: 9, Method: "sys.nope", Reply: make(chan drive.Result, 1)}
	rt.DriveSink().Enqueue(cmd)
	rt.RunOneFrame()
	res := <-cmd.Reply
	if res.Err == nil || res.Err.Code != proto.ErrInternal {
		t.Fatalf("result err = %+v, want internal typed error", res.Err)
	}
	if !strings.Contains(res.Err.Detail, "sys.nope") {
		t.Fatalf("detail %q must name the method", res.Err.Detail)
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
	cmd := drive.Command{ID: 3, Method: "sys.panic-test", Grants: drive.Grants{Read: true}, Reply: make(chan drive.Result, 1)}
	rt.DriveSink().Enqueue(cmd)
	rt.RunOneFrame()
	res := <-cmd.Reply
	if res.Err == nil || res.Err.Code != proto.ErrInternal {
		t.Fatalf("result err = %+v, want internal.panic", res.Err)
	}
	if !strings.Contains(res.Err.Detail, "boom") {
		t.Fatalf("detail %q must carry the panic value", res.Err.Detail)
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
	cmd := drive.Command{ID: 2, Method: "sys.info", Grants: drive.Grants{Read: true}, Reply: make(chan drive.Result, 1)}
	rt.DriveSink().Enqueue(cmd)
	rt.RunOneFrame()
	res := <-cmd.Reply
	if res.Err != nil {
		t.Fatalf("sys.info error: %+v", res.Err)
	}
	var payload struct {
		V       int      `json:"v"`
		Frame   uint64   `json:"frame"`
		Methods []string `json:"methods"`
		Drive   struct {
			CommandsDrained uint64 `json:"commandsDrained"`
		} `json:"drive"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload.V != proto.ProtocolMajor {
		t.Fatalf("v = %d", payload.V)
	}
	if payload.Frame != 1 {
		t.Fatalf("frame = %d, want the cut epoch 1", payload.Frame)
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
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(b)
	cfg.Drive.Enabled = true
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
