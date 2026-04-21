package runtime

import (
	"sync"
	"time"
)

var (
	phase1HooksMu sync.RWMutex
	phase1Hooks   []func(time.Duration)
)

// RegisterPhase1TickHook registers a callback that runs at the start of phase 1.
// It returns an unregister function.
func RegisterPhase1TickHook(fn func(time.Duration)) func() {
	if fn == nil {
		return func() {}
	}
	phase1HooksMu.Lock()
	phase1Hooks = append(phase1Hooks, fn)
	index := len(phase1Hooks) - 1
	phase1HooksMu.Unlock()
	return func() {
		phase1HooksMu.Lock()
		if index >= 0 && index < len(phase1Hooks) && phase1Hooks[index] != nil {
			phase1Hooks[index] = nil
		}
		phase1HooksMu.Unlock()
	}
}

func runPhase1TickHooks(dt time.Duration) {
	phase1HooksMu.RLock()
	hooks := append([]func(time.Duration){}, phase1Hooks...)
	phase1HooksMu.RUnlock()
	for _, hook := range hooks {
		if hook != nil {
			hook(dt)
		}
	}
}
