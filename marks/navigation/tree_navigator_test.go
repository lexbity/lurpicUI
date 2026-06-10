package navigation

import (
	"fmt"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

type treeNavigatorRuntimeStub struct {
	tabsRuntimeStub
	icons map[string]runtimepkg.IconAsset
}

func (s treeNavigatorRuntimeStub) ResolveIcon(ref string) (runtimepkg.IconAsset, bool) {
	asset, ok := s.icons[ref]
	return asset, ok
}

func TestTreeNavigatorMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	tree, rt, measureCtx := newTreeNavigatorTestFixture(t, defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(tree, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := tree.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 760, H: 1400}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(10, 14, result.Size.W, result.Size.H)
	tree.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: tree.Layout.Parent,
		ChildGroup:  tree.Layout.Child,
	}, bounds)

	if got := tree.AccessibilityRole(); got != "tree" {
		t.Fatalf("accessibility role = %q, want tree", got)
	}
	if got := tree.AccessibleName(); got != "Project tree" {
		t.Fatalf("accessible name = %q, want Project tree", got)
	}
	if !tree.Focus.Focusable() {
		t.Fatal("expected tree navigator to be focusable")
	}
	if len(tree.Children()) != len(tree.cachedVisibleNodes) {
		t.Fatalf("expected %d child facets, got %d", len(tree.cachedVisibleNodes), len(tree.Children()))
	}
	if len(tree.cachedRowBounds) == 0 || tree.cachedContentHeight <= 0 {
		t.Fatalf("expected row geometry, got rows=%d content=%v", len(tree.cachedRowBounds), tree.cachedContentHeight)
	}

	rowHit := tree.Hit.HitTest(gfx.Point{
		X: tree.cachedRowBounds[0].Min.X + tree.cachedRowBounds[0].Width()*0.5,
		Y: tree.cachedRowBounds[0].Min.Y + tree.cachedRowBounds[0].Height()*0.5,
	})
	if !rowHit.Hit || (rowHit.MarkID != treeNavigatorMarkIDLabel && rowHit.MarkID != treeNavigatorMarkIDSelectionIndicator && rowHit.MarkID != treeNavigatorMarkIDTreeItem) {
		t.Fatalf("expected row hit, got %#v", rowHit)
	}
	discHit := tree.Hit.HitTest(gfx.Point{
		X: tree.cachedRowLeadingBounds[0].Min.X + tree.cachedRowLeadingBounds[0].Width()*0.5,
		Y: tree.cachedRowLeadingBounds[0].Min.Y + tree.cachedRowLeadingBounds[0].Height()*0.5,
	})
	if !discHit.Hit || discHit.MarkID != treeNavigatorMarkIDDisclosure {
		t.Fatalf("expected disclosure hit, got %#v", discHit)
	}

	anchors := tree.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := tree.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands")
	}
	var sawGlyphRun, sawFillPath bool
	for _, cmd := range cmds.Commands {
		switch cmd.(type) {
		case gfx.DrawGlyphRun:
			sawGlyphRun = true
		case gfx.FillPath:
			sawFillPath = true
		}
	}
	if !sawGlyphRun {
		t.Fatal("expected glyph commands")
	}
	if !sawFillPath {
		t.Fatal("expected fill path commands")
	}
}

func TestTreeNavigatorPointerKeyboardAndScroll(t *testing.T) {
	tree, rt, measureCtx := newTreeNavigatorTestFixture(t, defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(tree, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := tree.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 760, H: 160}})
	tree.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H))

	if !tree.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: tree.cachedRowLeadingBounds[0].Min.X + 1, Y: tree.cachedRowLeadingBounds[0].Min.Y + 1}, Button: platform.PointerLeft}) {
		t.Fatal("expected disclosure press to be handled")
	}
	if !tree.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: gfx.Point{X: tree.cachedRowLeadingBounds[0].Min.X + 1, Y: tree.cachedRowLeadingBounds[0].Min.Y + 1}, Button: platform.PointerLeft}) {
		t.Fatal("expected disclosure release to be handled")
	}
	if !tree.cachedVisibleNodes[0].Node.Expanded {
		t.Fatal("expected first node to expand after disclosure click")
	}
	tree.SetExpandedPath("test-item-1", true)

	secondRowCenter := gfx.Point{
		X: tree.cachedRowBounds[1].Min.X + tree.cachedRowBounds[1].Width()*0.5,
		Y: tree.cachedRowBounds[1].Min.Y + tree.cachedRowBounds[1].Height()*0.5,
	}
	if !tree.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: secondRowCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected row press to be handled")
	}
	if !tree.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: secondRowCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected row release to be handled")
	}

	if !tree.onScroll(facet.ScrollEvent{DeltaY: -24}) {
		t.Fatal("expected scroll to be handled")
	}
	if tree.scrollOffset == 0 {
		t.Fatal("expected scroll offset to change")
	}

	tree.onFocusLost()
	tree.onFocusGained()
	tree.focusedPath = "test-item-1"
	if !tree.focusedVisible {
		t.Fatal("expected keyboard focus to show focus ring")
	}
	if !tree.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected right key to be handled")
	}
	if !tree.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyLeft}) {
		t.Fatal("expected left key to be handled")
	}
	if !tree.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyDown}) {
		t.Fatal("expected down key to be handled")
	}
}

func TestTreeNavigatorDeepTreeHelpersIterative(t *testing.T) {
	const depth = 2048
	nodes := deepTreeNodes(depth)
	cloned := cloneTreeNodes(nodes)

	nodes[0].Label = "mutated-root"
	nodes[0].Children[0].Label = "mutated-child"
	if cloned[0].Label != "node-0000" {
		t.Fatalf("expected cloned root label to remain stable, got %q", cloned[0].Label)
	}
	if cloned[0].Children[0].Label != "node-0001" {
		t.Fatalf("expected cloned child label to remain stable, got %q", cloned[0].Children[0].Label)
	}

	clearSelection(nodes)
	deepLeafPath := deepTreePath(depth - 1)
	if !setSelectionByPath(nodes, deepLeafPath, true) {
		t.Fatalf("expected selection path %q to resolve", deepLeafPath)
	}
	if !selectedAtPath(nodes, deepLeafPath) {
		t.Fatalf("expected selection path %q to be selected", deepLeafPath)
	}
	parentPath := deepTreePath(depth - 2)
	if !setExpandedByPath(nodes, parentPath, false) {
		t.Fatalf("expected expansion path %q to resolve", parentPath)
	}
	if expandedAtPath(nodes, parentPath) {
		t.Fatalf("expected expansion path %q to be collapsed", parentPath)
	}
	if !toggleExpandedByPath(nodes, parentPath) {
		t.Fatalf("expected toggle path %q to resolve", parentPath)
	}
	if !expandedAtPath(nodes, parentPath) {
		t.Fatalf("expected expansion path %q to be expanded", parentPath)
	}

	tree := &TreeNavigator{Data: store.NewValueStore(cloneTreeNodes(nodes))}
	tree.rebuildVisibleNodes()
	if got := len(tree.cachedVisibleNodes); got != depth {
		t.Fatalf("visible node count = %d, want %d", got, depth)
	}
	if got := tree.cachedVisibleNodes[0].Path; got != deepTreePath(0) {
		t.Fatalf("first visible path = %q, want %q", got, deepTreePath(0))
	}
	if got := tree.cachedVisibleNodes[len(tree.cachedVisibleNodes)-1].Path; got != deepLeafPath {
		t.Fatalf("last visible path = %q, want %q", got, deepLeafPath)
	}
}

func TestTreeNavigatorGoldenDefault(t *testing.T) {
	AssertTreeNavigatorGolden(t, "default", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(tn *TreeNavigator) {})
}

func TestTreeNavigatorGoldenCompact(t *testing.T) {
	AssertTreeNavigatorGolden(t, "compact", defaultTabsTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(tn *TreeNavigator) {})
}

func TestTreeNavigatorGoldenDisabled(t *testing.T) {
	AssertTreeNavigatorGolden(t, "disabled", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(tn *TreeNavigator) {
		tn.Disabled = marks.Const(true)
	})
}

func TestTreeNavigatorGoldenHighContrast(t *testing.T) {
	AssertTreeNavigatorGolden(t, "high_contrast", highContrastTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(tn *TreeNavigator) {})
}

func TestTreeNavigatorGoldenHovered(t *testing.T) {
	AssertTreeNavigatorGolden(t, "hovered", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(tn *TreeNavigator) {
		tn.hoveredPath = "test-item-1"
	})
}

func TestTreeNavigatorGoldenPressed(t *testing.T) {
	AssertTreeNavigatorGolden(t, "pressed", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(tn *TreeNavigator) {
		tn.pressedPath = "test-item-1"
	})
}

func TestTreeNavigatorGoldenFocused(t *testing.T) {
	AssertTreeNavigatorGolden(t, "focused", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(tn *TreeNavigator) {
		tn.onFocusGained()
	})
}

func TestTreeNavigatorGoldenRTL(t *testing.T) {
	AssertTreeNavigatorGolden(t, "rtl", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(tn *TreeNavigator) {})
}

func AssertTreeNavigatorGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*TreeNavigator)) {
	t.Helper()
	tree, rt, measureCtx := newTreeNavigatorTestFixture(t, tokens, density, direction)
	if mutate != nil {
		mutate(tree)
	}
	renderTreeNavigatorToSurface(t, tree, rt, measureCtx, density, direction, name)
}

func renderTreeNavigatorToSurface(t *testing.T, tree *TreeNavigator, rt treeNavigatorRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(tree, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := tree.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 760, H: 1000}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	tree.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)
	cmds := tree.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands for golden")
	}
	surfaceW := int(result.Size.W) + 1
	surfaceH := int(result.Size.H) + 1
	if surfaceW < 1 {
		surfaceW = 1
	}
	if surfaceH < 1 {
		surfaceH = 1
	}
	surface := testkit.NewMemorySurface(surfaceW, surfaceH)
	r := softwarerenderer.NewSoftwareRenderer()
	if err := r.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      bounds,
				Opacity:     1,
				CommandHash: 1,
				Commands:    *cmds,
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "tree_navigator_"+goldenName)
}

func newTreeNavigatorTestFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*TreeNavigator, treeNavigatorRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	fonts := testkit.TestFontRegistry(t)
	rtTokens := tokens
	rtTokens.Density.Mode = densityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	tree := NewTreeNavigator("Project tree", []TreeNode{
		{
			Key:      "test-item-1",
			Label:    "test-item-1",
			Expanded: true,
			Children: []TreeNode{
				{Key: "test-item-1-1", Label: "test-item-1-1"},
				{Key: "test-item-1-2", Label: "test-item-1-2"},
			},
		},
		{Key: "test-item-2", Label: "test-item-2"},
		{Key: "test-item-3", Label: "test-item-3", Disabled: true},
		{Key: "test-item-4", Label: "test-item-4"},
		{Key: "test-item-5", Label: "test-item-5"},
		{Key: "test-item-6", Label: "test-item-6"},
		{Key: "test-item-7", Label: "test-item-7"},
		{Key: "test-item-8", Label: "test-item-8"},
		{Key: "test-item-9", Label: "test-item-9"},
		{Key: "test-item-10", Label: "test-item-10"},
	})
	rt := treeNavigatorRuntimeStub{
		tabsRuntimeStub: tabsRuntimeStub{rootStyle: rootStyle, fonts: fonts},
		icons: map[string]runtimepkg.IconAsset{
			"chevron-right": treeNavigatorIconAsset("chevron-right", gfx.NewPath().MoveTo(gfx.Point{X: 9, Y: 6}).LineTo(gfx.Point{X: 15, Y: 12}).LineTo(gfx.Point{X: 9, Y: 18}).Build()),
			"chevron-down":  treeNavigatorIconAsset("chevron-down", gfx.NewPath().MoveTo(gfx.Point{X: 6, Y: 9}).LineTo(gfx.Point{X: 12, Y: 15}).LineTo(gfx.Point{X: 18, Y: 9}).Build()),
		},
	}
	return tree, rt, resolved
}

func treeNavigatorIconAsset(ref string, path gfx.Path) runtimepkg.IconAsset {
	return runtimepkg.NewIconAsset(ref, 1, path, gfx.RectFromXYWH(0, 0, 24, 24))
}

func deepTreeNodes(depth int) []TreeNode {
	if depth <= 0 {
		return nil
	}
	children := []TreeNode(nil)
	for i := depth - 1; i >= 0; i-- {
		node := TreeNode{
			Key:      nodeKey(i),
			Label:    nodeKey(i),
			Expanded: true,
			Children: children,
		}
		children = []TreeNode{node}
	}
	return children
}

func deepTreePath(depth int) string {
	if depth < 0 {
		return ""
	}
	parts := make([]string, 0, depth+1)
	for i := 0; i <= depth; i++ {
		parts = append(parts, nodeKey(i))
	}
	return strings.Join(parts, "/")
}

func nodeKey(i int) string {
	return fmt.Sprintf("node-%04d", i)
}

func selectedAtPath(nodes []TreeNode, path string) bool {
	segments := splitPath(path)
	current := nodes
	for len(segments) > 0 {
		found := false
		for i := range current {
			if strings.TrimSpace(current[i].Key) != segments[0] {
				continue
			}
			if len(segments) == 1 {
				return current[i].Selected
			}
			current = current[i].Children
			segments = segments[1:]
			found = true
			break
		}
		if !found {
			return false
		}
	}
	return false
}

func expandedAtPath(nodes []TreeNode, path string) bool {
	segments := splitPath(path)
	current := nodes
	for len(segments) > 0 {
		found := false
		for i := range current {
			if strings.TrimSpace(current[i].Key) != segments[0] {
				continue
			}
			if len(segments) == 1 {
				return current[i].Expanded
			}
			current = current[i].Children
			segments = segments[1:]
			found = true
			break
		}
		if !found {
			return false
		}
	}
	return false
}
