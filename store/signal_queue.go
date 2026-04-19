package store

import "sync"

var (
	signalQueueMu   sync.RWMutex
	signalQueueHook func(func())
)

// SetSignalQueueHook installs a callback used to defer store notifications.
// Passing nil restores immediate delivery.
func SetSignalQueueHook(hook func(func())) {
	signalQueueMu.Lock()
	signalQueueHook = hook
	signalQueueMu.Unlock()
}

func enqueueSignal(fn func()) {
	signalQueueMu.RLock()
	hook := signalQueueHook
	signalQueueMu.RUnlock()
	if hook != nil {
		hook(fn)
		return
	}
	fn()
}
