package testkit

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/platform"
)

// The runtime processes input events against the PREVIOUS frame's hit map
// (runtime/frame.go): platform events are drained and routed at the start of
// the frame loop, before layout and projection rebuild the hit map for the
// next frame. On the very first frame after NewHarness the hit map is nil
// (projection.System.currentHitMap is only assigned inside a run), so pointer
// events injected before any RunFrame are silently dropped — the classic
// "click did not fire" test bug.
//
// The Drive* helpers below are the canonical entry point for interaction
// tests: each runs a warmup frame first (so the hit map exists), injects its
// event(s), and runs a frame after each so the runtime processes them. Tests
// that inject raw events via InjectEvent MUST call Warmup (or RunFrame) first.

// Warmup runs a single frame if the harness has not yet run one, so that
// projection builds the hit map. It is a no-op once the harness has run a
// frame. Drive* helpers call it automatically; raw InjectEvent users must call
// it (or RunFrame) before injecting pointer events.
func Warmup(h *Harness) {
	if h == nil {
		return
	}
	if h.FrameCount == 0 {
		h.RunFrame()
	}
}

// DriveClickAt routes a left-button press+release click at (x,y) through the
// runtime, running a warmup frame first and a frame after each of the press
// and release. The release is what fires activation-style handlers.
func DriveClickAt(h *Harness, x, y float32) {
	if h == nil {
		return
	}
	Warmup(h)
	h.InjectEvent(PointerMove(x, y))
	h.RunFrame()
	h.InjectEvent(PointerPress(x, y, platform.PointerLeft))
	h.RunFrame()
	h.InjectEvent(PointerRelease(x, y, platform.PointerLeft))
	h.RunFrame()
}

// DriveKeyPress routes a single key press through the runtime.
func DriveKeyPress(h *Harness, key platform.Key, mods platform.ModifierKeys) {
	if h == nil {
		return
	}
	Warmup(h)
	h.InjectEvent(KeyPress(key, mods))
	h.RunFrame()
}

// DriveKeyRelease routes a single key release through the runtime.
func DriveKeyRelease(h *Harness, key platform.Key, mods platform.ModifierKeys) {
	if h == nil {
		return
	}
	Warmup(h)
	h.InjectEvent(KeyRelease(key, mods))
	h.RunFrame()
}

// DriveType injects a sequence of text events.
func DriveType(h *Harness, s string) {
	if h == nil {
		return
	}
	Warmup(h)
	h.InjectEvent(TypeText(s))
	h.RunFrame()
}

// DriveDragFromTo routes a press at (fromX,fromY), interpolated moves to (toX,toY),
// and a release, running a frame after each event so hit-testing and hover
// track the moving pointer.
func DriveDragFromTo(h *Harness, fromX, fromY, toX, toY float32) {
	if h == nil {
		return
	}
	Warmup(h)
	for _, e := range Drag(fromX, fromY, toX, toY) {
		h.InjectEvent(e)
		h.RunFrame()
	}
}

// DriveScroll routes a scroll event at (x,y) with the given delta.
func DriveScroll(h *Harness, x, y, deltaX, deltaY float32) {
	if h == nil {
		return
	}
	Warmup(h)
	h.InjectEvent(Scroll(x, y, deltaX, deltaY))
	h.RunFrame()
}

func awaitDriveReply(t testing.TB, controlTurn func() error, runFrame func(), reply <-chan drive.Result, maxFrames int, method string) (json.RawMessage, *proto.Error) {
	t.Helper()
	for i := 0; i <= maxFrames; i++ {
		if err := controlTurn(); err != nil {
			t.Fatalf("testkit: drive control turn: %v", err)
		}
		select {
		case res := <-reply:
			if res.Err == nil {
				return res.Payload, nil
			}
			var perr *proto.Error
			if !errors.As(res.Err, &perr) {
				perr = &proto.Error{Code: proto.ErrInternalPanic, Detail: res.Err.Error()}
			}
			return res.Payload, perr
		default:
		}
		if i == maxFrames {
			break
		}
		runFrame()
	}
	t.Fatalf("testkit: drive %q did not reply within %d frames", method, maxFrames)
	return nil, nil
}

// harnessDriveClient is one Harness's sequential trusted session client.
type harnessDriveClient struct {
	mu   sync.Mutex
	sess *drive.Session
	next uint64
}

// DriveCall submits one drive method through the in-process port and runs
// frames until the runtime thread replies at a cut. The trusted session
// carries the full grant set; admission (ledger claim, grants, dispatch)
// is the same runtime-thread path a socket command takes.
func (h *Harness) DriveCall(t testing.TB, method string, params any) (json.RawMessage, *proto.Error) {
	t.Helper()
	h.driveMu.Lock()
	c := h.driveClient
	if c == nil {
		if h.driveSessions == nil {
			h.driveMu.Unlock()
			t.Fatalf("testkit: drive not enabled on this harness")
		}
		sess, _, err := h.driveSessions.Create(drive.Grants{
			Read: true, Input: true, Command: true, Time: true, Admin: true,
		})
		if err != nil {
			h.driveMu.Unlock()
			t.Fatalf("testkit: drive session: %v", err)
		}
		c = &harnessDriveClient{sess: sess}
		h.driveClient = c
	}
	h.driveMu.Unlock()
	var raw json.RawMessage
	if params != nil {
		enc, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal drive params: %v", err)
		}
		raw = enc
	}
	c.mu.Lock()
	c.next++
	id := c.next
	cmd := drive.Command{
		Trusted:    true,
		Session:    c.sess.ID,
		Generation: c.sess.Generation(),
		ID:         id,
		Method:     method,
		Params:     raw,
		Grants:     c.sess.Grants,
		Reply:      make(chan drive.Result, 1),
	}
	c.mu.Unlock()
	if err := h.rt.DrivePort().Submit(cmd); err != nil {
		t.Fatalf("testkit: submit drive %q: %v", method, err)
	}
	return awaitDriveReply(t, h.rt.RunControlTurn, h.RunFrame, cmd.Reply, 256, method)
}
