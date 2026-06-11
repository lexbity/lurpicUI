package linear

import (
	"fmt"
	"sort"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// Axis selects the main axis for the linear policy.
type Axis uint8

const (
	Horizontal Axis = iota
	Vertical
)

// Config configures a linear group policy.
type Config struct {
	Axis Axis
	Gap  float32
}

// Child is the narrow view of a child facet participating in linear placement.
type Child struct {
	FacetID    facet.FacetID
	Attachment facet.Attachment
	Layout     *facet.LayoutRole
	Contract   facet.GroupChildContract
}

// ArrangedChild captures a child arranged by linear placement.
type ArrangedChild struct {
	FacetID   facet.FacetID
	Bounds    gfx.Rect
	ZPriority int32
	Contract  facet.GroupChildContract
}

// Policy arranges children sequentially along a main axis.
type Policy struct {
	cfg Config
}

// New constructs a linear policy.
func New(cfg Config) *Policy {
	return &Policy{cfg: cfg}
}

// Measure computes the preferred size of the linear group.
func (p *Policy) Measure(children []Child, constraints gfx.Size) (gfx.Size, error) {
	if p == nil || len(children) == 0 {
		return gfx.Size{}, nil
	}
	ordered := sortedChildren(children)
	main := float32(0)
	cross := float32(0)
	for _, idx := range ordered {
		child := children[idx]
		if child.Layout == nil {
			continue
		}
		if !child.Contract.SupportedPlacement.Has(facet.PlacementLinear) {
			panic(fmt.Sprintf("layout contract violation: facet %d; layer %d; placement linear; violated contract: unsupported placement mode; guidance: set SupportedPlacement to include linear placement", child.FacetID, child.Attachment.LayerID))
		}
		if child.Attachment.Placement.Linear.CrossAxisAlign == facet.CrossAxisBaseline {
			panic(fmt.Sprintf("layout contract violation: facet %d; layer %d; placement linear; violated contract: baseline alignment not supported; guidance: use a non-baseline cross-axis alignment", child.FacetID, child.Attachment.LayerID))
		}
		size := measuredSize(child)
		main += mainSize(size, p.cfg.Axis)
		if cs := crossSize(size, p.cfg.Axis); cs > cross {
			cross = cs
		}
	}
	if count := len(ordered); count > 1 {
		main += p.cfg.Gap * float32(count-1)
	}
	if p.cfg.Axis == Horizontal {
		return gfx.Size{W: main, H: cross}, nil
	}
	return gfx.Size{W: cross, H: main}, nil
}

// Arrange positions children in order along the main axis.
func (p *Policy) Arrange(children []Child, bounds gfx.Rect) ([]ArrangedChild, error) {
	if p == nil || len(children) == 0 {
		return nil, nil
	}
	ordered := sortedChildren(children)
	mainAvail := mainExtent(bounds, p.cfg.Axis)
	crossAvail := crossExtent(bounds, p.cfg.Axis)
	baseMain := float32(0)
	stretchCount := 0
	sizes := make([]gfx.Size, len(children))
	orderedCount := 0
	for _, idx := range ordered {
		child := children[idx]
		if child.Layout == nil {
			continue
		}
		if !child.Contract.SupportedPlacement.Has(facet.PlacementLinear) {
			panic(fmt.Sprintf("layout contract violation: facet %d; layer %d; placement linear; violated contract: unsupported placement mode; guidance: set SupportedPlacement to include linear placement", child.FacetID, child.Attachment.LayerID))
		}
		placement := child.Attachment.Placement.Linear
		if placement.CrossAxisAlign == facet.CrossAxisBaseline {
			panic(fmt.Sprintf("layout contract violation: facet %d; layer %d; placement linear; violated contract: baseline alignment not supported; guidance: use a non-baseline cross-axis alignment", child.FacetID, child.Attachment.LayerID))
		}
		if placement.MainAxisSize == facet.MainAxisMax || stretchRequested(child, p.cfg.Axis) {
			stretchCount++
		}
		size := measuredSize(child)
		sizes[idx] = size
		baseMain += mainSize(size, p.cfg.Axis)
		orderedCount++
	}
	if orderedCount > 1 {
		baseMain += p.cfg.Gap * float32(orderedCount-1)
	}
	residual := mainAvail - baseMain
	if residual < 0 {
		residual = 0
	}
	stretchShare := float32(0)
	if stretchCount > 0 {
		stretchShare = residual / float32(stretchCount)
	}
	pos := mainOrigin(bounds, p.cfg.Axis)
	arranged := make([]ArrangedChild, 0, orderedCount)
	for _, idx := range ordered {
		child := children[idx]
		if child.Layout == nil {
			continue
		}
		size := sizes[idx]
		placement := child.Attachment.Placement.Linear
		main := mainSize(size, p.cfg.Axis)
		if placement.MainAxisSize == facet.MainAxisMax || stretchRequested(child, p.cfg.Axis) {
			main += stretchShare
		}
		cross := crossSize(size, p.cfg.Axis)
		if placement.CrossAxisAlign == facet.CrossAxisStretch {
			if !crossStretchAllowed(child, p.cfg.Axis) {
				panic(fmt.Sprintf("layout contract violation: facet %d; layer %d; placement linear; violated contract: cross-axis stretch is not supported by the child contract; guidance: update the child stretch contract or choose a non-stretch cross-axis alignment", child.FacetID, child.Attachment.LayerID))
			}
			cross = crossAvail
		}
		crossPos := alignCross(bounds, p.cfg.Axis, placement.CrossAxisAlign, crossAvail, cross)
		var rect gfx.Rect
		if p.cfg.Axis == Horizontal {
			rect = gfx.RectFromXYWH(pos, crossPos, main, cross)
		} else {
			rect = gfx.RectFromXYWH(crossPos, pos, cross, main)
		}
		child.Layout.Arrange(facet.ArrangeContext{Placement: child.Attachment.Placement}, rect)
		arranged = append(arranged, ArrangedChild{
			FacetID:   child.FacetID,
			Bounds:    rect,
			ZPriority: child.Attachment.ZPriority,
			Contract:  child.Contract,
		})
		pos += main + p.cfg.Gap
	}
	sort.SliceStable(arranged, func(i, j int) bool {
		left := placementOrder(children, arranged[i].FacetID)
		right := placementOrder(children, arranged[j].FacetID)
		if left != right {
			return left < right
		}
		return arranged[i].FacetID < arranged[j].FacetID
	})
	return arranged, nil
}

func sortedChildren(children []Child) []int {
	indices := make([]int, len(children))
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(i, j int) bool {
		left := children[indices[i]]
		right := children[indices[j]]
		lo := left.Attachment.Placement.Linear.Order
		ro := right.Attachment.Placement.Linear.Order
		if lo != ro {
			return lo < ro
		}
		if left.Attachment.ZPriority != right.Attachment.ZPriority {
			return left.Attachment.ZPriority > right.Attachment.ZPriority
		}
		return left.FacetID < right.FacetID
	})
	return indices
}

func placementOrder(children []Child, id facet.FacetID) int {
	for i := range children {
		if children[i].FacetID == id {
			return children[i].Attachment.Placement.Linear.Order
		}
	}
	return 0
}

func measuredSize(child Child) gfx.Size {
	if child.Layout == nil {
		return gfx.Size{}
	}
	if child.Layout.MeasuredSize != (gfx.Size{}) {
		return child.Layout.MeasuredSize
	}
	return child.Layout.Measure(facet.MeasureContext{}, facet.Constraints{}).Size
}

func mainSize(size gfx.Size, axis Axis) float32 {
	if axis == Horizontal {
		return size.W
	}
	return size.H
}

func crossSize(size gfx.Size, axis Axis) float32 {
	if axis == Horizontal {
		return size.H
	}
	return size.W
}

func mainExtent(bounds gfx.Rect, axis Axis) float32 {
	if axis == Horizontal {
		return bounds.Width()
	}
	return bounds.Height()
}

func crossExtent(bounds gfx.Rect, axis Axis) float32 {
	if axis == Horizontal {
		return bounds.Height()
	}
	return bounds.Width()
}

func mainOrigin(bounds gfx.Rect, axis Axis) float32 {
	if axis == Horizontal {
		return bounds.Min.X
	}
	return bounds.Min.Y
}

func alignCross(bounds gfx.Rect, axis Axis, align facet.CrossAxisAlignment, available, childSize float32) float32 {
	origin := crossOrigin(bounds, axis)
	switch align {
	case facet.CrossAxisCenter:
		return origin + maxFloat(0, (available-childSize)/2)
	case facet.CrossAxisEnd:
		return origin + maxFloat(0, available-childSize)
	default:
		return origin
	}
}

func crossOrigin(bounds gfx.Rect, axis Axis) float32 {
	if axis == Horizontal {
		return bounds.Min.Y
	}
	return bounds.Min.X
}

func stretchRequested(child Child, axis Axis) bool {
	if axis == Horizontal {
		switch child.Contract.Stretch.Width {
		case facet.StretchAlways, facet.StretchWhenParentRequests:
			return true
		}
		return child.Attachment.Placement.Linear.MainAxisSize == facet.MainAxisMax
	}
	switch child.Contract.Stretch.Height {
	case facet.StretchAlways, facet.StretchWhenParentRequests:
		return true
	}
	return child.Attachment.Placement.Linear.MainAxisSize == facet.MainAxisMax
}

func crossStretchAllowed(child Child, axis Axis) bool {
	if axis == Horizontal {
		return child.Contract.Stretch.Height != facet.StretchNever
	}
	return child.Contract.Stretch.Width != facet.StretchNever
}

func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
