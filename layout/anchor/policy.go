package anchor

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// Cache provides anchor lookup for a resolved parent.
type Cache interface {
	Get(id facet.AnchorID) (gfx.Point, bool)
}

// Child is the narrow view of a child facet participating in anchor placement.
type Child struct {
	FacetID    facet.FacetID
	Attachment facet.Attachment
	Layout     *facet.LayoutRole
	Contract   facet.GroupChildContract
}

// ArrangedChild captures a child arranged by anchor placement.
type ArrangedChild struct {
	FacetID   facet.FacetID
	Bounds    gfx.Rect
	Placement facet.AnchorPlacement
	ZPriority int32
	Contract  facet.GroupChildContract
}

// Policy places children relative to exported anchors.
type Policy struct{}

// New constructs an anchor-placement policy.
func New() *Policy { return &Policy{} }

// Measure returns zero size because anchor placement is resolved against the parent layer bounds.
func (p *Policy) Measure(children []Child, constraints gfx.Size) (gfx.Size, error) {
	return gfx.Size{}, nil
}

// Arrange positions each child relative to its referenced anchor.
func (p *Policy) Arrange(children []Child, bounds gfx.Rect, cache Cache, allowOverflow bool) ([]ArrangedChild, error) {
	if p == nil || len(children) == 0 {
		return nil, nil
	}
	if cache == nil {
		panic("layout/anchor: anchor placement requires an anchor cache")
	}
	arranged := make([]ArrangedChild, 0, len(children))
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		if !child.Contract.SupportedPlacement.Has(facet.PlacementAnchor) {
			panic(fmt.Sprintf("layout contract violation: facet %d; layer %d; placement anchor; violated contract: unsupported placement mode; guidance: set SupportedPlacement to include anchor placement", child.FacetID, child.Attachment.LayerID))
		}
		ref := child.Attachment.Placement.Anchor.AnchorRef
		if ref == "" {
			panic(fmt.Sprintf("layout contract violation: facet %d; layer %d; placement anchor; violated contract: missing anchor reference; guidance: export an anchor reference before arranging children", child.FacetID, child.Attachment.LayerID))
		}
		anchorPt, ok := cache.Get(ref)
		if !ok {
			panic(fmt.Sprintf("layout contract violation: facet %d; layer %d; placement anchor; violated contract: anchor %q does not exist; guidance: export the anchor before arranging children", child.FacetID, child.Attachment.LayerID, ref))
		}
		size := child.Layout.MeasuredSize
		if size == (gfx.Size{}) {
			size = child.Layout.Measure(facet.MeasureContext{}, facet.Constraints{}).Size
		}
		rect := anchorRect(anchorPt, size, child.Attachment.Placement.Anchor.Side, float32(child.Attachment.Placement.Anchor.Gap))
		rect.Min.X += float32(child.Attachment.Placement.Anchor.OffsetX)
		rect.Min.Y += float32(child.Attachment.Placement.Anchor.OffsetY)
		if !allowOverflow {
			rect = clampToBounds(rect, bounds)
		}
		child.Layout.Arrange(facet.ArrangeContext{Placement: child.Attachment.Placement}, rect)
		arranged = append(arranged, ArrangedChild{
			FacetID:   child.FacetID,
			Bounds:    rect,
			Placement: child.Attachment.Placement.Anchor,
			ZPriority: child.Attachment.ZPriority,
			Contract:  child.Contract,
		})
	}
	return arranged, nil
}
