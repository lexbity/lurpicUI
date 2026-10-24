package runtime

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"codeburg.org/lexbit/lurpicui/drive/proto"
	drive "codeburg.org/lexbit/lurpicui/drive/server"
	"codeburg.org/lexbit/lurpicui/internal/syncutil"
)

// driveRuntimeSink is the runtime-side implementation of drive.Sink (QP-1).
// The server enqueues commands from its connection goroutines; the runtime
// thread drains the inbox at the head of phase 2 (see runFrame) and executes
// every handler on THIS thread, at a cut (QP-6). Responses cross back
// through per-command buffered reply channels; the server owns the socket.
type driveRuntimeSink struct {
	rt    *Runtime
	stats *drive.Stats
	mu    sync.Mutex // guards inbox append vs. swap-nil drain
	inbox atomic.Pointer[[]drive.Command]
	epoch atomic.Uint64
}

// Enqueue implements drive.Sink. Thread-safe; the command executes at the
// next cut on the runtime thread.
func (s *driveRuntimeSink) Enqueue(cmd drive.Command) {
	if cmd.Reply == nil {
		panic("runtime: drive command without a reply channel")
	}
	s.mu.Lock()
	pending := s.inbox.Load()
	next := make([]drive.Command, 0, 1)
	if pending != nil {
		next = append(next, *pending...)
	}
	next = append(next, cmd)
	s.inbox.Store(&next)
	s.mu.Unlock()
	if s.rt.frameTimer != nil {
		s.rt.frameTimer.RequestFrame()
	}
}

// Epoch implements drive.Sink: the frame epoch at the last cut.
func (s *driveRuntimeSink) Epoch() uint64 { return s.epoch.Load() }

// installDriveSink wires the drive sink into the runtime. Called from New
// when cfg.Drive.Enabled. LDP is otherwise absent from the runtime: the
// frame loop sees one nil pointer check per frame (NFR-4).
func (rt *Runtime) installDriveSink() {
	rt.driveSink = &driveRuntimeSink{rt: rt, stats: &drive.Stats{}}
}

// DriveSink returns the drive seam, or nil when LDP is disabled (FR-4).
func (rt *Runtime) DriveSink() drive.Sink {
	if rt.driveSink == nil {
		return nil
	}
	return rt.driveSink
}

// DriveStats returns the shared drive counters, or nil when LDP is disabled.
func (rt *Runtime) DriveStats() *drive.Stats {
	if rt.driveSink == nil {
		return nil
	}
	return rt.driveSink.stats
}

// driveDrain executes pending drive commands on the runtime thread at a cut.
// Returns the number of commands drained. Zero clients ⇒ inbox nil ⇒ one
// atomic load, no allocation.
func (rt *Runtime) driveDrain() int {
	if rt.driveSink == nil {
		return 0
	}
	s := rt.driveSink
	pending := s.inbox.Swap(nil)
	if pending == nil {
		return 0
	}
	commands := *pending
	// Counted before execution so a sys.info in this batch observes the
	// batch it belongs to.
	s.stats.AddCommandsDrained(len(commands))
	for _, c := range commands {
		rt.driveDispatch(c)
	}
	return len(commands)
}

// driveHandler executes one drive method on the runtime thread.
type driveHandler func(rt *Runtime, params json.RawMessage) (json.RawMessage, *proto.Error)

// driveMethods is the runtime-side method table. The server-side table owns
// grants and connection concerns; this table owns execution semantics.
var driveMethods = map[string]driveHandler{
	"sys.ping": drivePing,
	"sys.info": driveInfo,
}

func drivePing(_ *Runtime, _ json.RawMessage) (json.RawMessage, *proto.Error) {
	payload, err := json.Marshal(map[string]bool{"pong": true})
	if err != nil {
		return nil, &proto.Error{Code: proto.ErrInternal, Detail: err.Error()}
	}
	return payload, nil
}

func driveInfo(rt *Runtime, _ json.RawMessage) (json.RawMessage, *proto.Error) {
	drained, sent, denied, panics := rt.driveSink.stats.Snapshot()
	payload, err := json.Marshal(map[string]any{
		"v":       proto.ProtocolMajor,
		"frame":   rt.driveSink.Epoch(),
		"methods": driveInfoMethodList,
		"drive": map[string]uint64{
			"commandsDrained":       drained,
			"responsesSent":         sent,
			"grantDenied":           denied,
			"socketGoroutinePanics": panics,
		},
	})
	if err != nil {
		return nil, &proto.Error{Code: proto.ErrInternal, Detail: err.Error()}
	}
	return payload, nil
}

// driveInfoMethodList is the sorted method list reported by sys.info. It is
// a separate literal (not derived from driveMethods) to break the
// package-level initialization cycle map→handler→map; a test asserts the
// two stay in sync.
var driveInfoMethodList = []string{"sys.info", "sys.ping"}

// driveDispatch runs one command's handler on the runtime thread and replies
// exactly once. A handler panic is converted to a typed internal.panic
// result (QP-16) — the connection goroutine must never block on a lost
// reply, and the app must never crash (NFR-7).
func (rt *Runtime) driveDispatch(cmd drive.Command) {
	syncutil.AssertRuntimeThread()
	frame := func() uint64 { return rt.driveSink.Epoch() }
	defer func() {
		if r := recover(); r != nil {
			cmd.Reply <- drive.Result{
				Err:   &proto.Error{Code: proto.ErrInternal, Detail: fmt.Sprint(r)},
				Frame: frame(),
			}
		}
	}()
	handler, ok := driveMethods[cmd.Method]
	if !ok {
		cmd.Reply <- drive.Result{
			Err:   &proto.Error{Code: proto.ErrInternal, Detail: "unknown method: " + cmd.Method},
			Frame: frame(),
		}
		return
	}
	payload, perr := handler(rt, cmd.Params)
	cmd.Reply <- drive.Result{Payload: payload, Err: perr, Frame: frame()}
}
