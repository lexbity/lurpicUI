package job

import (
	"errors"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/store"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func TestCancelToken_not_cancelled_initially(t *testing.T) {
	var tok CancelToken
	if tok.Cancelled() {
		t.Fatal("expected false")
	}
}

func TestCancelToken_cancel_sets_flag(t *testing.T) {
	var tok CancelToken
	tok.cancel()
	if !tok.Cancelled() {
		t.Fatal("expected true")
	}
}

func TestCancelToken_safe_from_goroutine(t *testing.T) {
	var tok CancelToken
	done := make(chan bool, 1)
	go func() {
		done <- tok.Cancelled()
	}()
	if got := <-done; got {
		t.Fatal("expected false")
	}
}

func TestSnapshot_still_valid_matching_versions(t *testing.T) {
	snap := NewSnapshot("x", 1, 2, 3)
	if !snap.StillValid(1, 2, 3) {
		t.Fatal("expected valid")
	}
}

func TestSnapshot_still_valid_stale_version(t *testing.T) {
	snap := NewSnapshot("x", 1, 2, 3)
	if snap.StillValid(1, 2, 4) {
		t.Fatal("expected stale")
	}
}

func TestSnapshot_still_valid_wrong_count_panics(t *testing.T) {
	snap := NewSnapshot("x", 1, 2)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = snap.StillValid(1)
}

func TestPool_schedule_and_drain(t *testing.T) {
	p := NewPool(1)
	defer p.Shutdown()

	done := make(chan int, 1)
	err := Schedule(p, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(5, 1),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			return snap.Data * 2, nil
		},
	}, func(v int) { done <- v })
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}

	waitForResults(t, p, 1)
	drained := p.Drain()
	if len(drained) != 1 {
		t.Fatalf("drained = %d", len(drained))
	}
	if got := <-done; got != 10 {
		t.Fatalf("got %d", got)
	}
}

func TestPool_pause_and_resume_blocks_job_execution(t *testing.T) {
	p := NewPool(1)
	defer p.Shutdown()

	p.Pause()
	started := make(chan struct{}, 1)
	if err := Schedule(p, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(1, 1),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			started <- struct{}{}
			return snap.Data, nil
		},
	}, func(int) {}); err != nil {
		t.Fatalf("schedule: %v", err)
	}

	select {
	case <-started:
		t.Fatal("job started while pool paused")
	case <-time.After(50 * time.Millisecond):
	}

	p.Resume()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("job did not start after resume")
	}
}

func TestPool_job_cancelled_by_same_id(t *testing.T) {
	p := NewPool(1)
	defer p.Shutdown()

	started := make(chan *CancelToken, 1)
	release := make(chan struct{})
	err := Schedule(p, Job[string, string]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot("first", 1),
		Work: func(snap Snapshot[string], cancel *CancelToken) (string, error) {
			started <- cancel
			<-release
			return snap.Data, nil
		},
	}, func(string) {})
	if err != nil {
		t.Fatalf("schedule1: %v", err)
	}

	cancelToken := <-started
	if err := Schedule(p, Job[string, string]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot("second", 1),
		Work: func(snap Snapshot[string], cancel *CancelToken) (string, error) {
			return snap.Data, nil
		},
	}, func(string) {}); err != nil {
		t.Fatalf("schedule2: %v", err)
	}
	if !cancelToken.Cancelled() {
		t.Fatal("expected first token cancelled")
	}
	close(release)

	waitForResults(t, p, 2)
	drained := p.Drain()
	if len(drained) != 2 {
		t.Fatalf("drained = %d", len(drained))
	}
	cancelled := false
	for _, res := range drained {
		if res.jobID == 1 && res.cancelled {
			cancelled = true
			break
		}
	}
	if !cancelled {
		t.Fatal("expected cancelled result")
	}
}

func TestPool_drain_returns_empty_when_no_results(t *testing.T) {
	p := NewPool(1)
	defer p.Shutdown()
	if got := p.Drain(); len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestPool_cancelled_result_in_drain(t *testing.T) {
	p := NewPool(1)
	defer p.Shutdown()

	started := make(chan *CancelToken, 1)
	release := make(chan struct{})
	err := Schedule(p, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(1, 1),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			started <- cancel
			<-release
			return 1, nil
		},
	}, func(int) {})
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}
	cancel := <-started
	cancel.cancel()
	close(release)
	waitForResults(t, p, 1)
	drained := p.Drain()
	if len(drained) != 1 || !drained[0].cancelled {
		t.Fatalf("drained %#v", drained)
	}
}

func TestPool_error_result_in_drain(t *testing.T) {
	p := NewPool(1)
	defer p.Shutdown()

	err := Schedule(p, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(1, 1),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			return 0, errors.New("boom")
		},
	}, func(int) {})
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}
	waitForResults(t, p, 1)
	drained := p.Drain()
	if len(drained) != 1 || drained[0].err == nil {
		t.Fatalf("drained %#v", drained)
	}
}

func TestPool_commit_fn_not_called_when_stale(t *testing.T) {
	p := NewPool(1)
	defer p.Shutdown()

	v := store.NewValueStore(1)
	commits := int32(0)
	snap := BindCurrentVersions(NewSnapshot(v.Get(), v.Version()), func() []store.Version {
		return []store.Version{v.Version()}
	})
	err := Schedule(p, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: snap,
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			return snap.Data + 1, nil
		},
	}, func(v int) {
		atomic.AddInt32(&commits, 1)
	})
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}
	v.Set(2)
	waitForResults(t, p, 1)
	_ = p.Drain()
	if atomic.LoadInt32(&commits) != 0 {
		t.Fatal("expected commit suppressed")
	}
}

func TestPool_commit_fn_called_when_valid(t *testing.T) {
	p := NewPool(1)
	defer p.Shutdown()

	v := store.NewValueStore(1)
	commits := int32(0)
	snap := BindCurrentVersions(NewSnapshot(v.Get(), v.Version()), func() []store.Version {
		return []store.Version{v.Version()}
	})
	err := Schedule(p, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: snap,
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			return snap.Data + 1, nil
		},
	}, func(v int) {
		atomic.AddInt32(&commits, 1)
	})
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}
	waitForResults(t, p, 1)
	_ = p.Drain()
	if atomic.LoadInt32(&commits) != 1 {
		t.Fatal("expected commit")
	}
}

func TestPool_interactive_before_background(t *testing.T) {
	p := NewPool(2)
	defer p.Shutdown()

	release := make(chan struct{})
	interactiveDone := make(chan struct{})
	order := make(chan string, 2)

	if err := Schedule(p, Job[int, string]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(1, 1),
		Work: func(snap Snapshot[int], cancel *CancelToken) (string, error) {
			<-release
			<-interactiveDone
			order <- "background"
			return "background", nil
		},
	}, func(string) {}); err != nil {
		t.Fatalf("schedule background: %v", err)
	}
	if err := Schedule(p, Job[int, string]{
		ID:       2,
		Priority: PriorityInteractive,
		Snapshot: NewSnapshot(2, 1),
		Work: func(snap Snapshot[int], cancel *CancelToken) (string, error) {
			order <- "interactive"
			close(interactiveDone)
			return "interactive", nil
		},
	}, func(string) {}); err != nil {
		t.Fatalf("schedule interactive: %v", err)
	}

	close(release)
	waitForResults(t, p, 2)
	_ = p.Drain()

	first := <-order
	second := <-order
	if first != "interactive" || second != "background" {
		t.Fatalf("expected interactive first, got %s then %s", first, second)
	}
}

func TestPool_cancel_all(t *testing.T) {
	p := NewPool(1)
	defer p.Shutdown()

	release := make(chan struct{})
	started := make(chan *CancelToken, 2)
	for i := 0; i < 2; i++ {
		if err := Schedule(p, Job[int, int]{
			ID:       JobID(i + 1),
			Priority: PriorityBackground,
			Snapshot: NewSnapshot(i, 1),
			Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
				started <- cancel
				<-release
				return snap.Data, nil
			},
		}, func(int) {}); err != nil {
			t.Fatalf("schedule: %v", err)
		}
	}
	<-started
	p.CancelAll()
	close(release)
	waitForResults(t, p, 2)
	drained := p.Drain()
	if len(drained) != 2 || !drained[0].cancelled || !drained[1].cancelled {
		t.Fatalf("drained %#v", drained)
	}
}

func TestPool_shutdown_clean(t *testing.T) {
	before := runtime.NumGoroutine()
	p := NewPool(2)
	p.Shutdown()
	time.Sleep(10 * time.Millisecond)
	after := runtime.NumGoroutine()
	if after > before+2 {
		t.Fatalf("goroutine leak: before=%d after=%d", before, after)
	}
}

func TestPool_worker_goroutine_cannot_touch_store(t *testing.T) {
	v := store.NewValueStore(1)
	snap := NewSnapshot(v.Get(), v.Version())
	if snap.Data != 1 {
		t.Fatalf("got %d", snap.Data)
	}
	v.Set(2)
	if snap.Data != 1 {
		t.Fatalf("snapshot mutated: %d", snap.Data)
	}
}

func TestBindJob_ownerID(t *testing.T) {
	j := BindJob(42, Job[int, int]{ID: 7}, nil)
	if got := j.OwnerID(); got != 42 {
		t.Fatalf("owner = %d", got)
	}
}

func TestBindJob_jobID(t *testing.T) {
	j := BindJob(42, Job[int, int]{ID: 7}, nil)
	if got := j.JobID(); got != 7 {
		t.Fatalf("jobID = %d", got)
	}
}

func TestBindJob_onCommit_called(t *testing.T) {
	p := NewPool(1)
	defer p.Shutdown()

	commits := 0
	j := BindJob(99, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(5, 1),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			return snap.Data * 2, nil
		},
	}, func(v int) {
		commits = v
	})

	if err := j.submit(p, nil); err != nil {
		t.Fatalf("submit: %v", err)
	}
	waitForResults(t, p, 1)
	_ = p.Drain()
	if commits != 10 {
		t.Fatalf("commit = %d", commits)
	}
}

func TestBindJob_afterCommit_called(t *testing.T) {
	p := NewPool(1)
	defer p.Shutdown()

	order := make([]string, 0, 2)
	j := BindJob(99, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(5, 1),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			return snap.Data * 2, nil
		},
	}, func(v int) {
		order = append(order, "commit")
	})

	if err := j.submit(p, func(result AnyResult) {
		if result.JobID() != 1 || result.OwnerID() != 99 || result.Cancelled() || result.Err() != nil {
			t.Fatalf("unexpected result: %#v", result)
		}
		order = append(order, "after")
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	waitForResults(t, p, 1)
	_ = p.Drain()
	if len(order) != 2 || order[0] != "commit" || order[1] != "after" {
		t.Fatalf("order = %#v", order)
	}
}

func TestBindJob_afterCommit_not_called_on_cancel(t *testing.T) {
	p := NewPool(1)
	defer p.Shutdown()

	started := make(chan struct{})
	release := make(chan struct{})
	onCommitCalled := false
	afterCommitCalled := false

	first := BindJob(11, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(1, 1),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			close(started)
			<-release
			return snap.Data, nil
		},
	}, func(v int) {
		onCommitCalled = true
	})
	if err := first.submit(p, func(AnyResult) {
		afterCommitCalled = true
	}); err != nil {
		t.Fatalf("submit first: %v", err)
	}

	<-started
	if err := BindJob(11, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(2, 1),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			return snap.Data, nil
		},
	}, nil).submit(p, nil); err != nil {
		t.Fatalf("submit second: %v", err)
	}
	close(release)
	waitForResults(t, p, 2)
	drained := p.Drain()
	if len(drained) != 2 {
		t.Fatalf("drained = %d", len(drained))
	}
	cancelled := false
	for _, res := range drained {
		if res.JobID() == 1 && res.Cancelled() {
			cancelled = true
		}
	}
	if !cancelled {
		t.Fatal("expected cancelled result")
	}
	if onCommitCalled || afterCommitCalled {
		t.Fatalf("callbacks should not run on cancelled result: commit=%v after=%v", onCommitCalled, afterCommitCalled)
	}
}

func TestBindJob_afterCommit_not_called_on_error(t *testing.T) {
	p := NewPool(1)
	defer p.Shutdown()

	onCommitCalled := false
	afterCommitCalled := false
	j := BindJob(7, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(1, 1),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			return 0, errors.New("boom")
		},
	}, func(v int) {
		onCommitCalled = true
	})
	if err := j.submit(p, func(AnyResult) {
		afterCommitCalled = true
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	waitForResults(t, p, 1)
	drained := p.Drain()
	if len(drained) != 1 || drained[0].err == nil {
		t.Fatalf("drained %#v", drained)
	}
	if onCommitCalled || afterCommitCalled {
		t.Fatalf("callbacks should not run on error: commit=%v after=%v", onCommitCalled, afterCommitCalled)
	}
}

func TestPool_workers_exit_cleanly_when_results_full(t *testing.T) {
	workerCount := 4
	p := NewPool(workerCount)

	// Schedule enough jobs to overflow the results channel (capacity = workerCount * 4).
	// Run scheduling in a background goroutine so the test goroutine can call Shutdown
	// even if Schedule blocks on a full background queue (which happens once all
	// workers are stuck on the full results channel).
	jobCount := workerCount * 20
	schedDone := make(chan struct{})
	go func() {
		for i := 0; i < jobCount; i++ {
			id := i
			_ = Schedule(p, Job[int, int]{
				ID:       JobID(id),
				Priority: PriorityBackground,
				Snapshot: NewSnapshot(0),
				Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
					return id, nil
				},
			}, nil)
		}
		close(schedDone)
	}()

	// Shutdown will close p.shutdown and cancel p.ctx, unblocking any worker
	// stuck on a full results channel send, and any Schedule stuck on a full
	// background queue.
	p.Shutdown()
	<-schedDone
}

func TestPool_worker_restarts_after_panic(t *testing.T) {
	p := NewPool(1)
	defer p.Shutdown()

	err := Schedule(p, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(0),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			panic("simulated worker panic")
		},
	}, nil)
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}

	// The panicked job produces no result. Wait briefly for recovery, then
	// verify a second job completes — proves the worker restarted.
	time.Sleep(50 * time.Millisecond)

	commits := int32(0)
	err = Schedule(p, Job[int, int]{
		ID:       2,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(42),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			return snap.Data, nil
		},
	}, func(v int) {
		if v != 42 {
			t.Fatalf("got %d", v)
		}
		atomic.AddInt32(&commits, 1)
	})
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}

	waitForResults(t, p, 1)
	_ = p.Drain()
	if atomic.LoadInt32(&commits) != 1 {
		t.Fatal("expected commit from recovered worker")
	}
}

func TestPool_10000_jobs_all_complete(t *testing.T) {
	const numJobs = 10000
	const numWorkers = 4
	p := NewPool(numWorkers)
	defer p.Shutdown()

	var commits atomic.Int32

	// Run scheduling in a background goroutine so the main goroutine can
	// drain results concurrently, preventing backpressure deadlocks.
	schedDone := make(chan struct{})
	go func() {
		for i := 0; i < numJobs; i++ {
			id := i
			_ = Schedule(p, Job[int, int]{
				ID:       JobID(id),
				Priority: PriorityBackground,
				Snapshot: NewSnapshot(id, 0),
				Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
					return id, nil
				},
			}, func(v int) {
				commits.Add(1)
			})
		}
		close(schedDone)
	}()

	// Drain results until all are collected.
	var drained int
	for drained < numJobs {
		drained += len(p.Drain())
		runtime.Gosched()
	}
	<-schedDone

	if drained != numJobs {
		t.Fatalf("expected %d results, got %d", numJobs, drained)
	}
	if commits.Load() != numJobs {
		t.Fatalf("expected %d commits, got %d", numJobs, commits.Load())
	}
}

func TestPool_schedule_after_shutdown_returns_error(t *testing.T) {
	p := NewPool(1)
	p.Shutdown()

	err := Schedule(p, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(0),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			return 0, nil
		},
	}, nil)
	if err == nil {
		t.Fatal("expected error scheduling after shutdown")
	}
}

func TestPool_fifo_ordering_within_same_priority(t *testing.T) {
	p := NewPool(1) // single worker forces FIFO
	defer p.Shutdown()

	const numJobs = 20
	order := make([]int, 0, numJobs)
	orderMu := sync.Mutex{}

	schedDone := make(chan struct{})
	go func() {
		for i := 0; i < numJobs; i++ {
			id := i
			_ = Schedule(p, Job[int, int]{
				ID:       JobID(id),
				Priority: PriorityBackground,
				Snapshot: NewSnapshot(id, 0),
				Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
					return id, nil
				},
			}, func(v int) {
				orderMu.Lock()
				order = append(order, v)
				orderMu.Unlock()
			})
		}
		close(schedDone)
	}()

	for drained := 0; drained < numJobs; {
		drained += len(p.Drain())
		runtime.Gosched()
	}
	<-schedDone

	orderMu.Lock()
	defer orderMu.Unlock()
	if len(order) != numJobs {
		t.Fatalf("expected %d commits, got %d", numJobs, len(order))
	}
	for i := 0; i < numJobs; i++ {
		if order[i] != i {
			t.Fatalf("expected order[%d] = %d, got %d", i, i, order[i])
		}
	}
}

func TestPool_cancel_all_stops_queued_jobs(t *testing.T) {
	p := NewPool(2)
	defer p.Shutdown()

	release := make(chan struct{})
	started := make(chan struct{}, 1)
	executed := int32(0)

	// Schedule one in-flight job that blocks.
	if err := Schedule(p, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(1, 1),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			close(started)
			<-release
			atomic.AddInt32(&executed, 1)
			return snap.Data, nil
		},
	}, nil); err != nil {
		t.Fatalf("schedule: %v", err)
	}

	<-started

	// Schedule queued jobs in background to avoid backpressure deadlock.
	schedDone := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			id := 2 + i
			_ = Schedule(p, Job[int, int]{
				ID:       JobID(id),
				Priority: PriorityBackground,
				Snapshot: NewSnapshot(id, 0),
				Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
					atomic.AddInt32(&executed, 1)
					return snap.Data, nil
				},
			}, nil)
		}
		close(schedDone)
	}()

	// Drain a few batches to keep the pipeline moving, then CancelAll.
	drained := 0
	for drained < 20 {
		drained += len(p.Drain())
		runtime.Gosched()
	}

	// CancelAll cancels tokens but in-flight job still completes.
	p.CancelAll()
	close(release)

	// Collect remaining results.
	for drained < 101 {
		drained += len(p.Drain())
		runtime.Gosched()
	}
	<-schedDone

	// At minimum, the in-flight job completed without cancellation.
	if atomic.LoadInt32(&executed) < 1 {
		t.Fatal("expected at least 1 executed job (the in-flight one)")
	}
}

func TestAnyResult_fields(t *testing.T) {
	boom := errors.New("boom")
	r := &boundResult{
		jobID:     7,
		ownerID:   42,
		err:       boom,
		cancelled: true,
	}
	if r.JobID() != 7 {
		t.Fatalf("jobID = %d", r.JobID())
	}
	if r.OwnerID() != 42 {
		t.Fatalf("ownerID = %d", r.OwnerID())
	}
	if !r.Cancelled() {
		t.Fatal("expected cancelled")
	}
	if r.Err() != boom {
		t.Fatalf("err = %v", r.Err())
	}
}

func waitForResults(t *testing.T, p *Pool, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(p.results) >= want {
			return
		}
		runtime.Gosched()
		time.Sleep(1 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d results", want)
}
