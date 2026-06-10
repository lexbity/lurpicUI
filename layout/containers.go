package layout

import (
	"math"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// StackLayout places all children at the same origin.
type StackLayout struct {
	facet.Facet
	layout facet.LayoutRole

	children  []facet.FacetImpl
	Alignment Alignment
}

// NewStackLayout constructs a stack layout.
func NewStackLayout(alignment Alignment) *StackLayout {
	s := &StackLayout{Facet: facet.NewFacet(), Alignment: alignment}
	s.layout.OnMeasure = func(ctx facet.MeasureContext, c Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: s.onMeasure(c)}
	}
	s.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		s.onArrange(bounds)
	}
	s.AddRole(&s.layout)
	return s
}

// AddChild appends a child facet to the stack.
func (s *StackLayout) AddChild(child facet.FacetImpl) {
	if s == nil || child == nil {
		return
	}
	s.children = append(s.children, child)
	s.Facet.AddChild(child.Base())
}

func (s *StackLayout) onMeasure(c Constraints) gfx.Size {
	if s == nil {
		return gfx.Size{}
	}
	maxSize := gfx.Size{}
	layoutC := c
	for _, child := range s.children {
		size := measureChild(child, layoutC)
		if size.W > maxSize.W {
			maxSize.W = size.W
		}
		if size.H > maxSize.H {
			maxSize.H = size.H
		}
	}
	return layoutC.Constrain(maxSize)
}

func (s *StackLayout) onArrange(bounds gfx.Rect) {
	if s == nil {
		return
	}
	for _, child := range s.children {
		if child == nil || child.Base() == nil {
			continue
		}
		role := child.Base().LayoutRole()
		if role == nil {
			continue
		}
		childSize := role.MeasuredSize
		origin := alignedOrigin(childSize, bounds, s.Alignment)
		arrangeChild(child, gfx.RectFromXYWH(origin.X, origin.Y, childSize.W, childSize.H))
	}
}

// FlexChild wraps a child facet with sizing parameters.
type FlexChild struct {
	Facet   facet.FacetImpl
	Flex    float32
	MinSize gfx.Size
	MaxSize gfx.Size
}

// Fixed constructs a non-flex child.
func Fixed(f facet.FacetImpl) FlexChild { return FlexChild{Facet: f} }

// Flexible constructs a flex child.
func Flexible(f facet.FacetImpl, flex float32) FlexChild {
	return FlexChild{Facet: f, Flex: flex}
}

// RowLayout arranges children left-to-right.
type RowLayout struct {
	facet.Facet
	layout facet.LayoutRole

	children       []FlexChild
	Gap            float32
	Padding        gfx.Insets
	CrossAlignment Alignment
}

// NewRowLayout constructs a row layout.
func NewRowLayout() *RowLayout {
	r := &RowLayout{Facet: facet.NewFacet()}
	r.layout.OnMeasure = func(ctx facet.MeasureContext, c Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: r.onMeasure(c)}
	}
	r.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		r.onArrange(bounds)
	}
	r.AddRole(&r.layout)
	return r
}

// Add appends a child to the row.
func (r *RowLayout) Add(child FlexChild) {
	if r == nil || child.Facet == nil {
		return
	}
	r.children = append(r.children, child)
	r.AddChild(child.Facet.Base())
}

// ColumnLayout arranges children top-to-bottom.
type ColumnLayout struct {
	facet.Facet
	layout facet.LayoutRole

	children       []FlexChild
	Gap            float32
	Padding        gfx.Insets
	CrossAlignment Alignment
}

// NewColumnLayout constructs a column layout.
func NewColumnLayout() *ColumnLayout {
	c := &ColumnLayout{Facet: facet.NewFacet()}
	c.layout.OnMeasure = func(ctx facet.MeasureContext, cons Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: c.onMeasure(cons)}
	}
	c.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		c.onArrange(bounds)
	}
	c.AddRole(&c.layout)
	return c
}

// Add appends a child to the column.
func (c *ColumnLayout) Add(child FlexChild) {
	if c == nil || child.Facet == nil {
		return
	}
	c.children = append(c.children, child)
	c.AddChild(child.Facet.Base())
}

func (r *RowLayout) onMeasure(c Constraints) gfx.Size {
	return linearMeasure(r.children, c, r.Gap, r.Padding, true)
}

func (r *RowLayout) onArrange(bounds gfx.Rect) {
	linearArrange(r.children, bounds, r.Gap, r.Padding, r.CrossAlignment, true)
}

func (c *ColumnLayout) onMeasure(cons Constraints) gfx.Size {
	return linearMeasure(c.children, cons, c.Gap, c.Padding, false)
}

func (c *ColumnLayout) onArrange(bounds gfx.Rect) {
	linearArrange(c.children, bounds, c.Gap, c.Padding, c.CrossAlignment, false)
}

func linearMeasure(children []FlexChild, c Constraints, gap float32, padding gfx.Insets, horizontal bool) gfx.Size {
	inner := deflateConstraints(c, padding)
	totalGap := gap * float32(maxInt(0, len(children)-1))

	mainFixed := totalGap
	crossMax := float32(0)
	totalFlex := float32(0)
	fixedSizes := make([]gfx.Size, len(children))
	flexIndices := make([]int, 0, len(children))

	for i, child := range children {
		if child.Facet == nil {
			continue
		}
		if child.Flex <= 0 {
			size := measureChild(child.Facet, inner)
			size = clampChildSize(size, child)
			fixedSizes[i] = size
			mainFixed += mainSize(size, horizontal)
			if cs := crossSize(size, horizontal); cs > crossMax {
				crossMax = cs
			}
		} else {
			totalFlex += child.Flex
			flexIndices = append(flexIndices, i)
		}
	}

	remaining := mainAvailable(inner, horizontal) - mainFixed
	if remaining < 0 {
		remaining = 0
	}
	for _, i := range flexIndices {
		child := children[i]
		share := remaining
		if totalFlex > 0 {
			share = remaining * (child.Flex / totalFlex)
		}
		measureC := flexConstraints(inner, child, share, horizontal)
		size := measureChild(child.Facet, measureC)
		size = clampChildSize(size, child)
		fixedSizes[i] = size
		if cs := crossSize(size, horizontal); cs > crossMax {
			crossMax = cs
		}
	}

	mainTotal := mainFixed
	if len(flexIndices) > 0 {
		mainTotal = mainAvailable(inner, horizontal)
	}
	result := gfx.Size{}
	if horizontal {
		result.W = mainTotal + padding.Left + padding.Right
		result.H = crossMax + padding.Top + padding.Bottom
	} else {
		result.W = crossMax + padding.Left + padding.Right
		result.H = mainTotal + padding.Top + padding.Bottom
	}
	return c.Constrain(result)
}

func linearArrange(children []FlexChild, bounds gfx.Rect, gap float32, padding gfx.Insets, crossAlign Alignment, horizontal bool) {
	innerMin := gfx.Point{X: bounds.Min.X + padding.Left, Y: bounds.Min.Y + padding.Top}
	innerBounds := gfx.Rect{
		Min: innerMin,
		Max: gfx.Point{
			X: bounds.Max.X - padding.Right,
			Y: bounds.Max.Y - padding.Bottom,
		},
	}
	if innerBounds.Max.X < innerBounds.Min.X {
		innerBounds.Max.X = innerBounds.Min.X
	}
	if innerBounds.Max.Y < innerBounds.Min.Y {
		innerBounds.Max.Y = innerBounds.Min.Y
	}
	along := innerBounds.Min.X
	if !horizontal {
		along = innerBounds.Min.Y
	}

	for _, child := range children {
		if child.Facet == nil || child.Facet.Base() == nil {
			continue
		}
		role := child.Facet.Base().LayoutRole()
		if role == nil {
			continue
		}
		size := role.MeasuredSize
		if horizontal {
			y := alignedCrossOrigin(size.H, innerBounds, crossAlign, false)
			arrangeChild(child.Facet, gfx.RectFromXYWH(along, y, size.W, size.H))
			along += size.W + gap
		} else {
			x := alignedCrossOrigin(size.W, innerBounds, crossAlign, true)
			arrangeChild(child.Facet, gfx.RectFromXYWH(x, along, size.W, size.H))
			along += size.H + gap
		}
	}
}

func mainAvailable(c Constraints, horizontal bool) float32 {
	if horizontal {
		return c.MaxSize.W
	}
	return c.MaxSize.H
}

func mainSize(size gfx.Size, horizontal bool) float32 {
	if horizontal {
		return size.W
	}
	return size.H
}

func crossSize(size gfx.Size, horizontal bool) float32 {
	if horizontal {
		return size.H
	}
	return size.W
}

func flexConstraints(c Constraints, child FlexChild, share float32, horizontal bool) Constraints {
	if horizontal {
		out := c
		out.MinSize.W = child.MinSize.W
		if child.MaxSize.W > 0 {
			out.MaxSize.W = child.MaxSize.W
		} else {
			out.MaxSize.W = share
		}
		if c.MaxSize.H > 0 {
			out.MaxSize.H = c.MaxSize.H
		}
		return out
	}
	out := c
	out.MinSize.H = child.MinSize.H
	if child.MaxSize.H > 0 {
		out.MaxSize.H = child.MaxSize.H
	} else {
		out.MaxSize.H = share
	}
	if c.MaxSize.W > 0 {
		out.MaxSize.W = c.MaxSize.W
	}
	return out
}

func clampChildSize(size gfx.Size, child FlexChild) gfx.Size {
	return Constraints{MinSize: child.MinSize, MaxSize: child.MaxSize}.Constrain(size)
}

func alignedCrossOrigin(childExtent float32, bounds gfx.Rect, a Alignment, horizontal bool) float32 {
	if horizontal {
		delta := bounds.Width() - childExtent
		if delta < 0 {
			delta = 0
		}
		origin := bounds.Min.X
		switch a {
		case AlignTopCenter, AlignCenter, AlignBottomCenter:
			origin += delta / 2
		case AlignTopRight, AlignCenterRight, AlignBottomRight:
			origin += delta
		}
		return origin
	}
	delta := bounds.Height() - childExtent
	if delta < 0 {
		delta = 0
	}
	origin := bounds.Min.Y
	switch a {
	case AlignCenterLeft, AlignCenter, AlignCenterRight:
		origin += delta / 2
	case AlignBottomLeft, AlignBottomCenter, AlignBottomRight:
		origin += delta
	}
	return origin
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// PaddingLayout wraps a single child with insets.
type PaddingLayout struct {
	facet.Facet
	layout facet.LayoutRole

	Child   facet.FacetImpl
	Padding gfx.Insets
}

// NewPaddingLayout constructs a padding layout.
func NewPaddingLayout(child facet.FacetImpl, padding gfx.Insets) *PaddingLayout {
	p := &PaddingLayout{Facet: facet.NewFacet(), Child: child, Padding: padding}
	p.layout.OnMeasure = func(ctx facet.MeasureContext, c Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: p.onMeasure(c)}
	}
	p.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		p.onArrange(bounds)
	}
	p.AddRole(&p.layout)
	if child != nil {
		p.AddChild(child.Base())
	}
	return p
}

func (p *PaddingLayout) onMeasure(c Constraints) gfx.Size {
	layoutC := c
	inner := deflateConstraints(layoutC, p.Padding)
	childSize := gfx.Size{}
	if p.Child != nil {
		childSize = measureChild(p.Child, inner)
	}
	return layoutC.Constrain(gfx.Size{
		W: childSize.W + p.Padding.Left + p.Padding.Right,
		H: childSize.H + p.Padding.Top + p.Padding.Bottom,
	})
}

func (p *PaddingLayout) onArrange(bounds gfx.Rect) {
	if p.Child == nil {
		return
	}
	arrangeChild(p.Child, gfx.Rect{
		Min: gfx.Point{X: bounds.Min.X + p.Padding.Left, Y: bounds.Min.Y + p.Padding.Top},
		Max: gfx.Point{X: bounds.Max.X - p.Padding.Right, Y: bounds.Max.Y - p.Padding.Bottom},
	})
}

// SizedBox forces a fixed size or acts as empty space.
type SizedBox struct {
	facet.Facet
	layout facet.LayoutRole

	Child  facet.FacetImpl
	Width  float32
	Height float32
}

// NewSizedBox constructs a sized box.
func NewSizedBox(w, h float32, child facet.FacetImpl) *SizedBox {
	s := &SizedBox{Facet: facet.NewFacet(), Child: child, Width: w, Height: h}
	s.layout.OnMeasure = func(ctx facet.MeasureContext, c Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: s.onMeasure(c)}
	}
	s.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		s.onArrange(bounds)
	}
	s.AddRole(&s.layout)
	if child != nil {
		s.AddChild(child.Base())
	}
	return s
}

func (s *SizedBox) onMeasure(c Constraints) gfx.Size {
	layoutC := c
	forced := gfx.Size{W: s.Width, H: s.Height}
	if s.Child != nil {
		measureChild(s.Child, Tight(forced))
	}
	return layoutC.Constrain(forced)
}

func (s *SizedBox) onArrange(bounds gfx.Rect) {
	if s.Child == nil {
		return
	}
	arrangeChild(s.Child, bounds)
}

// SplitAxis selects the split direction.
type SplitAxis uint8

const (
	SplitHorizontal SplitAxis = iota
	SplitVertical
)

// SplitLayout divides space between two children.
type SplitLayout struct {
	facet.Facet
	layout facet.LayoutRole

	First, Second facet.FacetImpl
	Axis          SplitAxis
	SplitFraction float32
	DividerWidth  float32
	MinFirstSize  float32
	MinSecondSize float32
}

// NewSplitLayout constructs a split layout.
func NewSplitLayout(axis SplitAxis, fraction float32) *SplitLayout {
	s := &SplitLayout{Facet: facet.NewFacet(), Axis: axis, SplitFraction: fraction}
	s.layout.OnMeasure = func(ctx facet.MeasureContext, c Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: s.onMeasure(c)}
	}
	s.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		s.onArrange(bounds)
	}
	s.AddRole(&s.layout)
	return s
}

// SetFirst assigns the first child and adds it to the tree.
func (s *SplitLayout) SetFirst(child facet.FacetImpl) {
	s.First = child
	if child != nil {
		s.AddChild(child.Base())
	}
}

// SetSecond assigns the second child and adds it to the tree.
func (s *SplitLayout) SetSecond(child facet.FacetImpl) {
	s.Second = child
	if child != nil {
		s.AddChild(child.Base())
	}
}

func (s *SplitLayout) onMeasure(c Constraints) gfx.Size {
	layoutC := c
	return layoutC.Constrain(layoutC.MaxSize)
}

func (s *SplitLayout) onArrange(bounds gfx.Rect) {
	total := bounds.Width()
	if s.Axis == SplitVertical {
		total = bounds.Height()
	}
	fraction := s.SplitFraction
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	usable := total - s.DividerWidth
	if usable < 0 {
		usable = 0
	}
	firstSize := float32(math.Round(float64(usable * fraction)))
	secondSize := usable - firstSize
	if firstSize < s.MinFirstSize {
		firstSize = s.MinFirstSize
		secondSize = usable - firstSize
	}
	if secondSize < s.MinSecondSize {
		secondSize = s.MinSecondSize
		firstSize = usable - secondSize
	}
	if firstSize < 0 {
		firstSize = 0
	}
	if secondSize < 0 {
		secondSize = 0
	}
	if s.Axis == SplitHorizontal {
		if s.First != nil {
			arrangeChild(s.First, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, firstSize, bounds.Height()))
		}
		if s.Second != nil {
			arrangeChild(s.Second, gfx.RectFromXYWH(bounds.Min.X+firstSize+s.DividerWidth, bounds.Min.Y, secondSize, bounds.Height()))
		}
	} else {
		if s.First != nil {
			arrangeChild(s.First, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), firstSize))
		}
		if s.Second != nil {
			arrangeChild(s.Second, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y+firstSize+s.DividerWidth, bounds.Width(), secondSize))
		}
	}
}

// ScrollAxes indicates which axes can scroll.
type ScrollAxes uint8

const (
	ScrollVertical ScrollAxes = 1 << iota
	ScrollHorizontal
	ScrollBoth = ScrollVertical | ScrollHorizontal
)

// ScrollLayout makes its child scrollable.
type ScrollLayout struct {
	facet.Facet
	layout facet.LayoutRole

	Child        facet.FacetImpl
	ScrollAxes   ScrollAxes
	ScrollOffset gfx.Point
	contentSize  gfx.Size
}

// NewScrollLayout constructs a scroll layout.
func NewScrollLayout(axes ScrollAxes, child facet.FacetImpl) *ScrollLayout {
	s := &ScrollLayout{Facet: facet.NewFacet(), Child: child, ScrollAxes: axes}
	s.layout.OnMeasure = func(ctx facet.MeasureContext, c Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: s.onMeasure(c)}
	}
	s.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		s.onArrange(bounds)
	}
	s.AddRole(&s.layout)
	if child != nil {
		s.AddChild(child.Base())
	}
	return s
}

func (s *ScrollLayout) onMeasure(c Constraints) gfx.Size {
	inner := c
	childConstraints := inner
	if s.ScrollAxes&ScrollVertical != 0 {
		childConstraints = childConstraints.WithMaxHeight(0)
	}
	if s.ScrollAxes&ScrollHorizontal != 0 {
		childConstraints = childConstraints.WithMaxWidth(0)
	}
	if s.Child != nil {
		s.contentSize = measureChild(s.Child, childConstraints)
	} else {
		s.contentSize = gfx.Size{}
	}
	size := inner.MaxSize
	if size.W == 0 {
		size.W = s.contentSize.W
	}
	if size.H == 0 {
		size.H = s.contentSize.H
	}
	return inner.Constrain(size)
}

func (s *ScrollLayout) onArrange(bounds gfx.Rect) {
	if s.Child == nil {
		return
	}
	arrangeChild(s.Child, gfx.RectFromXYWH(
		bounds.Min.X-s.ScrollOffset.X,
		bounds.Min.Y-s.ScrollOffset.Y,
		s.contentSize.W,
		s.contentSize.H,
	))
}
