package selection

import (
	"math"
	"reflect"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

const (
	buttonGroupMarkIDRoot              facet.MarkID = 1
	buttonGroupMarkIDGroupSurface      facet.MarkID = 2
	buttonGroupMarkIDOptionButtons     facet.MarkID = 3
	buttonGroupMarkIDSelectedIndicator facet.MarkID = 4
	buttonGroupMarkIDFocusRing         facet.MarkID = 5
)

// ButtonGroupMode selects the canonical selection behavior.
type ButtonGroupMode uint8

const (
	// ButtonGroupExclusive allows only one selected option.
	ButtonGroupExclusive ButtonGroupMode = iota
	// ButtonGroupMultiple allows toggling many selected options.
	ButtonGroupMultiple
)

func (m ButtonGroupMode) String() string {
	switch m {
	case ButtonGroupExclusive:
		return "exclusive"
	case ButtonGroupMultiple:
		return "multiple"
	default:
		return "unknown"
	}
}

// ButtonGroupOption describes one button in the group.
type ButtonGroupOption struct {
	Key      string
	Label    string
	Icon     primitive.IconSource
	Disabled bool
}

// ButtonGroup implements the selection.button_group standard mark.
type ButtonGroup struct {
	marks.Core

	Activated signal.Signal[string]

	Label    marks.Binding[string]
	Options  []ButtonGroupOption
	Value    *store.ValueStore[[]string]
	Mode     marks.Binding[ButtonGroupMode]
	Disabled marks.Binding[bool]

	textRole facet.TextRole

	hoveredIndex     int
	pressedIndex     int
	focusedIndex     int
	focusedVisible   bool
	focusFromPointer bool

	cachedTokens           theme.Tokens
	cachedRecipe           shared.ButtonGroupSlots
	cachedRootBounds       gfx.Rect
	cachedGroupSurface     gfx.Rect
	cachedOptionBounds     []gfx.Rect
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedRadius           float32
	cachedWritingDirection facet.WritingDirection

	cachedItems []*buttonGroupItem
}

type buttonGroupItem struct {
	marks.Core

	textRole facet.TextRole

	parent *ButtonGroup
	index  int

	label     string
	icon      primitive.IconSource
	disabled  bool
	selected  bool
	iconMark  *primitive.Icon
	labelMark *primitive.Text

	cachedTokens           theme.Tokens
	cachedRecipe           shared.ButtonGroupSlots
	cachedBounds           gfx.Rect
	cachedIconBounds       gfx.Rect
	cachedLabelBounds      gfx.Rect
	cachedRadius           float32
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedWritingDirection facet.WritingDirection
}

var _ facet.FacetImpl = (*ButtonGroup)(nil)
var _ layout.AnchorExporter = (*ButtonGroup)(nil)
var _ marks.Mark = (*ButtonGroup)(nil)
var _ facet.FacetImpl = (*buttonGroupItem)(nil)

// NewButtonGroup constructs a selection.button_group mark with canonical defaults.
func NewButtonGroup(label string, options []ButtonGroupOption) *ButtonGroup {
	bg := &ButtonGroup{
		Label:        marks.Const(label),
		Mode:         marks.Const(ButtonGroupExclusive),
		Disabled:     marks.Const(false),
		hoveredIndex: -1,
		pressedIndex: -1,
		focusedIndex: -1,
		Value:        store.NewValueStore[[]string](nil),
	}
	bg.Core.Facet = facet.NewFacet()
	bg.AddBinding(bg.Label)
	bg.AddBinding(bg.Mode)
	bg.AddBinding(bg.Disabled)
	bg.SetOptions(options)

	bg.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearHorizontal,
		Policy:   buttonGroupPolicy{group: bg},
		Children: bg,
	}
	bg.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := bg.measureIntrinsic(ctx, constraints)
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
			Height: facet.StretchWhenParentRequests,
		},
		Baseline: facet.BaselineNone,
	}
	bg.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return bg.measure(ctx, constraints)
	}
	bg.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		bg.Layout.ArrangedBounds = bounds
		bg.arrange(ctx, bounds)
	}
	bg.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return bg.hitTest(p) }
	bg.Input.OnPointer = func(e facet.PointerEvent) bool { return bg.onPointer(e) }
	bg.Input.OnKey = func(e facet.KeyEvent) bool { return bg.onKey(e) }
	bg.Focus.Focusable = func() bool { return !bg.Disabled.Get() && len(bg.Options) > 0 }
	bg.Focus.TabIndex = 0
	bg.Focus.OnFocusGained = func() { bg.onFocusGained() }
	bg.Focus.OnFocusLost = func() { bg.onFocusLost() }
	bg.textRole.IMEEnabled = false

	bg.Render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := bg.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	bg.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return bg.buildCommands(bg.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	bg.RegisterRoles()
	bg.AddRole(&bg.textRole)
	bg.rebuildChildren()
	return bg
}

// Base satisfies facet.FacetImpl.
func (bg *ButtonGroup) Base() *facet.Facet {
	bg.Facet.BindImpl(bg)
	return &bg.Facet
}

// Descriptor satisfies marks.Mark.
func (bg *ButtonGroup) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "selection", TypeName: "button_group"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (bg *ButtonGroup) AccessibilityRole() string { return "group" }

// AccessibleName reports the semantic name source required by the spec.
func (bg *ButtonGroup) AccessibleName() string { return bg.Label.Get() }

// SetOptions updates the available buttons.
func (bg *ButtonGroup) SetOptions(options []ButtonGroupOption) {
	if bg == nil {
		return
	}
	next := append([]ButtonGroupOption(nil), options...)
	for i := range next {
		next[i].Key = strings.TrimSpace(next[i].Key)
		next[i].Label = strings.TrimSpace(next[i].Label)
	}
	bg.Options = next
	bg.rebuildChildren()
	bg.clampSelection()
	bg.syncChildState()
	bg.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetSelectedKeys updates the canonical selected keys.
func (bg *ButtonGroup) SetSelectedKeys(keys ...string) {
	if bg == nil {
		return
	}
	next := bg.normalizeSelection(keys)
	if bg.Value == nil {
		bg.Value = store.NewValueStore[[]string](next)
		bg.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return
	}
	if sameStringSlice(bg.Value.Get(), next) {
		return
	}
	bg.Value.Set(next)
	bg.syncChildState()
	bg.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// ExportAnchors publishes the button-group anchor set.
func (bg *ButtonGroup) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if bg == nil {
		return nil
	}
	bounds := bg.Layout.ArrangedBounds
	out := bg.Core.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	if len(bg.cachedOptionBounds) > 0 {
		idx := bg.focusedIndex
		if idx < 0 || idx >= len(bg.cachedOptionBounds) {
			idx = bg.selectedIndex()
		}
		if idx >= 0 && idx < len(bg.cachedOptionBounds) && !bg.cachedOptionBounds[idx].IsEmpty() {
			if idx < len(bg.cachedItems) && bg.cachedItems[idx] != nil && !bg.cachedItems[idx].cachedLabelBounds.IsEmpty() {
				out["baseline"] = gfx.Point{X: bg.cachedItems[idx].cachedLabelBounds.Min.X, Y: bg.cachedItems[idx].cachedLabelBounds.Min.Y}
			} else {
				out["baseline"] = gfx.Point{X: bg.cachedOptionBounds[idx].Min.X, Y: bg.cachedOptionBounds[idx].Min.Y}
			}
		} else {
			out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
		}
	} else {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	return out
}

// Children returns the facet's immediate child list.
func (bg *ButtonGroup) Children() []facet.GroupChild {
	if bg == nil {
		return nil
	}
	bg.rebuildChildren()
	out := make([]facet.GroupChild, 0, len(bg.cachedItems))
	for i := range bg.cachedItems {
		child := bg.cachedItems[i]
		if child == nil {
			continue
		}
		base := child.Base()
		layoutRole := base.LayoutRole()
		if layoutRole == nil {
			continue
		}
		out = append(out, facet.GroupChild{
			FacetID: base.ID(),
			MarkID:  buttonGroupMarkIDOptionButtons,
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

// OnAttach wires store invalidation for the bound selection store.
func (bg *ButtonGroup) OnAttach(ctx facet.AttachContext) {
	bg.Core.OnAttach()
	if bg.Value == nil {
		bg.Value = store.NewValueStore[[]string](nil)
	}
	facet.Store(facet.Subscribe(bg), &bg.Value.OnChange, bg.Value.Version, func(signal.Change[[]string]) {
		bg.syncChildState()
		bg.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	})
}

// OnActivate is unused.
func (bg *ButtonGroup) OnActivate() { bg.Core.OnActivate() }

// OnDeactivate is unused.
func (bg *ButtonGroup) OnDeactivate() { bg.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (bg *ButtonGroup) OnDetach() {
	bg.Core.OnDetach()
	bg.cachedTokens = theme.Tokens{}
	bg.cachedRecipe = shared.ButtonGroupSlots{}
	bg.cachedRootBounds = gfx.Rect{}
	bg.cachedGroupSurface = gfx.Rect{}
	bg.cachedOptionBounds = nil
	bg.cachedPadX = 0
	bg.cachedPadY = 0
	bg.cachedGap = 0
	bg.cachedRadius = 0
}

func (bg *ButtonGroup) invalidate(flags facet.DirtyFlags) {
	if bg == nil {
		return
	}
	bg.Base().Invalidate(flags)
}

func (bg *ButtonGroup) rebuildChildren() {
	if bg == nil {
		return
	}
	if len(bg.cachedItems) != len(bg.Options) {
		bg.cachedItems = make([]*buttonGroupItem, len(bg.Options))
	}
	for i := range bg.Options {
		if bg.cachedItems[i] == nil {
			bg.cachedItems[i] = newButtonGroupItem(bg, i, bg.Options[i])
		}
		child := bg.cachedItems[i]
		child.parent = bg
		child.index = i
		child.setLabel(bg.Options[i].Label)
		child.setIcon(bg.Options[i].Icon)
		child.setDisabled(bg.Disabled.Get() || bg.Options[i].Disabled)
		child.setSelected(bg.isSelectedKey(bg.Options[i].Key))
		child.cachedWritingDirection = bg.cachedWritingDirection
	}
}

func (bg *ButtonGroup) syncChildState() {
	if bg == nil {
		return
	}
	for i := range bg.cachedItems {
		child := bg.cachedItems[i]
		if child == nil || i >= len(bg.Options) {
			continue
		}
		child.setLabel(bg.Options[i].Label)
		child.setIcon(bg.Options[i].Icon)
		child.setDisabled(bg.Disabled.Get() || bg.Options[i].Disabled)
		child.setSelected(bg.isSelectedKey(bg.Options[i].Key))
		child.cachedWritingDirection = bg.cachedWritingDirection
	}
}

func (bg *ButtonGroup) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uiinput.ResolveButtonGroupRecipe(style)
	bg.cachedTokens = resolved.TokenSet()
	bg.cachedRecipe = slots
	bg.cachedWritingDirection = ctx.WritingDirection
	bg.cachedPadX = mathutil.Max(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	bg.cachedPadY = mathutil.Max(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	bg.cachedGap = 0
	bg.cachedRadius = float32(resolved.Radius(theme.RadiusM))
	bg.rebuildChildren()
	bg.syncChildState()

	contentW := float32(0)
	contentH := float32(0)
	bg.cachedOptionBounds = make([]gfx.Rect, len(bg.cachedItems))
	for i := range bg.cachedItems {
		child := bg.cachedItems[i]
		if child == nil {
			continue
		}
		size := child.Layout.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: mathutil.Max(0, constraints.MaxSize.W), H: constraints.MaxSize.H}})
		if size.Size.W > contentW {
			contentW = size.Size.W
		}
		if size.Size.H > contentH {
			contentH = size.Size.H
		}
	}
	if len(bg.cachedItems) > 0 {
		contentW = 0
		for i := range bg.cachedItems {
			if bg.cachedItems[i] == nil {
				continue
			}
			if i > 0 {
				contentW += bg.cachedGap
			}
			contentW += bg.cachedItems[i].Layout.MeasuredSize.W
			if bg.cachedItems[i].Layout.MeasuredSize.H > contentH {
				contentH = bg.cachedItems[i].Layout.MeasuredSize.H
			}
		}
	}
	if contentH <= 0 {
		contentH = resolved.Density.Scale(36)
	}
	size := gfx.Size{
		W: mathutil.Max(resolved.Density.Scale(120), contentW+bg.cachedPadX*2),
		H: mathutil.Max(resolved.Density.Scale(36), contentH+bg.cachedPadY*2),
	}
	if constraints.MaxSize.W > 0 {
		size.W = mathutil.Min(size.W, constraints.MaxSize.W)
	}
	if constraints.MaxSize.H > 0 {
		size.H = mathutil.Min(size.H, constraints.MaxSize.H)
	}
	bg.Layout.MeasuredSize = constraints.Constrain(size)
	bg.Layout.MeasuredResult = facet.MeasureResult{
		Size: bg.Layout.MeasuredSize,
		Intrinsic: facet.IntrinsicSize{
			Min:       bg.Layout.MeasuredSize,
			Preferred: bg.Layout.MeasuredSize,
			Max:       bg.Layout.MeasuredSize,
		},
		Constraints: constraints,
	}
	bg.textRole.Layout = nil
	return bg.Layout.MeasuredResult
}

func (bg *ButtonGroup) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return bg.measure(ctx, constraints).Size
}

func (bg *ButtonGroup) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	bg.cachedRootBounds = bounds
	bg.cachedGroupSurface = bounds
	bg.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		bg.cachedOptionBounds = nil
		return
	}
	bg.rebuildChildren()
	bg.syncChildState()
	inner := bounds.Inset(bg.cachedPadX, bg.cachedPadY)
	if inner.IsEmpty() {
		inner = bounds
	}
	children := bg.Children()
	if len(children) == 0 {
		bg.cachedOptionBounds = nil
		return
	}
	totalW := float32(0)
	maxH := float32(0)
	for i := range bg.cachedItems {
		if bg.cachedItems[i] == nil {
			continue
		}
		if i > 0 {
			totalW += bg.cachedGap
		}
		totalW += bg.cachedItems[i].Layout.MeasuredSize.W
		if h := bg.cachedItems[i].Layout.MeasuredSize.H; h > maxH {
			maxH = h
		}
	}
	if maxH <= 0 {
		maxH = inner.Height()
	}
	startX := inner.Min.X
	if bg.cachedWritingDirection == facet.WritingDirectionRTL {
		startX = inner.Max.X - totalW
	}
	arranged := make([]gfx.Rect, len(bg.cachedItems))
	curX := startX
	for i := range bg.cachedItems {
		child := bg.cachedItems[i]
		if child == nil {
			continue
		}
		w := child.Layout.MeasuredSize.W
		h := maxH
		y := inner.Min.Y + mathutil.Max(0, (inner.Height()-h)*0.5)
		if bg.cachedWritingDirection == facet.WritingDirectionRTL {
			rect := gfx.RectFromXYWH(curX, y, w, h)
			child.Layout.Arrange(facet.ArrangeContext{Runtime: ctx.Runtime, Theme: ctx.Theme, ParentGroup: child.Layout.Parent, ChildGroup: child.Layout.Child, Placement: facet.Placement{Mode: facet.PlacementLinear, Linear: facet.LinearPlacement{Order: i, CrossAxisAlign: facet.CrossAxisStretch}}}, rect)
			arranged[i] = rect
			curX += w + bg.cachedGap
		} else {
			rect := gfx.RectFromXYWH(curX, y, w, h)
			child.Layout.Arrange(facet.ArrangeContext{Runtime: ctx.Runtime, Theme: ctx.Theme, ParentGroup: child.Layout.Parent, ChildGroup: child.Layout.Child, Placement: facet.Placement{Mode: facet.PlacementLinear, Linear: facet.LinearPlacement{Order: i, CrossAxisAlign: facet.CrossAxisStretch}}}, rect)
			arranged[i] = rect
			curX += w + bg.cachedGap
		}
	}
	bg.cachedOptionBounds = arranged
}

func (bg *ButtonGroup) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.ButtonGroupSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: bg.cachedTokens}, bg.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, bg.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uiinput.ResolveButtonGroupRecipe(style)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: bg.cachedTokens}, bg.cachedRecipe
}

func (bg *ButtonGroup) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if bg == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := bg.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := bg.interactionState()
	root := slots.Root.Resolve(state, tokens)
	surface := slots.GroupSurface.Resolve(state, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)

	cmds := make([]gfx.Command, 0, 64)
	if !theme.IsTransparentMaterial(root) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(bounds), root)...)
	}
	if !theme.IsTransparentMaterial(surface) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(bounds, bg.cachedRadius), surface)...)
	}
	for i := range bg.cachedItems {
		if bg.cachedItems[i] == nil {
			continue
		}
		childCmds := bg.cachedItems[i].Projection.Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: bg.cachedOptionBounds[i], ContentScale: contentScale})
		if childCmds != nil {
			cmds = append(cmds, childCmds.Commands...)
		}
	}
	if bg.focusedVisible && bg.focusedIndex >= 0 && bg.focusedIndex < len(bg.cachedOptionBounds) && !theme.IsTransparentMaterial(focus) {
		rect := bg.cachedOptionBounds[bg.focusedIndex]
		inset := mathutil.Max(1, rect.Height()*0.08)
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(rect.Inset(-inset, -inset), bg.cachedRadius+inset), focus)...)
	}
	return cmds
}

func (bg *ButtonGroup) hitTest(p gfx.Point) facet.HitResult {
	if bg == nil || bg.Layout.ArrangedBounds.IsEmpty() || !bg.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := bg.cursorShape()
	if bg.focusedVisible && bg.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: buttonGroupMarkIDFocusRing, Cursor: cursor}
	}
	for i := range bg.cachedOptionBounds {
		if !bg.cachedOptionBounds[i].Contains(p) {
			continue
		}
		if bg.isSelectedKey(bg.keyAt(i)) {
			return facet.HitResult{Hit: true, MarkID: buttonGroupMarkIDSelectedIndicator, Cursor: cursor}
		}
		return facet.HitResult{Hit: true, MarkID: buttonGroupMarkIDOptionButtons, Cursor: cursor}
	}
	if bg.cachedGroupSurface.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: buttonGroupMarkIDGroupSurface, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: buttonGroupMarkIDRoot, Cursor: cursor}
}

func (bg *ButtonGroup) pointInFocusRing(p gfx.Point) bool {
	if bg.focusedIndex < 0 || bg.focusedIndex >= len(bg.cachedOptionBounds) {
		return false
	}
	rect := bg.cachedOptionBounds[bg.focusedIndex]
	if rect.IsEmpty() || !rect.Contains(p) {
		return false
	}
	inner := rect.Inset(mathutil.Max(1, rect.Height()*0.08), mathutil.Max(1, rect.Height()*0.08))
	if inner.IsEmpty() {
		return true
	}
	return !inner.Contains(p)
}

func (bg *ButtonGroup) cursorShape() facet.CursorShape {
	if bg.Disabled.Get() {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (bg *ButtonGroup) onPointer(e facet.PointerEvent) bool {
	if bg.Disabled.Get() {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		if idx := bg.indexAt(e.Position); idx >= 0 {
			bg.hoveredIndex = idx
			bg.invalidate(facet.DirtyProjection)
			return true
		}
		return false
	case platform.PointerLeave:
		bg.hoveredIndex = -1
		bg.pressedIndex = -1
		bg.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerMove:
		if idx := bg.indexAt(e.Position); idx >= 0 {
			bg.hoveredIndex = idx
			if bg.pressedIndex >= 0 {
				bg.focusedIndex = idx
			}
			bg.invalidate(facet.DirtyProjection)
			return true
		}
		return false
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		if idx := bg.indexAt(e.Position); idx >= 0 && !bg.isDisabledIndex(idx) {
			bg.hoveredIndex = idx
			bg.pressedIndex = idx
			bg.focusFromPointer = true
			bg.focusedVisible = false
			bg.focusedIndex = idx
			bg.invalidate(facet.DirtyProjection)
			return true
		}
		return false
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := bg.pressedIndex >= 0
		idx := bg.indexAt(e.Position)
		pressed := bg.pressedIndex
		bg.pressedIndex = -1
		bg.invalidate(facet.DirtyProjection)
		if wasPressed && idx >= 0 && idx == pressed {
			bg.activateIndex(idx)
			return true
		}
		return wasPressed
	default:
		return false
	}
}

func (bg *ButtonGroup) onKey(e facet.KeyEvent) bool {
	if bg.Disabled.Get() || len(bg.Options) == 0 {
		return false
	}
	switch e.Key {
	case platform.KeyLeft, platform.KeyUp:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			bg.moveFocus(-1)
			return true
		}
	case platform.KeyRight, platform.KeyDown:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			bg.moveFocus(1)
			return true
		}
	case platform.KeyHome:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			bg.setFocusIndex(0)
			return true
		}
	case platform.KeyEnd:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			bg.setFocusIndex(len(bg.Options) - 1)
			return true
		}
	case platform.KeySpace, platform.KeyEnter:
		switch e.Kind {
		case platform.KeyPress, platform.KeyRepeat:
			bg.pressedIndex = bg.focusIndexOrSelected()
			bg.invalidate(facet.DirtyProjection)
			return true
		case platform.KeyRelease:
			wasPressed := bg.pressedIndex >= 0
			bg.pressedIndex = -1
			bg.invalidate(facet.DirtyProjection)
			if wasPressed {
				bg.activateIndex(bg.focusIndexOrSelected())
			}
			return wasPressed
		}
	}
	return false
}

func (bg *ButtonGroup) indexAt(p gfx.Point) int {
	for i := range bg.cachedOptionBounds {
		if bg.cachedOptionBounds[i].Contains(p) {
			return i
		}
	}
	return -1
}

func (bg *ButtonGroup) onFocusGained() {
	bg.focusedVisible = !bg.focusFromPointer
	bg.focusFromPointer = false
	if bg.focusedIndex < 0 {
		bg.focusedIndex = bg.focusIndexOrSelected()
	}
	bg.invalidate(facet.DirtyProjection)
}

func (bg *ButtonGroup) onFocusLost() {
	bg.focusedVisible = false
	bg.pressedIndex = -1
	bg.focusFromPointer = false
	bg.invalidate(facet.DirtyProjection)
}

func (bg *ButtonGroup) interactionState() theme.InteractionState {
	switch {
	case bg.Disabled.Get():
		return theme.StateDisabled
	case bg.pressedIndex >= 0:
		return theme.StatePressed
	case bg.hoveredIndex >= 0:
		return theme.StateHover
	case bg.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (bg *ButtonGroup) optionState(idx int) theme.InteractionState {
	switch {
	case bg.Disabled.Get() || bg.isDisabledIndex(idx):
		return theme.StateDisabled
	case bg.pressedIndex == idx:
		return theme.StatePressed
	case bg.hoveredIndex == idx:
		return theme.StateHover
	case idx == bg.focusedIndex && bg.focusedVisible:
		return theme.StateFocused
	case bg.isSelectedIndex(idx):
		return theme.StateSelected
	default:
		return theme.StateDefault
	}
}

func (bg *ButtonGroup) selectedIndex() int {
	if bg == nil || len(bg.Options) == 0 || bg.Value == nil {
		return -1
	}
	selected := bg.normalizeSelection(bg.Value.Get())
	for _, key := range selected {
		if idx := bg.indexByKey(key); idx >= 0 {
			return idx
		}
	}
	return -1
}

func (bg *ButtonGroup) focusIndexOrSelected() int {
	if bg.focusedIndex >= 0 && bg.focusedIndex < len(bg.Options) {
		return bg.focusedIndex
	}
	if idx := bg.selectedIndex(); idx >= 0 {
		return idx
	}
	return 0
}

func (bg *ButtonGroup) setFocusIndex(idx int) {
	if idx < 0 || idx >= len(bg.Options) {
		return
	}
	bg.focusedIndex = idx
	bg.invalidate(facet.DirtyProjection)
}

func (bg *ButtonGroup) moveFocus(delta int) {
	if len(bg.Options) == 0 {
		return
	}
	idx := bg.focusIndexOrSelected() + delta
	if idx < 0 {
		idx = 0
	}
	if idx >= len(bg.Options) {
		idx = len(bg.Options) - 1
	}
	bg.focusedIndex = idx
	if bg.Mode.Get() == ButtonGroupExclusive {
		bg.activateIndex(idx)
		return
	}
	bg.invalidate(facet.DirtyProjection)
}

func (bg *ButtonGroup) activateIndex(idx int) {
	if idx < 0 || idx >= len(bg.Options) || bg.isDisabledIndex(idx) {
		return
	}
	key := bg.Options[idx].Key
	if key == "" {
		return
	}
	bg.focusedIndex = idx
	if bg.Mode.Get() == ButtonGroupMultiple {
		bg.toggleKey(key)
	} else {
		bg.SetSelectedKeys(key)
	}
	bg.Activated.Emit(key)
	bg.invalidate(facet.DirtyProjection)
}

func (bg *ButtonGroup) toggleKey(key string) {
	current := bg.normalizeSelection(bg.currentSelection())
	next := make([]string, 0, len(bg.Options))
	found := false
	for _, option := range bg.Options {
		if option.Key == "" || bg.isDisabledKey(option.Key) {
			continue
		}
		if option.Key == key {
			found = true
			continue
		}
		for _, selected := range current {
			if selected == option.Key {
				next = append(next, option.Key)
				break
			}
		}
	}
	if !found {
		next = append(next, key)
	}
	bg.SetSelectedKeys(next...)
}

func (bg *ButtonGroup) currentSelection() []string {
	if bg == nil || bg.Value == nil {
		return nil
	}
	return append([]string(nil), bg.Value.Get()...)
}

func (bg *ButtonGroup) normalizeSelection(keys []string) []string {
	if bg == nil || len(bg.Options) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(keys))
	out := make([]string, 0, len(keys))
	for _, option := range bg.Options {
		if option.Key == "" || bg.isDisabledKey(option.Key) {
			continue
		}
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key == option.Key {
				if _, ok := seen[key]; ok {
					break
				}
				seen[key] = struct{}{}
				out = append(out, key)
				break
			}
		}
	}
	if bg.Mode.Get() == ButtonGroupExclusive && len(out) > 1 {
		return out[:1]
	}
	return out
}

func (bg *ButtonGroup) clampSelection() {
	if bg == nil {
		return
	}
	if bg.Value == nil {
		return
	}
	bg.Value.Set(bg.normalizeSelection(bg.Value.Get()))
}

func (bg *ButtonGroup) isSelectedKey(key string) bool {
	if key == "" || bg == nil {
		return false
	}
	for _, selected := range bg.currentSelection() {
		if selected == key {
			return true
		}
	}
	return false
}

func (bg *ButtonGroup) isSelectedIndex(idx int) bool {
	if idx < 0 || idx >= len(bg.Options) {
		return false
	}
	return bg.isSelectedKey(bg.Options[idx].Key)
}

func (bg *ButtonGroup) isDisabledIndex(idx int) bool {
	if idx < 0 || idx >= len(bg.Options) {
		return true
	}
	return bg.Disabled.Get() || bg.Options[idx].Disabled
}

func (bg *ButtonGroup) isDisabledKey(key string) bool {
	if key == "" {
		return true
	}
	for i := range bg.Options {
		if bg.Options[i].Key == key {
			return bg.Disabled.Get() || bg.Options[i].Disabled
		}
	}
	return true
}

func (bg *ButtonGroup) indexByKey(key string) int {
	for i := range bg.Options {
		if bg.Options[i].Key == key {
			return i
		}
	}
	return -1
}

func (bg *ButtonGroup) keyAt(idx int) string {
	if idx < 0 || idx >= len(bg.Options) {
		return ""
	}
	return bg.Options[idx].Key
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func newButtonGroupItem(parent *ButtonGroup, index int, option ButtonGroupOption) *buttonGroupItem {
	it := &buttonGroupItem{
		parent: parent,
		index:  index,
		label:  strings.TrimSpace(option.Label),
		icon:   option.Icon,
	}
	it.Core.Facet = facet.NewFacet()
	it.labelMark = primitive.NewText(marks.Const(it.label))
	it.labelMark.Typography = marks.Const(theme.TextLabelM)
	it.labelMark.Overflow = marks.Const(primitive.TextOverflowTruncate)
	if it.icon != nil {
		it.iconMark = primitive.NewIcon(it.icon)
		it.iconMark.Decorative = marks.Const(true)
	}
	it.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearHorizontal,
		Policy:   buttonGroupItemPolicy{item: it},
		Children: it,
	}
	it.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := it.measure(ctx, constraints)
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
			Height: facet.StretchWhenParentRequests,
		},
		Baseline: facet.BaselineNone,
	}
	it.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		size := it.measure(ctx, constraints)
		return facet.MeasureResult{
			Size: size,
			Intrinsic: facet.IntrinsicSize{
				Min:       size,
				Preferred: size,
				Max:       size,
			},
			Constraints: constraints,
		}
	}
	it.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		it.Layout.ArrangedBounds = bounds
		it.arrange(ctx, bounds)
	}
	it.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return it.hitTest(p) }
	it.Input.OnPointer = func(e facet.PointerEvent) bool { return it.onPointer(e) }
	it.textRole.IMEEnabled = false

	it.Render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := it.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	it.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return it.buildCommands(it.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	it.RegisterRoles()
	it.AddRole(&it.textRole)
	return it
}

func (it *buttonGroupItem) Base() *facet.Facet {
	it.Facet.BindImpl(it)
	return &it.Facet
}

func (it *buttonGroupItem) OnAttach(ctx facet.AttachContext) { it.Core.OnAttach() }
func (it *buttonGroupItem) OnActivate()                      { it.Core.OnActivate() }
func (it *buttonGroupItem) OnDeactivate()                    { it.Core.OnDeactivate() }
func (it *buttonGroupItem) OnDetach() {
	it.Core.OnDetach()
	it.cachedTokens = theme.Tokens{}
	it.cachedRecipe = shared.ButtonGroupSlots{}
	it.cachedBounds = gfx.Rect{}
	it.cachedIconBounds = gfx.Rect{}
	it.cachedLabelBounds = gfx.Rect{}
	it.cachedRadius = 0
	it.cachedPadX = 0
	it.cachedPadY = 0
	it.cachedGap = 0
}

func (it *buttonGroupItem) setLabel(label string) {
	if it == nil || it.label == label {
		return
	}
	it.label = label
	if it.labelMark == nil {
		it.labelMark = primitive.NewText(marks.Const(label))
	} else {
		it.labelMark.Content = marks.Const(label)
		it.labelMark.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
	it.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func (it *buttonGroupItem) setIcon(icon primitive.IconSource) {
	if it == nil || it.icon == icon {
		return
	}
	it.icon = icon
	if icon == nil {
		it.iconMark = nil
	} else if it.iconMark == nil {
		it.iconMark = primitive.NewIcon(icon)
		it.iconMark.Decorative = marks.Const(true)
	} else {
		it.iconMark.Source = icon
	}
	it.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func (it *buttonGroupItem) setDisabled(disabled bool) {
	if it == nil || it.disabled == disabled {
		return
	}
	it.disabled = disabled
	it.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

func (it *buttonGroupItem) setSelected(selected bool) {
	if it == nil || it.selected == selected {
		return
	}
	it.selected = selected
	it.invalidate(facet.DirtyProjection)
}

func (it *buttonGroupItem) invalidate(flags facet.DirtyFlags) {
	if it == nil {
		return
	}
	it.Base().Invalidate(flags)
}

func (it *buttonGroupItem) Children() []facet.GroupChild {
	if it == nil {
		return nil
	}
	out := make([]facet.GroupChild, 0, 2)
	if it.cachedWritingDirection == facet.WritingDirectionRTL {
		if it.labelMark != nil {
			if child := groupChildForFacet(it.labelMark.Base(), 0); child.FacetID != 0 {
				out = append(out, child)
			}
		}
		if it.iconMark != nil {
			if child := groupChildForFacet(it.iconMark.Base(), 1); child.FacetID != 0 {
				out = append(out, child)
			}
		}
		return out
	}
	if it.iconMark != nil {
		if child := groupChildForFacet(it.iconMark.Base(), 0); child.FacetID != 0 {
			out = append(out, child)
		}
	}
	if it.labelMark != nil {
		if child := groupChildForFacet(it.labelMark.Base(), 1); child.FacetID != 0 {
			out = append(out, child)
		}
	}
	return out
}

func (it *buttonGroupItem) measure(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	if it == nil {
		return gfx.Size{}
	}
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uiinput.ResolveButtonGroupRecipe(style)
	it.cachedTokens = resolved.TokenSet()
	it.cachedRecipe = slots
	it.cachedWritingDirection = ctx.WritingDirection
	it.cachedPadX = mathutil.Max(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	it.cachedPadY = mathutil.Max(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	it.cachedGap = float32(resolved.Spacing(theme.SpacingXS))
	it.cachedRadius = float32(resolved.Radius(theme.RadiusM))
	it.syncChildVisuals(resolved, ctx.Runtime, constraints)

	iconSize := gfx.Size{}
	labelSize := gfx.Size{}
	if it.iconMark != nil {
		iconSize = it.iconMark.Base().LayoutRole().Measure(facet.MeasureContext{
			Runtime:          ctx.Runtime,
			Theme:            resolved,
			ContentScale:     ctx.ContentScale,
			Density:          ctx.Density,
			WritingDirection: ctx.WritingDirection,
		}, facet.Constraints{MaxSize: gfx.Size{W: resolved.Density.Scale(20), H: resolved.Density.Scale(20)}}).Size
	}
	maxTextWidth := constraints.MaxSize.W
	if maxTextWidth <= 0 {
		maxTextWidth = resolved.Density.Scale(180)
	}
	if it.iconMark != nil && iconSize.W > 0 {
		maxTextWidth -= iconSize.W + it.cachedGap
	}
	if maxTextWidth < 0 {
		maxTextWidth = 0
	}
	if it.labelMark != nil {
		it.labelMark.MaxWidth = marks.Const(maxTextWidth)
		labelSize = it.labelMark.Base().LayoutRole().Measure(facet.MeasureContext{
			Runtime:          ctx.Runtime,
			Theme:            resolved,
			ContentScale:     ctx.ContentScale,
			Density:          ctx.Density,
			WritingDirection: ctx.WritingDirection,
		}, facet.Constraints{MaxSize: gfx.Size{W: maxTextWidth, H: constraints.MaxSize.H}}).Size
	}
	content := layout.InlineFlowSize([]gfx.Size{iconSize, labelSize}, it.cachedGap)
	width := content.W + it.cachedPadX*2
	height := content.H + it.cachedPadY*2
	minHeight := resolved.Density.Scale(36)
	if height < minHeight {
		height = minHeight
	}
	if width < resolved.Density.Scale(48) {
		width = resolved.Density.Scale(48)
	}
	size := gfx.Size{W: width, H: height}
	it.Layout.MeasuredSize = size
	it.Layout.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	return size
}

func (it *buttonGroupItem) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return it.measure(ctx, constraints)
}

func (it *buttonGroupItem) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	it.cachedBounds = bounds
	it.cachedIconBounds = gfx.Rect{}
	it.cachedLabelBounds = gfx.Rect{}
	it.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	inner := bounds.Inset(it.cachedPadX, it.cachedPadY)
	if inner.IsEmpty() {
		inner = bounds
	}
	iconSize := gfx.Size{}
	if it.iconMark != nil {
		iconSize = it.iconMark.Base().LayoutRole().MeasuredSize
	}
	labelSize := gfx.Size{}
	if it.labelMark != nil {
		labelSize = it.labelMark.Base().LayoutRole().MeasuredSize
	}
	rects := layout.ArrangeInlineFlow(inner, it.cachedPadX, it.cachedGap, []gfx.Size{iconSize, labelSize}, it.cachedWritingDirection == facet.WritingDirectionRTL)
	if it.iconMark != nil {
		it.cachedIconBounds = rects[0]
		it.iconMark.Base().LayoutRole().Arrange(ctx, it.cachedIconBounds)
	}
	if it.labelMark != nil {
		it.cachedLabelBounds = rects[1]
		it.labelMark.Base().LayoutRole().Arrange(ctx, it.cachedLabelBounds)
	}
}

func (it *buttonGroupItem) syncChildVisuals(resolved theme.ResolvedContext, runtime any, constraints facet.Constraints) {
	if it == nil {
		return
	}
	labelToken := theme.ColorText
	iconToken := theme.ColorTextSecondary
	switch {
	case it.disabled:
		labelToken = theme.ColorTextDisabled
		iconToken = theme.ColorTextDisabled
	case it.selected:
		labelToken = theme.ColorOnPrimary
		iconToken = theme.ColorOnPrimary
	}
	if it.labelMark != nil {
		it.labelMark.Content = marks.Const(it.label)
		it.labelMark.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
		it.labelMark.Typography = marks.Const(theme.TextLabelM)
		it.labelMark.Foreground = marks.Const(labelToken)
		it.labelMark.Disabled = marks.Const(it.disabled)
		it.labelMark.Overflow = marks.Const(primitive.TextOverflowTruncate)
		it.labelMark.MaxWidth = marks.Const(mathutil.Max(0, constraints.MaxSize.W))
	}
	if it.iconMark != nil {
		if it.iconMark.Source == nil && it.icon != nil {
			it.iconMark.Source = it.icon
		}
		it.iconMark.DensityBehavior = marks.Const(primitive.IconDensityScaleWithDensity)
		it.iconMark.Size = marks.Const(resolved.Density.Scale(16))
		it.iconMark.ColorSlot = marks.Const(iconToken)
		it.iconMark.Decorative = marks.Const(true)
	}
}

func (it *buttonGroupItem) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.ButtonGroupSlots) {
	if it == nil {
		return theme.StyleContext{}, shared.ButtonGroupSlots{}
	}
	if runtime == nil {
		return theme.StyleContext{Tokens: it.cachedTokens}, it.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, it.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uiinput.ResolveButtonGroupRecipe(style)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: it.cachedTokens}, it.cachedRecipe
}

func (it *buttonGroupItem) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if it == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := it.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := it.interactionState()
	option := slots.OptionButtons.Resolve(state, tokens)
	selected := slots.SelectedIndicator.Resolve(theme.StateSelected, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)

	path := buttonGroupItemPath(bounds, it.cachedRadius, it.index, len(it.parent.cachedItems), it.parent.cachedWritingDirection == facet.WritingDirectionRTL)
	cmds := make([]gfx.Command, 0, 16)
	if !theme.IsTransparentMaterial(option) {
		cmds = append(cmds, theme.MaterialCommands(path, option)...)
	}
	if it.selected && !theme.IsTransparentMaterial(selected) {
		cmds = append(cmds, theme.MaterialCommands(path, selected)...)
	}
	if it.iconMark != nil {
		if iconCmds := it.iconMark.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: it.cachedIconBounds, ContentScale: contentScale}); iconCmds != nil {
			cmds = append(cmds, iconCmds.Commands...)
		}
	}
	if it.labelMark != nil {
		if labelCmds := it.labelMark.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: it.cachedLabelBounds, ContentScale: contentScale}); labelCmds != nil {
			cmds = append(cmds, labelCmds.Commands...)
		}
	}
	if it.parent != nil && it.parent.focusedVisible && it.parent.focusedIndex == it.index && !theme.IsTransparentMaterial(focus) {
		inset := mathutil.Max(1, bounds.Height()*0.08)
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(bounds.Inset(-inset, -inset), it.cachedRadius+inset), focus)...)
	}
	return cmds
}

func (it *buttonGroupItem) hitTest(p gfx.Point) facet.HitResult {
	if it == nil || it.Layout.ArrangedBounds.IsEmpty() || !it.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := it.cursorShape()
	if it.parent != nil && it.parent.focusedVisible && it.parent.focusedIndex == it.index && it.parent.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: buttonGroupMarkIDFocusRing, Cursor: cursor}
	}
	if it.selected {
		return facet.HitResult{Hit: true, MarkID: buttonGroupMarkIDSelectedIndicator, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: buttonGroupMarkIDOptionButtons, Cursor: cursor}
}

func (it *buttonGroupItem) cursorShape() facet.CursorShape {
	if it == nil || it.disabled || (it.parent != nil && it.parent.Disabled.Get()) {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (it *buttonGroupItem) onPointer(e facet.PointerEvent) bool {
	if it == nil || it.parent == nil || it.disabled || it.parent.Disabled.Get() {
		return false
	}
	return it.parent.onChildPointer(it.index, e)
}

func (it *buttonGroupItem) interactionState() theme.InteractionState {
	if it == nil {
		return theme.StateDefault
	}
	switch {
	case it.disabled || (it.parent != nil && it.parent.Disabled.Get()):
		return theme.StateDisabled
	case it.parent != nil && it.parent.pressedIndex == it.index:
		return theme.StatePressed
	case it.parent != nil && it.parent.hoveredIndex == it.index:
		return theme.StateHover
	case it.parent != nil && it.parent.focusedVisible && it.parent.focusedIndex == it.index:
		return theme.StateFocused
	case it.selected:
		return theme.StateSelected
	default:
		return theme.StateDefault
	}
}

func groupChildForFacet(base *facet.Facet, order int) facet.GroupChild {
	if base == nil {
		return facet.GroupChild{}
	}
	layoutRole := base.LayoutRole()
	if layoutRole == nil {
		return facet.GroupChild{}
	}
	return facet.GroupChild{
		FacetID: base.ID(),
		MarkID:  buttonGroupMarkIDOptionButtons,
		Attachment: facet.Attachment{
			Placement: facet.Placement{
				Mode:   facet.PlacementLinear,
				Linear: facet.LinearPlacement{Order: order, CrossAxisAlign: facet.CrossAxisStretch},
			},
		},
		Layout:   layoutRole,
		Contract: layoutRole.Child,
	}
}

func (bg *ButtonGroup) onChildPointer(index int, e facet.PointerEvent) bool {
	if bg == nil || bg.Disabled.Get() || index < 0 || index >= len(bg.Options) || bg.isDisabledIndex(index) {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		bg.hoveredIndex = index
		bg.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		bg.hoveredIndex = -1
		if bg.pressedIndex < 0 {
			bg.focusFromPointer = false
		}
		bg.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerMove:
		bg.hoveredIndex = index
		if bg.pressedIndex >= 0 {
			bg.focusedIndex = index
		}
		bg.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		bg.hoveredIndex = index
		bg.pressedIndex = index
		bg.focusFromPointer = true
		bg.focusedVisible = false
		bg.focusedIndex = index
		bg.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := bg.pressedIndex >= 0
		pressed := bg.pressedIndex
		bg.pressedIndex = -1
		bg.invalidate(facet.DirtyProjection)
		if wasPressed && pressed == index {
			bg.activateIndex(index)
			return true
		}
		return wasPressed
	default:
		return false
	}
}

type buttonGroupItemPolicy struct {
	item *buttonGroupItem
}

func (buttonGroupItemPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }

func (p buttonGroupItemPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.item == nil {
		return facet.GroupMeasureResult{}, nil
	}
	size := p.item.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}})
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p buttonGroupItemPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.item == nil {
		return nil, nil
	}
	p.item.arrange(ctx.ArrangeContext, ctx.Bounds)
	arranged := make([]facet.ArrangedGroupChild, 0, len(children))
	for i := range children {
		child := children[i]
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

type buttonGroupPolicy struct {
	group *ButtonGroup
}

func (buttonGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }

func (p buttonGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.group == nil {
		return facet.GroupMeasureResult{}, nil
	}
	if len(children) == 0 {
		return facet.GroupMeasureResult{}, nil
	}
	main := float32(0)
	cross := float32(0)
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		size := child.Layout.Measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}}).Size
		main += size.W
		if size.H > cross {
			cross = size.H
		}
	}
	if len(children) > 1 {
		main += p.group.cachedGap * float32(len(children)-1)
	}
	return facet.GroupMeasureResult{Size: gfx.Size{W: main + p.group.cachedPadX*2, H: cross + p.group.cachedPadY*2}}, nil
}

func (p buttonGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.group == nil {
		return nil, nil
	}
	if len(children) == 0 {
		return nil, nil
	}
	ordered := make([]int, 0, len(children))
	for i := range children {
		ordered = append(ordered, i)
	}
	start := ctx.Bounds.Min.X + p.group.cachedPadX
	totalW := float32(0)
	for i := range children {
		if children[i].Layout == nil {
			continue
		}
		totalW += children[i].Layout.MeasuredSize.W
	}
	if len(children) > 1 {
		totalW += p.group.cachedGap * float32(len(children)-1)
	}
	if p.group.cachedWritingDirection == facet.WritingDirectionRTL {
		start = ctx.Bounds.Max.X - p.group.cachedPadX - totalW
	}
	y := ctx.Bounds.Min.Y + p.group.cachedPadY
	height := mathutil.Max(0, ctx.Bounds.Height()-p.group.cachedPadY*2)
	if height <= 0 {
		height = ctx.Bounds.Height()
	}
	rects := make([]gfx.Rect, len(children))
	curX := start
	for _, idx := range ordered {
		child := children[idx]
		if child.Layout == nil {
			continue
		}
		size := child.Layout.MeasuredSize
		rect := gfx.RectFromXYWH(curX, y, size.W, height)
		child.Layout.Arrange(facet.ArrangeContext{
			Runtime:     ctx.Runtime,
			Theme:       ctx.Theme,
			ParentGroup: ctx.ParentGroup,
			ChildGroup:  child.Contract,
			Placement: facet.Placement{
				Mode: facet.PlacementLinear,
				Linear: facet.LinearPlacement{
					Order:          idx,
					CrossAxisAlign: facet.CrossAxisStretch,
				},
			},
		}, rect)
		rects[idx] = rect
		curX += size.W + p.group.cachedGap
	}
	arranged := make([]facet.ArrangedGroupChild, 0, len(children))
	for i := range children {
		arranged = append(arranged, facet.ArrangedGroupChild{
			FacetID:   children[i].FacetID,
			MarkID:    children[i].MarkID,
			Bounds:    rects[i],
			Placement: children[i].Attachment.Placement,
			ZPriority: children[i].Attachment.ZPriority,
			Contract:  children[i].Contract,
		})
	}
	return arranged, nil
}

func buttonGroupItemPath(bounds gfx.Rect, radius float32, index, total int, rtl bool) gfx.Path {
	if bounds.IsEmpty() {
		return gfx.RectPath(bounds)
	}
	if total <= 1 {
		return gfx.RoundedRectPath(bounds, radius)
	}
	if index == 0 {
		if rtl {
			return rightRoundedRectPath(bounds, radius)
		}
		return leftRoundedRectPath(bounds, radius)
	}
	if index == total-1 {
		if rtl {
			return leftRoundedRectPath(bounds, radius)
		}
		return rightRoundedRectPath(bounds, radius)
	}
	return gfx.RectPath(bounds)
}

func leftRoundedRectPath(r gfx.Rect, radius float32) gfx.Path {
	if r.IsEmpty() {
		return gfx.Path{}
	}
	maxRadius := float32(math.Min(float64(r.Width()), float64(r.Height()))) / 2
	if radius <= 0 {
		return gfx.RectPath(r)
	}
	if radius > maxRadius {
		radius = maxRadius
	}
	minX, minY := r.Min.X, r.Min.Y
	maxX, maxY := r.Max.X, r.Max.Y
	rx := radius
	ry := radius

	return gfx.NewPath().
		MoveTo(gfx.Point{X: minX + rx, Y: minY}).
		LineTo(gfx.Point{X: maxX, Y: minY}).
		LineTo(gfx.Point{X: maxX, Y: maxY}).
		LineTo(gfx.Point{X: minX + rx, Y: maxY}).
		QuadTo(gfx.Point{X: minX, Y: maxY}, gfx.Point{X: minX, Y: maxY - ry}).
		LineTo(gfx.Point{X: minX, Y: minY + ry}).
		QuadTo(gfx.Point{X: minX, Y: minY}, gfx.Point{X: minX + rx, Y: minY}).
		Close().
		Build()
}

func rightRoundedRectPath(r gfx.Rect, radius float32) gfx.Path {
	if r.IsEmpty() {
		return gfx.Path{}
	}
	maxRadius := float32(math.Min(float64(r.Width()), float64(r.Height()))) / 2
	if radius <= 0 {
		return gfx.RectPath(r)
	}
	if radius > maxRadius {
		radius = maxRadius
	}
	minX, minY := r.Min.X, r.Min.Y
	maxX, maxY := r.Max.X, r.Max.Y
	rx := radius
	ry := radius

	return gfx.NewPath().
		MoveTo(gfx.Point{X: minX, Y: minY}).
		LineTo(gfx.Point{X: maxX - rx, Y: minY}).
		QuadTo(gfx.Point{X: maxX, Y: minY}, gfx.Point{X: maxX, Y: minY + ry}).
		LineTo(gfx.Point{X: maxX, Y: maxY - ry}).
		QuadTo(gfx.Point{X: maxX, Y: maxY}, gfx.Point{X: maxX - rx, Y: maxY}).
		LineTo(gfx.Point{X: minX, Y: maxY}).
		Close().
		Build()
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
