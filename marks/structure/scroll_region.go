package structure

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uistruct"
)

const (
	scrollRegionMarkIDRoot                        facet.MarkID = 1
	scrollRegionMarkIDViewport                    facet.MarkID = 2
	scrollRegionMarkIDContent                     facet.MarkID = 3
	scrollRegionMarkIDScrollbarVerticalOptional   facet.MarkID = 4
	scrollRegionMarkIDScrollbarHorizontalOptional facet.MarkID = 5
	scrollRegionMarkIDScrollShadowsOptional       facet.MarkID = 6
	scrollRegionMarkIDFocusRing                   facet.MarkID = 7
)

// ScrollDirection selects the primary content flow direction.
type ScrollDirection uint8

const (
	ScrollDirectionVertical ScrollDirection = iota
	ScrollDirectionHorizontal
)

// ScrollRegionChild describes one scroll-region child facet.
type ScrollRegionChild struct {
	Facet     facet.FacetImpl
	MarkID    facet.MarkID
	Placement facet.Placement
	ZPriority int32
}

// ScrollRegion implements the structure.scroll_region canonical mark.
type ScrollRegion struct {
	marks.Core

	Label       marks.Binding[string]
	Disabled    marks.Binding[bool]
	Direction   marks.Binding[ScrollDirection]
	Gap         marks.Binding[float32]
	ScrollToEnd marks.Binding[bool]

	children []ScrollRegionChild

	textRole facet.TextRole

	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool
	dragging         bool
	draggingAxis     ScrollDirection

	scrollOffset gfx.Point

	cachedTokens           theme.Tokens
	cachedRecipe           shared.ScrollRegionSlots
	cachedBounds           gfx.Rect
	cachedViewportBounds   gfx.Rect
	cachedContentBounds    gfx.Rect
	cachedContentSize      gfx.Size
	cachedVerticalTrack    gfx.Rect
	cachedVerticalThumb    gfx.Rect
	cachedHorizontalTrack  gfx.Rect
	cachedHorizontalThumb  gfx.Rect
	cachedFocusBounds      gfx.Rect
	cachedChildBounds      map[facet.FacetID]gfx.Rect
	cachedChildOrder       []facet.FacetID
	cachedWritingDirection facet.WritingDirection
}

var _ facet.FacetImpl = (*ScrollRegion)(nil)
var _ layout.AnchorExporter = (*ScrollRegion)(nil)
var _ marks.Mark = (*ScrollRegion)(nil)

// NewScrollRegion constructs a structure.scroll_region mark with canonical defaults.
func NewScrollRegion(label string) *ScrollRegion {
	sr := &ScrollRegion{
		Label:             marks.Const(label),
		Disabled:          marks.Const(false),
		Direction:         marks.Const(ScrollDirectionVertical),
		Gap:               marks.Const(float32(0)),
		ScrollToEnd:       marks.Const(false),
		cachedChildBounds: make(map[facet.FacetID]gfx.Rect),
	}
	sr.Core.Facet = facet.NewFacet()
	sr.AddBinding(sr.Label)
	sr.AddBinding(sr.Disabled)
	sr.AddBinding(sr.Direction)
	sr.AddBinding(sr.Gap)
	sr.AddBinding(sr.ScrollToEnd)

	sr.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   scrollRegionGroupPolicy{region: sr},
		Children: sr,
		Overflow: facet.OverflowScroll,
		Clipping: facet.GroupClipBounds,
	}
	sr.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := sr.measure(ctx, constraints).Size
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
	sr.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return sr.measure(ctx, constraints)
	}
	sr.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		sr.Layout.ArrangedBounds = bounds
		sr.arrange(ctx, bounds)
	}
	sr.Render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := sr.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	sr.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return sr.buildCommands(sr.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	sr.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return sr.hitTest(p) }
	sr.Input.OnPointer = func(e facet.PointerEvent) bool { return sr.onPointer(e) }
	sr.Input.OnScroll = func(e facet.ScrollEvent) bool { return sr.onScroll(e) }
	sr.Input.OnKey = func(e facet.KeyEvent) bool { return sr.onKey(e) }
	sr.Focus.Focusable = func() bool { return !sr.Disabled.Get() }
	sr.Focus.TabIndex = 0
	sr.Focus.OnFocusGained = func() { sr.onFocusGained() }
	sr.Focus.OnFocusLost = func() { sr.onFocusLost() }
	sr.textRole.IMEEnabled = false
	sr.RegisterRoles()
	sr.AddRole(&sr.Viewport)
	sr.AddRole(&sr.textRole)
	sr.updateParentKind()
	return sr
}

// Base satisfies facet.FacetImpl.
func (sr *ScrollRegion) Base() *facet.Facet {
	sr.Facet.BindImpl(sr)
	return &sr.Facet
}

// Descriptor satisfies marks.Mark.
func (sr *ScrollRegion) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "structure", TypeName: "scroll_region"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (sr *ScrollRegion) AccessibilityRole() string { return "region" }

// AccessibleName reports the semantic name source required by the spec.
func (sr *ScrollRegion) AccessibleName() string { return strings.TrimSpace(sr.Label.Get()) }

// ExportAnchors publishes the scroll-region anchor set.
func (sr *ScrollRegion) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if sr == nil {
		return nil
	}
	bounds := sr.Layout.ArrangedBounds
	out := sr.Core.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	out["baseline"] = bounds.Min
	if !sr.cachedViewportBounds.IsEmpty() {
		out["viewport"] = rectCenter(sr.cachedViewportBounds)
	}
	if !sr.cachedContentBounds.IsEmpty() {
		out["content"] = rectCenter(sr.cachedContentBounds)
	}
	return out
}

// SetChildren replaces the scrollable content children and invalidates layout.
func (sr *ScrollRegion) SetChildren(children []ScrollRegionChild) {
	next := append([]ScrollRegionChild(nil), children...)
	sr.children = next
	sr.Invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// Children returns the facet's immediate child list.
func (sr *ScrollRegion) Children() []facet.GroupChild {
	if sr == nil {
		return nil
	}
	out := make([]facet.GroupChild, 0, len(sr.children))
	for i := range sr.children {
		if child := sr.groupChild(sr.children[i]); child.Layout != nil {
			out = append(out, child)
		}
	}
	return out
}

func (sr *ScrollRegion) OnAttach(ctx facet.AttachContext) { sr.Core.OnAttach() }
func (sr *ScrollRegion) OnActivate()                      { sr.Core.OnActivate() }
func (sr *ScrollRegion) OnDeactivate()                    { sr.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (sr *ScrollRegion) OnDetach() {
	sr.Core.OnDetach()
	sr.cachedTokens = theme.Tokens{}
	sr.cachedRecipe = shared.ScrollRegionSlots{}
	sr.cachedBounds = gfx.Rect{}
	sr.cachedViewportBounds = gfx.Rect{}
	sr.cachedContentBounds = gfx.Rect{}
	sr.cachedContentSize = gfx.Size{}
	sr.cachedVerticalTrack = gfx.Rect{}
	sr.cachedVerticalThumb = gfx.Rect{}
	sr.cachedHorizontalTrack = gfx.Rect{}
	sr.cachedHorizontalThumb = gfx.Rect{}
	sr.cachedFocusBounds = gfx.Rect{}
	sr.cachedChildBounds = nil
	sr.cachedChildOrder = nil
	sr.scrollOffset = gfx.Point{}
	sr.dragging = false
	sr.draggingAxis = ScrollDirectionVertical
}

func (sr *ScrollRegion) invalidate(flags facet.DirtyFlags) {
	if sr == nil {
		return
	}
	sr.Facet.Invalidate(flags)
}

func (sr *ScrollRegion) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uistruct.ResolveScrollRegionRecipe(style)
	sr.cachedTokens = resolved.TokenSet()
	sr.cachedRecipe = slots
	sr.cachedWritingDirection = ctx.WritingDirection
	sr.updateParentKind()
	sr.syncContentChildren()
	children := sr.Children()
	if len(children) == 0 {
		size := constraints.Constrain(gfx.Size{W: 0, H: 0})
		sr.Layout.MeasuredSize = size
		sr.Layout.MeasuredResult = facet.MeasureResult{Size: size, Intrinsic: facet.IntrinsicSize{Min: size, Preferred: size, Max: size}, Constraints: constraints}
		return sr.Layout.MeasuredResult
	}
	childMeasureCtx := facet.MeasureContext{
		Runtime:          ctx.Runtime,
		Theme:            ctx.Theme,
		ContentScale:     ctx.ContentScale,
		Density:          ctx.Density,
		WritingDirection: ctx.WritingDirection,
	}
	childConstraints := facet.Constraints{MaxSize: constraints.MaxSize}
	if sr.Direction.Get() == ScrollDirectionHorizontal {
		childConstraints = childConstraints.WithMaxWidth(0)
	} else {
		childConstraints = childConstraints.WithMaxHeight(0)
	}
	contentW := float32(0)
	contentH := float32(0)
	for i := range children {
		if children[i].Layout == nil {
			continue
		}
		size := children[i].Layout.Measure(childMeasureCtx, childConstraints).Size
		if sr.Direction.Get() == ScrollDirectionHorizontal {
			contentW += size.W
			if size.H > contentH {
				contentH = size.H
			}
		} else {
			contentH += size.H
			if size.W > contentW {
				contentW = size.W
			}
		}
		if i < len(children)-1 {
			if sr.Direction.Get() == ScrollDirectionHorizontal {
				contentW += sr.Gap.Get()
			} else {
				contentH += sr.Gap.Get()
			}
		}
	}
	size := constraints.Constrain(gfx.Size{W: contentW, H: contentH})
	sr.cachedContentSize = gfx.Size{W: contentW, H: contentH}
	sr.Layout.MeasuredSize = size
	sr.Layout.MeasuredResult = facet.MeasureResult{
		Size:        size,
		Intrinsic:   facet.IntrinsicSize{Min: size, Preferred: size, Max: size},
		Constraints: constraints,
	}
	return sr.Layout.MeasuredResult
}

func (sr *ScrollRegion) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	sr.cachedBounds = bounds
	sr.cachedViewportBounds = bounds
	sr.cachedContentBounds = gfx.Rect{}
	sr.cachedVerticalTrack = gfx.Rect{}
	sr.cachedVerticalThumb = gfx.Rect{}
	sr.cachedHorizontalTrack = gfx.Rect{}
	sr.cachedHorizontalThumb = gfx.Rect{}
	sr.cachedFocusBounds = bounds.Inset(maxFloat(1, bounds.Width()*0.04), maxFloat(1, bounds.Height()*0.04))
	sr.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	sr.syncContentChildren()
	children := sr.Children()
	if len(children) == 0 {
		return
	}
	childBounds := make(map[facet.FacetID]gfx.Rect, len(children))
	order := make([]facet.FacetID, 0, len(children))
	cursorX := bounds.Min.X - sr.scrollOffset.X
	cursorY := bounds.Min.Y - sr.scrollOffset.Y
	for _, child := range children {
		if child.Layout == nil {
			continue
		}
		measured := child.Layout.MeasuredSize
		if measured == (gfx.Size{}) {
			measured = child.Layout.MeasuredResult.Size
		}
		childRect := gfx.RectFromXYWH(bounds.Min.X, cursorY, bounds.Width(), measured.H)
		if sr.Direction.Get() == ScrollDirectionHorizontal {
			childRect = gfx.RectFromXYWH(cursorX, bounds.Min.Y, measured.W, bounds.Height())
		}
		placement := child.Attachment.Placement
		if !child.Contract.SupportedPlacement.Has(placement.Mode) {
			switch {
			case child.Contract.SupportedPlacement.Has(facet.PlacementLinear):
				placement.Mode = facet.PlacementLinear
			case child.Contract.SupportedPlacement.Has(facet.PlacementFree):
				placement.Mode = facet.PlacementFree
			case child.Contract.SupportedPlacement.Has(facet.PlacementAnchor):
				placement.Mode = facet.PlacementAnchor
			case child.Contract.SupportedPlacement.Has(facet.PlacementGrid):
				placement.Mode = facet.PlacementGrid
			}
		}
		child.Layout.Arrange(facet.ArrangeContext{
			Runtime:     ctx.Runtime,
			Theme:       ctx.Theme,
			ParentGroup: child.Layout.Parent,
			ChildGroup:  child.Layout.Child,
			Placement:   placement,
		}, childRect)
		childBounds[child.FacetID] = childRect
		order = append(order, child.FacetID)
		if sr.Direction.Get() == ScrollDirectionHorizontal {
			cursorX += measured.W + sr.Gap.Get()
		} else {
			cursorY += measured.H + sr.Gap.Get()
		}
	}
	sr.cachedChildBounds = childBounds
	sr.cachedChildOrder = order
	if len(order) > 0 {
		first := childBounds[order[0]]
		minX, minY := first.Min.X, first.Min.Y
		maxX, maxY := first.Max.X, first.Max.Y
		for i := 1; i < len(order); i++ {
			b := childBounds[order[i]]
			if b.Min.X < minX {
				minX = b.Min.X
			}
			if b.Min.Y < minY {
				minY = b.Min.Y
			}
			if b.Max.X > maxX {
				maxX = b.Max.X
			}
			if b.Max.Y > maxY {
				maxY = b.Max.Y
			}
		}
		sr.cachedContentBounds = gfx.RectFromXYWH(minX, minY, maxX-minX, maxY-minY)
	}
	sr.updateScrollBounds(bounds)
	sr.Viewport.WorldBounds = bounds
}

func (sr *ScrollRegion) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if sr == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := sr.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := theme.StateDefault
	if sr.Disabled.Get() {
		state = theme.StateDisabled
	}
	root := slots.Root.Resolve(state, tokens)
	viewport := slots.Viewport.Resolve(state, tokens)
	content := slots.Content.Resolve(state, tokens)
	vertical := slots.ScrollbarVerticalOptional.Resolve(state, tokens)
	horizontal := slots.ScrollbarHorizontalOptional.Resolve(state, tokens)
	shadows := slots.ScrollShadowsOptional.Resolve(state, tokens)
	focus := slots.FocusRingOptional.Resolve(theme.StateFocused, tokens)

	cmds := make([]gfx.Command, 0, 64)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(viewport) {
		cmds = append(cmds, materialCommands(gfx.RectPath(sr.cachedViewportBounds), viewport)...)
	}
	if !isTransparentMaterial(content) && !sr.cachedContentBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.RectPath(sr.cachedContentBounds), content)...)
	}
	if !sr.cachedViewportBounds.IsEmpty() {
		cmds = append(cmds, gfx.PushClipRect{Rect: sr.cachedViewportBounds})
		cmds = append(cmds, gfx.PushTransform{Matrix: gfx.Translation(-sr.scrollOffset.X, -sr.scrollOffset.Y)})
		for _, child := range sr.children {
			if child.Facet == nil {
				continue
			}
			childImpl := child.Facet
			childBase := childImpl.Base()
			if childBase == nil || childBase.LayoutRole() == nil {
				continue
			}
			childBounds := sr.childBoundsForProjection(childBase.ID())
			if childBounds.IsEmpty() {
				continue
			}
			if projected := childBase.ProjectionRole().Project(facet.ProjectionContext{
				Runtime:      runtimeServicesOrNil(runtime),
				Bounds:       childBounds,
				ContentScale: contentScale,
			}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		cmds = append(cmds, gfx.PopTransform{})
		cmds = append(cmds, gfx.PopClip{})
	}
	if !isTransparentMaterial(shadows) {
		if !sr.cachedVerticalThumb.IsEmpty() {
			shadowRect := gfx.RectFromXYWH(sr.cachedVerticalThumb.Min.X, sr.cachedVerticalThumb.Min.Y, sr.cachedVerticalThumb.Width(), sr.cachedVerticalThumb.Height())
			cmds = append(cmds, materialCommands(gfx.RectPath(shadowRect), shadows)...)
		}
	}
	if !sr.cachedVerticalTrack.IsEmpty() {
		cmds = append(cmds, sr.barCommands(sr.cachedVerticalTrack, vertical, 0.24)...)
	}
	if !sr.cachedVerticalThumb.IsEmpty() {
		cmds = append(cmds, sr.barCommands(sr.cachedVerticalThumb, vertical, 1)...)
	}
	if !sr.cachedHorizontalTrack.IsEmpty() {
		cmds = append(cmds, sr.barCommands(sr.cachedHorizontalTrack, horizontal, 0.24)...)
	}
	if !sr.cachedHorizontalThumb.IsEmpty() {
		cmds = append(cmds, sr.barCommands(sr.cachedHorizontalThumb, horizontal, 1)...)
	}
	if sr.focusedVisible && !isTransparentMaterial(focus) {
		cmds = append(cmds, materialCommands(gfx.RectPath(sr.cachedFocusBounds), focus)...)
	}
	return cmds
}

func (sr *ScrollRegion) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.ScrollRegionSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: sr.cachedTokens}, sr.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, sr.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uistruct.ResolveScrollRegionRecipe(style)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: sr.cachedTokens}, sr.cachedRecipe
}

func (sr *ScrollRegion) hitTest(p gfx.Point) facet.HitResult {
	if sr == nil || sr.Layout.ArrangedBounds.IsEmpty() || !sr.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := sr.cursorShape()
	if sr.focusedVisible && sr.cachedFocusBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: scrollRegionMarkIDFocusRing, Cursor: cursor}
	}
	if sr.cachedVerticalThumb.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: scrollRegionMarkIDScrollbarVerticalOptional, Cursor: facet.CursorGrab}
	}
	if sr.cachedHorizontalThumb.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: scrollRegionMarkIDScrollbarHorizontalOptional, Cursor: facet.CursorGrab}
	}
	if sr.cachedVerticalTrack.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: scrollRegionMarkIDScrollbarVerticalOptional, Cursor: facet.CursorGrab}
	}
	if sr.cachedHorizontalTrack.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: scrollRegionMarkIDScrollbarHorizontalOptional, Cursor: facet.CursorGrab}
	}
	if sr.cachedViewportBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: scrollRegionMarkIDContent, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: scrollRegionMarkIDRoot, Cursor: cursor}
}

func (sr *ScrollRegion) onPointer(e facet.PointerEvent) bool {
	if sr.Disabled.Get() {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		sr.hovered = true
		sr.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		sr.hovered = false
		if !sr.pressed {
			sr.focusFromPointer = false
		}
		sr.dragging = false
		sr.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		if sr.cachedVerticalThumb.Contains(e.Position) {
			sr.dragging = true
			sr.draggingAxis = ScrollDirectionVertical
			sr.focusFromPointer = true
			sr.focusedVisible = false
			sr.updateOffsetFromDrag(e.Position)
			sr.invalidate(facet.DirtyProjection)
			return true
		}
		if sr.cachedHorizontalThumb.Contains(e.Position) {
			sr.dragging = true
			sr.draggingAxis = ScrollDirectionHorizontal
			sr.focusFromPointer = true
			sr.focusedVisible = false
			sr.updateOffsetFromDrag(e.Position)
			sr.invalidate(facet.DirtyProjection)
			return true
		}
		sr.pressed = true
		sr.focusFromPointer = true
		sr.focusedVisible = false
		sr.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerMove:
		if sr.dragging {
			sr.updateOffsetFromDrag(e.Position)
			sr.invalidate(facet.DirtyProjection)
			return true
		}
		return false
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := sr.pressed || sr.dragging
		sr.pressed = false
		sr.dragging = false
		sr.invalidate(facet.DirtyProjection)
		return wasPressed
	default:
		return false
	}
}

func (sr *ScrollRegion) onScroll(e facet.ScrollEvent) bool {
	if sr.Disabled.Get() {
		return false
	}
	if e.DeltaX == 0 && e.DeltaY == 0 {
		return false
	}
	next := sr.scrollOffset
	next.X -= e.DeltaX
	next.Y -= e.DeltaY
	sr.scrollOffset = sr.clampScrollOffset(next)
	if sr.scrollOffset != next {
		sr.invalidate(facet.DirtyProjection)
		return true
	}
	if next != sr.scrollOffset {
		sr.invalidate(facet.DirtyProjection)
	}
	return true
}

func (sr *ScrollRegion) onKey(e facet.KeyEvent) bool {
	if sr.Disabled.Get() {
		return false
	}
	if e.Kind != platform.KeyPress && e.Kind != platform.KeyRepeat {
		return false
	}
	step := sr.keyboardStep()
	switch e.Key {
	case platform.KeyUp:
		sr.scrollOffset.Y -= step
	case platform.KeyDown:
		sr.scrollOffset.Y += step
	case platform.KeyLeft:
		sr.scrollOffset.X -= step
	case platform.KeyRight:
		sr.scrollOffset.X += step
	case platform.KeyPageUp:
		sr.scrollOffset.Y -= sr.viewportSpanY() * 0.8
	case platform.KeyPageDown:
		sr.scrollOffset.Y += sr.viewportSpanY() * 0.8
	case platform.KeyHome:
		sr.scrollOffset = gfx.Point{}
	case platform.KeyEnd:
		sr.scrollOffset = gfx.Point{X: sr.maxScrollX(), Y: sr.maxScrollY()}
	default:
		return false
	}
	sr.scrollOffset = sr.clampScrollOffset(sr.scrollOffset)
	sr.invalidate(facet.DirtyProjection)
	return true
}

func (sr *ScrollRegion) onFocusGained() {
	sr.focusedVisible = !sr.focusFromPointer
	sr.focusFromPointer = false
	sr.invalidate(facet.DirtyProjection)
}

func (sr *ScrollRegion) onFocusLost() {
	sr.focusedVisible = false
	sr.pressed = false
	sr.focusFromPointer = false
	sr.dragging = false
	sr.invalidate(facet.DirtyProjection)
}

func (sr *ScrollRegion) updateParentKind() {
	switch sr.Direction.Get() {
	case ScrollDirectionHorizontal:
		sr.Layout.Parent.Kind = facet.GroupLayoutLinearHorizontal
	default:
		sr.Layout.Parent.Kind = facet.GroupLayoutLinearVertical
	}
}

func (sr *ScrollRegion) syncContentChildren() {
	if sr == nil {
		return
	}
	if len(sr.children) == 0 {
		sr.cachedChildBounds = nil
		sr.cachedChildOrder = nil
	}
}

func (sr *ScrollRegion) groupChild(spec ScrollRegionChild) facet.GroupChild {
	if spec.Facet == nil {
		return facet.GroupChild{}
	}
	base := spec.Facet.Base()
	if base == nil || base.LayoutRole() == nil {
		return facet.GroupChild{}
	}
	return facet.GroupChild{
		FacetID: base.ID(),
		MarkID:  spec.MarkID,
		Attachment: facet.Attachment{
			Placement: spec.Placement,
			ZPriority: spec.ZPriority,
		},
		Layout:   base.LayoutRole(),
		Contract: base.LayoutRole().Child,
	}
}

func (sr *ScrollRegion) updateScrollBounds(bounds gfx.Rect) {
	sr.cachedViewportBounds = bounds
	maxX := maxFloat(0, sr.cachedContentSize.W-bounds.Width())
	maxY := maxFloat(0, sr.cachedContentSize.H-bounds.Height())
	sr.scrollOffset = gfx.Point{
		X: clampFloat(sr.scrollOffset.X, 0, maxX),
		Y: clampFloat(sr.scrollOffset.Y, 0, maxY),
	}
	track := sr.trackThickness()
	if maxY > 0 {
		trackHeight := bounds.Height()
		if maxX > 0 {
			trackHeight -= track
		}
		if trackHeight < 0 {
			trackHeight = 0
		}
		sr.cachedVerticalTrack = gfx.RectFromXYWH(bounds.Max.X-track, bounds.Min.Y, track, trackHeight)
		thumbHeight := maxFloat(track*2, trackHeight*(bounds.Height()/maxFloat(1, sr.cachedContentSize.H)))
		if thumbHeight > trackHeight {
			thumbHeight = trackHeight
		}
		maxOffset := maxFloat(1, maxY)
		thumbY := bounds.Min.Y + (sr.scrollOffset.Y/maxOffset)*(trackHeight-thumbHeight)
		sr.cachedVerticalThumb = gfx.RectFromXYWH(bounds.Max.X-track, thumbY, track, thumbHeight)
	}
	if maxX > 0 {
		trackWidth := bounds.Width()
		if maxY > 0 {
			trackWidth -= track
		}
		if trackWidth < 0 {
			trackWidth = 0
		}
		sr.cachedHorizontalTrack = gfx.RectFromXYWH(bounds.Min.X, bounds.Max.Y-track, trackWidth, track)
		thumbWidth := maxFloat(track*2, trackWidth*(bounds.Width()/maxFloat(1, sr.cachedContentSize.W)))
		if thumbWidth > trackWidth {
			thumbWidth = trackWidth
		}
		maxOffset := maxFloat(1, maxX)
		thumbX := bounds.Min.X + (sr.scrollOffset.X/maxOffset)*(trackWidth-thumbWidth)
		sr.cachedHorizontalThumb = gfx.RectFromXYWH(thumbX, bounds.Max.Y-track, thumbWidth, track)
	}
}

func (sr *ScrollRegion) updateOffsetFromDrag(p gfx.Point) {
	if sr == nil {
		return
	}
	switch sr.draggingAxis {
	case ScrollDirectionHorizontal:
		trackRect := sr.cachedHorizontalTrack
		thumbRect := sr.cachedHorizontalThumb
		maxOffset := sr.maxScrollX()
		if trackRect.IsEmpty() || thumbRect.IsEmpty() || maxOffset <= 0 {
			return
		}
		trackSpan := maxFloat(1, trackRect.Width()-thumbRect.Width())
		pos := p.X - trackRect.Min.X - thumbRect.Width()*0.5
		sr.scrollOffset.X = clampFloat((pos/trackSpan)*maxOffset, 0, maxOffset)
	default:
		trackRect := sr.cachedVerticalTrack
		thumbRect := sr.cachedVerticalThumb
		maxOffset := sr.maxScrollY()
		if trackRect.IsEmpty() || thumbRect.IsEmpty() || maxOffset <= 0 {
			return
		}
		trackSpan := maxFloat(1, trackRect.Height()-thumbRect.Height())
		pos := p.Y - trackRect.Min.Y - thumbRect.Height()*0.5
		sr.scrollOffset.Y = clampFloat((pos/trackSpan)*maxOffset, 0, maxOffset)
	}
	sr.scrollOffset = sr.clampScrollOffset(sr.scrollOffset)
}

func (sr *ScrollRegion) childBoundsForProjection(id facet.FacetID) gfx.Rect {
	if sr == nil {
		return gfx.Rect{}
	}
	if sr.cachedChildBounds == nil {
		return gfx.Rect{}
	}
	return sr.cachedChildBounds[id]
}

func (sr *ScrollRegion) cachedChildBoundsIsValid() bool {
	return sr != nil && len(sr.cachedChildBounds) > 0
}

func (sr *ScrollRegion) cursorShape() facet.CursorShape {
	if sr.Disabled.Get() {
		return facet.CursorDefault
	}
	return facet.CursorDefault
}

func (sr *ScrollRegion) trackThickness() float32 {
	if sr.cachedTokens.Spacing.TouchTarget > 0 {
		return maxFloat(8, sr.cachedTokens.Spacing.TouchTarget*0.12)
	}
	return 8
}

func (sr *ScrollRegion) viewportSpanY() float32 {
	if sr.cachedViewportBounds.IsEmpty() {
		return 0
	}
	return sr.cachedViewportBounds.Height()
}

func (sr *ScrollRegion) keyboardStep() float32 {
	span := sr.viewportSpanY()
	if span <= 0 {
		return 24
	}
	return maxFloat(24, span*0.12)
}

func (sr *ScrollRegion) maxScrollX() float32 {
	return maxFloat(0, sr.cachedContentSize.W-sr.cachedViewportBounds.Width())
}

func (sr *ScrollRegion) maxScrollY() float32 {
	return maxFloat(0, sr.cachedContentSize.H-sr.cachedViewportBounds.Height())
}

func clampFloat(v, minV, maxV float32) float32 {
	if v < minV {
		v = minV
	}
	if v > maxV {
		v = maxV
	}
	return v
}

func (sr *ScrollRegion) clampScrollOffset(next gfx.Point) gfx.Point {
	return gfx.Point{
		X: clampFloat(next.X, 0, sr.maxScrollX()),
		Y: clampFloat(next.Y, 0, sr.maxScrollY()),
	}
}

func (sr *ScrollRegion) barCommands(bounds gfx.Rect, material theme.Material, opacity float32) []gfx.Command {
	if bounds.IsEmpty() || isTransparentMaterial(material) {
		return nil
	}
	cmds := make([]gfx.Command, 0, 4)
	if opacity > 0 && opacity < 1 {
		cmds = append(cmds, gfx.PushOpacity{Alpha: opacity})
	}
	cmds = append(cmds, materialCommands(gfx.RectPath(bounds), material)...)
	if opacity > 0 && opacity < 1 {
		cmds = append(cmds, gfx.PopOpacity{})
	}
	return cmds
}

type scrollRegionGroupPolicy struct {
	region *ScrollRegion
}

func (p scrollRegionGroupPolicy) Kind() facet.GroupLayoutKind {
	if p.region != nil && p.region.Direction.Get() == ScrollDirectionHorizontal {
		return facet.GroupLayoutLinearHorizontal
	}
	return facet.GroupLayoutLinearVertical
}

func (p scrollRegionGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.region == nil {
		return facet.GroupMeasureResult{}, nil
	}
	return facet.GroupMeasureResult{Size: p.region.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}}).Size}, nil
}

func (p scrollRegionGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.region == nil {
		return nil, nil
	}
	p.region.arrange(ctx.ArrangeContext, ctx.Bounds)
	arranged := make([]facet.ArrangedGroupChild, 0, len(children))
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		arranged = append(arranged, facet.ArrangedGroupChild{
			FacetID:   child.FacetID,
			MarkID:    child.MarkID,
			Bounds:    p.region.childBoundsForProjection(child.FacetID),
			Placement: child.Attachment.Placement,
			ZPriority: child.Attachment.ZPriority,
			Contract:  child.Contract,
		})
	}
	return arranged, nil
}
