package runtime

import (
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
)

// Test-side drive seam plumbing: every drive test runtime gets a real
// session table and ledger admission, the same path a socket command
// takes. The trusted in-process session carries the full grant set; grant
// scoping is the socket path's concern.

var (
	testClientMu sync.Mutex
	testClients  = map[*Runtime]*driveTestClient{}
)

// allGrants is the trusted in-process session grant set.
func allGrants() drive.Grants {
	return drive.Grants{Read: true, Input: true, Command: true, Time: true, Admin: true}
}

// testSessionTable creates the instance/session pair for one test runtime
// and installs it on cfg.
func testSessionTable(t testing.TB, cfg *Config) *drive.SessionTable {
	t.Helper()
	inst, err := drive.NewInstance()
	if err != nil {
		t.Fatalf("drive instance: %v", err)
	}
	cfg.Drive.Sessions = drive.NewSessionTable(inst)
	cfg.Drive.Stats = &drive.Stats{}
	return cfg.Drive.Sessions
}

// driveTestClient is one trusted session's sequential call channel: it
// allocates contiguous request IDs and submits through the runtime port.
type driveTestClient struct {
	rt   *Runtime
	sess *drive.Session
	next uint64
}

// testClientFor lazily binds one client to a runtime's session table.
func testClientFor(t testing.TB, rt *Runtime) *driveTestClient {
	testClientMu.Lock()
	defer testClientMu.Unlock()
	c, ok := testClients[rt]
	if ok {
		return c
	}
	if rt.drivePort == nil {
		t.Fatalf("runtime drive port not installed")
	}
	sess, _, err := rt.config.Drive.Sessions.Create(allGrants())
	if err != nil {
		t.Fatalf("drive session: %v", err)
	}
	c = &driveTestClient{rt: rt, sess: sess}
	testClients[rt] = c
	return c
}

// call submits one command and runs frames until the runtime thread has
// replied at a cut (the manual-frame equivalent of a socket round trip).
func (c *driveTestClient) call(t testing.TB, method string, params any, grants drive.Grants) drive.Result {
	t.Helper()
	var raw json.RawMessage
	if params != nil {
		enc, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
		raw = enc
	}
	c.next++
	id := c.next
	cmd := drive.Command{
		Trusted:    true,
		Session:    c.sess.ID,
		Generation: c.sess.Generation(),
		ID:         id,
		OpID:       c.rt.config.Drive.Sessions.InstanceID() + "/" + c.sess.ID + "/" + strconv.FormatUint(id, 10),
		Method:     method,
		Params:     raw,
		Grants:     grants,
		Reply:      make(chan drive.Result, 1),
	}
	if err := c.rt.DrivePort().Submit(cmd); err != nil {
		t.Fatalf("submit drive %q: %v", method, err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 128; i++ {
		c.rt.RunOneFrameAt(base.Add(time.Duration(i) * time.Second))
		select {
		case res := <-cmd.Reply:
			return res
		default:
		}
	}
	t.Fatalf("drive %q did not reply within 128 frames", method)
	return drive.Result{}
}

// driveCall enqueues a command and runs frames until the runtime thread
// has replied (the manual-frame equivalent of the socket round trip).
func driveCall(t testing.TB, rt *Runtime, method string, params any, grants drive.Grants) drive.Result {
	t.Helper()
	return testClientFor(t, rt).call(t, method, params, grants)
}

// resultCode extracts a typed error's code ("" when the result succeeded).
func resultCode(err error) string {
	if err == nil {
		return ""
	}
	var perr *proto.Error
	if errors.As(err, &perr) {
		return string(perr.Code)
	}
	return err.Error()
}

// errDetail extracts a typed error's detail text.
func errDetail(err error) string {
	var perr *proto.Error
	if errors.As(err, &perr) {
		return perr.Detail
	}
	if err != nil {
		return err.Error()
	}
	return ""
}
