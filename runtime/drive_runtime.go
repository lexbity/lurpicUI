package runtime

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

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
	// wake unblocks the parked Virtual-mode Run loop (QP-5); buffered 1 so
	// Enqueue never blocks.
	wake chan struct{}
	// pendingAdvances are virtual-time advances requested at a cut by the
	// time.advance handler; drivePostFrame executes each as exactly one full
	// frame and then replies. Runtime-thread only.
	pendingAdvances []pendingAdvance
	// drainedThisFrame feeds the per-frame FrameStats counter.
	drainedThisFrame int
	// recentWrites is the drive frame log (QP-12): the last 64 store.write
	// addresses, exposed via sys.info until Phase 7's frame.completed
	// subscription consumes the stream.
	recentWrites []string
	// preempted is the interim own.preempted buffer (QP-11): preemption
	// notifications until Phase 7's subscription stream replaces it. DELETED
	// in Phase 7 — no compat.
	preempted []PreemptedNotice
}

// PreemptedNotice is one buffered own.preempted notification (interim
// until P7).
type PreemptedNotice struct {
	CommandID uint64 `json:"command_id"`
	ClientID  string `json:"client_id"`
}

// bufferPreempted records an own.preempted notification (QP-11 interim).
func (s *driveRuntimeSink) bufferPreempted(commandID uint64, clientID string) {
	s.preempted = append(s.preempted, PreemptedNotice{CommandID: commandID, ClientID: clientID})
	if len(s.preempted) > 64 {
		s.preempted = s.preempted[len(s.preempted)-64:]
	}
}

// recordWrite appends one store.write address to the bounded frame log.
func (s *driveRuntimeSink) recordWrite(address string) {
	s.recentWrites = append(s.recentWrites, address)
	if len(s.recentWrites) > 64 {
		s.recentWrites = s.recentWrites[len(s.recentWrites)-64:]
	}
}

// pendingAdvance pairs an advance duration with the command awaiting its
// post-frame reply.
type pendingAdvance struct {
	d   time.Duration
	cmd drive.Command
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
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Epoch implements drive.Sink: the frame epoch at the last cut.
func (s *driveRuntimeSink) Epoch() uint64 { return s.epoch.Load() }

// installDriveSink wires the drive sink into the runtime. Called from New
// when cfg.Drive.Enabled. LDP is otherwise absent from the runtime: the
// frame loop sees one nil pointer check per frame (NFR-4).
func (rt *Runtime) installDriveSink() {
	rt.driveSink = &driveRuntimeSink{rt: rt, stats: &drive.Stats{}, wake: make(chan struct{}, 1)}
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
	s.drainedThisFrame += len(commands)
	for _, c := range commands {
		rt.driveDispatch(c)
	}
	return len(commands)
}

// driveOutcome is a handler's result: either an immediate Result, or a
// deferred one (time.advance replies only after its frame has executed).
type driveOutcome struct {
	result   drive.Result
	deferred bool
}

func payloadOutcome(payload json.RawMessage, frame uint64) driveOutcome {
	return driveOutcome{result: drive.Result{Payload: payload, Frame: frame}}
}

func errorOutcome(perr *proto.Error, frame uint64) driveOutcome {
	return driveOutcome{result: drive.Result{Err: perr, Frame: frame}}
}

// driveHandler executes one drive method on the runtime thread. Handlers run
// at a cut; only time.advance defers its reply (to after its frame).
type driveHandler func(rt *Runtime, cmd drive.Command) driveOutcome

// driveMethods is the runtime-side method table. The server-side table owns
// connection concerns (framing, auth); this table owns execution semantics.
// Grant names live in the server table (drive.MethodGrant) — one source.
// Populated in init: a composite literal initializer would form an
// initialization cycle through the step executor back into the map.
var driveMethods map[string]driveHandler

func init() {
	driveMethods = map[string]driveHandler{
		"sys.ping":       drivePing,
		"sys.info":       driveInfo,
		"time.mode":      driveTimeMode,
		"time.advance":   driveTimeAdvance,
		"frame.run":      driveFrameRun,
		"locate.resolve": driveLocateResolve,
		"tree.dump":      driveTreeDump,
		"store.list":     driveStoreList,
		"store.read":     driveStoreRead,
		"store.write":    driveStoreWrite,
		"derived.read":   driveDerivedRead,
		"facet.query":    driveFacetQuery,
		"frame.pixels":   driveFramePixels,
		"frame.packet":   driveFramePacket,
		"input.click":    driveInputClick,
		"input.press":    driveInputPress,
		"input.release":  driveInputRelease,
		"input.move":     driveInputMove,
		"input.drag":     driveInputDrag,
		"input.type":     driveInputType,
		"input.scroll":   driveInputScroll,
		"input.focus":    driveInputFocus,
		"cmd.list":       driveCmdList,
		"cmd.execute":    driveCmdExecute,
	}
}

func drivePing(rt *Runtime, _ drive.Command) driveOutcome {
	payload, err := json.Marshal(map[string]bool{"pong": true})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, rt.driveSink.Epoch())
	}
	return payloadOutcome(payload, rt.driveSink.Epoch())
}

func driveInfo(rt *Runtime, _ drive.Command) driveOutcome {
	drained, sent, denied, panics := rt.driveSink.stats.Snapshot()
	payload, err := json.Marshal(map[string]any{
		"v":       proto.ProtocolMajor,
		"frame":   rt.driveSink.Epoch(),
		"methods": driveInfoMethodList,
		"drive": map[string]any{
			"commandsDrained":       drained,
			"responsesSent":         sent,
			"grantDenied":           denied,
			"socketGoroutinePanics": panics,
			"recentWrites":          rt.driveSink.recentWrites,
			"namedStores":           rt.namedStores.Names(),
		},
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternal, Detail: err.Error()}, rt.driveSink.Epoch())
	}
	return payloadOutcome(payload, rt.driveSink.Epoch())
}

// driveInfoMethodList is the sorted method list reported by sys.info. It is
// a separate literal (not derived from driveMethods) to break the
// package-level initialization cycle map→handler→map; a test asserts the
// two stay in sync.
var driveInfoMethodList = []string{"cmd.execute", "cmd.list", "derived.read", "facet.query", "frame.packet", "frame.pixels", "frame.run", "input.click", "input.drag", "input.focus", "input.move", "input.press", "input.release", "input.scroll", "input.type", "locate.resolve", "store.list", "store.read", "store.write", "sys.info", "sys.ping", "time.advance", "time.mode", "tree.dump"}

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
	// Grant check at the execution seam too (FR-3): the socket server checks
	// per request, the runtime re-checks so the in-process harness path and
	// frame.run steps cannot bypass it.
	if grant, _ := drive.MethodGrant(cmd.Method); grant != "" && !cmd.Grants.Allows(grant) {
		rt.driveSink.stats.AddGrantDenied()
		cmd.Reply <- drive.Result{
			Err:   &proto.Error{Code: proto.ErrGrant, Detail: grant},
			Frame: frame(),
		}
		return
	}
	out := handler(rt, cmd)
	if out.deferred {
		return
	}
	cmd.Reply <- out.result
}

// drivePostFrame completes virtual-time advances deferred at the last cut:
// each pending advance moves the clock and runs exactly one full frame, then
// replies with the new epoch (FR-5). Advances orphaned by a mode flip back
// to Wall are refused with a typed error, never silently dropped.
func (rt *Runtime) drivePostFrame() {
	if rt.driveSink == nil {
		return
	}
	pending := rt.driveSink.pendingAdvances
	if len(pending) == 0 {
		return
	}
	rt.driveSink.pendingAdvances = nil
	for _, adv := range pending {
		if rt.clock.Mode() != ClockVirtual {
			adv.cmd.Reply <- drive.Result{
				Err: &proto.Error{
					Code:   proto.ErrTimeMode,
					Detail: "clock left virtual mode before the advance executed",
				},
				Frame: rt.driveSink.Epoch(),
			}
			continue
		}
		now := rt.clock.Advance(adv.d)
		rt.runFrame(now, true)
		payload, err := json.Marshal(map[string]uint64{"frame": rt.frameNumber})
		if err != nil {
			adv.cmd.Reply <- drive.Result{
				Err:   &proto.Error{Code: proto.ErrInternal, Detail: err.Error()},
				Frame: rt.driveSink.Epoch(),
			}
			continue
		}
		adv.cmd.Reply <- drive.Result{Payload: payload, Frame: rt.frameNumber}
	}
}
