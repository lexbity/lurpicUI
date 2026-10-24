package testkit

import (
	"encoding/json"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive/proto"
)

func TestDriveHarness_PingAtCut(t *testing.T) {
	root := newClickCounterFacet()
	h := NewDriveHarness(t, 100, 100, root)
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
	// The command executed at the cut of the frame after the enqueue.
	if h.FrameCount != 2 {
		t.Fatalf("frame count = %d, want 2", h.FrameCount)
	}
}

func TestDriveHarness_InfoReportsEpoch(t *testing.T) {
	root := newClickCounterFacet()
	h := NewDriveHarness(t, 100, 100, root)
	h.RunFrames(3)

	payload := h.CallOK("sys.info", nil)
	var info struct {
		V     int    `json:"v"`
		Frame uint64 `json:"frame"`
		Drive struct {
			CommandsDrained uint64 `json:"commandsDrained"`
		} `json:"drive"`
	}
	if err := json.Unmarshal(payload, &info); err != nil {
		t.Fatalf("sys.info payload: %v", err)
	}
	if info.V != proto.ProtocolMajor {
		t.Fatalf("v = %d", info.V)
	}
	if info.Frame != 4 {
		t.Fatalf("frame = %d, want 4", info.Frame)
	}
	if info.Drive.CommandsDrained != 1 {
		t.Fatalf("commandsDrained = %d, want 1", info.Drive.CommandsDrained)
	}
}

func TestDriveHarness_UnknownMethodTypedError(t *testing.T) {
	root := newClickCounterFacet()
	h := NewDriveHarness(t, 100, 100, root)
	_, perr := h.Call("sys.nope", nil)
	if perr == nil || perr.Code != proto.ErrInternal {
		t.Fatalf("err = %+v, want typed internal error", perr)
	}
}

func TestDriveHarness_PlainHarnessHasNoSink(t *testing.T) {
	root := newClickCounterFacet()
	h := NewHarness(t, testHarnessConfig(t), root)
	if h.Runtime().DriveSink() != nil {
		t.Fatal("plain Harness must not install a drive sink")
	}
}
