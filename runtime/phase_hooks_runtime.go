package runtime

import (
	"time"
)

type phase1HookSlot struct {
	fn         func(time.Duration)
	generation uint64
	sequence   uint64
}

// RegisterPhase1TickHook registers a phase-1 callback on this runtime.
func (rt *Runtime) RegisterPhase1TickHook(fn func(time.Duration)) func() {
	if fn == nil {
		return func() {}
	}
	rt.phase1HooksMu.Lock()
	var index int
	if n := len(rt.phase1HookFree); n > 0 {
		index = rt.phase1HookFree[n-1]
		rt.phase1HookFree = rt.phase1HookFree[:n-1]
		rt.phase1Hooks[index].generation++
	} else {
		index = len(rt.phase1Hooks)
		rt.phase1Hooks = append(rt.phase1Hooks, phase1HookSlot{generation: 1})
	}
	rt.phase1HookSeq++
	sequence := rt.phase1HookSeq
	generation := rt.phase1Hooks[index].generation
	rt.phase1Hooks[index].sequence = sequence
	rt.phase1Hooks[index].fn = fn
	rt.phase1HooksMu.Unlock()
	var released bool
	return func() {
		rt.phase1HooksMu.Lock()
		if !released && index >= 0 && index < len(rt.phase1Hooks) {
			slot := &rt.phase1Hooks[index]
			if slot.generation == generation && slot.sequence == sequence && slot.fn != nil {
				slot.fn = nil
				rt.phase1HookFree = append(rt.phase1HookFree, index)
			}
			released = true
		}
		rt.phase1HooksMu.Unlock()
	}
}

func (rt *Runtime) runPhase1TickHooks(dt time.Duration) {
	rt.phase1HooksMu.RLock()
	limit := rt.phase1HookSeq
	length := len(rt.phase1Hooks)
	rt.phase1HooksMu.RUnlock()
	for i := 0; i < length; i++ {
		rt.phase1HooksMu.RLock()
		slot := rt.phase1Hooks[i]
		rt.phase1HooksMu.RUnlock()
		if slot.fn != nil && slot.sequence <= limit {
			slot.fn(dt)
		}
	}
}

func (rt *Runtime) clearPhase1TickHooks() {
	rt.phase1HooksMu.Lock()
	clear(rt.phase1Hooks)
	rt.phase1Hooks = nil
	rt.phase1HookFree = nil
	rt.phase1HooksMu.Unlock()
}

// RegisterShutdownHook registers a callback that runs during runtime shutdown.
func (rt *Runtime) RegisterShutdownHook(fn func()) func() {
	if fn == nil {
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
	rt.shutdownHooksMu.Lock()
	rt.shutdownHooks = nil
	rt.shutdownHooksMu.Unlock()
}
