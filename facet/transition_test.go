package facet

import (
	"testing"
)

func TestTransition_activate_before_attach_panics(t *testing.T) {
	f := newTestFacet()
	mustPanicContains(t, "invalid lifecycle transition", func() {
		Activate(f)
	})
}

func TestTransition_double_attach_panics(t *testing.T) {
	f := newTestFacet()
	Attach(f, AttachContext{})
	mustPanicContains(t, "invalid lifecycle transition", func() {
		Attach(f, AttachContext{})
	})
}

func TestTransition_deactivate_before_activate_panics(t *testing.T) {
	f := newTestFacet()
	Attach(f, AttachContext{})
	mustPanicContains(t, "invalid lifecycle transition", func() {
		Deactivate(f)
	})
}

// Dispose permits direct transition from any attachable state, so
// there is no panic for disposing an active facet.

func TestTransition_attach_after_dispose_panics(t *testing.T) {
	f := newTestFacet()
	Attach(f, AttachContext{})
	Activate(f)
	Deactivate(f)
	Dispose(f)
	mustPanicContains(t, "invalid lifecycle transition", func() {
		Attach(f, AttachContext{})
	})
}

func TestTransition_nil_impl_panics(t *testing.T) {
	mustPanicContains(t, "nil FacetImpl", func() {
		Attach(nil, AttachContext{})
	})
}

func TestTransition_dispose_already_disposed_panics(t *testing.T) {
	f := newTestFacet()
	Attach(f, AttachContext{})
	Activate(f)
	Deactivate(f)
	Dispose(f)
	mustPanicContains(t, "invalid lifecycle transition", func() {
		Dispose(f)
	})
}

// nilBaseFacet returns nil from Base() to trigger the "nil Base" panic.
type nilBaseFacet struct {
	Facet
}

func (f *nilBaseFacet) Base() *Facet               { return nil }
func (f *nilBaseFacet) OnAttach(ctx AttachContext) {}
func (f *nilBaseFacet) OnDetach()                  {}
func (f *nilBaseFacet) OnActivate()                {}
func (f *nilBaseFacet) OnDeactivate()              {}

func TestTransition_impl_nil_base_panics(t *testing.T) {
	mustPanicContains(t, "nil Base", func() {
		Attach(&nilBaseFacet{}, AttachContext{})
	})
}

func TestTransition_require_state_panics(t *testing.T) {
	f := newTestFacet()
	// requireState is called internally by Attach. After Dispose,
	// calling Activate triggers requireState(StateActive) which fails.
	Attach(f, AttachContext{})
	Activate(f)
	Deactivate(f)
	Dispose(f)
	mustPanicContains(t, "invalid lifecycle transition", func() {
		Activate(f)
	})
}
