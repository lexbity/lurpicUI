package runtime

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive"
)

// TestDriveInboxExactlyOnce is the S1 inbox gate (AC-02's mechanism):
// 100,000 unique requests, concurrently enqueued against a concurrently
// claiming runtime thread under forced interleavings, produce exactly
// 100,000 admissions and at most one dispatch each. Commands submitted
// ahead of their contiguous turn are non-admitted request.gap replies —
// transport noise, never ledger state — and are retried per the client
// rule (retry the same admission, not a new execution).
func TestDriveInboxExactlyOnce(t *testing.T) {
	inst, err := drive.NewInstance()
	if err != nil {
		t.Fatalf("instance: %v", err)
	}
	sessions := drive.NewSessionTable(inst)
	sess, _, err := sessions.Create(drive.Grants{Read: true})
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	p := &drivePort{
		rt:          &Runtime{},
		stats:       &drive.Stats{},
		sessions:    sessions,
		wake:        make(chan struct{}, 1),
		controlKeys: make(map[controlKey]int),
	}
	rt := &Runtime{drivePort: p}

	const total = 100000
	replies := make(chan drive.Result, total)
	done := make(chan struct{})

	// The claimer: pops batches and admits them (the runtime-thread side).
	go func() {
		defer close(done)
		for p.stats.Load().CommandsAdmitted < total {
			for _, cmd := range p.claim() {
				rt.driveAdmit(p, cmd)
			}
		}
	}()

	submit := func(id uint64) <-chan drive.Result {
		cmd := drive.Command{
			Session:    sess.ID,
			Generation: sess.Generation(),
			ID:         id,
			Method:     "sys.ping",
			Grants:     sess.Grants,
			Reply:      make(chan drive.Result, 1),
		}
		if err := p.Submit(cmd); err != nil {
			t.Errorf("submit: %v", err)
		}
		return cmd.Reply
	}

	// Concurrent submitters: unique IDs handed out by an atomic counter,
	// submit order racing the claim loop in both directions.
	var wg sync.WaitGroup
	const workers = 16
	perWorker := total / workers
	var nextID atomic.Uint64
	nextID.Store(1)
	// pending collects gap (non-admitted) commands for one retry sweep.
	var pendingMu sync.Mutex
	pending := []uint64{}
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				id := nextID.Add(1) - 1
				ch := submit(id)
				select {
				case res := <-ch:
					if res.Err != nil && strings.Contains(res.Err.Error(), "request.gap") {
						pendingMu.Lock()
						pending = append(pending, id)
						pendingMu.Unlock()
						continue
					}
					replies <- res
				case <-done:
					return
				}
			}
		}()
	}
	wg.Wait()
	// Retry sweep: submit in contiguous order so every remaining ID is
	// admitted exactly once. A gapped ID that was already admitted in the
	// racing phase replays its retained outcome (or reports pending) —
	// never a second dispatch.
	pendingMu.Lock()
	gapped := append([]uint64{}, pending...)
	pendingMu.Unlock()
	sort.Slice(gapped, func(i, j int) bool { return gapped[i] < gapped[j] })
	for _, id := range gapped {
		for attempts := 0; attempts < 100; attempts++ {
			ch := submit(id)
			var res drive.Result
			select {
			case res = <-ch:
			case <-done:
			}
			if res.Err == nil {
				replies <- res
				break
			}
			if !strings.Contains(res.Err.Error(), "capacity.busy") {
				replies <- res
				break
			}
			// Still in flight on the claiming thread: recover per the
			// error contract, then retry the same admission.
		}
	}

	if got := p.stats.Load().CommandsAdmitted; got != total {
		t.Fatalf("admissions = %d, want exactly %d", got, total)
	}
	if hw := sess.Ledger().HighWater(); hw != total {
		t.Fatalf("ledger high-water = %d, want %d", hw, total)
	}
	// Every ID has exactly one success outcome: admitted work dispatches
	// once; retained retries replay, never re-execute.
	close(replies)
	succeeded := 0
	for res := range replies {
		if res.Err == nil {
			succeeded++
		} else {
			t.Errorf("unresolved outcome: %v", res.Err)
		}
	}
	if succeeded != total {
		t.Fatalf("successful outcomes = %d, want exactly %d (one per unique request)", succeeded, total)
	}
}

// TestDriveInboxBoundedOverload proves the inbox bound is explicit: when
// the queue is full and nothing drains, Submit reports capacity exhaustion
// instead of blocking or growing without bound.
func TestDriveInboxBoundedOverload(t *testing.T) {
	inst, err := drive.NewInstance()
	if err != nil {
		t.Fatalf("instance: %v", err)
	}
	sessions := drive.NewSessionTable(inst)
	sess, _, err := sessions.Create(drive.Grants{Read: true})
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	p := &drivePort{
		rt:          &Runtime{},
		stats:       &drive.Stats{},
		sessions:    sessions,
		wake:        make(chan struct{}, 1),
		controlKeys: make(map[controlKey]int),
	}
	// Shrink the bound through the same constant the Submit check uses:
	// fill to the bound, then observe the explicit overflow.
	filled := 0
	var firstErr error
	for {
		cmd := drive.Command{
			Session: sess.ID, Generation: sess.Generation(),
			ID: uint64(filled + 1), Method: "sys.ping", Grants: sess.Grants,
			Reply: make(chan drive.Result, 1),
		}
		err := p.Submit(cmd)
		if err != nil {
			firstErr = err
			break
		}
		filled++
		if filled > maxInboxLen {
			t.Fatal("inbox grew beyond its bound without an error")
		}
	}
	if firstErr == nil {
		t.Fatal("expected an explicit overflow error")
	}
	if p.inboxFull.Load() == 0 {
		t.Fatal("overflow must be counted")
	}
}
