package input

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
)

// RecoveryHook decides whether a facet callback should run during delivery.
// It returns true if the callback ran to completion, false if it was skipped
// (facet quarantined) or panicked (now quarantined). The runtime installs its
// recovery hook in start() and clears it in shutdown(); it is nil in unit
// tests of input in isolation, in which case callbacks run unrecovered —
// today's behavior.
type RecoveryHook func(id facet.FacetID, role string, cb func()) bool

var (
	recoveryHook   RecoveryHook
	recoveryHookMu sync.RWMutex
)

// SetRecoveryHook installs the facet-callback recovery hook.
func SetRecoveryHook(h RecoveryHook) {
	recoveryHookMu.Lock()
	recoveryHook = h
	recoveryHookMu.Unlock()
}

// ClearRecoveryHook removes the facet-callback recovery hook.
func ClearRecoveryHook() {
	SetRecoveryHook(nil)
}

func currentRecoveryHook() RecoveryHook {
	recoveryHookMu.RLock()
	defer recoveryHookMu.RUnlock()
	return recoveryHook
}

// runRecovered invokes a bool-returning facet callback under the installed
// recovery hook. With no hook installed (isolated input tests) the callback
// runs directly, preserving today's behavior. The returned bool is the
// callback's own "event handled" result, or false when the callback was
// skipped because its facet is quarantined.
func runRecovered(role string, id facet.FacetID, cb func() bool) bool {
	h := currentRecoveryHook()
	if h == nil {
		return cb()
	}
	ran := false
	if h(id, role, func() { ran = cb() }) {
		return ran
	}
	return false
}
