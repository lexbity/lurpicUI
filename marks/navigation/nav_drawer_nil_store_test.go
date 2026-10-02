package navigation

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/store"
)

var drawerSections = []NavDrawerSection{
	{Label: "Enterprise", Items: []NavDrawerItem{
		{Key: "sources", Label: "Sources"},
		{Key: "hub", Label: "Hub"},
	}},
}

// TestNavDrawer_nilStoreConstruction pins the RX-2 Q2 doctrine for the
// drawer: nil open/currentIndex stores mean the mark owns both — before the
// value contract a nil open store made every Open.Get() dereference nil.
func TestNavDrawer_nilStoreConstruction(t *testing.T) {
	d := NewNavDrawer("Sources", drawerSections, nil, nil)
	if d.OpenStore() == nil {
		t.Fatal("nil open did not create an internal store")
	}
	if d.CurrentIndexStore() == nil {
		t.Fatal("nil currentIndex did not create an internal store")
	}
	if d.Open.Get() {
		t.Fatal("internal open store must start closed")
	}
	d.SetOpen(true)
	if !d.OpenStore().Get() {
		t.Fatal("SetOpen did not write the internal store")
	}
	d.SetOpen(false)
	if d.OpenStore().Get() {
		t.Fatal("SetOpen(false) did not write the internal store")
	}
}

// TestNavDrawer_injectedStoresBoundNotCopied pins the injection half: an
// injected open store is the live truth in both directions.
func TestNavDrawer_injectedStoresBoundNotCopied(t *testing.T) {
	open := store.NewValueStore(false)
	current := store.NewValueStore(0)
	d := NewNavDrawer("Sources", drawerSections, open, current)
	if d.OpenStore() != open || d.CurrentIndexStore() != current {
		t.Fatal("Store accessors must return the injected stores themselves")
	}
	d.SetOpen(true)
	if !open.Get() {
		t.Fatal("SetOpen did not write the injected store")
	}
	open.Set(false)
	if d.Open.Get() {
		t.Fatal("external close not visible through the mark")
	}
}
