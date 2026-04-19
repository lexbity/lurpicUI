package canvas_test

import (
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/graph/canvas"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

// nodeID returns a function usable as CollectionStore's identify for GraphNode.
func nodeIdentify(n canvas.GraphNode) store.ItemID { return n.ID }

// edgeIdentify returns a function usable as CollectionStore's identify for GraphEdge.
func edgeIdentify(e canvas.GraphEdge) store.ItemID { return e.ID }

// defaultStores builds three stores for tests.
func defaultStores() (
	*store.CollectionStore[canvas.GraphNode],
	*store.CollectionStore[canvas.GraphEdge],
	*store.ValueStore[canvas.ViewportState],
) {
	gs := store.NewCollectionStore(nodeIdentify)
	es := store.NewCollectionStore(edgeIdentify)
	vs := store.NewValueStore(canvas.ViewportState{Zoom: 1.0})
	return gs, es, vs
}

// testNode creates a GraphNode with a 50x50 bounding rect at (x,y).
func testNode(id store.ItemID, x, y float32) canvas.GraphNode {
	return canvas.GraphNode{
		ID:     id,
		Bounds: gfx.RectFromXYWH(x, y, 50, 50),
	}
}

// placeholderRGBA is the expected placeholder fill color (240, 240, 245, 255).
var placeholderRGBA = color.RGBA{R: 240, G: 240, B: 245, A: 255}

// nodeRGBA is the expected node fill color (100, 150, 220, 255).
var nodeRGBA = color.RGBA{R: 100, G: 150, B: 220, A: 255}

func TestGraphCanvas_initial_frame_shows_placeholder(t *testing.T) {
	gs, es, vs := defaultStores()
	f := canvas.NewGraphCanvasFacet(gs, es, vs)
	h := testkit.NewHarness(t, testkit.DefaultHarnessConfig(), f)
	h.RunFrame()
	// Before any Replace, nodeIndex is nil → placeholder color.
	testkit.AssertRegionColor(t, h.Surface(),
		gfx.RectFromXYWH(10, 10, 100, 100),
		placeholderRGBA, 5)
}

func TestGraphCanvas_index_builds_and_commits(t *testing.T) {
	gs, es, vs := defaultStores()
	f := canvas.NewGraphCanvasFacet(gs, es, vs)
	h := testkit.NewHarness(t, testkit.DefaultHarnessConfig(), f)

	h.RunFrame() // trigger OnAttach
	gs.Replace([]canvas.GraphNode{testNode(1, 100, 100)})

	ok := h.RunUntil(func() bool { return f.NodeIndex() != nil }, 30)
	if !ok {
		t.Fatal("index did not commit within 30 frames")
	}
}

func TestGraphCanvas_nodes_visible_after_index(t *testing.T) {
	gs, es, vs := defaultStores()
	f := canvas.NewGraphCanvasFacet(gs, es, vs)
	h := testkit.NewHarness(t, testkit.DefaultHarnessConfig(), f)

	h.RunFrame() // trigger OnAttach so subscriptions are set up
	// Place a node at screen position ~(100,100) with default zoom=1.
	gs.Replace([]canvas.GraphNode{testNode(1, 100, 100)})
	h.RunUntil(func() bool { return f.NodeIndex() != nil }, 30)

	// Node occupies (100,100)→(150,150) in world space = same in screen space at zoom=1.
	testkit.AssertPixelColor(t, h.Surface(), 110, 110, nodeRGBA, 10)
}

func TestGraphCanvas_store_change_triggers_rebuild(t *testing.T) {
	gs, es, vs := defaultStores()
	f := canvas.NewGraphCanvasFacet(gs, es, vs)
	h := testkit.NewHarness(t, testkit.DefaultHarnessConfig(), f)
	h.RunFrame()

	before := f.BuildCount()
	gs.Replace([]canvas.GraphNode{testNode(1, 0, 0)})
	h.RunFrame()

	if f.BuildCount() <= before {
		t.Fatalf("BuildCount did not increase after Replace: before=%d after=%d", before, f.BuildCount())
	}
}

func TestGraphCanvas_viewport_change_no_rebuild(t *testing.T) {
	gs, es, vs := defaultStores()
	f := canvas.NewGraphCanvasFacet(gs, es, vs)
	h := testkit.NewHarness(t, testkit.DefaultHarnessConfig(), f)

	gs.Replace([]canvas.GraphNode{testNode(1, 0, 0)})
	h.RunUntil(func() bool { return f.NodeIndex() != nil }, 30)

	before := f.BuildCount()
	vs.Set(canvas.ViewportState{Pan: gfx.Point{X: 50, Y: 50}, Zoom: 1.0})
	h.RunFrame()

	if f.BuildCount() != before {
		t.Fatalf("viewport change triggered unexpected rebuild: before=%d after=%d", before, f.BuildCount())
	}
}

func TestGraphCanvas_hit_test_finds_node(t *testing.T) {
	gs, es, vs := defaultStores()
	f := canvas.NewGraphCanvasFacet(gs, es, vs)

	var lastMarkID facet.MarkID
	f.SetOnPointer(func(e facet.PointerEvent) bool {
		lastMarkID = e.MarkID
		return true
	})

	h := testkit.NewHarness(t, testkit.DefaultHarnessConfig(), f)

	h.RunFrame() // trigger OnAttach
	gs.Replace([]canvas.GraphNode{testNode(1, 100, 100)})
	h.RunUntil(func() bool { return f.NodeIndex() != nil }, 30)

	// Click center of node at screen pos (125, 125) (zoom=1, pan=0).
	h.InjectEvent(testkit.PointerPress(125, 125, platform.PointerLeft))
	h.RunFrame()

	if lastMarkID != facet.MarkID(1) {
		t.Fatalf("expected MarkID=1, got %d", lastMarkID)
	}
}

func TestGraphCanvas_hit_test_misses_gap(t *testing.T) {
	gs, es, vs := defaultStores()
	f := canvas.NewGraphCanvasFacet(gs, es, vs)

	hit := false
	f.SetOnPointer(func(e facet.PointerEvent) bool {
		hit = true
		return true
	})

	h := testkit.NewHarness(t, testkit.DefaultHarnessConfig(), f)

	h.RunFrame() // trigger OnAttach
	gs.Replace([]canvas.GraphNode{testNode(1, 100, 100)})
	h.RunUntil(func() bool { return f.NodeIndex() != nil }, 30)

	// Click far from the node.
	h.InjectEvent(testkit.PointerPress(400, 400, platform.PointerLeft))
	h.RunFrame()

	if hit {
		t.Fatal("expected no hit far from node")
	}
}

func TestGraphCanvas_lod_clusters_at_low_zoom(t *testing.T) {
	gs, es, vs := defaultStores()
	f := canvas.NewGraphCanvasFacet(gs, es, vs)
	h := testkit.NewHarness(t, testkit.DefaultHarnessConfig(), f)

	h.RunFrame() // trigger OnAttach
	// Place many small nodes close together so they cluster at low zoom.
	nodes := make([]canvas.GraphNode, 50)
	for i := range nodes {
		nodes[i] = canvas.GraphNode{
			ID:     store.ItemID(i + 1),
			Bounds: gfx.RectFromXYWH(float32(i)*6, 0, 5, 5),
		}
	}
	gs.Replace(nodes)
	h.RunUntil(func() bool { return f.NodeIndex() != nil }, 30)

	// Set very low zoom so nodes are tiny (<8px each on screen).
	vs.Set(canvas.ViewportState{Zoom: 0.05})
	h.RunFrame()
	// At zoom=0.05 each 5-unit-wide node renders as 0.25px — must cluster.
	// Surface should not be blank (clusters are drawn).
	testkit.AssertNotBlank(t, h.Surface())
}

func TestGraphCanvas_stale_index_discarded(t *testing.T) {
	gs, es, vs := defaultStores()
	f := canvas.NewGraphCanvasFacet(gs, es, vs)
	h := testkit.NewHarness(t, testkit.DefaultHarnessConfig(), f)
	h.RunFrame() // trigger OnAttach

	// Trigger two rapid replacements; only the second set should be visible.
	first := []canvas.GraphNode{testNode(1, 100, 100)}
	second := []canvas.GraphNode{testNode(2, 200, 200)}
	gs.Replace(first)
	gs.Replace(second)

	h.RunUntil(func() bool { return f.NodeIndex() != nil }, 60)

	// NodeIndex must reflect the latest store state (node 2 at 200,200).
	idx := f.NodeIndex()
	if idx == nil {
		t.Fatal("expected index to be built")
	}
	_, found := idx.QueryPoint(gfx.Point{X: 225, Y: 225}, 30)
	if !found {
		t.Fatal("expected node from second Replace to be indexed")
	}
}
