package selection

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

const (
	dropdownSelectMarkIDRoot            facet.MarkID = 1
	dropdownSelectMarkIDTrigger         facet.MarkID = 2
	dropdownSelectMarkIDSelectedValue   facet.MarkID = 3
	dropdownSelectMarkIDChevron         facet.MarkID = 4
	dropdownSelectMarkIDFloatingListbox facet.MarkID = 5
	dropdownSelectMarkIDOptionItems     facet.MarkID = 6
	dropdownSelectMarkIDFocusRing       facet.MarkID = 7
)

// DropdownOption describes one selectable choice.
type DropdownOption struct {
	Value string
	Label string
}

// DropdownSelect implements the selection.dropdown_select canonical mark.
type DropdownSelect struct {
	marks.Core

	Value *store.ValueStore[string]

	Label       marks.Binding[string]
	Placeholder marks.Binding[string]
	Options     marks.Binding[[]DropdownOption]
	Variant     marks.Binding[uiinput.SelectVariant]
	Disabled    marks.Binding[bool]
	Invalid     marks.Binding[bool]

	textRole facet.TextRole

	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool
	open             bool
	activeIndex      int
	scrollOffset     float32

	cachedTokens           theme.Tokens
	cachedRecipe           shared.SelectSlots
	cachedRootBounds       gfx.Rect
	cachedLabelLayout      *text.TextLayout
	cachedLabelBounds      gfx.Rect
	cachedTriggerBounds    gfx.Rect
	cachedValueBounds      gfx.Rect
	cachedChevronBounds    gfx.Rect
	cachedListboxBounds    gfx.Rect
	cachedOptionRects      []gfx.Rect
	cachedOptionGap        float32
	cachedOptionHeight     float32
	cachedTriggerHeight    float32
	cachedTriggerRadius    float32
	cachedLabelStyle       text.TextStyle
	cachedValueStyle       text.TextStyle
	cachedWritingDirection facet.WritingDirection
}

var _ facet.FacetImpl = (*DropdownSelect)(nil)
var _ layout.AnchorExporter = (*DropdownSelect)(nil)
var _ marks.Mark = (*DropdownSelect)(nil)

// NewDropdownSelect constructs a dropdown select with canonical defaults.
func NewDropdownSelect(label string, options []DropdownOption) *DropdownSelect {
	ds := &DropdownSelect{
		Label:       marks.Const(label),
		Placeholder: marks.Const("Select..."),
		Options:     marks.Const(append([]DropdownOption(nil), options...)),
		Variant:     marks.Const(uiinput.SelectStandard),
		Disabled:    marks.Const(false),
		Invalid:     marks.Const(false),
		Value:       store.NewValueStore[string](""),
		activeIndex: 0,
	}
	ds.Core.Facet = facet.NewFacet()
	ds.AddBinding(ds.Label)
	ds.AddBinding(ds.Placeholder)
	ds.AddBinding(ds.Options)
	ds.AddBinding(ds.Variant)
	ds.AddBinding(ds.Disabled)
	ds.AddBinding(ds.Invalid)

	ds.Layout.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearVertical,
		Policy: dropdownSelectGroupPolicy{},
	}
	ds.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := ds.measureIntrinsic(ctx, constraints)
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionClip,
			BelowMinHeight: facet.CompressionClip,
			AboveMaxWidth:  facet.ExpansionClip,
			AboveMaxHeight: facet.ExpansionClip,
		},
		Stretch:  facet.StretchPolicy{Width: facet.StretchNever, Height: facet.StretchNever},
		Baseline: facet.BaselineNone,
	}
	ds.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return ds.measure(ctx, constraints)
	}
	ds.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		ds.Layout.ArrangedBounds = bounds
		ds.arrange(ctx, bounds)
	}
	ds.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return ds.hitTest(p) }
	ds.Input.OnPointer = func(e facet.PointerEvent) bool { return ds.onPointer(e) }
	ds.Input.OnScroll = func(e facet.ScrollEvent) bool { return ds.onScroll(e) }
	ds.Input.OnKey = func(e facet.KeyEvent) bool { return ds.onKey(e) }
	ds.Input.OnDismiss = func(e facet.DismissEvent) bool { return ds.onDismiss(e) }
	ds.Focus.Focusable = func() bool { return !ds.Disabled.Get() }
	ds.Focus.TabIndex = 0
	ds.Focus.OnFocusGained = func() { ds.onFocusGained() }
	ds.Focus.OnFocusLost = func() { ds.onFocusLost() }
	ds.Viewport.Transform = gfx.Identity()
	ds.textRole.IMEEnabled = false
	ds.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return ds.buildCommands(ds.Layout.ArrangedBounds, ctx.Runtime)
	}
	ds.RegisterRoles()
	ds.AddRole(&ds.textRole)
	return ds
}

// Base satisfies facet.FacetImpl.
func (ds *DropdownSelect) Base() *facet.Facet {
	ds.Facet.BindImpl(ds)
	return &ds.Facet
}

// Descriptor satisfies marks.Mark.
func (ds *DropdownSelect) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "selection", TypeName: "dropdown_select"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (ds *DropdownSelect) AccessibilityRole() string { return "combobox" }

// AccessibleName reports the semantic name source required by the spec.
func (ds *DropdownSelect) AccessibleName() string {
	if ds == nil {
		return ""
	}
	return ds.Label.Get()
}

// ExportAnchors publishes the select anchor set.
func (ds *DropdownSelect) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	bounds := ds.Layout.ArrangedBounds
	if bounds.IsEmpty() && !ctx.ResolvedLayer.Bounds.IsEmpty() {
		bounds = ctx.ResolvedLayer.Bounds
	}
	if bounds.IsEmpty() {
		return nil
	}
	anchors := ds.Core.DefaultAnchors(bounds, ctx)
	if ds.cachedLabelBounds.IsEmpty() || ds.textRole.Layout == nil {
		anchors["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	} else {
		anchors["baseline"] = gfx.Point{X: ds.cachedLabelBounds.Min.X, Y: ds.cachedLabelBounds.Min.Y + ds.textRole.Layout.Baseline}
	}
	if !ds.cachedTriggerBounds.IsEmpty() {
		anchors["content_anchor"] = gfx.Point{X: ds.cachedTriggerBounds.Min.X, Y: ds.cachedTriggerBounds.Max.Y}
	}
	return anchors
}

// Children returns the facet's immediate child list.
func (ds *DropdownSelect) Children() []facet.GroupChild { return nil }

// OnAttach wires store invalidation for the bound value store.
func (ds *DropdownSelect) OnAttach(ctx facet.AttachContext) {
	ds.Core.OnAttach()
	if ds.Value == nil {
		ds.Value = store.NewValueStore[string]("")
	}
	facet.Store(facet.Subscribe(ds), &ds.Value.OnChange, ds.Value.Version, func(signal.Change[string]) {
		ds.syncActiveIndex()
		ds.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	})
}

// OnActivate is unused.
func (ds *DropdownSelect) OnActivate() { ds.Core.OnActivate() }

// OnDeactivate is unused.
func (ds *DropdownSelect) OnDeactivate() { ds.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (ds *DropdownSelect) OnDetach() {
	ds.Core.OnDetach()
	ds.cachedTokens = theme.Tokens{}
	ds.cachedRecipe = shared.SelectSlots{}
	ds.cachedRootBounds = gfx.Rect{}
	ds.cachedLabelLayout = nil
	ds.cachedLabelBounds = gfx.Rect{}
	ds.cachedTriggerBounds = gfx.Rect{}
	ds.cachedValueBounds = gfx.Rect{}
	ds.cachedChevronBounds = gfx.Rect{}
	ds.cachedListboxBounds = gfx.Rect{}
	ds.cachedOptionRects = nil
	ds.cachedOptionGap = 0
	ds.cachedOptionHeight = 0
	ds.cachedTriggerHeight = 0
	ds.cachedTriggerRadius = 0
	ds.cachedLabelStyle = text.TextStyle{}
	ds.cachedValueStyle = text.TextStyle{}
}

func (ds *DropdownSelect) invalidate(flags facet.DirtyFlags) {
	if ds == nil {
		return
	}
	ds.Base().Invalidate(flags)
}

func (ds *DropdownSelect) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uiinput.ResolveSelectRecipe(style, ds.Variant.Get())
	ds.cachedTokens = resolved.TokenSet()
	ds.cachedRecipe = slots
	ds.cachedWritingDirection = ctx.WritingDirection
	ds.cachedTriggerHeight = maxFloat(resolved.Density.Scale(36), resolved.Density.Scale(resolved.TokenSet().Spacing.TouchTarget))
	ds.cachedTriggerRadius = float32(resolved.Radius(theme.RadiusM))
	ds.cachedOptionGap = float32(resolved.Spacing(theme.SpacingXS))
	ds.cachedLabelStyle = resolved.TextStyle(theme.TextLabelM)
	ds.cachedValueStyle = resolved.TextStyle(theme.TextBodyM)
	shaper := ds.newShaper(ctx.Runtime)
	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = resolved.Density.Scale(320)
	}
	labelLayout := (*text.TextLayout)(nil)
	valueLayout := (*text.TextLayout)(nil)
	if shaper != nil {
		shaper.SetContentScale(ctx.ContentScale)
		labelLayout = shaper.ShapeTruncated(ds.Label.Get(), ds.cachedLabelStyle, maxWidth)
		selected := ds.displayValue()
		if selected == "" {
			selected = ds.Placeholder.Get()
		}
		valueLayout = shaper.ShapeTruncated(selected, ds.cachedValueStyle, maxWidth)
	}
	ds.textRole.Layout = valueLayout
	ds.textRole.Selection = text.TextRange{}
	ds.textRole.CaretVisible = false
	ds.textRole.CaretPosition = text.TextPosition{}
	labelH := text.Height(labelLayout)
	valueH := text.Height(valueLayout)
	triggerH := maxFloat(ds.cachedTriggerHeight, maxFloat(valueH, resolved.Density.Scale(36)))
	width := maxFloat(240, maxFloat(text.Width(labelLayout), text.Width(valueLayout))+resolved.Density.Scale(48))
	if width <= 0 {
		width = resolved.Density.Scale(240)
	}
	height := float32(0)
	if labelH > 0 {
		height += labelH + float32(resolved.Spacing(theme.SpacingXS))
	}
	height += triggerH
	if ds.open {
		if len(ds.Options.Get()) > 0 {
			height += float32(resolved.Spacing(theme.SpacingXS))
			itemH := maxFloat(float32(resolved.Density.Scale(32)), triggerH*0.72)
			if itemH < 28 {
				itemH = 28
			}
			ds.cachedOptionHeight = itemH
			count := len(ds.Options.Get())
			if count > 6 {
				count = 6
			}
			height += float32(count)*itemH + float32(count-1)*float32(resolved.Spacing(theme.SpacingXS))
		}
	}
	if height <= 0 {
		height = triggerH
	}
	ds.cachedRootBounds = gfx.RectFromXYWH(0, 0, width, height)
	ds.cachedLabelLayout = labelLayout
	ds.Layout.MeasuredSize = gfx.Size{W: width, H: height}
	ds.Layout.MeasuredResult = facet.MeasureResult{
		Size: gfx.Size{W: width, H: height},
		Intrinsic: facet.IntrinsicSize{
			Min:       gfx.Size{W: width, H: triggerH},
			Preferred: gfx.Size{W: width, H: height},
			Max:       gfx.Size{W: width, H: height},
		},
		Constraints: constraints,
	}
	ds.textRole.Layout = valueLayout
	return ds.Layout.MeasuredResult
}

func (ds *DropdownSelect) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return ds.measure(ctx, constraints).Size
}

func (ds *DropdownSelect) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	ds.cachedRootBounds = bounds
	ds.cachedLabelBounds = gfx.Rect{}
	ds.cachedTriggerBounds = gfx.Rect{}
	ds.cachedValueBounds = gfx.Rect{}
	ds.cachedChevronBounds = gfx.Rect{}
	ds.cachedListboxBounds = gfx.Rect{}
	ds.cachedOptionRects = nil
	ds.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	labelH := text.Height(ds.textRole.Layout)
	rects := layout.ArrangeVerticalFlow(bounds, 0, float32(resolved.Spacing(theme.SpacingXS)), []gfx.Size{
		{W: bounds.Width(), H: labelH},
		{W: bounds.Width(), H: ds.cachedTriggerHeight},
	}, ds.cachedWritingDirection == facet.WritingDirectionRTL)
	if ds.textRole.Layout != nil {
		ds.cachedLabelBounds = rects[0]
	}
	ds.cachedTriggerBounds = rects[1]
	padding := maxFloat(12, ds.cachedTriggerBounds.Height()*0.28)
	chevronSize := maxFloat(10, ds.cachedTriggerBounds.Height()*0.22)
	textWidth := maxFloat(0, ds.cachedTriggerBounds.Width()-padding*3-chevronSize)
	valueLayout := ds.textRole.Layout
	if valueLayout == nil {
		valueLayout = nil
	}
	valueH := text.Height(valueLayout)
	valueX := ds.cachedTriggerBounds.Min.X + padding
	if ds.cachedWritingDirection == facet.WritingDirectionRTL {
		valueX = ds.cachedTriggerBounds.Max.X - padding - textWidth
	}
	ds.cachedValueBounds = text.AlignRectY(gfx.RectFromXYWH(valueX, ds.cachedTriggerBounds.Min.Y, textWidth, maxFloat(valueH, 0)), ds.cachedTriggerBounds.Min.Y, ds.cachedTriggerBounds.Height())
	chevronX := ds.cachedTriggerBounds.Max.X - padding - chevronSize
	if ds.cachedWritingDirection == facet.WritingDirectionRTL {
		chevronX = ds.cachedTriggerBounds.Min.X + padding
	}
	ds.cachedChevronBounds = text.AlignRectY(gfx.RectFromXYWH(chevronX, ds.cachedTriggerBounds.Min.Y, chevronSize, chevronSize), ds.cachedTriggerBounds.Min.Y, ds.cachedTriggerBounds.Height())
	if ds.open && len(ds.Options.Get()) > 0 {
		itemH := ds.cachedOptionHeight
		if itemH <= 0 {
			itemH = maxFloat(float32(resolved.Density.Scale(32)), ds.cachedTriggerBounds.Height()*0.72)
			if itemH < 28 {
				itemH = 28
			}
		}
		count := len(ds.Options.Get())
		if count > 6 {
			count = 6
		}
		listboxH := float32(count)*itemH + float32(count-1)*float32(resolved.Spacing(theme.SpacingXS))
		ds.cachedListboxBounds = gfx.RectFromXYWH(bounds.Min.X, ds.cachedTriggerBounds.Max.Y+float32(resolved.Spacing(theme.SpacingXS)), bounds.Width(), listboxH)
		ds.cachedOptionRects = ds.layoutOptionRects(ds.cachedListboxBounds, resolved)
	}
}

func (ds *DropdownSelect) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.SelectSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: ds.cachedTokens}, ds.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, ds.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uiinput.ResolveSelectRecipe(style, ds.Variant.Get())
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: ds.cachedTokens}, ds.cachedRecipe
}

func (ds *DropdownSelect) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if ds == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := ds.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	interaction := ds.interactionState()
	valueState := ds.valueState()
	root := slots.Root.Resolve(interaction, tokens)
	trigger := slots.Trigger.Resolve(interaction, tokens)
	label := slots.SelectedValueLabel.Resolve(valueState, tokens)
	chevron := slots.Chevron.Resolve(interaction, tokens)
	listbox := slots.FloatingListbox.Resolve(ds.listboxState(), tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)
	if ds.Invalid.Get() {
		focus = theme.FromToken(tokens.Color.Error)
	}
	cmds := make([]gfx.Command, 0, 32)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(trigger) && !ds.cachedTriggerBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ds.cachedTriggerBounds, ds.cachedTriggerRadius), trigger)...)
	}
	if !isTransparentMaterial(label) && !ds.cachedLabelBounds.IsEmpty() {
		if ds.cachedLabelLayout != nil {
			cmds = append(cmds, primitive.TextLayoutCommands(ds.cachedLabelLayout, ds.cachedLabelBounds, gfx.SolidBrush(materialColor(label)))...)
		}
	}
	if !isTransparentMaterial(chevron) && !ds.cachedChevronBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(ds.chevronPath(ds.cachedChevronBounds), chevron)...)
	}
	if ds.open && !ds.cachedListboxBounds.IsEmpty() {
		if !isTransparentMaterial(listbox) {
			cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ds.cachedListboxBounds, ds.cachedTriggerRadius), listbox)...)
		}
		for i, rect := range ds.cachedOptionRects {
			if i < 0 || i >= len(ds.Options.Get()) {
				continue
			}
			row := NewListItem(marks.Const(ds.labelAt(i)))
			row.Selected = marks.Const(ds.optionValueAt(i) == ds.selectedValue())
			row.Active = marks.Const(i == ds.activeIndex)
			row.Disabled = marks.Const(ds.Disabled.Get())
			row.focusedVisible = i == ds.activeIndex && ds.open
			row.cachedWritingDirection = ds.cachedWritingDirection
			runtimeServices, _ := runtime.(facet.RuntimeServices)
			densityID := facet.DensityID(theme.DensityIDComfortable)
			switch tokens.Density.Mode {
			case theme.DensityCompact:
				densityID = facet.DensityID(theme.DensityIDCompact)
			case theme.DensityTouch:
				densityID = facet.DensityID(theme.DensityIDTouch)
			}
			row.Layout.Measure(facet.MeasureContext{
				Runtime:          runtimeServices,
				Theme:            style,
				ContentScale:     1,
				Density:          densityID,
				WritingDirection: ds.cachedWritingDirection,
			}, facet.Constraints{MaxSize: gfx.Size{W: rect.Width(), H: rect.Height()}})
			row.Layout.Arrange(facet.ArrangeContext{
				Runtime:     runtimeServices,
				Theme:       style,
				ParentGroup: row.Layout.Parent,
				ChildGroup:  row.Layout.Child,
			}, rect)
			cmds = append(cmds, row.buildCommands(rect, runtime)...)
		}
	}
	if ds.focusedVisible && !isTransparentMaterial(focus) {
		inset := maxFloat(1, ds.cachedTriggerBounds.Height()*0.08)
		ringBounds := ds.cachedTriggerBounds.Inset(-inset, -inset)
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ringBounds, ds.cachedTriggerRadius+inset), focus)...)
	}
	return cmds
}

func (ds *DropdownSelect) hitTest(p gfx.Point) facet.HitResult {
	if ds == nil || ds.Layout.ArrangedBounds.IsEmpty() || !ds.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := ds.cursorShape()
	if ds.focusedVisible && ds.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: dropdownSelectMarkIDFocusRing, Cursor: cursor}
	}
	if ds.open && ds.cachedListboxBounds.Contains(p) {
		for _, rect := range ds.cachedOptionRects {
			if rect.Contains(p) {
				return facet.HitResult{Hit: true, MarkID: dropdownSelectMarkIDOptionItems, Cursor: cursor}
			}
		}
		return facet.HitResult{Hit: true, MarkID: dropdownSelectMarkIDFloatingListbox, Cursor: cursor}
	}
	if ds.cachedChevronBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: dropdownSelectMarkIDChevron, Cursor: cursor}
	}
	if ds.cachedValueBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: dropdownSelectMarkIDSelectedValue, Cursor: cursor}
	}
	if ds.cachedTriggerBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: dropdownSelectMarkIDTrigger, Cursor: cursor}
	}
	if ds.cachedLabelBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: dropdownSelectMarkIDSelectedValue, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: dropdownSelectMarkIDRoot, Cursor: cursor}
}

func (ds *DropdownSelect) pointInFocusRing(p gfx.Point) bool {
	bounds := ds.cachedTriggerBounds
	if bounds.IsEmpty() || !bounds.Contains(p) {
		return false
	}
	inset := maxFloat(1, bounds.Height()*0.08)
	inner := bounds.Inset(inset, inset)
	if inner.IsEmpty() {
		return true
	}
	return !inner.Contains(p)
}

func (ds *DropdownSelect) cursorShape() facet.CursorShape {
	if ds.Disabled.Get() {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (ds *DropdownSelect) onPointer(e facet.PointerEvent) bool {
	if ds.Disabled.Get() {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		ds.hovered = true
		ds.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		ds.hovered = false
		if !ds.pressed {
			ds.focusFromPointer = false
		}
		ds.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		ds.hovered = true
		ds.pressed = true
		ds.focusFromPointer = true
		ds.focusedVisible = false
		ds.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := ds.pressed
		ds.pressed = false
		ds.invalidate(facet.DirtyProjection)
		if !wasPressed {
			return false
		}
		if ds.open && ds.cachedListboxBounds.Contains(e.Position) {
			if idx, ok := ds.optionIndexAt(e.Position); ok {
				ds.chooseIndex(idx)
				ds.open = false
				return true
			}
		}
		if ds.cachedTriggerBounds.Contains(e.Position) {
			ds.open = !ds.open
			if ds.open {
				ds.syncActiveIndex()
			}
			ds.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
			return true
		}
		return true
	case platform.PointerMove:
		if ds.open && ds.cachedListboxBounds.Contains(e.Position) {
			if idx, ok := ds.optionIndexAt(e.Position); ok {
				ds.activeIndex = idx
				ds.invalidate(facet.DirtyProjection)
			}
		}
		return true
	default:
		return false
	}
}

func (ds *DropdownSelect) onScroll(e facet.ScrollEvent) bool {
	if ds.Disabled.Get() || !ds.open || ds.cachedListboxBounds.IsEmpty() {
		return false
	}
	if e.DeltaY == 0 {
		return false
	}
	ds.scrollOffset -= e.DeltaY
	maxOffset := maxFloat(0, float32(len(ds.Options.Get()))*ds.cachedOptionHeight-ds.cachedListboxBounds.Height())
	ds.scrollOffset = clampFloat(ds.scrollOffset, 0, maxOffset)
	ds.invalidate(facet.DirtyProjection)
	return true
}

func (ds *DropdownSelect) onKey(e facet.KeyEvent) bool {
	if ds.Disabled.Get() {
		return false
	}
	switch e.Key {
	case platform.KeySpace, platform.KeyEnter:
		switch e.Kind {
		case platform.KeyPress, platform.KeyRepeat:
			ds.pressed = true
			ds.invalidate(facet.DirtyProjection)
			return true
		case platform.KeyRelease:
			wasPressed := ds.pressed
			ds.pressed = false
			ds.invalidate(facet.DirtyProjection)
			if wasPressed {
				if ds.open {
					ds.chooseIndex(ds.activeIndex)
				}
				ds.open = !ds.open
				if ds.open {
					ds.syncActiveIndex()
				}
				ds.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
			}
			return wasPressed
		}
	case platform.KeyEscape:
		if e.Kind == platform.KeyPress {
			if ds.open {
				ds.open = false
				ds.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
				return true
			}
		}
	case platform.KeyUp, platform.KeyDown, platform.KeyHome, platform.KeyEnd, platform.KeyPageUp, platform.KeyPageDown:
		if e.Kind == platform.KeyPress && ds.open {
			ds.navigateKey(e.Key)
			return true
		}
	default:
		if e.Kind == platform.KeyPress {
			if ds.typeahead(e.Key) {
				return true
			}
		}
	}
	return false
}

func (ds *DropdownSelect) onDismiss(e facet.DismissEvent) bool {
	_ = e
	if ds.Disabled.Get() || !ds.open {
		return false
	}
	ds.open = false
	ds.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	return true
}

func (ds *DropdownSelect) onFocusGained() {
	ds.focusedVisible = !ds.focusFromPointer
	ds.focusFromPointer = false
	ds.invalidate(facet.DirtyProjection)
}

func (ds *DropdownSelect) onFocusLost() {
	ds.focusedVisible = false
	ds.pressed = false
	ds.focusFromPointer = false
	ds.invalidate(facet.DirtyProjection)
}

func (ds *DropdownSelect) interactionState() theme.InteractionState {
	switch {
	case ds.Disabled.Get():
		return theme.StateDisabled
	case ds.pressed:
		return theme.StatePressed
	case ds.hovered:
		return theme.StateHover
	case ds.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (ds *DropdownSelect) valueState() theme.InteractionState {
	if ds.selectedValue() != "" {
		return theme.StateSelected
	}
	return ds.interactionState()
}

func (ds *DropdownSelect) listboxState() theme.InteractionState {
	switch {
	case ds.Disabled.Get():
		return theme.StateDisabled
	case ds.open:
		return theme.StateSelected
	default:
		return ds.interactionState()
	}
}

func (ds *DropdownSelect) optionItemsState() theme.InteractionState {
	if ds.Disabled.Get() {
		return theme.StateDisabled
	}
	if ds.open {
		return theme.StateSelected
	}
	return theme.StateDefault
}

func (ds *DropdownSelect) selectedValue() string {
	if ds == nil || ds.Value == nil {
		return ""
	}
	return ds.Value.Get()
}

func (ds *DropdownSelect) optionValueAt(i int) string {
	if i < 0 || i >= len(ds.Options.Get()) {
		return ""
	}
	return ds.Options.Get()[i].Value
}

func (ds *DropdownSelect) labelAt(i int) string {
	if i < 0 || i >= len(ds.Options.Get()) {
		return ""
	}
	return ds.Options.Get()[i].Label
}

func (ds *DropdownSelect) displayValue() string {
	value := ds.selectedValue()
	if value == "" {
		return ""
	}
	if label := ds.labelForValue(value); label != "" {
		return label
	}
	return value
}

func (ds *DropdownSelect) labelForValue(value string) string {
	for _, opt := range ds.Options.Get() {
		if opt.Value == value {
			return opt.Label
		}
	}
	return ""
}

func (ds *DropdownSelect) syncActiveIndex() {
	selected := ds.selectedValue()
	for i, opt := range ds.Options.Get() {
		if opt.Value == selected {
			ds.activeIndex = i
			return
		}
	}
	if ds.activeIndex < 0 || ds.activeIndex >= len(ds.Options.Get()) {
		ds.activeIndex = 0
	}
}

func (ds *DropdownSelect) chooseIndex(i int) {
	if i < 0 || i >= len(ds.Options.Get()) {
		return
	}
	ds.Value.Set(ds.Options.Get()[i].Value)
}

func (ds *DropdownSelect) navigateKey(key platform.Key) {
	if len(ds.Options.Get()) == 0 {
		return
	}
	switch key {
	case platform.KeyHome:
		ds.activeIndex = 0
	case platform.KeyEnd:
		ds.activeIndex = len(ds.Options.Get()) - 1
	case platform.KeyPageUp:
		ds.activeIndex = maxInt(0, ds.activeIndex-5)
	case platform.KeyPageDown:
		ds.activeIndex = minInt(len(ds.Options.Get())-1, ds.activeIndex+5)
	case platform.KeyUp:
		ds.activeIndex = maxInt(0, ds.activeIndex-1)
	case platform.KeyDown:
		ds.activeIndex = minInt(len(ds.Options.Get())-1, ds.activeIndex+1)
	}
	ds.invalidate(facet.DirtyProjection)
}

func (ds *DropdownSelect) typeahead(key platform.Key) bool {
	if key < platform.KeyA || key > platform.KeyZ {
		return false
	}
	if len(ds.Options.Get()) == 0 {
		return false
	}
	target := strings.ToLower(string(rune('a' + int(key-platform.KeyA))))
	start := ds.activeIndex + 1
	for offset := 0; offset < len(ds.Options.Get()); offset++ {
		i := (start + offset) % len(ds.Options.Get())
		label := strings.ToLower(ds.Options.Get()[i].Label)
		if strings.HasPrefix(label, target) {
			ds.activeIndex = i
			if ds.open {
				ds.invalidate(facet.DirtyProjection)
			}
			return true
		}
	}
	return false
}

func (ds *DropdownSelect) optionIndexAt(p gfx.Point) (int, bool) {
	for i, rect := range ds.cachedOptionRects {
		if rect.Contains(p) {
			return i, true
		}
	}
	return -1, false
}

func (ds *DropdownSelect) layoutOptionRects(listbox gfx.Rect, resolved theme.ResolvedContext) []gfx.Rect {
	if listbox.IsEmpty() || len(ds.Options.Get()) == 0 {
		return nil
	}
	itemH := ds.cachedOptionHeight
	if itemH <= 0 {
		itemH = maxFloat(float32(resolved.Density.Scale(32)), 28)
	}
	gap := float32(resolved.Spacing(theme.SpacingXS))
	outerPad := float32(resolved.Spacing(theme.SpacingS))
	rects := make([]gfx.Rect, 0, len(ds.Options.Get()))
	y := listbox.Min.Y + outerPad
	for i := range ds.Options.Get() {
		if i > 0 {
			y += gap
		}
		rect := gfx.RectFromXYWH(listbox.Min.X+outerPad, y, listbox.Width()-outerPad*2, itemH)
		rects = append(rects, rect)
		y += itemH
	}
	return rects
}

func (ds *DropdownSelect) shapeOptionLabel(runtime any, i int, maxWidth float32) *text.TextLayout {
	if i < 0 || i >= len(ds.Options.Get()) {
		return nil
	}
	shaper := ds.newShaper(runtime)
	if shaper == nil {
		return nil
	}
	shaper.SetContentScale(1)
	return shaper.ShapeTruncated(ds.Options.Get()[i].Label, ds.cachedValueStyle, maxWidth)
}

func (ds *DropdownSelect) chevronPath(bounds gfx.Rect) gfx.Path {
	if bounds.IsEmpty() {
		return gfx.Path{}
	}
	if ds.cachedWritingDirection == facet.WritingDirectionRTL {
		return gfx.NewPath().
			MoveTo(gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y + bounds.Height()*0.35}).
			LineTo(gfx.Point{X: bounds.Min.X + bounds.Width()*0.5, Y: bounds.Max.Y}).
			LineTo(gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y + bounds.Height()*0.35}).
			Build()
	}
	return gfx.NewPath().
		MoveTo(gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y + bounds.Height()*0.35}).
		LineTo(gfx.Point{X: bounds.Min.X + bounds.Width()*0.5, Y: bounds.Max.Y}).
		LineTo(gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y + bounds.Height()*0.35}).
		Build()
}

func (ds *DropdownSelect) newShaper(runtime any) *text.Shaper {
	registry := ds.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (ds *DropdownSelect) fontRegistry(runtime any) *text.FontRegistry {
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

func (ds *DropdownSelect) onKeyDownOrUp(key platform.Key) bool {
	if !ds.open || len(ds.Options.Get()) == 0 {
		return false
	}
	ds.navigateKey(key)
	return true
}

func minInt(a, b int) int {
	if a < b {
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

type dropdownSelectGroupPolicy struct{}

func (dropdownSelectGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (dropdownSelectGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}

func (dropdownSelectGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}
