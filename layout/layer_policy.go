package layout

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	gridpolicy "codeburg.org/lexbit/lurpicui/layout/grid"
)

// LayerChild is the narrow view of a child facet participating in a root layer policy.
type LayerChild struct {
	FacetID    facet.FacetID
	MarkID     facet.MarkID
	Attachment facet.Attachment
	Layout     *facet.LayoutRole
	Descriptor facet.GroupChildContract
}

// LayerMeasureContext carries the current resolved snapshot for root layer measurement.
type LayerMeasureContext struct {
	Runtime          facet.RuntimeServices
	Theme            any
	Layer            facet.LayerContext
	Bounds           gfx.Rect
	Recipe           ResolvedLayerLayoutRecipe
	ContentScale     float32
	Density          facet.DensityID
	WritingDirection facet.WritingDirection
	AnchorCache      *AnchorPositionCache
}

// LayerMeasureResult is the concrete root-layer measurement result.
type LayerMeasureResult struct {
	Size gfx.Size
}

// LayerArrangeContext carries the current resolved snapshot for root layer arrangement.
type LayerArrangeContext struct {
	LayerMeasureContext
	ClipRect gfx.Rect
}

// ArrangedLayerChild captures a child arranged by a root layer policy.
type ArrangedLayerChild struct {
	FacetID   facet.FacetID
	MarkID    facet.MarkID
	Bounds    gfx.Rect
	Placement facet.Placement
	ZPriority int32
	Contract  facet.GroupChildContract
}

// LayerLayoutPolicy arranges children within a resolved root layer.
type LayerLayoutPolicy interface {
	Kind() LayerLayoutKind
	MeasureLayer(ctx LayerMeasureContext, children []LayerChild) (LayerMeasureResult, error)
	ArrangeLayer(ctx LayerArrangeContext, children []LayerChild) ([]ArrangedLayerChild, error)
}

// ResolveLayerLayoutPolicy returns the policy implementation for a resolved recipe.
func ResolveLayerLayoutPolicy(recipe ResolvedLayerLayoutRecipe) LayerLayoutPolicy {
	switch recipe.PolicyKind {
	case LayerLayoutAnchor:
		return &anchorLayerPolicy{recipe: normalizeLayerRecipe(recipe)}
	case LayerLayoutFree:
		return &freeLayerPolicy{recipe: normalizeLayerRecipe(recipe)}
	case LayerLayoutGrid:
		fallthrough
	case LayerLayoutNone:
		fallthrough
	default:
		return &gridLayerPolicy{recipe: normalizeLayerRecipe(recipe)}
	}
}

// DefaultLayerLayoutPolicy returns the canonical 5x5 fallback policy.
func DefaultLayerLayoutPolicy() LayerLayoutPolicy {
	return ResolveLayerLayoutPolicy(DefaultLayerLayoutRecipe())
}

func layerGridConfig(recipe ResolvedLayerLayoutRecipe) gridpolicy.Config {
	cols := recipe.Grid.Columns
	rows := recipe.Grid.Rows
	if cols <= 0 {
		cols = DefaultGridConfig().Columns
	}
	if rows <= 0 {
		rows = DefaultGridConfig().Rows
	}
	return gridpolicy.Config{
		Columns:   defaultFlexTracks(cols),
		Rows:      defaultFlexTracks(rows),
		ColumnGap: float32(recipe.Grid.ColumnGap),
		RowGap:    float32(recipe.Grid.RowGap),
	}
}

func defaultFlexTracks(count int) []gridpolicy.TrackDef {
	if count <= 0 {
		count = 1
	}
	out := make([]gridpolicy.TrackDef, count)
	for i := range out {
		out[i] = gridpolicy.TrackDef{Sizing: gridpolicy.TrackFlex, Value: 1}
	}
	return out
}

func toGridChildren(children []LayerChild) []gridpolicy.Child {
	out := make([]gridpolicy.Child, 0, len(children))
	for i := range children {
		child := children[i]
		out = append(out, gridpolicy.Child{
			FacetID:    child.FacetID,
			Attachment: child.Attachment,
			Layout:     child.Layout,
			Contract:   child.Descriptor,
		})
	}
	return out
}

type gridLayerPolicy struct {
	recipe ResolvedLayerLayoutRecipe
}

type anchorLayerPolicy struct {
	recipe ResolvedLayerLayoutRecipe
}

type freeLayerPolicy struct {
	recipe ResolvedLayerLayoutRecipe
}

func (p *gridLayerPolicy) Kind() LayerLayoutKind   { return LayerLayoutGrid }
func (p *anchorLayerPolicy) Kind() LayerLayoutKind { return LayerLayoutAnchor }
func (p *freeLayerPolicy) Kind() LayerLayoutKind   { return LayerLayoutFree }

func normalizeLayerRecipe(recipe ResolvedLayerLayoutRecipe) ResolvedLayerLayoutRecipe {
	if recipe.PolicyKind == LayerLayoutNone {
		recipe = DefaultLayerLayoutRecipe()
	}
	if recipe.Grid.Columns <= 0 {
		recipe.Grid.Columns = DefaultGridConfig().Columns
	}
	if recipe.Grid.Rows <= 0 {
		recipe.Grid.Rows = DefaultGridConfig().Rows
	}
	return recipe
}

func (p *gridLayerPolicy) String() string {
	return fmt.Sprintf("grid(%dx%d)", p.recipe.Grid.Columns, p.recipe.Grid.Rows)
}
