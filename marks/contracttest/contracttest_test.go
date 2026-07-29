package contracttest

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/store"
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
