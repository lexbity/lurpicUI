package testevidence

import (
	"context"
	"sync"
	"time"
)

// testWatchdog enforces the manifest's per-test wall budget for one controlled
// repetition. It samples the run state while a test is inside its "run" action
// and invokes hit exactly once when the budget is exceeded. The caller owns
// process termination; the watchdog only decides that the budget is gone.
//
// It is deliberately polling-based rather than a per-test timer: go test -json
// supplies start/completion actions, and a poller cannot leak a timer per test.
type testWatchdog struct {
	ctx    context.Context
	state  *runState
	budget time.Duration
	now    func() time.Time
	hit    func(name string)

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// startTestWatchdog starts a watchdog. A non-positive budget disables it and
// still returns a stopped watchdog, so callers have one shutdown path.
func startTestWatchdog(ctx context.Context, state *runState, budget time.Duration, now func() time.Time, hit func(string)) *testWatchdog {
	w := &testWatchdog{
		ctx:    ctx,
		state:  state,
		budget: budget,
		now:    now,
		hit:    hit,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	go w.run()
	return w
}

func (w *testWatchdog) run() {
	defer close(w.doneCh)
	if w.budget <= 0 {
		<-w.stopCh
		return
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.stopCh:
			return
		case now := <-ticker.C:
			w.state.mu.Lock()
			var over string
			for name, started := range w.state.running {
				if now.Sub(started) > w.budget {
					over = name
					break
				}
			}
			w.state.mu.Unlock()
			if over != "" {
				w.hit(over)
				return
			}
		}
	}
}

// Stop ends sampling and waits for the sampler goroutine to finish. It is
// idempotent and never blocks on the process under test.
func (w *testWatchdog) Stop() {
	if w == nil {
		return
	}
	w.stopOnce.Do(func() { close(w.stopCh) })
	<-w.doneCh
}
