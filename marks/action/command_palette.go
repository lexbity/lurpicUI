package action

import (
	"sort"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
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
	commandPaletteMarkIDModalSurface  facet.MarkID = 3
	commandPaletteMarkIDSearchField   facet.MarkID = 4
	commandPaletteMarkIDResultsList   facet.MarkID = 5
	commandPaletteMarkIDFocusRing     facet.MarkID = 6
)

// CommandPalette implements the action.command_palette standard mark.
type CommandPalette struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	focusRole      facet.FocusRole
	textRole       facet.TextRole

	Activated signal.Signal[string]

	Label     string
	Placeholder string
	Open      bool
	Disabled  bool

	searchField *input.TextField
	resultsList  *commandPaletteResultsGroup
	registry     *runtimepkg.CommandRegistry

	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool
	activeIndex      int
	query            string

	cachedTokens          theme.Tokens
	cachedRecipe          shared.CommandPaletteSlots
	cachedRootBounds      gfx.Rect
	cachedBackdropBounds  gfx.Rect
	cachedSurfaceBounds   gfx.Rect
	cachedSearchBounds    gfx.Rect
	cachedResultsBounds   gfx.Rect
	cachedFocusBounds     gfx.Rect
	cachedPadX            float32
	cachedPadY            float32
	cachedGap             float32
	cachedSurfaceRadius   float32
	cachedWritingDirection facet.WritingDirection
	cachedCommands        []runtimepkg.CommandEntry
	cachedFiltered        []runtimepkg.CommandEntry
	cachedSearchSub       signal.SubscriptionID
	cachedRegistrySub     signal.SubscriptionID
	cachedResultsSub      signal.SubscriptionID
	cachedSearchKey       func(facet.KeyEvent) bool
}

var _ facet.FacetImpl = (*CommandPalette)(nil)
var _ layout.AnchorExporter = (*CommandPalette)(nil)

// NewCommandPalette constructs an action.command_palette mark with canonical defaults.
func NewCommandPalette(label string, registry *runtimepkg.CommandRegistry) *CommandPalette {
	p := &CommandPalette{
		Facet:        facet.NewFacet(),
		Label:        label,
		Placeholder:  "Type a command or search",
		Open:         true,
		registry:     registry,
		focusedVisible: true,
		activeIndex:  -1,
		Activated:    signal.NewSignal[string]("command_palette_activated"),
	}
	p.searchField = input.NewTextField("Search", uiinput.TextInputOutlined)
	p.searchField.SetPlaceholder(p.Placeholder)
	p.resultsList = newCommandPaletteResultsGroup(p)
	p.resultsList.SetDisabled(p.Disabled || !p.Open)
	p.resultsList.SetLabel("Command results")
	p.resultsList.SetEmptyState("No matching commands")
	p.resultsList.ItemVariant = uiinput.ListItemStandard

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
			if p.Disabled || !p.Open {
				return false
			}
			p.SetOpen(false)
			return true
		}
	}
	if role := p.resultsList.Base().InputRole(); role != nil {
		role.OnDismiss = func(e facet.DismissEvent) bool {
			_ = e
			if p.Disabled || !p.Open {
				return false
			}
			p.SetOpen(false)
			return true
		}
	}
	p.cachedResultsSub = p.resultsList.Activated.Subscribe(func(index int) {
		p.activateAt(index)
	})

	p.layoutRole.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   commandPaletteGroupPolicy{palette: p},
		Children: p,
		Overflow: facet.OverflowClip,
		Clipping: facet.GroupClipBounds,
	}
	p.layoutRole.Child = facet.GroupChildContract{
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
	p.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		size := p.measure(ctx, constraints)
		return facet.MeasureResult{
			Size:        size,
			Intrinsic:   facet.IntrinsicSize{Min: size, Preferred: size, Max: size},
			Constraints: constraints,
		}
	}
	p.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		p.layoutRole.ArrangedBounds = bounds
		p.arrange(ctx, bounds)
	}
	p.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := p.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	p.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := p.buildCommands(p.layoutRole.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	p.hitRole.OnHitTest = func(pt gfx.Point) facet.HitResult { return p.hitTest(pt) }
	p.inputRole.OnPointer = func(e facet.PointerEvent) bool { return p.onPointer(e) }
	p.inputRole.OnKey = func(e facet.KeyEvent) bool { return p.onKey(e) }
	p.inputRole.OnDismiss = func(e facet.DismissEvent) bool { return p.onDismiss(e) }
	p.focusRole.Focusable = func() bool { return !p.Disabled && p.Open }
	p.focusRole.TabIndex = -1
	p.focusRole.OnFocusGained = func() { p.onFocusGained() }
	p.focusRole.OnFocusLost = func() { p.onFocusLost() }
	p.textRole.IMEEnabled = false
	p.AddRole(&p.layoutRole)
	p.AddRole(&p.renderRole)
	p.AddRole(&p.projectionRole)
	p.AddRole(&p.hitRole)
	p.AddRole(&p.inputRole)
	p.AddRole(&p.focusRole)
	p.AddRole(&p.textRole)
	p.syncCommands()
	p.syncChildren()
	return p
}

// Base satisfies facet.FacetImpl.
func (p *CommandPalette) Base() *facet.Facet {
	p.Facet.BindImpl(p)
	return &p.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (p *CommandPalette) AccessibilityRole() string { return "dialog_combobox" }

// AccessibleName reports the semantic name source required by the spec.
func (p *CommandPalette) AccessibleName() string {
	if p == nil {
		return ""
	}
	if name := strings.TrimSpace(p.Label); name != "" {
		return name
	}
	return "Command palette"
}

// SetLabel updates the authored accessible label.
func (p *CommandPalette) SetLabel(label string) {
	if p == nil || p.Label == label {
		return
	}
	p.Label = label
	p.invalidate(facet.DirtyProjection)
}

// SetPlaceholder updates the search placeholder.
func (p *CommandPalette) SetPlaceholder(placeholder string) {
	if p == nil || p.Placeholder == placeholder {
		return
	}
	p.Placeholder = placeholder
	if p.searchField != nil {
		p.searchField.SetPlaceholder(placeholder)
	}
	p.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetOpen toggles the modal open state.
func (p *CommandPalette) SetOpen(open bool) {
	if p == nil || p.Open == open {
		return
	}
	p.Open = open
	if p.searchField != nil {
		p.searchField.SetDisabled(p.Disabled || !p.Open)
	}
	if p.resultsList != nil {
		p.resultsList.SetDisabled(p.Disabled || !p.Open)
	}
	if !open {
		p.hovered = false
		p.pressed = false
		p.focusedVisible = false
	} else if !p.Disabled {
		p.focusedVisible = true
	}
	p.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetDisabled toggles disabled state.
func (p *CommandPalette) SetDisabled(disabled bool) {
	if p == nil || p.Disabled == disabled {
		return
	}
	p.Disabled = disabled
	if p.searchField != nil {
		p.searchField.SetDisabled(disabled || !p.Open)
	}
	if p.resultsList != nil {
		p.resultsList.SetDisabled(disabled || !p.Open)
	}
	if disabled {
		p.hovered = false
		p.pressed = false
		p.focusedVisible = false
		p.SetOpen(false)
	}
	p.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// ExportAnchors publishes the command palette anchor set.
func (p *CommandPalette) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if p == nil {
		return nil
	}
	bounds := p.layoutRole.ArrangedBounds
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
	if p == nil {
		return
	}
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
func (p *CommandPalette) OnActivate() {}

// OnDeactivate is unused.
func (p *CommandPalette) OnDeactivate() {}

// OnDetach clears cached projection state.
func (p *CommandPalette) OnDetach() {
	if p != nil && p.resultsList != nil && p.cachedResultsSub != 0 {
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

func (p *CommandPalette) invalidate(flags facet.DirtyFlags) {
	if p == nil {
		return
	}
	p.Base().Invalidate(flags)
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
		p.searchField.SetDisabled(p.Disabled || !p.Open)
		p.searchField.SetPlaceholder(p.Placeholder)
		if value := p.searchField.Value; value != nil && value.Get() != p.query {
			value.Set(p.query)
		}
	}
	if p.resultsList != nil {
		p.resultsList.SetDisabled(p.Disabled || !p.Open)
		p.resultsList.SetEmptyState("No matching commands")
		if len(p.cachedFiltered) == 0 {
			p.resultsList.SetEntries(nil, p.activeIndex)
			return
		}
		p.resultsList.SetEntries(p.cachedFiltered, p.activeIndex)
	}
}

func (p *CommandPalette) measure(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	if p == nil || p.Disabled || !p.Open {
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
	innerMaxW := maxFloat(320, maxW-(p.cachedPadX*2))
	innerMaxH := maxFloat(240, maxH-(p.cachedPadY*2))
	searchSize := p.searchField.Base().LayoutRole().Measure(ctx, facet.Constraints{
		MaxSize: gfx.Size{W: innerMaxW, H: innerMaxH},
	}).Size
	listSize := p.resultsList.Base().LayoutRole().Measure(ctx, facet.Constraints{
		MaxSize: gfx.Size{W: innerMaxW, H: maxFloat(0, innerMaxH-searchSize.H-p.cachedGap)},
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
	if p.Disabled || !p.Open || p.searchField == nil || p.resultsList == nil || bounds.IsEmpty() {
		return
	}
	surfaceW, surfaceH := p.surfaceSize(bounds)
	surfaceX := bounds.Min.X + (bounds.Width()-surfaceW)*0.5
	surfaceY := bounds.Min.Y + maxFloat(p.cachedPadY, (bounds.Height()-surfaceH)*0.2)
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
	listH := maxFloat(0, inner.Max.Y-listY)
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
	contentW := maxFloat(search.W, results.W)
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
	if p == nil || bounds.IsEmpty() || p.Disabled || !p.Open {
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
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(backdrop) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), backdrop)...)
	}
	if !isTransparentMaterial(surface) && !p.cachedSurfaceBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(p.cachedSurfaceBounds, p.cachedSurfaceRadius), surface)...)
	}
	if p.focusedVisible && !isTransparentMaterial(focusRing) && !p.cachedSurfaceBounds.IsEmpty() {
		ringInset := maxFloat(1, p.cachedGap*0.5)
		ringBounds := p.cachedSurfaceBounds.Inset(-ringInset, -ringInset)
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ringBounds, p.cachedSurfaceRadius+ringInset), focusRing)...)
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
	case p.Disabled:
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
	if p == nil || p.Disabled || !p.Open {
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
		p.SetOpen(false)
		return true
	default:
		return false
	}
}

func (p *CommandPalette) onPointer(e facet.PointerEvent) bool {
	if p == nil || p.Disabled || !p.Open {
		return false
	}
	if e.Kind != platform.PointerPress {
		return false
	}
	hit := p.hitRole.HitTest(e.Position)
	if !hit.Hit {
		return false
	}
	if hit.MarkID == commandPaletteMarkIDBackdrop {
		p.SetOpen(false)
		return true
	}
	return hit.MarkID == commandPaletteMarkIDModalSurface || hit.MarkID == commandPaletteMarkIDSearchField || hit.MarkID == commandPaletteMarkIDResultsList
}

func (p *CommandPalette) onKey(e facet.KeyEvent) bool {
	if p == nil || p.Disabled || !p.Open {
		return false
	}
	if e.Kind != platform.KeyPress {
		return false
	}
	if e.Key == platform.KeyEscape {
		p.SetOpen(false)
		return true
	}
	return false
}

func (p *CommandPalette) onDismiss(e facet.DismissEvent) bool {
	_ = e
	if p == nil || p.Disabled || !p.Open {
		return false
	}
	p.SetOpen(false)
	return true
}

func (p *CommandPalette) onFocusGained() {
	if p == nil || p.Disabled {
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
	if p == nil || p.Disabled || !p.Open || p.cachedRootBounds.IsEmpty() {
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
			p.SetOpen(false)
			return
		}
	}
	if !entry.Disabled {
		p.Activated.Emit(entry.ID)
		p.SetOpen(false)
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
