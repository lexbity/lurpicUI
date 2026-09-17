package runtime

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// BenchmarkLayoutPass_StudioTree measures the runtime layout pass after a
// content change routed to a deep leaf, on a studio-sized tree (a few hundred
// leaves across a host hierarchy). NFR-3 guardrail: p50 ≤ 2ms, p95 ≤ 5ms at
// 1280x800. Run with `go test -bench BenchmarkLayoutPass_StudioTree -count=5
// -run '^$' ./runtime/` and inspect the median/p95 across runs.
func BenchmarkLayoutPass_StudioTree(b *testing.B) {
	root := buildBenchmarkStudioTree(10, 30) // 10 cards x 30 leaves = 300 leaves
	rt := mustRuntimeTree(b, root)
	rt.markTreeDirty(root, facet.DirtyLayout)
	rt.runLayoutPass(gfx.Size{W: 1280, H: 800})

	// Collect all leaves so each iteration routes a different deep facet.
	var leaves []*layoutCountLeaf
	collectLeaves(root, &leaves)
	if len(leaves) == 0 {
		b.Fatal("no leaves in benchmark tree")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		leaf := leaves[i%len(leaves)]
		rt.dirtyFacets = map[facet.FacetID]facet.DirtyFlags{leaf.Base().ID(): facet.DirtyLayout}
		start := time.Now()
		rt.runLayoutPass(gfx.Size{W: 1280, H: 800})
		elapsed := time.Since(start)
		b.SetBytes(int64(elapsed))
	}
}

// buildBenchmarkStudioTree builds a root group host with cards, each card
// hosting leaves — the studio's exhibit/card/mark shape.
func buildBenchmarkStudioTree(cards, leavesPerCard int) facet.FacetImpl {
	root := facet.NewFacet()
	root.BindImpl(&root)
	var rootLayout facet.LayoutRole
	rootLayout.Parent = facet.GroupParentContract{Kind: facet.GroupLayoutLinearHorizontal}
	rootLayout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: c.MaxSize}
	}
	rootLayout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		rootLayout.ArrangedBounds = bounds
	}
	rootLayout.Child = facet.GroupChildContract{SupportedPlacement: facet.SupportsGrid | facet.SupportsLinear}
	root.AddRole(&rootLayout)

	for i := 0; i < cards; i++ {
		card := newBenchCard(i, leavesPerCard)
		root.AddChildRuntime(card.Base())
	}
	return &root
}

func newBenchCard(index, leafCount int) *benchCard {
	c := &benchCard{Facet: facet.NewFacet(), index: index}
	c.layout = facet.LayoutRole{
		Parent: facet.GroupParentContract{Kind: facet.GroupLayoutLinearVertical},
		OnMeasure: func(ctx facet.MeasureContext, c2 facet.Constraints) facet.MeasureResult {
			for _, l := range c.leaves {
				if l != nil && l.Base().LayoutRole() != nil {
					l.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: c2.MaxSize})
				}
			}
			return facet.MeasureResult{Size: c2.MaxSize}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			c.layout.ArrangedBounds = bounds
			y := bounds.Min.Y
			for _, l := range c.leaves {
				if l == nil || l.Base().LayoutRole() == nil {
					continue
				}
				h := l.Base().LayoutRole().MeasuredSize.H
				l.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, y, bounds.Width(), h))
				y += h
			}
		},
	}
	c.layout.Child = facet.GroupChildContract{SupportedPlacement: facet.SupportsGrid | facet.SupportsLinear}
	c.AddRole(&c.layout)
	for j := 0; j < leafCount; j++ {
		leaf := newLayoutCountLeaf(gfx.Size{W: 32, H: 18})
		c.leaves = append(c.leaves, leaf)
		c.AddChild(leaf.Base())
	}
	return c
}

type benchCard struct {
	facet.Facet
	layout facet.LayoutRole
	index  int
	leaves []*layoutCountLeaf
}

func (c *benchCard) Base() *facet.Facet             { c.BindImpl(c); return &c.Facet }
func (c *benchCard) OnAttach(_ facet.AttachContext) {}
func (c *benchCard) OnDetach()                      {}
func (c *benchCard) OnActivate()                    {}
func (c *benchCard) OnDeactivate()                  {}

func collectLeaves(f facet.FacetImpl, out *[]*layoutCountLeaf) {
	if f == nil || f.Base() == nil {
		return
	}
	for _, child := range f.Base().Children() {
		if child == nil {
			continue
		}
		impl := child.Impl()
		if lf, ok := impl.(*layoutCountLeaf); ok {
			*out = append(*out, lf)
		}
		collectLeaves(impl, out)
	}
}
