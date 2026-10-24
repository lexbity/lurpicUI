package testkit

import (
	"encoding/json"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive/proto"
	drive "codeburg.org/lexbit/lurpicui/drive/server"
	"codeburg.org/lexbit/lurpicui/facet"
)

// driveReplyFrames bounds how many frames Call runs while waiting for the
// runtime thread to drain the command at a cut.
const driveReplyFrames = 128

// DriveHarness boots a facet tree on the testkit runtime with the drive sink
// installed in-process (no socket) and exposes Call(method, params), which
// enqueues a drive command and runs frames until the runtime thread has
// executed it at a cut — the in-process equivalent of a socket round trip
// (NFR-1 in-process path).
type DriveHarness struct {
	*Harness
	nextID uint64
}

// NewDriveHarness constructs the in-process drive harness.
func NewDriveHarness(t testing.TB, w, h float32, root facet.FacetImpl) *DriveHarness {
	t.Helper()
	cfg := HarnessConfig{
		Width:         int(w),
		Height:        int(h),
		LayerRegistry: StandardLayerRegistry(t),
		Drive:         true,
	}
	return NewDriveHarnessWithConfig(t, cfg, root)
}

// NewDriveHarnessWithConfig boots the drive harness with a caller-built
// config (the harness layer fills LayerRegistry/Drive defaults when unset).
func NewDriveHarnessWithConfig(t testing.TB, cfg HarnessConfig, root facet.FacetImpl) *DriveHarness {
	t.Helper()
	cfg.Drive = true
	if cfg.Width <= 0 {
		cfg.Width = 480
	}
	if cfg.Height <= 0 {
		cfg.Height = 240
	}
	if cfg.LayerRegistry == nil {
		cfg.LayerRegistry = StandardLayerRegistry(t)
	}
	return &DriveHarness{Harness: NewHarness(t, cfg, root)}
}

// Sink returns the runtime's drive seam.
func (h *DriveHarness) Sink() drive.Sink {
	if h.rt == nil || h.rt.DriveSink() == nil {
		h.t.Fatal("testkit: drive sink not installed")
	}
	return h.rt.DriveSink()
}

// Call executes one drive method at a cut and returns its payload or typed
// error. Fails the test if the runtime does not reply within
// driveReplyFrames frames.
func (h *DriveHarness) Call(method string, params any) (json.RawMessage, *proto.Error) {
	h.t.Helper()
	var raw json.RawMessage
	if params != nil {
		enc, err := json.Marshal(params)
		if err != nil {
			h.t.Fatalf("testkit: marshal drive params: %v", err)
		}
		raw = enc
	}
	h.nextID++
	// The in-process harness is the trusted path: commands carry the full
	// grant set (the socket path is where grants are scoped).
	cmd := drive.Command{
		ID:     h.nextID,
		Method: method,
		Params: raw,
		Grants: drive.Grants{Read: true, Input: true, Write: true, Time: true, Admin: true},
		Reply:  make(chan drive.Result, 1),
	}
	h.Sink().Enqueue(cmd)
	for i := 0; i < driveReplyFrames; i++ {
		h.RunFrame()
		select {
		case res := <-cmd.Reply:
			return res.Payload, res.Err
		default:
		}
	}
	h.t.Fatalf("testkit: drive method %q did not reply within %d frames", method, driveReplyFrames)
	return nil, nil
}

// CallOK is Call with a success assertion, returning the payload.
func (h *DriveHarness) CallOK(method string, params any) json.RawMessage {
	h.t.Helper()
	payload, perr := h.Call(method, params)
	if perr != nil {
		h.t.Fatalf("testkit: drive %q failed: %v", method, perr)
	}
	return payload
}
