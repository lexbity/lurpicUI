package action

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiaction"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

const (
	ribbonMarkIDRoot             facet.MarkID = 1
	ribbonMarkIDRibbonSurface    facet.MarkID = 2
	ribbonMarkIDGroups           facet.MarkID = 3
	ribbonMarkIDGroupLabels      facet.MarkID = 4
	ribbonMarkIDActionItems      facet.MarkID = 5
	ribbonMarkIDOverflowControls facet.MarkID = 6
	ribbonMarkIDFocusRing        facet.MarkID = 7
)

// RibbonSection describes one ribbon tab and its associated toolbar collection.
type RibbonSection struct {
	Key      string
	Label    string
	IconRef  string
	Toolbars []*Toolbar
	Disabled bool
}

// Ribbon implements the action.ribbon canonical mark.
type Ribbon struct {
	marks.Core

	textRole facet.TextRole

	Activated signal.Signal[int]

	Label       string
	Sections    []RibbonSection
	ActiveIndex int
	Disabled    marks.Binding[bool]

	hoveredTabIndex     int
	pressedTabIndex     int
	focusedTabIndex     int
	focusedVisible      bool
	focusFromPointer    bool
	hoveredToolbarIndex int
	pressedToolbarIndex int

	cachedTokens           theme.Tokens
	cachedRecipe           shared.RibbonSlots
	cachedRootBounds       gfx.Rect
	cachedTabStripBounds   gfx.Rect
	cachedBodyBounds       gfx.Rect
	cachedSurfaceBounds    gfx.Rect
	cachedTabBounds        []gfx.Rect
	cachedToolbarBounds    []gfx.Rect
	cachedButtonMargins    []gfx.Rect
	cachedTabPadX          float32
	cachedTabPadY          float32
	cachedTabGap           float32
	cachedToolbarGap       float32
	cachedSectionGap       float32
	cachedRadius           float32
	cachedWritingDirection facet.WritingDirection
	cachedTabButtons       []*Button
	cachedSectionToolbars  []*Toolbar
	cachedActiveTabLayout  *text.TextLayout
}

var _ facet.FacetImpl = (*Ribbon)(nil)
var _ layout.AnchorExporter = (*Ribbon)(nil)
var _ marks.Mark = (*Ribbon)(nil)

// NewRibbon constructs an action.ribbon mark with canonical defaults.
func NewRibbon(label string, sections []RibbonSection) *Ribbon {
	r := &Ribbon{
		Core:                marks.Core{Facet: facet.NewFacet()},
		Label:               label,
		Sections:            normalizeRibbonSections(sections),
		Disabled:            marks.Const(false),
		focusedTabIndex:     -1,
		hoveredTabIndex:     -1,
		pressedTabIndex:     -1,
		hoveredToolbarIndex: -1,
		pressedToolbarIndex: -1,
		Activated:           signal.NewSignal[int]("ribbon_activated"),
	}
	r.AddBinding(r.Disabled)

	r.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   ribbonGroupPolicy{ribbon: r},
		Overflow: facet.OverflowClip,
		Clipping: facet.GroupClipBounds,
	}
	r.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := r.measureIntrinsic(ctx, constraints)
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
	r.Focus.Focusable = func() bool { return !r.Disabled.Get() && len(r.Sections) > 0 }
	r.Focus.OnFocusGained = func() { r.onFocusGained() }
	r.Focus.OnFocusLost = func() { r.onFocusLost() }
	r.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return r.buildCommands(r.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	r.RegisterRoles()
	r.AddRole(&r.textRole)
	r.syncChildren()
	return r
}

// Base satisfies facet.FacetImpl.
func (r *Ribbon) Base() *facet.Facet {
	r.BindImpl(r)
	return &r.Facet
}

// Descriptor satisfies marks.Mark.
func (r *Ribbon) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "action", TypeName: "ribbon"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (r *Ribbon) AccessibilityRole() string { return "toolbar" }

// AccessibleName reports the semantic name source required by the spec.
func (r *Ribbon) AccessibleName() string {
	if r == nil {
		return ""
	}
	if name := strings.TrimSpace(r.Label); name != "" {
		return name
	}
	for _, section := range r.Sections {
		if name := strings.TrimSpace(section.Label); name != "" {
			return name
		}
	}
	return ""
}

// ExportAnchors publishes the ribbon anchor set.
func (r *Ribbon) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if r == nil {
		return nil
	}
	bounds := r.Layout.ArrangedBounds
	out := r.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	if r.textRole.Layout != nil {
		out["baseline"] = gfx.Point{
			X: bounds.Min.X,
			Y: bounds.Min.Y + r.textRole.Layout.Baseline,
		}
	} else if len(r.cachedTabButtons) > 0 && r.cachedTabButtons[0] != nil && r.cachedTabButtons[0].textRole.Layout != nil {
		out["baseline"] = gfx.Point{
			X: r.cachedTabButtons[0].cachedLabelBounds.Min.X,
			Y: r.cachedTabButtons[0].cachedLabelBounds.Min.Y + r.cachedTabButtons[0].textRole.Layout.Baseline,
		}
	}
	return out
}

// Children returns the ribbon's immediate child list.
func (r *Ribbon) Children() []facet.GroupChild {
	if r == nil {
		return nil
	}
	r.syncChildren()
	out := make([]facet.GroupChild, 0, len(r.cachedTabButtons)+len(r.cachedSectionToolbars))
	for i, btn := range r.cachedTabButtons {
		if btn == nil || btn.Base() == nil || btn.Base().LayoutRole() == nil {
			continue
		}
		out = append(out, facet.GroupChild{
			FacetID: btn.Base().ID(),
			MarkID:  ribbonMarkIDGroupLabels,
			Attachment: facet.Attachment{
				Placement: facet.Placement{
					Mode:   facet.PlacementLinear,
					Linear: facet.LinearPlacement{Order: i, CrossAxisAlign: facet.CrossAxisStretch},
				},
			},
			Layout:   btn.Base().LayoutRole(),
			Contract: btn.Base().LayoutRole().Child,
		})
	}
	section := r.activeSection()
	if section == nil {
		return out
	}
	for i, toolbar := range section.Toolbars {
		if toolbar == nil || toolbar.Base() == nil || toolbar.Base().LayoutRole() == nil {
			continue
		}
		out = append(out, facet.GroupChild{
			FacetID: toolbar.Base().ID(),
			MarkID:  ribbonMarkIDActionItems,
			Attachment: facet.Attachment{
				Placement: facet.Placement{
					Mode:   facet.PlacementLinear,
					Linear: facet.LinearPlacement{Order: len(r.cachedTabButtons) + i, CrossAxisAlign: facet.CrossAxisStretch},
				},
			},
			Layout:   toolbar.Base().LayoutRole(),
			Contract: toolbar.Base().LayoutRole().Child,
		})
	}
	return out
}

// OnAttach subscribes binding sources.
func (r *Ribbon) OnAttach(ctx facet.AttachContext) { r.Core.OnAttach() }

// OnActivate is unused.
func (r *Ribbon) OnActivate() { r.Core.OnActivate() }

// OnDeactivate is unused.
func (r *Ribbon) OnDeactivate() { r.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (r *Ribbon) OnDetach() {
	r.Core.OnDetach()
	r.cachedTokens = theme.Tokens{}
	r.cachedRecipe = shared.RibbonSlots{}
	r.cachedRootBounds = gfx.Rect{}
	r.cachedTabStripBounds = gfx.Rect{}
	r.cachedBodyBounds = gfx.Rect{}
	r.cachedSurfaceBounds = gfx.Rect{}
	r.cachedTabBounds = nil
	r.cachedToolbarBounds = nil
	r.cachedButtonMargins = nil
	r.cachedTabPadX = 0
	r.cachedTabPadY = 0
	r.cachedTabGap = 0
	r.cachedToolbarGap = 0
	r.cachedSectionGap = 0
	r.cachedRadius = 0
	r.cachedTabButtons = nil
	r.cachedSectionToolbars = nil
	r.cachedActiveTabLayout = nil
}

func (r *Ribbon) invalidate(flags facet.DirtyFlags) {
	if r == nil {
		return
	}
	r.Invalidate(flags)
}

func (r *Ribbon) syncChildren() {
	if r == nil {
		return
	}
	if len(r.Sections) == 0 {
		r.cachedTabButtons = nil
		r.cachedSectionToolbars = nil
		r.focusedTabIndex = -1
		r.ActiveIndex = 0
		return
	}
	if len(r.cachedTabButtons) != len(r.Sections) {
		next := make([]*Button, len(r.Sections))
		for i := range r.Sections {
			section := r.Sections[i]
			btn := NewButton(marks.Const(section.Label), marks.Const(uiinput.ButtonText))
			btn.LeadingIconRef = marks.Const(section.IconRef)
			btn.Disabled = marks.Const(r.Disabled.Get() || section.Disabled)
			index := i
			btn.Activated.Subscribe(func(signal.Unit) {
				r.activateSection(index, true)
			})
			next[i] = btn
		}
		r.cachedTabButtons = next
	} else {
		for i := range r.cachedTabButtons {
			btn := r.cachedTabButtons[i]
			if btn == nil {
				continue
			}
			section := r.Sections[i]
			btn.Label = marks.Const(section.Label)
			btn.LeadingIconRef = marks.Const(section.IconRef)
			{
				disabled := r.Disabled.Get() || section.Disabled
				btn.Disabled = marks.Const(disabled)
				if disabled {
					btn.hovered = false
					btn.pressed = false
					btn.spaceDown = false
					btn.enterDown = false
					btn.focusedVisible = false
				}
				btn.Invalidate(facet.DirtyProjection | facet.DirtyHit)
			}
		}
	}
	r.ActiveIndex = r.clampSectionIndex(r.ActiveIndex)
	r.focusedTabIndex = r.clampSectionIndex(r.focusedTabIndex)
	if r.focusedTabIndex < 0 {
		r.focusedTabIndex = r.ActiveIndex
	}
	r.syncButtonVariants()
	r.syncToolbarChildren()
}

func (r *Ribbon) syncButtonVariants() {
	for i, btn := range r.cachedTabButtons {
		if btn == nil {
			continue
		}
		if i == r.ActiveIndex {
			btn.Variant = marks.Const(uiinput.ButtonTonal)
			btn.Invalidate(facet.DirtyProjection)
		} else {
			btn.Variant = marks.Const(uiinput.ButtonText)
			btn.Invalidate(facet.DirtyProjection)
		}
	}
}

func (r *Ribbon) syncToolbarChildren() {
	if r == nil {
		return
	}
	r.cachedSectionToolbars = r.cachedSectionToolbars[:0]
	section := r.activeSection()
	if section == nil {
		return
	}
	for _, toolbar := range section.Toolbars {
		if toolbar == nil {
			continue
		}
		toolbar.Disabled = marks.Const(r.Disabled.Get() || section.Disabled)
		r.cachedSectionToolbars = append(r.cachedSectionToolbars, toolbar)
	}
}

func (r *Ribbon) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return r.measure(ctx, constraints).Size
}

func (r *Ribbon) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uiaction.ResolveRibbonRecipe(style)
	r.cachedTokens = resolved.TokenSet()
	r.cachedRecipe = slots
	r.cachedWritingDirection = ctx.WritingDirection
	r.textRole.Layout = nil
	r.cachedTabPadX = mathutil.Max(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	r.cachedTabPadY = mathutil.Max(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	r.cachedTabGap = mathutil.Max(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	r.cachedToolbarGap = mathutil.Max(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	r.cachedSectionGap = mathutil.Max(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	r.cachedRadius = float32(resolved.Radius(theme.RadiusM))

	r.syncChildren()
	if len(r.cachedTabButtons) == 0 {
		r.Layout.MeasuredResult = facet.MeasureResult{}
		return facet.MeasureResult{}
	}
	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = resolved.Density.Scale(1280)
	}
	tabSizes := make([]gfx.Size, len(r.cachedTabButtons))
	tabWidths := make([]float32, len(r.cachedTabButtons))
	tabHeights := make([]float32, len(r.cachedTabButtons))
	for i, btn := range r.cachedTabButtons {
		if btn == nil || btn.Base() == nil || btn.Base().LayoutRole() == nil {
			continue
		}
		size := btn.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: maxWidth, H: constraints.MaxSize.H}}).Size
		tabSizes[i] = size
		tabWidths[i] = size.W
		tabHeights[i] = size.H
	}
	tabRowW := float32(0)
	tabRowH := float32(0)
	for i := range tabWidths {
		tabRowW += tabWidths[i]
		if i > 0 {
			tabRowW += r.cachedTabGap
		}
		if tabHeights[i] > tabRowH {
			tabRowH = tabHeights[i]
		}
	}
	if tabRowH <= 0 {
		tabRowH = resolved.Density.Scale(32)
	}

	toolbarSizes := make([]gfx.Size, len(r.cachedSectionToolbars))
	toolbarWidths := make([]float32, len(r.cachedSectionToolbars))
	toolbarHeights := make([]float32, len(r.cachedSectionToolbars))
	toolbarBandW := float32(0)
	toolbarBandH := float32(0)
	for i, toolbar := range r.cachedSectionToolbars {
		if toolbar == nil || toolbar.Base() == nil || toolbar.Base().LayoutRole() == nil {
			continue
		}
		size := toolbar.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: maxWidth, H: constraints.MaxSize.H}}).Size
		toolbarSizes[i] = size
		toolbarWidths[i] = size.W
		toolbarHeights[i] = size.H
		toolbarBandW += size.W
		if i > 0 {
			toolbarBandW += r.cachedToolbarGap
		}
		if size.H > toolbarBandH {
			toolbarBandH = size.H
		}
	}
	if len(toolbarSizes) > 0 {
		toolbarBandH = mathutil.Max(toolbarBandH, resolved.Density.Scale(40))
	}
	contentH := toolbarBandH
	if contentH > 0 {
		contentH += r.cachedSectionGap
	}
	naturalWidth := mathutil.Max(tabRowW, toolbarBandW) + r.cachedTabPadX*2
	naturalHeight := tabRowH + r.cachedTabPadY*2
	if contentH > 0 {
		naturalHeight += contentH
	}
	measured := constraints.Constrain(gfx.Size{W: naturalWidth, H: naturalHeight})
	r.cachedTabBounds = make([]gfx.Rect, len(r.cachedTabButtons))
	r.cachedToolbarBounds = make([]gfx.Rect, len(r.cachedSectionToolbars))
	r.cachedButtonMargins = make([]gfx.Rect, len(r.cachedTabButtons))
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
	_ = tabSizes
	_ = toolbarSizes
	return r.Layout.MeasuredResult
}

func (r *Ribbon) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	r.cachedRootBounds = bounds
	r.cachedTabStripBounds = gfx.Rect{}
	r.cachedBodyBounds = gfx.Rect{}
	r.cachedSurfaceBounds = gfx.Rect{}
	r.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	inner := bounds.Inset(r.cachedTabPadX, r.cachedTabPadY)
	tabY := inner.Min.Y
	tabHeights := make([]float32, len(r.cachedTabButtons))
	for i, btn := range r.cachedTabButtons {
		if btn == nil || btn.Base() == nil || btn.Base().LayoutRole() == nil {
			continue
		}
		size := btn.Base().LayoutRole().MeasuredSize
		if size.H > tabHeights[i] {
			tabHeights[i] = size.H
		}
	}
	tabRowH := float32(0)
	for _, h := range tabHeights {
		if h > tabRowH {
			tabRowH = h
		}
	}
	if tabRowH <= 0 {
		tabRowH = mathutil.Max(bounds.Height()*0.18, 32)
	}
	r.cachedTabStripBounds = gfx.RectFromXYWH(inner.Min.X, tabY, inner.Width(), tabRowH)
	curX := inner.Min.X
	if r.cachedWritingDirection == facet.WritingDirectionRTL {
		curX = inner.Max.X
	}
	for i, btn := range r.cachedTabButtons {
		if btn == nil || btn.Base() == nil || btn.Base().LayoutRole() == nil {
			continue
		}
		size := btn.Base().LayoutRole().MeasuredSize
		if r.cachedWritingDirection == facet.WritingDirectionRTL {
			curX -= size.W
			rect := gfx.RectFromXYWH(curX, tabY, size.W, tabRowH)
			r.cachedTabBounds[i] = rect
			btn.Base().LayoutRole().Arrange(ctx, rect)
			curX -= r.cachedTabGap
		} else {
			rect := gfx.RectFromXYWH(curX, tabY, size.W, tabRowH)
			r.cachedTabBounds[i] = rect
			btn.Base().LayoutRole().Arrange(ctx, rect)
			curX += size.W + r.cachedTabGap
		}
	}
	toolbarY := tabY + tabRowH + r.cachedSectionGap
	curX = inner.Min.X
	if r.cachedWritingDirection == facet.WritingDirectionRTL {
		curX = inner.Max.X
	}
	toolbarRowH := float32(0)
	for _, toolbar := range r.cachedSectionToolbars {
		if toolbar == nil || toolbar.Base() == nil || toolbar.Base().LayoutRole() == nil {
			continue
		}
		size := toolbar.Base().LayoutRole().MeasuredSize
		if size.H > toolbarRowH {
			toolbarRowH = size.H
		}
	}
	for i, toolbar := range r.cachedSectionToolbars {
		if toolbar == nil || toolbar.Base() == nil || toolbar.Base().LayoutRole() == nil {
			continue
		}
		size := toolbar.Base().LayoutRole().MeasuredSize
		if r.cachedWritingDirection == facet.WritingDirectionRTL {
			curX -= size.W
			rect := gfx.RectFromXYWH(curX, toolbarY, size.W, toolbarRowH)
			r.cachedToolbarBounds[i] = rect
			toolbar.Base().LayoutRole().Arrange(ctx, rect)
			curX -= r.cachedToolbarGap
		} else {
			rect := gfx.RectFromXYWH(curX, toolbarY, size.W, toolbarRowH)
			r.cachedToolbarBounds[i] = rect
			toolbar.Base().LayoutRole().Arrange(ctx, rect)
			curX += size.W + r.cachedToolbarGap
		}
	}
	if toolbarRowH > 0 {
		r.cachedBodyBounds = gfx.RectFromXYWH(inner.Min.X, toolbarY, inner.Width(), toolbarRowH)
		r.cachedSurfaceBounds = r.cachedBodyBounds
	}
	active := r.activeSection()
	if active != nil && len(active.Toolbars) > 0 {
		for _, toolbar := range active.Toolbars {
			if toolbar == nil || toolbar.Base() == nil || toolbar.Base().TextRole() == nil {
				continue
			}
			if toolbar.Base().TextRole().Layout != nil {
				r.textRole.Layout = toolbar.Base().TextRole().Layout
				break
			}
		}
	}
	if r.textRole.Layout == nil && len(r.cachedTabButtons) > 0 {
		if btn := r.cachedTabButtons[r.clampSectionIndex(r.ActiveIndex)]; btn != nil && btn.Base() != nil && btn.Base().TextRole() != nil {
			r.textRole.Layout = btn.Base().TextRole().Layout
		}
	}
	r.focusedTabIndex = r.clampSectionIndex(r.focusedTabIndex)
}

func (r *Ribbon) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.RibbonSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: r.cachedTokens}, r.cachedRecipe
	}
	return theme.StyleContext{Tokens: r.cachedTokens}, r.cachedRecipe
}

func (r *Ribbon) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if r == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := r.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := theme.StateDefault
	if r.Disabled.Get() {
		state = theme.StateDisabled
	}
	root := slots.Root.Resolve(state, tokens)
	surface := slots.RibbonSurface.Resolve(state, tokens)
	groups := slots.Groups.Resolve(state, tokens)
	groupLabels := slots.GroupLabels.Resolve(state, tokens)
	actionItems := slots.ActionItems.Resolve(state, tokens)
	overflowControls := slots.OverflowControls.Resolve(state, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)
	_ = groupLabels

	cmds := make([]gfx.Command, 0, 64)
	if !theme.IsTransparentMaterial(root) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(bounds), root)...)
	}
	if !theme.IsTransparentMaterial(surface) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(bounds, r.cachedRadius), surface)...)
	}
	if !theme.IsTransparentMaterial(groups) && !r.cachedTabStripBounds.IsEmpty() {
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(r.cachedTabStripBounds, r.cachedRadius), groups)...)
	}
	if !theme.IsTransparentMaterial(actionItems) && !r.cachedBodyBounds.IsEmpty() {
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(r.cachedBodyBounds, r.cachedRadius), actionItems)...)
	}
	for i, btn := range r.cachedTabButtons {
		if btn == nil || btn.Base() == nil || btn.Base().ProjectionRole() == nil {
			continue
		}
		rect := r.cachedTabBounds[i]
		if rect.IsEmpty() {
			continue
		}
		if child := btn.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: rect, ContentScale: contentScale}); child != nil {
			cmds = append(cmds, child.Commands...)
		}
		_ = groupLabels
		_ = actionItems
	}
	for i, toolbar := range r.cachedSectionToolbars {
		if toolbar == nil || toolbar.Base() == nil || toolbar.Base().ProjectionRole() == nil {
			continue
		}
		rect := r.cachedToolbarBounds[i]
		if rect.IsEmpty() {
			continue
		}
		if child := toolbar.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: rect, ContentScale: contentScale}); child != nil {
			cmds = append(cmds, child.Commands...)
		}
	}
	if r.focusedVisible && !bounds.IsEmpty() && !theme.IsTransparentMaterial(focus) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(bounds, r.cachedRadius), focus)...)
	}
	if !theme.IsTransparentMaterial(overflowControls) && !r.cachedTabStripBounds.IsEmpty() {
		separator := gfx.RectFromXYWH(r.cachedTabStripBounds.Min.X, r.cachedTabStripBounds.Max.Y-1, r.cachedTabStripBounds.Width(), 1)
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(separator), overflowControls)...)
	}
	return cmds
}

func (r *Ribbon) hitTest(p gfx.Point) facet.HitResult {
	if r == nil || r.cachedRootBounds.IsEmpty() || !r.cachedRootBounds.Contains(p) {
		return facet.HitResult{}
	}
	for i, btn := range r.cachedTabButtons {
		if btn == nil || btn.Base() == nil || btn.Base().HitRole() == nil {
			continue
		}
		rect := r.cachedTabBounds[i]
		if rect.IsEmpty() || !rect.Contains(p) {
			continue
		}
		if hit := btn.Base().HitRole().HitTest(p); hit.Hit {
			return facet.HitResult{Hit: true, MarkID: ribbonMarkIDGroupLabels}
		}
	}
	for i, toolbar := range r.cachedSectionToolbars {
		if toolbar == nil || toolbar.Base() == nil || toolbar.Base().HitRole() == nil {
			continue
		}
		rect := r.cachedToolbarBounds[i]
		if rect.IsEmpty() || !rect.Contains(p) {
			continue
		}
		if hit := toolbar.Base().HitRole().HitTest(p); hit.Hit {
			if hit.MarkID == toolbarMarkIDOverflowMenu {
				return facet.HitResult{Hit: true, MarkID: ribbonMarkIDOverflowControls}
			}
			return facet.HitResult{Hit: true, MarkID: ribbonMarkIDActionItems}
		}
	}
	return facet.HitResult{Hit: true, MarkID: ribbonMarkIDRibbonSurface}
}

func (r *Ribbon) onPointer(e facet.PointerEvent) bool {
	if r == nil || r.Disabled.Get() {
		return false
	}
	for i, btn := range r.cachedTabButtons {
		if btn == nil || btn.Base() == nil || btn.Base().InputRole() == nil {
			continue
		}
		if i >= len(r.cachedTabBounds) || r.cachedTabBounds[i].IsEmpty() || !r.cachedTabBounds[i].Contains(e.Position) {
			continue
		}
		if btn.Base().InputRole().OnPointer != nil && btn.Base().InputRole().OnPointer(e) {
			return true
		}
		return true
	}
	for i, toolbar := range r.cachedSectionToolbars {
		if toolbar == nil || toolbar.Base() == nil || toolbar.Base().InputRole() == nil {
			continue
		}
		if i >= len(r.cachedToolbarBounds) || r.cachedToolbarBounds[i].IsEmpty() || !r.cachedToolbarBounds[i].Contains(e.Position) {
			continue
		}
		if toolbar.Base().InputRole().OnPointer != nil && toolbar.Base().InputRole().OnPointer(e) {
			return true
		}
		return true
	}
	return false
}

func (r *Ribbon) onKey(e facet.KeyEvent) bool {
	if r == nil || r.Disabled.Get() || len(r.cachedTabButtons) == 0 {
		return false
	}
	if e.Kind != platform.KeyPress && e.Kind != platform.KeyRelease {
		return false
	}
	switch e.Key {
	case platform.KeyLeft:
		if e.Kind == platform.KeyPress {
			r.selectSection(r.focusedTabIndex-1, true)
		}
		return true
	case platform.KeyRight:
		if e.Kind == platform.KeyPress {
			r.selectSection(r.focusedTabIndex+1, true)
		}
		return true
	case platform.KeyHome:
		if e.Kind == platform.KeyPress {
			r.selectSection(0, true)
		}
		return true
	case platform.KeyEnd:
		if e.Kind == platform.KeyPress {
			r.selectSection(len(r.cachedTabButtons)-1, true)
		}
		return true
	case platform.KeyEnter, platform.KeySpace:
		if btn := r.focusedButton(); btn != nil && btn.Base() != nil && btn.Base().InputRole() != nil {
			return btn.Base().InputRole().OnKey != nil && btn.Base().InputRole().OnKey(e)
		}
		return true
	default:
		return false
	}
}

func (r *Ribbon) onFocusGained() {
	r.focusedVisible = true
	r.focusedTabIndex = r.clampSectionIndex(r.ActiveIndex)
	if btn := r.focusedButton(); btn != nil {
		btn.onFocusGained()
	}
	r.invalidate(facet.DirtyProjection)
}

func (r *Ribbon) onFocusLost() {
	r.focusedVisible = false
	if btn := r.focusedButton(); btn != nil {
		btn.onFocusLost()
	}
	r.focusFromPointer = false
	r.invalidate(facet.DirtyProjection)
}

func (r *Ribbon) selectSection(index int, emit bool) {
	if r == nil || len(r.Sections) == 0 {
		return
	}
	index = r.clampSectionIndex(index)
	if index < 0 {
		return
	}
	previous := r.ActiveIndex
	r.ActiveIndex = index
	r.focusedTabIndex = index
	r.syncButtonVariants()
	r.syncToolbarChildren()
	if previous != index || emit {
		if previous >= 0 && previous < len(r.cachedTabButtons) {
			if btn := r.cachedTabButtons[previous]; btn != nil {
				btn.onFocusLost()
			}
		}
		if btn := r.focusedButton(); btn != nil {
			btn.onFocusGained()
		}
		r.Activated.Emit(index)
		r.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	}
}

func (r *Ribbon) activateSection(index int, emit bool) {
	if r == nil {
		return
	}
	r.selectSection(index, emit)
}

func (r *Ribbon) focusedButton() *Button {
	if r == nil || len(r.cachedTabButtons) == 0 {
		return nil
	}
	idx := r.clampSectionIndex(r.focusedTabIndex)
	if idx < 0 || idx >= len(r.cachedTabButtons) {
		return nil
	}
	return r.cachedTabButtons[idx]
}

func (r *Ribbon) activeSection() *RibbonSection {
	if r == nil || len(r.Sections) == 0 {
		return nil
	}
	idx := r.clampSectionIndex(r.ActiveIndex)
	if idx < 0 || idx >= len(r.Sections) {
		return nil
	}
	return &r.Sections[idx]
}

func (r *Ribbon) clampIndices() {
	r.ActiveIndex = r.clampSectionIndex(r.ActiveIndex)
	r.focusedTabIndex = r.clampSectionIndex(r.focusedTabIndex)
}

func (r *Ribbon) clampSectionIndex(index int) int {
	if len(r.Sections) == 0 {
		return -1
	}
	if index < 0 {
		return 0
	}
	if index >= len(r.Sections) {
		return len(r.Sections) - 1
	}
	return index
}

func normalizeRibbonSections(sections []RibbonSection) []RibbonSection {
	next := append([]RibbonSection(nil), sections...)
	for i := range next {
		next[i].Key = strings.TrimSpace(next[i].Key)
		next[i].Label = strings.TrimSpace(next[i].Label)
		next[i].IconRef = strings.TrimSpace(next[i].IconRef)
	}
	return next
}

type ribbonGroupPolicy struct {
	ribbon *Ribbon
}

func (ribbonGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (p ribbonGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.ribbon == nil {
		return facet.GroupMeasureResult{}, nil
	}
	size := p.ribbon.measure(ctx.MeasureContext, facet.Constraints{
		MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()},
	}).Size
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p ribbonGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.ribbon == nil {
		return nil, nil
	}
	p.ribbon.arrange(ctx.ArrangeContext, ctx.Bounds)
	return nil, nil
}
