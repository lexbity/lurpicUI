// Package readstore implements the LDP store-addressing model (QP-8): a
// named registry for app-domain stores plus a fail-closed structural walk
// over a facet's exported store fields. The runtime executes reads and
// writes through this package on the runtime thread, at a cut.
package readstore

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"codeburg.org/lexbit/lurpicui/store"
)

// NamedStore binds an app-domain store to a stable name (QP-8 path (b) —
// the preferred address for app-domain stores). Store must be
// *store.ValueStore[T], *store.CollectionStore[T], or *store.Derived[T].
type NamedStore struct {
	Name  string
	Store any
}

// Registry is the named-store surface. Runtime-thread owned: registration
// happens at startup (or before the first frame via Runtime.RegisterNamedStore),
// reads and writes happen at cuts on the runtime thread.
type Registry struct {
	byName map[string]any
}

// NewRegistry validates and indexes the startup set (fail-loud: a name
// collision or a non-store value is a startup error, never a silent skip).
func NewRegistry(named []NamedStore) (*Registry, error) {
	r := &Registry{byName: make(map[string]any, len(named))}
	for _, ns := range named {
		if ns.Name == "" {
			return nil, fmt.Errorf("drive: named store with empty name (%T)", ns.Store)
		}
		if err := ValidateStore(ns.Store); err != nil {
			return nil, fmt.Errorf("drive: named store %q: %w", ns.Name, err)
		}
		if _, exists := r.byName[ns.Name]; exists {
			return nil, fmt.Errorf("drive: named store %q registered twice", ns.Name)
		}
		r.byName[ns.Name] = ns.Store
	}
	return r, nil
}

// Register adds one store after startup (runtime thread, before frames).
// Re-registering an existing name replaces it — the freeze tests make
// registry churn a reviewable diff, so the replacement is honest.
func (r *Registry) Register(name string, s any) error {
	if name == "" {
		return fmt.Errorf("drive: named store with empty name (%T)", s)
	}
	if err := ValidateStore(s); err != nil {
		return fmt.Errorf("drive: named store %q: %w", name, err)
	}
	r.byName[name] = s
	return nil
}

// Names returns the sorted registered names.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.byName))
	for name := range r.byName {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Lookup returns the store registered under name.
func (r *Registry) Lookup(name string) (any, bool) {
	s, ok := r.byName[name]
	return s, ok
}

// ValidateStore accepts exactly the three addressable store shapes
// (fail-closed: anything else is an error naming the actual type).
func ValidateStore(s any) error {
	if s == nil {
		return fmt.Errorf("nil store")
	}
	switch s.(type) {
	case *store.ValueStore[any]:
		return nil
	}
	t := reflect.TypeOf(s)
	if t.Kind() != reflect.Pointer {
		return fmt.Errorf("%T is not a store pointer", s)
	}
	// Instantiated generics carry type parameters in their name
	// ("ValueStore[float64]"); match on the bare name.
	switch strings.SplitN(t.Elem().Name(), "[", 2)[0] {
	case "ValueStore", "CollectionStore", "Derived":
		return nil
	default:
		return fmt.Errorf("%T is not *store.ValueStore, *store.CollectionStore, or *store.Derived", s)
	}
}

// Kind classifies a store value for store.list output.
func Kind(s any) string {
	if s == nil {
		return ""
	}
	t := reflect.TypeOf(s)
	if t.Kind() != reflect.Pointer {
		return ""
	}
	switch strings.SplitN(t.Elem().Name(), "[", 2)[0] {
	case "ValueStore":
		return "value"
	case "CollectionStore":
		return "collection"
	case "Derived":
		return "derived"
	default:
		return ""
	}
}

// ReadValue returns a store's current value as JSON: ValueStore.Get,
// CollectionStore.All (as an array), or Derived.Peek's last computed value.
// The reflection surface is bounded to these method names — no interface{}
// escape hatch beyond the JSON encoding itself.
func ReadValue(s any) (json.RawMessage, error) {
	if s == nil {
		return nil, fmt.Errorf("drive: nil store")
	}
	v := reflect.ValueOf(s)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return nil, fmt.Errorf("drive: %T is not a non-nil store pointer", s)
	}
	switch Kind(s) {
	case "collection":
		all := v.MethodByName("All").Call(nil)[0]
		return marshalReflect(all)
	default:
		get := v.MethodByName("Get")
		if !get.IsValid() {
			return nil, fmt.Errorf("drive: %T has no Get — not a readable store", s)
		}
		return marshalReflect(get.Call(nil)[0])
	}
}

// ReadDerivedPeek returns a Derived's last computed value and dirty flag
// without recomputing (QP-9). A non-derived store is a typed error —
// derived.read on a value store would silently duplicate store.read.
func ReadDerivedPeek(s any) (json.RawMessage, bool, error) {
	if s == nil {
		return nil, false, fmt.Errorf("drive: nil store")
	}
	if Kind(s) != "derived" {
		return nil, false, fmt.Errorf("drive: %T is not a *store.Derived (derived.read is peek-only)", s)
	}
	v := reflect.ValueOf(s)
	peek := v.MethodByName("Peek")
	if !peek.IsValid() {
		return nil, false, fmt.Errorf("drive: %T has no Peek", s)
	}
	out := peek.Call(nil)
	value, err := marshalReflect(out[0])
	if err != nil {
		return nil, false, err
	}
	return value, out[1].Bool(), nil
}

// WriteValue stores a decoded JSON value into a ValueStore. Collection
// writes are Phase 5 (refused here with a typed error); Derived is
// read-only by construction.
func WriteValue(s any, payload json.RawMessage) error {
	switch Kind(s) {
	case "value":
	case "collection":
		return fmt.Errorf("drive: collection writes land in Phase 5; store.write refuses collections for now")
	default:
		return fmt.Errorf("drive: %T is not a writable *store.ValueStore", s)
	}
	v := reflect.ValueOf(s)
	set := v.MethodByName("Set")
	if !set.IsValid() {
		return fmt.Errorf("drive: %T has no Set — not a writable store", s)
	}
	paramType := set.Type().In(0)
	target := reflect.New(paramType)
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, target.Interface()); err != nil {
			return fmt.Errorf("drive: store.write value does not decode to %s: %w", paramType, err)
		}
	}
	set.Call([]reflect.Value{target.Elem()})
	return nil
}

// StoreField is one structural addressing hit: an exported field of a
// facet's implementation that holds an addressable store.
type StoreField struct {
	Name string
	Kind string
}

// StructFields enumerates a facet implementation's addressable store fields
// (fail-closed reflection: exported fields only, only the three store
// shapes, panicking types are skipped — they cannot be addressed).
func StructFields(impl any) []StoreField {
	if impl == nil {
		return nil
	}
	t := reflect.TypeOf(impl)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	out := make([]StoreField, 0, 4)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue // unexported
		}
		if kind := Kind(reflect.Zero(f.Type).Interface()); kind != "" {
			// reflect.Zero of a store pointer type: Kind only needs the
			// type, and a typed nil is fine for classification.
			out = append(out, StoreField{Name: f.Name, Kind: kind})
		}
	}
	return out
}

// FieldStore resolves a facet implementation's named store field
// (fail-closed: unknown or unaddressable fields are errors naming the
// field's actual type, never a zero value).
func FieldStore(impl any, field string) (any, error) {
	if impl == nil {
		return nil, fmt.Errorf("drive: nil facet implementation")
	}
	v := reflect.ValueOf(impl)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil, fmt.Errorf("drive: nil facet implementation")
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("drive: facet implementation %T is not a struct", impl)
	}
	f := v.FieldByName(field)
	if !f.IsValid() {
		return nil, fmt.Errorf("drive: facet %T has no field %q", impl, field)
	}
	if !f.CanInterface() {
		return nil, fmt.Errorf("drive: facet %T field %q is unexported", impl, field)
	}
	s := f.Interface()
	if err := ValidateStore(s); err != nil {
		return nil, fmt.Errorf("drive: facet %T field %q: %w", impl, field, err)
	}
	return s, nil
}

func marshalReflect(v reflect.Value) (json.RawMessage, error) {
	payload, err := json.Marshal(v.Interface())
	if err != nil {
		return nil, fmt.Errorf("drive: store value %s is not JSON-encodable: %w", v.Type(), err)
	}
	return payload, nil
}
