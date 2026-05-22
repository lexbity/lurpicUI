package action

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiaction"
)

const (
	toolbarMarkIDRoot         facet.MarkID = 1
	toolbarMarkIDSurface      facet.MarkID = 2
	toolbarMarkIDActionItems  facet.MarkID = 3
	toolbarMarkIDGroups       facet.MarkID = 4
	toolbarMarkIDSeparators   facet.MarkID = 5
	toolbarMarkIDOverflowMenu facet.MarkID = 6
	toolbarMarkIDFocusRing    facet.MarkID = 7
)

// ToolbarGroup describes one grouped cluster of action toolbar items.
type ToolbarGroup struct {
	Key     string
	Actions []ActionGroupAction
}

// ToolbarOverflow describes the trailing overflow menu for a toolbar.
type ToolbarOverflow struct {
	AccessibleLabel string
	TriggerIconRef  string
	Entries         []MenuButtonEntry
}

// Toolbar implements the action.toolbar standard mark.
type Toolbar struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	focusRole      facet.FocusRole
	textRole       facet.TextRole

	Activated signal.Signal[string]

	Label    string
	Groups   []ToolbarGroup
	Overflow *ToolbarOverflow
	Disabled bool

	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool
	focusedIndex     int
	hoveredIndex     int
	pressedIndex     int

	cachedTokens           theme.Tokens
	cachedRecipe           shared.ToolbarSlots
	cachedRootBounds       gfx.Rect
	cachedSurfaceBounds    gfx.Rect
	cachedChildBounds      []gfx.Rect
	cachedSeparatorBounds  []gfx.Rect
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedGroupGap         float32
	cachedRadius           float32
	cachedWritingDirection facet.WritingDirection
	cachedChildren         []*toolbarChild
}

type toolbarChildKind uint8

const (
	toolbarChildGroup toolbarChildKind = iota
	toolbarChildOverflow
)

type toolbarChild struct {
	parent   *Toolbar
	index    int
	kind     toolbarChildKind
	group    *ActionGroup
	overflow *MenuButton
	subID    signal.SubscriptionID
}

var _ facet.FacetImpl = (*Toolbar)(nil)
var _ layout.AnchorExporter = (*Toolbar)(nil)

// NewToolbar constructs an action.toolbar mark with canonical defaults.
func NewToolbar(label string, groups []ToolbarGroup, overflow *ToolbarOverflow) *Toolbar {
	t := &Toolbar{
		Facet:        facet.NewFacet(),
		Label:        label,
		Groups:       normalizeToolbarGroups(groups),
		Overflow:     normalizeToolbarOverflow(overflow),
		focusedIndex: -1,
		hoveredIndex: -1,
		pressedIndex: -1,
		Activated:    signal.NewSignal[string]("toolbar_activated"),
	}
	t.layoutRole.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearHorizontal,
		Policy: toolbarGroupPolicy{toolbar: t},
	}
	t.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsRadial,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := t.measureIntrinsic(ctx, constraints)
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionTruncate,
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
	t.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return t.measure(ctx, constraints)
	}
	t.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		t.layoutRole.ArrangedBounds = bounds
		t.arrange(ctx, bounds)
	}
	t.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := t.buildCommands(bounds, nil)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	t.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := t.buildCommands(t.layoutRole.ArrangedBounds, ctx.Runtime)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	t.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult {
		return t.hitTest(p)
	}
	t.inputRole.OnPointer = func(e facet.PointerEvent) bool {
		return t.onPointer(e)
	}
	t.inputRole.OnKey = func(e facet.KeyEvent) bool {
		return t.onKey(e)
	}
	t.focusRole.Focusable = func() bool {
		return !t.Disabled && (len(t.Groups) > 0 || t.Overflow != nil)
	}
	t.focusRole.TabIndex = 0
	t.focusRole.OnFocusGained = func() { t.onFocusGained() }
	t.focusRole.OnFocusLost = func() { t.onFocusLost() }
	t.textRole.IMEEnabled = false
	t.AddRole(&t.layoutRole)
	t.AddRole(&t.renderRole)
	t.AddRole(&t.projectionRole)
	t.AddRole(&t.hitRole)
	t.AddRole(&t.inputRole)
	t.AddRole(&t.focusRole)
	t.AddRole(&t.textRole)
	t.syncChildren()
	return t
}

// Base satisfies facet.FacetImpl.
func (t *Toolbar) Base() *facet.Facet {
	t.Facet.BindImpl(t)
	return &t.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (t *Toolbar) AccessibilityRole() string { return "toolbar" }

// AccessibleName reports the semantic name required by the spec.
func (t *Toolbar) AccessibleName() string {
	if t == nil {
		return ""
	}
	if name := strings.TrimSpace(t.Label); name != "" {
		return name
	}
	for _, group := range t.Groups {
		for _, action := range group.Actions {
			if name := strings.TrimSpace(action.AccessibleLabel); name != "" {
				return name
			}
			if name := strings.TrimSpace(action.Label); name != "" {
				return name
			}
		}
	}
	if t.Overflow != nil {
		if name := strings.TrimSpace(t.Overflow.AccessibleLabel); name != "" {
			return name
		}
		for _, entry := range t.Overflow.Entries {
			if name := strings.TrimSpace(entry.AccessibleLabel); name != "" {
				return name
			}
			if name := strings.TrimSpace(entry.Label); name != "" {
				return name
			}
		}
	}
	return ""
}

// SetLabel updates the authored label text used for accessibility.
func (t *Toolbar) SetLabel(label string) {
	if t == nil || t.Label == label {
		return
	}
	t.Label = label
	t.invalidate(facet.DirtyProjection)
}

// SetGroups replaces the toolbar action groups.
func (t *Toolbar) SetGroups(groups []ToolbarGroup) {
	if t == nil {
		return
	}
	t.Groups = normalizeToolbarGroups(groups)
	t.syncChildren()
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetOverflow replaces the toolbar overflow menu.
func (t *Toolbar) SetOverflow(overflow *ToolbarOverflow) {
	if t == nil {
		return
	}
	t.Overflow = normalizeToolbarOverflow(overflow)
	t.syncChildren()
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetDisabled toggles disabled state.
func (t *Toolbar) SetDisabled(disabled bool) {
	if t == nil || t.Disabled == disabled {
		return
	}
	t.Disabled = disabled
	if disabled {
		t.hovered = false
		t.pressed = false
		t.focusedVisible = false
		t.focusFromPointer = false
		t.hoveredIndex = -1
		t.pressedIndex = -1
	}
	t.syncChildren()
	t.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// ExportAnchors publishes the toolbar anchor set.
func (t *Toolbar) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if t == nil {
		return nil
	}
	bounds := t.layoutRole.ArrangedBounds
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
	out["baseline"] = t.baselinePoint(bounds)
	return out
}

// Children returns the facet's immediate child list.
func (t *Toolbar) Children() []facet.GroupChild {
	if t == nil {
		return nil
	}
	t.syncChildren()
	out := make([]facet.GroupChild, 0, len(t.cachedChildren))
	for i := range t.cachedChildren {
		child := t.cachedChildren[i]
		if child == nil {
			continue
		}
		base := child.base()
		if base == nil {
			continue
		}
		layoutRole := base.LayoutRole()
		if layoutRole == nil {
			continue
		}
		markID := toolbarMarkIDGroups
		if child.kind == toolbarChildOverflow {
			markID = toolbarMarkIDOverflowMenu
		}
		out = append(out, facet.GroupChild{
			FacetID: base.ID(),
			MarkID:  markID,
			Attachment: facet.Attachment{
				Placement: facet.Placement{
					Mode:   facet.PlacementLinear,
					Linear: facet.LinearPlacement{Order: i, CrossAxisAlign: facet.CrossAxisStretch},
				},
			},
			Layout:   layoutRole,
			Contract: layoutRole.Child,
		})
	}
	return out
}

// OnAttach is unused.
func (t *Toolbar) OnAttach(ctx facet.AttachContext) {}

// OnActivate is unused.
func (t *Toolbar) OnActivate() {}

// OnDeactivate is unused.
func (t *Toolbar) OnDeactivate() {}

// OnDetach clears cached projection state.
func (t *Toolbar) OnDetach() {
	t.cachedTokens = theme.Tokens{}
	t.cachedRecipe = shared.ToolbarSlots{}
	t.cachedRootBounds = gfx.Rect{}
	t.cachedSurfaceBounds = gfx.Rect{}
	t.cachedChildBounds = nil
	t.cachedSeparatorBounds = nil
	t.cachedPadX = 0
	t.cachedPadY = 0
	t.cachedGap = 0
	t.cachedGroupGap = 0
	t.cachedRadius = 0
	t.cachedChildren = nil
}

func (t *Toolbar) invalidate(flags facet.DirtyFlags) {
	if t == nil {
		return
	}
	t.Facet.Invalidate(flags)
}

func (t *Toolbar) syncChildren() {
	if t == nil {
		return
	}
	specs := t.childSpecs()
	if len(t.cachedChildren) > len(specs) {
		for _, child := range t.cachedChildren[len(specs):] {
			if child != nil {
				child.dispose()
			}
		}
		t.cachedChildren = t.cachedChildren[:len(specs)]
	}
	if len(t.cachedChildren) < len(specs) {
		next := make([]*toolbarChild, len(specs))
		copy(next, t.cachedChildren)
		t.cachedChildren = next
	}
	for i := range specs {
		if t.cachedChildren[i] == nil {
			t.cachedChildren[i] = newToolbarChild(t, i, specs[i])
		}
		t.cachedChildren[i].index = i
		t.cachedChildren[i].setSpec(specs[i])
	}
	if len(t.cachedChildBounds) != len(t.cachedChildren) {
		t.cachedChildBounds = make([]gfx.Rect, len(t.cachedChildren))
	}
	if len(t.cachedChildren) == 0 {
		t.focusedIndex = -1
		return
	}
	if t.focusedIndex >= len(t.cachedChildren) {
		t.focusedIndex = len(t.cachedChildren) - 1
	}
	if t.focusedIndex < 0 {
		t.focusedIndex = t.firstEnabledChildIndex()
	}
}

func (t *Toolbar) childSpecs() []toolbarChildSpec {
	if t == nil {
		return nil
	}
	specs := make([]toolbarChildSpec, 0, len(t.Groups)+1)
	for _, group := range t.Groups {
		specs = append(specs, toolbarChildSpec{kind: toolbarChildGroup, group: normalizeToolbarGroup(group)})
	}
	if overflow := normalizeToolbarOverflow(t.Overflow); overflow != nil {
		specs = append(specs, toolbarChildSpec{kind: toolbarChildOverflow, overflow: overflow})
	}
	return specs
}

type toolbarChildSpec struct {
	kind     toolbarChildKind
	group    ToolbarGroup
	overflow *ToolbarOverflow
}

func newToolbarChild(parent *Toolbar, index int, spec toolbarChildSpec) *toolbarChild {
	child := &toolbarChild{parent: parent, index: index, kind: spec.kind}
	child.setSpec(spec)
	return child
}

func (c *toolbarChild) dispose() {
	if c == nil {
		return
	}
	switch c.kind {
	case toolbarChildGroup:
		if c.group != nil && c.subID != 0 {
			c.group.Activated.Unsubscribe(c.subID)
		}
	case toolbarChildOverflow:
		if c.overflow != nil && c.subID != 0 {
			c.overflow.Activated.Unsubscribe(c.subID)
		}
	}
	c.subID = 0
}

func (c *toolbarChild) base() *facet.Facet {
	if c == nil {
		return nil
	}
	switch c.kind {
	case toolbarChildGroup:
		if c.group != nil {
			return c.group.Base()
		}
	case toolbarChildOverflow:
		if c.overflow != nil {
			return c.overflow.Base()
		}
	}
	return nil
}

func (c *toolbarChild) setSpec(spec toolbarChildSpec) {
	if c == nil {
		return
	}
	switch c.kind {
	case toolbarChildGroup:
		if c.group == nil {
			c.group = NewActionGroup("", spec.group.Actions)
			c.group.focusRole.Focusable = func() bool { return false }
			c.subID = c.group.Activated.Subscribe(func(key string) {
				if c.parent != nil {
					c.parent.Activated.Emit(key)
				}
			})
		} else {
			c.group.SetLabel("")
			c.group.SetActions(spec.group.Actions)
		}
		c.group.SetDisabled(c.parent != nil && c.parent.Disabled)
	case toolbarChildOverflow:
		if c.overflow == nil {
			c.overflow = NewMenuButton("", spec.overflow.Entries)
			c.overflow.TriggerIconRef = spec.overflow.TriggerIconRef
			c.overflow.AccessibleLabel = spec.overflow.AccessibleLabel
			c.overflow.focusRole.Focusable = func() bool { return false }
			c.subID = c.overflow.Activated.Subscribe(func(key string) {
				if c.parent != nil {
					c.parent.Activated.Emit(key)
				}
			})
		} else {
			c.overflow.SetLabel("")
			c.overflow.SetAccessibleLabel(spec.overflow.AccessibleLabel)
			c.overflow.SetTriggerIconRef(spec.overflow.TriggerIconRef)
			c.overflow.SetEntries(spec.overflow.Entries)
		}
		c.overflow.SetDisabled(c.parent != nil && c.parent.Disabled)
	}
}

func (c *toolbarChild) measure(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	if c == nil {
		return gfx.Size{}
	}
	switch c.kind {
	case toolbarChildGroup:
		if c.group == nil || c.group.Base() == nil || c.group.Base().LayoutRole() == nil {
			return gfx.Size{}
		}
		return c.group.Base().LayoutRole().Measure(ctx, constraints).Size
	case toolbarChildOverflow:
		if c.overflow == nil || c.overflow.Base() == nil || c.overflow.Base().LayoutRole() == nil {
			return gfx.Size{}
		}
		return c.overflow.Base().LayoutRole().Measure(ctx, constraints).Size
	default:
		return gfx.Size{}
	}
}

func (c *toolbarChild) measureSize() gfx.Size {
	if c == nil {
		return gfx.Size{}
	}
	switch c.kind {
	case toolbarChildGroup:
		if c.group == nil || c.group.Base() == nil || c.group.Base().LayoutRole() == nil {
			return gfx.Size{}
		}
		return c.group.Base().LayoutRole().MeasuredSize
	case toolbarChildOverflow:
		if c.overflow == nil || c.overflow.Base() == nil || c.overflow.Base().LayoutRole() == nil {
			return gfx.Size{}
		}
		return c.overflow.Base().LayoutRole().MeasuredSize
	default:
		return gfx.Size{}
	}
}

func (c *toolbarChild) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if c == nil {
		return
	}
	switch c.kind {
	case toolbarChildGroup:
		if c.group == nil || c.group.Base() == nil || c.group.Base().LayoutRole() == nil {
			return
		}
		c.group.Base().LayoutRole().Arrange(ctx, bounds)
	case toolbarChildOverflow:
		if c.overflow == nil || c.overflow.Base() == nil || c.overflow.Base().LayoutRole() == nil {
			return
		}
		c.overflow.Base().LayoutRole().Arrange(ctx, bounds)
	}
}

func (c *toolbarChild) project(runtime facet.RuntimeServices, bounds gfx.Rect, contentScale float32) *gfx.CommandList {
	if c == nil || bounds.IsEmpty() {
		return nil
	}
	switch c.kind {
	case toolbarChildGroup:
		if c.group == nil || c.group.Base() == nil || c.group.Base().ProjectionRole() == nil {
			return nil
		}
		return c.group.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtime, Bounds: bounds, ContentScale: contentScale})
	case toolbarChildOverflow:
		if c.overflow == nil || c.overflow.Base() == nil || c.overflow.Base().ProjectionRole() == nil {
			return nil
		}
		return c.overflow.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtime, Bounds: bounds, ContentScale: contentScale})
	default:
		return nil
	}
}

func (c *toolbarChild) pointer(e facet.PointerEvent) bool {
	if c == nil {
		return false
	}
	switch c.kind {
	case toolbarChildGroup:
		if c.group == nil {
			return false
		}
		return c.group.onPointer(e)
	case toolbarChildOverflow:
		if c.overflow == nil {
			return false
		}
		return c.overflow.onPointer(e)
	default:
		return false
	}
}

func (c *toolbarChild) keyEvent(e facet.KeyEvent) bool {
	if c == nil {
		return false
	}
	switch c.kind {
	case toolbarChildGroup:
		if c.group == nil {
			return false
		}
		return c.group.onKey(e)
	case toolbarChildOverflow:
		if c.overflow == nil {
			return false
		}
		return c.overflow.onKey(e)
	default:
		return false
	}
}

func (c *toolbarChild) hitTest(p gfx.Point) facet.HitResult {
	if c == nil {
		return facet.HitResult{}
	}
	switch c.kind {
	case toolbarChildGroup:
		if c.group == nil {
			return facet.HitResult{}
		}
		return c.group.hitTest(p)
	case toolbarChildOverflow:
		if c.overflow == nil {
			return facet.HitResult{}
		}
		return c.overflow.hitTest(p)
	default:
		return facet.HitResult{}
	}
}

func (c *toolbarChild) focusedIndex() int {
	if c == nil {
		return -1
	}
	switch c.kind {
	case toolbarChildGroup:
		if c.group == nil {
			return -1
		}
		return c.group.focusedIndex
	case toolbarChildOverflow:
		if c.overflow == nil {
			return -1
		}
		return c.overflow.focusedIndex
	default:
		return -1
	}
}

func (c *toolbarChild) setFocusedIndex(index int) {
	if c == nil {
		return
	}
	switch c.kind {
	case toolbarChildGroup:
		if c.group == nil {
			return
		}
		c.group.focusedIndex = index
	case toolbarChildOverflow:
		if c.overflow == nil {
			return
		}
		c.overflow.focusedIndex = index
	}
}

func (c *toolbarChild) hasActiveItem() bool {
	if c == nil {
		return false
	}
	switch c.kind {
	case toolbarChildGroup:
		if c.group == nil {
			return false
		}
		for i := range c.group.cachedItemLayouts {
			if c.group.cachedItemLayouts[i].item.Active {
				return true
			}
		}
	case toolbarChildOverflow:
		if c.overflow == nil {
			return false
		}
		for i := range c.overflow.cachedEntryLayouts {
			if c.overflow.cachedEntryLayouts[i].entry.Selected {
				return true
			}
		}
		if c.overflow.Open {
			return true
		}
	}
	return false
}

func (t *Toolbar) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, recipe, ok := t.resolveTheme(ctx)
	if !ok {
		t.textRole.Layout = nil
		return facet.MeasureResult{}
	}
	t.cachedTokens = resolved.TokenSet()
	t.cachedRecipe = recipe
	t.cachedWritingDirection = ctx.WritingDirection
	t.cachedPadX = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	t.cachedPadY = maxFloat(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	t.cachedGap = maxFloat(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	t.cachedGroupGap = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	t.cachedRadius = float32(resolved.Radius(theme.RadiusM))

	t.syncChildren()
	if len(t.cachedChildren) == 0 {
		t.layoutRole.MeasuredResult = facet.MeasureResult{}
		return facet.MeasureResult{}
	}
	childSizes := make([]gfx.Size, len(t.cachedChildren))
	maxChildH := float32(0)
	for i := range t.cachedChildren {
		child := t.cachedChildren[i]
		if child == nil {
			continue
		}
		size := child.measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: constraints.MaxSize.W, H: constraints.MaxSize.H}})
		childSizes[i] = size
		if size.H > maxChildH {
			maxChildH = size.H
		}
	}

	segments := make([]layout.InlineFlowSegment, 0, len(childSizes))
	for i, size := range childSizes {
		segment := layout.InlineFlowSegment{Size: size}
		if i < len(childSizes)-1 {
			segment.GapAfter = t.cachedGroupGap
		}
		segments = append(segments, segment)
	}
	content := layout.InlineFlowSegmentsSize(segments)
	minHeight := maxFloat(resolved.Density.Scale(40), maxChildH+t.cachedPadY*2)
	size := gfx.Size{
		W: maxFloat(resolved.Density.Scale(120), content.W+t.cachedPadX*2),
		H: minHeight,
	}
	size = constraints.Constrain(size)
	t.layoutRole.MeasuredSize = size
	t.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	return t.layoutRole.MeasuredResult
}

func (t *Toolbar) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return t.measure(ctx, constraints).Size
}

func (t *Toolbar) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	t.cachedRootBounds = bounds
	t.cachedSurfaceBounds = bounds
	t.layoutRole.ArrangedBounds = bounds
	t.cachedSeparatorBounds = t.cachedSeparatorBounds[:0]
	if bounds.IsEmpty() {
		t.cachedChildBounds = nil
		return
	}
	t.syncChildren()
	rtl := t.cachedWritingDirection == facet.WritingDirectionRTL
	childSizes := make([]gfx.Size, len(t.cachedChildren))
	for i := range t.cachedChildren {
		childSizes[i] = t.cachedChildren[i].measureSize()
	}
	segments := make([]layout.InlineFlowSegment, 0, len(childSizes))
	for i, size := range childSizes {
		segment := layout.InlineFlowSegment{Size: size}
		if i < len(childSizes)-1 {
			segment.GapAfter = t.cachedGroupGap
		}
		segments = append(segments, segment)
	}
	rects := layout.ArrangeInlineFlowSegments(bounds, t.cachedPadX, segments, rtl)
	t.cachedChildBounds = t.cachedChildBounds[:0]
	for i := range t.cachedChildren {
		child := t.cachedChildren[i]
		rect := rects[i]
		if child == nil {
			t.cachedChildBounds = append(t.cachedChildBounds, gfx.Rect{})
			continue
		}
		child.arrange(ctx, rect)
		t.cachedChildBounds = append(t.cachedChildBounds, rect)
	}
	for i := 0; i < len(t.cachedChildren)-1; i++ {
		left := t.cachedChildBounds[i]
		right := t.cachedChildBounds[i+1]
		if left.IsEmpty() || right.IsEmpty() {
			continue
		}
		sepX := (left.Max.X + right.Min.X) * 0.5
		if rtl {
			sepX = (right.Max.X + left.Min.X) * 0.5
		}
		sepH := bounds.Height() - t.cachedPadY*0.8
		if sepH < 1 {
			sepH = 1
		}
		t.cachedSeparatorBounds = append(t.cachedSeparatorBounds, gfx.RectFromXYWH(sepX, bounds.Min.Y+t.cachedPadY*0.4, 1, sepH))
	}
}

func (t *Toolbar) resolveTheme(ctx facet.MeasureContext) (theme.ResolvedContext, shared.ToolbarSlots, bool) {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{
		Tokens:    resolved.TokenSet(),
		Materials: resolved.Materials,
		Depth:     resolved.Depth,
	}
	slots, _ := uiaction.ResolveToolbarRecipe(style)
	return resolved, slots, true
}

func (t *Toolbar) resolveProjectionTheme(runtime any) shared.ToolbarSlots {
	if runtime != nil {
		if store := theme.NearestStyleContext(runtime, t.Base().ID()); store != nil {
			slots, _ := uiaction.ResolveToolbarRecipe(store.Get())
			return slots
		}
	}
	return t.cachedRecipe
}

func (t *Toolbar) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if t == nil || bounds.IsEmpty() {
		return nil
	}
	slots := t.resolveProjectionTheme(runtime)
	tokens := t.cachedTokens
	if runtime != nil {
		if store := theme.NearestStyleContext(runtime, t.Base().ID()); store != nil {
			tokens = store.Get().Tokens
		}
	}
	state := t.interactionState()
	root := slots.Root.Resolve(state, tokens)
	surface := slots.ToolbarSurface.Resolve(state, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)
	groupStyle := slots.Groups
	sepMat := slots.Separators.Resolve(theme.StateDefault, tokens)
	overflowStyle := slots.OverflowMenu

	cmds := make([]gfx.Command, 0, 128)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(surface) {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(bounds, t.cachedRadius), surface)...)
	}
	for i := range t.cachedChildren {
		child := t.cachedChildren[i]
		if child == nil || t.cachedChildBounds[i].IsEmpty() {
			continue
		}
		childState := theme.StateDefault
		if child.hasActiveItem() {
			childState = theme.StateSelected
		}
		switch child.kind {
		case toolbarChildGroup:
			groupMat := groupStyle.Resolve(childState, tokens)
			if !isTransparentMaterial(groupMat) {
				cmds = append(cmds, materialCommands(gfx.RoundedRectPath(t.cachedChildBounds[i], maxFloat(0, t.cachedRadius*0.75)), groupMat)...)
			}
		case toolbarChildOverflow:
			overflowMat := overflowStyle.Resolve(childState, tokens)
			if !isTransparentMaterial(overflowMat) {
				cmds = append(cmds, materialCommands(gfx.RoundedRectPath(t.cachedChildBounds[i], maxFloat(0, t.cachedRadius*0.75)), overflowMat)...)
			}
		}
		if childCmds := child.project(runtimeServicesOrNil(runtime), t.cachedChildBounds[i], 1); childCmds != nil {
			cmds = append(cmds, childCmds.Commands...)
		}
	}
	if !isTransparentMaterial(sepMat) {
		for _, sep := range t.cachedSeparatorBounds {
			cmds = append(cmds, materialCommands(gfx.RectPath(sep), sepMat)...)
		}
	}
	if t.focusedVisible && !isTransparentMaterial(focus) {
		inset := maxFloat(1, t.cachedPadY*0.5)
		ringBounds := bounds.Inset(-inset, -inset)
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ringBounds, t.cachedRadius+inset), focus)...)
	}
	return cmds
}

func (t *Toolbar) hitTest(p gfx.Point) facet.HitResult {
	if t == nil || t.layoutRole.ArrangedBounds.IsEmpty() || !t.layoutRole.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := t.cursorShape()
	if t.focusedVisible && t.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: toolbarMarkIDFocusRing, Cursor: cursor}
	}
	for i := range t.cachedChildren {
		child := t.cachedChildren[i]
		if child == nil || t.cachedChildBounds[i].IsEmpty() || !t.cachedChildBounds[i].Contains(p) {
			continue
		}
		hit := child.hitTest(p)
		switch hit.MarkID {
		case actionGroupMarkIDSeparators:
			return facet.HitResult{Hit: true, MarkID: toolbarMarkIDSeparators, Cursor: cursor}
		case actionGroupMarkIDActionItems:
			return facet.HitResult{Hit: true, MarkID: toolbarMarkIDActionItems, Cursor: cursor}
		case menuButtonMarkIDMenuItems, menuButtonMarkIDTrigger, menuButtonMarkIDChevron, menuButtonMarkIDFloatingMenuSurface:
			return facet.HitResult{Hit: true, MarkID: toolbarMarkIDOverflowMenu, Cursor: cursor}
		case menuButtonMarkIDFocusRing:
			return facet.HitResult{Hit: true, MarkID: toolbarMarkIDFocusRing, Cursor: cursor}
		}
		if child.kind == toolbarChildOverflow {
			return facet.HitResult{Hit: true, MarkID: toolbarMarkIDOverflowMenu, Cursor: cursor}
		}
		return facet.HitResult{Hit: true, MarkID: toolbarMarkIDGroups, Cursor: cursor}
	}
	for _, sep := range t.cachedSeparatorBounds {
		if sep.Contains(p) {
			return facet.HitResult{Hit: true, MarkID: toolbarMarkIDSeparators, Cursor: cursor}
		}
	}
	if t.cachedSurfaceBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: toolbarMarkIDSurface, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: toolbarMarkIDRoot, Cursor: cursor}
}

func (t *Toolbar) cursorShape() facet.CursorShape {
	if t.Disabled {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (t *Toolbar) onPointer(e facet.PointerEvent) bool {
	if t.Disabled {
		return false
	}
	idx := t.childIndexAt(e.Position)
	switch e.Kind {
	case platform.PointerEnter, platform.PointerMove:
		if idx != t.hoveredIndex {
			t.hoveredIndex = idx
			t.invalidate(facet.DirtyProjection)
		}
		if idx >= 0 {
			return t.cachedChildren[idx].pointer(e)
		}
		t.hovered = true
		t.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		if t.hoveredIndex >= 0 && t.hoveredIndex < len(t.cachedChildren) {
			_ = t.cachedChildren[t.hoveredIndex].pointer(facet.PointerEvent{Kind: platform.PointerLeave})
		}
		t.hoveredIndex = -1
		t.pressedIndex = -1
		t.hovered = false
		t.pressed = false
		t.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		if idx >= 0 {
			t.focusFromPointer = true
			t.focusedVisible = false
			t.focusedIndex = idx
			t.pressedIndex = idx
			t.hoveredIndex = idx
			t.invalidate(facet.DirtyProjection)
			return t.cachedChildren[idx].pointer(e)
		}
		t.pressed = true
		t.focusFromPointer = true
		t.focusedVisible = false
		t.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		if idx >= 0 {
			wasPressed := t.pressedIndex == idx
			t.pressedIndex = -1
			t.invalidate(facet.DirtyProjection)
			handled := t.cachedChildren[idx].pointer(e)
			return wasPressed || handled
		}
		wasPressed := t.pressed
		t.pressed = false
		t.invalidate(facet.DirtyProjection)
		return wasPressed
	default:
		return false
	}
}

func (t *Toolbar) onKey(e facet.KeyEvent) bool {
	if t.Disabled || len(t.cachedChildren) == 0 {
		return false
	}
	child := t.focusedChild()
	if child != nil {
		before := child.focusedIndex()
		if child.keyEvent(e) {
			after := child.focusedIndex()
			if (e.Key == platform.KeyLeft || e.Key == platform.KeyRight) && before == after && child.kind == toolbarChildGroup {
				if t.moveFocusByKey(e.Key) {
					return true
				}
			}
			return true
		}
	}
	switch e.Key {
	case platform.KeyLeft:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			return t.moveFocus(-1)
		}
	case platform.KeyRight:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			return t.moveFocus(1)
		}
	case platform.KeyHome:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			return t.setFocusIndex(t.firstEnabledChildIndex())
		}
	case platform.KeyEnd:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			return t.setFocusIndex(t.lastEnabledChildIndex())
		}
	}
	return false
}

func (t *Toolbar) onFocusGained() {
	t.focusedVisible = !t.focusFromPointer
	t.focusFromPointer = false
	if t.focusedIndex < 0 {
		t.focusedIndex = t.firstEnabledChildIndex()
	}
	if child := t.focusedChild(); child != nil && child.kind == toolbarChildGroup {
		if child.focusedIndex() < 0 {
			child.setFocusedIndex(child.group.firstEnabledIndex())
		}
	}
	t.invalidate(facet.DirtyProjection)
}

func (t *Toolbar) onFocusLost() {
	if child := t.focusedChild(); child != nil {
		_ = child.pointer(facet.PointerEvent{Kind: platform.PointerLeave})
	}
	t.focusedVisible = false
	t.pressed = false
	t.hovered = false
	t.focusFromPointer = false
	t.pressedIndex = -1
	t.hoveredIndex = -1
	t.invalidate(facet.DirtyProjection)
}

func (t *Toolbar) interactionState() theme.InteractionState {
	switch {
	case t.Disabled:
		return theme.StateDisabled
	case t.pressed:
		return theme.StatePressed
	case t.hovered:
		return theme.StateHover
	case t.anyActiveChild():
		return theme.StateSelected
	case t.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (t *Toolbar) pointInFocusRing(p gfx.Point) bool {
	if !t.layoutRole.ArrangedBounds.Contains(p) {
		return false
	}
	inset := maxFloat(1, t.layoutRole.ArrangedBounds.Height()*0.08)
	inner := t.layoutRole.ArrangedBounds.Inset(inset, inset)
	if inner.IsEmpty() {
		return true
	}
	return !inner.Contains(p)
}

func (t *Toolbar) childIndexAt(p gfx.Point) int {
	for i := range t.cachedChildren {
		if t.cachedChildBounds[i].Contains(p) {
			return i
		}
	}
	return -1
}

func (t *Toolbar) focusedChild() *toolbarChild {
	if t == nil || t.focusedIndex < 0 || t.focusedIndex >= len(t.cachedChildren) {
		return nil
	}
	return t.cachedChildren[t.focusedIndex]
}

func (t *Toolbar) moveFocus(delta int) bool {
	if len(t.cachedChildren) == 0 {
		return false
	}
	next := t.focusedIndex + delta
	if next < 0 {
		next = 0
	}
	if next >= len(t.cachedChildren) {
		next = len(t.cachedChildren) - 1
	}
	return t.setFocusIndex(next)
}

func (t *Toolbar) moveFocusByKey(key platform.Key) bool {
	switch key {
	case platform.KeyLeft:
		return t.moveFocus(-1)
	case platform.KeyRight:
		return t.moveFocus(1)
	default:
		return false
	}
}

func (t *Toolbar) setFocusIndex(index int) bool {
	if index < 0 || index >= len(t.cachedChildren) {
		return false
	}
	if t.focusedIndex == index {
		return false
	}
	t.focusedIndex = index
	child := t.cachedChildren[index]
	if child != nil && child.kind == toolbarChildGroup && child.focusedIndex() < 0 {
		child.setFocusedIndex(child.group.firstEnabledIndex())
	}
	t.invalidate(facet.DirtyProjection)
	return true
}

func (t *Toolbar) firstEnabledChildIndex() int {
	for i := range t.cachedChildren {
		if t.childEnabled(i) {
			return i
		}
	}
	return -1
}

func (t *Toolbar) lastEnabledChildIndex() int {
	for i := len(t.cachedChildren) - 1; i >= 0; i-- {
		if t.childEnabled(i) {
			return i
		}
	}
	return -1
}

func (t *Toolbar) childEnabled(index int) bool {
	if index < 0 || index >= len(t.cachedChildren) {
		return false
	}
	child := t.cachedChildren[index]
	if child == nil {
		return false
	}
	switch child.kind {
	case toolbarChildGroup:
		return child.group != nil && !child.group.Disabled && len(child.group.Actions) > 0
	case toolbarChildOverflow:
		return child.overflow != nil && !child.overflow.Disabled && len(child.overflow.Entries) > 0
	default:
		return false
	}
}

func (t *Toolbar) anyActiveChild() bool {
	for i := range t.cachedChildren {
		if t.cachedChildren[i] != nil && t.cachedChildren[i].hasActiveItem() {
			return true
		}
	}
	return false
}

func (t *Toolbar) baselinePoint(bounds gfx.Rect) gfx.Point {
	if t == nil {
		return gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	for i := range t.cachedChildren {
		child := t.cachedChildren[i]
		if child == nil || t.cachedChildBounds[i].IsEmpty() {
			continue
		}
		switch child.kind {
		case toolbarChildGroup:
			if child.group != nil && len(child.group.cachedItemLayouts) > 0 {
				first := &child.group.cachedItemLayouts[0]
				if first.labelLayout != nil {
					return gfx.Point{X: t.cachedChildBounds[i].Min.X, Y: first.labelBounds.Min.Y + first.labelLayout.Baseline}
				}
			}
		case toolbarChildOverflow:
			if child.overflow != nil && child.overflow.cachedTriggerLabelLayout != nil {
				return gfx.Point{X: t.cachedChildBounds[i].Min.X, Y: child.overflow.cachedTriggerLabelBounds.Min.Y + child.overflow.cachedTriggerLabelLayout.Baseline}
			}
		}
	}
	return gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
}

func normalizeToolbarGroups(groups []ToolbarGroup) []ToolbarGroup {
	if len(groups) == 0 {
		return nil
	}
	out := make([]ToolbarGroup, len(groups))
	for i := range groups {
		out[i] = normalizeToolbarGroup(groups[i])
	}
	return out
}

func normalizeToolbarGroup(group ToolbarGroup) ToolbarGroup {
	group.Key = strings.TrimSpace(group.Key)
	group.Actions = normalizeActionGroupActions(group.Actions)
	if group.Key == "" {
		for _, action := range group.Actions {
			if name := strings.TrimSpace(action.Key); name != "" {
				group.Key = name
				break
			}
			if name := strings.TrimSpace(action.AccessibleLabel); name != "" {
				group.Key = name
				break
			}
			if name := strings.TrimSpace(action.Label); name != "" {
				group.Key = name
				break
			}
		}
	}
	return group
}

func normalizeToolbarOverflow(overflow *ToolbarOverflow) *ToolbarOverflow {
	if overflow == nil {
		return nil
	}
	next := &ToolbarOverflow{
		AccessibleLabel: strings.TrimSpace(overflow.AccessibleLabel),
		TriggerIconRef:  strings.TrimSpace(overflow.TriggerIconRef),
		Entries:         normalizeMenuButtonEntries(overflow.Entries),
	}
	if next.TriggerIconRef == "" {
		next.TriggerIconRef = "more"
	}
	return next
}

type toolbarGroupPolicy struct {
	toolbar *Toolbar
}

func (toolbarGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }

func (p toolbarGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.toolbar == nil {
		return facet.GroupMeasureResult{}, nil
	}
	return facet.GroupMeasureResult{Size: p.toolbar.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}}).Size}, nil
}

func (p toolbarGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.toolbar == nil {
		return nil, nil
	}
	p.toolbar.arrange(ctx.ArrangeContext, ctx.Bounds)
	arranged := make([]facet.ArrangedGroupChild, 0, len(children))
	for _, child := range children {
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
