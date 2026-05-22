package action

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/platform"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

type commandPaletteResultsGroup struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	focusRole      facet.FocusRole
	textRole       facet.TextRole
	viewportRole   facet.ViewportRole

	Activated signal.Signal[int]

	Label      string
	EmptyState string
	ItemVariant uiinput.ListItemVariant
	Disabled   bool

	parent *CommandPalette

	rows           []*selection.ListItem
	rowRects       []gfx.Rect
	activeIndex    int
	scrollOffset   float32
	cachedBounds   gfx.Rect
	cachedContent  gfx.Rect
	cachedEmpty    *primitive.Text
	cachedRowGap   float32
	cachedWriting  facet.WritingDirection
}

var _ facet.FacetImpl = (*commandPaletteResultsGroup)(nil)
var _ layout.AnchorExporter = (*commandPaletteResultsGroup)(nil)

func newCommandPaletteResultsGroup(parent *CommandPalette) *commandPaletteResultsGroup {
	g := &commandPaletteResultsGroup{
		Facet:        facet.NewFacet(),
		EmptyState:    "No matching commands",
		ItemVariant:   uiinput.ListItemStandard,
		parent:        parent,
		Activated:     signal.NewSignal[int]("command_palette_results_activated"),
		activeIndex:   -1,
		cachedRowGap:  0,
	}
	g.layoutRole.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   commandPaletteResultsGroupPolicy{group: g},
		Children: g,
		Overflow: facet.OverflowScroll,
		Clipping: facet.GroupClipBounds,
	}
	g.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := g.measure(ctx, constraints)
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
	g.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		size := g.measure(ctx, constraints)
		return facet.MeasureResult{Size: size, Intrinsic: facet.IntrinsicSize{Min: size, Preferred: size, Max: size}, Constraints: constraints}
	}
	g.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		g.layoutRole.ArrangedBounds = bounds
		g.arrange(ctx, bounds)
	}
	g.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := g.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	g.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := g.buildCommands(g.layoutRole.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	g.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult { return g.hitTest(p) }
	g.inputRole.OnPointer = func(e facet.PointerEvent) bool { return g.onPointer(e) }
	g.inputRole.OnScroll = func(e facet.ScrollEvent) bool { return g.onScroll(e) }
	g.inputRole.OnKey = func(e facet.KeyEvent) bool { return g.onKey(e) }
	g.inputRole.OnDismiss = func(e facet.DismissEvent) bool { return g.onDismiss(e) }
	g.focusRole.Focusable = func() bool { return !g.Disabled && len(g.rows) > 0 }
	g.focusRole.TabIndex = 1
	g.focusRole.OnFocusGained = func() { g.onFocusGained() }
	g.focusRole.OnFocusLost = func() { g.onFocusLost() }
	g.textRole.IMEEnabled = false
	g.viewportRole.Transform = gfx.Identity()
	g.AddRole(&g.layoutRole)
	g.AddRole(&g.renderRole)
	g.AddRole(&g.projectionRole)
	g.AddRole(&g.hitRole)
	g.AddRole(&g.inputRole)
	g.AddRole(&g.focusRole)
	g.AddRole(&g.textRole)
	g.AddRole(&g.viewportRole)
	return g
}

func (g *commandPaletteResultsGroup) Base() *facet.Facet {
	g.Facet.BindImpl(g)
	return &g.Facet
}

func (g *commandPaletteResultsGroup) AccessibilityRole() string { return "listbox" }

func (g *commandPaletteResultsGroup) AccessibleName() string {
	if g == nil {
		return ""
	}
	return strings.TrimSpace(g.Label)
}

func (g *commandPaletteResultsGroup) SetLabel(label string) {
	if g == nil || g.Label == label {
		return
	}
	g.Label = label
	g.invalidate(facet.DirtyProjection)
}

func (g *commandPaletteResultsGroup) SetEmptyState(text string) {
	if g == nil || g.EmptyState == text {
		return
	}
	g.EmptyState = text
	g.syncRows(g.parent.cachedFiltered, g.activeIndex)
	g.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func (g *commandPaletteResultsGroup) SetDisabled(disabled bool) {
	if g == nil || g.Disabled == disabled {
		return
	}
	g.Disabled = disabled
	if disabled {
		g.focusRole.OnFocusLost()
	}
	g.syncRows(g.parent.cachedFiltered, g.activeIndex)
	g.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

func (g *commandPaletteResultsGroup) SetEntries(entries []runtimepkg.CommandEntry, active int) {
	if g == nil {
		return
	}
	g.syncRows(entries, active)
	g.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func (g *commandPaletteResultsGroup) Children() []facet.GroupChild {
	if g == nil || len(g.rows) == 0 {
		return nil
	}
	out := make([]facet.GroupChild, 0, len(g.rows))
	for i, row := range g.rows {
		if row == nil || row.Base() == nil || row.Base().LayoutRole() == nil {
			continue
		}
		out = append(out, commandPaletteResultsChild(row.Base(), commandPaletteMarkIDResultsList, i))
	}
	return out
}

func (g *commandPaletteResultsGroup) OnAttach(ctx facet.AttachContext) {}
func (g *commandPaletteResultsGroup) OnActivate()                      {}
func (g *commandPaletteResultsGroup) OnDeactivate()                    {}
func (g *commandPaletteResultsGroup) OnDetach()                        {}

func (g *commandPaletteResultsGroup) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if g == nil {
		return nil
	}
	bounds := g.cachedBounds
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
	if len(g.rowRects) > 0 {
		active := clampInt(g.activeIndex, 0, len(g.rowRects)-1)
		rect := g.rowRects[active]
		if !rect.IsEmpty() {
			out["active_item"] = gfx.Point{
				X: rect.Min.X + rect.Width()*0.5,
				Y: rect.Min.Y + rect.Height()*0.5,
			}
		}
	}
	if g.cachedEmpty != nil && g.cachedEmpty.Base() != nil && g.cachedEmpty.Base().LayoutRole() != nil {
		if rect := g.cachedEmpty.Base().LayoutRole().ArrangedBounds; !rect.IsEmpty() {
			out["empty_state"] = gfx.Point{
				X: rect.Min.X + rect.Width()*0.5,
				Y: rect.Min.Y + rect.Height()*0.5,
			}
		}
	}
	return out
}

func (g *commandPaletteResultsGroup) invalidate(flags facet.DirtyFlags) {
	if g == nil {
		return
	}
	g.Base().Invalidate(flags)
}

func (g *commandPaletteResultsGroup) syncRows(entries []runtimepkg.CommandEntry, active int) {
	if g == nil {
		return
	}
	if active < 0 {
		active = -1
	}
	if len(entries) == 0 {
		g.rows = nil
		g.rowRects = nil
		g.activeIndex = -1
		g.cachedContent = gfx.Rect{}
		g.cachedEmpty = primitive.NewText(strings.TrimSpace(g.EmptyState))
		g.cachedEmpty.SetTypography(theme.TextBodyS)
		g.cachedEmpty.SetForeground(theme.ColorTextSecondary)
		g.cachedEmpty.SetOverflow(primitive.TextOverflowTruncate)
		if g.Disabled {
			g.cachedEmpty.SetDisabled(true)
		}
		return
	}
	if active >= len(entries) {
		active = len(entries) - 1
	}
	if g.cachedEmpty != nil {
		g.cachedEmpty = nil
	}
	nextRows := make([]*selection.ListItem, len(entries))
	for i, entry := range entries {
		row := g.rowsAt(entries, i)
		row.SetLabel(commandPaletteDisplayLabel(entry))
		row.SetSupportingText(entry.Shortcut)
		row.SetLeadingIconRef(entry.IconRef)
		row.SetDisabled(g.Disabled || entry.Disabled)
		row.SetActive(i == active)
		row.SetSelected(false)
		row.Variant = g.ItemVariant
		row.ShowContainer = true
		row.ShowLeadingIcon = strings.TrimSpace(entry.IconRef) != ""
		row.ShowSelectionIndicator = false
		row.ShowFocusRing = false
		row.ShowLabel = true
		nextRows[i] = row
	}
	g.rows = nextRows
	g.activeIndex = active
	g.rebindRowSignals()
}

func (g *commandPaletteResultsGroup) rowsAt(entries []runtimepkg.CommandEntry, index int) *selection.ListItem {
	if g == nil {
		return nil
	}
	if index >= 0 && index < len(g.rows) && g.rows[index] != nil {
		return g.rows[index]
	}
	return selection.NewListItem(commandPaletteDisplayLabel(entries[index]))
}

func (g *commandPaletteResultsGroup) rebindRowSignals() {
	if g == nil {
		return
	}
	for i, row := range g.rows {
		if row == nil {
			continue
		}
		index := i
		row.Activated.UnsubscribeAll()
		row.Activated.Subscribe(func(signal.Unit) {
			if g != nil {
				g.Activated.Emit(index)
			}
		})
	}
}

func (g *commandPaletteResultsGroup) measure(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	if g == nil || g.Disabled {
		return constraints.Constrain(gfx.Size{})
	}
	g.cachedWriting = ctx.WritingDirection
	g.cachedRowGap = float32(theme.DefaultResolvedContext().Spacing(theme.SpacingS))
	maxW := constraints.MaxSize.W
	if maxW <= 0 {
		maxW = 640
	}
	maxH := constraints.MaxSize.H
	if maxH <= 0 {
		maxH = 320
	}
	var width float32
	var height float32
	if len(g.rows) == 0 {
		if g.cachedEmpty != nil && g.cachedEmpty.Base() != nil && g.cachedEmpty.Base().LayoutRole() != nil {
			size := g.cachedEmpty.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: maxW, H: maxH}}).Size
			return constraints.Constrain(size)
		}
		return constraints.Constrain(gfx.Size{})
	}
	for i, row := range g.rows {
		if row == nil || row.Base() == nil || row.Base().LayoutRole() == nil {
			continue
		}
		size := row.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: maxW, H: maxH}}).Size
		if i > 0 {
			height += g.cachedRowGap
		}
		height += size.H
		width = maxFloat(width, size.W)
	}
	if height > maxH {
		height = maxH
	}
	return constraints.Constrain(gfx.Size{W: width, H: height})
}

func (g *commandPaletteResultsGroup) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if g == nil {
		return
	}
	g.cachedBounds = bounds
	g.cachedContent = gfx.Rect{}
	g.rowRects = g.rowRects[:0]
	if bounds.IsEmpty() || g.Disabled {
		return
	}
	if len(g.rows) == 0 {
		if g.cachedEmpty != nil && g.cachedEmpty.Base() != nil && g.cachedEmpty.Base().LayoutRole() != nil {
			size := g.cachedEmpty.Base().LayoutRole().MeasuredSize
			rect := gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), size.H)
			g.cachedEmpty.Base().LayoutRole().Arrange(ctx, rect)
			g.cachedContent = rect
		}
		return
	}
	y := bounds.Min.Y - g.scrollOffset
	for i, row := range g.rows {
		if row == nil || row.Base() == nil || row.Base().LayoutRole() == nil {
			continue
		}
		size := row.Base().LayoutRole().MeasuredSize
		if i > 0 {
			y += g.cachedRowGap
		}
		rect := gfx.RectFromXYWH(bounds.Min.X, y, bounds.Width(), size.H)
		row.Base().LayoutRole().Arrange(ctx, rect)
		g.rowRects = append(g.rowRects, rect)
		if g.cachedContent.IsEmpty() {
			g.cachedContent = rect
		} else {
			g.cachedContent = commandPaletteUnionRect(g.cachedContent, rect)
		}
		y += size.H
	}
}

func (g *commandPaletteResultsGroup) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if g == nil || bounds.IsEmpty() {
		return nil
	}
	cmds := make([]gfx.Command, 0, 32)
	if len(g.rows) == 0 {
		if g.cachedEmpty != nil && g.cachedEmpty.Base() != nil && g.cachedEmpty.Base().ProjectionRole() != nil {
			if projected := g.cachedEmpty.Base().ProjectionRole().Project(facet.ProjectionContext{
				Runtime:      runtimeServicesOrNil(runtime),
				Bounds:       g.cachedEmpty.Base().LayoutRole().ArrangedBounds,
				ContentScale: contentScale,
			}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		return cmds
	}
	cmds = append(cmds, gfx.PushClipRect{Rect: bounds})
	for _, row := range g.rows {
		if row == nil || row.Base() == nil || row.Base().LayoutRole() == nil {
			continue
		}
		rect := row.Base().LayoutRole().ArrangedBounds
		if rect.IsEmpty() {
			continue
		}
		if projected := row.Base().ProjectionRole().Project(facet.ProjectionContext{
			Runtime:      runtimeServicesOrNil(runtime),
			Bounds:       rect,
			ContentScale: contentScale,
		}); projected != nil {
			cmds = append(cmds, projected.Commands...)
		}
	}
	cmds = append(cmds, gfx.PopClip{})
	return cmds
}

func (g *commandPaletteResultsGroup) hitTest(pt gfx.Point) facet.HitResult {
	if g == nil || g.Disabled || g.cachedBounds.IsEmpty() || !g.cachedBounds.Contains(pt) {
		return facet.HitResult{}
	}
	if len(g.rows) == 0 {
		if g.cachedEmpty != nil && g.cachedEmpty.Base() != nil && g.cachedEmpty.Base().LayoutRole() != nil {
			if b := g.cachedEmpty.Base().LayoutRole().ArrangedBounds; !b.IsEmpty() && b.Contains(pt) {
				return facet.HitResult{Hit: true, MarkID: commandPaletteMarkIDResultsList, Cursor: facet.CursorDefault}
			}
		}
		return facet.HitResult{Hit: true, MarkID: commandPaletteMarkIDResultsList, Cursor: facet.CursorDefault}
	}
	for _, row := range g.rows {
		if row == nil || row.Base() == nil || row.Base().LayoutRole() == nil {
			continue
		}
		if b := row.Base().LayoutRole().ArrangedBounds; !b.IsEmpty() && b.Contains(pt) {
			if hit := row.Base().HitRole().HitTest(pt); hit.Hit {
				return hit
			}
			return facet.HitResult{Hit: true, MarkID: commandPaletteMarkIDResultsList, Cursor: facet.CursorPointer}
		}
	}
	return facet.HitResult{Hit: true, MarkID: commandPaletteMarkIDResultsList, Cursor: facet.CursorPointer}
}

func (g *commandPaletteResultsGroup) onPointer(e facet.PointerEvent) bool {
	if g == nil || g.Disabled || e.Kind != platform.PointerPress {
		return false
	}
	hit := g.hitTest(e.Position)
	if !hit.Hit {
		return false
	}
	if hit.MarkID == commandPaletteMarkIDResultsList {
		return true
	}
	return true
}

func (g *commandPaletteResultsGroup) onScroll(e facet.ScrollEvent) bool {
	if g == nil || g.Disabled || len(g.rows) == 0 {
		return false
	}
	delta := e.DeltaY
	if delta == 0 {
		return false
	}
	g.scrollOffset = maxFloat(0, g.scrollOffset-delta*24)
	g.invalidate(facet.DirtyProjection)
	return true
}

func (g *commandPaletteResultsGroup) onKey(e facet.KeyEvent) bool {
	if g == nil || g.Disabled || len(g.rows) == 0 || e.Kind != platform.KeyPress {
		if g != nil && e.Kind == platform.KeyPress && e.Key == platform.KeyEscape {
			if g.parent != nil {
				g.parent.SetOpen(false)
			}
			return true
		}
		return false
	}
	switch e.Key {
	case platform.KeyDown:
		g.moveActive(1)
		return true
	case platform.KeyUp:
		g.moveActive(-1)
		return true
	case platform.KeyPageDown:
		g.moveActive(5)
		return true
	case platform.KeyPageUp:
		g.moveActive(-5)
		return true
	case platform.KeyHome:
		g.setActive(0)
		return true
	case platform.KeyEnd:
		g.setActive(len(g.rows) - 1)
		return true
	case platform.KeyEnter, platform.KeySpace:
		g.Activated.Emit(g.activeIndex)
		return true
	case platform.KeyEscape:
		if g.parent != nil {
			g.parent.SetOpen(false)
		}
		return true
	default:
		return false
	}
}

func (g *commandPaletteResultsGroup) onDismiss(e facet.DismissEvent) bool {
	_ = e
	if g == nil || g.Disabled {
		return false
	}
	if g.parent != nil {
		g.parent.SetOpen(false)
	}
	return true
}

func (g *commandPaletteResultsGroup) onFocusGained() {
	if g == nil || g.Disabled {
		return
	}
	if g.parent != nil {
		g.parent.focusedVisible = true
	}
}

func (g *commandPaletteResultsGroup) onFocusLost() {}

func (g *commandPaletteResultsGroup) moveActive(delta int) {
	if g == nil || len(g.rows) == 0 {
		return
	}
	g.setActive(clampInt(g.activeIndex+delta, 0, len(g.rows)-1))
}

func (g *commandPaletteResultsGroup) setActive(index int) {
	if g == nil || len(g.rows) == 0 {
		return
	}
	index = clampInt(index, 0, len(g.rows)-1)
	if g.activeIndex == index {
		return
	}
	g.activeIndex = index
	for i, row := range g.rows {
		if row == nil {
			continue
		}
		row.SetActive(i == index)
	}
	g.ensureActiveVisible()
	g.invalidate(facet.DirtyProjection)
}

func (g *commandPaletteResultsGroup) ensureActiveVisible() {
	if g == nil || g.activeIndex < 0 || g.activeIndex >= len(g.rowRects) {
		return
	}
	rect := g.rowRects[g.activeIndex]
	if rect.IsEmpty() {
		return
	}
	if rect.Min.Y < g.cachedBounds.Min.Y {
		g.scrollOffset = maxFloat(0, g.scrollOffset-(g.cachedBounds.Min.Y-rect.Min.Y))
	}
	if rect.Max.Y > g.cachedBounds.Max.Y {
		g.scrollOffset += rect.Max.Y - g.cachedBounds.Max.Y
	}
}

func commandPaletteResultsChild(base *facet.Facet, markID facet.MarkID, order int) facet.GroupChild {
	if base == nil || base.LayoutRole() == nil {
		return facet.GroupChild{}
	}
	return facet.GroupChild{
		FacetID: base.ID(),
		MarkID:  markID,
		Attachment: facet.Attachment{
			Placement: facet.Placement{
				Mode:   facet.PlacementLinear,
				Linear: facet.LinearPlacement{Order: order, CrossAxisAlign: facet.CrossAxisStretch},
			},
		},
		Layout:   base.LayoutRole(),
		Contract: base.LayoutRole().Child,
	}
}

type commandPaletteResultsGroupPolicy struct {
	group *commandPaletteResultsGroup
}

func (commandPaletteResultsGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (p commandPaletteResultsGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.group == nil {
		return facet.GroupMeasureResult{}, nil
	}
	size := p.group.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}})
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p commandPaletteResultsGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.group == nil {
		return nil, nil
	}
	p.group.arrange(ctx.ArrangeContext, ctx.Bounds)
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

func commandPaletteDisplayLabel(entry runtimepkg.CommandEntry) string {
	title := strings.TrimSpace(entry.Title)
	if category := strings.TrimSpace(entry.Category); category != "" {
		if title == "" {
			return category
		}
		return category + ": " + title
	}
	return title
}

func commandPaletteUnionRect(a, b gfx.Rect) gfx.Rect {
	if a.IsEmpty() {
		return b
	}
	if b.IsEmpty() {
		return a
	}
	minX := minFloat(a.Min.X, b.Min.X)
	minY := minFloat(a.Min.Y, b.Min.Y)
	maxX := maxFloat(a.Max.X, b.Max.X)
	maxY := maxFloat(a.Max.Y, b.Max.Y)
	return gfx.RectFromXYWH(minX, minY, maxX-minX, maxY-minY)
}
