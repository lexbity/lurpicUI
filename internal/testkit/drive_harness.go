package testkit

import (
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/facet"
)

// driveReplyFrames bounds how many frames Call runs while waiting for the
// runtime thread to drain the command at a cut.
const driveReplyFrames = 128

// DriveHarness boots a facet tree on the testkit runtime with the drive
// port installed in-process (no socket) and exposes Call(method, params),
// which submits a drive command and runs frames until the runtime thread
// has admitted and executed it at a cut — the in-process equivalent of a
// socket round trip.
type DriveHarness struct {
	*Harness
	nextID   uint64
	sessions *drive.SessionTable
	session  *drive.Session
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
	if cfg.DriveSessions == nil {
		inst, err := drive.NewInstance()
		if err != nil {
			t.Fatalf("testkit: drive instance: %v", err)
		}
		cfg.DriveSessions = drive.NewSessionTable(inst)
	}
	sessions := cfg.DriveSessions
	// The trusted in-process session carries the full grant set; grant
	// scoping is the socket path's concern.
	sess, _, err := sessions.Create(drive.Grants{Read: true, Input: true, Command: true, Time: true, Admin: true})
	if err != nil {
		t.Fatalf("testkit: drive session: %v", err)
	}
	h := &DriveHarness{Harness: NewHarness(t, cfg, root), sessions: sessions, session: sess}
	return h
}

// Port returns the runtime's drive seam.
func (h *DriveHarness) Port() drive.RuntimePort {
	if h.rt == nil || h.rt.DrivePort() == nil {
		h.t.Fatal("testkit: drive port not installed")
	}
	return h.rt.DrivePort()
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
	// grant set (the socket path is where grants are scoped). Admission —
	// ledger claim, grants, dispatch — is the same runtime-thread path the
	// socket uses.
	cmd := drive.Command{
		Session:    h.session.ID,
		Generation: h.session.Generation(),
		ID:         h.nextID,
		OpID:       h.sessions.InstanceID() + "/" + h.session.ID + "/" + strconv.FormatUint(h.nextID, 10),
		Method:     method,
		Params:     raw,
		Grants:     h.session.Grants,
		Reply:      make(chan drive.Result, 1),
	}
	if err := h.Port().Submit(cmd); err != nil {
		h.t.Fatalf("testkit: submit drive %q: %v", method, err)
	}
	for i := 0; i < driveReplyFrames; i++ {
		h.RunFrame()
		select {
		case res := <-cmd.Reply:
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

// Sessions exposes the harness's drive session table (the socket test
// path binds a real server against it).
func (h *DriveHarness) Sessions() *drive.SessionTable { return h.sessions }
