package contracttest

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

// badValueMark simulates the anti-pattern: the mark owns its own ValueStore
// and ignores the caller's store — the canonical flaw 2.
type badValueMark struct {
	facet.Facet
	Value *store.ValueStore[int]
}

func (m *badValueMark) Base() *facet.Facet             { return &m.Facet }
func (m *badValueMark) OnAttach(_ facet.AttachContext) {}
func (m *badValueMark) OnDetach()                      {}
func (m *badValueMark) OnActivate()                    {}
func (m *badValueMark) OnDeactivate()                  {}

// goodValueMark uses the caller's store correctly — the reference pattern.
type goodValueMark struct {
	facet.Facet
	Value *store.ValueStore[int]
}

func (m *goodValueMark) Base() *facet.Facet             { return &m.Facet }
func (m *goodValueMark) OnAttach(_ facet.AttachContext) {}
func (m *goodValueMark) OnDetach()                      {}
func (m *goodValueMark) OnActivate()                    {}
func (m *goodValueMark) OnDeactivate()                  {}

// failRecorder wraps *testing.T and captures assertion failures without
// terminating the goroutine (unlike t.Fatal/FailNow).
type failRecorder struct {
	*testing.T
	failed bool
}

func (r *failRecorder) Fatalf(format string, args ...any) {
	r.failed = true
	r.T.Logf("expected assertion failure: "+format, args...)
}

func TestAssertValueSurvivesDispose_DetectsBadMark(t *testing.T) {
	rec := &failRecorder{T: t}
	AssertValueSurvivesDispose[int](
		rec,
		func() *store.ValueStore[int] { return store.NewValueStore[int](0) },
		func(s *store.ValueStore[int]) facet.FacetImpl {
			m := &badValueMark{Value: store.NewValueStore[int](0)}
			m.Facet = facet.NewFacet()
			return m
		},
		func(m facet.FacetImpl) {
			m.(*badValueMark).Value.Set(42)
		},
	)
	if !rec.failed {
		t.Fatal("AssertValueSurvivesDispose should have detected a mark that ignores the caller's store")
	}
}

// badDataMark simulates the anti-pattern: the mark owns its own CollectionStore
// and ignores the caller's store, returning its own from BoundData().
type badDataMark struct {
	facet.Facet
	DataStore *store.CollectionStore[int]
}

func (m *badDataMark) Base() *facet.Facet             { return &m.Facet }
func (m *badDataMark) BoundData() any                 { return m.DataStore }
func (m *badDataMark) OnAttach(_ facet.AttachContext) {}
func (m *badDataMark) OnDetach()                      {}
func (m *badDataMark) OnActivate()                    {}
func (m *badDataMark) OnDeactivate()                  {}

func TestAssertDataBound_DetectsBadMark(t *testing.T) {
	rec := &failRecorder{T: t}
	ident := func(i int) store.ItemID { return store.ItemID(i) }
	AssertDataBound[int](
		rec,
		func() *store.CollectionStore[int] { return store.NewCollectionStore(ident) },
		func(s *store.CollectionStore[int]) facet.FacetImpl {
			m := &badDataMark{DataStore: store.NewCollectionStore(ident)}
			m.Facet = facet.NewFacet()
			return m
		},
		func(s *store.CollectionStore[int]) { s.Insert(1) },
	)
	if !rec.failed {
		t.Fatal("AssertDataBound should have detected a mark that owns its own CollectionStore")
	}
}

// badAnchorMark simulates the anti-pattern: returns anchors outside the
// arranged bounds, which violates the in-bounds contract.
type badAnchorMark struct {
	facet.Facet
}

func (m *badAnchorMark) Base() *facet.Facet             { return &m.Facet }
func (m *badAnchorMark) OnAttach(_ facet.AttachContext) {}
func (m *badAnchorMark) OnDetach()                      {}
func (m *badAnchorMark) OnActivate()                    {}
func (m *badAnchorMark) OnDeactivate()                  {}

func (m *badAnchorMark) ExportAnchors(_ layout.AnchorExportContext) layout.AnchorSet {
	return layout.AnchorSet{
		"out_of_bounds": {X: 9999, Y: 9999},
	}
}

// badChildMark returns a GroupChild entry with a nil Layout field.
type badChildMark struct {
	facet.Facet
}

func (m *badChildMark) Base() *facet.Facet             { return &m.Facet }
func (m *badChildMark) OnAttach(_ facet.AttachContext) {}
func (m *badChildMark) OnDetach()                      {}
func (m *badChildMark) OnActivate()                    {}
func (m *badChildMark) OnDeactivate()                  {}

func (m *badChildMark) Children() []facet.GroupChild {
	return []facet.GroupChild{{FacetID: 1, Layout: nil}}
}

func TestAssertGroupChildren_DetectsBadMark_NilLayout(t *testing.T) {
	rec := &failRecorder{T: t}
	AssertGroupChildren(rec,
		func() facet.FacetImpl {
			return &badChildMark{Facet: facet.NewFacet()}
		},
		func(facet.FacetImpl) {},
	)
	if !rec.failed {
		t.Fatal("AssertGroupChildren should have detected a mark with nil Layout in Children()")
	}
}

func TestAssertAnchorExport_DetectsBadMark(t *testing.T) {
	rec := &failRecorder{T: t}
	bounds := gfx.RectFromXYWH(0, 0, 100, 100)
	AssertAnchorExport(rec,
		func() facet.FacetImpl {
			m := &badAnchorMark{Facet: facet.NewFacet()}
			return m
		},
		func(m facet.FacetImpl, ctx facet.AttachContext, b gfx.Rect) {},
		bounds,
		theme.ResolvedContext{},
	)
	if !rec.failed {
		t.Fatal("AssertAnchorExport should have detected a mark with anchors outside bounds")
	}
}

// badAccessibleMark returns an empty AccessibilityRole.
type badAccessibleMark struct {
	facet.Facet
}

func (m *badAccessibleMark) Base() *facet.Facet             { return &m.Facet }
func (m *badAccessibleMark) OnAttach(_ facet.AttachContext) {}
func (m *badAccessibleMark) OnDetach()                      {}
func (m *badAccessibleMark) OnActivate()                    {}
func (m *badAccessibleMark) OnDeactivate()                  {}

func (m *badAccessibleMark) AccessibilityRole() string { return "" }
func (m *badAccessibleMark) AccessibleName() string    { return "bad" }

// badFocusableMark returns Focusable() == true when disabled.
type badFocusableMark struct {
	facet.Facet
}

func (m *badFocusableMark) Base() *facet.Facet             { return &m.Facet }
func (m *badFocusableMark) OnAttach(_ facet.AttachContext) {}
func (m *badFocusableMark) OnDetach()                      {}
func (m *badFocusableMark) OnActivate()                    {}
func (m *badFocusableMark) OnDeactivate()                  {}

func (m *badFocusableMark) Focusable() bool { return true }

func TestAssertAccessible_DetectsBadMark(t *testing.T) {
	rec := &failRecorder{T: t}
	AssertAccessible(rec,
		func(label string) facet.FacetImpl {
			return &badAccessibleMark{Facet: facet.NewFacet()}
		},
		"",
	)
	if !rec.failed {
		t.Fatal("AssertAccessible should have detected a mark with empty AccessibilityRole")
	}
}

func TestAssertFocusable_DetectsBadMark(t *testing.T) {
	rec := &failRecorder{T: t}
	AssertFocusable(rec,
		func(disabled bool) facet.FacetImpl {
			return &badFocusableMark{Facet: facet.NewFacet()}
		},
	)
	if !rec.failed {
		t.Fatal("AssertFocusable should have detected a mark that reports Focusable()==true when disabled")
	}
}

func TestAssertValueSurvivesDispose_PassesOnGoodMark(t *testing.T) {
	AssertValueSurvivesDispose[int](
		t,
		func() *store.ValueStore[int] { return store.NewValueStore[int](0) },
		func(s *store.ValueStore[int]) facet.FacetImpl {
			m := &goodValueMark{Value: s}
			m.Facet = facet.NewFacet()
			return m
		},
		func(m facet.FacetImpl) {
			m.(*goodValueMark).Value.Set(42)
		},
	)
}
