package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

// fixedPane is a minimal test facet that measures to a fixed size — a fixture
// for exercising the split host's N-pane behaviour without the shell.
type fixedPane struct {
	facet.Facet
	layout facet.LayoutRole
	size   gfx.Size
}

func newFixedPane(w, h float32) *fixedPane {
	p := &fixedPane{size: gfx.Size{W: w, H: h}}
	p.layout.OnMeasure = func(_ facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		size := p.size
		return facet.MeasureResult{Size: size}
	}
	p.layout.OnArrange = func(_ facet.ArrangeContext, bounds gfx.Rect) {
		p.layout.ArrangedBounds = bounds
	}
	p.Facet = facet.NewFacet()
	p.AddRole(&p.layout)
	return p
}

func (p *fixedPane) Base() *facet.Facet             { p.BindImpl(p); return &p.Facet }
func (p *fixedPane) OnAttach(_ facet.AttachContext) {}
func (p *fixedPane) OnDetach()                      {}
func (p *fixedPane) OnActivate()                    {}
func (p *fixedPane) OnDeactivate()                  {}

func paneRect(t *testing.T, p Pane) gfx.Rect {
	t.Helper()
	return p.Facet.Base().LayoutRole().ArrangedBounds
}

func runSplit(t *testing.T, panes []Pane) []gfx.Rect {
	t.Helper()
	split := NewGallerySplit(panes, dividerSize)
	h := testkit.NewStandardHarness(t, 1280, 600, split)
	h.RunFrame()
	out := make([]gfx.Rect, len(panes))
	for i := range panes {
		out[i] = paneRect(t, panes[i])
	}
	return out
}

// TestGallerySplit_nPaneProperty asserts the split host's invariants for an
// arbitrary number of panes: dividers total DividerSize*(K-1), the flex pane
// absorbs the residual, and fixed panes honour their declared width.
func TestGallerySplit_nPaneProperty(t *testing.T) {
	const availW float32 = 1280

	t.Run("three panes with flex middle", func(t *testing.T) {
		panes := []Pane{
			{Facet: newFixedPane(200, 100), FixedWidth: 200, MinWidth: 200},
			{Facet: newFixedPane(100, 100), Flex: 1, MinWidth: 100},
			{Facet: newFixedPane(250, 100), FixedWidth: 250, MinWidth: 250},
		}
		rects := runSplit(t, panes)

		// Dividers total = DividerSize * (K-1) = 2 gutters.
		totalGutters := (rects[1].Min.X - rects[0].Max.X) + (rects[2].Min.X - rects[1].Max.X)
		if totalGutters != 2*dividerSize {
			t.Fatalf("divider total = %v, want %d", totalGutters, 2*dividerSize)
		}
		// Fixed panes honour their declared width; flex pane absorbs residual.
		if rects[0].Width() != 200 || rects[2].Width() != 250 {
			t.Fatalf("fixed widths = %v,%v, want 200,250", rects[0].Width(), rects[2].Width())
		}
		residual := availW - 200 - 250 - 2*dividerSize
		if rects[1].Width() != residual {
			t.Fatalf("flex pane width = %v, want residual %v", rects[1].Width(), residual)
		}
		// All panes share the split's height.
		if rects[0].Height() != rects[1].Height() || rects[1].Height() != rects[2].Height() {
			t.Fatalf("pane heights = %v,%v,%v, want equal", rects[0].Height(), rects[1].Height(), rects[2].Height())
		}
		if rects[0].Height() != 600 {
			t.Fatalf("pane height = %v, want split height 600 (cross fill)", rects[0].Height())
		}
	})

	t.Run("two panes", func(t *testing.T) {
		panes := []Pane{
			{Facet: newFixedPane(300, 100), FixedWidth: 300, MinWidth: 300},
			{Facet: newFixedPane(100, 100), Flex: 1, MinWidth: 100},
		}
		rects := runSplit(t, panes)
		if d := rects[1].Min.X - rects[0].Max.X; d != dividerSize {
			t.Fatalf("divider = %v, want %d", d, dividerSize)
		}
		if got := rects[1].Width(); got != availW-300-dividerSize {
			t.Fatalf("flex width = %v, want %v", got, availW-300-dividerSize)
		}
	})

	t.Run("four panes one flex", func(t *testing.T) {
		panes := []Pane{
			{Facet: newFixedPane(100, 100), FixedWidth: 100, MinWidth: 100},
			{Facet: newFixedPane(100, 100), FixedWidth: 100, MinWidth: 100},
			{Facet: newFixedPane(100, 100), Flex: 1, MinWidth: 100},
			{Facet: newFixedPane(100, 100), FixedWidth: 100, MinWidth: 100},
		}
		rects := runSplit(t, panes)
		var gutters float32
		for i := 0; i+1 < len(rects); i++ {
			gutters += rects[i+1].Min.X - rects[i].Max.X
		}
		if gutters != 3*dividerSize {
			t.Fatalf("divider total = %v, want %d (3 gutters)", gutters, 3*dividerSize)
		}
		residual := availW - 300 - 3*dividerSize
		if rects[2].Width() != residual {
			t.Fatalf("flex width = %v, want %v", rects[2].Width(), residual)
		}
	})

	t.Run("flex honours min width", func(t *testing.T) {
		// The flex pane's min (400) exceeds the residual after two wide fixed
		// panes; it must not shrink below the min.
		panes := []Pane{
			{Facet: newFixedPane(500, 100), FixedWidth: 500, MinWidth: 500},
			{Facet: newFixedPane(100, 100), Flex: 1, MinWidth: 400},
			{Facet: newFixedPane(500, 100), FixedWidth: 500, MinWidth: 500},
		}
		rects := runSplit(t, panes)
		if rects[1].Width() != 400 {
			t.Fatalf("flex pane width = %v, want min 400", rects[1].Width())
		}
		// Fixed panes keep their width even when the flex pane is at its min.
		if rects[0].Width() != 500 || rects[2].Width() != 500 {
			t.Fatalf("fixed widths = %v,%v, want 500,500", rects[0].Width(), rects[2].Width())
		}
	})
}

// TestGallerySplit_handlesAreFreshPerPass guards the ChildArrangeHandle
// contract: arranging the split twice must not trip the "called twice" panic
// (fresh handles per arrange pass).
func TestGallerySplit_handlesAreFreshPerPass(t *testing.T) {
	panes := []Pane{
		{Facet: newFixedPane(200, 100), FixedWidth: 200, MinWidth: 200},
		{Facet: newFixedPane(100, 100), Flex: 1, MinWidth: 100},
		{Facet: newFixedPane(240, 100), FixedWidth: 240, MinWidth: 240},
	}
	split := NewGallerySplit(panes, dividerSize)
	h := testkit.NewStandardHarness(t, 1280, 600, split)
	h.RunFrame()
	h.RunFrame() // a second frame re-arranges without panic
	if got := paneRect(t, panes[1]).Width(); got == 0 {
		t.Fatal("flex pane lost its arranged bounds across frames")
	}
}
