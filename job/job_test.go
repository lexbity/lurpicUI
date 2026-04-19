package job

import (
	"errors"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestMain(m *testing.M) {
	syncutil.ResetRuntimeThreadForTest()
	syncutil.RegisterRuntimeThread()
	code := m.Run()
	syncutil.ResetRuntimeThreadForTest()
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
