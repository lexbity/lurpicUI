package structure

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout/linear"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

// familyName is the descriptor family shared by every structure mark.
const familyName = "structure"

// CrossAlign selects the cross-axis alignment for Row/Column children.
type CrossAlign uint8

const (
	// CrossAlignStart aligns children at the cross-axis start.
	CrossAlignStart CrossAlign = iota
	// CrossAlignCenter centers children on the cross axis.
	CrossAlignCenter
	// CrossAlignEnd aligns children at the cross-axis end.
	CrossAlignEnd
	// CrossAlignStretch stretches children to fill the cross axis.
	CrossAlignStretch
)

// AxisChild describes one child facet placed in a Row or Column.
type AxisChild struct {
	Facet  facet.FacetImpl
	MarkID facet.MarkID
	// Weight distributes free main-axis space proportionally. 0 = hug the
	// child's measured size; weights are honored only when the linear
	// placement requests fill (MainAxisMax), which the axis marks set for
	// every Weight > 0 child.
	Weight float32
	// Align overrides the mark-level cross-axis alignment for this child.
	Align CrossAlign
}

// linearMarkIDFirstChild is the fallback MarkID for axis children that do not
// declare one. The offset keeps axis children clear of mark-internal IDs.
const linearMarkIDFirstChild facet.MarkID = 100

// AxisConfig is the construction-time configuration for Row/Column (the
// content/config rule: geometry declared once at construction; only fields
// whose source changes at runtime are assigned a dynamic binding afterwards).
type AxisConfig struct {
	// Gap is the spacing between adjacent children along the main axis.
	Gap float32
	// PadX insets the content horizontally on both sides.
	PadX float32
	// PadY insets the content vertically on both sides.
	PadY float32
	// CrossAlign is the default cross-axis alignment for children that do
	// not declare their own.
	CrossAlign CrossAlign
}

// axis is the shared implementation behind Row and Column. The exported types
// exist so descriptors and type identity distinguish the orientations; all
// layout logic lives here, parameterized by axis. The binding fields are
// declared on axis and promoted onto both marks.
type axis struct {
	// Gap is the spacing between adjacent children along the main axis.
	Gap marks.Binding[float32]
	// PadX insets the content horizontally on both sides.
	PadX marks.Binding[float32]
	// PadY insets the content vertically on both sides.
	PadY marks.Binding[float32]
	// CrossAlign is the default cross-axis alignment for children that do
	// not declare their own.
	CrossAlign marks.Binding[CrossAlign]

	horizontal bool
	overflow   facet.OverflowPolicy
	children   []AxisChild
}

func newAxis(horizontal bool, children []AxisChild, cfg AxisConfig) axis {
	return axis{
		Gap:        marks.Const(cfg.Gap),
		PadX:       marks.Const(cfg.PadX),
		PadY:       marks.Const(cfg.PadY),
		CrossAlign: marks.Const(cfg.CrossAlign),
		horizontal: horizontal,
		overflow:   facet.OverflowScroll,
		children:   append([]AxisChild(nil), children...),
	}
}

// axisLayoutSource is the view the group policy adapter needs of a Row/Column.
type axisLayoutSource interface {
	facet.ChildSource
	measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult
	arrange(ctx facet.ArrangeContext, bounds gfx.Rect)
}

// initAxisContracts wires the shared layout contracts onto the mark's core.
func initAxisContracts(src axisLayoutSource, core *marks.Core, a *axis) {
	kind := facet.GroupLayoutLinearVertical
	if a.horizontal {
		kind = facet.GroupLayoutLinearHorizontal
	}
	core.Layout.Parent = facet.GroupParentContract{
		Kind:     kind,
		Policy:   axisGroupPolicy{src: src, horizontal: a.horizontal},
		Children: src,
		Overflow: a.overflow,
	}
	core.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := src.measure(ctx, constraints).Size
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionClip,
			BelowMinHeight: facet.CompressionClip,
			AboveMaxWidth:  facet.ExpansionClip,
			AboveMaxHeight: facet.ExpansionClip,
		},
		Stretch: facet.StretchPolicy{
			Width:  facet.StretchWhenParentRequests,
			Height: facet.StretchWhenParentRequests,
		},
		Baseline: facet.BaselineNone,
	}
	core.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return src.measure(ctx, constraints)
	}
	core.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		src.arrange(ctx, bounds)
	}
}

// axisGroupPolicy adapts the axis marks to the GroupLayoutPolicy contract
// (facet requires a policy whenever the parent kind is not GroupLayoutNone).
// The runtime layout pass drives measure/arrange through the layout role; the
// policy methods report the same results for contract inspection.
type axisGroupPolicy struct {
	src        axisLayoutSource
	horizontal bool
}

func (p axisGroupPolicy) Kind() facet.GroupLayoutKind {
	if p.horizontal {
		return facet.GroupLayoutLinearHorizontal
	}
	return facet.GroupLayoutLinearVertical
}

func (p axisGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.src == nil {
		return facet.GroupMeasureResult{}, nil
	}
	size := p.src.measure(ctx.MeasureContext, facet.Constraints{
		MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()},
	}).Size
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p axisGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.src == nil {
		return nil, nil
	}
	p.src.arrange(ctx.ArrangeContext, ctx.Bounds)
	arranged := make([]facet.ArrangedGroupChild, 0, len(children))
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		arranged = append(arranged, facet.ArrangedGroupChild{
			FacetID:   child.FacetID,
			MarkID:    child.MarkID,
			Bounds:    child.Layout.ArrangedBounds,
			Placement: child.Attachment.Placement,
			ZOrder:    child.Attachment.ZOrder,
			Contract:  child.Contract,
		})
	}
	return arranged, nil
}

// crossAlignToFacet converts CrossAlign to the facet enumeration.
func crossAlignToFacet(a CrossAlign) facet.CrossAxisAlignment {
	switch a {
	case CrossAlignCenter:
		return facet.CrossAxisCenter
	case CrossAlignEnd:
		return facet.CrossAxisEnd
	case CrossAlignStretch:
		return facet.CrossAxisStretch
	default:
		return facet.CrossAxisStart
	}
}

// mainAxisSize converts a Weight to the linear placement's main-axis sizing.
func mainAxisSize(w float32) facet.MainAxisSize {
	if w > 0 {
		return facet.MainAxisMax
	}
	return facet.MainAxisMin
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// groupChildren builds the GroupChild list for the declared children. The
// per-child cross-axis alignment falls back to the mark-level CrossAlign
// binding when the child declares CrossAlignStart.
func (a *axis) groupChildren() []facet.GroupChild {
	out := make([]facet.GroupChild, 0, len(a.children))
	for i, child := range a.children {
		if child.Facet == nil || child.Facet.Base() == nil || child.Facet.Base().LayoutRole() == nil {
			continue
		}
		base := child.Facet.Base()
		markID := child.MarkID
		if markID == 0 {
			markID = linearMarkIDFirstChild + facet.MarkID(i)
		}
		align := child.Align
		if align == CrossAlignStart {
			align = a.CrossAlign.Get()
		}
		out = append(out, facet.GroupChild{
			FacetID: base.ID(),
			MarkID:  markID,
			Attachment: facet.Attachment{
				Placement: facet.Placement{
					Mode: facet.PlacementLinear,
					Linear: facet.LinearPlacement{
						Order:          i,
						CrossAxisAlign: crossAlignToFacet(align),
						MainAxisSize:   mainAxisSize(child.Weight),
						Weight:         child.Weight,
					},
				},
			},
			Layout:   base.LayoutRole(),
			Contract: base.LayoutRole().Child,
		})
	}
	return out
}

func groupChildToLinear(g facet.GroupChild) linear.Child {
	return linear.Child{
		FacetID:    g.FacetID,
		Attachment: g.Attachment,
		Layout:     g.Layout,
		Contract:   g.Contract,
	}
}

// measure implements the shared axis measurement. Children are measured
// first (the policy reads each child's cached MeasuredSize), then the linear
// policy sums them with gap and padding.
func (a *axis) measure(core *marks.Core, ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	gap := a.Gap.Get()
	padX := a.PadX.Get()
	padY := a.PadY.Get()

	group := a.groupChildren()
	kids := make([]linear.Child, 0, len(group))
	inner := gfx.Size{
		W: maxFloat32(constraints.MaxSize.W-padX*2, 0),
		H: maxFloat32(constraints.MaxSize.H-padY*2, 0),
	}
	for _, g := range group {
		if g.Layout != nil {
			g.Layout.Measure(ctx, facet.Constraints{MaxSize: inner})
		}
		kids = append(kids, groupChildToLinear(g))
	}
	var measured gfx.Size
	if len(kids) > 0 {
		var policy *linear.Policy
		if a.horizontal {
			policy = linear.NewHorizontal(gap)
		} else {
			policy = linear.NewVertical(gap)
		}
		size, err := policy.Measure(kids, gfx.Size{
			W: inner.W,
			H: inner.H,
		})
		if err != nil {
			// The policy panics on contract violations; an error here means
			// the constraints themselves were incoherent. Measure to zero and
			// let the constraint pass decide the final size.
			size = gfx.Size{}
		}
		measured = size
	}
	measured.W += padX * 2
	measured.H += padY * 2
	measured = constraints.Constrain(measured)

	core.Layout.MeasuredSize = measured
	core.Layout.MeasuredResult = facet.MeasureResult{
		Size:        measured,
		Intrinsic:   facet.IntrinsicSize{Min: measured, Preferred: measured, Max: measured},
		Constraints: constraints,
	}
	return core.Layout.MeasuredResult
}

// arrange implements the shared axis arrangement. The policy applies each
// child's arranged bounds via child.Layout.Arrange.
func (a *axis) arrange(_ *marks.Core, _ facet.ArrangeContext, bounds gfx.Rect) {
	if bounds.IsEmpty() {
		return
	}
	gap := a.Gap.Get()
	padX := a.PadX.Get()
	padY := a.PadY.Get()

	group := a.groupChildren()
	if len(group) == 0 {
		return
	}
	kids := make([]linear.Child, 0, len(group))
	for _, g := range group {
		kids = append(kids, groupChildToLinear(g))
	}
	var policy *linear.Policy
	if a.horizontal {
		policy = linear.NewHorizontal(gap)
	} else {
		policy = linear.NewVertical(gap)
	}
	inner := bounds.Inset(padX, padY)
	if inner.IsEmpty() {
		inner = bounds
	}
	_, _ = policy.Arrange(kids, inner)
}

// attachAxisChildren attaches declared children to the facet tree so the
// runtime projects and hit-tests them at their arranged bounds.
func attachAxisChildren(core *marks.Core, kids []AxisChild) {
	for _, child := range kids {
		if child.Facet == nil || child.Facet.Base() == nil {
			continue
		}
		core.AddChild(child.Facet.Base())
	}
}

// Row implements a horizontal linear layout mark over layout/linear.
// Children are attached as real tree children so interactive marks receive
// input. Row draws no chrome; wrap it in a Card or provide a background.
type Row struct {
	marks.Core
	axis
}

// Column implements a vertical linear layout mark over layout/linear.
// Children are attached as real tree children so interactive marks receive
// input. Column draws no chrome; wrap it in a Card or provide a background.
type Column struct {
	marks.Core
	axis
}

var _ facet.FacetImpl = (*Row)(nil)
var _ marks.Mark = (*Row)(nil)
var _ facet.FacetImpl = (*Column)(nil)
var _ marks.Mark = (*Column)(nil)
var _ facet.FacetImpl = (*Divider)(nil)
var _ marks.Mark = (*Divider)(nil)

// NewRow constructs a horizontal linear layout mark.
func NewRow(children []AxisChild, cfg AxisConfig) *Row {
	r := &Row{axis: newAxis(true, children, cfg)}
	r.Facet = facet.NewFacet()
	r.AddBinding(r.Gap)
	r.AddBinding(r.PadX)
	r.AddBinding(r.PadY)
	r.AddBinding(r.CrossAlign)
	initAxisContracts(r, &r.Core, &r.axis)
	r.RegisterRoles()
	return r
}

// NewColumn constructs a vertical linear layout mark.
func NewColumn(children []AxisChild, cfg AxisConfig) *Column {
	c := &Column{axis: newAxis(false, children, cfg)}
	c.Facet = facet.NewFacet()
	c.AddBinding(c.Gap)
	c.AddBinding(c.PadX)
	c.AddBinding(c.PadY)
	c.AddBinding(c.CrossAlign)
	initAxisContracts(c, &c.Core, &c.axis)
	c.RegisterRoles()
	return c
}

// Base satisfies facet.FacetImpl.
func (r *Row) Base() *facet.Facet {
	r.BindImpl(r)
	return &r.Facet
}

// Descriptor satisfies marks.Mark.
func (r *Row) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: familyName, TypeName: "row"}
}

// measure delegates to the shared axis implementation.
func (r *Row) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	return r.axis.measure(&r.Core, ctx, constraints)
}

// arrange delegates to the shared axis implementation.
func (r *Row) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	r.Layout.ArrangedBounds = bounds
	r.axis.arrange(&r.Core, ctx, bounds)
}

// AccessibilityRole reports the semantic role.
func (r *Row) AccessibilityRole() string { return "group" }

// Children returns the row's immediate child facets with linear placement.
func (r *Row) Children() []facet.GroupChild {
	if r == nil {
		return nil
	}
	return r.axis.groupChildren()
}

// OnAttach attaches the row's children to the facet tree.
func (r *Row) OnAttach(ctx facet.AttachContext) {
	r.Core.OnAttach(ctx)
	attachAxisChildren(&r.Core, r.children)
}
func (r *Row) OnActivate()   { r.Core.OnActivate() }
func (r *Row) OnDeactivate() { r.Core.OnDeactivate() }

// OnDetach clears cached state; the runtime disposes the tree children.
func (r *Row) OnDetach() { r.Core.OnDetach() }

// Base satisfies facet.FacetImpl.
func (c *Column) Base() *facet.Facet {
	c.BindImpl(c)
	return &c.Facet
}

// Descriptor satisfies marks.Mark.
func (c *Column) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: familyName, TypeName: "column"}
}

// measure delegates to the shared axis implementation.
func (c *Column) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	return c.axis.measure(&c.Core, ctx, constraints)
}

// arrange delegates to the shared axis implementation.
func (c *Column) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	c.Layout.ArrangedBounds = bounds
	c.axis.arrange(&c.Core, ctx, bounds)
}

// AccessibilityRole reports the semantic role.
func (c *Column) AccessibilityRole() string { return "group" }

// Children returns the column's immediate child facets with linear placement.
func (c *Column) Children() []facet.GroupChild {
	if c == nil {
		return nil
	}
	return c.axis.groupChildren()
}

// OnAttach attaches the column's children to the facet tree.
func (c *Column) OnAttach(ctx facet.AttachContext) {
	c.Core.OnAttach(ctx)
	attachAxisChildren(&c.Core, c.children)
}
func (c *Column) OnActivate()   { c.Core.OnActivate() }
func (c *Column) OnDeactivate() { c.Core.OnDeactivate() }

// OnDetach clears cached state; the runtime disposes the tree children.
func (c *Column) OnDetach() { c.Core.OnDetach() }

// Divider is a themed stroke separator. Its stroke orientation follows the
// parent axis: inside a Row it is a vertical stroke, inside a Column a
// horizontal stroke; standalone it defaults to horizontal. The stroke fills
// the cross axis (stretch) and is Thickness deep on the main axis.
type Divider struct {
	marks.Core

	// Thickness is the stroke depth on the divider's main axis, in dp.
	// Values <= 0 clamp to 1.
	Thickness marks.Binding[float32]
	// Color is the stroke color. The zero color resolves to the theme's
	// OnSurfaceVariant token at projection time.
	Color marks.Binding[gfx.Color]

	cachedTokens theme.Tokens
}

// NewDivider constructs a themed divider stroke.
func NewDivider() *Divider {
	d := &Divider{
		Thickness: marks.Const(float32(1)),
		Color:     marks.Const(gfx.Color{}),
	}
	d.Facet = facet.NewFacet()
	d.AddBinding(d.Thickness)
	d.AddBinding(d.Color)

	// Divider is a leaf: no group parent contract, no children.
	d.Layout.Parent = facet.GroupParentContract{Kind: facet.GroupLayoutNone, Overflow: facet.OverflowClip}
	d.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := d.measure(ctx, constraints).Size
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionClip,
			BelowMinHeight: facet.CompressionClip,
			AboveMaxWidth:  facet.ExpansionClip,
			AboveMaxHeight: facet.ExpansionClip,
		},
		// The divider fills its cross axis through measurement (cross extent
		// comes from the parent's constraints) and hugs its main axis at
		// Thickness. Main-axis stretch is deliberately Never: a divider must
		// never absorb free main-axis space. Do not place a divider with
		// CrossAxisStretch — its measured cross size already fills the host.
		Stretch: facet.StretchPolicy{
			Width:  facet.StretchNever,
			Height: facet.StretchNever,
		},
		Baseline: facet.BaselineNone,
	}
	d.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return d.measure(ctx, constraints)
	}
	d.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		d.Layout.ArrangedBounds = bounds
	}
	d.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return d.buildCommands(d.Layout.ArrangedBounds)
	}
	d.RegisterRoles()
	return d
}

// Base satisfies facet.FacetImpl.
func (d *Divider) Base() *facet.Facet {
	d.BindImpl(d)
	return &d.Facet
}

// Descriptor satisfies marks.Mark.
func (d *Divider) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: familyName, TypeName: "divider"}
}

// Children is a facet.ChildSource conformance stub; the divider is a leaf.
//
// nolint:LL031
func (d *Divider) Children() []facet.GroupChild { return nil }

// AccessibilityRole reports the semantic role.
func (d *Divider) AccessibilityRole() string { return "separator" }

// strokeVertical reports whether the divider draws a vertical stroke, from
// the parent axis at attach time. A divider inside a Row (horizontal parent
// axis) is a vertical stroke; anywhere else it is horizontal.
func (d *Divider) strokeVertical() bool {
	parent := d.Facet.Parent()
	if parent == nil {
		return false
	}
	role := parent.LayoutRole()
	if role == nil {
		return false
	}
	return role.Parent.Kind == facet.GroupLayoutLinearHorizontal
}

// measure sizes the divider: Thickness on its main axis, the parent's cross
// extent (via constraints) on the cross axis.
func (d *Divider) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	d.cachedTokens = resolved.TokenSet()

	thickness := d.Thickness.Get()
	if thickness <= 0 {
		thickness = 1
	}
	var size gfx.Size
	if d.strokeVertical() {
		size = gfx.Size{W: thickness, H: constraints.MaxSize.H}
	} else {
		size = gfx.Size{W: constraints.MaxSize.W, H: thickness}
	}
	size = constraints.Constrain(size)
	d.Layout.MeasuredSize = size
	d.Layout.MeasuredResult = facet.MeasureResult{
		Size:        size,
		Intrinsic:   facet.IntrinsicSize{Min: size, Preferred: size, Max: size},
		Constraints: constraints,
	}
	return d.Layout.MeasuredResult
}

func (d *Divider) OnAttach(ctx facet.AttachContext) { d.Core.OnAttach(ctx) }
func (d *Divider) OnActivate()                      { d.Core.OnActivate() }
func (d *Divider) OnDeactivate()                    { d.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (d *Divider) OnDetach() {
	d.Core.OnDetach()
	d.cachedTokens = theme.Tokens{}
}

// buildCommands fills the arranged bounds with the resolved stroke color. The
// measure step already shapes the bounds (Thickness deep on the main axis),
// so the stroke orientation falls out of the bounds geometry.
func (d *Divider) buildCommands(bounds gfx.Rect) []gfx.Command {
	if d == nil || bounds.IsEmpty() {
		return nil
	}
	color := d.Color.Get()
	if color == (gfx.Color{}) {
		color = d.cachedTokens.Color.OnSurfaceVariant
	}
	if color == (gfx.Color{}) {
		return nil
	}

	var cmds []gfx.Command
	if color.A > 0 && color.A < 1 {
		cmds = append(cmds, gfx.PushOpacity{Alpha: color.A})
	}
	cmds = append(cmds, theme.MaterialCommands(
		gfx.RectPath(bounds),
		theme.Material{
			Fills: []theme.Fill{{
				Type:    theme.FillSolid,
				Color:   gfx.Color{R: color.R, G: color.G, B: color.B, A: 1},
				Opacity: 1,
			}},
			Opacity: 1,
		})...)
	if color.A > 0 && color.A < 1 {
		cmds = append(cmds, gfx.PopOpacity{})
	}
	return cmds
}
