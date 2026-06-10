package navigation

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
	"codeburg.org/lexbit/lurpicui/theme/recipes/uinav"
)

const (
	breadcrumbsMarkIDRoot           facet.MarkID = 1
	breadcrumbsMarkIDSegmentList    facet.MarkID = 2
	breadcrumbsMarkIDSegmentLink    facet.MarkID = 3
	breadcrumbsMarkIDSeparator      facet.MarkID = 4
	breadcrumbsMarkIDCurrentSegment facet.MarkID = 5
	breadcrumbsMarkIDFocusRing      facet.MarkID = 6
)

// BreadcrumbItem describes one breadcrumb segment.
type BreadcrumbItem struct {
	Label    string
	Disabled bool
}

// Breadcrumbs implements the navigation.breadcrumbs standard mark.
type Breadcrumbs struct {
	marks.Core

	Label        marks.Binding[string]
	Items        []BreadcrumbItem
	CurrentIndex marks.Binding[int]
	Disabled     marks.Binding[bool]

	Activated signal.Signal[int]

	textRole facet.TextRole

	hoveredIndex     int
	pressedIndex     int
	focusedIndex     int
	focusedVisible   bool
	focusFromPointer bool

	cachedTokens            theme.Tokens
	cachedRecipe            shared.BreadcrumbSlots
	cachedRootBounds        gfx.Rect
	cachedSegmentListBounds gfx.Rect
	cachedItemBounds        []gfx.Rect
	cachedLabelBounds       []gfx.Rect
	cachedSeparatorBounds   []gfx.Rect
	cachedLabelLayouts      []*text.TextLayout
	cachedSeparatorLayout   *text.TextLayout
	cachedLinkStyle         text.TextStyle
	cachedCurrentStyle      text.TextStyle
	cachedSeparatorStyle    text.TextStyle
	cachedGap               float32
	cachedPadX              float32
	cachedPadY              float32
	cachedWritingDirection  facet.WritingDirection
}

var _ facet.FacetImpl = (*Breadcrumbs)(nil)
var _ layout.AnchorExporter = (*Breadcrumbs)(nil)
var _ marks.Mark = (*Breadcrumbs)(nil)

// NewBreadcrumbs constructs a navigation.breadcrumbs mark with canonical defaults.
func NewBreadcrumbs(label string, items []BreadcrumbItem) *Breadcrumbs {
	b := &Breadcrumbs{
		Label:        marks.Const(label),
		CurrentIndex: marks.Const(len(items) - 1),
		Disabled:     marks.Const(false),
		focusedIndex: len(items) - 1,
	}
	b.Facet = facet.NewFacet()
	b.AddBinding(b.Label)
	b.AddBinding(b.CurrentIndex)
	b.AddBinding(b.Disabled)
	b.SetItems(items)
	b.Layout.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearHorizontal,
		Policy: breadcrumbsGroupPolicy{},
	}
	b.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := b.measureIntrinsic(ctx, constraints)
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
	b.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return b.measure(ctx, constraints)
	}
	b.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		b.Layout.ArrangedBounds = bounds
		b.arrange(ctx, bounds)
	}
	b.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return b.hitTest(p) }
	b.Input.OnPointer = func(e facet.PointerEvent) bool { return b.onPointer(e) }
	b.Input.OnKey = func(e facet.KeyEvent) bool { return b.onKey(e) }
	b.Focus.Focusable = func() bool { return !b.Disabled.Get() && len(b.Items) > 0 }
	b.Focus.TabIndex = 0
	b.Focus.OnFocusGained = func() { b.onFocusGained() }
	b.Focus.OnFocusLost = func() { b.onFocusLost() }
	b.textRole.IMEEnabled = false
	b.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return b.buildCommands(b.Layout.ArrangedBounds, ctx.Runtime)
	}
	b.RegisterRoles()
	b.AddRole(&b.textRole)
	return b
}

// Base satisfies facet.FacetImpl.
func (b *Breadcrumbs) Base() *facet.Facet {
	b.BindImpl(b)
	return &b.Facet
}

// Descriptor satisfies marks.Mark.
func (b *Breadcrumbs) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "navigation", TypeName: "breadcrumbs"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (b *Breadcrumbs) AccessibilityRole() string { return "navigation" }

// AccessibleName reports the semantic name source required by the spec.
func (b *Breadcrumbs) AccessibleName() string { return b.Label.Get() }

// SetItems updates the breadcrumb items.
func (b *Breadcrumbs) SetItems(items []BreadcrumbItem) {
	if b == nil {
		return
	}
	next := append([]BreadcrumbItem(nil), items...)
	for i := range next {
		next[i].Label = strings.TrimSpace(next[i].Label)
	}
	b.Items = next
	b.clampIndices()
	b.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// ExportAnchors publishes the breadcrumbs anchor set.
func (b *Breadcrumbs) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if b == nil {
		return nil
	}
	bounds := b.Layout.ArrangedBounds
	out := b.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	if b.currentLayout() != nil {
		layout := b.currentLayout()
		if layout != nil {
			out["baseline"] = gfx.Point{
				X: b.currentLabelBounds().Min.X,
				Y: b.currentLabelBounds().Min.Y + layout.Baseline,
			}
		}
	}
	return out
}

// Children returns the facet's immediate child list.
func (b *Breadcrumbs) Children() []facet.GroupChild { return nil }

// OnAttach is unused beyond layout role setup.
func (b *Breadcrumbs) OnAttach(ctx facet.AttachContext) { b.Core.OnAttach() }

// OnActivate is unused.
func (b *Breadcrumbs) OnActivate() { b.Core.OnActivate() }

// OnDeactivate is unused.
func (b *Breadcrumbs) OnDeactivate() { b.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (b *Breadcrumbs) OnDetach() {
	b.Core.OnDetach()
	b.cachedTokens = theme.Tokens{}
	b.cachedRecipe = shared.BreadcrumbSlots{}
	b.cachedRootBounds = gfx.Rect{}
	b.cachedSegmentListBounds = gfx.Rect{}
	b.cachedItemBounds = nil
	b.cachedLabelBounds = nil
	b.cachedSeparatorBounds = nil
	b.cachedLabelLayouts = nil
	b.cachedSeparatorLayout = nil
	b.cachedLinkStyle = text.TextStyle{}
	b.cachedCurrentStyle = text.TextStyle{}
	b.cachedSeparatorStyle = text.TextStyle{}
	b.cachedGap = 0
	b.cachedPadX = 0
	b.cachedPadY = 0
}

func (b *Breadcrumbs) invalidate(flags facet.DirtyFlags) {
	if b == nil {
		return
	}
	b.Invalidate(flags)
}

func (b *Breadcrumbs) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uinav.ResolveBreadcrumbRecipe(style)
	b.cachedTokens = resolved.TokenSet()
	b.cachedRecipe = slots
	b.cachedWritingDirection = ctx.WritingDirection
	b.cachedGap = mathutil.Max(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(4))
	b.cachedPadX = mathutil.Max(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(8))
	b.cachedPadY = mathutil.Max(float32(resolved.Spacing(theme.SpacingXS)), resolved.Density.Scale(4))
	b.cachedLinkStyle = resolved.TextStyle(theme.TextLabelM)
	b.cachedCurrentStyle = resolved.TextStyle(theme.TextLabelM)
	b.cachedSeparatorStyle = resolved.TextStyle(theme.TextLabelM)
	if b.cachedWritingDirection == facet.WritingDirectionRTL {
	}
	shaper := b.newShaper(ctx.Runtime)
	if shaper == nil {
		b.cachedLabelLayouts = nil
		b.cachedSeparatorLayout = nil
		return facet.MeasureResult{}
	}
	shaper.SetContentScale(ctx.ContentScale)
	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = resolved.Density.Scale(1200)
	}
	labelLayouts := make([]*text.TextLayout, len(b.Items))
	labelBounds := make([]gfx.Rect, len(b.Items))
	itemBounds := make([]gfx.Rect, len(b.Items))
	for i := range b.Items {
		style := b.cachedLinkStyle
		if i == b.clampedCurrentIndex() {
			style = b.cachedCurrentStyle
		}
		labelLayouts[i] = shaper.ShapeTruncated(b.Items[i].Label, style, maxWidth)
		if labelLayouts[i] != nil {
			labelBounds[i] = gfx.RectFromXYWH(0, 0, labelLayouts[i].Bounds.Width(), labelLayouts[i].Bounds.Height())
		}
	}
	separatorLayout := shaper.ShapeTruncated("/", b.cachedSeparatorStyle, maxWidth)
	stripW := float32(0)
	stripH := float32(0)
	for i := range labelLayouts {
		if i > 0 {
			stripW += b.cachedGap
			if separatorLayout != nil {
				stripW += separatorLayout.Bounds.Width()
			}
			stripW += b.cachedGap
		}
		stripW += text.Width(labelLayouts[i])
		if h := text.Height(labelLayouts[i]); h > stripH {
			stripH = h
		}
	}
	if separatorLayout != nil && text.Height(separatorLayout) > stripH {
		stripH = text.Height(separatorLayout)
	}
	if stripH <= 0 {
		stripH = resolved.Density.Scale(20)
	}
	width := stripW + b.cachedPadX*2
	height := mathutil.Max(stripH+b.cachedPadY*2, resolved.Density.Scale(28))
	measured := constraints.Constrain(gfx.Size{W: width, H: height})
	b.cachedLabelLayouts = labelLayouts
	b.cachedSeparatorLayout = separatorLayout
	b.cachedItemBounds = itemBounds
	b.cachedLabelBounds = labelBounds
	if len(b.Items) > 1 {
		b.cachedSeparatorBounds = make([]gfx.Rect, len(b.Items)-1)
	} else {
		b.cachedSeparatorBounds = nil
	}
	b.Layout.MeasuredSize = measured
	b.Layout.MeasuredResult = facet.MeasureResult{
		Size: measured,
		Intrinsic: facet.IntrinsicSize{
			Min:       measured,
			Preferred: measured,
			Max:       measured,
		},
		Constraints: constraints,
	}
	if len(b.cachedLabelLayouts) > 0 {
		b.textRole.Layout = b.cachedLabelLayouts[b.clampedCurrentIndex()]
	}
	b.textRole.Selection = text.TextRange{}
	b.textRole.CaretVisible = false
	b.textRole.CaretPosition = text.TextPosition{}
	return b.Layout.MeasuredResult
}

func (b *Breadcrumbs) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return b.measure(ctx, constraints).Size
}

func (b *Breadcrumbs) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	b.cachedRootBounds = bounds
	b.cachedSegmentListBounds = gfx.Rect{}
	b.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() || len(b.Items) == 0 {
		return
	}
	listBounds := bounds.Inset(b.cachedPadX, b.cachedPadY)
	if listBounds.IsEmpty() {
		listBounds = bounds
	}
	b.cachedSegmentListBounds = listBounds
	stripH := listBounds.Height()
	if stripH <= 0 {
		stripH = mathutil.Max(bounds.Height()-b.cachedPadY*2, 0)
	}
	if stripH <= 0 {
		stripH = text.MaxHeight(b.cachedLabelLayouts...)
		if h := text.Height(b.cachedSeparatorLayout); h > stripH {
			stripH = h
		}
		if stripH <= 0 {
			stripH = 20
		}
	}
	visualIndices := b.visualIndices()
	separatorIndex := 0
	curX := listBounds.Min.X
	for vi, index := range visualIndices {
		labelLayout := b.cachedLabelLayouts[index]
		labelW := text.Width(labelLayout)
		labelH := text.Height(labelLayout)
		itemRect := gfx.RectFromXYWH(curX, listBounds.Min.Y, labelW, stripH)
		curX += labelW
		if b.cachedSeparatorLayout != nil && vi < len(visualIndices)-1 {
			curX += b.cachedGap
			sepW := text.Width(b.cachedSeparatorLayout)
			sepRect := text.AlignRectY(gfx.RectFromXYWH(curX-b.cachedSeparatorLayout.Bounds.Min.X, listBounds.Min.Y, sepW, text.Height(b.cachedSeparatorLayout)), listBounds.Min.Y, stripH)
			b.cachedSeparatorBounds[separatorIndex] = sepRect
			curX += sepW + b.cachedGap
			separatorIndex++
		}
		b.cachedItemBounds[index] = itemRect
		if labelLayout != nil {
			labelX := itemRect.Min.X + mathutil.Max(0, (itemRect.Width()-labelW)*0.5)
			b.cachedLabelBounds[index] = text.AlignRectY(gfx.RectFromXYWH(labelX-labelLayout.Bounds.Min.X, itemRect.Min.Y, labelW, labelH), itemRect.Min.Y, itemRect.Height())
		}
	}
	if len(b.cachedLabelLayouts) > 0 {
		idx := b.clampedCurrentIndex()
		if idx >= 0 && idx < len(b.cachedLabelLayouts) {
			b.textRole.Layout = b.cachedLabelLayouts[idx]
		}
	}
	b.textRole.Selection = text.TextRange{}
	b.textRole.CaretVisible = false
	b.textRole.CaretPosition = text.TextPosition{}
}

func (b *Breadcrumbs) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.BreadcrumbSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: b.cachedTokens}, b.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, b.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uinav.ResolveBreadcrumbRecipe(style)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: b.cachedTokens}, b.cachedRecipe
}

func (b *Breadcrumbs) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if b == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := b.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	root := slots.Root.Resolve(b.rootState(), tokens)
	list := slots.SegmentList.Resolve(theme.StateDefault, tokens)
	link := slots.SegmentLink.Resolve(theme.StateDefault, tokens)
	current := slots.CurrentSegment.Resolve(theme.StateSelected, tokens)
	separator := slots.Separator.Resolve(theme.StateDefault, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)

	cmds := make([]gfx.Command, 0, 64)
	if !theme.IsTransparentMaterial(root) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(bounds), root)...)
	}
	if !theme.IsTransparentMaterial(list) && !b.cachedSegmentListBounds.IsEmpty() {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(b.cachedSegmentListBounds), list)...)
	}
	for i := range b.Items {
		rect := b.cachedItemBounds[i]
		if rect.IsEmpty() {
			continue
		}
		state := b.itemState(i)
		material := link
		if i == b.clampedCurrentIndex() {
			material = current
		}
		switch state {
		case theme.StateDisabled:
			if i == b.clampedCurrentIndex() {
				material = slots.CurrentSegment.Resolve(theme.StateDisabled, tokens)
			} else {
				material = slots.SegmentLink.Resolve(theme.StateDisabled, tokens)
			}
		case theme.StateHover:
			if i == b.clampedCurrentIndex() {
				material = slots.CurrentSegment.Resolve(theme.StateHover, tokens)
			} else {
				material = slots.SegmentLink.Resolve(theme.StateHover, tokens)
			}
		case theme.StatePressed:
			if i == b.clampedCurrentIndex() {
				material = slots.CurrentSegment.Resolve(theme.StatePressed, tokens)
			} else {
				material = slots.SegmentLink.Resolve(theme.StatePressed, tokens)
			}
		case theme.StateFocused:
			if i == b.clampedCurrentIndex() {
				material = slots.CurrentSegment.Resolve(theme.StateFocused, tokens)
			} else {
				material = slots.SegmentLink.Resolve(theme.StateFocused, tokens)
			}
		}
		if label := b.cachedLabelLayouts[i]; label != nil && !theme.IsTransparentMaterial(material) {
			cmds = append(cmds, primitive.TextLayoutCommands(label, b.cachedLabelBounds[i], gfx.SolidBrush(theme.MaterialColor(material)))...)
		}
	}
	for i := range b.cachedSeparatorBounds {
		if b.cachedSeparatorLayout == nil || b.cachedSeparatorBounds[i].IsEmpty() || theme.IsTransparentMaterial(separator) {
			continue
		}
		cmds = append(cmds, primitive.TextLayoutCommands(b.cachedSeparatorLayout, b.cachedSeparatorBounds[i], gfx.SolidBrush(theme.MaterialColor(separator)))...)
	}
	if b.focusedVisible && !theme.IsTransparentMaterial(focus) && b.focusedIndex >= 0 && b.focusedIndex < len(b.cachedItemBounds) {
		active := b.cachedItemBounds[b.focusedIndex]
		if !active.IsEmpty() {
			inset := mathutil.Max(1, active.Height()*0.18)
			cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(active.Inset(-inset, -inset), float32(tokens.Radius.SM)+inset), focus)...)
		}
	}
	return cmds
}

func (b *Breadcrumbs) hitTest(p gfx.Point) facet.HitResult {
	if b == nil || b.Layout.ArrangedBounds.IsEmpty() || !b.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := b.cursorShape()
	if b.focusedVisible && b.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: breadcrumbsMarkIDFocusRing, Cursor: cursor}
	}
	for i := range b.cachedSeparatorBounds {
		if b.cachedSeparatorBounds[i].Contains(p) {
			return facet.HitResult{Hit: true, MarkID: breadcrumbsMarkIDSeparator, Cursor: cursor}
		}
	}
	for i := range b.cachedItemBounds {
		if !b.cachedItemBounds[i].Contains(p) {
			continue
		}
		if i == b.clampedCurrentIndex() {
			return facet.HitResult{Hit: true, MarkID: breadcrumbsMarkIDCurrentSegment, Cursor: b.cursorForIndex(i)}
		}
		return facet.HitResult{Hit: true, MarkID: breadcrumbsMarkIDSegmentLink, Cursor: b.cursorForIndex(i)}
	}
	if b.cachedSegmentListBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: breadcrumbsMarkIDSegmentList, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: breadcrumbsMarkIDRoot, Cursor: cursor}
}

func (b *Breadcrumbs) onPointer(e facet.PointerEvent) bool {
	if b.Disabled.Get() {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		b.hoveredIndex = b.indexAt(e.Position)
		b.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		b.hoveredIndex = -1
		if b.pressedIndex < 0 {
			b.focusFromPointer = false
		}
		b.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		idx := b.indexAt(e.Position)
		if idx < 0 || b.isDisabledIndex(idx) || idx == b.clampedCurrentIndex() {
			return false
		}
		b.hoveredIndex = idx
		b.pressedIndex = idx
		b.focusFromPointer = true
		b.focusedVisible = false
		b.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := b.pressedIndex >= 0
		idx := b.pressedIndex
		b.pressedIndex = -1
		b.invalidate(facet.DirtyProjection)
		if wasPressed {
			if hit := b.indexAt(e.Position); hit >= 0 && hit == idx && !b.isDisabledIndex(hit) && hit != b.clampedCurrentIndex() {
				b.activateIndex(hit)
				return true
			}
			return true
		}
		return false
	case platform.PointerMove:
		b.hoveredIndex = b.indexAt(e.Position)
		b.invalidate(facet.DirtyProjection)
		return true
	default:
		return false
	}
}

func (b *Breadcrumbs) onKey(e facet.KeyEvent) bool {
	if b.Disabled.Get() || len(b.Items) == 0 {
		return false
	}
	switch e.Key {
	case platform.KeyLeft, platform.KeyRight, platform.KeyHome, platform.KeyEnd, platform.KeyPageUp, platform.KeyPageDown, platform.KeySpace, platform.KeyEnter:
		switch e.Kind {
		case platform.KeyPress, platform.KeyRepeat:
			switch e.Key {
			case platform.KeyLeft, platform.KeyPageUp:
				b.moveFocus(-1)
				return true
			case platform.KeyRight, platform.KeyPageDown:
				b.moveFocus(1)
				return true
			case platform.KeyHome:
				b.setFirstFocus()
				return true
			case platform.KeyEnd:
				b.setLastFocus()
				return true
			case platform.KeySpace, platform.KeyEnter:
				b.pressedIndex = b.clampedFocusedIndex()
				b.invalidate(facet.DirtyProjection)
				return true
			}
		case platform.KeyRelease:
			if e.Key == platform.KeySpace || e.Key == platform.KeyEnter {
				wasPressed := b.pressedIndex >= 0
				b.pressedIndex = -1
				b.invalidate(facet.DirtyProjection)
				if wasPressed {
					idx := b.clampedFocusedIndex()
					if idx >= 0 && idx < len(b.Items) && !b.isDisabledIndex(idx) && idx != b.clampedCurrentIndex() {
						b.activateIndex(idx)
						return true
					}
				}
			}
		}
	}
	return false
}

func (b *Breadcrumbs) onFocusGained() {
	b.focusedVisible = !b.focusFromPointer
	b.focusFromPointer = false
	b.focusedIndex = b.firstEnabledIndex()
	b.invalidate(facet.DirtyProjection)
}

func (b *Breadcrumbs) onFocusLost() {
	b.focusedVisible = false
	b.pressedIndex = -1
	b.focusFromPointer = false
	b.invalidate(facet.DirtyProjection)
}

func (b *Breadcrumbs) rootState() theme.InteractionState {
	switch {
	case b.Disabled.Get():
		return theme.StateDisabled
	case b.pressedIndex >= 0:
		return theme.StatePressed
	case b.hoveredIndex >= 0:
		return theme.StateHover
	case b.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (b *Breadcrumbs) itemState(index int) theme.InteractionState {
	if b.Disabled.Get() || b.isDisabledIndex(index) {
		return theme.StateDisabled
	}
	if index == b.clampedCurrentIndex() {
		if b.pressedIndex == index {
			return theme.StatePressed
		}
		if b.hoveredIndex == index {
			return theme.StateHover
		}
		if b.focusedVisible && b.clampedFocusedIndex() == index {
			return theme.StateFocused
		}
		return theme.StateSelected
	}
	if b.pressedIndex == index {
		return theme.StatePressed
	}
	if b.hoveredIndex == index {
		return theme.StateHover
	}
	if b.focusedVisible && b.clampedFocusedIndex() == index {
		return theme.StateFocused
	}
	return theme.StateDefault
}

func (b *Breadcrumbs) activateIndex(index int) {
	if index < 0 || index >= len(b.Items) || b.isDisabledIndex(index) || index == b.clampedCurrentIndex() {
		return
	}
	b.Activated.Emit(index)
}

func (b *Breadcrumbs) moveFocus(delta int) {
	if len(b.Items) == 0 {
		return
	}
	start := b.clampedFocusedIndex()
	for step := 1; step <= len(b.Items); step++ {
		next := start + delta*step
		for next < 0 {
			next += len(b.Items)
		}
		next %= len(b.Items)
		if !b.isDisabledIndex(next) {
			b.focusedIndex = next
			b.invalidate(facet.DirtyProjection)
			return
		}
	}
}

func (b *Breadcrumbs) setFirstFocus() {
	if idx := b.firstEnabledIndex(); idx >= 0 {
		b.focusedIndex = idx
		b.invalidate(facet.DirtyProjection)
	}
}

func (b *Breadcrumbs) setLastFocus() {
	for i := len(b.Items) - 1; i >= 0; i-- {
		if !b.isDisabledIndex(i) {
			b.focusedIndex = i
			b.invalidate(facet.DirtyProjection)
			return
		}
	}
}

func (b *Breadcrumbs) firstEnabledIndex() int {
	for i := range b.Items {
		if !b.isDisabledIndex(i) {
			return i
		}
	}
	return b.clampedCurrentIndex()
}

func (b *Breadcrumbs) clampedCurrentIndex() int {
	idx := b.CurrentIndex.Get()
	if len(b.Items) == 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx >= len(b.Items) {
		return len(b.Items) - 1
	}
	return idx
}

func (b *Breadcrumbs) clampedFocusedIndex() int {
	if len(b.Items) == 0 {
		return 0
	}
	if b.focusedIndex < 0 {
		return 0
	}
	if b.focusedIndex >= len(b.Items) {
		return len(b.Items) - 1
	}
	return b.focusedIndex
}

func (b *Breadcrumbs) clampIndices() {
	if len(b.Items) == 0 {
		b.CurrentIndex = marks.Const(0)
		b.focusedIndex = 0
		return
	}
	ci := b.CurrentIndex.Get()
	if ci < 0 || ci >= len(b.Items) {
		b.CurrentIndex = marks.Const(len(b.Items) - 1)
	}
	if b.focusedIndex < 0 || b.focusedIndex >= len(b.Items) {
		b.focusedIndex = ci
	}
	if b.isDisabledIndex(b.focusedIndex) {
		for i := range b.Items {
			if !b.isDisabledIndex(i) {
				b.focusedIndex = i
				return
			}
		}
	}
}

func (b *Breadcrumbs) isDisabledIndex(index int) bool {
	if index < 0 || index >= len(b.Items) {
		return true
	}
	return b.Disabled.Get() || b.Items[index].Disabled
}

func (b *Breadcrumbs) indexAt(p gfx.Point) int {
	for i := range b.cachedItemBounds {
		if b.cachedItemBounds[i].Contains(p) {
			return i
		}
	}
	return -1
}

func (b *Breadcrumbs) cursorShape() facet.CursorShape {
	if b.Disabled.Get() {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (b *Breadcrumbs) cursorForIndex(index int) facet.CursorShape {
	if b.Disabled.Get() || b.isDisabledIndex(index) || index == b.clampedCurrentIndex() {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (b *Breadcrumbs) pointInFocusRing(p gfx.Point) bool {
	if !b.focusedVisible || len(b.cachedItemBounds) == 0 {
		return false
	}
	idx := b.clampedFocusedIndex()
	if idx < 0 || idx >= len(b.cachedItemBounds) {
		return false
	}
	active := b.cachedItemBounds[idx]
	if active.IsEmpty() || !active.Contains(p) {
		return false
	}
	ring := mathutil.Max(1, active.Height()*0.16)
	inner := active.Inset(ring, ring)
	if inner.IsEmpty() {
		return true
	}
	return !inner.Contains(p)
}

func (b *Breadcrumbs) visualIndices() []int {
	indices := make([]int, len(b.Items))
	if b.cachedWritingDirection == facet.WritingDirectionRTL {
		for i := range b.Items {
			indices[i] = len(b.Items) - 1 - i
		}
		return indices
	}
	for i := range b.Items {
		indices[i] = i
	}
	return indices
}

func (b *Breadcrumbs) currentLayout() *text.TextLayout {
	idx := b.clampedCurrentIndex()
	if idx < 0 || idx >= len(b.cachedLabelLayouts) {
		return nil
	}
	return b.cachedLabelLayouts[idx]
}

func (b *Breadcrumbs) currentLabelBounds() gfx.Rect {
	idx := b.clampedCurrentIndex()
	if idx < 0 || idx >= len(b.cachedLabelBounds) {
		return gfx.Rect{}
	}
	return b.cachedLabelBounds[idx]
}

func (b *Breadcrumbs) newShaper(runtime any) *text.Shaper {
	registry := b.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (b *Breadcrumbs) fontRegistry(runtime any) *text.FontRegistry {
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

type breadcrumbsGroupPolicy struct{}

func (breadcrumbsGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }
func (breadcrumbsGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}
func (breadcrumbsGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}
