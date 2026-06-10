package navigation

import (
	"sort"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uinav"
)

const (
	navRailMarkIDRoot            facet.MarkID = 1
	navRailMarkIDRailSurface     facet.MarkID = 2
	navRailMarkIDNavItems        facet.MarkID = 3
	navRailMarkIDActiveIndicator facet.MarkID = 4
	navRailMarkIDIcon            facet.MarkID = 5
	navRailMarkIDLabel           facet.MarkID = 6
	navRailMarkIDFocusRing       facet.MarkID = 7
)

// NavRailItem describes one navigation destination entry.
type NavRailItem struct {
	Key      string
	Label    string
	IconRef  string
	Disabled bool
}

// NavRail implements the navigation.nav_rail canonical mark.
type NavRail struct {
	marks.Core

	Label       marks.Binding[string]
	Items       []NavRailItem
	Collapsed   marks.Binding[bool]
	Disabled    marks.Binding[bool]
	ActiveIndex marks.Binding[int]

	Activated signal.Signal[int]

	textRole facet.TextRole

	hoveredIndex     int
	pressedIndex     int
	focusedIndex     int
	focusedVisible   bool
	focusFromPointer bool

	cachedTokens           theme.Tokens
	cachedRecipe           shared.NavRailSlots
	cachedRootBounds       gfx.Rect
	cachedRailBounds       gfx.Rect
	cachedItemBounds       []gfx.Rect
	cachedItemFacets       []*selection.ListItem
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedRailRadius       float32
	cachedWritingDirection facet.WritingDirection
}

var _ facet.FacetImpl = (*NavRail)(nil)
var _ layout.AnchorExporter = (*NavRail)(nil)
var _ marks.Mark = (*NavRail)(nil)

// NewNavRail constructs a navigation.nav_rail mark with canonical defaults.
func NewNavRail(label string, items []NavRailItem) *NavRail {
	r := &NavRail{
		Label:        marks.Const(label),
		Collapsed:    marks.Const(false),
		Disabled:     marks.Const(false),
		ActiveIndex:  marks.Const(-1),
		focusedIndex: -1,
		hoveredIndex: -1,
		pressedIndex: -1,
	}
	r.Core.Facet = facet.NewFacet()
	r.AddBinding(r.Label)
	r.AddBinding(r.Collapsed)
	r.AddBinding(r.Disabled)
	r.AddBinding(r.ActiveIndex)
	r.SetItems(items)
	r.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   navRailGroupPolicy{rail: r},
		Children: r,
	}
	r.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := r.measureIntrinsic(ctx, constraints)
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionClip,
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
	r.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return r.measure(ctx, constraints)
	}
	r.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		r.Layout.ArrangedBounds = bounds
		r.arrange(ctx, bounds)
	}
	r.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return r.hitTest(p) }
	r.Input.OnPointer = func(e facet.PointerEvent) bool { return r.onPointer(e) }
	r.Input.OnKey = func(e facet.KeyEvent) bool { return r.onKey(e) }
	r.Focus.Focusable = func() bool { return !r.Disabled.Get() && len(r.cachedItemFacets) > 0 }
	r.Focus.TabIndex = 0
	r.Focus.OnFocusGained = func() { r.onFocusGained() }
	r.Focus.OnFocusLost = func() { r.onFocusLost() }
	r.textRole.IMEEnabled = false
	r.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return r.buildCommands(r.Layout.ArrangedBounds, ctx.Runtime)
	}
	r.RegisterRoles()
	r.AddRole(&r.textRole)
	return r
}

// Base satisfies facet.FacetImpl.
func (r *NavRail) Base() *facet.Facet {
	r.Facet.BindImpl(r)
	return &r.Facet
}

// Descriptor satisfies marks.Mark.
func (r *NavRail) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "navigation", TypeName: "nav_rail"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (r *NavRail) AccessibilityRole() string { return "navigation" }

// AccessibleName reports the semantic name source required by the spec.
func (r *NavRail) AccessibleName() string { return r.Label.Get() }

// SetItems updates the rail destinations.
func (r *NavRail) SetItems(items []NavRailItem) {
	if r == nil {
		return
	}
	next := append([]NavRailItem(nil), items...)
	for i := range next {
		next[i].Key = strings.TrimSpace(next[i].Key)
		next[i].Label = strings.TrimSpace(next[i].Label)
		next[i].IconRef = strings.TrimSpace(next[i].IconRef)
	}
	r.Items = next
	r.rebuildChildFacets()
	r.clampIndices()
	r.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// ExportAnchors publishes the rail anchor set.
func (r *NavRail) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if r == nil {
		return nil
	}
	bounds := r.Layout.ArrangedBounds
	out := r.Core.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	for i := range r.cachedItemFacets {
		itemLayout := r.cachedItemFacets[i].Base().LayoutRole()
		if itemLayout != nil && !itemLayout.ArrangedBounds.IsEmpty() {
			out[layout.AnchorID("item_"+strings.TrimSpace(r.Items[i].Key))] = gfx.Point{
				X: (itemLayout.ArrangedBounds.Min.X + itemLayout.ArrangedBounds.Max.X) * 0.5,
				Y: (itemLayout.ArrangedBounds.Min.Y + itemLayout.ArrangedBounds.Max.Y) * 0.5,
			}
		}
	}
	if len(r.cachedItemFacets) > 0 {
		idx := r.clampedActiveIndex()
		if idx < 0 || idx >= len(r.cachedItemFacets) {
			idx = 0
		}
		if idx >= 0 && idx < len(r.cachedItemFacets) {
			layoutRole := r.cachedItemFacets[idx].Base().LayoutRole()
			if layoutRole != nil && !layoutRole.ArrangedBounds.IsEmpty() {
				out["baseline"] = gfx.Point{X: layoutRole.ArrangedBounds.Min.X, Y: layoutRole.ArrangedBounds.Min.Y}
			}
		}
	}
	return out
}

// Children returns the facet's immediate child list.
func (r *NavRail) Children() []facet.GroupChild {
	if r == nil {
		return nil
	}
	r.rebuildChildFacets()
	out := make([]facet.GroupChild, 0, len(r.cachedItemFacets))
	for i := range r.cachedItemFacets {
		item := r.cachedItemFacets[i]
		if item == nil {
			continue
		}
		base := item.Base()
		layoutRole := base.LayoutRole()
		if layoutRole == nil {
			continue
		}
		out = append(out, facet.GroupChild{
			FacetID:    base.ID(),
			MarkID:     navRailMarkIDNavItems,
			Attachment: facet.Attachment{Placement: facet.Placement{Mode: facet.PlacementLinear, Linear: facet.LinearPlacement{Order: i, CrossAxisAlign: facet.CrossAxisStretch}}},
			Layout:     layoutRole,
			Contract:   layoutRole.Child,
		})
	}
	return out
}

// OnAttach is unused beyond layout role setup.
func (r *NavRail) OnAttach(ctx facet.AttachContext) { r.Core.OnAttach() }

// OnActivate is unused.
func (r *NavRail) OnActivate() { r.Core.OnActivate() }

// OnDeactivate is unused.
func (r *NavRail) OnDeactivate() { r.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (r *NavRail) OnDetach() {
	r.Core.OnDetach()
	r.cachedTokens = theme.Tokens{}
	r.cachedRecipe = shared.NavRailSlots{}
	r.cachedRootBounds = gfx.Rect{}
	r.cachedRailBounds = gfx.Rect{}
	r.cachedItemBounds = nil
	r.cachedPadX = 0
	r.cachedPadY = 0
	r.cachedGap = 0
	r.cachedRailRadius = 0
}

func (r *NavRail) invalidate(flags facet.DirtyFlags) {
	if r == nil {
		return
	}
	r.Facet.Invalidate(flags)
}

func (r *NavRail) rebuildChildFacets() {
	if r == nil {
		return
	}
	if len(r.cachedItemFacets) != len(r.Items) {
		r.cachedItemFacets = make([]*selection.ListItem, len(r.Items))
	}
	for i := range r.Items {
		if r.cachedItemFacets[i] == nil {
			r.cachedItemFacets[i] = selection.NewListItem(marks.Const(r.Items[i].Label))
		}
		item := r.cachedItemFacets[i]
		item.Label = marks.Const(r.Items[i].Label)
		item.LeadingIconRef = marks.Const(r.Items[i].IconRef)
		item.Selected = marks.Const(i == r.clampedActiveIndex())
		item.Disabled = marks.Const(r.Disabled.Get() || r.Items[i].Disabled)
		item.ShowLabel = marks.Const(!r.Collapsed.Get())
		item.ShowContainer = marks.Const(false)
		item.ShowLeadingIcon = marks.Const(true)
		item.ShowSelectionIndicator = marks.Const(false)
		item.ShowFocusRing = marks.Const(false)
	}
}

func (r *NavRail) syncChildState() {
	if r == nil {
		return
	}
	for i := range r.cachedItemFacets {
		item := r.cachedItemFacets[i]
		if item == nil {
			continue
		}
		item.Selected = marks.Const(i == r.clampedActiveIndex())
		item.Disabled = marks.Const(r.Disabled.Get() || r.Items[i].Disabled)
		item.ShowLabel = marks.Const(!r.Collapsed.Get())
		item.ShowContainer = marks.Const(false)
		item.ShowLeadingIcon = marks.Const(true)
		item.ShowSelectionIndicator = marks.Const(false)
		item.ShowFocusRing = marks.Const(false)
	}
}

func (r *NavRail) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uinav.ResolveNavRailRecipe(style)
	r.cachedTokens = resolved.TokenSet()
	r.cachedRecipe = slots
	r.cachedWritingDirection = ctx.WritingDirection
	r.cachedPadX = mathutil.Max(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	r.cachedPadY = mathutil.Max(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	r.cachedGap = float32(resolved.Spacing(theme.SpacingS))
	r.cachedRailRadius = float32(resolved.Radius(theme.RadiusL))
	r.rebuildChildFacets()
	r.syncChildState()
	grp := navRailGroupPolicy{rail: r}
	groupSize, _ := grp.MeasureGroup(facet.GroupMeasureContext{MeasureContext: ctx}, r.Children())
	minWidth := resolved.Density.Scale(224)
	if r.Collapsed.Get() {
		minWidth = resolved.Density.Scale(72)
	}
	measured := constraints.Constrain(gfx.Size{
		W: mathutil.Max(minWidth, groupSize.Size.W+r.cachedPadX*2),
		H: mathutil.Max(resolved.Density.Scale(96), groupSize.Size.H+r.cachedPadY*2),
	})
	r.Layout.MeasuredSize = measured
	r.Layout.MeasuredResult = facet.MeasureResult{
		Size: measured,
		Intrinsic: facet.IntrinsicSize{
			Min:       measured,
			Preferred: measured,
			Max:       measured,
		},
		Constraints: constraints,
	}
	r.textRole.Layout = nil
	return r.Layout.MeasuredResult
}

func (r *NavRail) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return r.measure(ctx, constraints).Size
}

func (r *NavRail) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	r.cachedRootBounds = bounds
	r.cachedRailBounds = bounds
	r.cachedItemBounds = nil
	r.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	r.rebuildChildFacets()
	r.syncChildState()
	contentBounds := bounds.Inset(r.cachedPadX, r.cachedPadY)
	if contentBounds.IsEmpty() {
		contentBounds = bounds
	}
	policy := navRailGroupPolicy{rail: r}
	arranged, _ := policy.ArrangeGroup(facet.GroupArrangeContext{ArrangeContext: ctx, Bounds: contentBounds}, r.Children())
	r.cachedItemBounds = make([]gfx.Rect, 0, len(arranged))
	for _, child := range arranged {
		r.cachedItemBounds = append(r.cachedItemBounds, child.Bounds)
	}
}

func (r *NavRail) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.NavRailSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: r.cachedTokens}, r.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, r.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uinav.ResolveNavRailRecipe(style)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: r.cachedTokens}, r.cachedRecipe
}

func (r *NavRail) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if r == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := r.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	root := slots.Root.Resolve(r.rootState(), tokens)
	rail := slots.RailSurface.Resolve(theme.StateDefault, tokens)
	active := slots.ActiveIndicator.Resolve(theme.StateSelected, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)
	cmds := make([]gfx.Command, 0, 64)
	if !theme.IsTransparentMaterial(root) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(bounds), root)...)
	}
	if !theme.IsTransparentMaterial(rail) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(r.cachedRailBounds, r.cachedRailRadius), rail)...)
	}
	for i := range r.cachedItemFacets {
		if i >= len(r.cachedItemBounds) || r.cachedItemBounds[i].IsEmpty() {
			continue
		}
		state := r.itemStateAt(i)
		row := r.cachedItemBounds[i]
		if i == r.clampedActiveIndex() && !theme.IsTransparentMaterial(active) {
			indicatorBounds := row.Inset(row.Width()*0.08, row.Height()*0.18)
			if r.Collapsed.Get() {
				sz := mathutil.Max(24, mathutil.Min(indicatorBounds.Width(), indicatorBounds.Height()))
				indicatorBounds = gfx.RectFromXYWH(row.Min.X+(row.Width()-sz)*0.5, row.Min.Y+(row.Height()-sz)*0.5, sz, sz)
			}
			cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(indicatorBounds, r.cachedRailRadius*0.65), active)...)
		}
		switch state {
		case theme.StateHover:
			nav := slots.NavItems.Resolve(state, tokens)
			if !theme.IsTransparentMaterial(nav) {
				cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(row.Inset(row.Width()*0.08, row.Height()*0.12), r.cachedRailRadius*0.55), nav)...)
			}
		case theme.StatePressed:
			nav := slots.NavItems.Resolve(state, tokens)
			if !theme.IsTransparentMaterial(nav) {
				cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(row.Inset(row.Width()*0.08, row.Height()*0.12), r.cachedRailRadius*0.55), nav)...)
			}
		}
		childRuntime := runtimeServicesOrNil(runtime)
		childCmds := r.cachedItemFacets[i].Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: childRuntime, Bounds: row, ContentScale: 1})
		if childCmds != nil {
			cmds = append(cmds, childCmds.Commands...)
		}
		if state == theme.StateFocused && !theme.IsTransparentMaterial(focus) {
			inset := mathutil.Max(1, row.Height()*0.08)
			cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(row.Inset(inset, inset), r.cachedRailRadius), focus)...)
		}
	}
	return cmds
}

func (r *NavRail) hitTest(p gfx.Point) facet.HitResult {
	if r == nil || r.Layout.ArrangedBounds.IsEmpty() || !r.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := r.cursorShape()
	if r.focusedVisible && r.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: navRailMarkIDFocusRing, Cursor: cursor}
	}
	for i := range r.cachedItemBounds {
		if i >= len(r.cachedItemBounds) || !r.cachedItemBounds[i].Contains(p) {
			continue
		}
		if i == r.clampedActiveIndex() {
			return facet.HitResult{Hit: true, MarkID: navRailMarkIDActiveIndicator, Cursor: cursor}
		}
		return facet.HitResult{Hit: true, MarkID: navRailMarkIDNavItems, Cursor: r.cursorForItem(i)}
	}
	if r.cachedRailBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: navRailMarkIDRailSurface, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: navRailMarkIDRoot, Cursor: cursor}
}

func (r *NavRail) onPointer(e facet.PointerEvent) bool {
	if r.Disabled.Get() {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		r.hoveredIndex = r.indexAt(e.Position)
		r.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		r.hoveredIndex = -1
		if r.pressedIndex < 0 {
			r.focusFromPointer = false
		}
		r.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		idx := r.indexAt(e.Position)
		if idx >= 0 && !r.isDisabledIndex(idx) {
			r.hoveredIndex = idx
			r.pressedIndex = idx
			r.focusFromPointer = true
			r.focusedVisible = false
			r.invalidate(facet.DirtyProjection)
			return true
		}
		return false
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := r.pressedIndex >= 0
		idx := r.pressedIndex
		r.pressedIndex = -1
		r.invalidate(facet.DirtyProjection)
		if wasPressed {
			if hit := r.indexAt(e.Position); hit >= 0 && hit == idx && !r.isDisabledIndex(hit) {
				r.activateIndex(hit)
				return true
			}
			return true
		}
		return false
	case platform.PointerMove:
		idx := r.indexAt(e.Position)
		if idx != r.hoveredIndex {
			r.hoveredIndex = idx
			r.invalidate(facet.DirtyProjection)
		}
		return true
	default:
		return false
	}
}

func (r *NavRail) onKey(e facet.KeyEvent) bool {
	if r.Disabled.Get() || len(r.cachedItemFacets) == 0 {
		return false
	}
	switch e.Key {
	case platform.KeyUp, platform.KeyDown, platform.KeyHome, platform.KeyEnd, platform.KeySpace, platform.KeyEnter:
		switch e.Kind {
		case platform.KeyPress, platform.KeyRepeat:
			switch e.Key {
			case platform.KeyUp:
				r.moveFocus(-1)
				return true
			case platform.KeyDown:
				r.moveFocus(1)
				return true
			case platform.KeyHome:
				r.setFirstFocus()
				return true
			case platform.KeyEnd:
				r.setLastFocus()
				return true
			case platform.KeySpace, platform.KeyEnter:
				r.pressedIndex = r.clampedFocusedIndex()
				r.invalidate(facet.DirtyProjection)
				return true
			}
		case platform.KeyRelease:
			if e.Key == platform.KeySpace || e.Key == platform.KeyEnter {
				wasPressed := r.pressedIndex >= 0
				idx := r.pressedIndex
				r.pressedIndex = -1
				r.invalidate(facet.DirtyProjection)
				if wasPressed && idx >= 0 {
					r.activateIndex(idx)
					return true
				}
			}
		}
	}
	return false
}

func (r *NavRail) onFocusGained() {
	r.focusedVisible = !r.focusFromPointer
	r.focusFromPointer = false
	r.focusedIndex = r.firstEnabledIndex()
	r.invalidate(facet.DirtyProjection)
}

func (r *NavRail) onFocusLost() {
	r.focusedVisible = false
	r.pressedIndex = -1
	r.focusFromPointer = false
	r.invalidate(facet.DirtyProjection)
}

func (r *NavRail) rootState() theme.InteractionState {
	if r.Disabled.Get() {
		return theme.StateDisabled
	}
	if r.pressedIndex >= 0 {
		return theme.StatePressed
	}
	if r.hoveredIndex >= 0 {
		return theme.StateHover
	}
	if r.focusedVisible {
		return theme.StateFocused
	}
	return theme.StateDefault
}

func (r *NavRail) itemStateAt(index int) theme.InteractionState {
	if r.Disabled.Get() || r.isDisabledIndex(index) {
		return theme.StateDisabled
	}
	if r.pressedIndex == index {
		return theme.StatePressed
	}
	if r.hoveredIndex == index {
		return theme.StateHover
	}
	if r.focusedVisible && r.clampedFocusedIndex() == index {
		return theme.StateFocused
	}
	if index == r.clampedActiveIndex() {
		return theme.StateSelected
	}
	return theme.StateDefault
}

func (r *NavRail) moveFocus(delta int) {
	if len(r.cachedItemFacets) == 0 {
		return
	}
	start := r.clampedFocusedIndex()
	for step := 1; step <= len(r.cachedItemFacets); step++ {
		next := start + delta*step
		for next < 0 {
			next += len(r.cachedItemFacets)
		}
		next %= len(r.cachedItemFacets)
		if !r.isDisabledIndex(next) {
			r.focusedIndex = next
			r.invalidate(facet.DirtyProjection)
			return
		}
	}
}

func (r *NavRail) setFirstFocus() {
	if idx := r.firstEnabledIndex(); idx >= 0 {
		r.focusedIndex = idx
		r.invalidate(facet.DirtyProjection)
	}
}

func (r *NavRail) setLastFocus() {
	for i := len(r.cachedItemFacets) - 1; i >= 0; i-- {
		if !r.isDisabledIndex(i) {
			r.focusedIndex = i
			r.invalidate(facet.DirtyProjection)
			return
		}
	}
}

func (r *NavRail) firstEnabledIndex() int {
	for i := range r.cachedItemFacets {
		if !r.isDisabledIndex(i) {
			return i
		}
	}
	return r.clampedActiveIndex()
}

func (r *NavRail) activateIndex(index int) {
	if index < 0 || index >= len(r.cachedItemFacets) || r.isDisabledIndex(index) {
		return
	}
	r.ActiveIndex = marks.Const(index)
	r.syncChildState()
	r.Activated.Emit(index)
	r.invalidate(facet.DirtyProjection)
}

func (r *NavRail) clampedActiveIndex() int {
	idx := r.ActiveIndex.Get()
	if len(r.cachedItemFacets) == 0 {
		return -1
	}
	if idx < 0 {
		return -1
	}
	if idx >= len(r.cachedItemFacets) {
		return len(r.cachedItemFacets) - 1
	}
	return idx
}

func (r *NavRail) clampedFocusedIndex() int {
	if len(r.cachedItemFacets) == 0 {
		return 0
	}
	if r.focusedIndex < 0 {
		return 0
	}
	if r.focusedIndex >= len(r.cachedItemFacets) {
		return len(r.cachedItemFacets) - 1
	}
	return r.focusedIndex
}

func (r *NavRail) clampIndices() {
	if len(r.cachedItemFacets) == 0 {
		r.ActiveIndex = marks.Const(-1)
		r.focusedIndex = -1
		return
	}
	ai := r.ActiveIndex.Get()
	if ai >= len(r.cachedItemFacets) {
		r.ActiveIndex = marks.Const(len(r.cachedItemFacets) - 1)
	}
	if r.focusedIndex < 0 || r.focusedIndex >= len(r.cachedItemFacets) {
		r.focusedIndex = r.firstEnabledIndex()
	}
	if r.isDisabledIndex(r.focusedIndex) {
		r.focusedIndex = r.firstEnabledIndex()
	}
}

func (r *NavRail) isDisabledIndex(index int) bool {
	if index < 0 || index >= len(r.cachedItemFacets) {
		return true
	}
	return r.Disabled.Get() || r.Items[index].Disabled
}

func (r *NavRail) indexAt(p gfx.Point) int {
	for i := range r.cachedItemBounds {
		if r.cachedItemBounds[i].Contains(p) {
			return i
		}
	}
	return -1
}

func (r *NavRail) pointInFocusRing(p gfx.Point) bool {
	if !r.focusedVisible || len(r.cachedItemBounds) == 0 {
		return false
	}
	idx := r.clampedFocusedIndex()
	if idx < 0 || idx >= len(r.cachedItemBounds) {
		return false
	}
	active := r.cachedItemBounds[idx]
	if active.IsEmpty() || !active.Contains(p) {
		return false
	}
	ring := mathutil.Max(1, active.Height()*0.08)
	inner := active.Inset(ring, ring)
	if inner.IsEmpty() {
		return true
	}
	return !inner.Contains(p)
}

func (r *NavRail) cursorShape() facet.CursorShape {
	if r.Disabled.Get() {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (r *NavRail) cursorForItem(index int) facet.CursorShape {
	if r.Disabled.Get() || r.isDisabledIndex(index) {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

type navRailGroupPolicy struct {
	rail *NavRail
}

func (navRailGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (p navRailGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.rail == nil || len(children) == 0 {
		return facet.GroupMeasureResult{}, nil
	}
	ordered := orderedNavRailChildren(children)
	width := float32(0)
	height := float32(0)
	for i, idx := range ordered {
		child := children[idx]
		if child.Layout == nil {
			continue
		}
		size := child.Layout.MeasuredSize
		if size == (gfx.Size{}) {
			size = child.Layout.Measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}}).Size
		}
		width = mathutil.Max(width, size.W)
		height += size.H
		if i < len(ordered)-1 {
			height += p.rail.cachedGap
		}
	}
	return facet.GroupMeasureResult{Size: gfx.Size{W: width, H: height}}, nil
}

func (p navRailGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.rail == nil || len(children) == 0 {
		return nil, nil
	}
	ordered := orderedNavRailChildren(children)
	y := ctx.Bounds.Min.Y
	arranged := make([]facet.ArrangedGroupChild, 0, len(ordered))
	for i, idx := range ordered {
		child := children[idx]
		if child.Layout == nil {
			continue
		}
		size := child.Layout.MeasuredSize
		if size == (gfx.Size{}) {
			size = gfx.Size{W: ctx.Bounds.Width(), H: p.rail.childHeightHint(idx)}
		}
		rect := gfx.RectFromXYWH(ctx.Bounds.Min.X, y, ctx.Bounds.Width(), size.H)
		child.Layout.Arrange(facet.ArrangeContext{Placement: child.Attachment.Placement}, rect)
		arranged = append(arranged, facet.ArrangedGroupChild{
			FacetID:   child.FacetID,
			MarkID:    child.MarkID,
			Bounds:    rect,
			Placement: child.Attachment.Placement,
			ZPriority: child.Attachment.ZPriority,
			Contract:  child.Contract,
		})
		y += rect.Height()
		if i < len(ordered)-1 {
			y += p.rail.cachedGap
		}
	}
	return arranged, nil
}

func (r *NavRail) childHeightHint(index int) float32 {
	if r == nil {
		return 56
	}
	if index >= 0 && index < len(r.cachedItemFacets) {
		if item := r.cachedItemFacets[index]; item != nil {
			if size := item.Base().LayoutRole().MeasuredSize; size.H > 0 {
				return size.H
			}
			if item.ShowLabel.Get() {
				return 56
			}
			return 72
		}
	}
	if r.Collapsed.Get() {
		return 72
	}
	return 56
}

func runtimeServicesOrNil(runtime any) facet.RuntimeServices {
	if runtime == nil {
		return nil
	}
	services, ok := runtime.(facet.RuntimeServices)
	if !ok {
		return nil
	}
	return services
}

func orderedNavRailChildren(children []facet.GroupChild) []int {
	indices := make([]int, len(children))
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(i, j int) bool {
		left := children[indices[i]]
		right := children[indices[j]]
		if left.Attachment.Placement.Linear.Order != right.Attachment.Placement.Linear.Order {
			return left.Attachment.Placement.Linear.Order < right.Attachment.Placement.Linear.Order
		}
		if left.Attachment.ZPriority != right.Attachment.ZPriority {
			return left.Attachment.ZPriority > right.Attachment.ZPriority
		}
		return false
	})
	return indices
}
