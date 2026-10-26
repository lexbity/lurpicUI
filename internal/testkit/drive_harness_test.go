package testkit

import (
	"encoding/json"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestDriveHarness_PingAtCut(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	root := newClickCounterFacet()
	h := NewDriveHarness(t, scope, 100, 100, root)
	h.RunFrame() // epoch 1

	payload, perr := h.Call("sys.ping", nil)
	if perr != nil {
		t.Fatalf("sys.ping: %v", perr)
	}
	var got struct {
		Pong bool `json:"pong"`
	}
	if err := json.Unmarshal(payload, &got); err != nil || !got.Pong {
		t.Fatalf("payload = %s (%v)", payload, err)
	}
	// Ping is control-only and is answered without authorizing another frame.
	if h.FrameCount != 1 {
		t.Fatalf("frame count = %d, want 1", h.FrameCount)
	}
}

func TestControlAwaitCreatesNoFrame(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	cfg := StandardHarnessConfig(t, 100, 100)
	cfg.Clock = runtime.ClockConfig{InitialMode: runtime.ClockVirtual, EpochNS: 1_800_000_000_000_000_000}
	h := NewDriveHarnessWithConfig(t, scope, cfg, newClickCounterFacet())
	h.RunFrame() // the one explicit virtual bootstrap frame
	before := h.Runtime().LastFrameStats().FrameNumber
	if before != 1 {
		t.Fatalf("bootstrap frame = %d, want 1", before)
	}
	payload, perr := h.Call("sys.ping", nil)
	if perr != nil {
		t.Fatalf("sys.ping: %v", perr)
	}
	var got struct {
		Pong bool `json:"pong"`
	}
	if err := json.Unmarshal(payload, &got); err != nil || !got.Pong {
		t.Fatalf("payload = %s (%v)", payload, err)
	}
	if after := h.Runtime().LastFrameStats().FrameNumber; after != before {
		t.Fatalf("control await advanced frame from %d to %d", before, after)
	}
}

func TestDriveHarness_InfoReportsEpoch(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	root := newClickCounterFacet()
	h := NewDriveHarness(t, scope, 100, 100, root)
	h.RunFrames(3)

	payload := h.CallOK("sys.info", nil)
	var info struct {
		V     int    `json:"v"`
		Frame string `json:"frame"`
		Drive struct {
			CommandsDrained uint64 `json:"commands_drained"`
		} `json:"drive"`
	}
	if err := json.Unmarshal(payload, &info); err != nil {
		t.Fatalf("sys.info payload: %v", err)
	}
	if info.V != proto.ProtocolMajor {
		t.Fatalf("v = %d", info.V)
	}
	// The call executes at the boundary before frame 4: it observes the
	// sealed cut after frame 3.
	if info.Frame != "3" {
		t.Fatalf("frame = %s, want the cut after frame 3", info.Frame)
	}
	if info.Drive.CommandsDrained != 1 {
		t.Fatalf("commandsDrained = %d, want 1", info.Drive.CommandsDrained)
	}
}

func TestDriveHarness_UnknownMethodTypedError(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	root := newClickCounterFacet()
	h := NewDriveHarness(t, scope, 100, 100, root)
	_, perr := h.Call("sys.nope", nil)
	if perr == nil || perr.Code != proto.ErrUnknownMethod {
		t.Fatalf("err = %+v, want typed internal error", perr)
	}
}

func TestDriveHarness_PlainHarnessHasNoSink(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	root := newClickCounterFacet()
	h := NewHarness(t, scope, testHarnessConfig(t), root)
	if h.Runtime().DrivePort() != nil {
		t.Fatal("plain Harness must not install a drive sink")
	}
}
