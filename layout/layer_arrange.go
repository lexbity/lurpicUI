package layout

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	anchorpolicy "codeburg.org/lexbit/lurpicui/layout/anchor"
	freepolicy "codeburg.org/lexbit/lurpicui/layout/free"
	gridpolicy "codeburg.org/lexbit/lurpicui/layout/grid"
)

func (p *gridLayerPolicy) ArrangeLayer(ctx LayerArrangeContext, children []LayerChild) ([]ArrangedLayerChild, error) {
	if p == nil {
		return nil, nil
	}
	return arrangeGridLayer(ctx, p.recipe, children)
}

func (p *anchorLayerPolicy) ArrangeLayer(ctx LayerArrangeContext, children []LayerChild) ([]ArrangedLayerChild, error) {
	if p == nil {
		return nil, nil
	}
	return arrangeAnchorLayer(ctx, p.recipe, children)
}

func (p *freeLayerPolicy) ArrangeLayer(ctx LayerArrangeContext, children []LayerChild) ([]ArrangedLayerChild, error) {
	if p == nil {
		return nil, nil
	}
	return arrangeFreeLayer(ctx, p.recipe, children)
}

func arrangeGridLayer(ctx LayerArrangeContext, recipe ResolvedLayerLayoutRecipe, children []LayerChild) ([]ArrangedLayerChild, error) {
	policy := gridpolicy.New(layerGridConfig(recipe))
	arranged, err := policy.Arrange(toGridChildren(children), ctx.Bounds)
	if err != nil {
		return nil, err
	}
	byFacetID := make(map[facet.FacetID]LayerChild, len(children))
	for i := range children {
		byFacetID[children[i].FacetID] = children[i]
	}
	out := make([]ArrangedLayerChild, 0, len(arranged))
	for i := range arranged {
		child, ok := byFacetID[arranged[i].FacetID]
		if !ok {
			continue
		}
		if child.Layout == nil {
			continue
		}
		out = append(out, ArrangedLayerChild{
			FacetID:   arranged[i].FacetID,
			MarkID:    0,
			Bounds:    arranged[i].Bounds,
			Placement: child.Attachment.Placement,
			ZPriority: arranged[i].ZPriority,
			Contract:  child.Descriptor,
		})
	}
	return out, nil
}

func arrangeAnchorLayer(ctx LayerArrangeContext, recipe ResolvedLayerLayoutRecipe, children []LayerChild) ([]ArrangedLayerChild, error) {
	inner := insetRect(ctx.Bounds, recipe.Insets)
	bounds := inner
	if !ctx.ClipRect.IsEmpty() {
		bounds = ctx.ClipRect
	}
	policy := anchorpolicy.New()
	arranged, err := policy.Arrange(toAnchorChildren(children), bounds, anchorCacheAdapter{cache: ctx.AnchorCache}, false)
	if err != nil {
		return nil, err
	}
	out := make([]ArrangedLayerChild, 0, len(arranged))
	for i := range arranged {
		child, ok := findLayerChild(children, arranged[i].FacetID)
		if !ok || child.Layout == nil {
			continue
		}
		out = append(out, ArrangedLayerChild{
			FacetID:   arranged[i].FacetID,
			MarkID:    0,
			Bounds:    arranged[i].Bounds,
			Placement: child.Attachment.Placement,
			ZPriority: arranged[i].ZPriority,
			Contract:  child.Descriptor,
		})
	}
	return out, nil
}

func arrangeFreeLayer(ctx LayerArrangeContext, recipe ResolvedLayerLayoutRecipe, children []LayerChild) ([]ArrangedLayerChild, error) {
	inner := insetRect(ctx.Bounds, recipe.Insets)
	bounds := inner
	allowOverflow := true
	if !ctx.ClipRect.IsEmpty() {
		bounds = ctx.ClipRect
		allowOverflow = false
	}
	policy := freepolicy.New()
	arranged, err := policy.Arrange(toFreeChildren(children), bounds, allowOverflow)
	if err != nil {
		return nil, err
	}
	out := make([]ArrangedLayerChild, 0, len(arranged))
	for i := range arranged {
		child, ok := findLayerChild(children, arranged[i].FacetID)
		if !ok || child.Layout == nil {
			continue
		}
		out = append(out, ArrangedLayerChild{
			FacetID:   arranged[i].FacetID,
			MarkID:    0,
			Bounds:    arranged[i].Bounds,
			Placement: child.Attachment.Placement,
			ZPriority: arranged[i].ZPriority,
			Contract:  child.Descriptor,
		})
	}
	return out, nil
}

func normalizeGridPlacement(grid facet.GridPlacement, cols, rows int, nextCol, nextRow *int) (int, int, int, int, error) {
	colStart := grid.ColStart
	rowStart := grid.RowStart
	colSpan := grid.ColSpan
	rowSpan := grid.RowSpan
	auto := colStart == 0 && rowStart == 0 && colSpan == 0 && rowSpan == 0
	if auto {
		colSpan = 1
		rowSpan = 1
		if nextCol == nil || nextRow == nil {
			return 0, 0, 0, 0, fmt.Errorf("layout: grid auto-placement requires cursor state")
		}
		colStart = *nextCol
		rowStart = *nextRow
		*nextCol = *nextCol + 1
		if *nextCol >= cols {
			*nextCol = 0
			*nextRow = *nextRow + 1
		}
		if *nextRow >= rows {
			*nextRow = 0
		}
	}
	if colSpan <= 0 || rowSpan <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("layout: grid span must be positive")
	}
	if colStart < 0 || rowStart < 0 {
		return 0, 0, 0, 0, fmt.Errorf("layout: grid line must be non-negative")
	}
	if colStart >= cols || rowStart >= rows {
		return 0, 0, 0, 0, fmt.Errorf("layout: grid start outside track range")
	}
	if remaining := cols - colStart; colSpan > remaining {
		colSpan = remaining
	}
	if remaining := rows - rowStart; rowSpan > remaining {
		rowSpan = remaining
	}
	if colSpan <= 0 || rowSpan <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("layout: grid span collapsed after clamping")
	}
	return colStart, rowStart, colSpan, rowSpan, nil
}

type anchorCacheAdapter struct {
	cache *AnchorPositionCache
}

func (a anchorCacheAdapter) Get(id facet.AnchorID) (gfx.Point, bool) {
	if a.cache == nil {
		return gfx.Point{}, false
	}
	return a.cache.Get(AnchorID(id))
}

func toAnchorChildren(children []LayerChild) []anchorpolicy.Child {
	out := make([]anchorpolicy.Child, 0, len(children))
	for i := range children {
		child := children[i]
		out = append(out, anchorpolicy.Child{
			FacetID:    child.FacetID,
			Attachment: child.Attachment,
			Layout:     child.Layout,
			Contract:   child.Descriptor,
		})
	}
	return out
}

func toFreeChildren(children []LayerChild) []freepolicy.Child {
	out := make([]freepolicy.Child, 0, len(children))
	for i := range children {
		child := children[i]
		out = append(out, freepolicy.Child{
			FacetID:    child.FacetID,
			Attachment: child.Attachment,
			Layout:     child.Layout,
			Contract:   child.Descriptor,
		})
	}
	return out
}

func findLayerChild(children []LayerChild, id facet.FacetID) (LayerChild, bool) {
	for i := range children {
		if children[i].FacetID == id {
			return children[i], true
		}
	}
	return LayerChild{}, false
}
