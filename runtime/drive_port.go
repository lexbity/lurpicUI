package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/job"
)

// drivePort is the runtime-side implementation of drive.RuntimePort. The
// transport submits commands from its connection goroutines; the runtime
// thread admits and executes every command on THIS thread, at a cut:
// ledger claim → grant/availability check → handler → immutable terminal
// record → exactly-once reply. All admission decisions are made here —
// socket receipt is not acceptance.
type drivePort struct {
	rt       *Runtime
	stats    *drive.Stats
	sessions *drive.SessionTable

	// inbox is the bounded MPSC queue with ONE synchronization protocol:
	// append and claim-and-clear both happen under mu; nonEmpty is an
	// atomic idle fast path so a zero-client frame pays one atomic load.
	inboxMu   sync.Mutex
	inbox     []drive.Command
	nonEmpty  atomic.Bool
	inboxFull atomic.Uint64 // cumulative explicit overload events

	// epoch is the last submitted frame number (decimal via Observe).
	epoch atomic.Uint64
	// cut is the sealed cut identifier: it advances when a frame is sealed.
	cut atomic.Uint64

	// wake unblocks the parked Virtual-mode Run loop; buffered 1 so Submit
	// never blocks.
	wake chan struct{}

	// controlMu guards the coalesced control signals observed at the start
	// of each runtime turn — a full ordinary inbox can never suppress a
	// disconnect or server-stop wake.
	controlMu     sync.Mutex
	controlKeys   map[controlKey]int
	controlWanted atomic.Bool

	// pendingAdvances are virtual-time advances admitted at a cut by the
	// time.advance handler; drivePostFrame executes each as exactly one
	// full frame and then completes the ledger + reply. Runtime-thread only.
	pendingAdvances []pendingAdvance
	// pendingCommandActivations are admitted GUI-backed commands waiting
	// for the next normal frame's activation boundary. pendingCommandFrames
	// retain their immutable outcome until that frame's render acknowledgment.
	pendingCommandActivations []pendingCommand
	pendingCommandFrames      map[uint64][]pendingCommand
	waitingCommandJobs        map[string]pendingCommand
	commandJobScopes          map[job.JobID]*commandJobScope
	activeCommandCount        int

	// drainedThisFrame feeds the per-frame FrameStats counter.
	drainedThisFrame int
}

// controlKey deduplicates control signals per (session, generation).
type controlKey struct {
	session    string
	generation uint64
}

// maxInboxBounds is the bounded inbox capacity: Submit beyond it fails
// explicitly (capacity.busy, non-admitted), never blocks or grows without
// bound.
const maxInboxLen = 65536

// ErrInboxFull is Submit's explicit overload result.
var ErrInboxFull = errors.New("runtime: drive inbox at capacity")

// Submit implements drive.RuntimePort. Thread-safe; bounded; the command
// is admitted at the next cut on the runtime thread.
func (p *drivePort) Submit(cmd drive.Command) error {
	if cmd.Reply == nil {
		panic("runtime: drive command without a reply channel")
	}
	p.inboxMu.Lock()
	if len(p.inbox) >= maxInboxLen {
		p.inboxMu.Unlock()
		p.inboxFull.Add(1)
		return ErrInboxFull
	}
	p.inbox = append(p.inbox, cmd)
	p.inboxMu.Unlock()
	p.nonEmpty.Store(true)
	if p.rt.frameTimer != nil {
		p.rt.frameTimer.RequestFrame()
	}
	select {
	case p.wake <- struct{}{}:
	default:
	}
	return nil
}

// SignalControl implements drive.RuntimePort: a disconnect, lease expiry,
// or server stop is recorded for the next runtime turn and wakes the loop.
// Signals are generation-tagged and coalesced per (session, generation).
func (p *drivePort) SignalControl(sig drive.ControlSignal) {
	p.controlMu.Lock()
	key := controlKey{session: sig.Session, generation: sig.Generation}
	p.controlKeys[key]++
	p.controlMu.Unlock()
	p.controlWanted.Store(true)
	select {
	case p.wake <- struct{}{}:
	default:
	}
	if p.rt.frameTimer != nil {
		p.rt.frameTimer.RequestFrame()
	}
}

// Observe implements drive.RuntimePort: the immutable published protocol
// snapshot.
func (p *drivePort) Observe() drive.PublishedSnapshot {
	inst := ""
	if p.sessions != nil {
		inst = p.sessions.InstanceID()
	}
	return drive.PublishedSnapshot{
		Cut:        strconv.FormatUint(p.cut.Load(), 10),
		Frame:      strconv.FormatUint(p.epoch.Load(), 10),
		InstanceID: inst,
	}
}

// Cut returns the sealed cut identifier (decimal string attribution for
// replies).
func (p *drivePort) Cut() string { return strconv.FormatUint(p.cut.Load(), 10) }

// Frame returns the last submitted frame (decimal string).
func (p *drivePort) Frame() string { return strconv.FormatUint(p.epoch.Load(), 10) }

// Epoch returns the numeric last submitted frame (trusted-seam handlers).
func (p *drivePort) Epoch() uint64 { return p.epoch.Load() }

// installDrivePort wires the runtime side of the drive seam. Called from
// New when cfg.Drive.Enabled. LDP is otherwise absent from the runtime:
// an idle frame sees one atomic load (NFR/AC-15).
func (rt *Runtime) installDrivePort() error {
	if rt.config.Drive.Sessions == nil {
		return errors.New("runtime: drive enabled without a session table")
	}
	rt.drivePort = &drivePort{
		rt:                   rt,
		stats:                rt.config.Drive.Stats,
		sessions:             rt.config.Drive.Sessions,
		wake:                 make(chan struct{}, 1),
		controlKeys:          make(map[controlKey]int),
		pendingCommandFrames: make(map[uint64][]pendingCommand),
		waitingCommandJobs:   make(map[string]pendingCommand),
		commandJobScopes:     make(map[job.JobID]*commandJobScope),
	}
	if rt.drivePort.stats == nil {
		rt.drivePort.stats = &drive.Stats{}
	}
	return nil
}

// DrivePort returns the drive seam, or nil when LDP is disabled.
func (rt *Runtime) DrivePort() drive.RuntimePort {
	if rt.drivePort == nil {
		return nil
	}
	return rt.drivePort
}

// DriveStats returns the shared drive counters, or nil when LDP is disabled.
func (rt *Runtime) DriveStats() *drive.Stats {
	if rt.drivePort == nil {
		return nil
	}
	return rt.drivePort.stats
}

// driveClaim pops the queued commands under the single inbox mutex.
// Exactly once: a command appears in exactly one claimed batch because
// append and clear share the mutex; a claimed batch is never re-queued.
func (p *drivePort) claim() []drive.Command {
	if !p.nonEmpty.Load() {
		return nil
	}
	p.inboxMu.Lock()
	cmds := p.inbox
	p.inbox = nil
	p.inboxMu.Unlock()
	p.nonEmpty.Store(false)
	return cmds
}

// serviceControl processes coalesced control signals before any other
// work of the turn: lost generations fence their accepted work before
// another atom is selected.
func (p *drivePort) serviceControl() {
	if !p.controlWanted.Load() {
		return
	}
	p.controlMu.Lock()
	keys := p.controlKeys
	p.controlKeys = make(map[controlKey]int)
	p.controlMu.Unlock()
	p.controlWanted.Store(false)
	for key := range keys {
		p.fenceGeneration(key.session, key.generation)
	}
}

// fenceGeneration cancels the unstarted remainder of a lost generation's
// work at this safe boundary. S1 work is single-step, so fencing cancels
// an in-flight drive input step owned by that session/generation; applied
// effects stand and nothing new starts.
func (p *drivePort) fenceGeneration(session string, generation uint64) {
	for i := 0; i < len(p.pendingCommandActivations); {
		pending := p.pendingCommandActivations[i]
		if pending.cmd.Session == session && pending.cmd.Generation == generation {
			p.pendingCommandActivations = append(p.pendingCommandActivations[:i], p.pendingCommandActivations[i+1:]...)
			p.finishCommand(pending, drive.Result{Err: &proto.Error{Code: proto.ErrOperationCancelled, Detail: "connection.lost"}})
			continue
		}
		i++
	}
	for frame, pending := range p.pendingCommandFrames {
		kept := pending[:0]
		for _, command := range pending {
			if command.cmd.Session == session && command.cmd.Generation == generation {
				p.finishCommand(command, drive.Result{Err: &proto.Error{Code: proto.ErrOperationCancelled, Detail: "connection.lost"}})
				continue
			}
			kept = append(kept, command)
		}
		if len(kept) == 0 {
			delete(p.pendingCommandFrames, frame)
		} else {
			p.pendingCommandFrames[frame] = kept
		}
	}
	for opID, pending := range p.waitingCommandJobs {
		if pending.cmd.Session == session && pending.cmd.Generation == generation {
			p.finishCommand(pending, drive.Result{Err: &proto.Error{Code: proto.ErrOperationCancelled, Detail: "connection.lost"}})
			delete(p.waitingCommandJobs, opID)
		}
	}
	// A fenced step severs like a human preemption: remaining synthesized
	// events void, recognizers abort through the normal path, ownership
	// returns to the human. Applied effects stand.
	if p.rt.driveInputStep == nil {
		return
	}
	if generation > 0 {
		p.rt.preemptInputStepFor(session, generation)
	} else {
		p.rt.preemptInputStepOwned(func(*driveInputStep) bool { return true })
	}
	p.stats.AddPreempted()
}

// sealCut advances the sealed cut after a frame's submission completes.
// Called once per frame at the seal point.
func (p *drivePort) sealCut(frame uint64) {
	p.epoch.Store(frame)
	p.cut.Store(frame)
}

// driveDrain admits and executes pending drive commands on the runtime
// thread at a cut. Zero clients ⇒ empty fast path ⇒ one atomic load, no
// allocation.
func (rt *Runtime) driveDrain() int {
	if rt.drivePort == nil {
		return 0
	}
	p := rt.drivePort
	p.serviceControl()
	rt.driveCollectRenderCompletions()
	cmds := p.claim()
	if len(cmds) == 0 {
		return 0
	}
	p.stats.AddCommandsDrained(len(cmds))
	p.drainedThisFrame += len(cmds)
	for _, c := range cmds {
		rt.driveAdmit(p, c)
	}
	return len(cmds)
}

// driveAdmit runs the admission algorithm (RFC §Admission) for one
// command on the runtime thread, then executes or replies. A command is
// dispatched at most once; its ledger entry is claimed exactly once.
func (rt *Runtime) driveAdmit(p *drivePort, cmd drive.Command) {
	syncutil.AssertRuntimeThread()
	sess, ok := p.sessions.Get(cmd.Session)
	if !ok {
		// The transport context is authoritative; an unknown session is an
		// admission-context failure, not a schedulable request.
		p.reply(cmd, nil, &proto.Error{Code: proto.ErrSessionExpired, Detail: "unknown session", NonAdmitted: true}, true)
		return
	}
	if sess.Generation() != cmd.Generation || sess.State() != drive.SessionAttached {
		p.reply(cmd, nil, &proto.Error{Code: proto.ErrSessionFenced, Detail: "connection generation is no longer attached", NonAdmitted: true}, true)
		return
	}
	ledger := sess.Ledger()
	canonical := drive.CanonicalRequest(cmd.Method, cmd.Params)
	id := cmd.ID

	status, entry := ledger.Lookup(id)
	switch status {
	case drive.LookupRetained:
		// Retried retained ID: replay the original outcome; never schedule.
		p.stats.AddDuplicateHit()
		if entry.Canonical != canonical {
			p.stats.AddRequestConflict()
			p.reply(cmd, nil, &proto.Error{
				Code:   proto.ErrRequestConflict,
				Detail: "a different well-typed request was admitted under this id",
			}, true)
			return
		}
		if cmd.Method == driveMethodCommandExecute && entry.Admitted && entry.State == drive.EntryAdmitted {
			if !p.attachCommandWaiter(cmd, entry) {
				p.reply(cmd, nil, &proto.Error{Code: proto.ErrCapacityBusy, Retryable: true}, true)
				return
			}
			p.publishCommandReceipt(cmd, entry)
			return
		}
		p.replayRetained(cmd, entry)
		return
	case drive.LookupExpired:
		// Below the high-water mark with no retained result: explicitly
		// expired; it can never execute again.
		p.stats.AddRequestExpired()
		p.reply(cmd, nil, &proto.Error{
			Code:   proto.ErrRequestExpired,
			Detail: "the outcome for this id is no longer retained",
		}, true)
		return
	default:
	}

	// Contiguity: the server never schedules past an unadmitted gap.
	if id != ledger.HighWater()+1 {
		p.reply(cmd, nil, &proto.Error{
			Code:        proto.ErrRequestGap,
			Detail:      fmt.Sprintf("expected id %d", ledger.HighWater()+1),
			NonAdmitted: true,
		}, true)
		return
	}

	// Availability + grants: permanent rejections consume and retain their
	// ID so a lost rejection cannot become a later accepted request.
	spec, known := registrySpec(cmd.Method)
	if !known || (!spec.Advertised && !cmd.Trusted) {
		p.stats.AddAdmissionRejected()
		perr := &proto.Error{Code: proto.ErrUnknownMethod, Detail: "unknown method: " + cmd.Method}
		if rej, err := ledger.ClaimRejection(id, canonical, perr); err == nil {
			p.replyClaimed(cmd, rej, nil, perr)
		} else {
			p.reply(cmd, nil, perr, true)
		}
		return
	}
	if grant := specGrant(spec); grant != "" && !cmd.Grants.Allows(grant) {
		p.stats.AddGrantDenied()
		p.stats.AddAdmissionRejected()
		perr := &proto.Error{Code: proto.ErrGrant, Detail: grant}
		if rej, err := ledger.ClaimRejection(id, canonical, perr); err == nil {
			p.replyClaimed(cmd, rej, nil, perr)
		} else {
			p.reply(cmd, nil, perr, true)
		}
		return
	}
	var commandOutcome driveOutcome
	if cmd.Method == driveMethodCommandExecute {
		if p.activeCommandCount >= maxPendingDriveCommands {
			p.reply(cmd, nil, &proto.Error{Code: proto.ErrCapacityBusy, Retryable: true, NonAdmitted: true}, true)
			return
		}
		if rt.renderPipeline == nil || !rt.isSurfaceReady() {
			perr := &proto.Error{Code: proto.ErrRenderUnavailable, Detail: "no ready render surface for command activation"}
			if rej, claimErr := ledger.ClaimRejection(id, canonical, perr); claimErr == nil {
				p.replyClaimed(cmd, rej, nil, perr)
			} else {
				p.reply(cmd, nil, perr, true)
			}
			return
		}
		commandOutcome = rt.driveExecute(cmd)
		if commandOutcome.result.Err != nil {
			var perr *proto.Error
			if !errors.As(commandOutcome.result.Err, &perr) {
				perr = &proto.Error{Code: proto.ErrInternalPanic, Detail: "command validation failed"}
			}
			if rej, claimErr := ledger.ClaimRejection(id, canonical, perr); claimErr == nil {
				p.replyClaimed(cmd, rej, nil, perr)
			} else {
				p.reply(cmd, nil, perr, true)
			}
			return
		}
	}

	// Claim the ledger entry and advance the high-water atomically on the
	// runtime thread, before scheduling work.
	var claimed *drive.Entry
	var err error
	attached := sess.WithAttachedGeneration(cmd.Generation, func() {
		claimed, err = ledger.Claim(id, canonical, cmd.OpID)
	})
	if !attached {
		p.reply(cmd, nil, &proto.Error{Code: proto.ErrSessionFenced, Detail: "connection generation was replaced before admission", NonAdmitted: true}, true)
		return
	}
	if err != nil {
		p.reply(cmd, nil, &proto.Error{
			Code:        proto.ErrRequestGap,
			Detail:      err.Error(),
			NonAdmitted: true,
		}, true)
		return
	}
	p.stats.AddCommandsAdmitted(1)
	ledger.MarkAccepted(claimed, p.Cut(), p.Frame())
	if cmd.Method == driveMethodCommandExecute {
		p.publishCommandReceipt(cmd, claimed)
	}
	if cmd.Method == driveMethodCommandExecute {
		scope := &commandJobScope{operationID: claimed.OpID, jobs: make(map[job.JobID]struct{})}
		scope.active.Store(true)
		pending := pendingCommand{cmd: cmd, entry: claimed, activation: commandOutcome.activation, jobScope: scope}
		p.pendingCommandActivations = append(p.pendingCommandActivations, pending)
		p.activeCommandCount++
		return
	}

	out := rt.driveExecute(cmd)
	if out.deferred {
		// The deferred completer (drivePostFrame) owns the ledger
		// completion and reply.
		out.entry = claimed
		p.pendingAdvances = append(p.pendingAdvances, pendingAdvance{
			d: out.d, cmd: cmd, entry: claimed,
		})
		return
	}
	p.completeClaimed(claimed, cmd, out.result)
}

// driveExecute dispatches one admitted command to its handler. A handler
// panic becomes a typed internal.panic outcome — the app keeps rendering.
func (rt *Runtime) driveExecute(cmd drive.Command) (out driveOutcome) {
	defer func() {
		if r := recover(); r != nil {
			rt.drivePort.stats.AddHandlerPanic()
			out = driveOutcome{
				result: drive.Result{
					Err: &proto.Error{Code: proto.ErrInternalPanic, Detail: "runtime handler panicked"},
					Cut: rt.drivePort.Cut(), Frame: rt.drivePort.Frame(),
				},
			}
		}
	}()
	handler, ok := driveMethods[cmd.Method]
	if !ok {
		return driveOutcome{result: drive.Result{
			Err: &proto.Error{Code: proto.ErrUnknownMethod, Detail: "unknown method: " + cmd.Method},
			Cut: rt.drivePort.Cut(), Frame: rt.drivePort.Frame(),
		}}
	}
	return handler(rt, cmd)
}

// driveOutcome is a handler's result: either an immediate Result, or a
// deferred one (time.advance replies only after its frame has executed).
type driveOutcome struct {
	result     drive.Result
	deferred   bool
	activation *commandActivation
	// entry is set by the admitter for deferred outcomes so the completer
	// records the immutable terminal record.
	entry *drive.Entry
	// d carries a deferred advance's duration (time.advance only).
	d time.Duration
}

const maxPendingDriveCommands = 128
const maxCommandWaiters = 32

type commandActivation struct {
	name    string
	version uint64
}

type pendingCommand struct {
	cmd        drive.Command
	entry      *drive.Entry
	activation *commandActivation
	failure    *proto.Error
	waiters    []drive.Command
	jobScope   *commandJobScope
}

// commandJobScope is runtime-owned authority for continuations scheduled by
// one GUI command activation. The worker only calls active.Load through the
// commit gate; the runtime thread owns jobs, counts, and failure attribution.
type commandJobScope struct {
	operationID string
	active      atomic.Bool
	jobs        map[job.JobID]struct{}
	total       int
	completed   int
	failure     *proto.Error
}

func (p *drivePort) publishCommandReceipt(cmd drive.Command, entry *drive.Entry) {
	if cmd.Accepted == nil {
		return
	}
	sess, ok := p.sessions.Get(cmd.Session)
	if !ok {
		return
	}
	select {
	case cmd.Accepted <- drive.Receipt{
		OperationID: entry.OpID, AcceptedCut: entry.AcceptedCut,
		AcceptedFrame: entry.AcceptedFrame, NextID: sess.Ledger().HighWater() + 1,
	}:
	default:
	}
}

func (p *drivePort) attachCommandWaiter(cmd drive.Command, entry *drive.Entry) bool {
	appendWaiter := func(pending *pendingCommand) bool {
		if len(pending.waiters) >= maxCommandWaiters {
			return false
		}
		pending.waiters = append(pending.waiters, cmd)
		return true
	}
	for i := range p.pendingCommandActivations {
		if p.pendingCommandActivations[i].entry.OpID == entry.OpID {
			return appendWaiter(&p.pendingCommandActivations[i])
		}
	}
	for frame := range p.pendingCommandFrames {
		pending := p.pendingCommandFrames[frame]
		for i := range pending {
			if pending[i].entry.OpID == entry.OpID {
				if !appendWaiter(&pending[i]) {
					return false
				}
				p.pendingCommandFrames[frame] = pending
				return true
			}
		}
	}
	if pending, ok := p.waitingCommandJobs[entry.OpID]; ok {
		if !appendWaiter(&pending) {
			return false
		}
		p.waitingCommandJobs[entry.OpID] = pending
		return true
	}
	return false
}

// driveActivatePendingCommands runs one accepted application command at the
// runtime's frame activation boundary. The command and the GUI palette share
// CommandRegistry.ExecuteVersion; the socket thread never invokes app code.
func (rt *Runtime) driveActivatePendingCommands() {
	p := rt.drivePort
	if p == nil {
		return
	}
	p.serviceControl()
	if len(p.pendingCommandActivations) == 0 {
		return
	}
	// Application commands execute FIFO through their rendered/job terminal
	// boundary. A later callback cannot start while an earlier activation or
	// one of its owned continuations is still pending.
	if len(p.pendingCommandFrames) != 0 || len(p.waitingCommandJobs) != 0 {
		return
	}
	pending := p.pendingCommandActivations[0]
	p.pendingCommandActivations = p.pendingCommandActivations[1:]
	sess, ok := p.sessions.Get(pending.cmd.Session)
	if !ok || sess.Generation() != pending.cmd.Generation || sess.State() != drive.SessionAttached {
		p.finishCommand(pending, drive.Result{Err: &proto.Error{Code: proto.ErrOperationCancelled, Detail: "connection.lost"}})
		return
	}
	var invoked bool
	var recovered any
	rt.currentDriveJobScope = pending.jobScope
	func() {
		defer func() { recovered = recover() }()
		registry := rt.CommandRegistry()
		if registry != nil {
			invoked = registry.ExecuteVersion(pending.activation.name, pending.activation.version)
		}
	}()
	rt.currentDriveJobScope = nil
	if recovered != nil {
		p.stats.AddHandlerPanic()
		pending.failure = &proto.Error{Code: proto.ErrInternalPanic, Detail: "command callback panicked after activation began; effect is unknown_partial"}
	} else if !invoked {
		pending.failure = &proto.Error{Code: proto.ErrCommandUnavailable, Detail: "command became unavailable before activation"}
	}
	if pending.failure == nil && pending.jobScope.failure != nil {
		pending.failure = pending.jobScope.failure
	}
	p.pendingCommandFrames[rt.frameNumber] = append(p.pendingCommandFrames[rt.frameNumber], pending)
	if len(p.pendingCommandActivations) > 0 && rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

func (p *drivePort) finishCommand(pending pendingCommand, result drive.Result) {
	if pending.jobScope != nil {
		if result.Err != nil {
			p.cancelCommandJobs(pending.jobScope)
		} else if len(pending.jobScope.jobs) == 0 {
			pending.jobScope.active.Store(false)
		}
	}
	delete(p.waitingCommandJobs, pending.entry.OpID)
	if p.activeCommandCount > 0 {
		p.activeCommandCount--
	}
	p.completeClaimed(pending.entry, pending.cmd, result)
	for _, waiter := range pending.waiters {
		select {
		case waiter.Reply <- result:
		default:
		}
	}
}

func (p *drivePort) cancelCommandJobs(scope *commandJobScope) {
	if scope == nil {
		return
	}
	scope.active.Store(false)
	for id := range scope.jobs {
		if p.rt.jobPool != nil {
			p.rt.jobPool.CancelJob(id)
		}
	}
}

func (p *drivePort) registerCommandJob(scope *commandJobScope, id job.JobID) bool {
	if scope == nil || !scope.active.Load() || id == 0 {
		return false
	}
	if _, exists := scope.jobs[id]; exists {
		panic("runtime: command operation scheduled a duplicate job ID")
	}
	if _, exists := p.commandJobScopes[id]; exists {
		panic("runtime: command operation reused an active scoped job ID")
	}
	scope.jobs[id] = struct{}{}
	scope.total++
	p.commandJobScopes[id] = scope
	return true
}

func (p *drivePort) rollbackCommandJob(scope *commandJobScope, id job.JobID, err error) {
	delete(scope.jobs, id)
	delete(p.commandJobScopes, id)
	scope.failure = &proto.Error{Code: proto.ErrCommandFailed, Detail: "could not schedule operation-owned command job"}
	_ = err // app-specific errors are not safe protocol diagnostics
}

func (p *drivePort) commandJobFinished(id job.JobID, cancelled, committed bool, jobErr error, frame uint64) {
	scope := p.commandJobScopes[id]
	if scope == nil {
		return
	}
	delete(p.commandJobScopes, id)
	delete(scope.jobs, id)
	scope.completed++
	if !scope.active.Load() {
		return
	}
	if cancelled || jobErr != nil || !committed {
		scope.failure = &proto.Error{Code: proto.ErrCommandFailed, Detail: "operation-owned command job did not complete"}
		p.cancelCommandJobs(scope)
		if pending, ok := p.waitingCommandJobs[scope.operationID]; ok {
			pending.failure = scope.failure
			p.finishCommand(pending, drive.Result{Err: scope.failure})
		}
		return
	}
	if len(scope.jobs) != 0 {
		return
	}
	if pending, ok := p.waitingCommandJobs[scope.operationID]; ok {
		delete(p.waitingCommandJobs, scope.operationID)
		if scope.failure != nil {
			pending.failure = scope.failure
			p.finishCommand(pending, drive.Result{Err: scope.failure})
			return
		}
		p.pendingCommandFrames[frame] = append(p.pendingCommandFrames[frame], pending)
	}
}

// driveCollectRenderCompletions consumes immutable renderer acknowledgments
// at a runtime control turn. A late acknowledgment cannot revive an operation
// removed by disconnect fencing.
func (rt *Runtime) driveCollectRenderCompletions() {
	p := rt.drivePort
	if p == nil || rt.renderPipeline == nil {
		return
	}
	for {
		select {
		case completion := <-rt.renderPipeline.Completions():
			rt.driveCompleteCommandFrame(p, completion)
		default:
			return
		}
	}
}

func (rt *Runtime) driveCompleteCommandFrame(p *drivePort, completion RenderCompletion) {
	pending := p.pendingCommandFrames[completion.Frame]
	if len(pending) == 0 {
		return
	}
	delete(p.pendingCommandFrames, completion.Frame)
	for _, command := range pending {
		if command.failure == nil && command.jobScope != nil && command.jobScope.failure != nil {
			command.failure = command.jobScope.failure
		}
		if command.failure == nil && completion.Err == nil && completion.Ready && command.jobScope != nil && len(command.jobScope.jobs) > 0 {
			p.waitingCommandJobs[command.entry.OpID] = command
			continue
		}
		result := drive.Result{Cut: p.Cut(), Frame: strconv.FormatUint(completion.Frame, 10)}
		if command.failure != nil {
			result.Err = command.failure
		} else if completion.Err != nil || !completion.Ready {
			detail := "render acknowledgment did not report a ready surface"
			if completion.Err != nil {
				detail = completion.Err.Error()
			}
			result.Err = &proto.Error{Code: proto.ErrRenderFailed, Detail: detail}
		} else {
			ownedJobs, completedJobs := 0, 0
			if command.jobScope != nil {
				ownedJobs = command.jobScope.total
				completedJobs = command.jobScope.completed
			}
			payload, err := json.Marshal(map[string]string{
				"operation_id":   command.entry.OpID,
				"state":          "completed",
				"executed":       command.activation.name,
				"version":        strconv.FormatUint(command.activation.version, 10),
				"applied_frame":  strconv.FormatUint(completion.Frame, 10),
				"render_status":  "ready",
				"ready_backend":  completion.Backend,
				"owned_jobs":     strconv.Itoa(ownedJobs),
				"jobs_completed": strconv.Itoa(completedJobs),
			})
			if err != nil {
				result.Err = &proto.Error{Code: proto.ErrInternalPanic, Detail: "operation outcome encoding failed"}
			} else {
				result.Payload = payload
			}
		}
		p.finishCommand(command, result)
	}
}

func payloadOutcome(payload json.RawMessage) driveOutcome {
	return driveOutcome{result: drive.Result{Payload: payload}}
}

func errorOutcome(perr *proto.Error) driveOutcome {
	return driveOutcome{result: drive.Result{Err: perr}}
}

// reply sends an unclaimed outcome (pre-admission failure) exactly once.
func (p *drivePort) reply(cmd drive.Command, payload json.RawMessage, perr *proto.Error, _ bool) {
	res := drive.Result{Payload: payload, Cut: p.Cut(), Frame: p.Frame()}
	if perr != nil {
		res.Err = perr
	}
	select {
	case cmd.Reply <- res:
	default:
	}
}

// completeClaimed records the immutable terminal record and replies — the
// one place an admitted command finishes.
func (p *drivePort) completeClaimed(entry *drive.Entry, cmd drive.Command, res drive.Result) {
	if res.Cut == "" {
		res.Cut = p.Cut()
	}
	if res.Frame == "" {
		res.Frame = p.Frame()
	}
	sess, ok := p.sessions.Get(cmd.Session)
	if ok {
		var perr *proto.Error
		if res.Err != nil {
			errors.As(res.Err, &perr)
		}
		ledger := sess.Ledger()
		if perr == nil {
			ledger.CompleteResult(entry, res.Payload, res.Cut, res.Frame)
		} else {
			ledger.CompleteError(entry, perr)
		}
	}
	select {
	case cmd.Reply <- res:
	default:
	}
}

// replyClaimed records a terminal rejection (already claimed) and replies.
func (p *drivePort) replyClaimed(cmd drive.Command, entry *drive.Entry, payload json.RawMessage, perr *proto.Error) {
	if sess, ok := p.sessions.Get(cmd.Session); ok {
		sess.Ledger().CompleteError(entry, perr)
	}
	p.reply(cmd, payload, perr, false)
}

// replayRetained answers a duplicate ID from the retained outcome without
// scheduling. The reply is a fresh transport write of the immutable record.
func (p *drivePort) replayRetained(cmd drive.Command, entry *drive.Entry) {
	res := drive.Result{Cut: entry.ResultCut, Frame: entry.ResultFrame}
	if entry.Admitted && entry.State == drive.EntryTerminal {
		if entry.ResultErr != nil {
			res.Err = entry.ResultErr
		} else {
			res.Payload = entry.ResultPayload
		}
	} else {
		// Still in flight: the client races its own request. Report it as
		// busy-retryable admission state, never a second dispatch.
		res.Err = &proto.Error{
			Code:      proto.ErrCapacityBusy,
			Detail:    "this id is admitted and its outcome is pending",
			Retryable: true,
		}
	}
	select {
	case cmd.Reply <- res:
	default:
	}
}

// pendingAdvance pairs an admitted advance with its claimed ledger entry;
// drivePostFrame executes each as exactly one full frame, then completes
// the record and replies. Runtime-thread only.
type pendingAdvance struct {
	d     time.Duration
	cmd   drive.Command
	entry *drive.Entry
}
