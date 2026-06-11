package navigation

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
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
	paginationMarkIDRoot             facet.MarkID = 1
	paginationMarkIDPageItems        facet.MarkID = 2
	paginationMarkIDPreviousButton   facet.MarkID = 3
	paginationMarkIDNextButton       facet.MarkID = 4
	paginationMarkIDEllipsis         facet.MarkID = 5
	paginationMarkIDCurrentIndicator facet.MarkID = 6
	paginationMarkIDFocusRing        facet.MarkID = 7
)

// PaginationItem describes one pageable destination.
type PaginationItem struct {
	Key      string
	Label    string
	Disabled bool
}

type paginationChildKind uint8

const (
	paginationChildPrev paginationChildKind = iota
	paginationChildPage
	paginationChildEllipsis
	paginationChildNext
)

type paginationChild struct {
	facet.Facet

	layoutRole facet.LayoutRole

	parent *Pagination
	kind   paginationChildKind
	index  int
	label  string
}

var _ facet.FacetImpl = (*paginationChild)(nil)

func newPaginationChild(parent *Pagination, kind paginationChildKind, index int, label string) *paginationChild {
	c := &paginationChild{
		Facet:  facet.NewFacet(),
		parent: parent,
		kind:   kind,
		index:  index,
		label:  strings.TrimSpace(label),
	}
	c.layoutRole.Parent = facet.GroupParentContract{Kind: facet.GroupLayoutNone}
	c.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := c.measure(ctx, constraints)
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
	c.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		size := c.measure(ctx, constraints)
		c.layoutRole.MeasuredSize = size
		c.layoutRole.MeasuredResult = facet.MeasureResult{
			Size: size,
			Intrinsic: facet.IntrinsicSize{
				Min:       size,
				Preferred: size,
				Max:       size,
			},
			Constraints: constraints,
		}
		return c.layoutRole.MeasuredResult
	}
	c.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		c.layoutRole.ArrangedBounds = bounds
	}
	c.AddRole(&c.layoutRole)
	return c
}

func (c *paginationChild) Base() *facet.Facet {
	c.BindImpl(c)
	return &c.Facet
}

func (c *paginationChild) OnAttach(ctx facet.AttachContext) {}
func (c *paginationChild) OnActivate()                      {}
func (c *paginationChild) OnDeactivate()                    {}
func (c *paginationChild) OnDetach()                        {}

func (c *paginationChild) measure(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	if c == nil || c.parent == nil {
		return gfx.Size{}
	}
	return c.parent.measureChildSize(ctx, constraints, c.kind, c.index, c.label)
}

// Pagination implements the navigation.pagination canonical mark.
type Pagination struct {
	marks.Core

	Label        marks.Binding[string]
	Items        []PaginationItem
	CurrentIndex marks.Binding[int]
	Disabled     marks.Binding[bool]

	Activated signal.Signal[int]

	textRole facet.TextRole

	hoveredEntryIndex int
	pressedEntryIndex int
	focusedEntryIndex int
	focusedVisible    bool
	focusFromPointer  bool

	cachedTokens           theme.Tokens
	cachedRecipe           shared.PaginationSlots
	cachedRootBounds       gfx.Rect
	cachedContentBounds    gfx.Rect
	cachedEntryBounds      []gfx.Rect
	cachedVisibleChildren  []*paginationChild
	cachedVisibleKinds     []paginationChildKind
	cachedVisibleIndices   []int
	cachedEntryLabels      []string
	cachedEntryLayouts     []*text.TextLayout
	cachedEntryStyles      []text.TextStyle
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedRadius           float32
	cachedWritingDirection facet.WritingDirection

	prevChild     *paginationChild
	nextChild     *paginationChild
	ellipsisLeft  *paginationChild
	ellipsisRight *paginationChild
	pageChildren  []*paginationChild
}

var _ facet.FacetImpl = (*Pagination)(nil)
var _ layout.AnchorExporter = (*Pagination)(nil)
var _ marks.Mark = (*Pagination)(nil)

// NewPagination constructs a navigation.pagination mark with canonical defaults.
func NewPagination(label string, items []PaginationItem) *Pagination {
	p := &Pagination{
		Label:             marks.Const(label),
		CurrentIndex:      marks.Const(0),
		Disabled:          marks.Const(false),
		focusedEntryIndex: 0,
	}
	p.Facet = facet.NewFacet()
	p.AddBinding(p.Label)
	p.AddBinding(p.CurrentIndex)
	p.AddBinding(p.Disabled)
	p.SetItems(items)
	p.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearHorizontal,
		Policy:   paginationGroupPolicy{pagination: p},
		Children: p,
	}
	p.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := p.measureIntrinsic(ctx, constraints)
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
	p.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return p.measure(ctx, constraints)
	}
	p.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		p.Layout.ArrangedBounds = bounds
		p.arrange(ctx, bounds)
	}
	p.Hit.OnHitTest = func(pt gfx.Point) facet.HitResult { return p.hitTest(pt) }
	p.Input.OnPointer = func(e facet.PointerEvent) bool { return p.onPointer(e) }
	p.Input.OnKey = func(e facet.KeyEvent) bool { return p.onKey(e) }
	p.Focus.Focusable = func() bool { return !p.Disabled.Get() && len(p.Items) > 0 }
	p.Focus.TabIndex = 0
	p.Focus.OnFocusGained = func() { p.onFocusGained() }
	p.Focus.OnFocusLost = func() { p.onFocusLost() }
	p.textRole.IMEEnabled = false
	p.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return p.buildCommands(p.Layout.ArrangedBounds, ctx.Runtime)
	}
	p.RegisterRoles()
	p.AddRole(&p.textRole)
	p.rebuildChildren()
	return p
}

// Base satisfies facet.FacetImpl.
func (p *Pagination) Base() *facet.Facet {
	p.BindImpl(p)
	return &p.Facet
}

// Descriptor satisfies marks.Mark.
func (p *Pagination) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: markTypeNavigation, TypeName: "pagination"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (p *Pagination) AccessibilityRole() string { return markTypeNavigation }

// AccessibleName reports the semantic name source required by the spec.
func (p *Pagination) AccessibleName() string { return p.Label.Get() }

// SetItems updates the page list.
func (p *Pagination) SetItems(items []PaginationItem) {
	if p == nil {
		return
	}
	next := append([]PaginationItem(nil), items...)
	for i := range next {
		next[i].Key = strings.TrimSpace(next[i].Key)
		next[i].Label = strings.TrimSpace(next[i].Label)
	}
	p.Items = next
	p.rebuildChildren()
	p.clampIndices()
	p.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// ExportAnchors publishes the pagination anchor set.
func (p *Pagination) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if p == nil {
		return nil
	}
	bounds := p.Layout.ArrangedBounds
	out := p.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	if idx := p.currentVisibleEntryIndex(); idx >= 0 && idx < len(p.cachedEntryBounds) {
		rect := p.cachedEntryBounds[idx]
		if !rect.IsEmpty() {
			out["baseline"] = gfx.Point{X: rect.Min.X, Y: rect.Min.Y}
		}
	}
	return out
}

// Children returns the facet's immediate child list.
func (p *Pagination) Children() []facet.GroupChild {
	if p == nil {
		return nil
	}
	if len(p.cachedVisibleChildren) == 0 {
		p.rebuildChildren()
	}
	out := make([]facet.GroupChild, 0, len(p.cachedVisibleChildren))
	for i, child := range p.cachedVisibleChildren {
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
			MarkID:  p.markIDForChild(child),
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

func (p *Pagination) OnAttach(ctx facet.AttachContext) { p.Core.OnAttach() }
func (p *Pagination) OnActivate()                      { p.Core.OnActivate() }
func (p *Pagination) OnDeactivate()                    { p.Core.OnDeactivate() }
func (p *Pagination) OnDetach() {
	p.Core.OnDetach()
	p.cachedTokens = theme.Tokens{}
	p.cachedRecipe = shared.PaginationSlots{}
	p.cachedRootBounds = gfx.Rect{}
	p.cachedContentBounds = gfx.Rect{}
	p.cachedEntryBounds = nil
	p.cachedVisibleChildren = nil
	p.cachedVisibleKinds = nil
	p.cachedVisibleIndices = nil
	p.cachedEntryLabels = nil
	p.cachedEntryLayouts = nil
	p.cachedEntryStyles = nil
	p.cachedPadX = 0
	p.cachedPadY = 0
	p.cachedGap = 0
	p.cachedRadius = 0
}

func (p *Pagination) invalidate(flags facet.DirtyFlags) {
	if p == nil {
		return
	}
	p.Invalidate(flags)
}

func (p *Pagination) rebuildChildren() {
	if p == nil {
		return
	}
	if p.prevChild == nil {
		p.prevChild = newPaginationChild(p, paginationChildPrev, -1, "\u2039")
	}
	if p.nextChild == nil {
		p.nextChild = newPaginationChild(p, paginationChildNext, -1, "\u203a")
	}
	if p.ellipsisLeft == nil {
		p.ellipsisLeft = newPaginationChild(p, paginationChildEllipsis, -1, "\u2026")
	}
	if p.ellipsisRight == nil {
		p.ellipsisRight = newPaginationChild(p, paginationChildEllipsis, -1, "\u2026")
	}
	if len(p.pageChildren) != len(p.Items) {
		p.pageChildren = make([]*paginationChild, len(p.Items))
	}
	for i := range p.Items {
		if p.pageChildren[i] == nil {
			p.pageChildren[i] = newPaginationChild(p, paginationChildPage, i, p.Items[i].Label)
		}
		p.pageChildren[i].index = i
		p.pageChildren[i].label = p.Items[i].Label
	}
}

func (p *Pagination) visibleEntries() []*paginationChild {
	if p == nil || len(p.Items) == 0 {
		return nil
	}
	p.rebuildChildren()
	out := make([]*paginationChild, 0, len(p.Items)+4)
	out = append(out, p.prevChild)
	indices := p.visiblePageIndices()
	lastAddedPage := -1
	for _, idx := range indices {
		if idx < 0 {
			if lastAddedPage >= 0 && lastAddedPage != len(p.Items)-1 {
				out = append(out, p.ellipsisLeft)
			}
			continue
		}
		if idx == len(p.Items)-1 && lastAddedPage >= 0 && lastAddedPage != idx-1 {
			out = append(out, p.ellipsisRight)
		}
		out = append(out, p.pageChildren[idx])
		lastAddedPage = idx
	}
	out = append(out, p.nextChild)
	return out
}

func (p *Pagination) visiblePageIndices() []int {
	n := len(p.Items)
	if n <= 7 {
		out := make([]int, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, i)
		}
		return out
	}
	current := p.clampedCurrentIndex()
	if current <= 2 {
		return []int{0, 1, 2, 3, -1, n - 1}
	}
	if current >= n-3 {
		return []int{0, -1, n - 4, n - 3, n - 2, n - 1}
	}
	return []int{0, -1, current - 1, current, current + 1, -1, n - 1}
}

func (p *Pagination) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return p.measure(ctx, constraints).Size
}

func (p *Pagination) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uinav.ResolvePaginationRecipe(style)
	p.cachedTokens = resolved.TokenSet()
	p.cachedRecipe = slots
	p.cachedWritingDirection = ctx.WritingDirection
	p.cachedPadX = paginationMaxFloat(resolved.Density.Scale(10), float32(resolved.Spacing(theme.SpacingS)))
	p.cachedPadY = paginationMaxFloat(resolved.Density.Scale(6), float32(resolved.Spacing(theme.SpacingXS)))
	p.cachedGap = float32(resolved.Spacing(theme.SpacingS))
	p.cachedRadius = float32(resolved.Radius(theme.RadiusM))
	p.rebuildChildren()
	visible := p.visibleEntries()
	p.cachedVisibleChildren = visible
	p.cachedVisibleKinds = p.cachedVisibleKinds[:0]
	p.cachedVisibleIndices = p.cachedVisibleIndices[:0]
	p.cachedEntryLabels = p.cachedEntryLabels[:0]
	p.cachedEntryLayouts = p.cachedEntryLayouts[:0]
	p.cachedEntryStyles = p.cachedEntryStyles[:0]
	p.cachedEntryBounds = p.cachedEntryBounds[:0]
	shaper := p.newShaper(ctx.Runtime)
	totalW := float32(0)
	maxH := float32(0)
	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = resolved.Density.Scale(480)
	}
	for i, child := range visible {
		if child == nil {
			continue
		}
		size, layout := p.measureVisibleChild(ctx, constraints, child, shaper, maxWidth)
		child.layoutRole.MeasuredSize = size
		child.layoutRole.MeasuredResult = facet.MeasureResult{
			Size:        size,
			Intrinsic:   facet.IntrinsicSize{Min: size, Preferred: size, Max: size},
			Constraints: constraints,
		}
		p.cachedVisibleKinds = append(p.cachedVisibleKinds, child.kind)
		p.cachedVisibleIndices = append(p.cachedVisibleIndices, child.index)
		p.cachedEntryLabels = append(p.cachedEntryLabels, child.label)
		p.cachedEntryLayouts = append(p.cachedEntryLayouts, layout)
		p.cachedEntryStyles = append(p.cachedEntryStyles, resolved.TextStyle(theme.TextLabelM))
		p.cachedEntryBounds = append(p.cachedEntryBounds, gfx.RectFromXYWH(0, 0, size.W, size.H))
		if i > 0 {
			totalW += p.cachedGap
		}
		totalW += size.W
		maxH = paginationMaxFloat(maxH, size.H)
	}
	measured := constraints.Constrain(gfx.Size{W: totalW, H: maxH})
	p.Layout.MeasuredSize = measured
	p.Layout.MeasuredResult = facet.MeasureResult{
		Size: measured,
		Intrinsic: facet.IntrinsicSize{
			Min:       measured,
			Preferred: measured,
			Max:       measured,
		},
		Constraints: constraints,
	}
	return p.Layout.MeasuredResult
}

func (p *Pagination) measureVisibleChild(ctx facet.MeasureContext, constraints facet.Constraints, child *paginationChild, shaper *text.Shaper, maxWidth float32) (gfx.Size, *text.TextLayout) {
	if child == nil {
		return gfx.Size{}, nil
	}
	label := childLabelForKind(p, child.kind, child.index, child.label)
	style := theme.DefaultResolvedContext().TextStyle(theme.TextLabelM)
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if ok {
		style = resolved.TextStyle(theme.TextLabelM)
	}
	layout := (*text.TextLayout)(nil)
	if shaper != nil && label != "" {
		shaper.SetContentScale(ctx.ContentScale)
		layout = shaper.ShapeTruncated(label, style, maxWidth)
	}
	padX := p.cachedPadX
	padY := p.cachedPadY
	if child.kind == paginationChildEllipsis {
		padX = paginationMaxFloat(0, padX*0.5)
	}
	textW := float32(0)
	textH := float32(0)
	if layout != nil {
		textW = layout.Bounds.Width()
		textH = layout.Bounds.Height()
	}
	minSide := resolved.Density.Scale(32)
	if child.kind == paginationChildEllipsis {
		minSide = resolved.Density.Scale(20)
	}
	size := gfx.Size{W: paginationMaxFloat(minSide, textW+padX*2), H: paginationMaxFloat(minSide, textH+padY*2)}
	return size, layout
}

func (p *Pagination) measureChildSize(ctx facet.MeasureContext, constraints facet.Constraints, kind paginationChildKind, index int, label string) gfx.Size {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := resolved.TextStyle(theme.TextLabelM)
	shaper := p.newShaper(ctx.Runtime)
	content := childLabelForKind(p, kind, index, label)
	layout := (*text.TextLayout)(nil)
	if shaper != nil && content != "" {
		shaper.SetContentScale(ctx.ContentScale)
		layout = shaper.ShapeTruncated(content, style, constraints.MaxSize.W)
	}
	textW := float32(0)
	textH := float32(0)
	if layout != nil {
		textW = layout.Bounds.Width()
		textH = layout.Bounds.Height()
	}
	padX := p.cachedPadX
	padY := p.cachedPadY
	if padX == 0 {
		padX = paginationMaxFloat(resolved.Density.Scale(10), float32(resolved.Spacing(theme.SpacingS)))
	}
	if padY == 0 {
		padY = paginationMaxFloat(resolved.Density.Scale(6), float32(resolved.Spacing(theme.SpacingXS)))
	}
	minSide := resolved.Density.Scale(32)
	if kind == paginationChildEllipsis {
		minSide = resolved.Density.Scale(20)
	}
	return gfx.Size{W: paginationMaxFloat(minSide, textW+padX*2), H: paginationMaxFloat(minSide, textH+padY*2)}
}

func (p *Pagination) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	p.cachedRootBounds = bounds
	p.cachedContentBounds = gfx.Rect{}
	p.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	rtl := p.cachedWritingDirection == facet.WritingDirectionRTL
	x := bounds.Min.X
	if rtl {
		x = bounds.Max.X
	}
	contentTop := bounds.Min.Y
	contentH := bounds.Height()
	for i, child := range p.cachedVisibleChildren {
		if child == nil {
			continue
		}
		size := gfx.Size{W: p.cachedEntryBounds[i].Width(), H: p.cachedEntryBounds[i].Height()}
		y := contentTop + paginationMaxFloat(0, (contentH-size.H)*0.5)
		if rtl {
			x -= size.W
			rect := gfx.RectFromXYWH(x, y, size.W, size.H)
			child.layoutRole.ArrangedBounds = rect
			p.cachedEntryBounds[i] = rect
			x -= p.cachedGap
			if p.cachedContentBounds.IsEmpty() {
				p.cachedContentBounds = rect
			} else {
				p.cachedContentBounds = p.cachedContentBounds.Union(rect)
			}
			continue
		}
		rect := gfx.RectFromXYWH(x, y, size.W, size.H)
		child.layoutRole.ArrangedBounds = rect
		p.cachedEntryBounds[i] = rect
		x += size.W + p.cachedGap
		if p.cachedContentBounds.IsEmpty() {
			p.cachedContentBounds = rect
		} else {
			p.cachedContentBounds = p.cachedContentBounds.Union(rect)
		}
	}
}

func (p *Pagination) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if p == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := p.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	cmds := make([]gfx.Command, 0, 32)
	if !paginationIsTransparentMaterial(slots.Root.Resolve(theme.StateDefault, tokens)) {
		cmds = append(cmds, paginationMaterialCommands(gfx.RectPath(bounds), slots.Root.Resolve(theme.StateDefault, tokens))...)
	}
	for i, child := range p.cachedVisibleChildren {
		if child == nil {
			continue
		}
		rect := p.cachedEntryBounds[i]
		label := p.cachedEntryLabels[i]
		layout := p.cachedEntryLayouts[i]
		state := p.childInteractionState(i)
		switch child.kind {
		case paginationChildPrev, paginationChildNext:
			mat := slots.Nav.Resolve(state, tokens)
			cmds = append(cmds, p.paintTextOnly(rect, mat, label, layout)...)
		case paginationChildEllipsis:
			mat := slots.Separator.Resolve(state, tokens)
			cmds = append(cmds, p.paintTextOnly(rect, mat, label, layout)...)
		case paginationChildPage:
			mat := slots.Page.Resolve(state, tokens)
			if child.index == p.clampedCurrentIndex() {
				currentMat := slots.Current.Resolve(state, tokens)
				cmds = append(cmds, p.paintCurrentChip(rect, currentMat)...)
				cmds = append(cmds, p.paintTextOnly(rect, theme.FromToken(tokens.Color.OnPrimary), label, layout)...)
				break
			}
			cmds = append(cmds, p.paintTextOnly(rect, mat, label, layout)...)
		}
		if child.index == p.focusedEntryIndex && p.focusedVisible && !paginationIsTransparentMaterial(slots.FocusRing.Resolve(theme.StateFocused, tokens)) {
			inset := paginationMaxFloat(1, rect.Height()*0.08)
			cmds = append(cmds, paginationMaterialCommands(gfx.RoundedRectPath(rect.Inset(-inset, -inset), p.cachedRadius+inset), slots.FocusRing.Resolve(theme.StateFocused, tokens))...)
		}
	}
	return cmds
}

func (p *Pagination) paintCurrentChip(bounds gfx.Rect, material theme.Material) []gfx.Command {
	if bounds.IsEmpty() || paginationIsTransparentMaterial(material) {
		return nil
	}
	cmds := paginationMaterialCommands(gfx.RoundedRectPath(bounds, p.cachedRadius), material)
	return cmds
}

func (p *Pagination) paintTextOnly(bounds gfx.Rect, material theme.Material, label string, layout *text.TextLayout) []gfx.Command {
	if bounds.IsEmpty() || paginationIsTransparentMaterial(material) {
		return nil
	}
	return p.paintText(bounds, material, label, layout)
}

func (p *Pagination) paintText(bounds gfx.Rect, material theme.Material, label string, layout *text.TextLayout) []gfx.Command {
	if bounds.IsEmpty() || paginationIsTransparentMaterial(material) || layout == nil {
		return nil
	}
	origin := gfx.RectFromXYWH(
		bounds.Min.X+(bounds.Width()-layout.Bounds.Width())*0.5,
		bounds.Min.Y+(bounds.Height()-layout.Bounds.Height())*0.5,
		bounds.Width(),
		bounds.Height(),
	)
	cmds := primitive.TextLayoutCommands(layout, origin, gfx.SolidBrush(paginationMaterialColor(material)))
	if len(cmds) == 0 && label != "" {
		_ = label
	}
	return cmds
}

func (p *Pagination) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.PaginationSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: p.cachedTokens}, p.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, p.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uinav.ResolvePaginationRecipe(style)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: p.cachedTokens}, p.cachedRecipe
}

func (p *Pagination) childInteractionState(entryIndex int) theme.InteractionState {
	if p.Disabled.Get() {
		return theme.StateDisabled
	}
	switch {
	case entryIndex == p.pressedEntryIndex:
		return theme.StatePressed
	case entryIndex == p.hoveredEntryIndex:
		return theme.StateHover
	case entryIndex == p.focusedEntryIndex && p.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (p *Pagination) hitTest(pt gfx.Point) facet.HitResult {
	if p == nil || p.Layout.ArrangedBounds.IsEmpty() || !p.Layout.ArrangedBounds.Contains(pt) {
		return facet.HitResult{}
	}
	cursor := facet.CursorPointer
	if p.Disabled.Get() {
		cursor = facet.CursorDefault
	}
	if p.focusedVisible && p.currentVisibleEntryIndex() >= 0 {
		idx := p.currentVisibleEntryIndex()
		if idx < len(p.cachedEntryBounds) && p.cachedEntryBounds[idx].Contains(pt) {
			return facet.HitResult{Hit: true, MarkID: paginationMarkIDFocusRing, Cursor: cursor}
		}
	}
	for i, rect := range p.cachedEntryBounds {
		if !rect.Contains(pt) {
			continue
		}
		switch p.cachedVisibleKinds[i] {
		case paginationChildPrev:
			return facet.HitResult{Hit: true, MarkID: paginationMarkIDPreviousButton, Cursor: cursor}
		case paginationChildNext:
			return facet.HitResult{Hit: true, MarkID: paginationMarkIDNextButton, Cursor: cursor}
		case paginationChildEllipsis:
			return facet.HitResult{Hit: true, MarkID: paginationMarkIDEllipsis, Cursor: facet.CursorDefault}
		case paginationChildPage:
			if p.cachedVisibleIndices[i] == p.clampedCurrentIndex() {
				return facet.HitResult{Hit: true, MarkID: paginationMarkIDCurrentIndicator, Cursor: cursor}
			}
			return facet.HitResult{Hit: true, MarkID: paginationMarkIDPageItems, Cursor: cursor}
		}
	}
	return facet.HitResult{Hit: true, MarkID: paginationMarkIDRoot, Cursor: cursor}
}

func (p *Pagination) onPointer(e facet.PointerEvent) bool {
	if p.Disabled.Get() {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		if idx := p.entryIndexAt(e.Position); idx >= 0 {
			p.hoveredEntryIndex = idx
			p.invalidate(facet.DirtyProjection)
			return true
		}
	case platform.PointerLeave:
		p.hoveredEntryIndex = -1
		if p.pressedEntryIndex < 0 {
			p.focusFromPointer = false
		}
		p.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		if idx := p.entryIndexAt(e.Position); idx >= 0 {
			p.hoveredEntryIndex = idx
			p.pressedEntryIndex = idx
			p.focusFromPointer = true
			p.focusedVisible = false
			p.invalidate(facet.DirtyProjection)
			return true
		}
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		idx := p.entryIndexAt(e.Position)
		wasPressed := p.pressedEntryIndex >= 0
		if wasPressed && idx == p.pressedEntryIndex {
			p.activateEntry(idx)
		}
		p.pressedEntryIndex = -1
		p.invalidate(facet.DirtyProjection)
		return wasPressed
	}
	return false
}

func (p *Pagination) onKey(e facet.KeyEvent) bool {
	if p.Disabled.Get() {
		return false
	}
	switch e.Key {
	case platform.KeyLeft:
		if e.Kind == platform.KeyPress {
			p.moveCurrent(-1)
			return true
		}
	case platform.KeyRight:
		if e.Kind == platform.KeyPress {
			p.moveCurrent(1)
			return true
		}
	case platform.KeyHome:
		if e.Kind == platform.KeyPress {
			p.setCurrentIndex(0)
			p.Activated.Emit(0)
			return true
		}
	case platform.KeyEnd:
		if e.Kind == platform.KeyPress {
			if len(p.Items) > 0 {
				last := len(p.Items) - 1
				p.setCurrentIndex(last)
				p.Activated.Emit(last)
				return true
			}
		}
	case platform.KeyPageUp:
		if e.Kind == platform.KeyPress {
			p.moveCurrent(-1)
			return true
		}
	case platform.KeyPageDown:
		if e.Kind == platform.KeyPress {
			p.moveCurrent(1)
			return true
		}
	case platform.KeySpace, platform.KeyEnter:
		switch e.Kind {
		case platform.KeyPress, platform.KeyRepeat:
			p.focusedVisible = true
			p.invalidate(facet.DirtyProjection)
			return true
		case platform.KeyRelease:
			if p.currentVisibleEntryIndex() >= 0 {
				p.Activated.Emit(p.clampedCurrentIndex())
			}
			return true
		}
	}
	return false
}

func (p *Pagination) setCurrentIndex(index int) {
	if p == nil {
		return
	}
	if index < 0 {
		index = 0
	}
	if len(p.Items) > 0 && index >= len(p.Items) {
		index = len(p.Items) - 1
	}
	if len(p.Items) == 0 {
		index = 0
	}
	p.CurrentIndex = marks.Const(index)
	p.clampIndices()
	p.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func (p *Pagination) onFocusGained() {
	p.focusedVisible = !p.focusFromPointer
	p.focusFromPointer = false
	p.focusedEntryIndex = p.currentVisibleEntryIndex()
	p.invalidate(facet.DirtyProjection)
}

func (p *Pagination) onFocusLost() {
	p.focusedVisible = false
	p.pressedEntryIndex = -1
	p.focusFromPointer = false
	p.invalidate(facet.DirtyProjection)
}

func (p *Pagination) activateEntry(entryIndex int) {
	if entryIndex < 0 || entryIndex >= len(p.cachedVisibleChildren) {
		return
	}
	switch p.cachedVisibleKinds[entryIndex] {
	case paginationChildPrev:
		p.moveCurrent(-1)
	case paginationChildNext:
		p.moveCurrent(1)
	case paginationChildPage:
		if idx := p.cachedVisibleIndices[entryIndex]; idx >= 0 {
			p.setCurrentIndex(idx)
			p.Activated.Emit(idx)
			p.focusedEntryIndex = entryIndex
		}
	}
}

func (p *Pagination) moveCurrent(delta int) {
	if len(p.Items) == 0 {
		return
	}
	next := p.clampedCurrentIndex() + delta
	if next < 0 {
		next = 0
	}
	if next >= len(p.Items) {
		next = len(p.Items) - 1
	}
	p.setCurrentIndex(next)
	p.Activated.Emit(next)
	p.focusedEntryIndex = p.currentVisibleEntryIndex()
}

func (p *Pagination) entryIndexAt(pt gfx.Point) int {
	for i, rect := range p.cachedEntryBounds {
		if rect.Contains(pt) {
			return i
		}
	}
	return -1
}

func (p *Pagination) currentVisibleEntryIndex() int {
	current := p.clampedCurrentIndex()
	for i, idx := range p.cachedVisibleIndices {
		if idx == current && p.cachedVisibleKinds[i] == paginationChildPage {
			return i
		}
	}
	return -1
}

func (p *Pagination) clampedCurrentIndex() int {
	if p == nil {
		return 0
	}
	ci := p.CurrentIndex.Get()
	if ci < 0 {
		return 0
	}
	if len(p.Items) == 0 {
		return 0
	}
	if ci >= len(p.Items) {
		return len(p.Items) - 1
	}
	return ci
}

func (p *Pagination) clampIndices() {
	if p == nil {
		return
	}
	p.CurrentIndex = marks.Const(p.clampedCurrentIndex())
	if p.focusedEntryIndex < 0 || p.focusedEntryIndex >= len(p.cachedVisibleChildren) {
		p.focusedEntryIndex = p.currentVisibleEntryIndex()
	}
}

func (p *Pagination) markIDForChild(child *paginationChild) facet.MarkID {
	if child == nil {
		return paginationMarkIDRoot
	}
	switch child.kind {
	case paginationChildPrev:
		return paginationMarkIDPreviousButton
	case paginationChildNext:
		return paginationMarkIDNextButton
	case paginationChildEllipsis:
		return paginationMarkIDEllipsis
	case paginationChildPage:
		if child.index == p.clampedCurrentIndex() {
			return paginationMarkIDCurrentIndicator
		}
		return paginationMarkIDPageItems
	default:
		return paginationMarkIDRoot
	}
}

func (p *Pagination) newShaper(runtime any) *text.Shaper {
	registry := p.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (p *Pagination) fontRegistry(runtime any) *text.FontRegistry {
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

func childLabelForKind(p *Pagination, kind paginationChildKind, index int, label string) string {
	switch kind {
	case paginationChildPrev:
		return "\u2039"
	case paginationChildNext:
		return "\u203a"
	case paginationChildEllipsis:
		return "\u2026"
	case paginationChildPage:
		if label != "" {
			return label
		}
		if index >= 0 && index < len(p.Items) {
			return p.Items[index].Label
		}
	}
	return label
}

func paginationMaterialCommands(path gfx.Path, material theme.Material) []gfx.Command {
	return theme.MaterialCommands(path, material)
}

func paginationMaterialColor(material theme.Material) gfx.Color {
	return theme.MaterialColor(material)
}

func paginationIsTransparentMaterial(material theme.Material) bool {
	return theme.IsTransparentMaterial(material)
}

func paginationMaxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

type paginationGroupPolicy struct {
	pagination *Pagination
}

func (paginationGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }
func (paginationGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}
func (paginationGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}
