package action

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiaction"
)

const (
	actionGroupMarkIDRoot         facet.MarkID = 1
	actionGroupMarkIDGroupSurface facet.MarkID = 2
	actionGroupMarkIDActionItems  facet.MarkID = 3
	actionGroupMarkIDSeparators   facet.MarkID = 4
	actionGroupMarkIDFocusRing    facet.MarkID = 5
)

// ActionGroupAction describes one clustered action item.
type ActionGroupAction struct {
	Key             string
	Label           string
	AccessibleLabel string
	IconRef         string
	Disabled        bool
	Active          bool
}

// ActionGroup implements the action.action_group standard mark.
type ActionGroup struct {
	marks.Core

	Label    marks.Binding[string]
	Actions  marks.Binding[[]ActionGroupAction]
	Disabled marks.Binding[bool]

	Activated signal.Signal[string]

	textRole facet.TextRole

	hoveredIndex     int
	pressedIndex     int
	focusedIndex     int
	focusedVisible   bool
	focusFromPointer bool

	cachedTokens           theme.Tokens
	cachedRecipe           shared.ActionGroupSlots
	cachedRootBounds       gfx.Rect
	cachedGroupBounds      gfx.Rect
	cachedActionBounds     []gfx.Rect
	cachedSeparatorBounds  []gfx.Rect
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedRadius           float32
	cachedItemHeight       float32
	cachedItemIconSize     float32
	cachedWritingDirection facet.WritingDirection
	cachedLabelStyle       text.TextStyle
	cachedItemStyle        text.TextStyle
	cachedItemLayouts      []actionGroupItemLayout
}

type actionGroupItemLayout struct {
	item        ActionGroupAction
	labelLayout *text.TextLayout
	bounds      gfx.Rect
	labelBounds gfx.Rect
	iconBounds  gfx.Rect
	width       float32
	height      float32
}

var _ facet.FacetImpl = (*ActionGroup)(nil)
var _ layout.AnchorExporter = (*ActionGroup)(nil)
var _ marks.Mark = (*ActionGroup)(nil)

// NewActionGroup constructs an action.action_group mark with canonical defaults.
func NewActionGroup(label marks.Binding[string], actions marks.Binding[[]ActionGroupAction]) *ActionGroup {
	g := &ActionGroup{
		Label:        label,
		Actions:      actions,
		Disabled:     marks.Const(false),
		hoveredIndex: -1,
		pressedIndex: -1,
		focusedIndex: -1,
		Activated:    signal.NewSignal[string]("action_group_activated"),
	}
	g.Facet = facet.NewFacet()
	g.AddBinding(g.Label)
	g.AddBinding(g.Actions)
	g.AddBinding(g.Disabled)

	g.Layout.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearHorizontal,
		Policy: actionGroupPolicy{group: g},
	}
	g.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsRadial,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := g.measureIntrinsic(ctx, constraints)
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionTruncate,
			BelowMinHeight: facet.CompressionClip,
			AboveMaxWidth:  facet.ExpansionClip,
			AboveMaxHeight: facet.ExpansionClip,
		},
		Stretch: facet.StretchPolicy{
			Width:  facet.StretchNever,
			Height: facet.StretchNever,
		},
		Baseline: facet.BaselineNone,
	}
	g.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return g.measure(ctx, constraints)
	}
	g.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		g.Layout.ArrangedBounds = bounds
		g.arrange(bounds)
	}
	g.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return g.hitTest(p) }
	g.Input.OnPointer = func(e facet.PointerEvent) bool { return g.onPointer(e) }
	g.Input.OnKey = func(e facet.KeyEvent) bool { return g.onKey(e) }
	g.Focus.Focusable = func() bool { return !g.Disabled.Get() && len(g.Actions.Get()) > 0 }
	g.Focus.TabIndex = 0
	g.Focus.OnFocusGained = func() { g.onFocusGained() }
	g.Focus.OnFocusLost = func() { g.onFocusLost() }
	g.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return g.buildCommands(g.Layout.ArrangedBounds, ctx.Runtime)
	}
	g.RegisterRoles()
	g.AddRole(&g.textRole)
	return g
}

// Base satisfies facet.FacetImpl.
func (g *ActionGroup) Base() *facet.Facet {
	g.BindImpl(g)
	return &g.Facet
}

// Descriptor satisfies marks.Mark.
func (g *ActionGroup) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: markTypeAction, TypeName: "action_group"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (g *ActionGroup) AccessibilityRole() string { return "group" }

// AccessibleName reports the semantic name required by the spec.
func (g *ActionGroup) AccessibleName() string {
	if g == nil {
		return ""
	}
	if name := strings.TrimSpace(g.Label.Get()); name != "" {
		return name
	}
	for _, action := range g.Actions.Get() {
		if name := strings.TrimSpace(action.AccessibleLabel); name != "" {
			return name
		}
		if name := strings.TrimSpace(action.Label); name != "" {
			return name
		}
	}
	return ""
}

// ExportAnchors publishes the action group anchor set.
func (g *ActionGroup) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if g == nil {
		return nil
	}
	out := g.DefaultAnchors(g.Layout.ArrangedBounds, ctx)
	if out == nil {
		return nil
	}
	if len(g.cachedActionBounds) > 0 {
		out["content_anchor"] = gfx.Point{
			X: g.cachedActionBounds[0].Min.X + g.cachedActionBounds[0].Width()*0.5,
			Y: g.cachedActionBounds[0].Min.Y + g.cachedActionBounds[0].Height()*0.5,
		}
	} else {
		bounds := g.Layout.ArrangedBounds
		if bounds.IsEmpty() && !ctx.ResolvedLayer.Bounds.IsEmpty() {
			bounds = ctx.ResolvedLayer.Bounds
		}
		out["content_anchor"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	if g.cachedItemLayouts != nil && len(g.cachedItemLayouts) > 0 && g.cachedItemLayouts[0].labelLayout != nil {
		out["baseline"] = gfx.Point{X: out["bounds_top_left"].X, Y: g.cachedItemLayouts[0].labelBounds.Min.Y + g.cachedItemLayouts[0].labelLayout.Baseline}
	} else {
		out["baseline"] = out["bounds_top_left"]
	}
	return out
}

// Children returns the facet's immediate child list.
func (g *ActionGroup) Children() []facet.GroupChild { return nil }

// OnAttach is unused.
func (g *ActionGroup) OnAttach(ctx facet.AttachContext) { g.Core.OnAttach() }

// OnActivate is unused.
func (g *ActionGroup) OnActivate() { g.Core.OnActivate() }

// OnDeactivate is unused.
func (g *ActionGroup) OnDeactivate() { g.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (g *ActionGroup) OnDetach() {
	g.Core.OnDetach()
	g.cachedTokens = theme.Tokens{}
	g.cachedRecipe = shared.ActionGroupSlots{}
	g.cachedRootBounds = gfx.Rect{}
	g.cachedGroupBounds = gfx.Rect{}
	g.cachedActionBounds = nil
	g.cachedSeparatorBounds = nil
	g.cachedPadX = 0
	g.cachedPadY = 0
	g.cachedGap = 0
	g.cachedRadius = 0
	g.cachedItemHeight = 0
	g.cachedItemIconSize = 0
	g.cachedItemLayouts = nil
}

func (g *ActionGroup) invalidate(flags facet.DirtyFlags) {
	if g == nil {
		return
	}
	g.Invalidate(flags)
}

func (g *ActionGroup) resolveTheme(ctx facet.MeasureContext) (theme.ResolvedContext, shared.ActionGroupSlots, bool) {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{
		Tokens:    resolved.TokenSet(),
		Materials: resolved.Materials,
		Depth:     resolved.Depth,
	}
	slots, _ := uiaction.ResolveActionGroupRecipe(style)
	return resolved, slots, true
}

func (g *ActionGroup) resolveProjectionTheme(runtime any) shared.ActionGroupSlots {
	if runtime != nil {
		if store := theme.NearestStyleContext(runtime, g.Base().ID()); store != nil {
			slots, _ := uiaction.ResolveActionGroupRecipe(store.Get())
			return slots
		}
	}
	return g.cachedRecipe
}

func (g *ActionGroup) newShaper(runtime any) *text.Shaper {
	registry := g.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (g *ActionGroup) fontRegistry(runtime any) *text.FontRegistry {
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

func (g *ActionGroup) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, recipe, ok := g.resolveTheme(ctx)
	if !ok {
		g.textRole.Layout = nil
		return facet.MeasureResult{}
	}
	g.cachedTokens = resolved.TokenSet()
	g.cachedRecipe = recipe
	g.cachedWritingDirection = ctx.WritingDirection
	g.cachedPadX = mathutil.Max(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	g.cachedPadY = mathutil.Max(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	g.cachedGap = mathutil.Max(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	g.cachedRadius = float32(resolved.Radius(theme.RadiusM))
	g.cachedItemHeight = mathutil.Max(resolved.Density.Scale(36), resolved.Density.Scale(32))
	g.cachedItemIconSize = mathutil.Max(resolved.Density.Scale(18), 14)
	g.cachedLabelStyle = resolved.TextStyle(theme.TextLabelM)
	g.cachedItemStyle = resolved.TextStyle(theme.TextLabelM)

	shaper := g.newShaper(ctx.Runtime)
	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = resolved.Density.Scale(480)
	}
	layouts := make([]actionGroupItemLayout, len(g.Actions.Get()))
	maxItemW := float32(0)
	maxItemH := float32(0)
	for i := range g.Actions.Get() {
		item := normalizeActionGroupItem(g.Actions.Get()[i])
		layouts[i].item = item
		if shaper != nil && strings.TrimSpace(item.Label) != "" {
			shaper.SetContentScale(ctx.ContentScale)
			layouts[i].labelLayout = shaper.ShapeTruncated(item.Label, g.cachedLabelStyle, maxWidth)
		}
		labelW := text.Width(layouts[i].labelLayout)
		labelH := text.Height(layouts[i].labelLayout)
		iconW := float32(0)
		if strings.TrimSpace(item.IconRef) != "" {
			iconW = g.cachedItemIconSize
		}
		itemW := g.cachedPadX*2 + labelW
		if iconW > 0 {
			if labelW > 0 {
				itemW += g.cachedGap
			}
			itemW += iconW
		}
		if itemW < resolved.Density.Scale(76) {
			itemW = resolved.Density.Scale(76)
		}
		itemH := mathutil.Max(g.cachedItemHeight, mathutil.Max(labelH, iconW))
		itemH += g.cachedPadY * 2
		layouts[i].width = itemW
		layouts[i].height = itemH
		if itemW > maxItemW {
			maxItemW = itemW
		}
		if itemH > maxItemH {
			maxItemH = itemH
		}
	}
	g.cachedItemLayouts = layouts
	g.cachedActionBounds = make([]gfx.Rect, len(layouts))
	g.cachedSeparatorBounds = make([]gfx.Rect, 0, maxInt(0, len(layouts)-1))

	totalW := float32(0)
	for i := range layouts {
		totalW += layouts[i].width
		if i > 0 {
			totalW += g.cachedGap
		}
	}
	contentH := mathutil.Max(maxItemH, g.cachedItemHeight+g.cachedPadY*2)
	size := gfx.Size{
		W: mathutil.Max(totalW, resolved.Density.Scale(76)),
		H: contentH,
	}
	size = constraints.Constrain(size)
	g.Layout.MeasuredSize = size
	g.Layout.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	return g.Layout.MeasuredResult
}

func (g *ActionGroup) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return g.measure(ctx, constraints).Size
}

func (g *ActionGroup) arrange(bounds gfx.Rect) {
	g.cachedRootBounds = bounds
	g.cachedGroupBounds = gfx.Rect{}
	g.cachedSeparatorBounds = g.cachedSeparatorBounds[:0]
	g.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	inner := bounds.Inset(g.cachedPadX, g.cachedPadY)
	if inner.IsEmpty() {
		inner = bounds
	}
	rtl := g.cachedWritingDirection == facet.WritingDirectionRTL
	startX := inner.Min.X
	if rtl {
		startX = inner.Max.X
	}
	curX := startX
	for i := range g.cachedItemLayouts {
		entry := &g.cachedItemLayouts[i]
		itemW := entry.width
		itemH := entry.height
		itemY := inner.Min.Y + mathutil.Max(0, (inner.Height()-itemH)*0.5)
		if rtl {
			curX -= itemW
			entry.bounds = gfx.RectFromXYWH(curX, itemY, itemW, itemH)
		} else {
			entry.bounds = gfx.RectFromXYWH(curX, itemY, itemW, itemH)
			curX += itemW
		}
		g.cachedActionBounds[i] = entry.bounds
		g.cachedGroupBounds = unionRect(g.cachedGroupBounds, entry.bounds)
		textH := text.Height(entry.labelLayout)
		labelW := text.Width(entry.labelLayout)
		iconX := entry.bounds.Min.X + g.cachedPadX
		labelX := iconX
		if rtl {
			iconX = entry.bounds.Max.X - g.cachedPadX - g.cachedItemIconSize
			labelX = entry.bounds.Max.X - g.cachedPadX - labelW
		}
		if strings.TrimSpace(entry.item.IconRef) != "" {
			entry.iconBounds = gfx.RectFromXYWH(iconX, entry.bounds.Min.Y+(entry.bounds.Height()-g.cachedItemIconSize)*0.5, g.cachedItemIconSize, g.cachedItemIconSize)
			if rtl {
				labelX -= g.cachedGap
			} else {
				labelX += g.cachedItemIconSize + g.cachedGap
			}
		}
		if entry.labelLayout != nil {
			entry.labelBounds = gfx.RectFromXYWH(labelX, entry.bounds.Min.Y+(entry.bounds.Height()-textH)*0.5, labelW, textH)
		}
		if i < len(g.cachedItemLayouts)-1 {
			if rtl {
				sepX := curX - g.cachedGap*0.5
				g.cachedSeparatorBounds = append(g.cachedSeparatorBounds, gfx.RectFromXYWH(sepX, bounds.Min.Y+g.cachedPadY*0.4, 1, bounds.Height()-g.cachedPadY*0.8))
				curX -= g.cachedGap
			} else {
				sepX := curX + g.cachedGap*0.5
				g.cachedSeparatorBounds = append(g.cachedSeparatorBounds, gfx.RectFromXYWH(sepX, bounds.Min.Y+g.cachedPadY*0.4, 1, bounds.Height()-g.cachedPadY*0.8))
				curX += g.cachedGap
			}
		}
	}
	if g.cachedGroupBounds.IsEmpty() {
		g.cachedGroupBounds = bounds
	}
	g.cachedGroupBounds = g.cachedGroupBounds.Union(bounds)
	g.cachedGroupBounds = g.cachedGroupBounds.Inset(0, 0)
	g.cachedRootBounds = bounds
	g.cachedActionBounds = g.cachedActionBounds[:len(g.cachedItemLayouts)]
	g.cachedGroupBounds = unionRect(g.cachedGroupBounds, bounds)
}

func (g *ActionGroup) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if g == nil || bounds.IsEmpty() {
		return nil
	}
	slots := g.resolveProjectionTheme(runtime)
	tokens := g.cachedTokens
	if runtime != nil {
		if store := theme.NearestStyleContext(runtime, g.Base().ID()); store != nil {
			tokens = store.Get().Tokens
		}
	}
	root := slots.Root.Resolve(g.interactionState(), tokens)
	cmds := make([]gfx.Command, 0, 64)
	if !theme.IsTransparentMaterial(root) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(bounds), root)...)
	}
	if !g.cachedGroupBounds.IsEmpty() {
		for i := range g.cachedItemLayouts {
			item := &g.cachedItemLayouts[i]
			state := g.itemState(i)
			itemMat := slots.GroupSurface.Resolve(state, tokens)
			if !theme.IsTransparentMaterial(itemMat) {
				cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(item.bounds, g.cachedRadius), itemMat)...)
			}
			if item.item.IconRef != "" && !item.iconBounds.IsEmpty() {
				iconMat := slots.ActionItems.Resolve(state, tokens)
				if iconCmds := iconAssetCommands(runtimeServicesOrNil(runtime), item.item.IconRef, item.iconBounds, iconMat); len(iconCmds) > 0 {
					cmds = append(cmds, iconCmds...)
				}
			}
			if item.labelLayout != nil && !theme.IsTransparentMaterial(slots.ActionItems.Resolve(state, tokens)) {
				cmds = append(cmds, primitive.TextLayoutCommands(item.labelLayout, item.labelBounds, gfx.SolidBrush(theme.MaterialColor(slots.ActionItems.Resolve(state, tokens))))...)
			}
		}
		sepMat := slots.Separators.Resolve(theme.StateDefault, tokens)
		for _, sep := range g.cachedSeparatorBounds {
			if !theme.IsTransparentMaterial(sepMat) {
				cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(sep), sepMat)...)
			}
		}
	}
	if g.focusedVisible {
		focusMat := slots.FocusRing.Resolve(theme.StateFocused, tokens)
		if !theme.IsTransparentMaterial(focusMat) {
			inset := mathutil.Max(1, bounds.Height()*0.08)
			ringBounds := bounds.Inset(inset, inset)
			cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(ringBounds, g.cachedRadius), focusMat)...)
		}
	}
	return cmds
}

func (g *ActionGroup) hitTest(p gfx.Point) facet.HitResult {
	if g == nil || g.Layout.ArrangedBounds.IsEmpty() || !g.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := g.cursorShape()
	if g.focusedVisible && g.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: actionGroupMarkIDFocusRing, Cursor: cursor}
	}
	if idx := g.indexAt(p); idx >= 0 {
		return facet.HitResult{Hit: true, MarkID: actionGroupMarkIDActionItems, Cursor: cursor}
	}
	if sep := g.separatorAt(p); sep >= 0 {
		return facet.HitResult{Hit: true, MarkID: actionGroupMarkIDSeparators, Cursor: cursor}
	}
	if g.cachedGroupBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: actionGroupMarkIDGroupSurface, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: actionGroupMarkIDRoot, Cursor: cursor}
}

func (g *ActionGroup) cursorShape() facet.CursorShape {
	if g.Disabled.Get() {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (g *ActionGroup) onPointer(e facet.PointerEvent) bool {
	if g.Disabled.Get() {
		return false
	}
	idx := g.indexAt(e.Position)
	switch e.Kind {
	case platform.PointerEnter, platform.PointerMove:
		if idx != g.hoveredIndex {
			g.hoveredIndex = idx
			g.invalidate(facet.DirtyProjection)
		}
		return true
	case platform.PointerLeave:
		g.hoveredIndex = -1
		if g.pressedIndex < 0 {
			g.focusFromPointer = false
		}
		g.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		g.focusFromPointer = true
		g.focusedVisible = false
		if idx >= 0 && g.entryEnabled(idx) {
			g.pressedIndex = idx
			g.invalidate(facet.DirtyProjection)
			return true
		}
		return false
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := g.pressedIndex == idx && idx >= 0
		g.pressedIndex = -1
		g.invalidate(facet.DirtyProjection)
		if wasPressed && g.entryEnabled(idx) {
			g.activateItem(idx)
			return true
		}
		return wasPressed
	default:
		return false
	}
}

func (g *ActionGroup) onKey(e facet.KeyEvent) bool {
	if g.Disabled.Get() {
		return false
	}
	switch e.Key {
	case platform.KeyLeft, platform.KeyRight, platform.KeyHome, platform.KeyEnd:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			g.navigate(e.Key)
			return true
		}
	case platform.KeyEnter, platform.KeySpace:
		if e.Kind == platform.KeyRelease {
			if g.focusedIndex >= 0 {
				g.activateItem(g.focusedIndex)
				return true
			}
		}
		return e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat || e.Kind == platform.KeyRelease
	}
	return false
}

func (g *ActionGroup) onFocusGained() {
	g.focusedVisible = !g.focusFromPointer
	g.focusFromPointer = false
	if g.focusedIndex < 0 {
		g.focusedIndex = g.firstEnabledIndex()
	}
	g.invalidate(facet.DirtyProjection)
}

func (g *ActionGroup) onFocusLost() {
	g.focusedVisible = false
	g.pressedIndex = -1
	g.focusFromPointer = false
	g.hoveredIndex = -1
	g.invalidate(facet.DirtyProjection)
}

func (g *ActionGroup) interactionState() theme.InteractionState {
	switch {
	case g.Disabled.Get():
		return theme.StateDisabled
	case g.pressedIndex >= 0:
		return theme.StatePressed
	case g.hoveredIndex >= 0:
		return theme.StateHover
	case g.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (g *ActionGroup) itemState(index int) theme.InteractionState {
	if index < 0 || index >= len(g.cachedItemLayouts) {
		return theme.StateDefault
	}
	item := g.cachedItemLayouts[index].item
	switch {
	case item.Disabled:
		return theme.StateDisabled
	case g.pressedIndex == index:
		return theme.StatePressed
	case g.hoveredIndex == index:
		return theme.StateHover
	case g.focusedVisible && g.focusedIndex == index:
		return theme.StateFocused
	case item.Active:
		return theme.StateSelected
	default:
		return theme.StateDefault
	}
}

func (g *ActionGroup) pointInFocusRing(p gfx.Point) bool {
	if !g.Layout.ArrangedBounds.Contains(p) {
		return false
	}
	inset := mathutil.Max(1, g.Layout.ArrangedBounds.Height()*0.08)
	inner := g.Layout.ArrangedBounds.Inset(inset, inset)
	if inner.IsEmpty() {
		return true
	}
	return !inner.Contains(p)
}

func (g *ActionGroup) indexAt(p gfx.Point) int {
	for i := range g.cachedItemLayouts {
		if g.cachedItemLayouts[i].bounds.Contains(p) {
			return i
		}
	}
	return -1
}

func (g *ActionGroup) separatorAt(p gfx.Point) int {
	for i := range g.cachedSeparatorBounds {
		if g.cachedSeparatorBounds[i].Contains(p) {
			return i
		}
	}
	return -1
}

func (g *ActionGroup) entryEnabled(index int) bool {
	if index < 0 || index >= len(g.cachedItemLayouts) {
		return false
	}
	return !g.cachedItemLayouts[index].item.Disabled
}

func (g *ActionGroup) activateItem(index int) {
	if !g.entryEnabled(index) {
		return
	}
	item := g.cachedItemLayouts[index].item
	g.Activated.Emit(actionGroupItemKey(item))
}

func (g *ActionGroup) navigate(key platform.Key) {
	if len(g.cachedItemLayouts) == 0 {
		return
	}
	if g.focusedIndex < 0 {
		g.focusedIndex = g.firstEnabledIndex()
	}
	reversed := g.cachedWritingDirection == facet.WritingDirectionRTL
	switch key {
	case platform.KeyHome:
		g.focusedIndex = g.firstEnabledIndex()
	case platform.KeyEnd:
		g.focusedIndex = g.lastEnabledIndex()
	case platform.KeyLeft:
		if reversed {
			g.stepForward()
		} else {
			g.stepBackward()
		}
	case platform.KeyRight:
		if reversed {
			g.stepBackward()
		} else {
			g.stepForward()
		}
	}
	g.invalidate(facet.DirtyProjection)
}

func (g *ActionGroup) stepBackward() {
	for i := g.focusedIndex - 1; i >= 0; i-- {
		if g.entryEnabled(i) {
			g.focusedIndex = i
			return
		}
	}
}

func (g *ActionGroup) stepForward() {
	for i := g.focusedIndex + 1; i < len(g.cachedItemLayouts); i++ {
		if g.entryEnabled(i) {
			g.focusedIndex = i
			return
		}
	}
}

func (g *ActionGroup) firstEnabledIndex() int {
	for i := range g.cachedItemLayouts {
		if g.entryEnabled(i) {
			return i
		}
	}
	return -1
}

func (g *ActionGroup) lastEnabledIndex() int {
	for i := len(g.cachedItemLayouts) - 1; i >= 0; i-- {
		if g.entryEnabled(i) {
			return i
		}
	}
	return -1
}

func (g *ActionGroup) syncFocusIndex() {
	if g.focusedIndex >= 0 && g.focusedIndex < len(g.cachedItemLayouts) && g.entryEnabled(g.focusedIndex) {
		return
	}
	g.focusedIndex = g.firstEnabledIndex()
}

func normalizeActionGroupActions(actions []ActionGroupAction) []ActionGroupAction {
	if len(actions) == 0 {
		return nil
	}
	out := make([]ActionGroupAction, len(actions))
	for i := range actions {
		out[i] = normalizeActionGroupItem(actions[i])
	}
	return out
}

func normalizeActionGroupItem(item ActionGroupAction) ActionGroupAction {
	item.Key = strings.TrimSpace(item.Key)
	item.Label = strings.TrimSpace(item.Label)
	item.AccessibleLabel = strings.TrimSpace(item.AccessibleLabel)
	item.IconRef = strings.TrimSpace(item.IconRef)
	if item.Key == "" {
		switch {
		case item.AccessibleLabel != "":
			item.Key = item.AccessibleLabel
		case item.Label != "":
			item.Key = item.Label
		}
	}
	if item.AccessibleLabel == "" {
		if item.Label != "" {
			item.AccessibleLabel = item.Label
		} else {
			item.AccessibleLabel = item.Key
		}
	}
	return item
}

func actionGroupItemKey(item ActionGroupAction) string {
	if name := strings.TrimSpace(item.Key); name != "" {
		return name
	}
	if name := strings.TrimSpace(item.AccessibleLabel); name != "" {
		return name
	}
	return strings.TrimSpace(item.Label)
}

func unionRect(a, b gfx.Rect) gfx.Rect {
	if a.IsEmpty() {
		return b
	}
	if b.IsEmpty() {
		return a
	}
	if b.Min.X < a.Min.X {
		a.Min.X = b.Min.X
	}
	if b.Min.Y < a.Min.Y {
		a.Min.Y = b.Min.Y
	}
	if b.Max.X > a.Max.X {
		a.Max.X = b.Max.X
	}
	if b.Max.Y > a.Max.Y {
		a.Max.Y = b.Max.Y
	}
	return a
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type actionGroupPolicy struct {
	group *ActionGroup
}

func (actionGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }

func (p actionGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.group == nil {
		return facet.GroupMeasureResult{}, nil
	}
	size := p.group.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}}).Size
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p actionGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.group == nil {
		return nil, nil
	}
	p.group.arrange(ctx.Bounds)
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
