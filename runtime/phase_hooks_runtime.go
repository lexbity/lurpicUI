package runtime

import (
	"time"
)

// RegisterPhase1TickHook registers a phase-1 callback on this runtime.
// A nil runtime falls back to the process-global hook registry for tests
// and standalone timelines.
func (rt *Runtime) RegisterPhase1TickHook(fn func(time.Duration)) func() {
	if fn == nil {
		return func() {}
	}
	if rt == nil {
		return RegisterPhase1TickHook(fn)
	}
	rt.phase1HooksMu.Lock()
	rt.phase1Hooks = append(rt.phase1Hooks, fn)
	index := len(rt.phase1Hooks) - 1
	rt.phase1HooksMu.Unlock()
	return func() {
		rt.phase1HooksMu.Lock()
		if index >= 0 && index < len(rt.phase1Hooks) && rt.phase1Hooks[index] != nil {
			rt.phase1Hooks[index] = nil
		}
		rt.phase1HooksMu.Unlock()
	}
}

func (rt *Runtime) runPhase1TickHooks(dt time.Duration) {
	if rt == nil {
		runPhase1TickHooks(dt)
		return
	}
	rt.phase1HooksMu.RLock()
	hooks := append([]func(time.Duration){}, rt.phase1Hooks...)
	rt.phase1HooksMu.RUnlock()
	for _, hook := range hooks {
		if hook != nil {
			hook(dt)
		}
	}
}

func (rt *Runtime) clearPhase1TickHooks() {
	if rt == nil {
		return
	}
	rt.phase1HooksMu.Lock()
	rt.phase1Hooks = nil
	rt.phase1HooksMu.Unlock()
}
