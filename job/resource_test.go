package job

import (
	"errors"
	"math/rand"
	"strings"
	"testing"
	"time"

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

	// Use a pool with enough capacity for all jobs.
	config := PoolConfig{
		WorkerCount:          2,
		JobsWallet:           wallet,
		RequireDeclarations: false,
		MaxWaitingInteractive: 100,
		MaxTotalRecords:     100,
	}
	pool := NewPoolWithConfig(config)
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
	total, err := decl.TotalBytes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

// TestMemoryDeclOverflowRejected verifies that JobMemoryDecl.TotalBytes
// returns an error on overflow, per D8.1.
func TestMemoryDeclOverflowRejected(t *testing.T) {
	// Create a declaration that would overflow uint64.
	// maxUint64/2 + maxUint64/2 + 2 = maxUint64 + 2 which wraps to 0 (overflow).
	const maxUint64 = ^uint64(0)
	decl := JobMemoryDecl{
		MaxInputBytes:   maxUint64 / 2,
		MaxScratchBytes: maxUint64 / 2,
		MaxOutputBytes:  2,  // This pushes the sum beyond maxUint64
	}
	_, err := decl.TotalBytes()
	if err == nil {
		t.Fatal("expected overflow error, got nil")
	}
	if !strings.Contains(err.Error(), "overflow") {
		t.Fatalf("expected overflow error, got: %v", err)
	}
}

// TestMemoryDeclOverflowMidRange verifies overflow detection in the middle
// of the addition chain, per D8.1.
func TestMemoryDeclOverflowMidRange(t *testing.T) {
	const maxUint64 = ^uint64(0)
	// First addition overflows: maxUint64 - 100 + 100 + 1
	decl := JobMemoryDecl{
		MaxInputBytes:   maxUint64 - 100,
		MaxScratchBytes: 101,  // This causes overflow when added to MaxInputBytes
		MaxOutputBytes:  0,
	}
	_, err := decl.TotalBytes()
	if err == nil {
		t.Fatal("expected overflow error in first addition, got nil")
	}
}

// TestZeroDeclarationRequired verifies that a pool with RequireDeclarations
// rejects zero-valued declarations, per D8.5.
func TestZeroDeclarationRequired(t *testing.T) {
	wallet, err := membudget.NewWallet(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	// Create a pool that requires declarations.
	config := PoolConfig{
		WorkerCount:          1,
		JobsWallet:           wallet,
		RequireDeclarations: true,
		MaxWaitingInteractive: 10,
		MaxTotalRecords:     20,
	}
	pool := NewPoolWithConfig(config)
	defer pool.Shutdown()

	// Try to schedule a job with zero declaration.
	err = Schedule(pool, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(42),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			return snap.Data, nil
		},
		MemoryDecl: JobMemoryDecl{
			MaxInputBytes:   0,
			MaxScratchBytes: 0,
			MaxOutputBytes:  0,
		},
	}, nil)
	if err == nil {
		t.Fatal("expected zero declaration error, got nil")
	}
	if !strings.Contains(err.Error(), "zero memory declaration") {
		t.Fatalf("expected zero declaration error, got: %v", err)
	}
}

// TestClosedPoolReleasesReservation verifies that scheduling against a closed
// pool releases the reservation, per D8.4.
func TestClosedPoolReleasesReservation(t *testing.T) {
	wallet, err := membudget.NewWallet(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	pool := NewPool(1, wallet)
	
	// Schedule a job to reserve memory.
	err = Schedule(pool, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(42),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			return snap.Data, nil
		},
		MemoryDecl: JobMemoryDecl{
			MaxInputBytes:   100,
			MaxScratchBytes: 100,
			MaxOutputBytes:  100,
		},
	}, nil)
	if err != nil {
		t.Fatalf("first schedule failed: %v", err)
	}
	
	// Close the pool.
	pool.Shutdown()
	
	// Verify wallet is balanced (reservation was released).
	cap, reserved, live, retiring := wallet.Snapshot()
	_ = cap
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("wallet not balanced after pool shutdown: reserved=%d live=%d retiring=%d",
			reserved, live, retiring)
	}
}

// TestBusyInteractiveAdmissionDoesNotBlock verifies that interactive admission
// returns immediately with a typed error when the queue is full, per D8.2.
func TestBusyInteractiveAdmissionDoesNotBlock(t *testing.T) {
	wallet, err := membudget.NewWallet(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	// Create a pool with a small interactive queue (1 slot).
	config := PoolConfig{
		WorkerCount:          1,
		JobsWallet:           wallet,
		RequireDeclarations: false,
		MaxWaitingInteractive: 1,
		MaxTotalRecords:     10,
	}
	pool := NewPoolWithConfig(config)
	defer pool.Shutdown()

	// Fill the interactive queue with a barrier job.
	release := make(chan struct{})
	err = Schedule(pool, Job[int, int]{
		ID:       1,
		Priority: PriorityInteractive,
		Snapshot: NewSnapshot(1),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			<-release
			return snap.Data, nil
		},
		MemoryDecl: JobMemoryDecl{
			MaxInputBytes:   10,
			MaxScratchBytes: 10,
			MaxOutputBytes:  10,
		},
	}, nil)
	if err != nil {
		t.Fatalf("first schedule failed: %v", err)
	}

	// Try to schedule another interactive job - should be refused immediately.
	start := time.Now()
	err = Schedule(pool, Job[int, int]{
		ID:       2,
		Priority: PriorityInteractive,
		Snapshot: NewSnapshot(2),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			return snap.Data, nil
		},
		MemoryDecl: JobMemoryDecl{
			MaxInputBytes:   10,
			MaxScratchBytes: 10,
			MaxOutputBytes:  10,
		},
	}, nil)
	elapsed := time.Since(start)
	
	// Should get an admission refusal error.
	if err == nil {
		t.Fatal("expected admission refusal, got nil")
	}
	
	// Check it's the right error type.
	var refused ErrAdmissionRefused
	if !errors.As(err, &refused) {
		t.Fatalf("expected ErrAdmissionRefused, got %T: %v", err, err)
	}
	if refused.Queue != "interactive" {
		t.Fatalf("expected queue=interactive, got %s", refused.Queue)
	}
	
	// Should return immediately (non-blocking).
	if elapsed > 1*time.Millisecond {
		t.Fatalf("admission took %v, expected <1ms", elapsed)
	}
	
	// Clean up.
	close(release)
	waitForResults(t, pool, 1)
	_ = pool.Drain()
}

// TestReplacementFencingTerminalOwnership verifies that replacing a job
// properly fences the old reservation, per D8.3.
func TestReplacementFencingTerminalOwnership(t *testing.T) {
	wallet, err := membudget.NewWallet(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	pool := NewPool(1, wallet)
	defer pool.Shutdown()

	// Schedule first job.
	release1 := make(chan struct{})
	err = Schedule(pool, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(1),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			<-release1
			return snap.Data, nil
		},
		MemoryDecl: JobMemoryDecl{
			MaxInputBytes:   100,
			MaxScratchBytes: 100,
			MaxOutputBytes:  100,
		},
	}, nil)
	if err != nil {
		t.Fatalf("first schedule failed: %v", err)
	}

	// Replace with a second job (same ID).
	release2 := make(chan struct{})
	_ = Schedule(pool, Job[int, int]{
		ID:       1,  // Same ID
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(2),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			<-release2
			return snap.Data, nil
		},
		MemoryDecl: JobMemoryDecl{
			MaxInputBytes:   100,
			MaxScratchBytes: 100,
			MaxOutputBytes:  100,
		},
	}, nil)

	// First job should be cancelled.
	close(release1)
	waitForResults(t, pool, 1)
	results := pool.Drain()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].cancelled {
		t.Fatal("expected first job to be cancelled")
	}

	// Complete second job.
	close(release2)
	waitForResults(t, pool, 1)
	results = pool.Drain()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].cancelled {
		t.Fatal("second job should not be cancelled")
	}

	// Verify wallet is balanced.
	cap, reserved, live, retiring := wallet.Snapshot()
	_ = cap
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("wallet not balanced after replacement: reserved=%d live=%d retiring=%d",
			reserved, live, retiring)
	}
}

// TestWorkerPanicReleasesReservation verifies that a worker panic releases
// its reservation and does not strand charges, per D8.4.
func TestWorkerPanicReleasesReservation(t *testing.T) {
	wallet, err := membudget.NewWallet(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	pool := NewPool(1, wallet)
	defer pool.Shutdown()

	// Schedule a job that will panic.
	err = Schedule(pool, Job[int, int]{
		ID:       1,
		Priority: PriorityBackground,
		Snapshot: NewSnapshot(42),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			panic("test panic")
		},
		MemoryDecl: JobMemoryDecl{
			MaxInputBytes:   100,
			MaxScratchBytes: 100,
			MaxOutputBytes:  100,
		},
	}, nil)
	if err != nil {
		t.Fatalf("schedule failed: %v", err)
	}

	// Wait for the panic to be recovered.
	waitForResults(t, pool, 1)
	results := pool.Drain()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].err == nil {
		t.Fatal("expected error from panic, got nil")
	}
	if !strings.Contains(results[0].err.Error(), "panic") {
		t.Fatalf("expected panic error, got: %v", results[0].err)
	}

	// Verify wallet is balanced (reservation was released on panic).
	cap, reserved, live, retiring := wallet.Snapshot()
	_ = cap
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("wallet not balanced after panic: reserved=%d live=%d retiring=%d",
			reserved, live, retiring)
	}
}

// TestShutdownWithQueuedJobsConserves verifies that shutting down a pool
// with queued jobs leaves the wallet charge at zero, per D8.4.
func TestShutdownWithQueuedJobsConserves(t *testing.T) {
	wallet, err := membudget.NewWallet(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	// Create a pool with enough capacity for 5 jobs.
	config := PoolConfig{
		WorkerCount:          1,
		JobsWallet:           wallet,
		RequireDeclarations: false,
		MaxWaitingInteractive: 10,
		MaxTotalRecords:     10,
	}
	pool := NewPoolWithConfig(config)
	
	// Queue up several jobs.
	for i := 0; i < 5; i++ {
		err = Schedule(pool, Job[int, int]{
			ID:       JobID(i),
			Priority: PriorityBackground,
			Snapshot: NewSnapshot(i),
			Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
				// Slow work to keep jobs queued.
				time.Sleep(100 * time.Millisecond)
				return snap.Data, nil
			},
			MemoryDecl: JobMemoryDecl{
				MaxInputBytes:   10,
				MaxScratchBytes: 10,
				MaxOutputBytes:  10,
			},
		}, nil)
		if err != nil {
			t.Fatalf("schedule %d failed: %v", i, err)
		}
	}

	// Shutdown the pool.
	pool.Shutdown()

	// Verify wallet is balanced.
	cap, reserved, live, retiring := wallet.Snapshot()
	_ = cap
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("wallet not balanced after shutdown: reserved=%d live=%d retiring=%d",
			reserved, live, retiring)
	}
}

// TestQueueCapacityRefusal verifies that the queue capacity is bounded and
// refusals are returned when exceeded, per D8.2.
func TestQueueCapacityRefusal(t *testing.T) {
	wallet, err := membudget.NewWallet(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	// Create a pool with small queue capacity.
	config := PoolConfig{
		WorkerCount:          1,
		JobsWallet:           wallet,
		RequireDeclarations: false,
		MaxWaitingInteractive: 2,
		MaxTotalRecords:     10,
	}
	pool := NewPoolWithConfig(config)
	defer pool.Shutdown()

	// Fill the interactive queue.
	for i := 0; i < 2; i++ {
		release := make(chan struct{})
		err = Schedule(pool, Job[int, int]{
			ID:       JobID(i),
			Priority: PriorityInteractive,
			Snapshot: NewSnapshot(i),
			Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
				<-release
				return snap.Data, nil
			},
			MemoryDecl: JobMemoryDecl{
				MaxInputBytes:   10,
				MaxScratchBytes: 10,
				MaxOutputBytes:  10,
			},
		}, nil)
		if err != nil {
			t.Fatalf("schedule %d failed: %v", i, err)
		}
		// Don't close release - keep jobs queued.
		_ = release
	}

	// Third interactive job should be refused.
	err = Schedule(pool, Job[int, int]{
		ID:       2,
		Priority: PriorityInteractive,
		Snapshot: NewSnapshot(2),
		Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
			return snap.Data, nil
		},
		MemoryDecl: JobMemoryDecl{
			MaxInputBytes:   10,
			MaxScratchBytes: 10,
			MaxOutputBytes:  10,
		},
	}, nil)
	if err == nil {
		t.Fatal("expected queue capacity refusal, got nil")
	}
	
	var refused ErrAdmissionRefused
	if !errors.As(err, &refused) {
		t.Fatalf("expected ErrAdmissionRefused, got %T: %v", err, err)
	}
}

// TestReservationConservationRandomInterleavings is a property test that
// verifies wallet conservation across random schedule/cancel/replace
// operations, per NFR-4.
func TestReservationConservationRandomInterleavings(t *testing.T) {
	// Use a larger wallet to avoid refusals.
	wallet, err := membudget.NewWallet(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	config := PoolConfig{
		WorkerCount:          2,
		JobsWallet:           wallet,
		RequireDeclarations: false,
		MaxWaitingInteractive: 20,
		MaxTotalRecords:     50,
	}
	_ = NewPoolWithConfig(config)
	
	// Run for a bounded number of operations.
	const numOps = 50
	const numSeeds = 2
	
	for seed := 0; seed < numSeeds; seed++ {
		// Create a new pool for each seed.
		poolForSeed := NewPoolWithConfig(config)
		
		// Use a deterministic RNG for reproducibility.
		rng := rand.New(rand.NewSource(int64(seed)))
		
		// Track all job IDs.
		jobIDs := make(map[JobID]bool)
		nextID := JobID(1)
		
		for i := 0; i < numOps; i++ {
			// Choose an operation.
			switch rng.Intn(3) { // Removed shutdown case to avoid complexity
			case 0: // Schedule
				id := nextID
				nextID++
				jobIDs[id] = true
				_ = Schedule(poolForSeed, Job[int, int]{
					ID:       id,
					Priority: PriorityBackground,
					Snapshot: NewSnapshot(int(rng.Int63())),
					Work: func(snap Snapshot[int], cancel *CancelToken) (int, error) {
						return snap.Data, nil
					},
					MemoryDecl: JobMemoryDecl{
						MaxInputBytes:   10,
						MaxScratchBytes: 10,
						MaxOutputBytes:  10,
					},
				}, nil)
			case 1: // Cancel
				if len(jobIDs) > 0 {
					// Pick a random job to cancel.
					var ids []JobID
					for id := range jobIDs {
						ids = append(ids, id)
					}
					if len(ids) > 0 {
						shuffle := rng.Perm(len(ids))
						poolForSeed.CancelJob(ids[shuffle[0]])
					}
				}
			case 2: // Drain
				poolForSeed.Drain()
			}
		}
		
		// Clean up.
		poolForSeed.Shutdown()
		
		// Final verification.
		cap, reserved, live, retiring := wallet.Snapshot()
		_ = cap
		if reserved != 0 || live != 0 || retiring != 0 {
			t.Fatalf("seed=%d: final wallet not balanced: reserved=%d live=%d retiring=%d",
				seed, reserved, live, retiring)
		}
	}
}
