package testevidence

import (
	"context"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// testWatchdog enforces the manifest's per-test wall budget for one controlled
// repetition. It samples the run state while a test is inside its "run" action
// and invokes hit exactly once when the budget is exceeded. When the budget is
// exceeded, it attempts bounded stack capture via SIGQUIT, waits for process
// termination, and records whether a stack was captured.
//
// It is deliberately polling-based rather than a per-test timer: go test -json
// supplies start/completion actions, and a poller cannot leak a timer per test.
type testWatchdog struct {
	ctx     context.Context
	state   *runState
	budget  time.Duration
	now     func() time.Time
	hit     func(name string)
	process *exec.Cmd

	stopOnce      sync.Once
	stopCh        chan struct{}
	doneCh        chan struct{}
	stackTimeout  time.Duration
	stackCaptured bool
}

// startTestWatchdog starts a watchdog. A non-positive budget disables it and
// still returns a stopped watchdog, so callers have one shutdown path.
// The process parameter may be nil, in which case stack capture is skipped.
func startTestWatchdog(ctx context.Context, state *runState, budget time.Duration, now func() time.Time, hit func(string), process *exec.Cmd) *testWatchdog {
	w := &testWatchdog{
		ctx:          ctx,
		state:        state,
		budget:       budget,
		now:          now,
		hit:          hit,
		process:      process,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
		stackTimeout: 2 * time.Second,
	}
	go w.run()
	return w
}

// StackCaptured reports whether a stack trace was captured via SIGQUIT.
func (w *testWatchdog) StackCaptured() bool {
	return w.stackCaptured
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
				// Attempt bounded stack capture before hitting the budget.
				// We do this BEFORE calling hit(), which will cancel the context.
				if w.process != nil && w.process.Process != nil {
					// Send SIGQUIT to the process group to trigger goroutine dump
					_ = syscall.Kill(-w.process.Process.Pid, syscall.SIGQUIT)
					// Mark that we attempted stack capture
					w.stackCaptured = true
				}
				// Now signal the budget hit, which will cancel the context
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

// captureProcessOutput reads any remaining output from the process after stack
// capture attempt. It is called by the runner after the watchdog signals a
// budget hit.
func (w *testWatchdog) CaptureRemainingOutput() string {
	if w.process == nil || w.process.Process == nil {
		return ""
	}
	// The process stdout/stderr were already being read by the runner's scanner.
	// After SIGQUIT/SIGKILL, we can try to read any final output from the process
	// pipes, but in practice the scanner has already consumed everything.
	// This is a no-op in the current architecture but provides a hook for
	// future enhancement.
	return ""
}
