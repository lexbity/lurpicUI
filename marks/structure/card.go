package structure

import (
	"math"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	layoutgrid "codeburg.org/lexbit/lurpicui/layout/grid"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uistruct"
)

const (
	cardMarkIDRoot       facet.MarkID = 1
	cardMarkIDSurface    facet.MarkID = 2
	cardMarkIDFirstChild facet.MarkID = 3
)

// CardLayoutMode controls how child facets are arranged inside the card.
type CardLayoutMode uint8

const (
	// CardLayoutGrid arranges children in a grid, defaulting to 3x3.
	CardLayoutGrid CardLayoutMode = iota
	// CardLayoutVertical arranges children as a single-column grid.
	CardLayoutVertical
	// CardLayoutHorizontal arranges children as a single-row grid.
	CardLayoutHorizontal
)

// CardChild describes one reusable child facet placed inside the card shell.
type CardChild struct {
	Key       string
	Facet     facet.FacetImpl
	MarkID    facet.MarkID
	Grid      facet.GridPlacement
	ZPriority int32
}

// Card implements the structure.card canonical mark.
type Card struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	textRole       facet.TextRole

	Label string

	Disabled bool

	LayoutMode  CardLayoutMode
	GridColumns int
	GridRows    int

	ChildrenContent []CardChild

	cachedTokens     theme.Tokens
	cachedRecipe     shared.CardSlots
	cachedBounds     gfx.Rect
	cachedRadius     float32
	cachedPadX       float32
	cachedPadY       float32
	cachedColumnGap  float32
	cachedRowGap     float32
	cachedWritingDir facet.WritingDirection

	cachedChildBounds map[facet.FacetID]gfx.Rect
}

var _ facet.FacetImpl = (*Card)(nil)
var _ layout.AnchorExporter = (*Card)(nil)

// NewCard constructs a structure.card mark with canonical defaults.
func NewCard(label string) *Card {
	c := &Card{
		Facet:       facet.NewFacet(),
		Label:       label,
		LayoutMode:  CardLayoutGrid,
		GridColumns: 3,
		GridRows:    3,
	}
	c.layoutRole.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutGrid,
		Policy:   cardGroupPolicy{card: c},
		Children: c,
	}
	c.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := c.measure(ctx, constraints).Size
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
			Height: facet.StretchNever,
		},
		Baseline: facet.BaselineNone,
	}
	c.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return c.measure(ctx, constraints)
	}
	c.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		c.layoutRole.ArrangedBounds = bounds
		c.arrange(ctx, bounds)
	}
	c.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := c.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	c.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := c.buildCommands(c.layoutRole.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	c.textRole.IMEEnabled = false
	c.AddRole(&c.layoutRole)
	c.AddRole(&c.renderRole)
	c.AddRole(&c.projectionRole)
	c.AddRole(&c.textRole)
	return c
}

// Base satisfies facet.FacetImpl.
func (c *Card) Base() *facet.Facet {
	c.Facet.BindImpl(c)
	return &c.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (c *Card) AccessibilityRole() string { return "group" }

// AccessibleName reports the semantic name source required by the spec.
func (c *Card) AccessibleName() string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Label)
}

// SetLabel updates the authored accessible label.
func (c *Card) SetLabel(label string) {
	if c == nil || c.Label == label {
		return
	}
	c.Label = label
	c.invalidate(facet.DirtyProjection)
}

// SetDisabled toggles disabled state.
func (c *Card) SetDisabled(disabled bool) {
	if c == nil || c.Disabled == disabled {
		return
	}
	c.Disabled = disabled
	c.invalidate(facet.DirtyProjection)
}

// SetLayoutMode updates the local group composition mode.
func (c *Card) SetLayoutMode(mode CardLayoutMode) {
	if c == nil || c.LayoutMode == mode {
		return
	}
	c.LayoutMode = mode
	c.invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// SetGrid defines the default grid tracks for the card body.
func (c *Card) SetGrid(columns, rows int) {
	if c == nil {
		return
	}
	if columns < 1 {
		columns = 1
	}
	if rows < 1 {
		rows = 1
	}
	if c.GridColumns == columns && c.GridRows == rows {
		return
	}
	c.GridColumns = columns
	c.GridRows = rows
	c.invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// SetChildren updates the reusable child facet list.
func (c *Card) SetChildren(children []CardChild) {
	if c == nil {
		return
	}
	next := append([]CardChild(nil), children...)
	for i := range next {
		next[i].Key = strings.TrimSpace(next[i].Key)
	}
	c.ChildrenContent = next
	c.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// ExportAnchors publishes the card anchor set.
func (c *Card) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if c == nil {
		return nil
	}
	bounds := c.layoutRole.ArrangedBounds
	if bounds.IsEmpty() && !ctx.ResolvedLayer.Bounds.IsEmpty() {
		bounds = ctx.ResolvedLayer.Bounds
	}
	if bounds.IsEmpty() {
		return nil
	}
	out := layout.AnchorSet{
		"bounds_center":       gfx.Point{X: (bounds.Min.X + bounds.Max.X) * 0.5, Y: (bounds.Min.Y + bounds.Max.Y) * 0.5},
		"bounds_top_left":     bounds.Min,
		"bounds_top_right":    gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y},
		"bounds_bottom_left":  gfx.Point{X: bounds.Min.X, Y: bounds.Max.Y},
		"bounds_bottom_right": gfx.Point{X: bounds.Max.X, Y: bounds.Max.Y},
	}
	active := c.activeChildren()
	for i := range active {
		spec := active[i]
		if spec.Facet == nil || spec.Key == "" {
			continue
		}
		b, ok := c.cachedChildBounds[spec.Facet.Base().ID()]
		if !ok || b.IsEmpty() {
			continue
		}
		out[layout.AnchorID("child_"+spec.Key)] = gfx.Point{
			X: (b.Min.X + b.Max.X) * 0.5,
			Y: (b.Min.Y + b.Max.Y) * 0.5,
		}
	}
	out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	for _, b := range c.cachedChildBounds {
		if !b.IsEmpty() {
			out["baseline"] = gfx.Point{X: b.Min.X, Y: b.Min.Y}
			break
		}
	}
	return out
}

// Children returns the immediate child facet list.
func (c *Card) Children() []facet.GroupChild {
	if c == nil {
		return nil
	}
	specs := c.activeChildren()
	out := make([]facet.GroupChild, 0, len(specs))
	for i := range specs {
		spec := specs[i]
		if spec.Facet == nil {
			continue
		}
		base := spec.Facet.Base()
		if base == nil || base.LayoutRole() == nil {
			continue
		}
		placement := spec.Grid
		if placement == (facet.GridPlacement{}) {
			placement = c.defaultGridPlacement(i, len(specs))
		}
		markID := spec.MarkID
		if markID == 0 {
			markID = cardMarkIDFirstChild + facet.MarkID(i)
		}
		out = append(out, facet.GroupChild{
			FacetID: base.ID(),
			MarkID:  markID,
			Attachment: facet.Attachment{
				Placement: facet.Placement{
					Mode: facet.PlacementGrid,
					Grid: placement,
				},
				ZPriority: spec.ZPriority,
			},
			Layout:   base.LayoutRole(),
			Contract: base.LayoutRole().Child,
		})
	}
	return out
}

// OnAttach is unused.
func (c *Card) OnAttach(ctx facet.AttachContext) {}

// OnActivate is unused.
func (c *Card) OnActivate() {}

// OnDeactivate is unused.
func (c *Card) OnDeactivate() {}

// OnDetach clears cached projection state.
func (c *Card) OnDetach() {
	c.cachedTokens = theme.Tokens{}
	c.cachedRecipe = shared.CardSlots{}
	c.cachedBounds = gfx.Rect{}
	c.cachedRadius = 0
	c.cachedPadX = 0
	c.cachedPadY = 0
	c.cachedColumnGap = 0
	c.cachedRowGap = 0
	c.cachedChildBounds = nil
}

func (c *Card) invalidate(flags facet.DirtyFlags) {
	if c == nil {
		return
	}
	c.Base().Invalidate(flags)
}

func (c *Card) activeChildren() []CardChild {
	if c == nil {
		return nil
	}
	return append([]CardChild(nil), c.ChildrenContent...)
}

func (c *Card) defaultGridPlacement(index, count int) facet.GridPlacement {
	if count <= 0 {
		return facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1}
	}
	switch c.LayoutMode {
	case CardLayoutVertical:
		return facet.GridPlacement{ColStart: 0, RowStart: index, ColSpan: 1, RowSpan: 1}
	case CardLayoutHorizontal:
		return facet.GridPlacement{ColStart: index, RowStart: 0, ColSpan: 1, RowSpan: 1}
	default:
		cols := c.GridColumns
		if cols < 1 {
			cols = 3
		}
		rows := c.GridRows
		if rows < 1 {
			rows = 3
		}
		if count > cols*rows {
			rows = int(math.Ceil(float64(count) / float64(cols)))
		}
		_ = rows
		return facet.GridPlacement{ColStart: index % cols, RowStart: index / cols, ColSpan: 1, RowSpan: 1}
	}
}

func (c *Card) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uistruct.ResolveCardRecipe(style)
	c.cachedTokens = resolved.TokenSet()
	c.cachedRecipe = slots
	c.cachedWritingDir = ctx.WritingDirection
	c.cachedPadX = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(16))
	c.cachedPadY = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(16))
	c.cachedColumnGap = float32(resolved.Spacing(theme.SpacingS))
	c.cachedRowGap = float32(resolved.Spacing(theme.SpacingS))
	c.cachedRadius = float32(resolved.Radius(theme.RadiusL))

	active := c.activeChildren()
	gridChildren := c.measureChildren(ctx, constraints, active)
	gridCfg := c.gridConfig(len(gridChildren), resolved)
	policy := layoutgrid.New(gridCfg)
	measured, err := policy.Measure(gridChildren, constraints.MaxSize)
	if err != nil {
		measured = gfx.Size{}
	}
	measured.W += c.cachedPadX * 2
	measured.H += c.cachedPadY * 2
	if measured.W < resolved.Density.Scale(160) {
		measured.W = resolved.Density.Scale(160)
	}
	if measured.H < resolved.Density.Scale(120) {
		measured.H = resolved.Density.Scale(120)
	}
	measured = constraints.Constrain(measured)
	c.layoutRole.MeasuredSize = measured
	c.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: measured,
		Intrinsic: facet.IntrinsicSize{
			Min:       measured,
			Preferred: measured,
			Max:       measured,
		},
		Constraints: constraints,
	}
	c.textRole.Layout = nil
	return c.layoutRole.MeasuredResult
}

func (c *Card) measureChildren(ctx facet.MeasureContext, constraints facet.Constraints, children []CardChild) []layoutgrid.Child {
	out := make([]layoutgrid.Child, 0, len(children))
	for i := range children {
		spec := children[i]
		if spec.Facet == nil {
			continue
		}
		base := spec.Facet.Base()
		if base == nil || base.LayoutRole() == nil {
			continue
		}
		placement := spec.Grid
		if placement == (facet.GridPlacement{}) {
			placement = c.defaultGridPlacement(i, len(children))
		}
		base.LayoutRole().Measure(ctx, facet.Constraints{MaxSize: constraints.MaxSize})
		out = append(out, layoutgrid.Child{
			FacetID: base.ID(),
			Attachment: facet.Attachment{
				Placement: facet.Placement{
					Mode: facet.PlacementGrid,
					Grid: placement,
				},
				ZPriority: spec.ZPriority,
			},
			Layout:   base.LayoutRole(),
			Contract: base.LayoutRole().Child,
		})
	}
	return out
}

func (c *Card) gridConfig(childCount int, resolved theme.ResolvedContext) layoutgrid.Config {
	columns := c.GridColumns
	rows := c.GridRows
	if columns < 1 {
		columns = 3
	}
	if rows < 1 {
		rows = 3
	}
	switch c.LayoutMode {
	case CardLayoutVertical:
		columns = 1
		rows = maxInt(rows, maxInt(childCount, 1))
	case CardLayoutHorizontal:
		rows = 1
		columns = maxInt(columns, maxInt(childCount, 1))
	default:
		if childCount > columns*rows {
			rows = int(math.Ceil(float64(maxInt(childCount, 1)) / float64(columns)))
		}
	}
	return layoutgrid.Config{
		Columns:       flexibleTracks(columns),
		Rows:          flexibleTracks(rows),
		ColumnGap:     c.cachedColumnGap,
		RowGap:        c.cachedRowGap,
		AutoPlacement: layoutgrid.AutoRowFirst,
	}
}

func flexibleTracks(count int) []layoutgrid.TrackDef {
	if count < 1 {
		count = 1
	}
	out := make([]layoutgrid.TrackDef, count)
	for i := range out {
		out[i] = layoutgrid.TrackDef{Sizing: layoutgrid.TrackFlex, Value: 1, Min: 0}
	}
	return out
}

func (c *Card) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	c.cachedBounds = bounds
	c.cachedChildBounds = map[facet.FacetID]gfx.Rect{}
	c.layoutRole.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	inner := bounds.Inset(c.cachedPadX, c.cachedPadY)
	if inner.IsEmpty() {
		inner = bounds
	}
	active := c.activeChildren()
	gridChildren := c.arrangeChildren(ctx, inner, active)
	policy := layoutgrid.New(c.gridConfig(len(gridChildren), theme.DefaultResolvedContext()))
	arranged, err := policy.Arrange(gridChildren, inner)
	if err != nil {
		return
	}
	c.cachedChildBounds = make(map[facet.FacetID]gfx.Rect, len(arranged))
	for _, child := range arranged {
		c.cachedChildBounds[child.FacetID] = child.Bounds
	}
}

func (c *Card) arrangeChildren(ctx facet.ArrangeContext, bounds gfx.Rect, children []CardChild) []layoutgrid.Child {
	out := make([]layoutgrid.Child, 0, len(children))
	for i := range children {
		spec := children[i]
		if spec.Facet == nil {
			continue
		}
		base := spec.Facet.Base()
		if base == nil || base.LayoutRole() == nil {
			continue
		}
		placement := spec.Grid
		if placement == (facet.GridPlacement{}) {
			placement = c.defaultGridPlacement(i, len(children))
		}
		out = append(out, layoutgrid.Child{
			FacetID: base.ID(),
			Attachment: facet.Attachment{
				Placement: facet.Placement{
					Mode: facet.PlacementGrid,
					Grid: placement,
				},
				ZPriority: spec.ZPriority,
			},
			Layout:   base.LayoutRole(),
			Contract: base.LayoutRole().Child,
		})
	}
	return out
}

func (c *Card) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if c == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := c.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := theme.StateDefault
	if c.Disabled {
		state = theme.StateDisabled
	}
	root := slots.Root.Resolve(state, tokens)
	surface := slots.CardSurface.Resolve(state, tokens)

	cmds := make([]gfx.Command, 0, 32)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(surface) {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(bounds, c.cachedRadius), surface)...)
	}
	active := c.activeChildren()
	for i := range active {
		spec := active[i]
		if spec.Facet == nil {
			continue
		}
		b, ok := c.cachedChildBounds[spec.Facet.Base().ID()]
		if !ok || b.IsEmpty() {
			continue
		}
		if childCmds := spec.Facet.Base().ProjectionRole().Project(facet.ProjectionContext{
			Runtime:      runtimeServicesOrNil(runtime),
			Bounds:       b,
			ContentScale: contentScale,
		}); childCmds != nil {
			cmds = append(cmds, childCmds.Commands...)
		}
	}
	return cmds
}

func (c *Card) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.CardSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: c.cachedTokens}, c.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, c.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uistruct.ResolveCardRecipe(style)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: c.cachedTokens}, c.cachedRecipe
}

type cardGroupPolicy struct {
	card *Card
}

func (cardGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutGrid }

func (p cardGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.card == nil {
		return facet.GroupMeasureResult{}, nil
	}
	size := p.card.measure(ctx.MeasureContext, facet.Constraints{
		MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()},
	}).Size
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p cardGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.card == nil {
		return nil, nil
	}
	p.card.arrange(ctx.ArrangeContext, ctx.Bounds)
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
			ZPriority: child.Attachment.ZPriority,
			Contract:  child.Contract,
		})
	}
	return arranged, nil
}

func runtimeServicesOrNil(runtime any) facet.RuntimeServices {
	if runtime == nil {
		return nil
	}
	if services, ok := runtime.(facet.RuntimeServices); ok {
		return services
	}
	return nil
}

func isTransparentMaterial(material theme.Material) bool {
	return material.Opacity <= 0
}

func materialCommands(path gfx.Path, material theme.Material) []gfx.Command {
	if isTransparentMaterial(material) {
		return nil
	}
	cmds := make([]gfx.Command, 0, 2)
	for _, fill := range material.Fills {
		if fill.Type == theme.FillNone {
			continue
		}
		cmds = append(cmds, gfx.FillPath{Path: path, Brush: gfx.Brush{Color: fill.Color}})
	}
	for _, stroke := range material.Strokes {
		cmds = append(cmds, gfx.StrokePath{Path: path, Brush: gfx.Brush{Color: stroke.Paint.Color}, Stroke: gfx.DefaultStroke(stroke.Width)})
	}
	return cmds
}

func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
