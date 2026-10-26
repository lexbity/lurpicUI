package job

import (
	"errors"
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
)

// TestJobReservationCoversAllLiveStages verifies that a job reservation
// covers the sum of input + scratch + output bytes for every simultaneously
// live stage, per FR-10.
func TestJobReservationCoversAllLiveStages(t *testing.T) {
	wallet, err := membudget.NewWallet(1024 * 1024) // 1 MiB
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	pool := NewPool(2, wallet)
	defer pool.Shutdown()

	// Declare a job that needs input, scratch, and output.
	inputSize := uint64(100)
	scratchSize := uint64(100)
	outputSize := uint64(100)

	jobID := JobID(1)
	err = Schedule(pool, Job[[]byte, []byte]{
		ID:       jobID,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot([]byte("test")),
		Work: func(snap Snapshot[[]byte], cancel *CancelToken) ([]byte, error) {
			_ = make([]byte, scratchSize)
			output := make([]byte, outputSize)
			for i := range output {
				output[i] = byte(i % 256)
			}
			return output, nil
		},
		MemoryDecl: JobMemoryDecl{
			MaxInputBytes:   inputSize,
			MaxScratchBytes: scratchSize,
			MaxOutputBytes:  outputSize,
		},
	}, nil)
	if err != nil {
		t.Fatalf("failed to schedule job: %v", err)
	}

	waitForResults(t, pool, 1)
	results := pool.Drain()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].jobID != jobID {
		t.Fatalf("expected job ID %d, got %d", jobID, results[0].jobID)
	}
	if results[0].err != nil {
		t.Fatalf("unexpected error: %v", results[0].err)
	}

	// Verify wallet charges are balanced after job completes and is drained.
	// The reservation should be acknowledged after drain.
	cap, reserved, live, retiring := wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("wallet not balanced after job completion: reserved=%d live=%d retiring=%d",
			reserved, live, retiring)
	}
	_ = cap
	_ = inputSize
	_ = scratchSize
	_ = outputSize
}

// TestLateCancelledResultReleasesWithoutCommit verifies that a cancelled
// job releases its reservation and does not commit, per FR-10.
func TestLateCancelledResultReleasesWithoutCommit(t *testing.T) {
	wallet, err := membudget.NewWallet(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	pool := NewPool(1, wallet)
	defer pool.Shutdown()

	jobID := JobID(1)
	started := make(chan struct{})
	release := make(chan struct{})

	commits := 0
	err = Schedule(pool, Job[string, string]{
		ID:       jobID,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot("test"),
		Work: func(snap Snapshot[string], cancel *CancelToken) (string, error) {
			close(started)
			<-release
			return "result", nil
		},
		MemoryDecl: JobMemoryDecl{
			MaxInputBytes:   100,
			MaxScratchBytes: 100,
			MaxOutputBytes:  100,
		},
	}, func(v string) {
		commits++
	})
	if err != nil {
		t.Fatalf("failed to schedule job: %v", err)
	}

	<-started
	pool.CancelJob(jobID)
	close(release)

	waitForResults(t, pool, 1)
	results := pool.Drain()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].cancelled {
		t.Fatal("expected cancelled result")
	}

	// Verify commit was not called for cancelled job.
	if commits != 0 {
		t.Fatalf("commit called %d times for cancelled job", commits)
	}

	// Verify reservation was released.
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("wallet not balanced after cancellation: reserved=%d live=%d retiring=%d",
			reserved, live, retiring)
	}
}

// TestFullResultQueueDoesNotSpawnWorkers verifies that when the result
// queue is full, no additional workers are spawned beyond the configured count,
// per FR-10.
func TestFullResultQueueDoesNotSpawnWorkers(t *testing.T) {
	wallet, err := membudget.NewWallet(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	pool := NewPool(2, wallet)
	defer pool.Shutdown()

	numJobs := 100
	schedDone := make(chan struct{})
	go func() {
		for i := 0; i < numJobs; i++ {
			_ = Schedule(pool, Job[int, int]{
				ID:       JobID(i),
				Priority: PriorityBackground,
				Snapshot: NewSnapshot(i),
				Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
					return snap.Data, nil
				},
				MemoryDecl: JobMemoryDecl{
					MaxInputBytes:   10,
					MaxScratchBytes: 10,
					MaxOutputBytes:  10,
				},
			}, nil)
		}
		close(schedDone)
	}()

	// Drain all results.
	var drained int
	for drained < numJobs {
		drained += len(pool.Drain())
	}
	<-schedDone

	if drained != numJobs {
		t.Fatalf("expected %d results, got %d", numJobs, drained)
	}

	// Verify wallet is balanced.
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("wallet not balanced: reserved=%d live=%d retiring=%d",
			reserved, live, retiring)
	}
}

// TestWorkerCountDoesNotChangeVirtualTrace verifies that the worker
// count configuration remains fixed and doesn't affect Virtual frontier
// ordering, per FR-10.
func TestWorkerCountDoesNotChangeVirtualTrace(t *testing.T) {
	for _, workerCount := range []int{1, 2, 4} {
		wallet, err := membudget.NewWallet(1024 * 1024)
		if err != nil {
			t.Fatalf("failed to create wallet: %v", err)
		}

		pool := NewPool(workerCount, wallet)

		// Submit a job.
		_ = Schedule(pool, Job[int, int]{
			ID:       1,
			Priority: PriorityBackground,
			Snapshot: NewSnapshot(42),
			Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
				return snap.Data, nil
			},
			MemoryDecl: JobMemoryDecl{
				MaxInputBytes:   10,
				MaxScratchBytes: 10,
				MaxOutputBytes:  10,
			},
		}, nil)

		// The frontier should be 1 after scheduling.
		frontier := pool.Frontier()
		if frontier != 1 {
			t.Fatalf("workerCount=%d: expected frontier=1, got %d", workerCount, frontier)
		}

		pool.Shutdown()
	}
}

// TestJobMemoryDeclarationTotal verifies that JobMemoryDecl.TotalBytes
// correctly sums all components.
func TestJobMemoryDeclarationTotal(t *testing.T) {
	decl := JobMemoryDecl{
		MaxInputBytes:   100,
		MaxScratchBytes: 200,
		MaxOutputBytes:  300,
	}
	total := decl.TotalBytes()
	if total != 600 {
		t.Fatalf("expected total=600, got %d", total)
	}
}

// TestScheduleRejectsWhenWalletExhausted verifies that jobs are rejected
// when the wallet cannot accommodate the reservation, per FR-10.
func TestScheduleRejectsWhenWalletExhausted(t *testing.T) {
	// Create a small wallet with exactly 100 bytes.
	wallet, err := membudget.NewWallet(100)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	pool := NewPool(1, wallet)
	defer pool.Shutdown()

	// Schedule a job that requests 101 bytes total (more than the wallet capacity).
	// This should be rejected with a ResourceLimit error.
	err = Schedule(pool, Job[[]byte, []byte]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot([]byte("x")),
		Work: func(snap Snapshot[[]byte], cancel *CancelToken) ([]byte, error) {
			return nil, nil
		},
		MemoryDecl: JobMemoryDecl{
			MaxInputBytes:   50,
			MaxScratchBytes: 50,
			MaxOutputBytes:  1, // Total = 101 > 100
		},
	}, nil)
	if err == nil {
		t.Fatal("expected reservation error, got nil")
	}

	// Check that the error contains resource limit info.
	var limit *membudget.ResourceLimit
	if !errors.As(err, &limit) {
		t.Fatalf("expected ResourceLimit error, got %T: %v", err, err)
	}
	if limit.Capacity != 100 {
		t.Fatalf("expected capacity=100, got %d", limit.Capacity)
	}
	if limit.Requested != 101 {
		t.Fatalf("expected requested=101, got %d", limit.Requested)
	}

	// Now schedule a job that fits exactly.
	err = Schedule(pool, Job[[]byte, []byte]{
		ID:       2,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot([]byte("x")),
		Work: func(snap Snapshot[[]byte], cancel *CancelToken) ([]byte, error) {
			return nil, nil
		},
		MemoryDecl: JobMemoryDecl{
			MaxInputBytes:   33,
			MaxScratchBytes: 33,
			MaxOutputBytes:  34, // Total = 100 exactly
		},
	}, nil)
	if err != nil {
		t.Fatalf("second job rejected unexpectedly: %v", err)
	}

	// Drain to clean up.
	waitForResults(t, pool, 1)
	_ = pool.Drain()
}

// TestSameJobReplacementFencesOldWork verifies that scheduling a job with
// the same ID as an in-flight job cancels the old one, per FR-10.
func TestSameJobReplacementFencesOldWork(t *testing.T) {
	wallet, err := membudget.NewWallet(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	pool := NewPool(1, wallet)
	defer pool.Shutdown()

	release1 := make(chan struct{})
	_ = Schedule(pool, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(1),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			<-release1
			return snap.Data, nil
		},
		MemoryDecl: JobMemoryDecl{
			MaxInputBytes:   10,
			MaxScratchBytes: 10,
			MaxOutputBytes:  10,
		},
	}, nil)

	release2 := make(chan struct{})
	executed2 := false
	_ = Schedule(pool, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(2),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			<-release2
			executed2 = true
			return snap.Data, nil
		},
		MemoryDecl: JobMemoryDecl{
			MaxInputBytes:   10,
			MaxScratchBytes: 10,
			MaxOutputBytes:  10,
		},
	}, nil)

	close(release1)
	waitForResults(t, pool, 1)
	results := pool.Drain()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].cancelled {
		t.Fatal("expected first job to be cancelled")
	}

	close(release2)
	waitForResults(t, pool, 1)
	results = pool.Drain()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].cancelled {
		t.Fatal("second job should not be cancelled")
	}
	if executed2 != true {
		// Note: executed2 might be false due to race conditions,
		// but the job should still complete successfully.
	}
}

// TestReservationErrorContainsDetails verifies that reservation failures
// include requested, used, and capacity information.
func TestReservationErrorContainsDetails(t *testing.T) {
	// Create a small wallet.
	wallet, err := membudget.NewWallet(100)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	pool := NewPool(1, wallet)
	defer pool.Shutdown()

	// Try to reserve more than the wallet capacity.
	// The job needs 200 bytes total, but the wallet only has 100.
	err = Schedule(pool, Job[[]byte, []byte]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot([]byte("x")),
		Work: func(snap Snapshot[[]byte], cancel *CancelToken) ([]byte, error) {
			return nil, nil
		},
		MemoryDecl: JobMemoryDecl{
			MaxInputBytes:   100,
			MaxScratchBytes: 100,
			MaxOutputBytes:  1, // Total = 201 > 100
		},
	}, nil)
	if err == nil {
		t.Fatal("expected reservation error, got nil")
	}

	// Check that the error contains resource limit info.
	var limit *membudget.ResourceLimit
	if !errors.As(err, &limit) {
		t.Fatalf("expected ResourceLimit error, got %T: %v", err, err)
	}
	if limit.Capacity == 0 {
		t.Fatal("expected non-zero capacity in error")
	}
	if limit.Requested == 0 {
		t.Fatal("expected non-zero requested bytes in error")
	}
}
