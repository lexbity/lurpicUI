package action

import (
	"math"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/layout/radial"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiaction"
)

const (
	radialMenuMarkIDRoot       facet.MarkID = 1
	radialMenuMarkIDSurface    facet.MarkID = 2
	radialMenuMarkIDCenterSlot facet.MarkID = 3
	radialMenuMarkIDRadial     facet.MarkID = 4
	radialMenuMarkIDAnchor     facet.MarkID = 5
	radialMenuMarkIDFocusRing  facet.MarkID = 6
)

// RadialChild describes one child facet positioned on a radial track.
type RadialChild struct {
	Child     facet.FacetImpl
	Placement facet.RadialPlacement
}

// RadialMenu implements the action.radial_menu composed container.
type RadialMenu struct {
	marks.Core

	Activated signal.Signal[string]

	Label              marks.Binding[string]
	DefaultTrackRadius float32
	CenterChild        facet.FacetImpl
	RadialChildren     marks.Binding[[]RadialChild]
	Open               bool
	Disabled           marks.Binding[bool]

	hoveredRegion    radialMenuRegion
	pressedRegion    radialMenuRegion
	focusedVisible   bool
	focusFromPointer bool

	cachedTokens           theme.Tokens
	cachedRecipe           shared.RadialMenuSlots
	cachedBounds           gfx.Rect
	cachedFocusBounds      gfx.Rect
	cachedTrackRadius      float32
	cachedSurfaceRadius    float32
	cachedCenter           gfx.Point
	cachedWritingDirection facet.WritingDirection
	cachedArrangedChildren []facet.ArrangedGroupChild
}

type radialMenuRegion uint8

const (
	radialMenuRegionNone radialMenuRegion = iota
	radialMenuRegionRoot
	radialMenuRegionCenter
	radialMenuRegionRadial
	radialMenuRegionAnchor
	radialMenuRegionFocusRing
)

type radialMenuGroupPolicy struct {
	menu *RadialMenu
}

func (radialMenuGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutRadial }

func (p radialMenuGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.menu == nil || p.menu.Disabled.Get() || !p.menu.Open {
		return facet.GroupMeasureResult{}, nil
	}
	policy := radial.New(radial.Config{
		DefaultRadius:    p.menu.defaultTrackRadius(theme.DefaultResolvedContext()),
		StartAngle:       -math.Pi / 2,
		WritingDirection: ctx.WritingDirection,
	})
	size, err := policy.Measure(ctx.MeasureContext, toRadialChildren(children), gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()})
	if err != nil {
		return facet.GroupMeasureResult{}, err
	}
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p radialMenuGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.menu == nil || p.menu.Disabled.Get() || !p.menu.Open {
		return nil, nil
	}
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	policy := radial.New(radial.Config{
		DefaultRadius:    p.menu.defaultTrackRadius(resolved),
		StartAngle:       -math.Pi / 2,
		WritingDirection: p.menu.cachedWritingDirection,
	})
	arranged, err := policy.Arrange(ctx.ArrangeContext, toRadialChildren(children), ctx.Bounds)
	if err != nil {
		return nil, err
	}
	out := make([]facet.ArrangedGroupChild, 0, len(arranged))
	for i := range arranged {
		markID := radialMenuMarkIDRadial
		if i < len(children) && children[i].MarkID == radialMenuMarkIDCenterSlot {
			markID = radialMenuMarkIDCenterSlot
		}
		placement := facet.Placement{}
		if i < len(children) {
			placement = children[i].Attachment.Placement
		}
		out = append(out, facet.ArrangedGroupChild{
			FacetID:   arranged[i].FacetID,
			MarkID:    markID,
			Bounds:    arranged[i].Bounds,
			Placement: placement,
			ZPriority: arranged[i].ZPriority,
			Contract:  arranged[i].Contract,
		})
	}
	return out, nil
}

var _ facet.FacetImpl = (*RadialMenu)(nil)
var _ layout.AnchorExporter = (*RadialMenu)(nil)
var _ marks.Mark = (*RadialMenu)(nil)

// NewRadialMenu constructs an action.radial_menu mark with canonical defaults.
func NewRadialMenu(label string, center facet.FacetImpl, children []RadialChild) *RadialMenu {
	m := &RadialMenu{
		Label:              marks.Const(strings.TrimSpace(label)),
		DefaultTrackRadius: 0,
		Open:               true,
		Disabled:           marks.Const(false),
		RadialChildren:     marks.Const(normalizeRadialChildren(children)),
		focusFromPointer:   false,
		Activated:          signal.NewSignal[string]("radial_menu_activated"),
	}
	m.Core.Facet = facet.NewFacet()
	m.AddBinding(m.Label)
	m.AddBinding(m.Disabled)
	m.AddBinding(m.RadialChildren)

	m.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutRadial,
		Policy:   radialMenuGroupPolicy{menu: m},
		Overflow: facet.OverflowVisible,
		Clipping: facet.GroupClipVisible,
		Children: m,
	}
	m.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsRadial,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := m.measureIntrinsic(ctx, constraints)
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
	m.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return m.measure(ctx, constraints)
	}
	m.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		m.Layout.ArrangedBounds = bounds
		m.arrange(ctx, bounds)
	}
	m.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return m.buildCommands(m.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	m.Hit.OnHitTest = func(pt gfx.Point) facet.HitResult { return m.hitTest(pt) }
	m.Input.OnPointer = func(e facet.PointerEvent) bool { return m.onPointer(e) }
	m.Input.OnKey = func(e facet.KeyEvent) bool { return m.onKey(e) }
	m.Input.OnDismiss = func(e facet.DismissEvent) bool { return m.onDismiss(e) }
	m.Focus.Focusable = func() bool { return !m.Disabled.Get() && m.Open && len(m.Children()) > 0 }
	m.Focus.TabIndex = 0
	m.Focus.OnFocusGained = func() { m.onFocusGained() }
	m.Focus.OnFocusLost = func() { m.onFocusLost() }
	m.RegisterRoles()
	m.attachCenterChild(center)
	m.attachRadialChildren(children)
	return m
}

// Base satisfies facet.FacetImpl.
func (m *RadialMenu) Base() *facet.Facet {
	m.Facet.BindImpl(m)
	return &m.Facet
}

// Descriptor satisfies marks.Mark.
func (m *RadialMenu) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "action", TypeName: "radial_menu"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (m *RadialMenu) AccessibilityRole() string { return "radial_menu" }

// AccessibleName reports the semantic name source required by the spec.
func (m *RadialMenu) AccessibleName() string {
	if m == nil {
		return ""
	}
	if name := strings.TrimSpace(m.Label.Get()); name != "" {
		return name
	}
	return "Radial menu"
}

// ExportAnchors publishes the radial menu anchor set.
func (m *RadialMenu) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if m == nil {
		return nil
	}
	bounds := m.Layout.ArrangedBounds
	out := m.Core.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	if m.cachedCenter != (gfx.Point{}) {
		out["content_anchor"] = m.cachedCenter
	} else {
		out["content_anchor"] = centerOfRect(bounds)
	}
	out["baseline"] = out["content_anchor"]
	return out
}

// Children returns the facet's immediate child list.
func (m *RadialMenu) Children() []facet.GroupChild {
	if m == nil || m.Disabled.Get() || !m.Open {
		return nil
	}
	radialChildren := m.RadialChildren.Get()
	out := make([]facet.GroupChild, 0, len(radialChildren)+1)
	if m.CenterChild != nil && m.CenterChild.Base() != nil && m.CenterChild.Base().LayoutRole() != nil {
		contract := m.CenterChild.Base().LayoutRole().Child
		contract.SupportedPlacement |= facet.SupportsRadial
		out = append(out, facet.GroupChild{
			FacetID: m.CenterChild.Base().ID(),
			MarkID:  radialMenuMarkIDCenterSlot,
			Attachment: facet.Attachment{
				Placement: facet.Placement{
					Mode: facet.PlacementRadial,
					Radial: facet.RadialPlacement{
						Angle:       math.NaN(),
						RadiusTrack: 0,
					},
				},
			},
			Layout:   m.CenterChild.Base().LayoutRole(),
			Contract: contract,
		})
	}
	for i := range radialChildren {
		child := radialChildren[i]
		if child.Child == nil || child.Child.Base() == nil || child.Child.Base().LayoutRole() == nil {
			continue
		}
		contract := child.Child.Base().LayoutRole().Child
		contract.SupportedPlacement |= facet.SupportsRadial
		out = append(out, facet.GroupChild{
			FacetID: child.Child.Base().ID(),
			MarkID:  radialMenuMarkIDRadial,
			Attachment: facet.Attachment{
				Placement: facet.Placement{
					Mode:   facet.PlacementRadial,
					Radial: child.Placement,
				},
			},
			Layout:   child.Child.Base().LayoutRole(),
			Contract: contract,
		})
	}
	return out
}

// OnAttach subscribes dynamic bindings.
func (m *RadialMenu) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }

// OnActivate is unused.
func (m *RadialMenu) OnActivate() { m.Core.OnActivate() }

// OnDeactivate is unused.
func (m *RadialMenu) OnDeactivate() { m.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (m *RadialMenu) OnDetach() {
	m.Core.OnDetach()
	m.cachedTokens = theme.Tokens{}
	m.cachedRecipe = shared.RadialMenuSlots{}
	m.cachedBounds = gfx.Rect{}
	m.cachedFocusBounds = gfx.Rect{}
	m.cachedTrackRadius = 0
	m.cachedSurfaceRadius = 0
	m.cachedCenter = gfx.Point{}
	m.cachedArrangedChildren = nil
}

func (m *RadialMenu) setOpen(open bool) {
	if m == nil || m.Open == open {
		return
	}
	m.Open = open
	if !open {
		m.hoveredRegion = radialMenuRegionNone
		m.pressedRegion = radialMenuRegionNone
		m.focusedVisible = false
		m.focusFromPointer = false
	}
	m.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func (m *RadialMenu) attachCenterChild(child facet.FacetImpl) {
	if child == nil || child.Base() == nil {
		return
	}
	m.CenterChild = child
	if child.Base().Parent() != m.Base() {
		m.attachChild(child)
	}
}

func (m *RadialMenu) attachRadialChildren(children []RadialChild) {
	for _, child := range children {
		if child.Child == nil || child.Child.Base() == nil {
			continue
		}
		if child.Child.Base().Parent() != m.Base() {
			m.attachChild(child.Child)
		}
	}
}

func (m *RadialMenu) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, recipe, _ := m.resolveTheme(ctx)
	m.cachedTokens = resolved.TokenSet()
	m.cachedRecipe = recipe
	m.cachedWritingDirection = ctx.WritingDirection
	m.cachedTrackRadius = m.defaultTrackRadius(resolved)
	if !m.Open || m.Disabled.Get() {
		size := constraints.Constrain(gfx.Size{})
		m.Layout.MeasuredSize = size
		m.Layout.MeasuredResult = facet.MeasureResult{Size: size, Intrinsic: facet.IntrinsicSize{Min: size, Preferred: size, Max: size}, Constraints: constraints}
		return m.Layout.MeasuredResult
	}
	children := m.Children()
	policy := radial.New(radial.Config{
		DefaultRadius:    m.cachedTrackRadius,
		StartAngle:       -math.Pi / 2,
		WritingDirection: ctx.WritingDirection,
	})
	maxSize := constraints.MaxSize
	if maxSize.W <= 0 {
		maxSize.W = maxFloat(resolved.Density.Scale(320), m.cachedTrackRadius*2+64)
	}
	if maxSize.H <= 0 {
		maxSize.H = maxFloat(resolved.Density.Scale(320), m.cachedTrackRadius*2+64)
	}
	size, err := policy.Measure(ctx, toRadialChildren(children), maxSize)
	if err != nil {
		panic(err)
	}
	minSide := maxFloat(resolved.Density.Scale(96), m.cachedTrackRadius*2+32)
	if size.W < minSide {
		size.W = minSide
	}
	if size.H < minSide {
		size.H = minSide
	}
	size = constraints.Constrain(size)
	m.Layout.MeasuredSize = size
	m.Layout.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	return m.Layout.MeasuredResult
}

func (m *RadialMenu) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return m.measure(ctx, constraints).Size
}

func (m *RadialMenu) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	m.cachedBounds = bounds
	m.cachedFocusBounds = gfx.Rect{}
	m.cachedCenter = centerOfRect(bounds)
	m.cachedSurfaceRadius = maxFloat(0, minFloat(bounds.Width(), bounds.Height())*0.5-2)
	m.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() || !m.Open || m.Disabled.Get() {
		m.cachedArrangedChildren = nil
		return
	}
	children := m.Children()
	if len(children) == 0 {
		m.cachedArrangedChildren = nil
		return
	}
	policy := radial.New(radial.Config{
		DefaultRadius:    m.cachedTrackRadius,
		StartAngle:       -math.Pi / 2,
		WritingDirection: m.cachedWritingDirection,
	})
	arranged, err := policy.Arrange(ctx, toRadialChildren(children), bounds)
	if err != nil {
		panic(err)
	}
	m.cachedArrangedChildren = make([]facet.ArrangedGroupChild, len(arranged))
	for i := range arranged {
		markID := radialMenuMarkIDRadial
		if i < len(children) && children[i].MarkID == radialMenuMarkIDCenterSlot {
			markID = radialMenuMarkIDCenterSlot
		}
		placement := facet.Placement{}
		if i < len(children) {
			placement = children[i].Attachment.Placement
		}
		m.cachedArrangedChildren[i] = facet.ArrangedGroupChild{
			FacetID:   arranged[i].FacetID,
			MarkID:    markID,
			Bounds:    arranged[i].Bounds,
			Placement: placement,
			ZPriority: arranged[i].ZPriority,
			Contract:  arranged[i].Contract,
		}
	}
	if len(arranged) > 0 {
		minX, minY := arranged[0].Bounds.Min.X, arranged[0].Bounds.Min.Y
		maxX, maxY := arranged[0].Bounds.Max.X, arranged[0].Bounds.Max.Y
		for _, child := range arranged[1:] {
			if child.Bounds.Min.X < minX {
				minX = child.Bounds.Min.X
			}
			if child.Bounds.Min.Y < minY {
				minY = child.Bounds.Min.Y
			}
			if child.Bounds.Max.X > maxX {
				maxX = child.Bounds.Max.X
			}
			if child.Bounds.Max.Y > maxY {
				maxY = child.Bounds.Max.Y
			}
		}
		m.cachedFocusBounds = gfx.RectFromXYWH(minX, minY, maxX-minX, maxY-minY).Inset(-maxFloat(2, bounds.Width()*0.05), -maxFloat(2, bounds.Height()*0.05))
	}
}

func (m *RadialMenu) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if m == nil || bounds.IsEmpty() {
		return nil
	}
	style, recipe := m.resolveProjectionTheme(runtime)
	state := m.interactionState()
	tokens := style.Tokens
	root := recipe.Root.Resolve(state, tokens)
	surface := recipe.Surface.Resolve(state, tokens)
	centerSlot := recipe.CenterSlot.Resolve(state, tokens)
	track := recipe.RadialTrack.Resolve(state, tokens)
	anchor := recipe.AnchorArrow.Resolve(state, tokens)
	focusRing := recipe.FocusRing.Resolve(theme.StateFocused, tokens)

	cmds := make([]gfx.Command, 0, 16)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, radialMenuMaterialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(surface) && m.cachedSurfaceRadius > 0 {
		cmds = append(cmds, radialMenuMaterialCommands(gfx.CirclePath(centerOfRect(bounds), m.cachedSurfaceRadius), surface)...)
	}
	if m.cachedTrackRadius > 0 && !isTransparentMaterial(track) {
		cmds = append(cmds, radialMenuMaterialCommands(gfx.CirclePath(m.cachedCenter, m.cachedTrackRadius), track)...)
	}
	if !isTransparentMaterial(centerSlot) {
		centerRadius := maxFloat(12, m.cachedTrackRadius*0.22)
		cmds = append(cmds, radialMenuMaterialCommands(gfx.CirclePath(m.cachedCenter, centerRadius), centerSlot)...)
	}
	if !isTransparentMaterial(anchor) {
		arrowW := maxFloat(10, bounds.Width()*0.08)
		arrowH := maxFloat(7, arrowW*0.6)
		top := bounds.Min.Y + maxFloat(1, arrowH*0.2)
		left := m.cachedCenter.X - arrowW*0.5
		right := m.cachedCenter.X + arrowW*0.5
		path := gfx.NewPath().
			MoveTo(gfx.Point{X: left, Y: top + arrowH}).
			LineTo(gfx.Point{X: right, Y: top + arrowH}).
			LineTo(gfx.Point{X: m.cachedCenter.X, Y: top}).
			Close().
			Build()
		cmds = append(cmds, radialMenuMaterialCommands(path, anchor)...)
	}
	childRuntime := runtimeServicesOrNil(runtime)
	if len(m.cachedArrangedChildren) > 0 {
		childMap := make(map[facet.FacetID]facet.FacetImpl, len(m.Base().Children()))
		for _, child := range m.Base().Children() {
			if child == nil {
				continue
			}
			if impl := child.Impl(); impl != nil {
				childMap[child.ID()] = impl
			}
		}
		for _, arranged := range m.cachedArrangedChildren {
			child := childMap[arranged.FacetID]
			if child == nil || child.Base() == nil || child.Base().ProjectionRole() == nil {
				continue
			}
			projected := child.Base().ProjectionRole().Project(facet.ProjectionContext{
				Runtime:      childRuntime,
				Bounds:       arranged.Bounds,
				ContentScale: contentScale,
			})
			if projected != nil && len(projected.Commands) > 0 {
				cmds = append(cmds, projected.Commands...)
			}
		}
	}
	if m.focusedVisible && !m.cachedFocusBounds.IsEmpty() && !isTransparentMaterial(focusRing) {
		cmds = append(cmds, radialMenuMaterialCommands(gfx.CirclePath(m.cachedCenter, maxFloat(m.cachedFocusBounds.Width(), m.cachedFocusBounds.Height())*0.5), focusRing)...)
	}
	return cmds
}

func (m *RadialMenu) hitTest(pt gfx.Point) facet.HitResult {
	if m == nil || m.cachedBounds.IsEmpty() || !m.cachedBounds.Contains(pt) {
		return facet.HitResult{}
	}
	if m.focusedVisible && !m.cachedFocusBounds.IsEmpty() && m.cachedFocusBounds.Contains(pt) {
		return facet.HitResult{Hit: true, MarkID: radialMenuMarkIDFocusRing, Cursor: facet.CursorCrosshair}
	}
	for i := len(m.cachedArrangedChildren) - 1; i >= 0; i-- {
		child := m.cachedArrangedChildren[i]
		if child.Bounds.Contains(pt) {
			if child.MarkID == radialMenuMarkIDCenterSlot {
				return facet.HitResult{Hit: true, MarkID: radialMenuMarkIDCenterSlot, Cursor: facet.CursorCrosshair}
			}
			return facet.HitResult{Hit: true, MarkID: radialMenuMarkIDRadial, Cursor: facet.CursorCrosshair}
		}
	}
	if pt.Y <= m.cachedBounds.Min.Y+maxFloat(12, m.cachedBounds.Height()*0.1) {
		return facet.HitResult{Hit: true, MarkID: radialMenuMarkIDAnchor, Cursor: facet.CursorCrosshair}
	}
	return facet.HitResult{Hit: true, MarkID: radialMenuMarkIDRoot, Cursor: facet.CursorCrosshair}
}

func (m *RadialMenu) onPointer(e facet.PointerEvent) bool {
	if m.Disabled.Get() {
		return false
	}
	region := m.regionAt(e.Position)
	switch e.Kind {
	case platform.PointerEnter, platform.PointerMove:
		m.hoveredRegion = region
		if region == radialMenuRegionCenter || region == radialMenuRegionRadial {
			if m.dispatchPointerToChild(e) {
				m.invalidate(facet.DirtyProjection)
			}
		}
		m.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		m.focusFromPointer = true
		m.focusedVisible = false
		m.pressedRegion = region
		if region == radialMenuRegionCenter || region == radialMenuRegionRadial {
			if m.dispatchPointerToChild(e) {
				m.invalidate(facet.DirtyProjection)
				return true
			}
		}
		if region == radialMenuRegionNone {
			if m.Open {
				m.setOpen(false)
				return true
			}
			return false
		}
		m.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		m.pressedRegion = radialMenuRegionNone
		if region == radialMenuRegionCenter || region == radialMenuRegionRadial {
			if m.dispatchPointerToChild(e) {
				m.invalidate(facet.DirtyProjection)
				return true
			}
		}
		m.invalidate(facet.DirtyProjection)
		return region != radialMenuRegionNone
	case platform.PointerLeave:
		m.hoveredRegion = radialMenuRegionNone
		if !m.focusFromPointer {
			m.focusedVisible = false
		}
		m.invalidate(facet.DirtyProjection)
		return true
	default:
		return false
	}
}

func (m *RadialMenu) dispatchPointerToChild(e facet.PointerEvent) bool {
	if m == nil || len(m.cachedArrangedChildren) == 0 {
		return false
	}
	childMap := make(map[facet.FacetID]facet.FacetImpl, len(m.Base().Children()))
	for _, child := range m.Base().Children() {
		if child == nil {
			continue
		}
		if impl := child.Impl(); impl != nil {
			childMap[child.ID()] = impl
		}
	}
	for i := len(m.cachedArrangedChildren) - 1; i >= 0; i-- {
		arranged := m.cachedArrangedChildren[i]
		if !arranged.Bounds.Contains(e.Position) {
			continue
		}
		impl := childMap[arranged.FacetID]
		if impl == nil || impl.Base() == nil || impl.Base().InputRole() == nil || impl.Base().InputRole().OnPointer == nil {
			continue
		}
		childEvent := e
		childEvent.MarkID = arranged.MarkID
		if impl.Base().InputRole().OnPointer(childEvent) {
			return true
		}
	}
	return false
}

func (m *RadialMenu) onKey(e facet.KeyEvent) bool {
	if m.Disabled.Get() || e.Kind != platform.KeyPress && e.Kind != platform.KeyRepeat {
		return false
	}
	if e.Key != platform.KeyEscape {
		return false
	}
	if m.Open {
		m.setOpen(false)
		return true
	}
	return false
}

func (m *RadialMenu) onDismiss(e facet.DismissEvent) bool {
	_ = e
	if m.Disabled.Get() || !m.Open {
		return false
	}
	m.setOpen(false)
	return true
}

func (m *RadialMenu) onFocusGained() {
	if m.Disabled.Get() {
		return
	}
	m.focusedVisible = !m.focusFromPointer
	m.invalidate(facet.DirtyProjection)
}

func (m *RadialMenu) onFocusLost() {
	m.focusedVisible = false
	m.focusFromPointer = false
	m.pressedRegion = radialMenuRegionNone
	m.invalidate(facet.DirtyProjection)
}

func (m *RadialMenu) interactionState() theme.InteractionState {
	switch {
	case m.Disabled.Get():
		return theme.StateDisabled
	case m.pressedRegion != radialMenuRegionNone:
		return theme.StatePressed
	case m.hoveredRegion != radialMenuRegionNone:
		return theme.StateHover
	case m.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (m *RadialMenu) resolveTheme(ctx facet.MeasureContext) (theme.ResolvedContext, shared.RadialMenuSlots, bool) {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{
		Tokens:    resolved.TokenSet(),
		Materials: resolved.Materials,
		Depth:     resolved.Depth,
	}
	slots, _ := uiaction.ResolveRadialMenuRecipe(style)
	return resolved, slots, true
}

func (m *RadialMenu) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.RadialMenuSlots) {
	if runtime != nil {
		if store := theme.NearestStyleContext(runtime, m.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uiaction.ResolveRadialMenuRecipe(style)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: m.cachedTokens, Materials: nil, Depth: 0}, m.cachedRecipe
}

func (m *RadialMenu) invalidate(flags facet.DirtyFlags) {
	if m == nil {
		return
	}
	m.Facet.Invalidate(flags)
}

func (m *RadialMenu) regionAt(pt gfx.Point) radialMenuRegion {
	if m == nil || m.cachedBounds.IsEmpty() || !m.cachedBounds.Contains(pt) {
		return radialMenuRegionNone
	}
	if m.focusedVisible && !m.cachedFocusBounds.IsEmpty() && m.cachedFocusBounds.Contains(pt) {
		return radialMenuRegionFocusRing
	}
	for _, child := range m.cachedArrangedChildren {
		if child.Bounds.Contains(pt) {
			if child.MarkID == radialMenuMarkIDCenterSlot {
				return radialMenuRegionCenter
			}
			return radialMenuRegionRadial
		}
	}
	if pt.Y <= m.cachedBounds.Min.Y+maxFloat(12, m.cachedBounds.Height()*0.1) {
		return radialMenuRegionAnchor
	}
	return radialMenuRegionRoot
}

func (m *RadialMenu) defaultTrackRadius(resolved theme.ResolvedContext) float32 {
	if m == nil {
		return 0
	}
	if m.DefaultTrackRadius > 0 {
		return m.DefaultTrackRadius
	}
	return maxFloat(resolved.Density.Scale(96), 96)
}

func (m *RadialMenu) attachChild(child facet.FacetImpl) {
	if m == nil || child == nil || child.Base() == nil {
		return
	}
	if child.Base().Parent() != nil && child.Base().Parent() != m.Base() {
		panic("radial menu child already has a parent")
	}
	if child.Base().Parent() == m.Base() {
		return
	}
	if m.Base().State() == facet.StateCreated {
		m.Base().AddChild(child.Base())
		return
	}
	m.Base().AddChildRuntime(child.Base())
}

func (m *RadialMenu) detachChild(child facet.FacetImpl) {
	if m == nil || child == nil || child.Base() == nil || child.Base().Parent() != m.Base() {
		return
	}
	m.Base().RemoveChild(child.Base())
}

func normalizeRadialChildren(children []RadialChild) []RadialChild {
	out := make([]RadialChild, 0, len(children))
	for _, child := range children {
		if child.Child != nil {
			out = append(out, child)
		}
	}
	return out
}

func radialMenuMaterialCommands(path gfx.Path, material theme.Material) []gfx.Command {
	return theme.MaterialCommands(path, material)
}

func toRadialChildren(children []facet.GroupChild) []radial.Child {
	out := make([]radial.Child, 0, len(children))
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		out = append(out, radial.Child{
			FacetID:    child.FacetID,
			Attachment: child.Attachment,
			Layout:     child.Layout,
			Contract:   child.Contract,
		})
	}
	return out
}
