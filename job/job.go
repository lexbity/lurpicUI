package job

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"

	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// CancelToken is passed to workers so they can detect cancellation.
type CancelToken struct {
	cancelled atomic.Bool
}

// Cancelled reports whether the token has been cancelled.
func (t *CancelToken) Cancelled() bool {
	return t != nil && t.cancelled.Load()
}

func (t *CancelToken) cancel() {
	if t == nil {
		return
	}
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
	commitFn  func()
}

func (r anyResult) JobID() JobID    { return r.jobID }
func (r anyResult) Cancelled() bool { return r.cancelled }
func (r anyResult) Err() error      { return r.err }
func (r anyResult) Commit() {
	if r.commitFn != nil {
		r.commitFn()
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
	closed bool

	wg sync.WaitGroup

	onDrain signal.Signal[signal.Unit]
}

// Start is retained for runtime compatibility. Worker goroutines already start in NewPool.
func (p *Pool) Start() {}

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
		ctx:         ctx,
		cancel:      cancel,
		interactive: make(chan queuedJob),
		background:  make(chan queuedJob, workerCount*4),
		results:     make(chan anyResult, workerCount*4),
		shutdown:    make(chan struct{}),
		active:      make(map[JobID]*CancelToken),
	}
	for i := 0; i < workerCount; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

// Schedule submits a job to the pool.
func Schedule[I, O any](p *Pool, job Job[I, O], onCommit func(O)) error {
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
			res.commitFn = func() {
				if snap.currentVersions != nil {
					if !snap.StillValid(snap.currentVersions()...) {
						return
					}
				}
				if onCommit != nil {
					onCommit(output)
				}
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

// Drain returns all available completed results without blocking.
func (p *Pool) Drain() []anyResult {
	if p == nil {
		return nil
	}
	var out []anyResult
	for {
		select {
		case res := <-p.results:
			if res.commitFn != nil && !res.cancelled && res.err == nil {
				res.commitFn()
			}
			out = append(out, res)
		default:
			return out
		}
	}
}

// CancelJob cancels a specific job by ID.
func (p *Pool) CancelJob(id JobID) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if token := p.active[id]; token != nil {
		token.cancel()
	}
}

// CancelAll cancels all active and queued jobs.
func (p *Pool) CancelAll() {
	if p == nil {
		return
	}
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
	if p == nil {
		return
	}
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

	p.cancel()
	close(p.shutdown)
	p.wg.Wait()
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for {
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
	case <-p.shutdown:
	case <-p.ctx.Done():
	}
}
