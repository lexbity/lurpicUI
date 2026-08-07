package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

// block is a test fixture facet that renders a solid color — the minimal
// exhibit content the stage tests switch between.
type block struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	color  gfx.Color
}

func newBlock(color gfx.Color) *block {
	b := &block{color: color}
	b.layout.OnMeasure = func(_ facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: c.MaxSize}
	}
	b.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(b.color)})
	}
	b.Facet = facet.NewFacet()
	b.AddRole(&b.layout)
	b.AddRole(&b.render)
	return b
}

func (b *block) Base() *facet.Facet             { b.BindImpl(b); return &b.Facet }
func (b *block) OnAttach(_ facet.AttachContext) {}
func (b *block) OnDetach()                      {}
func (b *block) OnActivate()                    {}
func (b *block) OnDeactivate()                  {}

// testExhibit is a test fixture implementing Exhibit over a block.
type testExhibit struct {
	id    ExhibitID
	title string
	block *block
}

func (t testExhibit) ID() ExhibitID                         { return t.id }
func (t testExhibit) Title() string                         { return t.title }
func (t testExhibit) Build(*state.AppState) facet.FacetImpl { return t.block }

func blockBounds(f facet.FacetImpl) gfx.Rect {
	return f.Base().LayoutRole().ArrangedBounds
}

func TestStage_switchesActiveExhibit(t *testing.T) {
	appState := state.NewAppState(nil)
	a := &testExhibit{id: ExhibitPolicies, title: "Policies", block: newBlock(gfx.Color{R: 1, A: 1})}
	b := &testExhibit{id: ExhibitLayers, title: "Layers", block: newBlock(gfx.Color{B: 1, A: 1})}
	stage := NewStage([]Exhibit{a, b}, appState)
	h := testkit.NewStandardHarness(t, 400, 300, stage)
	h.RunFrame()

	if got := stage.ActiveExhibit().Get(); got != ExhibitPolicies {
		t.Fatalf("default active = %q, want %q", got, ExhibitPolicies)
	}
	if got := blockBounds(a.block); got.Min != (gfx.Point{}) || got.Max.X != 400 || got.Max.Y != 300 {
		t.Fatalf("active exhibit bounds = %v, want 400x300", got)
	}
	if got := blockBounds(b.block); !got.IsEmpty() {
		t.Fatalf("inactive exhibit bounds = %v, want empty", got)
	}

	// Switch: the new active is arranged; the old one goes to zero.
	stage.ActiveExhibit().Set(ExhibitLayers)
	h.RunFrame()
	if got := blockBounds(b.block); got.Min != (gfx.Point{}) || got.Max.X != 400 || got.Max.Y != 300 {
		t.Fatalf("switched-in exhibit bounds = %v, want 400x300", got)
	}
	if got := blockBounds(a.block); !got.IsEmpty() {
		t.Fatalf("switched-out exhibit bounds = %v, want empty", got)
	}
}

// TestStage_measuresOnlyActive asserts the visibility-gating contract
// (F-stage): the stage measures only the active exhibit, so an inactive
// exhibit keeps a zero measured size — the mechanism that avoids per-frame
// layout work for every exhibit.
func TestStage_measuresOnlyActive(t *testing.T) {
	appState := state.NewAppState(nil)
	a := &testExhibit{id: ExhibitPolicies, title: "Policies", block: newBlock(gfx.Color{R: 1, A: 1})}
	b := &testExhibit{id: ExhibitLayers, title: "Layers", block: newBlock(gfx.Color{B: 1, A: 1})}
	stage := NewStage([]Exhibit{a, b}, appState)
	h := testkit.NewStandardHarness(t, 400, 300, stage)
	h.RunFrame()

	if got := a.block.Base().LayoutRole().MeasuredSize; got == (gfx.Size{}) {
		t.Fatal("active exhibit was not measured")
	}
	if got := b.block.Base().LayoutRole().MeasuredSize; got != (gfx.Size{}) {
		t.Fatalf("inactive exhibit measured size = %v, want zero (never measured)", got)
	}

	// After a switch + frame, the new active is measured and the switched-out
	// exhibit is NOT re-measured (its measured size is unchanged — the stage
	// measures only the active, so a switch cannot cause a layout storm over
	// the whole catalog) and is arranged to zero.
	aMeasured := a.block.Base().LayoutRole().MeasuredSize
	stage.ActiveExhibit().Set(ExhibitLayers)
	h.RunFrame()
	if got := b.block.Base().LayoutRole().MeasuredSize; got == (gfx.Size{}) {
		t.Fatal("switched-in exhibit was not measured")
	}
	if got := a.block.Base().LayoutRole().MeasuredSize; got != aMeasured {
		t.Fatalf("switched-out exhibit was re-measured (storm): %v -> %v", aMeasured, got)
	}
	if got := blockBounds(a.block); !got.IsEmpty() {
		t.Fatalf("switched-out exhibit arranged bounds = %v, want zero", got)
	}
}

// TestStage_noLayoutStorm asserts a switch settles in one frame without
// disturbing the inactive exhibit (a dirty-layout storm would re-measure every
// exhibit).
func TestStage_noLayoutStorm(t *testing.T) {
	appState := state.NewAppState(nil)
	a := &testExhibit{id: ExhibitPolicies, title: "Policies", block: newBlock(gfx.Color{R: 1, A: 1})}
	b := &testExhibit{id: ExhibitLayers, title: "Layers", block: newBlock(gfx.Color{B: 1, A: 1})}
	stage := NewStage([]Exhibit{a, b}, appState)
	h := testkit.NewStandardHarness(t, 400, 300, stage)
	h.RunFrame()

	stage.ActiveExhibit().Set(ExhibitLayers)
	h.RunFrame() // the switch's invalidation is delivered + laid out this frame
	if got := blockBounds(b.block); got.IsEmpty() {
		t.Fatalf("switched-in exhibit not arranged after one frame: %v", got)
	}
	// A second idle frame must not change anything (no storm).
	before := blockBounds(b.block)
	h.RunFrame()
	if got := blockBounds(b.block); got != before {
		t.Fatalf("idle frame changed the arrangement: %v -> %v", before, got)
	}
}
