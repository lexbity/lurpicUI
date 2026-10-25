package testkit

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestDriveFramePacket_HashAndEpoch: frame.packet reports the last submitted
// primary frame's content digest and the cut epoch; before any frame it
// refuses with a typed error.
func TestDriveFramePacket_HashAndEpoch(t *testing.T) {
	// A harness whose first frame has not run: frame.packet refuses with a
	// typed error until a frame is submitted.
	h := newLocateHarnessNoFrame(t)
	if _, perr := h.Call("frame.packet", nil); perr == nil {
		t.Fatal("frame.packet before any frame must fail")
	}
	h.RunFrame()
	payload := h.CallOK("frame.packet", nil)
	var got struct {
		Epoch   string `json:"epoch"`
		Hash    string `json:"hash"`
		Batches int    `json:"batches"`
	}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if got.Hash == "" || got.Hash == "0" {
		t.Fatal("a rendered frame must hash nonzero")
	}
	if got.Epoch == "" || got.Epoch == "0" {
		t.Fatal("epoch must be reported")
	}
}

// TestDriveFramePixels_SoftwareSurface: frame.pixels returns PNG bytes of
// the current software surface at the surface's size.
func TestDriveFramePixels_SoftwareSurface(t *testing.T) {
	h := newLocateHarness(t)
	h.RunFrame()
	payload := h.CallOK("frame.pixels", nil)
	var got struct {
		PNG    string `json:"png"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("payload %.80s: %v", payload, err)
	}
	if !strings.HasPrefix(got.PNG, "data:image/png;base64,") {
		t.Fatalf("png field = %.40s", got.PNG)
	}
	if got.Width != 200 || got.Height != 120 {
		t.Fatalf("size = %dx%d, want 200x120", got.Width, got.Height)
	}
}

// newLocateHarnessNoFrame builds the locate harness without its settling
// frame (the caller owns the frame timeline).
func newLocateHarnessNoFrame(t *testing.T) *DriveHarness {
	t.Helper()
	root := newLocateProbe("probe-root", "shell/root", "region", "", "")
	save := newLocateProbe("probe-save", "action/button", "button", "Save", "")
	root.AddChild(save.Base())
	cfg := StandardHarnessConfig(t, 200, 120)
	return NewDriveHarnessWithConfig(t, cfg, root)
}
