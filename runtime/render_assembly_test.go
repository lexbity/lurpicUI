package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testframe"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
)

// stubLayerResolver is a deterministic frameLayerResolver for the assembly
// test: it assigns each facet a layer order (higher = closer) and an empty
// clip.
type stubLayerResolver struct {
	orders map[facet.FacetID]int
}

func (s stubLayerResolver) ResolveProjectionLayer(id facet.FacetID) (facet.ProjectionLayer, bool) {
	order, ok := s.orders[id]
	if !ok {
		return facet.ProjectionLayer{}, false
	}
	return facet.ProjectionLayer{
		RenderOrder: order,
	}, true
}

func (s stubLayerResolver) ResolveChildAttachment(id facet.FacetID) (facet.Attachment, bool) {
	return facet.Attachment{}, false
}

func (s stubLayerResolver) ResolveWindowBinding(id facet.FacetID) (layout.WindowBinding, bool) {
	return layout.WindowBinding{}, false
}

func batchOutput(id int, bounds gfx.Rect) projection.RenderBatchOutput {
	var list gfx.CommandList
	list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(1, 1, 1, 255))})
	return projection.RenderBatchOutput{
		FacetID:  facet.FacetID(id),
		Commands: list,
		Bounds:   bounds,
	}
}

// TestRenderAssembly_stableBackToFront pins the Q6 render-batch contract: the
// assembled frame's RenderBatchs are ordered back-to-front by layer order,
// independent of the input ordering (deterministic assembly).
func TestRenderAssembly_stableBackToFront(t *testing.T) {
	// Three facets with layer orders 1000 (base), 5000 (floating), 7000
	// (modal). Present the inputs in REVERSE (front-first) order to prove the
	// assembly re-sorts deterministically.
	inputs := []projection.RenderBatchOutput{
		batchOutput(3, gfx.RectFromXYWH(0, 0, 50, 50)), // modal (front)
		batchOutput(2, gfx.RectFromXYWH(0, 0, 50, 50)), // floating
		batchOutput(1, gfx.RectFromXYWH(0, 0, 50, 50)), // base (back)
	}
	resolver := stubLayerResolver{orders: map[facet.FacetID]int{
		3: 7000,
		2: 5000,
		1: 1000,
	}}

	frame := assembleFrameInputWithLayers(&projection.FrameOutput{RenderBatchs: inputs}, nil, resolver)

	wantOrder := []facet.FacetID{1, 2, 3} // base → floating → modal
	if len(frame.Batches) != len(wantOrder) {
		t.Fatalf("RenderBatchs = %d, want %d", len(frame.Batches), len(wantOrder))
	}
	for i, id := range wantOrder {
		if frame.Batches[i].ID != render.RenderBatchID(id) {
			t.Fatalf("batch[%d].ID = %d, want %d", i, frame.Batches[i].ID, id)
		}
	}
}

// TestRenderAssembly_layeredBatchGrouping pins the LayeredBatch grouping: the
// frame packet groups consecutive batches that share a layer order + clip
// rect, and starts a new group when either changes.
func TestRenderAssembly_layeredBatchGrouping(t *testing.T) {
	inputs := []projection.RenderBatchOutput{
		batchOutput(1, gfx.RectFromXYWH(0, 0, 50, 50)),
		batchOutput(2, gfx.RectFromXYWH(0, 0, 50, 50)),
		batchOutput(3, gfx.RectFromXYWH(0, 0, 50, 50)),
	}
	resolver := stubLayerResolver{orders: map[facet.FacetID]int{1: 1000, 2: 1000, 3: 7000}}

	frame := assembleFrameInputWithLayers(&projection.FrameOutput{RenderBatchs: inputs}, nil, resolver)

	if len(frame.Layers) != 2 {
		t.Fatalf("LayeredBatch count = %d, want 2 (base group + modal group)", len(frame.Layers))
	}
	if frame.Layers[0].RenderOrder != 1000 || frame.Layers[0].BatchCount != 2 {
		t.Fatalf("base group = %+v, want order 1000 with 2 batches", frame.Layers[0])
	}
	if frame.Layers[1].RenderOrder != 7000 || frame.Layers[1].BatchCount != 1 {
		t.Fatalf("modal group = %+v, want order 7000 with 1 batch", frame.Layers[1])
	}
}

// TestRenderAssembly_retainedOutput pins the retained-frame contract: the
// assembled frame carries every render batch and its commands survive the
// assembly (commands are copied into the batch, not dropped).
func TestRenderAssembly_retainedOutput(t *testing.T) {
	inputs := []projection.RenderBatchOutput{
		batchOutput(1, gfx.RectFromXYWH(0, 0, 50, 50)),
	}
	resolver := stubLayerResolver{orders: map[facet.FacetID]int{1: 1000}}

	input := assembleFrameInputWithLayers(&projection.FrameOutput{RenderBatchs: inputs}, nil, resolver)
	frame := testframe.Seal(input)
	t.Cleanup(func() { _ = frame.Release() })
	if frame.BatchCount() != 1 {
		t.Fatalf("RenderBatchs = %d, want 1", frame.BatchCount())
	}
	count, ok := frame.BatchCommandCount(0)
	if !ok || count == 0 {
		t.Fatal("assembled batch dropped its commands")
	}
	found := false
	frame.VisitBatchCommands(0, func(command gfx.Command) bool {
		if _, ok := command.(gfx.FillRect); ok {
			found = true
		}
		return true
	})
	if !found {
		t.Fatal("assembled command page does not contain FillRect")
	}
}

// TestRenderAssembly_stableUnderShuffle pins determinism under input shuffling:
// assembling the same facets in either order yields the same back-to-front
// batch sequence and the same layered grouping.
func TestRenderAssembly_stableUnderShuffle(t *testing.T) {
	inputs := []projection.RenderBatchOutput{
		batchOutput(1, gfx.RectFromXYWH(0, 0, 50, 50)),
		batchOutput(2, gfx.RectFromXYWH(0, 0, 50, 50)),
		batchOutput(3, gfx.RectFromXYWH(0, 0, 50, 50)),
	}
	resolver := stubLayerResolver{orders: map[facet.FacetID]int{1: 1000, 2: 5000, 3: 7000}}

	frameA := assembleFrameInputWithLayers(&projection.FrameOutput{RenderBatchs: inputs}, nil, resolver)
	reversed := []projection.RenderBatchOutput{inputs[2], inputs[1], inputs[0]}
	frameB := assembleFrameInputWithLayers(&projection.FrameOutput{RenderBatchs: reversed}, nil, resolver)

	if len(frameA.Batches) != len(frameB.Batches) {
		t.Fatalf("batch counts differ: %d vs %d", len(frameA.Batches), len(frameB.Batches))
	}
	for i := range frameA.Batches {
		if frameA.Batches[i].ID != frameB.Batches[i].ID {
			t.Fatalf("batch[%d] differs under shuffle: %d vs %d", i, frameA.Batches[i].ID, frameB.Batches[i].ID)
		}
	}
	if len(frameA.Layers) != len(frameB.Layers) {
		t.Fatalf("layered-group counts differ under shuffle: %d vs %d", len(frameA.Layers), len(frameB.Layers))
	}
}
