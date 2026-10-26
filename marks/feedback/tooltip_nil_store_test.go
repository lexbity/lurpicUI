package feedback

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/store"
)

// TestTooltip_nilStoreConstruction pins the RX-2 Q2 doctrine for the
// tooltip: a nil open store means the mark owns its open state — before the
// value contract a nil store disabled the subscription AND made every
// Open.Get() render path dereference nil.
func TestTooltip_nilStoreConstruction(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	tip := NewTooltip(scope, "hint", nil)
	if tip.Store() == nil {
		t.Fatal("nil open did not create an internal store")
	}
	if tip.Open.Get() {
		t.Fatal("internal open store must start closed")
	}
	tip.Show()
	if !tip.Store().Get() {
		t.Fatal("Show did not write the internal store")
	}
	tip.Hide()
	if tip.Store().Get() {
		t.Fatal("Hide did not write the internal store")
	}
}

// TestTooltip_injectedStoreBoundNotCopied pins the injection half: an
// injected open store is the live truth, and Show/Hide write through it.
func TestTooltip_injectedStoreBoundNotCopied(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	open := store.NewValueStore(scope, false)
	tip := NewTooltip(scope, "hint", open)
	if tip.Store() != open {
		t.Fatal("Store() must return the injected store itself")
	}
	tip.Show()
	if !open.Get() {
		t.Fatal("Show did not write the injected store")
	}
	open.Set(false)
	if tip.Open.Get() {
		t.Fatal("external close not visible through the mark")
	}
}
