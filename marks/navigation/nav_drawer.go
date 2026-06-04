package navigation

import (
	"sort"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	gfxsvg "codeburg.org/lexbit/lurpicui/gfx/svg"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
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
	navDrawerMarkIDRoot          facet.MarkID = 1
	navDrawerMarkIDScrimOptional facet.MarkID = 2
	navDrawerMarkIDDrawerSurface facet.MarkID = 3
	navDrawerMarkIDHeader        facet.MarkID = 4
	navDrawerMarkIDNavItems      facet.MarkID = 5
	navDrawerMarkIDSectionLabels facet.MarkID = 6
	navDrawerMarkIDFocusRing     facet.MarkID = 7
)

// NavDrawerItem describes one navigation destination entry.
type NavDrawerItem struct {
	Key      string
	Label    string
	IconRef  string
	Disabled bool
}

// NavDrawerSection describes a drawer section with an optional heading.
type NavDrawerSection struct {
	Label string
	Items []NavDrawerItem
}

// NavDrawer implements the navigation.nav_drawer canonical mark.
type NavDrawer struct {
	marks.Core

	Label        marks.Binding[string]
	Subtitle     marks.Binding[string]
	Sections     []NavDrawerSection
	Open         marks.Binding[bool]
	Disabled     marks.Binding[bool]
	CurrentIndex marks.Binding[int]

	Activated signal.Signal[int]

	hoveredIndex     int
	pressedIndex     int
	focusedIndex     int
	focusedVisible   bool
	focusFromPointer bool

	cachedTokens           theme.Tokens
	cachedRecipe           shared.NavDrawerSlots
	cachedRootBounds       gfx.Rect
	cachedScrimBounds      gfx.Rect
	cachedDrawerBounds     gfx.Rect
	cachedHeaderBounds     gfx.Rect
	cachedNavBounds        gfx.Rect
	cachedSectionBounds    []gfx.Rect
	cachedItemBounds       []gfx.Rect
	cachedItemGap          float32
	cachedSectionGap       float32
	cachedPadX             float32
	cachedPadY             float32
	cachedItemHeight       float32
	cachedDrawerWidth      float32
	cachedWritingDirection facet.WritingDirection
	cachedIconAssets       []runtimepkg.IconAsset
	cachedIconBounds       []gfx.Rect
	cachedFlatSectionIndex []int
	cachedFlatItems        []NavDrawerItem
	cachedHeaderFacet      *primitive.Text
	cachedSubtitleFacet    *primitive.Text
	cachedSectionFacets    []*primitive.Text
	cachedItemLabelFacets  []*primitive.Text

	groupHeaderFacet    facet.Facet
	groupNavItemsFacet  facet.Facet
	groupHeaderLayout   facet.LayoutRole
	groupNavItemsLayout facet.LayoutRole
}

var _ facet.FacetImpl = (*NavDrawer)(nil)
var _ layout.AnchorExporter = (*NavDrawer)(nil)
var _ marks.Mark = (*NavDrawer)(nil)

// NewNavDrawer constructs a navigation.nav_drawer mark with canonical defaults.
func NewNavDrawer(label string, sections []NavDrawerSection) *NavDrawer {
	d := &NavDrawer{
		Label:              marks.Const(label),
		Subtitle:           marks.Const(""),
		Open:               marks.Const(true),
		CurrentIndex:       marks.Const(0),
		Disabled:           marks.Const(false),
		focusedIndex:       0,
		groupHeaderFacet:   facet.NewFacet(),
		groupNavItemsFacet: facet.NewFacet(),
	}
	d.Core.Facet = facet.NewFacet()
	d.AddBinding(d.Label)
	d.AddBinding(d.Subtitle)
	d.AddBinding(d.Open)
	d.AddBinding(d.CurrentIndex)
	d.AddBinding(d.Disabled)
	d.SetSections(sections)
	d.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   navDrawerGroupPolicy{drawer: d},
		Children: d,
	}
	d.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := d.measureIntrinsic(ctx, constraints)
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
	d.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return d.measure(ctx, constraints)
	}
	d.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		d.Layout.ArrangedBounds = bounds
		d.arrange(ctx, bounds)
	}
	d.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return d.hitTest(p) }
	d.Input.OnPointer = func(e facet.PointerEvent) bool { return d.onPointer(e) }
	d.Input.OnKey = func(e facet.KeyEvent) bool { return d.onKey(e) }
	d.Input.OnDismiss = func(e facet.DismissEvent) bool { return d.onDismiss(e) }
	d.Focus.Focusable = func() bool { return !d.Disabled.Get() && d.Open.Get() && len(d.cachedFlatItems) > 0 }
	d.Focus.TabIndex = 0
	d.Focus.OnFocusGained = func() { d.onFocusGained() }
	d.Focus.OnFocusLost = func() { d.onFocusLost() }
	d.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return d.buildCommands(d.Layout.ArrangedBounds, ctx.Runtime)
	}
	d.RegisterRoles()
	return d
}

// Base satisfies facet.FacetImpl.
func (d *NavDrawer) Base() *facet.Facet {
	d.Facet.BindImpl(d)
	return &d.Facet
}

// Descriptor satisfies marks.Mark.
func (d *NavDrawer) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "navigation", TypeName: "nav_drawer"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (d *NavDrawer) AccessibilityRole() string { return "navigation" }

// AccessibleName reports the semantic name source required by the spec.
func (d *NavDrawer) AccessibleName() string { return d.Label.Get() }

// SetSections updates the drawer sections.
func (d *NavDrawer) SetSections(sections []NavDrawerSection) {
	if d == nil {
		return
	}
	next := append([]NavDrawerSection(nil), sections...)
	for i := range next {
		next[i].Label = strings.TrimSpace(next[i].Label)
		for j := range next[i].Items {
			next[i].Items[j].Key = strings.TrimSpace(next[i].Items[j].Key)
			next[i].Items[j].Label = strings.TrimSpace(next[i].Items[j].Label)
			next[i].Items[j].IconRef = strings.TrimSpace(next[i].Items[j].IconRef)
		}
	}
	d.Sections = next
	d.rebuildFlatItems()
	d.clampIndices()
	d.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// ExportAnchors publishes the drawer anchor set.
func (d *NavDrawer) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if d == nil {
		return nil
	}
	bounds := d.Layout.ArrangedBounds
	out := d.Core.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	if d.cachedHeaderFacet != nil {
		if anchors := d.cachedHeaderFacet.ExportAnchors(ctx); anchors != nil {
			if baseline, ok := anchors["baseline"]; ok {
				out["baseline"] = baseline
			}
		}
	}
	return out
}

// Children returns the facet's immediate child list.
func (d *NavDrawer) Children() []facet.GroupChild {
	if d == nil {
		return nil
	}
	contract := facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := gfx.Size{}
			if constraints.MaxSize.W > 0 || constraints.MaxSize.H > 0 {
				size = constraints.Constrain(size)
			}
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
	return []facet.GroupChild{
		{
			FacetID:    d.groupHeaderFacet.ID(),
			MarkID:     navDrawerMarkIDHeader,
			Attachment: facet.Attachment{Placement: facet.Placement{Mode: facet.PlacementLinear, Linear: facet.LinearPlacement{Order: 0, CrossAxisAlign: facet.CrossAxisStart}}},
			Layout:     &d.groupHeaderLayout,
			Contract:   contract,
		},
		{
			FacetID:    d.groupNavItemsFacet.ID(),
			MarkID:     navDrawerMarkIDNavItems,
			Attachment: facet.Attachment{Placement: facet.Placement{Mode: facet.PlacementLinear, Linear: facet.LinearPlacement{Order: 1, CrossAxisAlign: facet.CrossAxisStart}}},
			Layout:     &d.groupNavItemsLayout,
			Contract:   contract,
		},
	}
}

// OnAttach is unused beyond layout role setup.
func (d *NavDrawer) OnAttach(ctx facet.AttachContext) { d.Core.OnAttach() }

// OnActivate is unused.
func (d *NavDrawer) OnActivate() { d.Core.OnActivate() }

// OnDeactivate is unused.
func (d *NavDrawer) OnDeactivate() { d.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (d *NavDrawer) OnDetach() {
	d.Core.OnDetach()
	d.cachedTokens = theme.Tokens{}
	d.cachedRecipe = shared.NavDrawerSlots{}
	d.cachedRootBounds = gfx.Rect{}
	d.cachedScrimBounds = gfx.Rect{}
	d.cachedDrawerBounds = gfx.Rect{}
	d.cachedHeaderBounds = gfx.Rect{}
	d.cachedNavBounds = gfx.Rect{}
	d.cachedSectionBounds = nil
	d.cachedItemBounds = nil
	d.cachedItemGap = 0
	d.cachedSectionGap = 0
	d.cachedPadX = 0
	d.cachedPadY = 0
	d.cachedItemHeight = 0
	d.cachedDrawerWidth = 0
	d.cachedIconAssets = nil
	d.cachedIconBounds = nil
	d.cachedFlatSectionIndex = nil
	d.cachedFlatItems = nil
	d.cachedHeaderFacet = nil
	d.cachedSubtitleFacet = nil
	d.cachedSectionFacets = nil
	d.cachedItemLabelFacets = nil
	d.groupHeaderLayout = facet.LayoutRole{}
	d.groupNavItemsLayout = facet.LayoutRole{}
}

func (d *NavDrawer) invalidate(flags facet.DirtyFlags) {
	if d == nil {
		return
	}
	d.Facet.Invalidate(flags)
}

func (d *NavDrawer) syncTextFacets(maxWidth float32) {
	if d == nil {
		return
	}
	if maxWidth < 0 {
		maxWidth = 0
	}
	if d.cachedHeaderFacet == nil {
		d.cachedHeaderFacet = primitive.NewText(marks.Const(d.Label.Get()))
	}
	d.cachedHeaderFacet.Content = marks.Const(d.Label.Get())
	d.cachedHeaderFacet.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	d.cachedHeaderFacet.Typography = marks.Const(theme.TextHeadingS)
	d.cachedHeaderFacet.Foreground = marks.Const(theme.ColorText)
	d.cachedHeaderFacet.Disabled = marks.Const(d.Disabled.Get())
	d.cachedHeaderFacet.Overflow = marks.Const(primitive.TextOverflowTruncate)
	d.cachedHeaderFacet.MaxWidth = marks.Const(maxWidth)

	if d.cachedSubtitleFacet == nil {
		d.cachedSubtitleFacet = primitive.NewText(marks.Const(d.Subtitle.Get()))
	}
	d.cachedSubtitleFacet.Content = marks.Const(d.Subtitle.Get())
	d.cachedSubtitleFacet.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	d.cachedSubtitleFacet.Typography = marks.Const(theme.TextBodyM)
	d.cachedSubtitleFacet.Foreground = marks.Const(theme.ColorTextSecondary)
	d.cachedSubtitleFacet.Disabled = marks.Const(d.Disabled.Get())
	d.cachedSubtitleFacet.Overflow = marks.Const(primitive.TextOverflowTruncate)
	d.cachedSubtitleFacet.MaxWidth = marks.Const(maxWidth)

	if len(d.cachedSectionFacets) != len(d.Sections) {
		d.cachedSectionFacets = make([]*primitive.Text, len(d.Sections))
	}
	for i := range d.Sections {
		if d.cachedSectionFacets[i] == nil {
			d.cachedSectionFacets[i] = primitive.NewText(marks.Const(d.Sections[i].Label))
		}
		d.cachedSectionFacets[i].Content = marks.Const(d.Sections[i].Label)
		d.cachedSectionFacets[i].Invalidate(facet.DirtyLayout | facet.DirtyProjection)
		d.cachedSectionFacets[i].Typography = marks.Const(theme.TextLabelM)
		d.cachedSectionFacets[i].Foreground = marks.Const(theme.ColorTextSecondary)
		d.cachedSectionFacets[i].Disabled = marks.Const(d.Disabled.Get())
		d.cachedSectionFacets[i].Overflow = marks.Const(primitive.TextOverflowTruncate)
		d.cachedSectionFacets[i].MaxWidth = marks.Const(maxWidth)
	}

	if len(d.cachedItemLabelFacets) != len(d.cachedFlatItems) {
		d.cachedItemLabelFacets = make([]*primitive.Text, len(d.cachedFlatItems))
	}
	for i := range d.cachedFlatItems {
		if d.cachedItemLabelFacets[i] == nil {
			d.cachedItemLabelFacets[i] = primitive.NewText(marks.Const(d.cachedFlatItems[i].Label))
		}
		d.cachedItemLabelFacets[i].Content = marks.Const(d.cachedFlatItems[i].Label)
		d.cachedItemLabelFacets[i].Invalidate(facet.DirtyLayout | facet.DirtyProjection)
		d.cachedItemLabelFacets[i].Typography = marks.Const(theme.TextLabelM)
		d.cachedItemLabelFacets[i].Foreground = marks.Const(theme.ColorText)
		d.cachedItemLabelFacets[i].Disabled = marks.Const(d.Disabled.Get() || d.cachedFlatItems[i].Disabled)
		d.cachedItemLabelFacets[i].Overflow = marks.Const(primitive.TextOverflowTruncate)
		d.cachedItemLabelFacets[i].MaxWidth = marks.Const(maxWidth)
	}
}

func (d *NavDrawer) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uinav.ResolveNavDrawerRecipe(style)
	d.cachedTokens = resolved.TokenSet()
	d.cachedRecipe = slots
	d.cachedWritingDirection = ctx.WritingDirection
	d.cachedPadX = maxFloat(float32(resolved.Spacing(theme.SpacingL)), resolved.Density.Scale(16))
	d.cachedPadY = maxFloat(float32(resolved.Spacing(theme.SpacingL)), resolved.Density.Scale(16))
	d.cachedItemGap = float32(resolved.Spacing(theme.SpacingS))
	d.cachedSectionGap = float32(resolved.Spacing(theme.SpacingL))
	d.cachedItemHeight = maxFloat(resolved.Density.Scale(44), resolved.Density.Scale(resolved.TokenSet().Spacing.TouchTarget))
	d.cachedDrawerWidth = d.drawerWidth(resolved)
	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = resolved.Density.Scale(420)
	}
	d.syncTextFacets(maxWidth)
	headerSize := gfx.Size{}
	if d.cachedHeaderFacet != nil && d.cachedHeaderFacet.Base() != nil && d.cachedHeaderFacet.Base().LayoutRole() != nil {
		headerSize = d.cachedHeaderFacet.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: maxWidth, H: constraints.MaxSize.H}}).Size
	}
	subtitleSize := gfx.Size{}
	if d.cachedSubtitleFacet != nil && d.cachedSubtitleFacet.Base() != nil && d.cachedSubtitleFacet.Base().LayoutRole() != nil {
		subtitleSize = d.cachedSubtitleFacet.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: maxWidth, H: constraints.MaxSize.H}}).Size
	}
	flatCount := len(d.cachedFlatItems)
	d.cachedIconAssets = make([]runtimepkg.IconAsset, flatCount)
	d.cachedIconBounds = make([]gfx.Rect, flatCount)
	for i := range d.cachedFlatItems {
		if i < len(d.cachedItemLabelFacets) {
			if f := d.cachedItemLabelFacets[i]; f != nil && f.Base() != nil && f.Base().LayoutRole() != nil {
				_ = f.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: maxWidth, H: constraints.MaxSize.H}})
			}
		}
		if d.cachedFlatItems[i].IconRef != "" {
			if asset, ok := d.resolveIcon(ctx.Runtime, d.cachedFlatItems[i].IconRef); ok {
				d.cachedIconAssets[i] = asset
			}
		}
		if d.cachedFlatItems[i].IconRef != "" {
			size := resolved.TokenSet().Spacing.IconSize
			if size <= 0 {
				size = 20
			}
			d.cachedIconBounds[i] = gfx.RectFromXYWH(0, 0, size, size)
		}
	}
	itemsH := float32(0)
	for i, section := range d.Sections {
		sectionSize := gfx.Size{}
		if i < len(d.cachedSectionFacets) {
			if f := d.cachedSectionFacets[i]; f != nil && f.Base() != nil && f.Base().LayoutRole() != nil {
				sectionSize = f.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: maxWidth, H: constraints.MaxSize.H}}).Size
			}
		}
		if i > 0 {
			itemsH += d.cachedSectionGap
		}
		if sectionSize != (gfx.Size{}) {
			itemsH += sectionSize.H + d.cachedItemGap
		}
		itemsH += float32(len(section.Items)) * d.cachedItemHeight
		if len(section.Items) > 0 {
			itemsH += float32(len(section.Items)-1) * d.cachedItemGap
		}
	}
	drawerW := maxFloat(d.cachedDrawerWidth, maxFloat(headerSize.W, subtitleSize.W)+d.cachedPadX*2)
	for i := range d.cachedItemLabelFacets {
		itemSize := d.cachedItemLabelFacets[i].Base().LayoutRole().MeasuredSize
		w := itemSize.W + d.cachedPadX*2 + d.cachedItemHeight
		if w > drawerW {
			drawerW = w
		}
	}
	contentW := maxFloat(0, drawerW-d.cachedPadX*2)
	headerSize = gfx.Size{W: contentW, H: headerSize.H}
	if d.cachedSubtitleFacet != nil && headerSize.H > 0 && subtitleSize.H > 0 {
		headerSize.H += float32(resolved.Spacing(theme.SpacingXS)) + subtitleSize.H
	}
	itemsSize := gfx.Size{W: contentW, H: itemsH}
	d.groupHeaderLayout.MeasuredSize = headerSize
	d.groupHeaderLayout.MeasuredResult = facet.MeasureResult{Size: headerSize}
	d.groupNavItemsLayout.MeasuredSize = itemsSize
	d.groupNavItemsLayout.MeasuredResult = facet.MeasureResult{Size: itemsSize}
	groupSize, err := d.Layout.Parent.Policy.MeasureGroup(facet.GroupMeasureContext{MeasureContext: facet.MeasureContext{}}, d.Children())
	contentH := headerSize.H + itemsSize.H
	if len(d.Children()) > 1 {
		contentH += d.cachedSectionGap
	}
	if err == nil && groupSize.Size != (gfx.Size{}) {
		contentH = maxFloat(contentH, groupSize.Size.H)
	}
	measuredW := drawerW
	measuredH := contentH + d.cachedPadY*2
	if d.Open.Get() {
		if constraints.MaxSize.W > 0 {
			measuredW = maxFloat(measuredW, constraints.MaxSize.W)
		}
		if constraints.MaxSize.H > 0 {
			measuredH = maxFloat(measuredH, constraints.MaxSize.H)
		}
	}
	measured := constraints.Constrain(gfx.Size{W: measuredW, H: measuredH})
	d.Layout.MeasuredSize = measured
	d.Layout.MeasuredResult = facet.MeasureResult{
		Size: measured,
		Intrinsic: facet.IntrinsicSize{
			Min:       measured,
			Preferred: measured,
			Max:       measured,
		},
		Constraints: constraints,
	}
	return d.Layout.MeasuredResult
}

func (d *NavDrawer) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return d.measure(ctx, constraints).Size
}

func (d *NavDrawer) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	d.cachedRootBounds = bounds
	d.cachedScrimBounds = gfx.Rect{}
	d.cachedDrawerBounds = gfx.Rect{}
	d.cachedHeaderBounds = gfx.Rect{}
	d.cachedNavBounds = gfx.Rect{}
	d.cachedSectionBounds = nil
	d.cachedItemBounds = nil
	d.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() || !d.Open.Get() {
		return
	}
	drawerW := d.cachedDrawerWidth
	if drawerW <= 0 {
		drawerW = maxFloat(bounds.Width()*0.8, 280)
	}
	if drawerW > bounds.Width() {
		drawerW = bounds.Width()
	}
	drawerH := bounds.Height()
	if drawerH <= 0 {
		drawerH = bounds.Height()
	}
	if drawerH <= 0 {
		drawerH = d.Layout.MeasuredSize.H
	}
	if drawerH <= 0 {
		drawerH = 720
	}
	if d.cachedWritingDirection == facet.WritingDirectionRTL {
		d.cachedDrawerBounds = gfx.RectFromXYWH(bounds.Max.X-drawerW, bounds.Min.Y, drawerW, drawerH)
	} else {
		d.cachedDrawerBounds = gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, drawerW, drawerH)
	}
	d.cachedScrimBounds = bounds
	contentBounds := gfx.RectFromXYWH(
		d.cachedDrawerBounds.Min.X+d.cachedPadX,
		d.cachedDrawerBounds.Min.Y+d.cachedPadY,
		maxFloat(0, d.cachedDrawerBounds.Width()-d.cachedPadX*2),
		maxFloat(0, d.cachedDrawerBounds.Height()-d.cachedPadY*2),
	)
	if policy, ok := d.Layout.Parent.Policy.(navDrawerGroupPolicy); ok {
		if arranged, err := policy.ArrangeGroup(facet.GroupArrangeContext{ArrangeContext: facet.ArrangeContext{}, Bounds: contentBounds}, d.Children()); err == nil {
			for _, child := range arranged {
				switch child.MarkID {
				case navDrawerMarkIDHeader:
					d.cachedHeaderBounds = child.Bounds
				case navDrawerMarkIDNavItems:
					d.cachedNavBounds = child.Bounds
				}
			}
		}
	}
	if d.cachedHeaderFacet != nil && !d.cachedHeaderBounds.IsEmpty() {
		d.cachedHeaderFacet.Base().LayoutRole().Arrange(ctx, d.cachedHeaderBounds)
		if d.cachedSubtitleFacet != nil {
			subtitleH := d.cachedSubtitleFacet.Base().LayoutRole().MeasuredSize.H
			if subtitleH > 0 {
				subBounds := gfx.RectFromXYWH(d.cachedHeaderBounds.Min.X, d.cachedHeaderBounds.Max.Y+float32(d.cachedItemGap)*0.5, d.cachedHeaderBounds.Width(), subtitleH)
				d.cachedSubtitleFacet.Base().LayoutRole().Arrange(ctx, subBounds)
			}
		}
	}
	x := d.cachedNavBounds.Min.X
	y := d.cachedNavBounds.Min.Y
	if d.cachedHeaderBounds.IsEmpty() {
		x = d.cachedDrawerBounds.Min.X + d.cachedPadX
		y = d.cachedDrawerBounds.Min.Y + d.cachedPadY
	}
	flatIndex := 0
	sectionY := y
	d.cachedSectionBounds = make([]gfx.Rect, len(d.Sections))
	d.cachedItemBounds = make([]gfx.Rect, len(d.cachedFlatItems))
	for si, section := range d.Sections {
		if si > 0 {
			sectionY += d.cachedSectionGap
		}
		if si < len(d.cachedSectionFacets) && d.cachedSectionFacets[si] != nil {
			sh := d.cachedSectionFacets[si].Base().LayoutRole().MeasuredSize.H
			d.cachedSectionBounds[si] = gfx.RectFromXYWH(x, sectionY, d.cachedDrawerBounds.Width()-d.cachedPadX*2, sh)
			d.cachedSectionFacets[si].Base().LayoutRole().Arrange(ctx, d.cachedSectionBounds[si])
			sectionY += sh + d.cachedItemGap
		}
		for ii := range section.Items {
			row := gfx.RectFromXYWH(x, sectionY, d.cachedDrawerBounds.Width()-d.cachedPadX*2, d.cachedItemHeight)
			d.cachedItemBounds[flatIndex] = row
			iconRect := gfx.Rect{}
			if len(d.cachedIconBounds) > flatIndex && !d.cachedIconBounds[flatIndex].IsEmpty() {
				iconRect = gfx.RectFromXYWH(row.Min.X+d.cachedPadX, row.Min.Y+(row.Height()-d.cachedIconBounds[flatIndex].Height())*0.5, d.cachedIconBounds[flatIndex].Width(), d.cachedIconBounds[flatIndex].Height())
				d.cachedIconBounds[flatIndex] = iconRect
			}
			labelFacet := d.cachedItemLabelFacets[flatIndex]
			labelW := float32(0)
			labelH := float32(0)
			if labelFacet != nil {
				labelW = labelFacet.Base().LayoutRole().MeasuredSize.W
				labelH = labelFacet.Base().LayoutRole().MeasuredSize.H
			}
			labelX := row.Min.X + d.cachedPadX*1.5 + iconRect.Width()
			if d.cachedWritingDirection == facet.WritingDirectionRTL {
				labelX = row.Max.X - d.cachedPadX*1.5 - iconRect.Width() - labelW
			}
			labelRect := text.AlignRectY(gfx.RectFromXYWH(labelX, row.Min.Y, labelW, labelH), row.Min.Y, row.Height())
			if labelFacet != nil {
				labelFacet.Base().LayoutRole().Arrange(ctx, labelRect)
			}
			sectionY += d.cachedItemHeight
			if ii < len(section.Items)-1 {
				sectionY += d.cachedItemGap
			}
			flatIndex++
		}
	}
}

func (d *NavDrawer) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.NavDrawerSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: d.cachedTokens}, d.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, d.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uinav.ResolveNavDrawerRecipe(style)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: d.cachedTokens}, d.cachedRecipe
}

func (d *NavDrawer) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if d == nil || bounds.IsEmpty() || !d.Open.Get() {
		return nil
	}
	style, slots := d.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	root := slots.Root.Resolve(d.rootState(), tokens)
	scrim := slots.ScrimOptional.Resolve(d.rootState(), tokens)
	drawer := slots.DrawerSurface.Resolve(d.rootState(), tokens)
	header := slots.Header.Resolve(d.headerState(), tokens)
	items := slots.NavItems.Resolve(d.itemState(), tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)

	cmds := make([]gfx.Command, 0, 96)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(scrim) && !d.cachedScrimBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.RectPath(d.cachedScrimBounds), scrim)...)
	}
	if !isTransparentMaterial(drawer) && !d.cachedDrawerBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(d.cachedDrawerBounds, float32(tokens.Radius.LG)), drawer)...)
	}
	if !isTransparentMaterial(header) && !d.cachedHeaderBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.RectPath(d.cachedHeaderBounds), header)...)
	}
	if d.cachedHeaderFacet != nil && !d.cachedHeaderBounds.IsEmpty() {
		if projected := d.cachedHeaderFacet.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: d.cachedHeaderBounds, ContentScale: 1}); projected != nil {
			cmds = append(cmds, projected.Commands...)
		}
	}
	if d.cachedSubtitleFacet != nil && !d.cachedHeaderBounds.IsEmpty() {
		subtitleH := d.cachedSubtitleFacet.Base().LayoutRole().MeasuredSize.H
		if subtitleH > 0 {
			subBounds := gfx.RectFromXYWH(d.cachedHeaderBounds.Min.X, d.cachedHeaderBounds.Max.Y+float32(d.cachedItemGap)*0.5, d.cachedHeaderBounds.Width(), subtitleH)
			if projected := d.cachedSubtitleFacet.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: subBounds, ContentScale: 1}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
	}
	for si := range d.Sections {
		if si < len(d.cachedSectionBounds) && !d.cachedSectionBounds[si].IsEmpty() && si < len(d.cachedSectionFacets) {
			if projected := d.cachedSectionFacets[si].Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: d.cachedSectionBounds[si], ContentScale: 1}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
	}
	for i := range d.cachedFlatItems {
		rect := d.cachedItemBounds[i]
		if rect.IsEmpty() {
			continue
		}
		state := d.itemStateAt(i)
		material := items
		switch state {
		case theme.StateDisabled:
			material = slots.NavItems.Resolve(theme.StateDisabled, tokens)
		case theme.StateHover:
			material = slots.NavItems.Resolve(theme.StateHover, tokens)
		case theme.StatePressed:
			material = slots.NavItems.Resolve(theme.StatePressed, tokens)
		case theme.StateFocused:
			material = slots.NavItems.Resolve(theme.StateFocused, tokens)
		case theme.StateSelected:
			material = slots.NavItems.Resolve(theme.StateSelected, tokens)
		}
		if !isTransparentMaterial(material) {
			if i == d.clampedCurrentIndex() {
				cmds = append(cmds, materialCommands(gfx.RoundedRectPath(rect, float32(tokens.Radius.MD)), material)...)
			} else if state == theme.StateHover || state == theme.StatePressed || state == theme.StateFocused {
				cmds = append(cmds, materialCommands(gfx.RoundedRectPath(rect, float32(tokens.Radius.MD)), material)...)
			}
		}
		if len(d.cachedIconAssets) > i && len(d.cachedIconAssets[i].Path.Segments) > 0 && len(d.cachedIconBounds) > i && !d.cachedIconBounds[i].IsEmpty() {
			cmds = append(cmds, d.iconCommands(d.cachedIconAssets[i], d.cachedIconBounds[i], items)...)
		}
		if i < len(d.cachedItemLabelFacets) && !isTransparentMaterial(items) && !d.cachedItemBounds[i].IsEmpty() {
			if projected := d.cachedItemLabelFacets[i].Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: d.cachedItemLabelFacets[i].Base().LayoutRole().ArrangedBounds, ContentScale: 1}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
	}
	if d.focusedVisible && !isTransparentMaterial(focus) {
		idx := d.clampedFocusedIndex()
		if idx >= 0 && idx < len(d.cachedItemBounds) {
			active := d.cachedItemBounds[idx]
			if !active.IsEmpty() {
				inset := maxFloat(1, active.Height()*0.08)
				cmds = append(cmds, materialCommands(gfx.RoundedRectPath(active.Inset(-inset, -inset), float32(tokens.Radius.MD)+inset), focus)...)
			}
		}
	}
	return cmds
}

func (d *NavDrawer) hitTest(p gfx.Point) facet.HitResult {
	if d == nil || d.Layout.ArrangedBounds.IsEmpty() || !d.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := d.cursorShape()
	if d.focusedVisible && d.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: navDrawerMarkIDFocusRing, Cursor: cursor}
	}
	if d.cachedDrawerBounds.Contains(p) {
		if d.cachedHeaderBounds.Contains(p) {
			return facet.HitResult{Hit: true, MarkID: navDrawerMarkIDHeader, Cursor: cursor}
		}
		for _, rect := range d.cachedSectionBounds {
			if rect.Contains(p) {
				return facet.HitResult{Hit: true, MarkID: navDrawerMarkIDSectionLabels, Cursor: cursor}
			}
		}
		for i, rect := range d.cachedItemBounds {
			if rect.Contains(p) {
				return facet.HitResult{Hit: true, MarkID: navDrawerMarkIDNavItems, Cursor: d.cursorForItem(i)}
			}
		}
		return facet.HitResult{Hit: true, MarkID: navDrawerMarkIDDrawerSurface, Cursor: cursor}
	}
	if d.cachedScrimBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: navDrawerMarkIDScrimOptional, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: navDrawerMarkIDRoot, Cursor: cursor}
}

func (d *NavDrawer) onPointer(e facet.PointerEvent) bool {
	if d.Disabled.Get() {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		d.hoveredIndex = d.indexAt(e.Position)
		d.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		d.hoveredIndex = -1
		if d.pressedIndex < 0 {
			d.focusFromPointer = false
		}
		d.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		if d.cachedDrawerBounds.Contains(e.Position) {
			idx := d.indexAt(e.Position)
			if idx >= 0 && !d.isDisabledIndex(idx) {
				d.hoveredIndex = idx
				d.pressedIndex = idx
				d.focusFromPointer = true
				d.focusedVisible = false
				d.invalidate(facet.DirtyProjection)
				return true
			}
			d.pressedIndex = -1
			d.hoveredIndex = -1
			d.focusFromPointer = true
			d.focusedVisible = false
			d.invalidate(facet.DirtyProjection)
			return true
		}
		if d.Open.Get() {
			d.Open = marks.Const(false)
			d.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
			return true
		}
		return false
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := d.pressedIndex >= 0
		idx := d.pressedIndex
		d.pressedIndex = -1
		d.invalidate(facet.DirtyProjection)
		if wasPressed {
			if hit := d.indexAt(e.Position); hit >= 0 && hit == idx && !d.isDisabledIndex(hit) {
				d.activateIndex(hit)
				d.Open = marks.Const(false)
				d.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
				return true
			}
			return true
		}
		return false
	case platform.PointerMove:
		if d.cachedDrawerBounds.Contains(e.Position) {
			d.hoveredIndex = d.indexAt(e.Position)
			d.invalidate(facet.DirtyProjection)
			return true
		}
		d.hoveredIndex = -1
		d.invalidate(facet.DirtyProjection)
		return true
	default:
		return false
	}
}

func (d *NavDrawer) onKey(e facet.KeyEvent) bool {
	if d.Disabled.Get() || !d.Open.Get() || len(d.cachedFlatItems) == 0 {
		return false
	}
	switch e.Key {
	case platform.KeyUp, platform.KeyDown, platform.KeyHome, platform.KeyEnd, platform.KeyPageUp, platform.KeyPageDown, platform.KeySpace, platform.KeyEnter, platform.KeyEscape:
		switch e.Kind {
		case platform.KeyPress, platform.KeyRepeat:
			switch e.Key {
			case platform.KeyUp, platform.KeyPageUp:
				d.moveFocus(-1)
				return true
			case platform.KeyDown, platform.KeyPageDown:
				d.moveFocus(1)
				return true
			case platform.KeyHome:
				d.setFirstFocus()
				return true
			case platform.KeyEnd:
				d.setLastFocus()
				return true
			case platform.KeyEscape:
				d.Open = marks.Const(false)
				d.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
				return true
			case platform.KeySpace, platform.KeyEnter:
				d.pressedIndex = d.clampedFocusedIndex()
				d.invalidate(facet.DirtyProjection)
				return true
			}
		case platform.KeyRelease:
			if e.Key == platform.KeySpace || e.Key == platform.KeyEnter {
				wasPressed := d.pressedIndex >= 0
				idx := d.pressedIndex
				d.pressedIndex = -1
				d.invalidate(facet.DirtyProjection)
				if wasPressed && idx >= 0 {
					d.activateIndex(idx)
					d.Open = marks.Const(false)
					d.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
					return true
				}
			}
		}
	}
	return false
}

func (d *NavDrawer) onDismiss(e facet.DismissEvent) bool {
	_ = e
	if d.Disabled.Get() || !d.Open.Get() {
		return false
	}
	d.Open = marks.Const(false)
	d.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	return true
}

func (d *NavDrawer) onFocusGained() {
	d.focusedVisible = !d.focusFromPointer
	d.focusFromPointer = false
	d.focusedIndex = d.firstEnabledIndex()
	d.invalidate(facet.DirtyProjection)
}

func (d *NavDrawer) onFocusLost() {
	d.focusedVisible = false
	d.pressedIndex = -1
	d.focusFromPointer = false
	d.invalidate(facet.DirtyProjection)
}

func (d *NavDrawer) rootState() theme.InteractionState {
	switch {
	case d.Disabled.Get():
		return theme.StateDisabled
	case d.pressedIndex >= 0:
		return theme.StatePressed
	case d.hoveredIndex >= 0:
		return theme.StateHover
	case d.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (d *NavDrawer) headerState() theme.InteractionState {
	if d.Disabled.Get() {
		return theme.StateDisabled
	}
	return theme.StateDefault
}

func (d *NavDrawer) itemState() theme.InteractionState {
	if d.Disabled.Get() {
		return theme.StateDisabled
	}
	if d.pressedIndex >= 0 {
		return theme.StatePressed
	}
	if d.hoveredIndex >= 0 {
		return theme.StateHover
	}
	if d.focusedVisible {
		return theme.StateFocused
	}
	return theme.StateDefault
}

func (d *NavDrawer) itemStateAt(index int) theme.InteractionState {
	if d.Disabled.Get() || d.isDisabledIndex(index) {
		return theme.StateDisabled
	}
	if index == d.clampedCurrentIndex() {
		return theme.StateSelected
	}
	if d.pressedIndex == index {
		return theme.StatePressed
	}
	if d.hoveredIndex == index {
		return theme.StateHover
	}
	if d.focusedVisible && d.clampedFocusedIndex() == index {
		return theme.StateFocused
	}
	return theme.StateDefault
}

func (d *NavDrawer) moveFocus(delta int) {
	if len(d.cachedFlatItems) == 0 {
		return
	}
	start := d.clampedFocusedIndex()
	for step := 1; step <= len(d.cachedFlatItems); step++ {
		next := start + delta*step
		for next < 0 {
			next += len(d.cachedFlatItems)
		}
		next %= len(d.cachedFlatItems)
		if !d.isDisabledIndex(next) {
			d.focusedIndex = next
			d.invalidate(facet.DirtyProjection)
			return
		}
	}
}

func (d *NavDrawer) setFirstFocus() {
	if idx := d.firstEnabledIndex(); idx >= 0 {
		d.focusedIndex = idx
		d.invalidate(facet.DirtyProjection)
	}
}

func (d *NavDrawer) setLastFocus() {
	for i := len(d.cachedFlatItems) - 1; i >= 0; i-- {
		if !d.isDisabledIndex(i) {
			d.focusedIndex = i
			d.invalidate(facet.DirtyProjection)
			return
		}
	}
}

func (d *NavDrawer) firstEnabledIndex() int {
	for i := range d.cachedFlatItems {
		if !d.isDisabledIndex(i) {
			return i
		}
	}
	return d.clampedCurrentIndex()
}

func (d *NavDrawer) activateIndex(index int) {
	if index < 0 || index >= len(d.cachedFlatItems) || d.isDisabledIndex(index) {
		return
	}
	d.CurrentIndex = marks.Const(index)
	d.Activated.Emit(index)
	d.invalidate(facet.DirtyProjection)
}

func (d *NavDrawer) clampedCurrentIndex() int {
	idx := d.CurrentIndex.Get()
	if len(d.cachedFlatItems) == 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx >= len(d.cachedFlatItems) {
		return len(d.cachedFlatItems) - 1
	}
	return idx
}

func (d *NavDrawer) clampedFocusedIndex() int {
	if len(d.cachedFlatItems) == 0 {
		return 0
	}
	if d.focusedIndex < 0 {
		return 0
	}
	if d.focusedIndex >= len(d.cachedFlatItems) {
		return len(d.cachedFlatItems) - 1
	}
	return d.focusedIndex
}

func (d *NavDrawer) clampIndices() {
	if len(d.cachedFlatItems) == 0 {
		d.CurrentIndex = marks.Const(0)
		d.focusedIndex = 0
		return
	}
	ci := d.CurrentIndex.Get()
	if ci < 0 || ci >= len(d.cachedFlatItems) {
		d.CurrentIndex = marks.Const(0)
	}
	if d.focusedIndex < 0 || d.focusedIndex >= len(d.cachedFlatItems) {
		d.focusedIndex = ci
	}
	if d.isDisabledIndex(d.focusedIndex) {
		d.focusedIndex = d.firstEnabledIndex()
	}
}

func (d *NavDrawer) isDisabledIndex(index int) bool {
	if index < 0 || index >= len(d.cachedFlatItems) {
		return true
	}
	return d.Disabled.Get() || d.cachedFlatItems[index].Disabled
}

func (d *NavDrawer) indexAt(p gfx.Point) int {
	for i := range d.cachedItemBounds {
		if d.cachedItemBounds[i].Contains(p) {
			return i
		}
	}
	return -1
}

func (d *NavDrawer) pointInFocusRing(p gfx.Point) bool {
	if !d.focusedVisible || len(d.cachedItemBounds) == 0 {
		return false
	}
	idx := d.clampedFocusedIndex()
	if idx < 0 || idx >= len(d.cachedItemBounds) {
		return false
	}
	active := d.cachedItemBounds[idx]
	if active.IsEmpty() || !active.Contains(p) {
		return false
	}
	ring := maxFloat(1, active.Height()*0.08)
	inner := active.Inset(ring, ring)
	if inner.IsEmpty() {
		return true
	}
	return !inner.Contains(p)
}

func (d *NavDrawer) cursorShape() facet.CursorShape {
	if d.Disabled.Get() {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (d *NavDrawer) cursorForItem(index int) facet.CursorShape {
	if d.Disabled.Get() || d.isDisabledIndex(index) {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (d *NavDrawer) rebuildFlatItems() {
	d.cachedFlatItems = d.cachedFlatItems[:0]
	d.cachedFlatSectionIndex = d.cachedFlatSectionIndex[:0]
	for si := range d.Sections {
		for ii := range d.Sections[si].Items {
			d.cachedFlatItems = append(d.cachedFlatItems, d.Sections[si].Items[ii])
			d.cachedFlatSectionIndex = append(d.cachedFlatSectionIndex, si)
		}
	}
}

func (d *NavDrawer) drawerWidth(resolved theme.ResolvedContext) float32 {
	width := maxFloat(resolved.Density.Scale(280), resolved.Density.Scale(320))
	for _, item := range d.cachedFlatItems {
		if w := resolved.Density.Scale(56) + float32(len(item.Label))*resolved.Density.Scale(8); w > width {
			width = w
		}
	}
	return width
}

func (d *NavDrawer) resolveIcon(runtime any, ref string) (runtimepkg.IconAsset, bool) {
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

func (d *NavDrawer) newShaper(runtime any) *text.Shaper {
	registry := d.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (d *NavDrawer) fontRegistry(runtime any) *text.FontRegistry {
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

func (d *NavDrawer) iconCommands(asset runtimepkg.IconAsset, bounds gfx.Rect, material theme.Material) []gfx.Command {
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

type navDrawerGroupPolicy struct {
	drawer *NavDrawer
}

func (navDrawerGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }
func (p navDrawerGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.drawer == nil || len(children) == 0 {
		return facet.GroupMeasureResult{}, nil
	}
	ordered := orderedNavDrawerChildren(children)
	width := float32(0)
	height := float32(0)
	for i, idx := range ordered {
		child := children[idx]
		if child.Layout == nil {
			continue
		}
		size := child.Layout.MeasuredSize
		if size == (gfx.Size{}) {
			size = child.Layout.Measure(ctx.MeasureContext, facet.Constraints{}).Size
		}
		if size.W > width {
			width = size.W
		}
		height += size.H
		if i < len(ordered)-1 {
			height += p.drawer.cachedSectionGap
		}
	}
	return facet.GroupMeasureResult{Size: gfx.Size{W: width, H: height}}, nil
}
func (p navDrawerGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.drawer == nil || len(children) == 0 {
		return nil, nil
	}
	ordered := orderedNavDrawerChildren(children)
	y := ctx.Bounds.Min.Y
	arranged := make([]facet.ArrangedGroupChild, 0, len(ordered))
	for i, idx := range ordered {
		child := children[idx]
		if child.Layout == nil {
			continue
		}
		size := child.Layout.MeasuredSize
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
			y += p.drawer.cachedSectionGap
		}
	}
	return arranged, nil
}

func orderedNavDrawerChildren(children []facet.GroupChild) []int {
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
