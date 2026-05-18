package free

import (
	"fmt"
	"math"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// Child is the narrow view of a child facet participating in free placement.
type Child struct {
	FacetID    facet.FacetID
	Attachment facet.Attachment
	Layout     *facet.LayoutRole
	Contract   facet.GroupChildContract
}

// ArrangedChild captures a child arranged by free placement.
type ArrangedChild struct {
	FacetID   facet.FacetID
	Bounds    gfx.Rect
	ZPriority int32
	Contract  facet.GroupChildContract
}

// Policy places children at explicit coordinates inside the parent bounds.
type Policy struct{}

// New constructs a free-placement policy.
func New() *Policy { return &Policy{} }

// Measure returns zero size because free placement does not resize its parent.
func (p *Policy) Measure(children []Child, constraints gfx.Size) (gfx.Size, error) {
	return gfx.Size{}, nil
}

// Arrange positions each child at its explicit coordinates.
func (p *Policy) Arrange(children []Child, bounds gfx.Rect, allowOverflow bool) ([]ArrangedChild, error) {
	if p == nil || len(children) == 0 {
		return nil, nil
	}
	arranged := make([]ArrangedChild, 0, len(children))
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		if !child.Contract.SupportedPlacement.Has(facet.PlacementFree) {
			panic(fmt.Sprintf("layout contract violation: facet %d; layer %d; placement free; violated contract: unsupported placement mode; guidance: set SupportedPlacement to include free placement", child.FacetID, child.Attachment.LayerID))
		}
		free := child.Attachment.Placement.Free
		x := float64(free.X)
		y := float64(free.Y)
		if !finite(x) || !finite(y) {
			panic(fmt.Sprintf("layout contract violation: facet %d; layer %d; placement free; violated contract: free placement requires finite coordinates; guidance: supply finite X and Y", child.FacetID, child.Attachment.LayerID))
		}
		size := child.Layout.MeasuredSize
		if size == (gfx.Size{}) {
			size = child.Layout.Measure(facet.MeasureContext{}, facet.Constraints{}).Size
		}
		if free.Width.Valid {
			w := float64(free.Width.Value)
			if !finite(w) {
				panic(fmt.Sprintf("layout contract violation: facet %d; layer %d; placement free; violated contract: free placement width must be finite; guidance: supply a finite width or omit it", child.FacetID, child.Attachment.LayerID))
			}
			size.W = float32(w)
		}
		if free.Height.Valid {
			h := float64(free.Height.Value)
			if !finite(h) {
				panic(fmt.Sprintf("layout contract violation: facet %d; layer %d; placement free; violated contract: free placement height must be finite; guidance: supply a finite height or omit it", child.FacetID, child.Attachment.LayerID))
			}
			size.H = float32(h)
		}
		rect := gfx.RectFromXYWH(bounds.Min.X+float32(x), bounds.Min.Y+float32(y), size.W, size.H)
		if !allowOverflow {
			rect = clampToBounds(rect, bounds)
		}
		child.Layout.Arrange(facet.ArrangeContext{Placement: child.Attachment.Placement}, rect)
		arranged = append(arranged, ArrangedChild{
			FacetID:   child.FacetID,
			Bounds:    rect,
			ZPriority: child.Attachment.ZPriority,
			Contract:  child.Contract,
		})
	}
	return arranged, nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func clampToBounds(rect, bounds gfx.Rect) gfx.Rect {
	if rect.IsEmpty() || bounds.IsEmpty() {
		return rect
	}
	width := rect.Width()
	height := rect.Height()
	if width > bounds.Width() {
		width = bounds.Width()
	}
	if height > bounds.Height() {
		height = bounds.Height()
	}
	x := rect.Min.X
	y := rect.Min.Y
	if x < bounds.Min.X {
		x = bounds.Min.X
	}
	if y < bounds.Min.Y {
		y = bounds.Min.Y
	}
	if x+width > bounds.Max.X {
		x = bounds.Max.X - width
	}
	if y+height > bounds.Max.Y {
		y = bounds.Max.Y - height
	}
	return gfx.RectFromXYWH(x, y, width, height)
}

func (a ArrangedChild) String() string {
	return fmt.Sprintf("FacetID=%d Bounds=%v ZPriority=%d", a.FacetID, a.Bounds, a.ZPriority)
}
