package navigation

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	gfxsvg "codeburg.org/lexbit/lurpicui/gfx/svg"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uinav"
)

const (
	tabsMarkIDRoot            facet.MarkID = 1
	tabsMarkIDTabList         facet.MarkID = 2
	tabsMarkIDTab             facet.MarkID = 3
	tabsMarkIDTabLabel        facet.MarkID = 4
	tabsMarkIDActiveIndicator facet.MarkID = 5
	tabsMarkIDPanelAnchor     facet.MarkID = 6
	tabsMarkIDFocusRing       facet.MarkID = 7
)

// TabItem describes one tab trigger and its associated panel content.
type TabItem struct {
	Key       string
	Label     string
	PanelText string
	IconRef   string
	Disabled  bool
}

// Tabs implements the navigation.tabs standard mark.
type Tabs struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	focusRole      facet.FocusRole
	textRole       facet.TextRole

	Activated signal.Signal[int]

	Label       string
	Items       []TabItem
	Variant     uinav.TabsVariant
	Disabled    bool
	ActiveIndex int

	hoveredIndex     int
	pressedIndex     int
	focusedVisible   bool
	focusFromPointer bool

	cachedTokens           theme.Tokens
	cachedRecipe           shared.TabsSlots
	cachedRootBounds       gfx.Rect
	cachedTabListBounds    gfx.Rect
	cachedPanelBounds      gfx.Rect
	cachedTabBounds        []gfx.Rect
	cachedTabLabelBounds   []gfx.Rect
	cachedTabLabelLayouts  []*text.TextLayout
	cachedPanelLayout      *text.TextLayout
	cachedTabLabelStyle    text.TextStyle
	cachedPanelStyle       text.TextStyle
	cachedTabGap           float32
	cachedTabPadX          float32
	cachedTabPadY          float32
	cachedPanelGap         float32
	cachedPanelPadX        float32
	cachedPanelPadY        float32
	cachedIndicatorH       float32
	cachedWritingDirection facet.WritingDirection
	cachedIconAssets       []runtimepkg.IconAsset
	cachedIconBounds       []gfx.Rect
}

var _ facet.FacetImpl = (*Tabs)(nil)
var _ layout.AnchorExporter = (*Tabs)(nil)

// NewTabs constructs a navigation.tabs mark with canonical defaults.
func NewTabs(label string, items []TabItem) *Tabs {
	t := &Tabs{
		Facet:   facet.NewFacet(),
		Label:   label,
		Variant: uinav.TabsStandard,
	}
	t.SetItems(items)
	t.layoutRole.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearHorizontal,
		Policy: tabsGroupPolicy{},
	}
	t.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
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
			Width:  facet.StretchNever,
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
	t.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult { return t.hitTest(p) }
	t.inputRole.OnPointer = func(e facet.PointerEvent) bool { return t.onPointer(e) }
	t.inputRole.OnKey = func(e facet.KeyEvent) bool { return t.onKey(e) }
	t.focusRole.Focusable = func() bool { return !t.Disabled && len(t.Items) > 0 }
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
	return t
}

// Base satisfies facet.FacetImpl.
func (t *Tabs) Base() *facet.Facet {
	t.Facet.BindImpl(t)
	return &t.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (t *Tabs) AccessibilityRole() string { return "tablist" }

// AccessibleName reports the semantic name source required by the spec.
func (t *Tabs) AccessibleName() string { return t.Label }

// SetLabel updates the authored accessible label.
func (t *Tabs) SetLabel(label string) {
	if t == nil || t.Label == label {
		return
	}
	t.Label = label
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetItems updates the tab item list.
func (t *Tabs) SetItems(items []TabItem) {
	if t == nil {
		return
	}
	next := append([]TabItem(nil), items...)
	for i := range next {
		next[i].Key = strings.TrimSpace(next[i].Key)
		next[i].Label = strings.TrimSpace(next[i].Label)
		next[i].PanelText = strings.TrimSpace(next[i].PanelText)
		next[i].IconRef = strings.TrimSpace(next[i].IconRef)
	}
	t.Items = next
	t.clampActiveIndex()
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetActiveIndex updates the authored active tab.
func (t *Tabs) SetActiveIndex(index int) {
	if t == nil {
		return
	}
	if index < 0 {
		index = 0
	}
	if len(t.Items) > 0 && index >= len(t.Items) {
		index = len(t.Items) - 1
	}
	if len(t.Items) == 0 {
		index = 0
	}
	if t.ActiveIndex == index {
		return
	}
	t.ActiveIndex = index
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetVariant updates the authored tabs variant.
func (t *Tabs) SetVariant(variant uinav.TabsVariant) {
	if t == nil || t.Variant == variant {
		return
	}
	t.Variant = variant
	t.invalidate(facet.DirtyProjection)
}

// SetDisabled toggles disabled state.
func (t *Tabs) SetDisabled(disabled bool) {
	if t == nil || t.Disabled == disabled {
		return
	}
	t.Disabled = disabled
	if disabled {
		t.hoveredIndex = -1
		t.pressedIndex = -1
		t.focusedVisible = false
		t.focusFromPointer = false
	}
	t.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// ExportAnchors publishes the tabs anchor set.
func (t *Tabs) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
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
	if t.cachedPanelLayout != nil {
		out["baseline"] = gfx.Point{
			X: t.cachedPanelBounds.Min.X,
			Y: t.cachedPanelBounds.Min.Y + t.cachedPanelLayout.Baseline,
		}
	} else if len(t.cachedTabLabelLayouts) > 0 && t.cachedTabLabelLayouts[0] != nil && len(t.cachedTabLabelBounds) > 0 {
		out["baseline"] = gfx.Point{
			X: t.cachedTabLabelBounds[0].Min.X,
			Y: t.cachedTabLabelBounds[0].Min.Y + t.cachedTabLabelLayouts[0].Baseline,
		}
	}
	return out
}

// Children returns the facet's immediate child list.
func (t *Tabs) Children() []facet.GroupChild { return nil }

// OnAttach is unused beyond layout role setup.
func (t *Tabs) OnAttach(ctx facet.AttachContext) {}

// OnActivate is unused.
func (t *Tabs) OnActivate() {}

// OnDeactivate is unused.
func (t *Tabs) OnDeactivate() {}

// OnDetach clears cached projection state.
func (t *Tabs) OnDetach() {
	t.cachedTokens = theme.Tokens{}
	t.cachedRecipe = shared.TabsSlots{}
	t.cachedRootBounds = gfx.Rect{}
	t.cachedTabListBounds = gfx.Rect{}
	t.cachedPanelBounds = gfx.Rect{}
	t.cachedTabBounds = nil
	t.cachedTabLabelBounds = nil
	t.cachedTabLabelLayouts = nil
	t.cachedPanelLayout = nil
	t.cachedTabLabelStyle = text.TextStyle{}
	t.cachedPanelStyle = text.TextStyle{}
	t.cachedTabGap = 0
	t.cachedTabPadX = 0
	t.cachedTabPadY = 0
	t.cachedPanelGap = 0
	t.cachedPanelPadX = 0
	t.cachedPanelPadY = 0
	t.cachedIndicatorH = 0
	t.cachedIconAssets = nil
	t.cachedIconBounds = nil
}

func (t *Tabs) invalidate(flags facet.DirtyFlags) {
	if t == nil {
		return
	}
	t.Base().Invalidate(flags)
}

func (t *Tabs) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uinav.ResolveTabsRecipe(style, t.Variant)
	t.cachedTokens = resolved.TokenSet()
	t.cachedRecipe = slots
	t.cachedWritingDirection = ctx.WritingDirection
	t.cachedTabGap = float32(resolved.Spacing(theme.SpacingM))
	if t.Variant == uinav.TabsCompact {
		t.cachedTabGap = float32(resolved.Spacing(theme.SpacingS))
	}
	t.cachedTabPadX = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	t.cachedTabPadY = maxFloat(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	t.cachedPanelGap = float32(resolved.Spacing(theme.SpacingM))
	t.cachedPanelPadX = maxFloat(float32(resolved.Spacing(theme.SpacingL)), resolved.Density.Scale(16))
	t.cachedPanelPadY = maxFloat(float32(resolved.Spacing(theme.SpacingL)), resolved.Density.Scale(16))
	t.cachedIndicatorH = maxFloat(2, resolved.TokenSet().Spacing.BorderWeight*2)
	if t.cachedIndicatorH <= 0 {
		t.cachedIndicatorH = 2
	}
	t.cachedTabLabelStyle = resolved.TextStyle(theme.TextLabelM)
	if t.Variant == uinav.TabsCompact {
		t.cachedTabLabelStyle = resolved.TextStyle(theme.TextLabelS)
	}
	t.cachedPanelStyle = resolved.TextStyle(theme.TextHeadingS)
	shaper := t.newShaper(ctx.Runtime)
	if shaper == nil {
		t.cachedTabLabelLayouts = nil
		t.cachedPanelLayout = nil
		return facet.MeasureResult{}
	}
	shaper.SetContentScale(ctx.ContentScale)
	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = resolved.Density.Scale(960)
	}
	labelLayouts := make([]*text.TextLayout, len(t.Items))
	iconAssets := make([]runtimepkg.IconAsset, len(t.Items))
	iconBounds := make([]gfx.Rect, len(t.Items))
	tabWidths := make([]float32, len(t.Items))
	tabHeights := make([]float32, len(t.Items))
	for i := range t.Items {
		item := t.Items[i]
		labelLayouts[i] = shaper.ShapeTruncated(item.Label, t.cachedTabLabelStyle, maxWidth)
		if item.IconRef != "" {
			if asset, ok := t.resolveIcon(ctx.Runtime, item.IconRef); ok {
				iconAssets[i] = asset
			}
		}
		labelW := text.Width(labelLayouts[i])
		labelH := text.Height(labelLayouts[i])
		iconW := float32(0)
		iconH := float32(0)
		if item.IconRef != "" {
			iconW = resolved.TokenSet().Spacing.IconSize
			iconH = resolved.TokenSet().Spacing.IconSize
			if iconW <= 0 {
				iconW = 20
			}
			if iconH <= 0 {
				iconH = 20
			}
		}
		if iconW > 0 && labelW > 0 {
			labelW += t.cachedTabGap
		}
		tabWidths[i] = t.cachedTabPadX*2 + labelW + iconW
		if labelH > iconH {
			tabHeights[i] = labelH
		} else {
			tabHeights[i] = iconH
		}
		tabHeights[i] += t.cachedTabPadY * 2
		if iconW > 0 {
			iconBounds[i] = gfx.RectFromXYWH(0, 0, iconW, iconH)
		}
	}
	stripW := float32(0)
	stripH := float32(0)
	for i := range tabWidths {
		stripW += tabWidths[i]
		if i > 0 {
			stripW += t.cachedTabGap
		}
		if tabHeights[i] > stripH {
			stripH = tabHeights[i]
		}
	}
	if stripH <= 0 {
		stripH = resolved.Density.Scale(36)
	}
	panelText := t.activePanelText()
	panelLayout := shaper.ShapeTruncated(panelText, t.cachedPanelStyle, maxWidth)
	panelTextW := text.Width(panelLayout)
	panelTextH := text.Height(panelLayout)
	panelH := panelTextH + t.cachedPanelPadY*2
	if panelTextH <= 0 {
		panelH = maxFloat(resolved.Density.Scale(84), stripH)
	}
	panelW := maxFloat(stripW, panelTextW+t.cachedPanelPadX*2)
	if panelW <= 0 {
		panelW = stripW
	}
	width := maxFloat(stripW, panelW)
	height := stripH
	if panelH > 0 {
		height += t.cachedPanelGap + panelH
	}
	measured := constraints.Constrain(gfx.Size{W: width, H: height})
	t.cachedTabLabelLayouts = labelLayouts
	t.cachedPanelLayout = panelLayout
	t.cachedIconAssets = iconAssets
	t.cachedIconBounds = iconBounds
	t.cachedTabBounds = make([]gfx.Rect, len(t.Items))
	t.cachedTabLabelBounds = make([]gfx.Rect, len(t.Items))
	t.layoutRole.MeasuredSize = measured
	t.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: measured,
		Intrinsic: facet.IntrinsicSize{
			Min:       measured,
			Preferred: measured,
			Max:       measured,
		},
		Constraints: constraints,
	}
	return t.layoutRole.MeasuredResult
}

func (t *Tabs) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return t.measure(ctx, constraints).Size
}

func (t *Tabs) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	t.cachedRootBounds = bounds
	t.cachedTabListBounds = gfx.Rect{}
	t.cachedPanelBounds = gfx.Rect{}
	t.layoutRole.ArrangedBounds = bounds
	if bounds.IsEmpty() || len(t.Items) == 0 {
		return
	}
	tabY := bounds.Min.Y
	stripH := float32(0)
	for i := range t.cachedTabLabelLayouts {
		h := text.Height(t.cachedTabLabelLayouts[i])
		if iconH := t.cachedIconBounds[i].Height(); iconH > h {
			h = iconH
		}
		if h <= 0 {
			h = 20
		}
		h += t.cachedTabPadY * 2
		if h > stripH {
			stripH = h
		}
	}
	if stripH <= 0 {
		stripH = maxFloat(bounds.Height()*0.18, 32)
	}
	t.cachedTabListBounds = gfx.RectFromXYWH(bounds.Min.X, tabY, bounds.Width(), stripH)
	panelY := tabY + stripH + t.cachedPanelGap
	panelH := maxFloat(bounds.Height()-stripH-t.cachedPanelGap, 0)
	if panelH <= 0 {
		panelH = bounds.Height() * 0.6
	}
	t.cachedPanelBounds = gfx.RectFromXYWH(bounds.Min.X, panelY, bounds.Width(), panelH)
	curX := bounds.Min.X
	if t.cachedWritingDirection == facet.WritingDirectionRTL {
		curX = bounds.Max.X
	}
	for i := range t.Items {
		w := t.tabWidth(i)
		h := stripH
		if t.cachedWritingDirection == facet.WritingDirectionRTL {
			curX -= w
			t.cachedTabBounds[i] = gfx.RectFromXYWH(curX, tabY, w, h)
			curX -= t.cachedTabGap
		} else {
			t.cachedTabBounds[i] = gfx.RectFromXYWH(curX, tabY, w, h)
			curX += w + t.cachedTabGap
		}
		labelLayout := t.cachedTabLabelLayouts[i]
		labelW := text.Width(labelLayout)
		labelH := text.Height(labelLayout)
		iconRect := t.cachedIconBounds[i]
		contentW := labelW
		if !iconRect.IsEmpty() && labelW > 0 {
			contentW += t.cachedTabGap + iconRect.Width()
		} else if !iconRect.IsEmpty() {
			contentW = iconRect.Width()
		}
		contentH := labelH
		if iconRect.Height() > contentH {
			contentH = iconRect.Height()
		}
		contentX := t.cachedTabBounds[i].Min.X + maxFloat(0, (t.cachedTabBounds[i].Width()-contentW)*0.5)
		contentRect := text.CenterRect(t.cachedTabBounds[i], contentW, contentH)
		if t.cachedWritingDirection == facet.WritingDirectionRTL {
			contentX = t.cachedTabBounds[i].Max.X - maxFloat(0, (t.cachedTabBounds[i].Width()-contentW)*0.5) - contentW
		}
		if !iconRect.IsEmpty() {
			if t.cachedWritingDirection == facet.WritingDirectionRTL {
				iconRect = text.AlignRectY(gfx.RectFromXYWH(contentX+labelW+t.cachedTabGap, contentRect.Min.Y, iconRect.Width(), iconRect.Height()), contentRect.Min.Y, contentRect.Height())
				if labelLayout != nil {
					t.cachedTabLabelBounds[i] = text.AlignRectY(gfx.RectFromXYWH(contentX, contentRect.Min.Y, labelW, labelH), contentRect.Min.Y, contentRect.Height())
				}
			} else {
				iconRect = text.AlignRectY(gfx.RectFromXYWH(contentX, contentRect.Min.Y, iconRect.Width(), iconRect.Height()), contentRect.Min.Y, contentRect.Height())
				if labelLayout != nil {
					t.cachedTabLabelBounds[i] = text.AlignRectY(gfx.RectFromXYWH(iconRect.Max.X+t.cachedTabGap, contentRect.Min.Y, labelW, labelH), contentRect.Min.Y, contentRect.Height())
				}
			}
			t.cachedIconBounds[i] = iconRect
		} else if labelLayout != nil {
			t.cachedTabLabelBounds[i] = text.AlignRectY(gfx.RectFromXYWH(contentX, contentRect.Min.Y, labelW, labelH), contentRect.Min.Y, contentRect.Height())
		}
	}
	if len(t.cachedTabLabelLayouts) > 0 {
		idx := t.clampedActiveIndex()
		if idx >= 0 && idx < len(t.cachedTabLabelLayouts) {
			t.textRole.Layout = t.cachedTabLabelLayouts[idx]
		}
	}
	t.textRole.Selection = text.TextRange{}
	t.textRole.CaretVisible = false
	t.textRole.CaretPosition = text.TextPosition{}
}

func (t *Tabs) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.TabsSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: t.cachedTokens}, t.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, t.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uinav.ResolveTabsRecipe(style, t.Variant)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: t.cachedTokens}, t.cachedRecipe
}

func (t *Tabs) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if t == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := t.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	root := slots.Root.Resolve(t.interactionState(), tokens)
	tabList := slots.TabList.Resolve(theme.StateDefault, tokens)
	tabLabel := slots.TabLabel.Resolve(t.labelState(), tokens)
	activeIndicator := slots.ActiveIndicator.Resolve(theme.StateSelected, tokens)
	panelAnchor := slots.PanelAnchor.Resolve(theme.StateDefault, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)

	cmds := make([]gfx.Command, 0, 64)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(tabList) && !t.cachedTabListBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.RectPath(t.cachedTabListBounds), tabList)...)
	}
	if !isTransparentMaterial(panelAnchor) && !t.cachedPanelBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(t.cachedPanelBounds, float32(tokens.Radius.SM)), panelAnchor)...)
	}
	for i := range t.Items {
		rect := t.cachedTabBounds[i]
		if rect.IsEmpty() {
			continue
		}
		state := t.itemState(i)
		background := slots.Tab.Resolve(state, tokens)
		if !isTransparentMaterial(background) {
			cmds = append(cmds, materialCommands(gfx.RoundedRectPath(rect, float32(tokens.Radius.SM)), background)...)
		}
		if i == t.clampedActiveIndex() && !isTransparentMaterial(activeIndicator) {
			indicator := gfx.RectFromXYWH(rect.Min.X, rect.Max.Y-t.cachedIndicatorH, rect.Width(), t.cachedIndicatorH)
			cmds = append(cmds, gfx.FillRect{
				Rect:  indicator,
				Brush: gfx.SolidBrush(materialColor(activeIndicator)),
			})
		}
		if label := t.cachedTabLabelLayouts[i]; label != nil && !isTransparentMaterial(tabLabel) {
			cmds = append(cmds, t.textCommands(label, t.cachedTabLabelBounds[i], tabLabel)...)
		}
		if asset := t.cachedIconAssets[i]; len(asset.Path.Segments) > 0 && !t.cachedIconBounds[i].IsEmpty() {
			iconColor := tabLabel
			cmds = append(cmds, t.iconCommands(asset, t.cachedIconBounds[i], iconColor)...)
		}
	}
	if t.cachedPanelLayout != nil && !isTransparentMaterial(tabLabel) {
		panelTextBounds := t.cachedPanelTextBounds()
		cmds = append(cmds, t.textCommands(t.cachedPanelLayout, panelTextBounds, tabLabel)...)
	}
	if t.focusedVisible && !isTransparentMaterial(focus) {
		if len(t.cachedTabBounds) > 0 {
			active := t.cachedTabBounds[t.clampedActiveIndex()]
			if !active.IsEmpty() {
				inset := maxFloat(1, active.Height()*0.10)
				cmds = append(cmds, materialCommands(gfx.RoundedRectPath(active.Inset(-inset, -inset), float32(tokens.Radius.SM)+inset), focus)...)
			}
		}
	}
	return cmds
}

func (t *Tabs) hitTest(p gfx.Point) facet.HitResult {
	if t == nil || t.layoutRole.ArrangedBounds.IsEmpty() || !t.layoutRole.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := t.cursorShape()
	if t.focusedVisible && t.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: tabsMarkIDFocusRing, Cursor: cursor}
	}
	for i := range t.cachedTabBounds {
		if !t.cachedTabBounds[i].Contains(p) {
			continue
		}
		if t.clampedActiveIndex() == i && t.pointInIndicator(i, p) {
			return facet.HitResult{Hit: true, MarkID: tabsMarkIDActiveIndicator, Cursor: cursor}
		}
		if len(t.cachedTabLabelBounds) > i && t.cachedTabLabelBounds[i].Contains(p) {
			return facet.HitResult{Hit: true, MarkID: tabsMarkIDTabLabel, Cursor: cursor}
		}
		return facet.HitResult{Hit: true, MarkID: tabsMarkIDTab, Cursor: cursor}
	}
	if t.cachedPanelBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: tabsMarkIDPanelAnchor, Cursor: cursor}
	}
	if t.cachedTabListBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: tabsMarkIDTabList, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: tabsMarkIDRoot, Cursor: cursor}
}

func (t *Tabs) onPointer(e facet.PointerEvent) bool {
	if t.Disabled {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		t.hoveredIndex = t.indexAt(e.Position)
		t.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		t.hoveredIndex = -1
		if t.pressedIndex < 0 {
			t.focusFromPointer = false
		}
		t.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		idx := t.indexAt(e.Position)
		if idx < 0 || t.isDisabledIndex(idx) {
			return false
		}
		t.hoveredIndex = idx
		t.pressedIndex = idx
		t.focusFromPointer = true
		t.focusedVisible = false
		t.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := t.pressedIndex >= 0
		idx := t.pressedIndex
		t.pressedIndex = -1
		t.invalidate(facet.DirtyProjection)
		if wasPressed {
			if hit := t.indexAt(e.Position); hit >= 0 && hit == idx && !t.isDisabledIndex(hit) {
				t.activateIndex(hit)
				return true
			}
			return true
		}
		return false
	case platform.PointerMove:
		t.hoveredIndex = t.indexAt(e.Position)
		t.invalidate(facet.DirtyProjection)
		return true
	default:
		return false
	}
}

func (t *Tabs) onKey(e facet.KeyEvent) bool {
	if t.Disabled || len(t.Items) == 0 {
		return false
	}
	switch e.Key {
	case platform.KeyLeft, platform.KeyRight, platform.KeyHome, platform.KeyEnd, platform.KeySpace, platform.KeyEnter:
		switch e.Kind {
		case platform.KeyPress, platform.KeyRepeat:
			switch e.Key {
			case platform.KeyLeft:
				t.moveActive(-1)
				return true
			case platform.KeyRight:
				t.moveActive(1)
				return true
			case platform.KeyHome:
				t.setFirstEnabled()
				return true
			case platform.KeyEnd:
				t.setLastEnabled()
				return true
			case platform.KeySpace, platform.KeyEnter:
				t.pressedIndex = t.clampedActiveIndex()
				t.invalidate(facet.DirtyProjection)
				return true
			}
		case platform.KeyRelease:
			if e.Key == platform.KeySpace || e.Key == platform.KeyEnter {
				wasPressed := t.pressedIndex >= 0
				t.pressedIndex = -1
				t.invalidate(facet.DirtyProjection)
				if wasPressed {
					t.Activated.Emit(t.clampedActiveIndex())
					return true
				}
			}
		}
	}
	return false
}

func (t *Tabs) onFocusGained() {
	t.focusedVisible = !t.focusFromPointer
	t.focusFromPointer = false
	t.invalidate(facet.DirtyProjection)
}

func (t *Tabs) onFocusLost() {
	t.focusedVisible = false
	t.pressedIndex = -1
	t.focusFromPointer = false
	t.invalidate(facet.DirtyProjection)
}

func (t *Tabs) interactionState() theme.InteractionState {
	switch {
	case t.Disabled:
		return theme.StateDisabled
	case t.pressedIndex >= 0:
		return theme.StatePressed
	case t.hoveredIndex >= 0:
		return theme.StateHover
	case t.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (t *Tabs) itemState(index int) theme.InteractionState {
	if t.Disabled || t.isDisabledIndex(index) {
		return theme.StateDisabled
	}
	if index == t.clampedActiveIndex() {
		if t.pressedIndex == index {
			return theme.StatePressed
		}
		if t.hoveredIndex == index {
			return theme.StateHover
		}
		return theme.StateSelected
	}
	if t.pressedIndex == index {
		return theme.StatePressed
	}
	if t.hoveredIndex == index {
		return theme.StateHover
	}
	return theme.StateDefault
}

func (t *Tabs) labelState() theme.InteractionState {
	if t.Disabled {
		return theme.StateDisabled
	}
	return theme.StateDefault
}

func (t *Tabs) activePanelText() string {
	if len(t.Items) == 0 {
		return ""
	}
	idx := t.clampedActiveIndex()
	if idx >= 0 && idx < len(t.Items) {
		if t.Items[idx].PanelText != "" {
			return t.Items[idx].PanelText
		}
		return t.Items[idx].Label
	}
	return ""
}

func (t *Tabs) panelTextBounds() gfx.Rect {
	if t.cachedPanelLayout == nil || t.cachedPanelBounds.IsEmpty() {
		return gfx.Rect{}
	}
	w := t.cachedPanelLayout.Bounds.Width()
	h := t.cachedPanelLayout.Bounds.Height()
	x := t.cachedPanelBounds.Min.X + t.cachedPanelPadX + maxFloat(0, (t.cachedPanelBounds.Width()-t.cachedPanelPadX*2-w)*0.5)
	y := t.cachedPanelBounds.Min.Y + t.cachedPanelPadY + maxFloat(0, (t.cachedPanelBounds.Height()-t.cachedPanelPadY*2-h)*0.5)
	return gfx.RectFromXYWH(x, y, w, h)
}

func (t *Tabs) cachedPanelTextBounds() gfx.Rect {
	return t.panelTextBounds()
}

func (t *Tabs) pointInFocusRing(p gfx.Point) bool {
	if !t.focusedVisible {
		return false
	}
	if len(t.cachedTabBounds) == 0 {
		return false
	}
	active := t.cachedTabBounds[t.clampedActiveIndex()]
	if active.IsEmpty() || !active.Contains(p) {
		return false
	}
	ring := maxFloat(1, active.Height()*0.12)
	inner := active.Inset(ring, ring)
	if inner.IsEmpty() {
		return true
	}
	return !inner.Contains(p)
}

func (t *Tabs) pointInIndicator(index int, p gfx.Point) bool {
	if index < 0 || index >= len(t.cachedTabBounds) {
		return false
	}
	rect := t.cachedTabBounds[index]
	if rect.IsEmpty() || !rect.Contains(p) {
		return false
	}
	indicator := gfx.RectFromXYWH(rect.Min.X, rect.Max.Y-t.cachedIndicatorH, rect.Width(), t.cachedIndicatorH)
	return indicator.Contains(p)
}

func (t *Tabs) activateIndex(index int) {
	if index < 0 || index >= len(t.Items) || t.isDisabledIndex(index) {
		return
	}
	if t.ActiveIndex == index {
		t.Activated.Emit(index)
		return
	}
	t.ActiveIndex = index
	t.Activated.Emit(index)
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func (t *Tabs) moveActive(delta int) {
	if len(t.Items) == 0 {
		return
	}
	start := t.clampedActiveIndex()
	for step := 1; step <= len(t.Items); step++ {
		next := start + delta*step
		for next < 0 {
			next += len(t.Items)
		}
		next %= len(t.Items)
		if !t.isDisabledIndex(next) {
			t.activateIndex(next)
			return
		}
	}
}

func (t *Tabs) setFirstEnabled() {
	for i := range t.Items {
		if !t.isDisabledIndex(i) {
			t.activateIndex(i)
			return
		}
	}
}

func (t *Tabs) setLastEnabled() {
	for i := len(t.Items) - 1; i >= 0; i-- {
		if !t.isDisabledIndex(i) {
			t.activateIndex(i)
			return
		}
	}
}

func (t *Tabs) clampedActiveIndex() int {
	if len(t.Items) == 0 {
		return 0
	}
	if t.ActiveIndex < 0 {
		return 0
	}
	if t.ActiveIndex >= len(t.Items) {
		return len(t.Items) - 1
	}
	return t.ActiveIndex
}

func (t *Tabs) clampActiveIndex() {
	if len(t.Items) == 0 {
		t.ActiveIndex = 0
		return
	}
	if t.ActiveIndex < 0 || t.ActiveIndex >= len(t.Items) {
		t.ActiveIndex = 0
	}
	if t.isDisabledIndex(t.ActiveIndex) {
		for i := range t.Items {
			if !t.isDisabledIndex(i) {
				t.ActiveIndex = i
				return
			}
		}
	}
}

func (t *Tabs) isDisabledIndex(index int) bool {
	if index < 0 || index >= len(t.Items) {
		return true
	}
	return t.Disabled || t.Items[index].Disabled
}

func (t *Tabs) indexAt(p gfx.Point) int {
	for i := range t.cachedTabBounds {
		if t.cachedTabBounds[i].Contains(p) {
			return i
		}
	}
	return -1
}

func (t *Tabs) cursorShape() facet.CursorShape {
	if t.Disabled {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (t *Tabs) tabWidth(index int) float32 {
	if index < 0 || index >= len(t.cachedTabBounds) {
		if len(t.cachedTabLabelLayouts) > index && t.cachedTabLabelLayouts[index] != nil {
			return t.cachedTabLabelLayouts[index].Bounds.Width() + t.cachedTabPadX*2
		}
		return t.cachedTabPadX*2 + 48
	}
	if w := t.cachedTabBounds[index].Width(); w > 0 {
		return w
	}
	return t.cachedTabPadX*2 + 48
}

func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func (t *Tabs) pointInTabLabel(index int, p gfx.Point) bool {
	if index < 0 || index >= len(t.cachedTabLabelBounds) {
		return false
	}
	return t.cachedTabLabelBounds[index].Contains(p)
}

func (t *Tabs) newShaper(runtime any) *text.Shaper {
	registry := t.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (t *Tabs) fontRegistry(runtime any) *text.FontRegistry {
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

func (t *Tabs) textCommands(layout *text.TextLayout, bounds gfx.Rect, material theme.Material) []gfx.Command {
	return primitive.TextLayoutCommands(layout, bounds, gfx.SolidBrush(materialColor(material)))
}

func (t *Tabs) iconCommands(asset runtimepkg.IconAsset, bounds gfx.Rect, material theme.Material) []gfx.Command {
	if len(asset.Path.Segments) == 0 || bounds.IsEmpty() || isTransparentMaterial(material) {
		return nil
	}
	box := asset.ViewBox
	if box.IsEmpty() {
		box = gfxsvg.Bounds(asset.Path)
	}
	if box.IsEmpty() || box.Width() == 0 || box.Height() == 0 {
		return nil
	}
	sx := bounds.Width() / box.Width()
	sy := bounds.Height() / box.Height()
	scale := minFloat(sx, sy)
	if scale <= 0 {
		return nil
	}
	target := gfxsvg.Transformed(asset.Path, gfx.Identity().Multiply(gfx.Translation(bounds.Min.X-box.Min.X*scale, bounds.Min.Y-box.Min.Y*scale)).Multiply(gfx.Scale(scale, scale)))
	return []gfx.Command{gfx.FillPath{Path: target, Brush: gfx.SolidBrush(materialColor(material))}}
}

func (t *Tabs) resolveIcon(runtime any, ref string) (runtimepkg.IconAsset, bool) {
	type iconProvider interface {
		IconResolver() runtimepkg.IconResolver
	}
	if runtime == nil {
		return runtimepkg.IconAsset{}, false
	}
	if provider, ok := runtime.(iconProvider); ok {
		if resolver := provider.IconResolver(); resolver != nil {
			return resolver.ResolveIcon(ref)
		}
	}
	if resolver, ok := runtime.(interface {
		ResolveIcon(string) (runtimepkg.IconAsset, bool)
	}); ok {
		return resolver.ResolveIcon(ref)
	}
	return runtimepkg.IconAsset{}, false
}

func materialCommands(path gfx.Path, material theme.Material) []gfx.Command {
	if isTransparentMaterial(material) {
		return nil
	}
	cmds := make([]gfx.Command, 0, len(material.Fills)+len(material.Strokes))
	for _, fill := range material.Fills {
		if fill.Type != theme.FillSolid || fill.Opacity <= 0 {
			continue
		}
		cmds = append(cmds, gfx.FillPath{Path: path, Brush: gfx.SolidBrush(fill.Color.WithAlpha(fill.Opacity))})
	}
	for _, stroke := range material.Strokes {
		if stroke.Width <= 0 {
			continue
		}
		if stroke.Paint.Type != theme.FillSolid || stroke.Paint.Opacity <= 0 {
			continue
		}
		cmds = append(cmds, gfx.StrokePath{
			Path: path,
			Stroke: gfx.StrokeStyle{
				Width:      stroke.Width,
				Cap:        gfx.LineCapRound,
				Join:       gfx.LineJoinRound,
				MiterLimit: 10,
			},
			Brush: gfx.SolidBrush(stroke.Paint.Color.WithAlpha(stroke.Paint.Opacity)),
		})
	}
	return cmds
}

func materialColor(material theme.Material) gfx.Color {
	if len(material.Fills) > 0 && material.Fills[0].Type == theme.FillSolid {
		c := material.Fills[0].Color
		if material.Fills[0].Opacity > 0 && material.Fills[0].Opacity < 1 {
			c = c.WithAlpha(material.Fills[0].Opacity)
		}
		return c
	}
	if len(material.Strokes) > 0 && material.Strokes[0].Paint.Type == theme.FillSolid {
		c := material.Strokes[0].Paint.Color
		if material.Strokes[0].Paint.Opacity > 0 && material.Strokes[0].Paint.Opacity < 1 {
			c = c.WithAlpha(material.Strokes[0].Paint.Opacity)
		}
		return c
	}
	return gfx.Color{}
}

func isTransparentMaterial(material theme.Material) bool {
	if len(material.Fills) == 0 && len(material.Strokes) == 0 {
		return true
	}
	if len(material.Fills) > 0 {
		fill := material.Fills[0]
		if fill.Type == theme.FillNone || fill.Opacity <= 0 {
			return true
		}
	}
	if len(material.Strokes) == 0 {
		return false
	}
	stroke := material.Strokes[0]
	return stroke.Width <= 0 || stroke.Paint.Type == theme.FillNone || stroke.Paint.Opacity <= 0
}

func (t *Tabs) indexEnabledAt(index int) bool {
	return index >= 0 && index < len(t.Items) && !t.Items[index].Disabled && !t.Disabled
}

type tabsGroupPolicy struct{}

func (tabsGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }
func (tabsGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}
func (tabsGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}
