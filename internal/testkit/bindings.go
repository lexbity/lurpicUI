package testkit

import (
	"reflect"
	"sync"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
)

// bindingSources maps one binding element type to a minter that builds a
// dynamic binding over a fresh ValueStore of that type, plus a write
// function that emits through the store. Registration is separate (package
// internal/bindingsources) so testkit itself imports only the marks root
// package: marks subpackage tests use the walker, and testkit must not
// import the packages under test (import cycle in test).
var (
	bindingSourcesMu sync.RWMutex
	bindingSources   = map[reflect.Type]func(facet.DirtyFlags) (binding any, write func()){}
)

// RegisterBindingSource registers a minter for one concrete binding element
// type. The walker fails closed on an uncovered type — a mark that grows a
// new binding element type needs a new registration in
// internal/bindingsources, and the conformance walk surfaces the gap.
func RegisterBindingSource[T any](minter func(facet.DirtyFlags) (binding any, write func())) {
	var zero T
	bindingSourcesMu.Lock()
	defer bindingSourcesMu.Unlock()
	bindingSources[reflect.TypeOf(zero)] = minter
}

func bindingSourceFor(t reflect.Type) func(facet.DirtyFlags) (any, func()) {
	bindingSourcesMu.RLock()
	defer bindingSourcesMu.RUnlock()
	return bindingSources[t]
}

// bindingLike is the structural view of marks.Binding[T] the walker needs
// (marks.bindingSubscriber is unexported).
type bindingLike interface {
	IsDynamic() bool
	SubscribeOnChange(func()) func()
}

// bindingFields returns the index paths of the mark's exported binding
// fields: top level plus one level of anonymous embedded structs, excluding
// the embedded Core — the same rule as the declared-binding walk in
// marks/base.go.
func bindingFields(t reflect.Type) [][]int {
	var walk func(structType reflect.Type, prefix []int) [][]int
	walk = func(structType reflect.Type, prefix []int) [][]int {
		var out [][]int
		for i := 0; i < structType.NumField(); i++ {
			f := structType.Field(i)
			path := append(append([]int(nil), prefix...), i)
			if tag, ok := f.Tag.Lookup("binding"); ok && tag == "-" {
				continue
			}
			if f.Anonymous {
				ft := f.Type
				for ft.Kind() == reflect.Pointer {
					ft = ft.Elem()
				}
				if ft.Name() == "Core" {
					continue
				}
				if len(prefix) == 0 && f.PkgPath == "" {
					out = append(out, walk(ft, path)...)
					continue
				}
			}
			if f.PkgPath != "" {
				continue
			}
			if f.Type.Kind() == reflect.Struct && implementsBinding(f.Type) {
				out = append(out, path)
			}
		}
		return out
	}
	return walk(t, nil)
}

func implementsBinding(t reflect.Type) bool {
	_, ok := reflect.New(t).Interface().(bindingLike)
	return ok
}

// AssertBindingsAlive proves the declared-bindings contract (RX-2 FR-1) for
// one mark type: for every exported binding field, a dynamic binding swapped
// in before attach is subscribed at attach — a store write after attach
// invalidates the facet within the frame — and the subscription is cleaned up
// on dispose. The mark factory must build a fresh instance per call.
func AssertBindingsAlive(t *testing.T, mk func() facet.FacetImpl) {
	t.Helper()
	probe := mk()
	fields := bindingFields(reflect.TypeOf(probe).Elem())
	for _, path := range fields {
		assertBindingAlive(t, mk, path)
	}
}

func assertBindingAlive(t *testing.T, mk func() facet.FacetImpl, path []int) {
	t.Helper()
	m := mk()
	target := reflect.ValueOf(m).Elem()
	for _, idx := range path {
		target = target.Field(idx)
	}
	// Recover the binding's element type T from the marks.Binding[T] struct's
	// `val` field (type access needs no export) — the registry is keyed by
	// element type.
	valField, ok := target.Type().FieldByName("val")
	if !ok {
		t.Fatalf("marks.Binding shape changed: no val field on %s", target.Type())
	}
	mint := bindingSourceFor(valField.Type)
	if mint == nil {
		t.Fatalf("no binding source registered for element type %s (register it via testkit.RegisterBindingSource in internal/bindingsources)", valField.Type)
	}
	binding, write := mint(facet.DirtyProjection)

	// Swap the field to a dynamic binding before attach, exactly as an
	// author would.
	target.Set(reflect.ValueOf(binding).Convert(target.Type()))

	facet.Attach(m, facet.AttachContext{})

	// Marks may legitimately invalidate during attach (child sync, initial
	// projection state). The contract under test starts from a clean facet.
	m.Base().ClearDirty(facet.DirtyAll)
	if flags := m.Base().DirtyFlags(); flags != 0 {
		t.Fatalf("%T: facet dirty not clearable after attach (path %v)", m, path)
	}

	write()
	if flags := m.Base().DirtyFlags(); flags&facet.DirtyProjection == 0 {
		t.Fatalf("%T: declared binding field at path %v is dead — a store write after attach did not invalidate the facet (RX-2 FR-1)", m, path)
	}

	// Detach-managed: after dispose the subscription must be gone.
	m.Base().ClearDirty(facet.DirtyAll)
	facet.Dispose(m)
	write()
	if flags := m.Base().DirtyFlags(); flags != 0 {
		t.Fatalf("%T: binding field at path %v still subscribed after dispose (RX-2 FR-4)", m, path)
	}
}
