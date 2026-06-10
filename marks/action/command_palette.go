package action

import (
	"sort"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/platform"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiaction"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

const (
	commandPaletteMarkIDRoot         facet.MarkID = 1
	commandPaletteMarkIDBackdrop     facet.MarkID = 2
	commandPaletteMarkIDModalSurface facet.MarkID = 3
	commandPaletteMarkIDSearchField  facet.MarkID = 4
	commandPaletteMarkIDResultsList  facet.MarkID = 5
	commandPaletteMarkIDFocusRing    facet.MarkID = 6
)

// CommandPalette implements the action.command_palette standard mark.
type CommandPalette struct {
	marks.Core

	Label       marks.Binding[string]
	Placeholder marks.Binding[string]
	Disabled    marks.Binding[bool]

	Activated signal.Signal[string]

	textRole facet.TextRole

	searchField *input.TextField
	resultsList *commandPaletteResultsGroup
	registry    *runtimepkg.CommandRegistry

	Open             bool
	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool
	activeIndex      int
	query            string

	cachedTokens           theme.Tokens
	cachedRecipe           shared.CommandPaletteSlots
	cachedRootBounds       gfx.Rect
	cachedBackdropBounds   gfx.Rect
	cachedSurfaceBounds    gfx.Rect
	cachedSearchBounds     gfx.Rect
	cachedResultsBounds    gfx.Rect
	cachedFocusBounds      gfx.Rect
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedSurfaceRadius    float32
	cachedWritingDirection facet.WritingDirection
	cachedCommands         []runtimepkg.CommandEntry
	cachedFiltered         []runtimepkg.CommandEntry
	cachedSearchSub        signal.SubscriptionID
	cachedRegistrySub      signal.SubscriptionID
	cachedResultsSub       signal.SubscriptionID
	cachedSearchKey        func(facet.KeyEvent) bool
}

var _ facet.FacetImpl = (*CommandPalette)(nil)
var _ layout.AnchorExporter = (*CommandPalette)(nil)
var _ marks.Mark = (*CommandPalette)(nil)

// NewCommandPalette constructs an action.command_palette mark with canonical defaults.
func NewCommandPalette(label marks.Binding[string], registry *runtimepkg.CommandRegistry) *CommandPalette {
	p := &CommandPalette{
		Label:          label,
		Placeholder:    marks.Const("Type a command or search"),
		Disabled:       marks.Const(false),
		registry:       registry,
		Open:           true,
		focusedVisible: true,
		activeIndex:    -1,
		Activated:      signal.NewSignal[string]("command_palette_activated"),
	}
	p.Facet = facet.NewFacet()
	p.AddBinding(p.Label)
	p.AddBinding(p.Placeholder)
	p.AddBinding(p.Disabled)

	p.searchField = input.NewTextField("Search", uiinput.TextInputOutlined)
	p.searchField.Placeholder = marks.Const(p.Placeholder.Get())
	p.resultsList = newCommandPaletteResultsGroup(p)
	p.resultsList.Disabled = marks.Const(p.Disabled.Get() || !p.Open)
	p.resultsList.ItemVariant = marks.Const(uiinput.ListItemStandard)

	if role := p.searchField.Base().InputRole(); role != nil {
		p.cachedSearchKey = role.OnKey
		role.OnKey = func(e facet.KeyEvent) bool {
			if p.onSearchKey(e) {
				return true
			}
			if p.cachedSearchKey != nil && p.cachedSearchKey(e) {
				return true
			}
			return false
		}
		role.OnDismiss = func(e facet.DismissEvent) bool {
			_ = e
			if p.Disabled.Get() || !p.Open {
				return false
			}
			p.Open = false
			return true
		}
	}
	if role := p.resultsList.Base().InputRole(); role != nil {
		role.OnDismiss = func(e facet.DismissEvent) bool {
			_ = e
			if p.Disabled.Get() || !p.Open {
				return false
			}
			p.Open = false
			return true
		}
	}
	p.cachedResultsSub = p.resultsList.Activated.Subscribe(func(index int) {
		p.activateAt(index)
	})

	p.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   commandPaletteGroupPolicy{palette: p},
		Children: p,
		Overflow: facet.OverflowClip,
		Clipping: facet.GroupClipBounds,
	}
	p.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := p.measure(ctx, constraints)
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
	p.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		size := p.measure(ctx, constraints)
		return facet.MeasureResult{
			Size:        size,
			Intrinsic:   facet.IntrinsicSize{Min: size, Preferred: size, Max: size},
			Constraints: constraints,
		}
	}
	p.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		p.Layout.ArrangedBounds = bounds
		p.arrange(ctx, bounds)
	}
	p.Render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := p.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	p.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return p.buildCommands(p.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	p.Hit.OnHitTest = func(pt gfx.Point) facet.HitResult { return p.hitTest(pt) }
	p.Input.OnPointer = func(e facet.PointerEvent) bool { return p.onPointer(e) }
	p.Input.OnKey = func(e facet.KeyEvent) bool { return p.onKey(e) }
	p.Input.OnDismiss = func(e facet.DismissEvent) bool { return p.onDismiss(e) }
	p.Focus.Focusable = func() bool { return !p.Disabled.Get() && p.Open }
	p.Focus.TabIndex = -1
	p.Focus.OnFocusGained = func() { p.onFocusGained() }
	p.Focus.OnFocusLost = func() { p.onFocusLost() }
	p.textRole.IMEEnabled = false
	p.RegisterRoles()
	p.AddRole(&p.textRole)
	p.syncCommands()
	p.syncChildren()
	return p
}

// Base satisfies facet.FacetImpl.
func (p *CommandPalette) Base() *facet.Facet {
	p.BindImpl(p)
	return &p.Facet
}

// Descriptor satisfies marks.Mark.
func (p *CommandPalette) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "action", TypeName: "command_palette"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (p *CommandPalette) AccessibilityRole() string { return "dialog_combobox" }

// AccessibleName reports the semantic name source required by the spec.
func (p *CommandPalette) AccessibleName() string {
	if p == nil {
		return ""
	}
	if name := strings.TrimSpace(p.Label.Get()); name != "" {
		return name
	}
	return "Command palette"
}

// Children returns the facet's immediate child list.
func (p *CommandPalette) Children() []facet.GroupChild {
	if p == nil || !p.Open {
		return nil
	}
	out := make([]facet.GroupChild, 0, 2)
	if p.searchField != nil && p.searchField.Base() != nil && p.searchField.Base().LayoutRole() != nil {
		out = append(out, commandPaletteGroupChild(p.searchField.Base(), commandPaletteMarkIDSearchField, 0))
	}
	if p.resultsList != nil && p.resultsList.Base() != nil && p.resultsList.Base().LayoutRole() != nil {
		out = append(out, commandPaletteGroupChild(p.resultsList.Base(), commandPaletteMarkIDResultsList, 1))
	}
	return out
}

// OnAttach wires command registry and query invalidation.
func (p *CommandPalette) OnAttach(ctx facet.AttachContext) {
	p.Core.OnAttach()
	p.syncCommands()
	if p.searchField != nil && p.searchField.Value != nil {
		facet.Store(facet.Subscribe(p), &p.searchField.Value.OnChange, p.searchField.Value.Version, func(change signal.Change[string]) {
			p.syncFromQuery(change.New)
		})
	}
	if p.registry != nil {
		facet.Store(facet.Subscribe(p), &p.registry.OnChange, p.registry.Version, func(signal.Unit) {
			p.syncCommands()
		})
	}
}

// OnActivate is unused.
func (p *CommandPalette) OnActivate() { p.Core.OnActivate() }

// OnDeactivate is unused.
func (p *CommandPalette) OnDeactivate() { p.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (p *CommandPalette) OnDetach() {
	p.Core.OnDetach()
	if p.resultsList != nil && p.cachedResultsSub != 0 {
		p.resultsList.Activated.Unsubscribe(p.cachedResultsSub)
	}
	p.cachedTokens = theme.Tokens{}
	p.cachedRecipe = shared.CommandPaletteSlots{}
	p.cachedRootBounds = gfx.Rect{}
	p.cachedBackdropBounds = gfx.Rect{}
	p.cachedSurfaceBounds = gfx.Rect{}
	p.cachedSearchBounds = gfx.Rect{}
	p.cachedResultsBounds = gfx.Rect{}
	p.cachedFocusBounds = gfx.Rect{}
	p.cachedCommands = nil
	p.cachedFiltered = nil
	p.cachedSearchSub = 0
	p.cachedRegistrySub = 0
	p.cachedResultsSub = 0
}

// ExportAnchors publishes the command palette anchor set.
func (p *CommandPalette) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if p == nil {
		return nil
	}
	bounds := p.Layout.ArrangedBounds
	out := p.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	if !p.cachedSurfaceBounds.IsEmpty() {
		out["content_anchor"] = gfx.Point{
			X: p.cachedSurfaceBounds.Min.X + p.cachedSurfaceBounds.Width()*0.5,
			Y: p.cachedSurfaceBounds.Min.Y + p.cachedPadY,
		}
	} else {
		out["content_anchor"] = bounds.Min
	}
	out["baseline"] = out["content_anchor"]
	return out
}

func (p *CommandPalette) invalidate(flags facet.DirtyFlags) {
	if p == nil {
		return
	}
	p.Invalidate(flags)
}

func (p *CommandPalette) syncCommands() {
	if p == nil {
		return
	}
	if p.registry != nil {
		p.cachedCommands = p.registry.Snapshot()
	} else {
		p.cachedCommands = nil
	}
	p.syncFromQuery(p.query)
}

func (p *CommandPalette) syncFromQuery(query string) {
	if p == nil {
		return
	}
	p.query = query
	filtered := filterCommandEntries(p.cachedCommands, query)
	p.cachedFiltered = filtered
	if len(filtered) == 0 {
		p.activeIndex = -1
	} else if p.activeIndex < 0 || p.activeIndex >= len(filtered) {
		p.activeIndex = 0
	}
	p.syncChildren()
	p.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func (p *CommandPalette) syncChildren() {
	if p == nil {
		return
	}
	if p.searchField != nil {
		p.searchField.Disabled = marks.Const(p.Disabled.Get() || !p.Open)
		p.searchField.Placeholder = marks.Const(p.Placeholder.Get())
		if value := p.searchField.Value; value != nil && value.Get() != p.query {
			value.Set(p.query)
		}
	}
	if p.resultsList != nil {
		p.resultsList.Disabled = marks.Const(p.Disabled.Get() || !p.Open)
		p.resultsList.EmptyState = marks.Const("No matching commands")
		if len(p.cachedFiltered) == 0 {
			p.resultsList.syncRows(nil, p.activeIndex)
			return
		}
		p.resultsList.syncRows(p.cachedFiltered, p.activeIndex)
	}
}

func (p *CommandPalette) measure(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	if p == nil || p.Disabled.Get() || !p.Open {
		return constraints.Constrain(gfx.Size{})
	}
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uiaction.ResolveCommandPaletteRecipe(style)
	p.cachedTokens = resolved.TokenSet()
	p.cachedRecipe = slots
	p.cachedWritingDirection = ctx.WritingDirection
	p.cachedPadX = float32(resolved.Spacing(theme.SpacingL))
	p.cachedPadY = float32(resolved.Spacing(theme.SpacingM))
	p.cachedGap = float32(resolved.Spacing(theme.SpacingS))
	p.cachedSurfaceRadius = float32(resolved.Radius(theme.RadiusL))
	if p.searchField == nil || p.resultsList == nil {
		return constraints.Constrain(gfx.Size{})
	}
	p.syncCommands()
	maxW := constraints.MaxSize.W
	maxH := constraints.MaxSize.H
	if maxW <= 0 {
		maxW = 720
	}
	if maxH <= 0 {
		maxH = 540
	}
	innerMaxW := mathutil.Max(320, maxW-(p.cachedPadX*2))
	innerMaxH := mathutil.Max(240, maxH-(p.cachedPadY*2))
	searchSize := p.searchField.Base().LayoutRole().Measure(ctx, facet.Constraints{
		MaxSize: gfx.Size{W: innerMaxW, H: innerMaxH},
	}).Size
	listSize := p.resultsList.Base().LayoutRole().Measure(ctx, facet.Constraints{
		MaxSize: gfx.Size{W: innerMaxW, H: mathutil.Max(0, innerMaxH-searchSize.H-p.cachedGap)},
	}).Size
	_ = searchSize
	_ = listSize
	return constraints.Constrain(gfx.Size{W: maxW, H: maxH})
}

func (p *CommandPalette) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if p == nil {
		return
	}
	p.cachedRootBounds = bounds
	p.cachedBackdropBounds = bounds
	p.cachedSearchBounds = gfx.Rect{}
	p.cachedResultsBounds = gfx.Rect{}
	p.cachedFocusBounds = gfx.Rect{}
	if p.Disabled.Get() || !p.Open || p.searchField == nil || p.resultsList == nil || bounds.IsEmpty() {
		return
	}
	surfaceW, surfaceH := p.surfaceSize(bounds)
	surfaceX := bounds.Min.X + (bounds.Width()-surfaceW)*0.5
	surfaceY := bounds.Min.Y + mathutil.Max(p.cachedPadY, (bounds.Height()-surfaceH)*0.2)
	p.cachedSurfaceBounds = gfx.RectFromXYWH(surfaceX, surfaceY, surfaceW, surfaceH)
	inner := p.cachedSurfaceBounds.Inset(p.cachedPadX, p.cachedPadY)
	searchSize := p.searchField.Base().LayoutRole().MeasuredSize
	if searchSize.W == 0 && searchSize.H == 0 {
		searchSize = gfx.Size{W: inner.Width(), H: 0}
	}
	searchRect := gfx.RectFromXYWH(inner.Min.X, inner.Min.Y, inner.Width(), searchSize.H)
	p.searchField.Base().LayoutRole().Arrange(ctx, searchRect)
	p.cachedSearchBounds = searchRect
	listY := searchRect.Max.Y + p.cachedGap
	listH := mathutil.Max(0, inner.Max.Y-listY)
	listRect := gfx.RectFromXYWH(inner.Min.X, listY, inner.Width(), listH)
	p.resultsList.Base().LayoutRole().Arrange(ctx, listRect)
	p.cachedResultsBounds = listRect
	p.cachedFocusBounds = p.cachedSurfaceBounds
}

func (p *CommandPalette) surfaceSize(bounds gfx.Rect) (float32, float32) {
	if p == nil {
		return 0, 0
	}
	search := p.searchField.Base().LayoutRole().MeasuredSize
	results := p.resultsList.Base().LayoutRole().MeasuredSize
	contentW := mathutil.Max(search.W, results.W)
	contentH := search.H
	if results.H > 0 {
		contentH += p.cachedGap + results.H
	}
	surfaceW := clampFloat(contentW+p.cachedPadX*2, 420, bounds.Width())
	surfaceH := clampFloat(contentH+p.cachedPadY*2, 280, bounds.Height())
	if surfaceW <= 0 {
		surfaceW = bounds.Width()
	}
	if surfaceH <= 0 {
		surfaceH = bounds.Height()
	}
	return surfaceW, surfaceH
}

func (p *CommandPalette) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if p == nil || bounds.IsEmpty() || p.Disabled.Get() || !p.Open {
		return nil
	}
	style, slots := p.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := p.interactionState()
	root := slots.Root.Resolve(state, tokens)
	backdrop := slots.Backdrop.Resolve(theme.StateDefault, tokens)
	surface := slots.ModalSurface.Resolve(state, tokens)
	focusRing := slots.FocusRing.Resolve(theme.StateFocused, tokens)
	cmds := make([]gfx.Command, 0, 32)
	if !theme.IsTransparentMaterial(root) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(bounds), root)...)
	}
	if !theme.IsTransparentMaterial(backdrop) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(bounds), backdrop)...)
	}
	if !theme.IsTransparentMaterial(surface) && !p.cachedSurfaceBounds.IsEmpty() {
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(p.cachedSurfaceBounds, p.cachedSurfaceRadius), surface)...)
	}
	if p.focusedVisible && !theme.IsTransparentMaterial(focusRing) && !p.cachedSurfaceBounds.IsEmpty() {
		ringInset := mathutil.Max(1, p.cachedGap*0.5)
		ringBounds := p.cachedSurfaceBounds.Inset(ringInset, ringInset)
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(ringBounds, p.cachedSurfaceRadius), focusRing)...)
	}
	if !p.cachedSurfaceBounds.IsEmpty() {
		cmds = append(cmds, gfx.PushClipRect{Rect: p.cachedSurfaceBounds})
		if p.searchField != nil && !p.cachedSearchBounds.IsEmpty() {
			if projected := p.searchField.Base().ProjectionRole().Project(facet.ProjectionContext{
				Runtime:      runtimeServicesOrNil(runtime),
				Bounds:       p.cachedSearchBounds,
				ContentScale: contentScale,
			}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		if p.resultsList != nil && !p.cachedResultsBounds.IsEmpty() {
			if projected := p.resultsList.Base().ProjectionRole().Project(facet.ProjectionContext{
				Runtime:      runtimeServicesOrNil(runtime),
				Bounds:       p.cachedResultsBounds,
				ContentScale: contentScale,
			}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		cmds = append(cmds, gfx.PopClip{})
	}
	return cmds
}

func (p *CommandPalette) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.CommandPaletteSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: p.cachedTokens}, p.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, p.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uiaction.ResolveCommandPaletteRecipe(style)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: p.cachedTokens}, p.cachedRecipe
}

func (p *CommandPalette) interactionState() theme.InteractionState {
	switch {
	case p.Disabled.Get():
		return theme.StateDisabled
	case p.pressed:
		return theme.StatePressed
	case p.hovered:
		return theme.StateHover
	case p.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (p *CommandPalette) onSearchKey(e facet.KeyEvent) bool {
	if p == nil || p.Disabled.Get() || !p.Open {
		return false
	}
	if e.Kind != platform.KeyPress {
		return false
	}
	switch e.Key {
	case platform.KeyDown:
		p.moveActive(1)
		return true
	case platform.KeyUp:
		p.moveActive(-1)
		return true
	case platform.KeyPageDown:
		p.moveActive(5)
		return true
	case platform.KeyPageUp:
		p.moveActive(-5)
		return true
	case platform.KeyEnter:
		p.activateCurrent()
		return true
	case platform.KeyEscape:
		p.Open = false
		return true
	default:
		return false
	}
}

func (p *CommandPalette) onPointer(e facet.PointerEvent) bool {
	if p == nil || p.Disabled.Get() || !p.Open {
		return false
	}
	if e.Kind != platform.PointerPress {
		return false
	}
	hit := p.Hit.HitTest(e.Position)
	if !hit.Hit {
		return false
	}
	if hit.MarkID == commandPaletteMarkIDBackdrop {
		p.Open = false
		return true
	}
	return hit.MarkID == commandPaletteMarkIDModalSurface || hit.MarkID == commandPaletteMarkIDSearchField || hit.MarkID == commandPaletteMarkIDResultsList
}

func (p *CommandPalette) onKey(e facet.KeyEvent) bool {
	if p == nil || p.Disabled.Get() || !p.Open {
		return false
	}
	if e.Kind != platform.KeyPress {
		return false
	}
	if e.Key == platform.KeyEscape {
		p.Open = false
		return true
	}
	return false
}

func (p *CommandPalette) onDismiss(e facet.DismissEvent) bool {
	_ = e
	if p == nil || p.Disabled.Get() || !p.Open {
		return false
	}
	p.Open = false
	return true
}

func (p *CommandPalette) onFocusGained() {
	if p == nil || p.Disabled.Get() {
		return
	}
	p.focusedVisible = true
}

func (p *CommandPalette) onFocusLost() {
	if p == nil {
		return
	}
	p.focusedVisible = false
}

func (p *CommandPalette) hitTest(pt gfx.Point) facet.HitResult {
	if p == nil || p.Disabled.Get() || !p.Open || p.cachedRootBounds.IsEmpty() {
		return facet.HitResult{}
	}
	if !p.cachedSurfaceBounds.IsEmpty() && p.cachedSurfaceBounds.Contains(pt) {
		if !p.cachedSearchBounds.IsEmpty() && p.cachedSearchBounds.Contains(pt) {
			return facet.HitResult{Hit: true, MarkID: commandPaletteMarkIDSearchField, Cursor: facet.CursorText}
		}
		if !p.cachedResultsBounds.IsEmpty() && p.cachedResultsBounds.Contains(pt) {
			return facet.HitResult{Hit: true, MarkID: commandPaletteMarkIDResultsList, Cursor: facet.CursorPointer}
		}
		return facet.HitResult{Hit: true, MarkID: commandPaletteMarkIDModalSurface, Cursor: facet.CursorDefault}
	}
	return facet.HitResult{Hit: true, MarkID: commandPaletteMarkIDBackdrop, Cursor: facet.CursorDefault}
}

func (p *CommandPalette) moveActive(delta int) {
	if p == nil || len(p.cachedFiltered) == 0 {
		return
	}
	if p.activeIndex < 0 {
		p.activeIndex = 0
	} else {
		p.activeIndex = clampInt(p.activeIndex+delta, 0, len(p.cachedFiltered)-1)
	}
	p.syncChildren()
	p.invalidate(facet.DirtyProjection)
}

func (p *CommandPalette) activateCurrent() {
	if p == nil || len(p.cachedFiltered) == 0 {
		return
	}
	if p.activeIndex < 0 || p.activeIndex >= len(p.cachedFiltered) {
		return
	}
	p.activateEntry(p.cachedFiltered[p.activeIndex])
}

func (p *CommandPalette) activateAt(index int) {
	if p == nil || index < 0 || index >= len(p.cachedFiltered) {
		return
	}
	p.activeIndex = index
	p.syncChildren()
	p.invalidate(facet.DirtyProjection)
	p.activateEntry(p.cachedFiltered[index])
}

func (p *CommandPalette) activateEntry(entry runtimepkg.CommandEntry) {
	if p == nil || entry.ID == "" {
		return
	}
	if p.registry != nil && !entry.Disabled {
		if p.registry.Execute(entry.ID) {
			p.Activated.Emit(entry.ID)
			p.Open = false
			return
		}
	}
	if !entry.Disabled {
		p.Activated.Emit(entry.ID)
		p.Open = false
	}
}

func filterCommandEntries(entries []runtimepkg.CommandEntry, query string) []runtimepkg.CommandEntry {
	if len(entries) == 0 {
		return nil
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	type scoredEntry struct {
		entry runtimepkg.CommandEntry
		score int
	}
	out := make([]scoredEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Hidden {
			continue
		}
		score, ok := scoreCommandEntry(entry, needle)
		if !ok {
			continue
		}
		out = append(out, scoredEntry{entry: entry, score: score})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		if out[i].entry.Category != out[j].entry.Category {
			return out[i].entry.Category < out[j].entry.Category
		}
		if out[i].entry.Title != out[j].entry.Title {
			return out[i].entry.Title < out[j].entry.Title
		}
		return out[i].entry.ID < out[j].entry.ID
	})
	filtered := make([]runtimepkg.CommandEntry, len(out))
	for i := range out {
		filtered[i] = out[i].entry
	}
	return filtered
}

func scoreCommandEntry(entry runtimepkg.CommandEntry, needle string) (int, bool) {
	if needle == "" {
		return 1, true
	}
	searchSpaces := []string{
		strings.ToLower(entry.ID),
		strings.ToLower(entry.Title),
		strings.ToLower(entry.Category),
		strings.ToLower(entry.Shortcut),
	}
	best := 0
	for _, space := range searchSpaces {
		if space == "" {
			continue
		}
		if strings.HasPrefix(space, needle) {
			if len(space) < len(needle) {
				continue
			}
			best = maxInt(best, 100-len(space)+len(needle))
			continue
		}
		if strings.Contains(space, needle) {
			best = maxInt(best, 40)
		}
	}
	for _, kw := range entry.Keywords {
		kw = strings.ToLower(strings.TrimSpace(kw))
		if kw == "" {
			continue
		}
		if strings.HasPrefix(kw, needle) {
			best = maxInt(best, 80)
			continue
		}
		if strings.Contains(kw, needle) {
			best = maxInt(best, 30)
		}
	}
	return best, best > 0
}

func paletteCommandTitle(entry runtimepkg.CommandEntry) string {
	title := strings.TrimSpace(entry.Title)
	if category := strings.TrimSpace(entry.Category); category != "" {
		if title == "" {
			return category
		}
		return category + ": " + title
	}
	return title
}

func commandPaletteGroupChild(base *facet.Facet, markID facet.MarkID, order int) facet.GroupChild {
	if base == nil || base.LayoutRole() == nil {
		return facet.GroupChild{}
	}
	return facet.GroupChild{
		FacetID: base.ID(),
		MarkID:  markID,
		Attachment: facet.Attachment{
			Placement: facet.Placement{
				Mode:   facet.PlacementLinear,
				Linear: facet.LinearPlacement{Order: order, CrossAxisAlign: facet.CrossAxisStretch},
			},
		},
		Layout:   base.LayoutRole(),
		Contract: base.LayoutRole().Child,
	}
}

type commandPaletteGroupPolicy struct {
	palette *CommandPalette
}

func (commandPaletteGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (p commandPaletteGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.palette == nil {
		return facet.GroupMeasureResult{}, nil
	}
	size := p.palette.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}})
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p commandPaletteGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.palette == nil {
		return nil, nil
	}
	p.palette.arrange(ctx.ArrangeContext, ctx.Bounds)
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

func clampFloat(v, minV, maxV float32) float32 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
