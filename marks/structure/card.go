package structure

import (
	"math"
	"reflect"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	layoutgrid "codeburg.org/lexbit/lurpicui/layout/grid"
	"codeburg.org/lexbit/lurpicui/marks"
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
	marks.Core

	Label       marks.Binding[string]
	Disabled    marks.Binding[bool]
	LayoutMode  marks.Binding[CardLayoutMode]
	GridColumns marks.Binding[int]
	GridRows    marks.Binding[int]

	ChildrenContent []CardChild

	textRole facet.TextRole

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
var _ marks.Mark = (*Card)(nil)

// NewCard constructs a structure.card mark with canonical defaults.
func NewCard(label string) *Card {
	c := &Card{
		Label:       marks.Const(label),
		Disabled:    marks.Const(false),
		LayoutMode:  marks.Const(CardLayoutGrid),
		GridColumns: marks.Const(3),
		GridRows:    marks.Const(3),
	}
	c.Core.Facet = facet.NewFacet()
	c.AddBinding(c.Label)
	c.AddBinding(c.Disabled)
	c.AddBinding(c.LayoutMode)
	c.AddBinding(c.GridColumns)
	c.AddBinding(c.GridRows)

	c.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutGrid,
		Policy:   cardGroupPolicy{card: c},
		Children: c,
	}
	c.Layout.Child = facet.GroupChildContract{
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
	c.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return c.measure(ctx, constraints)
	}
	c.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		c.Layout.ArrangedBounds = bounds
		c.arrange(ctx, bounds)
	}
	c.Render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := c.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	c.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return c.buildCommands(c.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	c.textRole.IMEEnabled = false
	c.RegisterRoles()
	c.AddRole(&c.textRole)
	return c
}

// Base satisfies facet.FacetImpl.
func (c *Card) Base() *facet.Facet {
	c.Facet.BindImpl(c)
	return &c.Facet
}

// Descriptor satisfies marks.Mark.
func (c *Card) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "structure", TypeName: "card"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (c *Card) AccessibilityRole() string { return "group" }

// AccessibleName reports the semantic name source required by the spec.
func (c *Card) AccessibleName() string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Label.Get())
}

// ExportAnchors publishes the card anchor set.
func (c *Card) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if c == nil {
		return nil
	}
	bounds := c.Layout.ArrangedBounds
	out := c.Core.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
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
			out["baseline"] = b.Min
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
		markID := spec.MarkID
		if markID == 0 {
			markID = cardMarkIDFirstChild + facet.MarkID(i)
		}
		out = append(out, facet.GroupChild{
			FacetID:    base.ID(),
			MarkID:     markID,
			Attachment: c.gridChildAttachment(spec, i, len(specs)),
			Layout:     base.LayoutRole(),
			Contract:   base.LayoutRole().Child,
		})
	}
	return out
}

func (c *Card) OnAttach(ctx facet.AttachContext) { c.Core.OnAttach() }
func (c *Card) OnActivate()                      { c.Core.OnActivate() }
func (c *Card) OnDeactivate()                    { c.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (c *Card) OnDetach() {
	c.Core.OnDetach()
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
	c.Facet.Invalidate(flags)
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
	switch c.LayoutMode.Get() {
	case CardLayoutVertical:
		return facet.GridPlacement{ColStart: 0, RowStart: index, ColSpan: 1, RowSpan: 1}
	case CardLayoutHorizontal:
		return facet.GridPlacement{ColStart: index, RowStart: 0, ColSpan: 1, RowSpan: 1}
	default:
		cols := c.GridColumns.Get()
		if cols < 1 {
			cols = 3
		}
		rows := c.GridRows.Get()
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
	c.Layout.MeasuredSize = measured
	c.Layout.MeasuredResult = facet.MeasureResult{
		Size: measured,
		Intrinsic: facet.IntrinsicSize{
			Min:       measured,
			Preferred: measured,
			Max:       measured,
		},
		Constraints: constraints,
	}
	c.textRole.Layout = nil
	return c.Layout.MeasuredResult
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
		base.LayoutRole().Measure(ctx, facet.Constraints{MaxSize: constraints.MaxSize})
		out = append(out, c.gridLayoutChild(spec, i, len(children)))
	}
	return out
}

func (c *Card) gridConfig(childCount int, resolved theme.ResolvedContext) layoutgrid.Config {
	columns := c.GridColumns.Get()
	rows := c.GridRows.Get()
	if columns < 1 {
		columns = 3
	}
	if rows < 1 {
		rows = 3
	}
	switch c.LayoutMode.Get() {
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
	c.Layout.ArrangedBounds = bounds
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
		out = append(out, c.gridLayoutChild(spec, i, len(children)))
	}
	return out
}

func (c *Card) gridChildAttachment(spec CardChild, index, count int) facet.Attachment {
	placement := spec.Grid
	if placement == (facet.GridPlacement{}) {
		placement = c.defaultGridPlacement(index, count)
	}
	return facet.Attachment{
		Placement: facet.Placement{
			Mode: facet.PlacementGrid,
			Grid: placement,
		},
		ZPriority: spec.ZPriority,
	}
}

func (c *Card) gridLayoutChild(spec CardChild, index, count int) layoutgrid.Child {
	if spec.Facet == nil {
		return layoutgrid.Child{}
	}
	base := spec.Facet.Base()
	if base == nil || base.LayoutRole() == nil {
		return layoutgrid.Child{}
	}
	return layoutgrid.Child{
		FacetID:    base.ID(),
		Attachment: c.gridChildAttachment(spec, index, count),
		Layout:     base.LayoutRole(),
		Contract:   base.LayoutRole().Child,
	}
}

func (c *Card) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if c == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := c.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := theme.StateDefault
	if c.Disabled.Get() {
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
	if rt, ok := runtime.(ProjectionRuntime); ok {
		if store := resolveStyleContext(rt, c.Base().ID()); store != nil {
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
	services, ok := runtime.(facet.RuntimeServices)
	if !ok {
		return nil
	}
	v := reflect.ValueOf(services)
	switch v.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		if v.IsNil() {
			return nil
		}
	}
	return services
}

func isTransparentMaterial(material theme.Material) bool {
	return theme.IsTransparentMaterial(material)
}

func materialCommands(path gfx.Path, material theme.Material) []gfx.Command {
	return theme.MaterialCommands(path, material)
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
