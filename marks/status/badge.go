package status

import (
	"reflect"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uistatus"
)

const (
	badgeMarkIDRoot         facet.MarkID = 1
	badgeMarkIDContainer    facet.MarkID = 2
	badgeMarkIDLabel        facet.MarkID = 3
	badgeMarkIDOptionalIcon facet.MarkID = 4
)

// Badge implements the status.badge canonical mark.
type Badge struct {
	marks.Core

	Label    marks.Binding[string]
	IconRef  marks.Binding[string]
	Disabled marks.Binding[bool]

	cachedTokens           theme.Tokens
	cachedRecipe           shared.BadgeSlots
	cachedBounds           gfx.Rect
	cachedContainerBounds  gfx.Rect
	cachedLabelBounds      gfx.Rect
	cachedIconBounds       gfx.Rect
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedIconSize         float32
	cachedWritingDirection facet.WritingDirection
	cachedLabelFacet       *primitive.Text
	cachedIconFacet        *primitive.Icon
}

var _ facet.FacetImpl = (*Badge)(nil)
var _ layout.AnchorExporter = (*Badge)(nil)
var _ marks.Mark = (*Badge)(nil)

// NewBadge constructs a status.badge mark with canonical defaults.
func NewBadge(label string) *Badge {
	b := &Badge{
		Label:    marks.Const(label),
		IconRef:  marks.Const(""),
		Disabled: marks.Const(false),
	}
	b.Core.Facet = facet.NewFacet()
	b.AddBinding(b.Label)
	b.AddBinding(b.IconRef)
	b.AddBinding(b.Disabled)

	b.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   badgeGroupPolicy{badge: b},
		Children: b,
		Overflow: facet.OverflowClip,
		Clipping: facet.GroupClipBounds,
	}
	b.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := b.measure(ctx, constraints).Size
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
	b.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return b.measure(ctx, constraints)
	}
	b.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		b.Layout.ArrangedBounds = bounds
		b.arrange(ctx, bounds)
	}
	b.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return b.buildCommands(b.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	b.RegisterRoles()
	b.syncChildren()
	return b
}

// Base satisfies facet.FacetImpl.
func (b *Badge) Base() *facet.Facet {
	b.Facet.BindImpl(b)
	return &b.Facet
}

// Descriptor satisfies marks.Mark.
func (b *Badge) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "status", TypeName: "badge"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (b *Badge) AccessibilityRole() string { return "status" }

// AccessibleName reports the semantic name source required by the spec.
func (b *Badge) AccessibleName() string {
	if b == nil {
		return ""
	}
	return strings.TrimSpace(b.Label.Get())
}

// Children returns the immediate child list.
func (b *Badge) Children() []facet.GroupChild {
	if b == nil {
		return nil
	}
	b.syncChildren()
	children := make([]facet.GroupChild, 0, 2)
	if b.cachedIconFacet != nil {
		children = append(children, badgeGroupChild(b.cachedIconFacet.Base(), badgeMarkIDOptionalIcon, 0))
	}
	if b.cachedLabelFacet != nil {
		order := 0
		if b.cachedIconFacet != nil {
			order = 1
		}
		children = append(children, badgeGroupChild(b.cachedLabelFacet.Base(), badgeMarkIDLabel, order))
	}
	return children
}

// ExportAnchors publishes the badge anchor set.
func (b *Badge) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if b == nil {
		return nil
	}
	bounds := b.Layout.ArrangedBounds
	out := b.Core.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	if !b.cachedLabelBounds.IsEmpty() {
		out["label"] = gfx.Point{X: (b.cachedLabelBounds.Min.X + b.cachedLabelBounds.Max.X) * 0.5, Y: (b.cachedLabelBounds.Min.Y + b.cachedLabelBounds.Max.Y) * 0.5}
	}
	if !b.cachedIconBounds.IsEmpty() {
		out["optional_icon"] = gfx.Point{X: (b.cachedIconBounds.Min.X + b.cachedIconBounds.Max.X) * 0.5, Y: (b.cachedIconBounds.Min.Y + b.cachedIconBounds.Max.Y) * 0.5}
	}
	return out
}

// OnAttach subscribes to any attached store.
func (b *Badge) OnAttach(ctx facet.AttachContext) { b.Core.OnAttach() }

// OnActivate is unused.
func (b *Badge) OnActivate() { b.Core.OnActivate() }

// OnDeactivate is unused.
func (b *Badge) OnDeactivate() { b.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (b *Badge) OnDetach() {
	b.Core.OnDetach()
	b.cachedTokens = theme.Tokens{}
	b.cachedRecipe = shared.BadgeSlots{}
	b.cachedBounds = gfx.Rect{}
	b.cachedContainerBounds = gfx.Rect{}
	b.cachedLabelBounds = gfx.Rect{}
	b.cachedIconBounds = gfx.Rect{}
	b.cachedPadX = 0
	b.cachedPadY = 0
	b.cachedGap = 0
	b.cachedIconSize = 0
	b.cachedWritingDirection = facet.WritingDirectionLTR
	b.cachedLabelFacet = nil
	b.cachedIconFacet = nil
}

func (b *Badge) invalidate(flags facet.DirtyFlags) {
	if b == nil {
		return
	}
	b.Facet.Invalidate(flags)
}

func (b *Badge) syncChildren() {
	if b == nil {
		return
	}
	iconRef := strings.TrimSpace(b.IconRef.Get())
	label := strings.TrimSpace(b.Label.Get())
	if label == "" {
		b.cachedLabelFacet = nil
	} else {
		if b.cachedLabelFacet == nil {
			b.cachedLabelFacet = primitive.NewText(marks.Const(label))
		} else {
			b.cachedLabelFacet.Content = marks.Const(label)
			b.cachedLabelFacet.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
		}
		b.cachedLabelFacet.Typography = marks.Const(theme.TextLabelS)
		b.cachedLabelFacet.Overflow = marks.Const(primitive.TextOverflowTruncate)
		if b.Disabled.Get() {
			b.cachedLabelFacet.Foreground = marks.Const(theme.ColorTextDisabled)
			b.cachedLabelFacet.Disabled = marks.Const(true)
		} else {
			b.cachedLabelFacet.Foreground = marks.Const(theme.ColorOnPrimary)
			b.cachedLabelFacet.Disabled = marks.Const(false)
		}
	}
	if iconRef == "" {
		b.cachedIconFacet = nil
		return
	}
	if b.cachedIconFacet == nil {
		b.cachedIconFacet = primitive.NewIcon(primitive.IconRef(iconRef))
	} else {
		b.cachedIconFacet.Source = primitive.IconRef(iconRef)
		b.cachedIconFacet.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
	b.cachedIconFacet.Decorative = marks.Const(true)
	b.cachedIconFacet.ColorSlot = marks.Const(theme.ColorOnPrimary)
	if b.Disabled.Get() {
		b.cachedIconFacet.ColorSlot = marks.Const(theme.ColorTextDisabled)
	}
}

func (b *Badge) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uistatus.ResolveBadgeRecipe(style, b.badgeVariant())
	b.cachedTokens = resolved.TokenSet()
	b.cachedRecipe = slots
	b.cachedWritingDirection = ctx.WritingDirection
	b.cachedPadX = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(8))
	b.cachedPadY = maxFloat(float32(resolved.Spacing(theme.SpacingXS)), resolved.Density.Scale(4))
	b.cachedGap = maxFloat(float32(resolved.Spacing(theme.SpacingXS)), resolved.Density.Scale(2))
	b.cachedIconSize = maxFloat(resolved.Density.Scale(12), float32(resolved.Spacing(theme.SpacingM))*0.7)
	b.syncChildren()
	children := b.Children()
	if len(children) == 0 {
		size := constraints.Constrain(gfx.Size{})
		b.Layout.MeasuredSize = size
		b.Layout.MeasuredResult = facet.MeasureResult{
			Size:        size,
			Intrinsic:   facet.IntrinsicSize{Min: size, Preferred: size, Max: size},
			Constraints: constraints,
		}
		return b.Layout.MeasuredResult
	}
	innerWidth := constraints.MaxSize.W
	if innerWidth > 0 {
		innerWidth = maxFloat(0, innerWidth-b.cachedPadX*2)
	}
	measureCtx := facet.MeasureContext{
		Runtime:          ctx.Runtime,
		Theme:            ctx.Theme,
		ContentScale:     ctx.ContentScale,
		Density:          ctx.Density,
		WritingDirection: ctx.WritingDirection,
	}
	totalH := float32(0)
	maxW := float32(0)
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		childConstraints := constraints
		childConstraints.MaxSize = gfx.Size{W: innerWidth, H: constraints.MaxSize.H}
		if child.MarkID == badgeMarkIDOptionalIcon {
			childConstraints.MaxSize = gfx.Size{W: b.cachedIconSize, H: b.cachedIconSize}
			if icon := b.cachedIconFacet; icon != nil {
				icon.Size = marks.Const(b.cachedIconSize)
			}
		}
		size := child.Layout.Measure(measureCtx, childConstraints).Size
		if size.W > maxW {
			maxW = size.W
		}
		totalH += size.H
		if i < len(children)-1 {
			totalH += b.cachedGap
		}
	}
	if maxW <= 0 {
		maxW = constraints.MaxSize.W
	}
	size := gfx.Size{W: maxW + b.cachedPadX*2, H: totalH + b.cachedPadY*2}
	if len(children) == 1 && children[0].MarkID == badgeMarkIDOptionalIcon {
		size = gfx.Size{W: b.cachedIconSize + b.cachedPadX*2, H: b.cachedIconSize + b.cachedPadY*2}
	}
	measured := constraints.Constrain(size)
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
	return b.Layout.MeasuredResult
}

func (b *Badge) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	b.cachedBounds = bounds
	b.cachedContainerBounds = bounds
	b.cachedLabelBounds = gfx.Rect{}
	b.cachedIconBounds = gfx.Rect{}
	b.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	b.syncChildren()
	children := b.Children()
	if len(children) == 0 {
		return
	}
	inner := bounds.Inset(b.cachedPadX, b.cachedPadY)
	if inner.IsEmpty() {
		inner = bounds
	}
	sizes := make([]gfx.Size, 0, len(children))
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		size := child.Layout.MeasuredSize
		if size == (gfx.Size{}) {
			size = child.Layout.Measure(facet.MeasureContext{
				Runtime:          ctx.Runtime,
				Theme:            ctx.Theme,
				ContentScale:     1,
				WritingDirection: b.cachedWritingDirection,
			}, facet.Constraints{MaxSize: gfx.Size{W: inner.Width(), H: inner.Height()}}).Size
		}
		sizes = append(sizes, size)
	}
	rects := layout.ArrangeVerticalFlowAligned(inner, 0, b.cachedGap, sizes, b.cachedWritingDirection == facet.WritingDirectionRTL, layout.AlignStart)
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		rect := rects[i]
		child.Layout.ArrangedBounds = rect
		switch child.MarkID {
		case badgeMarkIDOptionalIcon:
			b.cachedIconBounds = rect
		case badgeMarkIDLabel:
			b.cachedLabelBounds = rect
		}
	}
}

func (b *Badge) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if b == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := b.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := theme.StateDefault
	if b.Disabled.Get() {
		state = theme.StateDisabled
	}
	root := slots.Root.Resolve(state, tokens)
	container := slots.BadgeContainer.Resolve(state, tokens)
	optionalIcon := slots.OptionalIcon.Resolve(state, tokens)
	cmds := make([]gfx.Command, 0, 32)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(container) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), container)...)
	}
	if !b.cachedIconBounds.IsEmpty() && !isTransparentMaterial(optionalIcon) {
		cmds = append(cmds, materialCommands(gfx.RectPath(b.cachedIconBounds), optionalIcon)...)
	}
	if !bounds.IsEmpty() {
		cmds = append(cmds, gfx.PushClipRect{Rect: bounds})
		if b.cachedIconFacet != nil && !b.cachedIconBounds.IsEmpty() {
			if projected := b.cachedIconFacet.Base().ProjectionRole().Project(facet.ProjectionContext{
				Runtime:      runtimeServicesOrNil(runtime),
				Bounds:       b.cachedIconBounds,
				ContentScale: contentScale,
			}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		if b.cachedLabelFacet != nil && !b.cachedLabelBounds.IsEmpty() {
			if projected := b.cachedLabelFacet.Base().ProjectionRole().Project(facet.ProjectionContext{
				Runtime:      runtimeServicesOrNil(runtime),
				Bounds:       b.cachedLabelBounds,
				ContentScale: contentScale,
			}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		cmds = append(cmds, gfx.PopClip{})
	}
	return cmds
}

func (b *Badge) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.BadgeSlots) {
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
			slots, _ := uistatus.ResolveBadgeRecipe(style, b.badgeVariant())
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: b.cachedTokens}, b.cachedRecipe
}

func (b *Badge) badgeVariant() uistatus.BadgeVariant {
	if b != nil && b.Disabled.Get() {
		return uistatus.BadgeDisabled
	}
	return uistatus.BadgeDefault
}

func badgeGroupChild(base *facet.Facet, markID facet.MarkID, order int) facet.GroupChild {
	if base == nil || base.LayoutRole() == nil {
		return facet.GroupChild{}
	}
	return facet.GroupChild{
		FacetID: base.ID(),
		MarkID:  markID,
		Attachment: facet.Attachment{
			Placement: facet.Placement{
				Mode: facet.PlacementLinear,
				Linear: facet.LinearPlacement{
					Order:          order,
					CrossAxisAlign: facet.CrossAxisStart,
				},
			},
		},
		Layout:   base.LayoutRole(),
		Contract: base.LayoutRole().Child,
	}
}

func runtimeServicesOrNil(runtime any) facet.RuntimeServices {
	if runtime == nil {
		return nil
	}
	services, ok := runtime.(facet.RuntimeServices)
	if !ok {
		return nil
	}
	v := reflect.ValueOf(services)
	switch v.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		if v.IsNil() {
			return nil
		}
	}
	return services
}

func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func isTransparentMaterial(material theme.Material) bool {
	return theme.IsTransparentMaterial(material)
}

func materialCommands(path gfx.Path, material theme.Material) []gfx.Command {
	return theme.MaterialCommands(path, material)
}

type badgeGroupPolicy struct {
	badge *Badge
}

func (badgeGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (p badgeGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.badge == nil {
		return facet.GroupMeasureResult{}, nil
	}
	return facet.GroupMeasureResult{Size: p.badge.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}}).Size}, nil
}

func (p badgeGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.badge == nil {
		return nil, nil
	}
	p.badge.arrange(ctx.ArrangeContext, ctx.Bounds)
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
