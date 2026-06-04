package action

import (
	"reflect"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiaction"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

const (
	actionBarMarkIDRoot         facet.MarkID = 1
	actionBarMarkIDBarSurface   facet.MarkID = 2
	actionBarMarkIDContextLabel facet.MarkID = 3
	actionBarMarkIDActionItems  facet.MarkID = 4
	actionBarMarkIDOverflowMenu facet.MarkID = 5
	actionBarMarkIDFocusRing    facet.MarkID = 6
)

// ActionBarAction describes one action bar child item.
type ActionBarAction struct {
	Key             string
	Label           string
	AccessibleLabel string
	IconRef         string
	Variant         uiinput.ButtonVariant
	Disabled        bool
}

// ActionBar implements the action.action_bar standard mark.
type ActionBar struct {
	marks.Core

	Label    marks.Binding[string]
	Actions  marks.Binding[[]ActionBarAction]
	Overflow marks.Binding[*ActionBarAction]
	Disabled marks.Binding[bool]

	Activated signal.Signal[string]

	textRole facet.TextRole

	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool
	focusedIndex     int
	hoveredIndex     int
	pressedIndex     int

	cachedTokens           theme.Tokens
	cachedRecipe           shared.ActionBarSlots
	cachedRootBounds       gfx.Rect
	cachedSurfaceBounds    gfx.Rect
	cachedLabelBounds      gfx.Rect
	cachedActionBounds     []gfx.Rect
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedLabelGap         float32
	cachedRadius           float32
	cachedWritingDirection facet.WritingDirection
	cachedLabelLayout      *text.TextLayout
	cachedLabelStyle       text.TextStyle
	cachedItems            []*actionBarItem
}

type actionBarItemKind uint8

const (
	actionBarItemButton actionBarItemKind = iota
	actionBarItemIconButton
)

type actionBarItem struct {
	parent *ActionBar
	index  int
	spec   ActionBarAction
	kind   actionBarItemKind

	button     *Button
	iconButton *IconButton
	subID      signal.SubscriptionID
}

var _ facet.FacetImpl = (*ActionBar)(nil)
var _ layout.AnchorExporter = (*ActionBar)(nil)
var _ marks.Mark = (*ActionBar)(nil)

// NewActionBar constructs an action.action_bar mark with canonical defaults.
func NewActionBar(label string, actions []ActionBarAction) *ActionBar {
	a := &ActionBar{
		Label:        marks.Const(label),
		Actions:      marks.Const(normalizeActionBarActions(actions)),
		Overflow:     marks.Const[*ActionBarAction](nil),
		Disabled:     marks.Const(false),
		focusedIndex: -1,
		hoveredIndex: -1,
		pressedIndex: -1,
		Activated:    signal.NewSignal[string]("action_bar_activated"),
	}
	a.Core.Facet = facet.NewFacet()
	a.AddBinding(a.Label)
	a.AddBinding(a.Actions)
	a.AddBinding(a.Overflow)
	a.AddBinding(a.Disabled)

	a.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearHorizontal,
		Policy:   actionBarGroupPolicy{bar: a},
		Children: a,
		Overflow: facet.OverflowClip,
		Clipping: facet.GroupClipBounds,
	}
	a.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := a.measureIntrinsic(ctx, constraints)
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
	a.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return a.measure(ctx, constraints)
	}
	a.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		a.Layout.ArrangedBounds = bounds
		a.arrange(ctx, bounds)
	}
	a.Hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		return a.hitTest(p)
	}
	a.Input.OnPointer = func(e facet.PointerEvent) bool {
		return a.onPointer(e)
	}
	a.Input.OnKey = func(e facet.KeyEvent) bool {
		return a.onKey(e)
	}
	a.Focus.Focusable = func() bool {
		return !a.Disabled.Get() && (strings.TrimSpace(a.Label.Get()) != "" || len(a.Actions.Get()) > 0 || a.Overflow.Get() != nil)
	}
	a.Focus.TabIndex = 0
	a.Focus.OnFocusGained = func() { a.onFocusGained() }
	a.Focus.OnFocusLost = func() { a.onFocusLost() }
	a.textRole.IMEEnabled = false

	a.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return a.buildCommands(a.Layout.ArrangedBounds, ctx.Runtime)
	}
	a.RegisterRoles()
	a.AddRole(&a.textRole)
	a.syncItems()
	return a
}

// Base satisfies facet.FacetImpl.
func (a *ActionBar) Base() *facet.Facet {
	a.Facet.BindImpl(a)
	return &a.Facet
}

// Descriptor satisfies marks.Mark.
func (a *ActionBar) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "action", TypeName: "action_bar"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (a *ActionBar) AccessibilityRole() string { return "toolbar" }

// AccessibleName reports the semantic name required by the spec.
func (a *ActionBar) AccessibleName() string {
	if a == nil {
		return ""
	}
	if strings.TrimSpace(a.Label.Get()) != "" {
		return strings.TrimSpace(a.Label.Get())
	}
	for _, action := range a.Actions.Get() {
		if name := strings.TrimSpace(action.AccessibleLabel); name != "" {
			return name
		}
		if name := strings.TrimSpace(action.Label); name != "" {
			return name
		}
	}
	if overflow := a.Overflow.Get(); overflow != nil {
		if name := strings.TrimSpace(overflow.AccessibleLabel); name != "" {
			return name
		}
		if name := strings.TrimSpace(overflow.Label); name != "" {
			return name
		}
	}
	return ""
}

// Children returns the facet's immediate child list.
func (a *ActionBar) Children() []facet.GroupChild {
	if a == nil {
		return nil
	}
	a.syncItems()
	out := make([]facet.GroupChild, 0, len(a.cachedItems))
	for i := range a.cachedItems {
		child := a.cachedItems[i]
		if child == nil {
			continue
		}
		base := child.base()
		layoutRole := base.LayoutRole()
		if layoutRole == nil {
			continue
		}
		markID := actionBarMarkIDActionItems
		if child.isOverflow() {
			markID = actionBarMarkIDOverflowMenu
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

// OnAttach subscribes binding sources.
func (a *ActionBar) OnAttach(ctx facet.AttachContext) { a.Core.OnAttach() }

// OnActivate is unused.
func (a *ActionBar) OnActivate() { a.Core.OnActivate() }

// OnDeactivate is unused.
func (a *ActionBar) OnDeactivate() { a.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (a *ActionBar) OnDetach() {
	a.Core.OnDetach()
	a.cachedTokens = theme.Tokens{}
	a.cachedRecipe = shared.ActionBarSlots{}
	a.cachedRootBounds = gfx.Rect{}
	a.cachedSurfaceBounds = gfx.Rect{}
	a.cachedLabelBounds = gfx.Rect{}
	a.cachedActionBounds = nil
	a.cachedPadX = 0
	a.cachedPadY = 0
	a.cachedGap = 0
	a.cachedLabelGap = 0
	a.cachedRadius = 0
	a.cachedLabelLayout = nil
	a.cachedItems = nil
}

// ExportAnchors publishes the action bar anchor set.
func (a *ActionBar) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if a == nil {
		return nil
	}
	out := a.Core.DefaultAnchors(a.Layout.ArrangedBounds, ctx)
	if out == nil {
		return nil
	}
	if a.cachedLabelLayout != nil {
		out["baseline"] = gfx.Point{X: a.Layout.ArrangedBounds.Min.X, Y: a.cachedLabelBounds.Min.Y + a.cachedLabelLayout.Baseline}
	} else {
		out["baseline"] = gfx.Point{X: a.Layout.ArrangedBounds.Min.X, Y: a.Layout.ArrangedBounds.Min.Y}
	}
	return out
}

func (a *ActionBar) invalidate(flags facet.DirtyFlags) {
	if a == nil {
		return
	}
	a.Facet.Invalidate(flags)
}

func (a *ActionBar) syncItems() {
	if a == nil {
		return
	}
	specs := a.itemSpecs()
	if len(a.cachedItems) > len(specs) {
		for _, item := range a.cachedItems[len(specs):] {
			if item != nil {
				item.dispose()
			}
		}
		a.cachedItems = a.cachedItems[:len(specs)]
	}
	if len(a.cachedItems) < len(specs) {
		next := make([]*actionBarItem, len(specs))
		copy(next, a.cachedItems)
		a.cachedItems = next
	}
	for i := range specs {
		if a.cachedItems[i] == nil {
			a.cachedItems[i] = newActionBarItem(a, i, specs[i])
		}
		a.cachedItems[i].index = i
		a.cachedItems[i].setSpec(specs[i])
	}
	if len(a.cachedActionBounds) != len(a.cachedItems) {
		a.cachedActionBounds = make([]gfx.Rect, len(a.cachedItems))
	}
	if len(a.cachedItems) == 0 {
		a.focusedIndex = -1
	} else if a.focusedIndex >= len(a.cachedItems) {
		a.focusedIndex = len(a.cachedItems) - 1
	}
}

func (a *ActionBar) itemSpecs() []ActionBarAction {
	if a == nil {
		return nil
	}
	actions := a.Actions.Get()
	out := make([]ActionBarAction, 0, len(actions)+1)
	out = append(out, normalizeActionBarActions(actions)...)
	if overflow := a.Overflow.Get(); overflow != nil {
		next := normalizeActionBarAction(*overflow)
		out = append(out, next)
	}
	return out
}

func (a *ActionBar) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, recipe, ok := a.resolveTheme(ctx)
	if !ok {
		a.cachedLabelLayout = nil
		a.textRole.Layout = nil
		return facet.MeasureResult{}
	}
	a.cachedTokens = resolved.TokenSet()
	a.cachedRecipe = recipe
	a.cachedWritingDirection = ctx.WritingDirection
	a.cachedPadX = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	a.cachedPadY = maxFloat(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	a.cachedGap = maxFloat(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	a.cachedLabelGap = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	a.cachedRadius = float32(resolved.Radius(theme.RadiusM))

	labelStyle := resolved.TextStyle(theme.TextLabelM)
	a.cachedLabelStyle = labelStyle
	labelLayout := a.resolveLabelLayout(ctx, resolved, labelStyle, constraints.MaxSize.W)
	a.cachedLabelLayout = labelLayout
	a.textRole.Layout = labelLayout
	if labelLayout != nil {
		a.cachedLabelBounds = gfx.RectFromXYWH(0, 0, labelLayout.Bounds.Width(), labelLayout.Bounds.Height())
	} else {
		a.cachedLabelBounds = gfx.Rect{}
	}

	a.syncItems()
	childSizes := make([]gfx.Size, 0, len(a.cachedItems))
	for i := range a.cachedItems {
		child := a.cachedItems[i]
		if child == nil {
			continue
		}
		size := child.measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: constraints.MaxSize.W, H: constraints.MaxSize.H}})
		childSizes = append(childSizes, size)
	}

	childFlow := layout.InlineFlowSize(childSizes, a.cachedGap)
	labelMaxWidth := constraints.MaxSize.W - a.cachedPadX*2 - childFlow.W
	if labelLayout != nil && len(childSizes) > 0 {
		labelMaxWidth -= a.cachedLabelGap
	}
	if labelMaxWidth < 0 {
		labelMaxWidth = 0
	}
	if labelLayout != nil {
		labelLayout = a.resolveLabelLayout(ctx, resolved, labelStyle, labelMaxWidth)
		a.cachedLabelLayout = labelLayout
		a.textRole.Layout = labelLayout
		if labelLayout != nil {
			a.cachedLabelBounds = gfx.RectFromXYWH(0, 0, labelLayout.Bounds.Width(), labelLayout.Bounds.Height())
		} else {
			a.cachedLabelBounds = gfx.Rect{}
		}
	}

	segments := make([]layout.InlineFlowSegment, 0, len(childSizes)+1)
	if a.cachedLabelLayout != nil {
		labelSegment := layout.InlineFlowSegment{
			Size: gfx.Size{
				W: a.cachedLabelLayout.Bounds.Width(),
				H: a.cachedLabelLayout.Bounds.Height(),
			},
		}
		if len(childSizes) > 0 {
			labelSegment.GapAfter = a.cachedLabelGap
		}
		segments = append(segments, labelSegment)
	}
	for i, size := range childSizes {
		segment := layout.InlineFlowSegment{Size: size}
		if i < len(childSizes)-1 {
			segment.GapAfter = a.cachedGap
		}
		segments = append(segments, segment)
	}
	content := layout.InlineFlowSegmentsSize(segments)
	contentH := content.H
	minHeight := maxFloat(resolved.Density.Scale(36), a.cachedPadY*2+contentH)
	size := gfx.Size{
		W: maxFloat(resolved.Density.Scale(120), content.W+a.cachedPadX*2),
		H: minHeight,
	}
	size = constraints.Constrain(size)
	a.Layout.MeasuredSize = size
	a.Layout.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	return a.Layout.MeasuredResult
}

func (a *ActionBar) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return a.measure(ctx, constraints).Size
}

func (a *ActionBar) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	a.cachedRootBounds = bounds
	a.cachedSurfaceBounds = bounds
	a.Layout.ArrangedBounds = bounds
	a.cachedLabelBounds = gfx.Rect{}
	if bounds.IsEmpty() {
		a.cachedActionBounds = nil
		return
	}
	a.syncItems()
	labelLayout := a.cachedLabelLayout
	childSizes := make([]gfx.Size, 0, len(a.cachedItems))
	for i := range a.cachedItems {
		child := a.cachedItems[i]
		if child == nil {
			continue
		}
		childSizes = append(childSizes, child.measureSize())
	}
	segments := make([]layout.InlineFlowSegment, 0, len(childSizes)+1)
	if labelLayout != nil {
		labelSegment := layout.InlineFlowSegment{
			Size: gfx.Size{
				W: labelLayout.Bounds.Width(),
				H: labelLayout.Bounds.Height(),
			},
		}
		if len(childSizes) > 0 {
			labelSegment.GapAfter = a.cachedLabelGap
		}
		segments = append(segments, labelSegment)
	}
	for i, size := range childSizes {
		segment := layout.InlineFlowSegment{Size: size}
		if i < len(childSizes)-1 {
			segment.GapAfter = a.cachedGap
		}
		segments = append(segments, segment)
	}
	rtl := a.cachedWritingDirection == facet.WritingDirectionRTL
	rects := layout.ArrangeInlineFlowSegments(bounds, a.cachedPadX, segments, rtl)
	a.cachedActionBounds = a.cachedActionBounds[:0]
	childOffset := 0
	if labelLayout != nil && len(rects) > 0 {
		a.cachedLabelBounds = rects[0]
		childOffset = 1
	}
	childRectIndex := 0
	for i := range a.cachedItems {
		child := a.cachedItems[i]
		if child == nil {
			a.cachedActionBounds = append(a.cachedActionBounds, gfx.Rect{})
			continue
		}
		rect := rects[childRectIndex+childOffset]
		childRectIndex++
		child.arrange(ctx, rect)
		a.cachedActionBounds = append(a.cachedActionBounds, rect)
	}
}

func (a *ActionBar) resolveTheme(ctx facet.MeasureContext) (theme.ResolvedContext, shared.ActionBarSlots, bool) {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{
		Tokens:    resolved.TokenSet(),
		Materials: resolved.Materials,
		Depth:     resolved.Depth,
	}
	slots, _ := uiaction.ResolveActionBarRecipe(style)
	return resolved, slots, true
}

func (a *ActionBar) resolveProjectionTheme(runtime any) shared.ActionBarSlots {
	if runtime != nil {
		if store := theme.NearestStyleContext(runtime, a.Base().ID()); store != nil {
			slots, _ := uiaction.ResolveActionBarRecipe(store.Get())
			return slots
		}
	}
	return a.cachedRecipe
}

func (a *ActionBar) resolveLabelLayout(ctx facet.MeasureContext, resolved theme.ResolvedContext, style text.TextStyle, maxWidth float32) *text.TextLayout {
	label := strings.TrimSpace(a.Label.Get())
	if label == "" {
		return nil
	}
	shaper := a.newShaper(ctx.Runtime)
	if shaper == nil {
		return nil
	}
	shaper.SetContentScale(ctx.ContentScale)
	return shaper.ShapeTruncated(label, style, maxWidth)
}

func (a *ActionBar) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if a == nil || bounds.IsEmpty() {
		return nil
	}
	slots := a.resolveProjectionTheme(runtime)
	tokens := a.cachedTokens
	state := a.interactionState()
	root := slots.Root.Resolve(state, tokens)
	surface := slots.BarSurface.Resolve(state, tokens)
	label := slots.ContextLabel.Resolve(state, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)

	cmds := make([]gfx.Command, 0, 64)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(surface) {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(bounds, a.cachedRadius), surface)...)
	}
	if a.cachedLabelLayout != nil {
		cmds = append(cmds, primitive.TextLayoutCommands(a.cachedLabelLayout, a.cachedLabelBounds, gfx.SolidBrush(materialColor(label)))...)
	}
	for i := range a.cachedItems {
		child := a.cachedItems[i]
		if child == nil {
			continue
		}
		childCmds := child.project(runtimeServicesOrNil(runtime), a.cachedActionBounds[i], 1)
		if childCmds != nil {
			cmds = append(cmds, childCmds.Commands...)
		}
	}
	if a.focusedVisible && !isTransparentMaterial(focus) {
		inset := maxFloat(1, a.cachedPadY*0.5)
		ringBounds := bounds.Inset(-inset, -inset)
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ringBounds, a.cachedRadius+inset), focus)...)
	}
	return cmds
}

func (a *ActionBar) hitTest(p gfx.Point) facet.HitResult {
	if a == nil || a.Layout.ArrangedBounds.IsEmpty() || !a.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := a.cursorShape()
	if a.focusedVisible && a.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: actionBarMarkIDFocusRing, Cursor: cursor}
	}
	if idx := a.indexAt(p); idx >= 0 {
		if a.cachedItems[idx].isOverflow() {
			return facet.HitResult{Hit: true, MarkID: actionBarMarkIDOverflowMenu, Cursor: cursor}
		}
		return facet.HitResult{Hit: true, MarkID: actionBarMarkIDActionItems, Cursor: cursor}
	}
	if a.cachedLabelBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: actionBarMarkIDContextLabel, Cursor: cursor}
	}
	if a.cachedSurfaceBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: actionBarMarkIDBarSurface, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: actionBarMarkIDRoot, Cursor: cursor}
}

func (a *ActionBar) cursorShape() facet.CursorShape {
	if a.Disabled.Get() {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (a *ActionBar) onPointer(e facet.PointerEvent) bool {
	if a.Disabled.Get() {
		return false
	}
	idx := a.indexAt(e.Position)
	switch e.Kind {
	case platform.PointerEnter, platform.PointerMove:
		if idx != a.hoveredIndex {
			a.hoveredIndex = idx
			a.invalidate(facet.DirtyProjection)
		}
		if idx >= 0 {
			return a.cachedItems[idx].pointer(e)
		}
		a.hovered = true
		a.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		if a.hoveredIndex >= 0 && a.hoveredIndex < len(a.cachedItems) {
			_ = a.cachedItems[a.hoveredIndex].pointer(facet.PointerEvent{Kind: platform.PointerLeave})
		}
		a.hoveredIndex = -1
		a.pressedIndex = -1
		a.hovered = false
		a.pressed = false
		a.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		if idx >= 0 {
			a.focusFromPointer = true
			a.focusedVisible = false
			a.pressedIndex = idx
			a.hoveredIndex = idx
			a.invalidate(facet.DirtyProjection)
			return a.cachedItems[idx].pointer(e)
		}
		a.pressed = true
		a.focusFromPointer = true
		a.focusedVisible = false
		a.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		if idx >= 0 {
			wasPressed := a.pressedIndex == idx
			a.pressedIndex = -1
			a.invalidate(facet.DirtyProjection)
			if wasPressed {
				return a.cachedItems[idx].pointer(e)
			}
			return a.cachedItems[idx].pointer(e)
		}
		wasPressed := a.pressed
		a.pressed = false
		a.invalidate(facet.DirtyProjection)
		return wasPressed
	default:
		return false
	}
}

func (a *ActionBar) onKey(e facet.KeyEvent) bool {
	if a.Disabled.Get() || len(a.cachedItems) == 0 {
		return false
	}
	switch e.Key {
	case platform.KeyLeft:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			a.moveFocus(-1)
			return true
		}
	case platform.KeyRight:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			a.moveFocus(1)
			return true
		}
	case platform.KeyHome:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			a.setFocusIndex(0)
			return true
		}
	case platform.KeyEnd:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			a.setFocusIndex(len(a.cachedItems) - 1)
			return true
		}
	case platform.KeyEnter, platform.KeySpace:
		if a.focusedIndex >= 0 && a.focusedIndex < len(a.cachedItems) {
			return a.cachedItems[a.focusedIndex].keyEvent(e)
		}
	}
	return false
}

func (a *ActionBar) onFocusGained() {
	a.focusedVisible = !a.focusFromPointer
	a.focusFromPointer = false
	if a.focusedIndex < 0 && len(a.cachedItems) > 0 {
		a.focusedIndex = 0
	}
	a.invalidate(facet.DirtyProjection)
}

func (a *ActionBar) onFocusLost() {
	a.focusedVisible = false
	a.pressed = false
	a.hovered = false
	a.focusFromPointer = false
	a.pressedIndex = -1
	a.hoveredIndex = -1
	a.invalidate(facet.DirtyProjection)
}

func (a *ActionBar) interactionState() theme.InteractionState {
	switch {
	case a.Disabled.Get():
		return theme.StateDisabled
	case a.pressed:
		return theme.StatePressed
	case a.hovered:
		return theme.StateHover
	case a.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (a *ActionBar) pointInFocusRing(p gfx.Point) bool {
	if !a.Layout.ArrangedBounds.Contains(p) {
		return false
	}
	inset := maxFloat(1, a.cachedPadY*0.5)
	ring := a.cachedSurfaceBounds.Inset(-inset, -inset)
	if ring.IsEmpty() {
		return true
	}
	return !ring.Contains(p)
}

func (a *ActionBar) indexAt(p gfx.Point) int {
	for i := range a.cachedActionBounds {
		if a.cachedActionBounds[i].Contains(p) {
			return i
		}
	}
	return -1
}

func (a *ActionBar) moveFocus(delta int) {
	if len(a.cachedItems) == 0 {
		return
	}
	next := a.focusedIndex + delta
	if next < 0 {
		next = 0
	}
	if next >= len(a.cachedItems) {
		next = len(a.cachedItems) - 1
	}
	a.setFocusIndex(next)
}

func (a *ActionBar) setFocusIndex(index int) {
	if index < 0 || index >= len(a.cachedItems) {
		return
	}
	if a.focusedIndex == index {
		return
	}
	a.focusedIndex = index
	a.invalidate(facet.DirtyProjection)
}

func (a *ActionBar) newShaper(runtime any) *text.Shaper {
	registry := a.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (a *ActionBar) fontRegistry(runtime any) *text.FontRegistry {
	if runtime == nil {
		return nil
	}
	type fontRegistryProvider interface {
		FontRegistry() *text.FontRegistry
	}
	if provider, ok := runtime.(fontRegistryProvider); ok {
		return provider.FontRegistry()
	}
	return nil
}

func normalizeActionBarActions(actions []ActionBarAction) []ActionBarAction {
	if len(actions) == 0 {
		return nil
	}
	out := make([]ActionBarAction, len(actions))
	for i := range actions {
		out[i] = normalizeActionBarAction(actions[i])
	}
	return out
}

func normalizeActionBarAction(action ActionBarAction) ActionBarAction {
	action.Key = strings.TrimSpace(action.Key)
	action.Label = strings.TrimSpace(action.Label)
	action.AccessibleLabel = strings.TrimSpace(action.AccessibleLabel)
	action.IconRef = strings.TrimSpace(action.IconRef)
	if action.Key == "" {
		switch {
		case action.AccessibleLabel != "":
			action.Key = action.AccessibleLabel
		case action.Label != "":
			action.Key = action.Label
		case action.IconRef != "":
			action.Key = action.IconRef
		}
	}
	if action.AccessibleLabel == "" {
		if action.Label != "" {
			action.AccessibleLabel = action.Label
		} else {
			action.AccessibleLabel = action.Key
		}
	}
	return action
}

func actionBarButtonVariant(action ActionBarAction) uiinput.ButtonVariant {
	switch action.Variant {
	case uiinput.ButtonOutlined, uiinput.ButtonText, uiinput.ButtonTonal:
		return action.Variant
	default:
		return uiinput.ButtonText
	}
}

func maxFloatSlice(values []float32) float32 {
	var out float32
	for _, v := range values {
		if v > out {
			out = v
		}
	}
	return out
}

func runtimeServicesOrNil(runtime any) facet.RuntimeServices {
	if runtime == nil {
		return nil
	}
	services, ok := runtime.(facet.RuntimeServices)
	if !ok {
		return nil
	}
	// reflect check catches typed nil (non-nil interface wrapping nil *Runtime).
	// Only applicable for nil-able kinds (ptr, slice, map, chan, func, iface).
	v := reflect.ValueOf(services)
	switch v.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		if v.IsNil() {
			return nil
		}
	}
	return services
}

func newActionBarItem(parent *ActionBar, index int, spec ActionBarAction) *actionBarItem {
	item := &actionBarItem{
		parent: parent,
		index:  index,
	}
	item.setSpec(spec)
	return item
}

func (it *actionBarItem) dispose() {
	if it == nil {
		return
	}
	switch it.kind {
	case actionBarItemButton:
		if it.button != nil && it.subID != 0 {
			it.button.Activated.Unsubscribe(it.subID)
		}
	case actionBarItemIconButton:
		if it.iconButton != nil && it.subID != 0 {
			it.iconButton.Activated.Unsubscribe(it.subID)
		}
	}
	it.subID = 0
}

func (it *actionBarItem) isOverflow() bool {
	return it != nil && it.parent != nil && it.index >= len(it.parent.Actions.Get())
}

func (it *actionBarItem) setSpec(spec ActionBarAction) {
	if it == nil {
		return
	}
	spec = normalizeActionBarAction(spec)
	desiredKind := actionBarItemButton
	if spec.Label == "" && spec.IconRef != "" {
		desiredKind = actionBarItemIconButton
	}
	if it.kind != desiredKind {
		it.dispose()
		it.button = nil
		it.iconButton = nil
		it.kind = desiredKind
	}
	it.spec = spec
	switch desiredKind {
	case actionBarItemIconButton:
		if it.iconButton == nil {
			it.iconButton = NewIconButton(primitive.IconRef(spec.IconRef))
			it.iconButton.AccessibleLabel = marks.Const(spec.AccessibleLabel)
			it.iconButton.Focus.Focusable = func() bool { return false }
			it.subID = it.iconButton.Activated.Subscribe(func(signal.Unit) {
				if it.parent != nil {
					it.parent.Activated.Emit(it.parent.itemKeyAt(it.index))
				}
			})
		} else {
			it.iconButton.Icon = primitive.IconRef(spec.IconRef)
			it.iconButton.AccessibleLabel = marks.Const(spec.AccessibleLabel)
		}
		it.iconButton.Disabled = marks.Const(it.parent != nil && it.parent.Disabled.Get() || spec.Disabled)
	case actionBarItemButton:
		if it.button == nil {
			it.button = NewButton(marks.Const(spec.Label), marks.Const(actionBarButtonVariant(spec)))
			if strings.TrimSpace(spec.IconRef) != "" {
				it.button.LeadingIconRef = marks.Const(spec.IconRef)
			}
			it.button.Focus.Focusable = func() bool { return false }
			it.subID = it.button.Activated.Subscribe(func(signal.Unit) {
				if it.parent != nil {
					it.parent.Activated.Emit(it.parent.itemKeyAt(it.index))
				}
			})
		} else {
			it.button.Label = marks.Const(spec.Label)
			it.button.Variant = marks.Const(actionBarButtonVariant(spec))
			it.button.LeadingIconRef = marks.Const(spec.IconRef)
		}
		it.button.Disabled = marks.Const(it.parent != nil && it.parent.Disabled.Get() || spec.Disabled)
	}
}

func (it *actionBarItem) base() *facet.Facet {
	if it == nil {
		return nil
	}
	switch it.kind {
	case actionBarItemIconButton:
		if it.iconButton != nil {
			return it.iconButton.Base()
		}
	case actionBarItemButton:
		if it.button != nil {
			return it.button.Base()
		}
	}
	return nil
}

func (it *actionBarItem) pointer(e facet.PointerEvent) bool {
	if it == nil {
		return false
	}
	switch it.kind {
	case actionBarItemIconButton:
		if it.iconButton == nil {
			return false
		}
		return it.iconButton.onPointer(e)
	case actionBarItemButton:
		if it.button == nil {
			return false
		}
		return it.button.onPointer(e)
	default:
		return false
	}
}

func (it *actionBarItem) keyEvent(e facet.KeyEvent) bool {
	if it == nil {
		return false
	}
	switch it.kind {
	case actionBarItemIconButton:
		if it.iconButton == nil {
			return false
		}
		return it.iconButton.onKey(e)
	case actionBarItemButton:
		if it.button == nil {
			return false
		}
		return it.button.onKey(e)
	default:
		return false
	}
}

func (it *actionBarItem) measure(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	if it == nil {
		return gfx.Size{}
	}
	switch it.kind {
	case actionBarItemIconButton:
		if it.iconButton == nil || it.iconButton.Base() == nil || it.iconButton.Base().LayoutRole() == nil {
			return gfx.Size{}
		}
		return it.iconButton.Base().LayoutRole().Measure(ctx, constraints).Size
	case actionBarItemButton:
		if it.button == nil || it.button.Base() == nil || it.button.Base().LayoutRole() == nil {
			return gfx.Size{}
		}
		return it.button.Base().LayoutRole().Measure(ctx, constraints).Size
	default:
		return gfx.Size{}
	}
}

func (it *actionBarItem) measureSize() gfx.Size {
	if it == nil {
		return gfx.Size{}
	}
	switch it.kind {
	case actionBarItemIconButton:
		if it.iconButton == nil || it.iconButton.Base() == nil || it.iconButton.Base().LayoutRole() == nil {
			return gfx.Size{}
		}
		return it.iconButton.Base().LayoutRole().MeasuredSize
	case actionBarItemButton:
		if it.button == nil || it.button.Base() == nil || it.button.Base().LayoutRole() == nil {
			return gfx.Size{}
		}
		return it.button.Base().LayoutRole().MeasuredSize
	default:
		return gfx.Size{}
	}
}

func (it *actionBarItem) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if it == nil {
		return
	}
	switch it.kind {
	case actionBarItemIconButton:
		if it.iconButton == nil || it.iconButton.Base() == nil || it.iconButton.Base().LayoutRole() == nil {
			return
		}
		it.iconButton.Base().LayoutRole().Arrange(ctx, bounds)
	case actionBarItemButton:
		if it.button == nil || it.button.Base() == nil || it.button.Base().LayoutRole() == nil {
			return
		}
		it.button.Base().LayoutRole().Arrange(ctx, bounds)
	}
}

func (it *actionBarItem) project(runtime facet.RuntimeServices, bounds gfx.Rect, contentScale float32) *gfx.CommandList {
	if it == nil || bounds.IsEmpty() {
		return nil
	}
	switch it.kind {
	case actionBarItemIconButton:
		if it.iconButton == nil || it.iconButton.Base() == nil || it.iconButton.Base().ProjectionRole() == nil {
			return nil
		}
		return it.iconButton.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtime, Bounds: bounds, ContentScale: contentScale})
	case actionBarItemButton:
		if it.button == nil || it.button.Base() == nil || it.button.Base().ProjectionRole() == nil {
			return nil
		}
		return it.button.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtime, Bounds: bounds, ContentScale: contentScale})
	default:
		return nil
	}
}

func (it *actionBarItem) childFacet() *facet.Facet {
	if it == nil {
		return nil
	}
	switch it.kind {
	case actionBarItemIconButton:
		if it.iconButton != nil {
			return it.iconButton.Base()
		}
	case actionBarItemButton:
		if it.button != nil {
			return it.button.Base()
		}
	}
	return nil
}

func (a *ActionBar) itemKeyAt(index int) string {
	if a == nil || index < 0 || index >= len(a.cachedItems) {
		return ""
	}
	return a.cachedItems[index].spec.Key
}

type actionBarGroupPolicy struct {
	bar *ActionBar
}

func (actionBarGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }

func (p actionBarGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.bar == nil {
		return facet.GroupMeasureResult{}, nil
	}
	return facet.GroupMeasureResult{Size: p.bar.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}}).Size}, nil
}

func (p actionBarGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.bar == nil {
		return nil, nil
	}
	p.bar.arrange(ctx.ArrangeContext, ctx.Bounds)
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
