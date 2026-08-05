package facet

import "sync"

// CallbackRecoveryHook decides whether a facet callback should run during
// synchronous focus management. It returns true if the callback ran to
// completion, false if it was skipped (facet quarantined) or panicked (now
// quarantined). The runtime installs its hook in start() and clears it in
// shutdown(); it is nil in unit tests of facet in isolation, in which case
// callbacks run unrecovered — today's behavior. This is a separate hook from
// input.RecoveryHook because facet and input are independently tested and
// independently imported; a shared hook in a third package would add a
// dependency edge for no gain.
type CallbackRecoveryHook func(id FacetID, role string, cb func()) bool

var (
	callbackRecoveryHook   CallbackRecoveryHook
	callbackRecoveryHookMu sync.RWMutex
)

// SetCallbackRecoveryHook installs the facet-callback recovery hook.
func SetCallbackRecoveryHook(h CallbackRecoveryHook) {
	callbackRecoveryHookMu.Lock()
	callbackRecoveryHook = h
	callbackRecoveryHookMu.Unlock()
}

// ClearCallbackRecoveryHook removes the facet-callback recovery hook.
func ClearCallbackRecoveryHook() {
	SetCallbackRecoveryHook(nil)
}

func currentCallbackRecoveryHook() CallbackRecoveryHook {
	callbackRecoveryHookMu.RLock()
	defer callbackRecoveryHookMu.RUnlock()
	return callbackRecoveryHook
}

// runFocusRecovered invokes a focus callback under the installed recovery
// hook. With no hook installed (isolated facet tests) the callback runs
// directly, preserving today's behavior. Focus callbacks return no value, so
// the hook's bool result is not propagated — a quarantined facet simply does
// not fire its focus callback.
func runFocusRecovered(role string, id FacetID, cb func()) {
	h := currentCallbackRecoveryHook()
	if h == nil {
		cb()
		return
	}
	h(id, role, cb)
}
