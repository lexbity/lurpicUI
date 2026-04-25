package android

import (
	"fmt"
	"sort"
	"sync"
)

// Storage describes Android storage behavior for a given API level.
type Storage interface {
	UsesScopedStorage() bool
}

// BackHandler describes Android back-navigation behavior for a given API level.
type BackHandler interface {
	SupportsPredictiveBack() bool
}

// Implementation groups the API-level-specific Android behavior registered by
// platform/android/apiNN packages.
type Implementation interface {
	APILevel() int
	Storage() Storage
	BackHandler() BackHandler
}

type registryState struct {
	mu    sync.RWMutex
	impls map[int]Implementation
}

var androidRegistry = registryState{impls: make(map[int]Implementation)}

// RegisterImplementation registers an API-level Android implementation.
func RegisterImplementation(impl Implementation) {
	if impl == nil {
		panic("android: cannot register nil implementation")
	}
	level := impl.APILevel()
	if level <= 0 {
		panic(fmt.Sprintf("android: invalid API level %d", level))
	}
	androidRegistry.mu.Lock()
	defer androidRegistry.mu.Unlock()
	if existing, ok := androidRegistry.impls[level]; ok && existing != nil {
		panic(fmt.Sprintf("android: API level %d already registered", level))
	}
	androidRegistry.impls[level] = impl
}

// ResetRegistryForTest clears the registry. It is intended for tests only.
func ResetRegistryForTest() {
	androidRegistry.mu.Lock()
	defer androidRegistry.mu.Unlock()
	androidRegistry.impls = make(map[int]Implementation)
}

// RegisteredAPIs returns the registered API levels in ascending order.
func RegisteredAPIs() []int {
	androidRegistry.mu.RLock()
	defer androidRegistry.mu.RUnlock()
	levels := make([]int, 0, len(androidRegistry.impls))
	for level := range androidRegistry.impls {
		levels = append(levels, level)
	}
	sort.Ints(levels)
	return levels
}

// ActiveImplementation returns the highest registered API implementation.
func ActiveImplementation() (Implementation, bool) {
	return SelectImplementation(0)
}

// SelectImplementation resolves the best implementation for the supplied target SDK.
// When targetSDK is zero, the highest registered API is returned.
func SelectImplementation(targetSDK int) (Implementation, bool) {
	androidRegistry.mu.RLock()
	defer androidRegistry.mu.RUnlock()
	if len(androidRegistry.impls) == 0 {
		return nil, false
	}
	levels := make([]int, 0, len(androidRegistry.impls))
	for level := range androidRegistry.impls {
		levels = append(levels, level)
	}
	sort.Ints(levels)
	if targetSDK <= 0 {
		level := levels[len(levels)-1]
		return androidRegistry.impls[level], true
	}
	var selected Implementation
	for _, level := range levels {
		if level > targetSDK {
			break
		}
		selected = androidRegistry.impls[level]
	}
	if selected == nil {
		return nil, false
	}
	return selected, true
}

// HasImplementation reports whether an implementation for the given API level is registered.
func HasImplementation(level int) bool {
	androidRegistry.mu.RLock()
	defer androidRegistry.mu.RUnlock()
	_, ok := androidRegistry.impls[level]
	return ok
}
