// Package readstore owns the transport-independent registry for explicit,
// immutable store read capabilities. It contains no reflection or mutation
// dispatch; declarations are typed by store.ReadExport constructors.
package readstore

import (
	"fmt"
	"sort"
	"sync"

	"codeburg.org/lexbit/lurpicui/store"
)

// Registry is the frozen named-export catalog for one app instance.
type Registry struct {
	mu     sync.RWMutex
	byName map[string]store.ReadExport
	frozen bool
}

// NewRegistry validates declarations and rejects duplicate names before the
// app publishes its protocol contract.
func NewRegistry(exports []store.ReadExport) (*Registry, error) {
	r := &Registry{byName: make(map[string]store.ReadExport, len(exports))}
	for _, export := range exports {
		info := export.Info()
		if !export.Valid() || info.Name == "" || info.Field != "" {
			return nil, fmt.Errorf("drive: invalid named read export %q", info.Name)
		}
		if _, exists := r.byName[info.Name]; exists {
			return nil, fmt.Errorf("drive: read export %q registered twice", info.Name)
		}
		r.byName[info.Name] = export
	}
	return r, nil
}

// Names returns public names in deterministic order.
func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byName))
	for name := range r.byName {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Lookup returns only the declared immutable capture capability.
func (r *Registry) Lookup(name string) (store.ReadExport, bool) {
	if r == nil {
		return store.ReadExport{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	export, ok := r.byName[name]
	return export, ok
}

// Register is only available before Freeze. Runtime construction freezes the
// catalog before listener startup; connected sessions never see mutation.
func (r *Registry) Register(export store.ReadExport) error {
	if r == nil {
		return fmt.Errorf("drive: nil read export registry")
	}
	info := export.Info()
	if !export.Valid() || info.Name == "" || info.Field != "" {
		return fmt.Errorf("drive: invalid named read export %q", info.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return fmt.Errorf("drive: read export catalog is frozen")
	}
	if _, exists := r.byName[info.Name]; exists {
		return fmt.Errorf("drive: read export %q registered twice", info.Name)
	}
	r.byName[info.Name] = export
	return nil
}

// Freeze closes registration and is idempotent.
func (r *Registry) Freeze() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.frozen = true
	r.mu.Unlock()
}
