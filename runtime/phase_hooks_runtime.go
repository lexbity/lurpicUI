package runtime

import (
	"time"
)

// RegisterPhase1TickHook registers a phase-1 callback on this runtime.
func (rt *Runtime) RegisterPhase1TickHook(fn func(time.Duration)) func() {
	if fn == nil {
		return func() {}
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
	rt.phase1HooksMu.Lock()
	rt.phase1Hooks = nil
	rt.phase1HooksMu.Unlock()
}

// RegisterShutdownHook registers a callback that runs during runtime shutdown.
func (rt *Runtime) RegisterShutdownHook(fn func()) func() {
	if fn == nil {
		return func() {}
	}
	if rt == nil {
		return func() {}
	}
	rt.shutdownHooksMu.Lock()
	rt.shutdownHooks = append(rt.shutdownHooks, fn)
	index := len(rt.shutdownHooks) - 1
	rt.shutdownHooksMu.Unlock()
	return func() {
		rt.shutdownHooksMu.Lock()
		if index >= 0 && index < len(rt.shutdownHooks) && rt.shutdownHooks[index] != nil {
			rt.shutdownHooks[index] = nil
		}
		rt.shutdownHooksMu.Unlock()
	}
}

func (rt *Runtime) runShutdownHooks() {
	if rt == nil {
		return
	}
	rt.shutdownHooksMu.RLock()
	hooks := append([]func(){}, rt.shutdownHooks...)
	rt.shutdownHooksMu.RUnlock()
	for _, hook := range hooks {
		if hook != nil {
			hook()
		}
	}
}

func (rt *Runtime) clearShutdownHooks() {
	if rt == nil {
		return
	}
	rt.shutdownHooksMu.Lock()
	rt.shutdownHooks = nil
	rt.shutdownHooksMu.Unlock()
}
