package job

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"

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
}

// Result is the typed output from a completed job.
type Result[Input, Output any] struct {
	JobID     JobID
	Snapshot  Snapshot[Input]
	Output    Output
	Err       error
	Cancelled bool
}

type anyResult struct {
	jobID     JobID
	cancelled bool
	err       error
	committed bool
	commitFn  func() bool
}

func (r anyResult) JobID() JobID    { return r.jobID }
func (r anyResult) Cancelled() bool { return r.cancelled }
func (r anyResult) Committed() bool { return r.committed }
func (r anyResult) Err() error      { return r.err }
func (r *anyResult) Commit() {
	r.committed = false
	if r.commitFn != nil {
		r.committed = r.commitFn()
	}
}

type queuedJob struct {
	jobID    JobID
	priority Priority
	cancel   *CancelToken
	execute  func() anyResult
}

// Pool manages a bounded worker goroutine set and job queues.
type Pool struct {
	ctx    context.Context
	cancel context.CancelFunc

	interactive chan queuedJob
	background  chan queuedJob
	results     chan anyResult
	shutdown    chan struct{}

	mu     sync.Mutex
	active map[JobID]*CancelToken
	// ready counts results already published to the bounded result channel.
	// drainedEarly pairs the rare receive-before-publish-bookkeeping race.
	// Both maps are bounded by result capacity plus worker count.
	ready        map[JobID]int
	drainedEarly map[JobID]int
	closed       bool

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
func NewPool(workerCount int) *Pool {
	if workerCount <= 0 {
		workerCount = runtime.NumCPU() - 1
		if workerCount < 1 {
			workerCount = 1
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		ctx:          ctx,
		cancel:       cancel,
		interactive:  make(chan queuedJob),
		background:   make(chan queuedJob, workerCount*4),
		results:      make(chan anyResult, workerCount*4),
		shutdown:     make(chan struct{}),
		active:       make(map[JobID]*CancelToken),
		ready:        make(map[JobID]int, workerCount*4),
		drainedEarly: make(map[JobID]int, workerCount),
	}
	p.pauseCond = sync.NewCond(&p.pauseMu)
	for i := 0; i < workerCount; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

// Schedule submits a job to the pool.
func Schedule[I, O any](p *Pool, job Job[I, O], onCommit func(O)) error {
	return ScheduleGuarded(p, job, nil, onCommit)
}

// ScheduleGuarded submits a job whose commit is allowed only while the
// supplied immutable-scope predicate remains true. The predicate is checked
// on the consumer thread immediately before version validation and mutation.
// It must be nonblocking and safe to call from that thread.
func ScheduleGuarded[I, O any](p *Pool, job Job[I, O], commitAllowed func() bool, onCommit func(O)) error {
	syncutil.AssertNotAnchorExporting("job.Schedule")
	if p == nil {
		return errors.New("job: nil pool")
	}
	if job.Work == nil {
		return errors.New("job: nil Work")
	}

	cancelToken := &CancelToken{}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return errors.New("job: pool closed")
	}
	if prev := p.active[job.ID]; prev != nil {
		prev.cancel()
	}
	p.active[job.ID] = cancelToken
	p.mu.Unlock()

	snap := job.Snapshot
	q := queuedJob{
		jobID:    job.ID,
		priority: job.Priority,
		cancel:   cancelToken,
	}
	q.execute = func() anyResult {
		output, err := job.Work(snap, cancelToken)
		res := anyResult{
			jobID:     job.ID,
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

	select {
	case <-p.ctx.Done():
		return errors.New("job: pool closed")
	default:
	}

	switch job.Priority {
	case PriorityInteractive:
		select {
		case p.interactive <- q:
		case <-p.ctx.Done():
			return errors.New("job: pool closed")
		}
	default:
		select {
		case p.background <- q:
		case <-p.ctx.Done():
			return errors.New("job: pool closed")
		}
	}
	return nil
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
func (p *Pool) Drain() []anyResult {
	var out []anyResult
	for {
		select {
		case res := <-p.results:
			p.mu.Lock()
			if p.ready[res.jobID] > 0 {
				p.ready[res.jobID]--
				if p.ready[res.jobID] == 0 {
					delete(p.ready, res.jobID)
				}
			} else {
				p.drainedEarly[res.jobID]++
			}
			p.mu.Unlock()
			if !res.cancelled && res.err == nil {
				res.Commit()
			}
			out = append(out, res)
		default:
			return out
		}
	}
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
	for _, token := range p.active {
		if token != nil {
			token.cancel()
		}
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
		return
	}
	res := q.execute()
	p.mu.Lock()
	if current := p.active[q.jobID]; current == q.cancel {
		delete(p.active, q.jobID)
	}
	p.mu.Unlock()

	select {
	case p.results <- res:
		p.mu.Lock()
		if p.drainedEarly[q.jobID] > 0 {
			p.drainedEarly[q.jobID]--
			if p.drainedEarly[q.jobID] == 0 {
				delete(p.drainedEarly, q.jobID)
			}
		} else {
			p.ready[q.jobID]++
		}
		p.mu.Unlock()
	case <-p.shutdown:
	case <-p.ctx.Done():
	}
}
