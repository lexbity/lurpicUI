package verifylayout

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

// goodRoot builds a simple root with two non-overlapping child panels,
// each arranged at distinct positions by the root's OnArrange.
func goodRoot() facet.FacetImpl {
	root := &struct{ facet.Facet }{Facet: facet.NewFacet()}
	root.Base().BindImpl(root)

	left := &struct{ facet.Facet }{Facet: facet.NewFacet()}
	right := &struct{ facet.Facet }{Facet: facet.NewFacet()}

	leftRole := &facet.LayoutRole{}
	leftRole.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 200}}
	}
	leftRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		leftRole.ArrangedBounds = bounds
	}
	left.AddRole(leftRole)

	rightRole := &facet.LayoutRole{}
	rightRole.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 200}}
	}
	rightRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		rightRole.ArrangedBounds = bounds
	}
	right.AddRole(rightRole)

	root.AddChild(left.Base())
	root.AddChild(right.Base())

	rootRole := &facet.LayoutRole{}
	rootRole.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 1280, H: 800}}
	}
	rootRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		rootRole.ArrangedBounds = bounds
		leftRole.Arrange(ctx, gfx.RectFromXYWH(0, 0, 100, 200))
		rightRole.Arrange(ctx, gfx.RectFromXYWH(200, 0, 100, 200))
	}
	root.AddRole(rootRole)

	return root
}

// badRoot produces a child that has a RenderRole but no LayoutRole —
// simulating the blank-canvas pattern where the facet is never arranged.
func badRoot() facet.FacetImpl {
	root := &struct{ facet.Facet }{Facet: facet.NewFacet()}
	root.Base().BindImpl(root)

	unarranged := &struct{ facet.Facet }{Facet: facet.NewFacet()}
	unarranged.AddRole(&facet.RenderRole{})
	root.AddChild(unarranged.Base())

	rootRole := &facet.LayoutRole{}
	rootRole.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 1280, H: 800}}
	}
	rootRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		rootRole.ArrangedBounds = bounds
	}
	root.AddRole(rootRole)

	return root
}

// overlapRoot builds a root whose two children are placed at overlapping
// positions by the root's OnArrange.  Neither child is an overlay or stack.
func overlapRoot() facet.FacetImpl {
	root := &struct{ facet.Facet }{Facet: facet.NewFacet()}
	root.Base().BindImpl(root)

	childA := &struct{ facet.Facet }{Facet: facet.NewFacet()}
	childB := &struct{ facet.Facet }{Facet: facet.NewFacet()}

	roleA := &facet.LayoutRole{}
	roleA.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 100}}
	}
	roleA.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		roleA.ArrangedBounds = bounds
	}
	childA.AddRole(roleA)
	childA.AddRole(&facet.RenderRole{})

	roleB := &facet.LayoutRole{}
	roleB.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 100}}
	}
	roleB.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		roleB.ArrangedBounds = bounds
	}
	childB.AddRole(roleB)
	childB.AddRole(&facet.RenderRole{})

	root.AddChild(childA.Base())
	root.AddChild(childB.Base())

	rootRole := &facet.LayoutRole{}
	rootRole.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 1280, H: 800}}
	}
	rootRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		rootRole.ArrangedBounds = bounds
		roleA.Arrange(ctx, gfx.RectFromXYWH(0, 0, 100, 100))
		roleB.Arrange(ctx, gfx.RectFromXYWH(50, 50, 100, 100))
	}
	root.AddRole(rootRole)

	return root
}

// exceedRoot builds a root whose OnArrange sets its own bounds tighter than
// the window (50x50) while the child is placed at a position that exceeds
// those tighter bounds (at {0,0,80,80}).
func exceedRoot() facet.FacetImpl {
	root := &struct{ facet.Facet }{Facet: facet.NewFacet()}
	root.Base().BindImpl(root)

	child := &struct{ facet.Facet }{Facet: facet.NewFacet()}

	childRole := &facet.LayoutRole{}
	childRole.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 80, H: 80}}
	}
	childRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		childRole.ArrangedBounds = bounds
	}
	child.AddRole(childRole)
	child.AddRole(&facet.RenderRole{})

	root.AddChild(child.Base())

	rootRole := &facet.LayoutRole{}
	rootRole.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 50, H: 50}}
	}
	rootRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		rootRole.ArrangedBounds = gfx.RectFromXYWH(0, 0, 50, 50)
		childRole.Arrange(ctx, gfx.RectFromXYWH(0, 0, 80, 80))
	}
	root.AddRole(rootRole)

	return root
}

// stackRoot builds a root whose two children are siblings inside a
// StackLayout, which legitimately places them at overlapping positions.
func stackRoot() facet.FacetImpl {
	root := &struct{ facet.Facet }{Facet: facet.NewFacet()}
	root.Base().BindImpl(root)

	childA := &struct{ facet.Facet }{Facet: facet.NewFacet()}
	childB := &struct{ facet.Facet }{Facet: facet.NewFacet()}

	roleA := &facet.LayoutRole{}
	roleA.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 100}}
	}
	roleA.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		roleA.ArrangedBounds = bounds
	}
	childA.AddRole(roleA)
	childA.AddRole(&facet.RenderRole{})

	roleB := &facet.LayoutRole{}
	roleB.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 100}}
	}
	roleB.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		roleB.ArrangedBounds = bounds
	}
	childB.AddRole(roleB)
	childB.AddRole(&facet.RenderRole{})

	stack := layout.NewStackLayout(layout.AlignTopLeft)
	stack.AddChild(childA)
	stack.AddChild(childB)

	root.AddChild(stack.Base())

	stackRole := stack.Base().LayoutRole()
	rootRole := &facet.LayoutRole{}
	rootRole.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 1280, H: 800}}
	}
	rootRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		rootRole.ArrangedBounds = bounds
		stackRole.Arrange(ctx, gfx.RectFromXYWH(0, 0, 200, 200))
	}
	root.AddRole(rootRole)

	return root
}

// overlayRoot builds a root whose two children overlap, but one has a
// HitRole which exempts it from the sibling-overlap check.
func overlayRoot() facet.FacetImpl {
	root := &struct{ facet.Facet }{Facet: facet.NewFacet()}
	root.Base().BindImpl(root)

	childA := &struct{ facet.Facet }{Facet: facet.NewFacet()}
	childB := &struct{ facet.Facet }{Facet: facet.NewFacet()}

	roleA := &facet.LayoutRole{}
	roleA.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 100}}
	}
	roleA.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		roleA.ArrangedBounds = bounds
	}
	childA.AddRole(roleA)
	childA.AddRole(&facet.RenderRole{})
	childA.AddRole(&facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult { return facet.HitResult{} }})

	roleB := &facet.LayoutRole{}
	roleB.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 100}}
	}
	roleB.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		roleB.ArrangedBounds = bounds
	}
	childB.AddRole(roleB)
	childB.AddRole(&facet.RenderRole{})

	root.AddChild(childA.Base())
	root.AddChild(childB.Base())

	rootRole := &facet.LayoutRole{}
	rootRole.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 1280, H: 800}}
	}
	rootRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		rootRole.ArrangedBounds = bounds
		roleA.Arrange(ctx, gfx.RectFromXYWH(0, 0, 100, 100))
		roleB.Arrange(ctx, gfx.RectFromXYWH(50, 50, 100, 100))
	}
	root.AddRole(rootRole)

	return root
}

// overflowVisibleRoot builds a root where the child exceeds the parent but
// the parent's OverflowPolicy is OverflowVisible, which exempts it.
func overflowVisibleRoot() facet.FacetImpl {
	root := &struct{ facet.Facet }{Facet: facet.NewFacet()}
	root.Base().BindImpl(root)

	child := &struct{ facet.Facet }{Facet: facet.NewFacet()}

	childRole := &facet.LayoutRole{}
	childRole.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 200, H: 200}}
	}
	childRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		childRole.ArrangedBounds = bounds
	}
	child.AddRole(childRole)
	child.AddRole(&facet.RenderRole{})

	root.AddChild(child.Base())

	rootRole := &facet.LayoutRole{}
	rootRole.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 100}}
	}
	rootRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		rootRole.ArrangedBounds = bounds
		childRole.Arrange(ctx, gfx.RectFromXYWH(0, 0, 200, 200))
	}
	rootRole.Parent.Kind = facet.GroupLayoutLinearVertical
	rootRole.Parent.Overflow = facet.OverflowVisible
	root.AddRole(rootRole)

	return root
}

// sourceRoot builds a tree with a child that can be looked up via SourceOf.
func sourceRoot() (facet.FacetImpl, facet.FacetID) {
	root := &struct{ facet.Facet }{Facet: facet.NewFacet()}
	root.Base().BindImpl(root)

	unarranged := &struct{ facet.Facet }{Facet: facet.NewFacet()}
	childID := unarranged.Base().ID()
	unarranged.AddRole(&facet.RenderRole{})
	root.AddChild(unarranged.Base())

	rootRole := &facet.LayoutRole{}
	rootRole.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 1280, H: 800}}
	}
	rootRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		rootRole.ArrangedBounds = bounds
	}
	root.AddRole(rootRole)

	return root, childID
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCheck_GoodTree_NoFindings(t *testing.T) {
	root := goodRoot()
	findings := Check(root, Options{SkipOverlap: true})
	if len(findings) != 0 {
		t.Errorf("expected 0 findings on good tree, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  %s: %s", f.Kind, f.Detail)
		}
	}
}

func TestCheck_GoodTree_NoFindings_WithoutSkipOverlap(t *testing.T) {
	root := goodRoot()
	findings := Check(root, Options{})
	if len(findings) != 0 {
		t.Errorf("expected 0 findings on good tree (no SkipOverlap), got %d", len(findings))
		for _, f := range findings {
			t.Logf("  %s: %s", f.Kind, f.Detail)
		}
	}
}

func TestCheck_BadTree_EmptyBounds(t *testing.T) {
	root := badRoot()
	findings := Check(root, Options{SkipOverlap: true})
	if !containsKind(findings, KindEmptyBounds) && !containsKind(findings, KindMeasuredNotArranged) {
		t.Errorf("expected empty-bounds or measured-not-arranged finding on bad tree, got %d findings", len(findings))
		for _, f := range findings {
			t.Logf("  %s: %s", f.Kind, f.Detail)
		}
	}
}

func TestCheck_BadTree_SiblingOverlap(t *testing.T) {
	root := overlapRoot()
	findings := Check(root, Options{})
	if !containsKind(findings, KindSiblingOverlap) {
		t.Errorf("expected sibling-overlap finding on overlap tree, got %d: %v", len(findings), findingKinds(findings))
	}
}

func TestCheck_BadTree_ChildOutOfParent(t *testing.T) {
	root := exceedRoot()
	findings := Check(root, Options{SkipOverlap: true})
	if !containsKind(findings, KindChildOutOfParent) {
		t.Errorf("expected child-out-of-parent finding on exceed tree, got %d: %v", len(findings), findingKinds(findings))
	}
}

func TestCheck_StackChildrenExempt(t *testing.T) {
	root := stackRoot()
	findings := Check(root, Options{})
	if containsKind(findings, KindSiblingOverlap) {
		t.Errorf("stack children should be exempt from sibling-overlap, but got overlap finding: %v", findingKinds(findings))
	}
}

func TestCheck_OverlayExempt(t *testing.T) {
	root := overlayRoot()
	findings := Check(root, Options{})
	if containsKind(findings, KindSiblingOverlap) {
		t.Errorf("overlay children should be exempt from sibling-overlap, but got overlap finding: %v", findingKinds(findings))
	}
}

func TestCheck_OverflowVisibleExempt(t *testing.T) {
	root := overflowVisibleRoot()
	findings := Check(root, Options{SkipOverlap: true})
	if containsKind(findings, KindChildOutOfParent) {
		t.Errorf("OverflowVisible parent should exempt child-out-of-parent, but got finding: %v", findingKinds(findings))
	}
}

func TestCheck_SourceOf(t *testing.T) {
	root, childID := sourceRoot()
	sourceMap := map[facet.FacetID]string{
		childID: "test_fixture.go:42",
	}
	findings := Check(root, Options{
		SkipOverlap: true,
		SourceOf: func(id facet.FacetID) string {
			return sourceMap[id]
		},
	})
	if len(findings) == 0 {
		t.Fatal("expected at least 1 finding on source tree, got 0")
	}
	for _, f := range findings {
		if f.Source != "test_fixture.go:42" {
			t.Errorf("expected Source 'test_fixture.go:42', got %q", f.Source)
		}
	}
}

func TestAssert_GoodTree_NoError(t *testing.T) {
	root := goodRoot()
	findings := Assert(t, root, Options{SkipOverlap: true})
	if len(findings) != 0 {
		t.Errorf("expected 0 findings on good tree, got %d", len(findings))
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func containsKind(findings []Finding, k Kind) bool {
	for _, f := range findings {
		if f.Kind == k {
			return true
		}
	}
	return false
}

func findingKinds(findings []Finding) []Kind {
	out := make([]Kind, len(findings))
	for i, f := range findings {
		out[i] = f.Kind
	}
	return out
}
