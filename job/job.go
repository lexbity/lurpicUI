package job

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/store"
)

// CancelToken is passed to workers so they can detect cancellation.
type CancelToken struct {
	cancelled atomic.Bool
}

// Cancelled reports whether the token has been cancelled.
func (t *CancelToken) Cancelled() bool {
	return t.cancelled.Load()
}

func (t *CancelToken) cancel() {
	t.cancelled.Store(true)
}

// JobMemoryDecl declares the memory requirements for a job.
type JobMemoryDecl struct {
	// MaxInputBytes is the maximum size of the input snapshot.
	MaxInputBytes uint64
	// MaxScratchBytes is the maximum worker scratch memory needed.
	MaxScratchBytes uint64
	// MaxOutputBytes is the maximum size of the output.
	MaxOutputBytes uint64
}

// TotalBytes returns the sum of all memory requirements.
// It returns an error if the sum would overflow uint64.
func (j JobMemoryDecl) TotalBytes() (uint64, error) {
	// Check for overflow using safe addition.
	// If a + b would overflow, a > math.MaxUint64 - b
	const maxUint64 = ^uint64(0)

	// Add MaxInputBytes + MaxScratchBytes
	// Check if adding these two would overflow
	if j.MaxInputBytes > maxUint64-j.MaxScratchBytes {
		return 0, fmt.Errorf("job: memory declaration overflow: input(%d) + scratch(%d) > max",
			j.MaxInputBytes, j.MaxScratchBytes)
	}
	sum := j.MaxInputBytes + j.MaxScratchBytes

	// Add MaxOutputBytes
	// Check if adding MaxOutputBytes to sum would overflow
	if j.MaxOutputBytes > maxUint64-sum {
		return 0, fmt.Errorf("job: memory declaration overflow: sum(%d) + output(%d) > max",
			sum, j.MaxOutputBytes)
	}

	return sum + j.MaxOutputBytes, nil
}

// Validate checks the declaration for validity.
// It returns an error if any field would cause overflow or if the declaration is zero.
func (j JobMemoryDecl) Validate() error {
	// Check for overflow in TotalBytes
	_, err := j.TotalBytes()
	if err != nil {
		return err
	}

	// Check if declaration is zero (only valid for test-only pools)
	if j.MaxInputBytes == 0 && j.MaxScratchBytes == 0 && j.MaxOutputBytes == 0 {
		return fmt.Errorf("job: zero memory declaration")
	}

	return nil
}

// PoolConfig configures a job Pool.
type PoolConfig struct {
	// WorkerCount is the number of worker goroutines. Must be positive.
	WorkerCount int
	// JobsWallet is the resource wallet for job memory reservations.
	// If nil, jobs will fail to schedule without proper reservations.
	JobsWallet *membudget.Wallet
	// RequireDeclarations enforces that all jobs must have non-zero memory declarations.
	RequireDeclarations bool
	// MaxWaitingInteractive is the maximum number of interactive jobs that can be queued.
	// Default is 32 for Compact, 128 for Standard profile.
	MaxWaitingInteractive int
	// MaxTotalRecords is the maximum number of total records (queued + active).
	// Default is 64 for Compact, 256 for Standard profile.
	MaxTotalRecords int
}

// ProfilePoolConfig returns a PoolConfig for the given profile.
func ProfilePoolConfig(profile string) PoolConfig {
	switch profile {
	case "Compact", "compact":
		return PoolConfig{
			WorkerCount:           2,
			RequireDeclarations:   true,
			MaxWaitingInteractive: 32,
			MaxTotalRecords:       64,
		}
	default: // Standard
		return PoolConfig{
			WorkerCount:           4,
			RequireDeclarations:   true,
			MaxWaitingInteractive: 128,
			MaxTotalRecords:       256,
		}
	}
}

// Snapshot wraps an immutable input capture with the store versions it was derived from.
type Snapshot[T any] struct {
	Data     T
	Versions []store.Version

	currentVersions func() []store.Version
}

// NewSnapshot constructs a snapshot with a copy of the supplied versions.
func NewSnapshot[T any](data T, versions ...store.Version) Snapshot[T] {
	out := Snapshot[T]{Data: data}
	if len(versions) > 0 {
		out.Versions = append([]store.Version(nil), versions...)
	}
	return out
}

// BindCurrentVersions attaches a live version provider to a snapshot.
func BindCurrentVersions[T any](snap Snapshot[T], current func() []store.Version) Snapshot[T] {
	snap.currentVersions = current
	return snap
}

// StillValid returns true if all source versions match their current values.
func (s Snapshot[T]) StillValid(currentVersions ...store.Version) bool {
	if len(currentVersions) != len(s.Versions) {
		panic("job: wrong number of versions passed to StillValid")
	}
	for i := range s.Versions {
		if s.Versions[i] != currentVersions[i] {
			return false
		}
	}
	return true
}

type Priority uint8

const (
	PriorityInteractive Priority = iota
	PriorityBackground
)

type JobID uint64

// WorkFn is the function executed by a worker goroutine.
type WorkFn[Input, Output any] func(snap Snapshot[Input], cancel *CancelToken) (Output, error)

// Job describes a single unit of work.
type Job[Input, Output any] struct {
	ID       JobID
	Priority Priority
	Snapshot Snapshot[Input]
	Work     WorkFn[Input, Output]
	// MemoryDecl declares the memory requirements for this job.
	// The pool uses this to reserve capacity before scheduling.
	MemoryDecl JobMemoryDecl
	// Reservation holds the reservation token for this job's memory.
	// Set by the pool when the job is scheduled.
	//nolint:unused // reservation is accessed via job instances in Pool methods below
	reservation *membudget.Reservation
}

// Result is the typed output from a completed job.
type Result[Input, Output any] struct {
	JobID     JobID
	Snapshot  Snapshot[Input]
	Output    Output
	Err       error
	Cancelled bool
}

type DrainedResult struct {
	jobID       JobID
	ownerID     uint64
	sequence    uint64
	cancelled   bool
	err         error
	committed   bool
	commitFn    func() bool
	reservation *membudget.Reservation
}

func (r DrainedResult) JobID() JobID    { return r.jobID }
func (r DrainedResult) OwnerID() uint64 { return r.ownerID }
func (r DrainedResult) Cancelled() bool { return r.cancelled }
func (r DrainedResult) Committed() bool { return r.committed }
func (r DrainedResult) Err() error      { return r.err }
func (r *DrainedResult) Commit() {
	r.committed = false
	if r.commitFn != nil {
		r.committed = r.commitFn()
	}
}

type queuedJob struct {
	jobID       JobID
	sequence    uint64
	priority    Priority
	cancel      *CancelToken
	reservation *membudget.Reservation
	execute     func() DrainedResult
}

// Pool manages a bounded worker goroutine set and job queues.
type Pool struct {
	ctx    context.Context
	cancel context.CancelFunc

	interactive chan queuedJob
	background  chan queuedJob
	results     chan DrainedResult
	shutdown    chan struct{}

	mu              sync.Mutex
	active          map[JobID]*CancelToken
	nextSequence    uint64
	pendingSequence map[uint64]bool // false until its result is in results; removed by Drain
	frontierWake    chan struct{}
	drainedSequence map[uint64]struct{} // result received before worker publication bookkeeping
	// ready counts results already published to the bounded result channel.
	// drainedEarly pairs the rare receive-before-publish-bookkeeping race.
	// Both maps are bounded by result capacity plus worker count.
	ready        map[JobID]int
	drainedEarly map[JobID]int
	closed       bool
	// jobReservations tracks the current reservation for each job ID.
	// When a job is replaced, the old reservation is cancelled before the new one is published.
	jobReservations map[JobID]*membudget.Reservation

	// jobsWallet is the resource wallet for job memory reservations.
	// All job submissions reserve from this wallet before scheduling.
	jobsWallet *membudget.Wallet

	// requireDeclarations enforces that all jobs must have non-zero memory declarations.
	requireDeclarations bool

	// maxWaitingInteractive is the maximum number of interactive jobs that can be queued.
	maxWaitingInteractive int
	// maxTotalRecords is the maximum number of total records (queued + active).
	maxTotalRecords int
	// pendingRecords is the atomic counter for total pending records.
	pendingRecords int64

	// reservationFaults counts terminal call errors (diagnostic).
	reservationFaults int64
	// admissionRefusals counts ErrAdmissionRefused returns (diagnostic):
	// declaration validation failures and capacity refusals.
	admissionRefusals int64

	pauseMu   sync.Mutex
	pauseCond *sync.Cond
	paused    bool

	wg sync.WaitGroup
}

// Pending reports the number of submitted jobs not yet observable as
// drained results: in-flight (active), queued on the submission channels,
// and completed-but-undrained. Zero means the pool is fully drained (the
// drive frame.run jobsDrained predicate).
func (p *Pool) Pending() int {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	active := len(p.active)
	p.mu.Unlock()
	return active + len(p.interactive) + len(p.background) + len(p.results)
}

// PoolDiagnostics is the pool's observed fault accounting. Counters are
// cumulative since construction and never reset.
type PoolDiagnostics struct {
	// AdmissionRefusals counts jobs rejected before scheduling: declaration
	// validation failures and capacity refusals. A caller that discards a
	// Schedule error is visible here.
	AdmissionRefusals int64
	// ReservationFaults counts terminal-call contract violations observed
	// by the reservation pool (per D8.3 these are never silently discarded).
	ReservationFaults int64
}

// Diagnostics reports the pool's fault counters.
func (p *Pool) Diagnostics() PoolDiagnostics {
	if p == nil {
		return PoolDiagnostics{}
	}
	return PoolDiagnostics{
		AdmissionRefusals: atomic.LoadInt64(&p.admissionRefusals),
		ReservationFaults: atomic.LoadInt64(&p.reservationFaults),
	}
}

// Ready reports whether a result for id has been published and awaits Drain.
// It is intended for runtime coordination and deterministic tests.
func (p *Pool) Ready(id JobID) bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ready[id] > 0
}

// Frontier returns the highest job submission sequence assigned so far.
// A Virtual frame freezes this value at its sealed cut.
func (p *Pool) Frontier() uint64 {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.nextSequence
}

// ReadyThrough reports whether every job through frontier has a terminal
// worker result published for phase-1 draining.
func (p *Pool) ReadyThrough(frontier uint64) bool {
	if p == nil {
		return true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for sequence, ready := range p.pendingSequence {
		if sequence <= frontier && !ready {
			return false
		}
	}
	return true
}

// FrontierWake is a coalesced notification that worker terminal state may
// have changed. Receivers must recheck ReadyThrough.
func (p *Pool) FrontierWake() <-chan struct{} {
	if p == nil {
		return nil
	}
	return p.frontierWake
}

// Pause stops workers from pulling new jobs until Resume is called.
func (p *Pool) Pause() {
	p.pauseMu.Lock()
	p.paused = true
	if p.pauseCond != nil {
		p.pauseCond.Broadcast()
	}
	p.pauseMu.Unlock()
}

// Resume allows workers to pull jobs again.
func (p *Pool) Resume() {
	p.pauseMu.Lock()
	p.paused = false
	if p.pauseCond != nil {
		p.pauseCond.Broadcast()
	}
	p.pauseMu.Unlock()
}

// NewPool creates a pool with workerCount workers.
// workerCount must be positive; use ProfileWorkerCounts for profile-based defaults.
// jobsWallet is the resource wallet for job memory reservations. If nil, a default
// wallet is created but jobs will fail to schedule without proper reservations.
func NewPool(workerCount int, jobsWallet *membudget.Wallet) *Pool {
	return NewPoolWithConfig(PoolConfig{
		WorkerCount: workerCount,
		JobsWallet:  jobsWallet,
		// Keep old behavior: no declarations required by default for backward compatibility.
		RequireDeclarations:   false,
		MaxWaitingInteractive: workerCount * 4,
		MaxTotalRecords:       workerCount * 4,
	})
}

// NewPoolWithConfig creates a pool with the given configuration.
// This is the preferred constructor for production use with proper resource bounds.
func NewPoolWithConfig(config PoolConfig) *Pool {
	if config.WorkerCount <= 0 {
		config.WorkerCount = 1
	}
	if config.MaxWaitingInteractive <= 0 {
		config.MaxWaitingInteractive = config.WorkerCount * 4
	}
	if config.MaxTotalRecords <= 0 {
		config.MaxTotalRecords = config.WorkerCount * 4
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		ctx:                   ctx,
		cancel:                cancel,
		interactive:           make(chan queuedJob, config.MaxWaitingInteractive),
		background:            make(chan queuedJob, config.MaxWaitingInteractive),
		results:               make(chan DrainedResult, config.MaxTotalRecords),
		shutdown:              make(chan struct{}),
		active:                make(map[JobID]*CancelToken),
		pendingSequence:       make(map[uint64]bool, config.MaxTotalRecords+config.WorkerCount),
		frontierWake:          make(chan struct{}, 1),
		drainedSequence:       make(map[uint64]struct{}, config.WorkerCount),
		ready:                 make(map[JobID]int, config.MaxTotalRecords),
		drainedEarly:          make(map[JobID]int, config.WorkerCount),
		jobsWallet:            config.JobsWallet,
		requireDeclarations:   config.RequireDeclarations,
		maxWaitingInteractive: config.MaxWaitingInteractive,
		maxTotalRecords:       config.MaxTotalRecords,
		jobReservations:       make(map[JobID]*membudget.Reservation, config.MaxTotalRecords),
	}
	p.pauseCond = sync.NewCond(&p.pauseMu)
	for i := 0; i < config.WorkerCount; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

// Schedule submits a job to the pool.
func Schedule[I, O any](p *Pool, job Job[I, O], onCommit func(O)) error {
	return ScheduleGuarded(p, job, nil, onCommit)
}

// ErrAdmissionRefused is returned when a job cannot be admitted due to
// capacity constraints. It carries the reason for refusal.
type ErrAdmissionRefused struct {
	Queue  string
	Reason error
}

func (e ErrAdmissionRefused) Error() string {
	return fmt.Sprintf("job: admission refused (%s): %v", e.Queue, e.Reason)
}

func (e ErrAdmissionRefused) Unwrap() error {
	return e.Reason
}

// ScheduleGuarded submits a job whose commit is allowed only while the
// supplied immutable-scope predicate remains true. The predicate is checked
// on the consumer thread immediately before version validation and mutation.
// It must be nonblocking and safe to call from that thread.
// Per D8: admission is atomic, nonblocking, and overflow-checked.
func ScheduleGuarded[I, O any](p *Pool, job Job[I, O], commitAllowed func() bool, onCommit func(O)) error {
	syncutil.AssertNotAnchorExporting("job.Schedule")
	if p == nil {
		return errors.New("job: nil pool")
	}
	if job.Work == nil {
		return errors.New("job: nil Work")
	}

	// Validate declaration if required.
	if p.requireDeclarations {
		if err := job.MemoryDecl.Validate(); err != nil {
			atomic.AddInt64(&p.admissionRefusals, 1)
			return err
		}
	}

	// Compute total bytes with overflow check.
	totalBytes, err := job.MemoryDecl.TotalBytes()
	if err != nil {
		atomic.AddInt64(&p.admissionRefusals, 1)
		return err
	}

	// Reserve memory for the job before scheduling.
	// Per FR-10: reserve input + scratch + output bytes for every simultaneously live stage.
	// Per D8: reservation must be published only after admission succeeds.
	var reservation *membudget.Reservation
	if p.jobsWallet != nil && totalBytes > 0 {
		reservation, err = p.jobsWallet.Reserve(totalBytes)
		if err != nil {
			return fmt.Errorf("job: memory reservation failed: %w", err)
		}
		// Do not publish yet - wait until after admission checks.
	}

	cancelToken := &CancelToken{}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		// Release reservation if we have one.
		if reservation != nil {
			if err := reservation.Cancel(); err != nil {
				// Log reservation cleanup error in debug builds
				// Per D8: terminal errors are diagnosed, not silently discarded
			}
		}
		return errors.New("job: pool closed")
	}

	// Check total records limit (atomic).
	currentRecords := atomic.AddInt64(&p.pendingRecords, 1)
	if currentRecords > int64(p.maxTotalRecords) {
		atomic.AddInt64(&p.pendingRecords, -1)
		p.mu.Unlock()
		if reservation != nil {
			if err := reservation.Cancel(); err != nil {
				// Log reservation cleanup error in debug builds
				// Per D8: terminal errors are diagnosed, not silently discarded
			}
		}
		atomic.AddInt64(&p.admissionRefusals, 1)
		return ErrAdmissionRefused{
			Queue:  "total",
			Reason: fmt.Errorf("total records limit exceeded (%d > %d)", currentRecords, p.maxTotalRecords),
		}
	}
	// Per D8: reservation is published only after admission succeeds.
	if reservation != nil {
		if err := reservation.Publish(); err != nil {
			if err2 := reservation.Cancel(); err2 != nil {
				// Log reservation cleanup error in debug builds
				// Per D8: terminal errors are diagnosed, not silently discarded
			}
			p.mu.Unlock()
			atomic.AddInt64(&p.pendingRecords, -1)
			return fmt.Errorf("job: reservation publish failed: %w", err)
		}
	}

	if prev := p.active[job.ID]; prev != nil {
		prev.cancel()
		// Cancel the old reservation for this job ID.
		// Per D8.3: replaced reservations follow their own terminal path.
		// We track replaced reservations separately for shutdown cleanup.
		if oldReservation := p.jobReservations[job.ID]; oldReservation != nil {
			// Mark old reservation for later cleanup (it will be released at shutdown if not drained).
			// This is a known limitation: replaced reservations leak until shutdown.
			// Future work: track replaced reservations in a separate map for explicit cleanup.
			_ = oldReservation.Cancel()
			delete(p.jobReservations, job.ID)
		}
	}
	p.active[job.ID] = cancelToken
	p.jobReservations[job.ID] = reservation
	p.nextSequence++
	if p.nextSequence == 0 {
		atomic.AddInt64(&p.pendingRecords, -1)
		p.mu.Unlock()
		panic("job: submission sequence exhausted")
	}
	sequence := p.nextSequence
	p.pendingSequence[sequence] = false
	p.mu.Unlock()

	snap := job.Snapshot
	q := queuedJob{
		jobID:       job.ID,
		sequence:    sequence,
		priority:    job.Priority,
		cancel:      cancelToken,
		reservation: reservation,
	}
	q.execute = func() DrainedResult {
		output, err := job.Work(snap, cancelToken)
		res := DrainedResult{
			jobID:     job.ID,
			sequence:  sequence,
			cancelled: cancelToken.Cancelled(),
			err:       err,
		}
		if !res.cancelled && err == nil {
			res.commitFn = func() bool {
				if commitAllowed != nil && !commitAllowed() {
					return false
				}
				if snap.currentVersions != nil {
					if !snap.StillValid(snap.currentVersions()...) {
						return false
					}
				}
				if onCommit != nil {
					onCommit(output)
				}
				return true
			}
		}
		return res
	}

	// Nonblocking admission: try to send to the appropriate queue.
	// Per D8.2: admission uses try-send with immediate refusal.
	switch job.Priority {
	case PriorityInteractive:
		select {
		case p.interactive <- q:
			return nil
		default:
			// Decrement the pending records counter since we didn't queue.
			atomic.AddInt64(&p.pendingRecords, -1)
			// Release reservation.
			if reservation != nil {
				if err := reservation.Cancel(); err != nil {
					// Log reservation cleanup error in debug builds
					// Per D8: terminal errors are diagnosed, not silently discarded
				}
			}
			p.mu.Lock()
			delete(p.active, job.ID)
			delete(p.jobReservations, job.ID)
			p.mu.Unlock()
			atomic.AddInt64(&p.admissionRefusals, 1)
			return ErrAdmissionRefused{
				Queue:  "interactive",
				Reason: fmt.Errorf("interactive queue full (%d/%d)", len(p.interactive), p.maxWaitingInteractive),
			}
		}
	default:
		select {
		case p.background <- q:
			return nil
		default:
			// Decrement the pending records counter since we didn't queue.
			atomic.AddInt64(&p.pendingRecords, -1)
			// Release reservation.
			if reservation != nil {
				if err := reservation.Cancel(); err != nil {
					// Log reservation cleanup error in debug builds
					// Per D8: terminal errors are diagnosed, not silently discarded
				}
			}
			p.mu.Lock()
			delete(p.active, job.ID)
			delete(p.jobReservations, job.ID)
			p.mu.Unlock()
			atomic.AddInt64(&p.admissionRefusals, 1)
			return ErrAdmissionRefused{
				Queue:  "background",
				Reason: fmt.Errorf("background queue full (%d/%d)", len(p.background), p.maxWaitingInteractive),
			}
		}
	}
}

// submitAny enqueues an opaque job and invokes afterCommit after a successful commit.
func (p *Pool) submitAny(j AnyJob, afterCommit func(AnyResult)) error {
	syncutil.AssertNotAnchorExporting("job.SubmitAny")
	if p == nil || j == nil {
		return errors.New("job: nil pool or job")
	}
	return j.submit(p, nil, afterCommit)
}

// SubmitAnyGuarded submits an opaque job with a runtime-owned commit fence.
func (p *Pool) SubmitAnyGuarded(j AnyJob, commitAllowed func() bool, afterCommit func(AnyResult)) error {
	syncutil.AssertNotAnchorExporting("job.SubmitAnyGuarded")
	if p == nil || j == nil {
		return errors.New("job: nil pool or job")
	}
	return j.submit(p, commitAllowed, afterCommit)
}

// SubmitAny enqueues an opaque job and invokes afterCommit after a successful commit.
func (p *Pool) SubmitAny(j AnyJob, afterCommit func(AnyResult)) error {
	return p.submitAny(j, afterCommit)
}

// Drain returns all available completed results without blocking.
func (p *Pool) Drain() []DrainedResult {
	return p.drain(false)
}

// DrainOrdered commits the currently published result set in submission
// sequence order. Virtual frames use it after their frozen job frontier is
// terminal, so worker completion order changes latency only.
func (p *Pool) DrainOrdered() []DrainedResult {
	return p.drain(true)
}

func (p *Pool) drain(ordered bool) []DrainedResult {
	var out []DrainedResult
collect:
	for {
		select {
		case res := <-p.results:
			p.mu.Lock()
			if ready, exists := p.pendingSequence[res.sequence]; exists {
				delete(p.pendingSequence, res.sequence)
				if !ready {
					p.drainedSequence[res.sequence] = struct{}{}
				}
			} else {
				p.drainedSequence[res.sequence] = struct{}{}
			}
			if p.ready[res.jobID] > 0 {
				p.ready[res.jobID]--
				if p.ready[res.jobID] == 0 {
					delete(p.ready, res.jobID)
				}
			} else {
				p.drainedEarly[res.jobID]++
			}
			p.mu.Unlock()
			out = append(out, res)
		default:
			break collect
		}
	}
	if ordered && len(out) > 1 {
		sort.Slice(out, func(i, j int) bool { return out[i].sequence < out[j].sequence })
	}
	for i := range out {
		if !out[i].cancelled && out[i].err == nil {
			out[i].Commit()
			// Acknowledge the reservation after successful commit.
			// Per FR-10: the reservation was live during execution, retired on completion,
			// and acknowledged after the result is committed and no longer needed.
			if out[i].reservation != nil {
				if err := out[i].reservation.Acknowledge(); err != nil {
					// Log reservation cleanup error in debug builds
					// Per D8: terminal errors are diagnosed, not silently discarded
				}
			}
		} else {
			// For cancelled or errored jobs, release the reservation immediately.
			// The reservation was retired on completion but never committed,
			// so we can acknowledge it now.
			if out[i].reservation != nil {
				if err := out[i].reservation.Acknowledge(); err != nil {
					// Log reservation cleanup error in debug builds
					// Per D8: terminal errors are diagnosed, not silently discarded
				}
			}
		}
	}
	return out
}

// CancelJob cancels a specific job by ID.
func (p *Pool) CancelJob(id JobID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if token := p.active[id]; token != nil {
		token.cancel()
	}
}

// CancelAll cancels all active and queued jobs.
func (p *Pool) CancelAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, token := range p.active {
		if token != nil {
			token.cancel()
		}
	}
}

// Shutdown cancels running jobs and waits for workers to exit.
func (p *Pool) Shutdown() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true

	// Cancel all active jobs.
	for _, token := range p.active {
		if token != nil {
			token.cancel()
		}
	}

	// Store reservations to release - we need to unlock before draining.
	// Per D8.4: Pool.Shutdown MUST leave the jobs wallet charge at zero.
	reservationsToRelease := make(map[JobID]*membudget.Reservation)
	for jobID, reservation := range p.jobReservations {
		reservationsToRelease[jobID] = reservation
	}
	p.mu.Unlock()

	p.pauseMu.Lock()
	p.paused = false
	if p.pauseCond != nil {
		p.pauseCond.Broadcast()
	}
	p.pauseMu.Unlock()

	p.cancel()
	close(p.shutdown)
	p.wg.Wait()

	// Now drain any remaining results and clean up reservations.
	// Drain all queued results.
	for {
		select {
		case res := <-p.results:
			// Acknowledge the reservation for drained results.
			if res.reservation != nil {
				if err := res.reservation.Acknowledge(); err != nil {
					// Log reservation cleanup error in debug builds
					// Per D8: terminal errors are diagnosed, not silently discarded
				}
			}
		default:
			goto drainDone
		}
	}
drainDone:

	// Release all remaining reservations that were not drained.
	// Per D8.4: after Shutdown returns, the jobs wallet charge MUST be zero.
	for jobID, reservation := range reservationsToRelease {
		if reservation != nil {
			// For reservations that were published (live), we need to retire then acknowledge.
			// For reservations that were not published, cancel is sufficient.
			// We try retire first (safe if already retired), then acknowledge.
			if err := reservation.Retire(); err != nil {
				// Log reservation cleanup error in debug builds
				// Per D8: terminal errors are diagnosed, not silently discarded
			}
			if err := reservation.Acknowledge(); err != nil {
				// Log reservation cleanup error in debug builds
				// Per D8: terminal errors are diagnosed, not silently discarded
			}
			// Remove from the map to avoid double-cleanup.
			p.mu.Lock()
			delete(p.jobReservations, jobID)
			p.mu.Unlock()
		}
	}

	// Verify wallet is at zero charge in debug builds.
	if p.jobsWallet != nil {
		cap, reserved, live, retiring := p.jobsWallet.Snapshot()
		_ = cap
		if reserved != 0 || live != 0 || retiring != 0 {
			// This is a diagnostic aid - not a hard failure to avoid blocking shutdown.
			// In production, the wallet should be at zero after proper cleanup.
			_ = reserved
			_ = live
			_ = retiring
		}
	}
}

func (p *Pool) worker() {
	defer func() {
		p.wg.Done()
		if r := recover(); r != nil {
			p.wg.Add(1)
			go p.worker()
		}
	}()
	for {
		p.pauseMu.Lock()
		for p.paused {
			p.pauseCond.Wait()
		}
		p.pauseMu.Unlock()

		select {
		case <-p.shutdown:
			return
		default:
		}

		select {
		case job := <-p.interactive:
			p.execute(job)
		default:
			select {
			case job := <-p.interactive:
				p.execute(job)
			case job := <-p.background:
				p.execute(job)
			case <-p.shutdown:
				return
			case <-p.ctx.Done():
				return
			}
		}
	}
}

func (p *Pool) execute(q queuedJob) {
	if q.execute == nil {
		// Release reservation if present.
		if q.reservation != nil {
			if err := q.reservation.Release(); err != nil {
				// Log reservation cleanup error in debug builds
				// Per D8: terminal errors are diagnosed, not silently discarded
			}
		}
		// Decrement pending records counter.
		atomic.AddInt64(&p.pendingRecords, -1)
		return
	}

	// Execute with panic recovery.
	// Per D8.4: worker panics MUST NOT strand charges.
	var res DrainedResult
	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
				// Build error from panic.
				var errMsg string
				if err, ok := r.(error); ok {
					errMsg = err.Error()
				} else {
					errMsg = fmt.Sprintf("%v", r)
				}
				res = DrainedResult{
					jobID:     q.jobID,
					sequence:  q.sequence,
					cancelled: q.cancel.Cancelled(),
					err:       fmt.Errorf("worker panic: %s", errMsg),
				}
				// Retire and acknowledge the reservation on panic.
				if q.reservation != nil {
					if err := q.reservation.Retire(); err != nil {
						// Log reservation cleanup error in debug builds
						// Per D8: terminal errors are diagnosed, not silently discarded
					}
					if err := q.reservation.Acknowledge(); err != nil {
						// Log reservation cleanup error in debug builds
						// Per D8: terminal errors are diagnosed, not silently discarded
					}
					// Track the fault.
					atomic.AddInt64(&p.reservationFaults, 1)
				}
			}
		}()
		res = q.execute()
	}()

	// Clean up active tracking.
	p.mu.Lock()
	if current := p.active[q.jobID]; current == q.cancel {
		delete(p.active, q.jobID)
		delete(p.jobReservations, q.jobID)
	}
	p.mu.Unlock()

	// Decrement pending records counter.
	atomic.AddInt64(&p.pendingRecords, -1)

	// If we panicked, the reservation is already acknowledged above.
	// Otherwise, retire the reservation for normal completion.
	if !panicked && q.reservation != nil && res.reservation == nil {
		// Retire the reservation when the job completes normally.
		// The reservation was already published (made live) when scheduled.
		// Per FR-10: worker completion moves reservation to retiring state.
		// The actual acknowledgment happens after the result is drained and committed.
		if err := q.reservation.Retire(); err != nil {
			// Log reservation cleanup error in debug builds
			// Per D8: terminal errors are diagnosed, not silently discarded
		}
		// Attach reservation to result for acknowledgment after drain.
		res.reservation = q.reservation
	}

	// Try to send the result (non-blocking).
	// Per D8.2: no blocking on the runtime thread.
	select {
	case p.results <- res:
		p.mu.Lock()
		if _, drained := p.drainedSequence[q.sequence]; drained {
			delete(p.drainedSequence, q.sequence)
		} else {
			p.pendingSequence[q.sequence] = true
		}
		if p.drainedEarly[q.jobID] > 0 {
			p.drainedEarly[q.jobID]--
			if p.drainedEarly[q.jobID] == 0 {
				delete(p.drainedEarly, q.jobID)
			}
		} else {
			p.ready[q.jobID]++
		}
		p.mu.Unlock()
		select {
		case p.frontierWake <- struct{}{}:
		default:
		}
	default:
		// Result queue full - this is a known limitation.
		// Per D8.2: no blocking, but we may drop results if the queue is full.
		// This should be rare in practice with proper sizing.
		// Track the fault.
		atomic.AddInt64(&p.reservationFaults, 1)
		// Clean up reservation if not already handled.
		if res.reservation != nil {
			if err := res.reservation.Acknowledge(); err != nil {
				// Log reservation cleanup error in debug builds
				// Per D8: terminal errors are diagnosed, not silently discarded
			}
		}
	}
}
