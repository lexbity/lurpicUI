package action

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	gfxsvg "codeburg.org/lexbit/lurpicui/gfx/svg"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiaction"
)

const (
	menuButtonMarkIDRoot                facet.MarkID = 1
	menuButtonMarkIDTrigger             facet.MarkID = 2
	menuButtonMarkIDTriggerLabel        facet.MarkID = 3
	menuButtonMarkIDTriggerIcon         facet.MarkID = 4
	menuButtonMarkIDChevron             facet.MarkID = 5
	menuButtonMarkIDFloatingMenuSurface facet.MarkID = 6
	menuButtonMarkIDMenuItems           facet.MarkID = 7
	menuButtonMarkIDFocusRing           facet.MarkID = 8
)

// MenuButtonEntryKind describes one menu-button entry shape.
type MenuButtonEntryKind uint8

const (
	// MenuButtonEntryItem is a regular command row.
	MenuButtonEntryItem MenuButtonEntryKind = iota
	// MenuButtonEntrySection is a non-interactive section header.
	MenuButtonEntrySection
	// MenuButtonEntryDivider is a visual separator.
	MenuButtonEntryDivider
)

// MenuButtonEntry describes one menu-button entry.
type MenuButtonEntry struct {
	Key             string
	Label           string
	AccessibleLabel string
	IconRef         string
	Shortcut        string
	Kind            MenuButtonEntryKind
	Disabled        bool
	Selected        bool
	Destructive     bool
}

// MenuButton implements the action.menu_button standard mark.
type MenuButton struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	focusRole      facet.FocusRole
	textRole       facet.TextRole

	Activated signal.Signal[string]

	Label           string
	AccessibleLabel string
	TriggerIconRef  string
	Entries         []MenuButtonEntry
	Disabled        bool
	Open            bool

	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool
	focusedIndex     int
	hoveredIndex     int
	pressedIndex     int

	cachedTokens             theme.Tokens
	cachedRecipe             shared.MenuButtonSlots
	cachedRootBounds         gfx.Rect
	cachedTriggerBounds      gfx.Rect
	cachedTriggerLabelBounds gfx.Rect
	cachedTriggerIconBounds  gfx.Rect
	cachedChevronBounds      gfx.Rect
	cachedMenuBounds         gfx.Rect
	cachedFocusBounds        gfx.Rect
	cachedEntryLayouts       []menuButtonEntryLayout
	cachedPadX               float32
	cachedPadY               float32
	cachedGap                float32
	cachedRowGap             float32
	cachedRadius             float32
	cachedTriggerLabelLayout *text.TextLayout
	cachedTriggerLabelStyle  text.TextStyle
	cachedItemStyle          text.TextStyle
	cachedShortcutStyle      text.TextStyle
	cachedSectionStyle       text.TextStyle
	cachedWritingDirection   facet.WritingDirection
	cachedTriggerHeight      float32
	cachedRowHeight          float32
	cachedSectionHeight      float32
	cachedDividerHeight      float32
	cachedTriggerIconSize    float32
	cachedChevronSize        float32
	cachedCheckSize          float32
	cachedMenuIconSize       float32
	cachedTriggerMeasuredW   float32
	cachedTriggerMeasuredH   float32
	cachedMenuMeasuredW      float32
	cachedMenuMeasuredH      float32
}

type menuButtonEntryLayout struct {
	entry          MenuButtonEntry
	labelLayout    *text.TextLayout
	shortcutLayout *text.TextLayout
	bounds         gfx.Rect
	labelBounds    gfx.Rect
	shortcutBounds gfx.Rect
	iconBounds     gfx.Rect
	checkBounds    gfx.Rect
	width          float32
	height         float32
}

var _ facet.FacetImpl = (*MenuButton)(nil)
var _ layout.AnchorExporter = (*MenuButton)(nil)

// NewMenuButton constructs an action.menu_button mark with canonical defaults.
func NewMenuButton(label string, entries []MenuButtonEntry) *MenuButton {
	m := &MenuButton{
		Facet:        facet.NewFacet(),
		Label:        label,
		Entries:      normalizeMenuButtonEntries(entries),
		focusedIndex: -1,
		hoveredIndex: -1,
		pressedIndex: -1,
		Activated:    signal.NewSignal[string]("menu_button_activated"),
	}
	m.layoutRole.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearVertical,
		Policy: menuButtonGroupPolicy{button: m},
	}
	m.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := m.measureIntrinsic(ctx, constraints)
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
	m.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return m.measure(ctx, constraints)
	}
	m.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		m.layoutRole.ArrangedBounds = bounds
		m.arrange(ctx, bounds)
	}
	m.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := m.buildCommands(bounds, nil)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	m.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := m.buildCommands(m.layoutRole.ArrangedBounds, ctx.Runtime)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	m.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult { return m.hitTest(p) }
	m.inputRole.OnPointer = func(e facet.PointerEvent) bool { return m.onPointer(e) }
	m.inputRole.OnKey = func(e facet.KeyEvent) bool { return m.onKey(e) }
	m.inputRole.OnDismiss = func(e facet.DismissEvent) bool { return m.onDismiss(e) }
	m.focusRole.Focusable = func() bool {
		return !m.Disabled && (strings.TrimSpace(m.Label) != "" || strings.TrimSpace(m.AccessibleLabel) != "" || len(m.Entries) > 0)
	}
	m.focusRole.TabIndex = 0
	m.focusRole.OnFocusGained = func() { m.onFocusGained() }
	m.focusRole.OnFocusLost = func() { m.onFocusLost() }
	m.textRole.IMEEnabled = false
	m.AddRole(&m.layoutRole)
	m.AddRole(&m.renderRole)
	m.AddRole(&m.projectionRole)
	m.AddRole(&m.hitRole)
	m.AddRole(&m.inputRole)
	m.AddRole(&m.focusRole)
	m.AddRole(&m.textRole)
	return m
}

// Base satisfies facet.FacetImpl.
func (m *MenuButton) Base() *facet.Facet {
	m.Facet.BindImpl(m)
	return &m.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (m *MenuButton) AccessibilityRole() string { return "button_with_menu" }

// AccessibleName reports the semantic name required by the spec.
func (m *MenuButton) AccessibleName() string {
	if m == nil {
		return ""
	}
	if name := strings.TrimSpace(m.AccessibleLabel); name != "" {
		return name
	}
	if name := strings.TrimSpace(m.Label); name != "" {
		return name
	}
	for _, entry := range m.Entries {
		if entry.Kind != MenuButtonEntryItem {
			continue
		}
		if name := strings.TrimSpace(entry.AccessibleLabel); name != "" {
			return name
		}
		if name := strings.TrimSpace(entry.Label); name != "" {
			return name
		}
	}
	return ""
}

// SetLabel updates the authored trigger label.
func (m *MenuButton) SetLabel(label string) {
	if m == nil || m.Label == label {
		return
	}
	m.Label = label
	m.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetAccessibleLabel updates the trigger accessible label.
func (m *MenuButton) SetAccessibleLabel(label string) {
	if m == nil || m.AccessibleLabel == label {
		return
	}
	m.AccessibleLabel = label
	m.invalidate(facet.DirtyProjection)
}

// SetTriggerIconRef updates the authored trigger icon reference.
func (m *MenuButton) SetTriggerIconRef(ref string) {
	if m == nil || m.TriggerIconRef == ref {
		return
	}
	m.TriggerIconRef = strings.TrimSpace(ref)
	m.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetEntries replaces the menu entries.
func (m *MenuButton) SetEntries(entries []MenuButtonEntry) {
	if m == nil {
		return
	}
	m.Entries = normalizeMenuButtonEntries(entries)
	m.syncFocusIndex()
	m.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetOpen updates the open state.
func (m *MenuButton) SetOpen(open bool) {
	if m == nil || m.Open == open {
		return
	}
	m.Open = open
	if open {
		m.syncFocusIndex()
	} else {
		m.pressedIndex = -1
		m.hoveredIndex = -1
	}
	m.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetDisabled toggles disabled state.
func (m *MenuButton) SetDisabled(disabled bool) {
	if m == nil || m.Disabled == disabled {
		return
	}
	m.Disabled = disabled
	if disabled {
		m.hovered = false
		m.pressed = false
		m.focusedVisible = false
		m.focusFromPointer = false
		m.hoveredIndex = -1
		m.pressedIndex = -1
		m.Open = false
	}
	m.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// ExportAnchors publishes the menu button anchor set.
func (m *MenuButton) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if m == nil {
		return nil
	}
	bounds := m.layoutRole.ArrangedBounds
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
	if m.cachedTriggerLabelLayout != nil {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: m.cachedTriggerLabelBounds.Min.Y + m.cachedTriggerLabelLayout.Baseline}
	} else {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	if !m.cachedTriggerBounds.IsEmpty() {
		out["content_anchor"] = gfx.Point{X: m.cachedTriggerBounds.Min.X + m.cachedTriggerBounds.Width()*0.5, Y: m.cachedTriggerBounds.Min.Y + m.cachedTriggerBounds.Height()*0.5}
	}
	return out
}

// Children returns the facet's immediate child list.
func (m *MenuButton) Children() []facet.GroupChild { return nil }

// OnAttach is unused.
func (m *MenuButton) OnAttach(ctx facet.AttachContext) {}

// OnActivate is unused.
func (m *MenuButton) OnActivate() {}

// OnDeactivate is unused.
func (m *MenuButton) OnDeactivate() {}

// OnDetach clears cached projection state.
func (m *MenuButton) OnDetach() {
	m.cachedTokens = theme.Tokens{}
	m.cachedRecipe = shared.MenuButtonSlots{}
	m.cachedRootBounds = gfx.Rect{}
	m.cachedTriggerBounds = gfx.Rect{}
	m.cachedTriggerLabelBounds = gfx.Rect{}
	m.cachedTriggerIconBounds = gfx.Rect{}
	m.cachedChevronBounds = gfx.Rect{}
	m.cachedMenuBounds = gfx.Rect{}
	m.cachedFocusBounds = gfx.Rect{}
	m.cachedEntryLayouts = nil
	m.cachedPadX = 0
	m.cachedPadY = 0
	m.cachedGap = 0
	m.cachedRowGap = 0
	m.cachedRadius = 0
	m.cachedTriggerLabelLayout = nil
	m.cachedTriggerLabelStyle = text.TextStyle{}
	m.cachedItemStyle = text.TextStyle{}
	m.cachedShortcutStyle = text.TextStyle{}
	m.cachedSectionStyle = text.TextStyle{}
	m.cachedTriggerHeight = 0
	m.cachedRowHeight = 0
	m.cachedSectionHeight = 0
	m.cachedDividerHeight = 0
	m.cachedTriggerIconSize = 0
	m.cachedChevronSize = 0
	m.cachedCheckSize = 0
	m.cachedMenuIconSize = 0
	m.cachedTriggerMeasuredW = 0
	m.cachedTriggerMeasuredH = 0
	m.cachedMenuMeasuredW = 0
	m.cachedMenuMeasuredH = 0
}

func (m *MenuButton) invalidate(flags facet.DirtyFlags) {
	if m == nil {
		return
	}
	m.Base().Invalidate(flags)
}

func (m *MenuButton) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uiaction.ResolveMenuButtonRecipe(style)
	m.cachedTokens = resolved.TokenSet()
	m.cachedRecipe = slots
	m.cachedWritingDirection = ctx.WritingDirection
	m.cachedPadX = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	m.cachedPadY = maxFloat(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	m.cachedGap = maxFloat(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	m.cachedRowGap = maxFloat(float32(resolved.Spacing(theme.SpacingXS)), resolved.Density.Scale(6))
	m.cachedRadius = float32(resolved.Radius(theme.RadiusM))
	m.cachedTriggerHeight = maxFloat(resolved.Density.Scale(36), resolved.Density.Scale(32))
	m.cachedRowHeight = maxFloat(resolved.Density.Scale(32), resolved.Density.Scale(28))
	m.cachedSectionHeight = maxFloat(resolved.Density.Scale(26), resolved.Density.Scale(22))
	m.cachedDividerHeight = maxFloat(1, resolved.Density.Scale(1))
	m.cachedTriggerIconSize = maxFloat(resolved.Density.Scale(18), 12)
	m.cachedChevronSize = maxFloat(resolved.Density.Scale(12), 10)
	m.cachedCheckSize = maxFloat(resolved.Density.Scale(12), 10)
	m.cachedMenuIconSize = maxFloat(resolved.Density.Scale(16), 12)

	triggerStyle := resolved.TextStyle(theme.TextLabelM)
	itemStyle := resolved.TextStyle(theme.TextBodyM)
	shortcutStyle := resolved.TextStyle(theme.TextLabelS)
	sectionStyle := resolved.TextStyle(theme.TextLabelS)
	m.cachedTriggerLabelStyle = triggerStyle
	m.cachedItemStyle = itemStyle
	m.cachedShortcutStyle = shortcutStyle
	m.cachedSectionStyle = sectionStyle

	triggerLabel := strings.TrimSpace(m.Label)
	if triggerLabel == "" {
		triggerLabel = strings.TrimSpace(m.AccessibleLabel)
	}
	shaper := m.newShaper(ctx.Runtime)
	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = resolved.Density.Scale(420)
	}

	var triggerLabelLayout *text.TextLayout
	if shaper != nil && triggerLabel != "" {
		shaper.SetContentScale(ctx.ContentScale)
		triggerLabelLayout = shaper.ShapeTruncated(triggerLabel, triggerStyle, maxWidth)
	}
	m.cachedTriggerLabelLayout = triggerLabelLayout
	m.textRole.Layout = triggerLabelLayout
	m.textRole.Selection = text.TextRange{}
	m.textRole.CaretVisible = false
	m.textRole.CaretPosition = text.TextPosition{}
	if triggerLabelLayout != nil {
		m.cachedTriggerLabelBounds = gfx.RectFromXYWH(0, 0, triggerLabelLayout.Bounds.Width(), triggerLabelLayout.Bounds.Height())
	} else {
		m.cachedTriggerLabelBounds = gfx.Rect{}
	}

	layouts := make([]menuButtonEntryLayout, len(m.Entries))
	maxEntryW := float32(0)
	totalMenuH := float32(0)
	for i := range m.Entries {
		entry := m.Entries[i]
		layouts[i].entry = entry
		switch entry.Kind {
		case MenuButtonEntryDivider:
			layouts[i].width = 0
			layouts[i].height = m.cachedDividerHeight
		case MenuButtonEntrySection:
			label := strings.TrimSpace(entry.Label)
			if shaper != nil && label != "" {
				layouts[i].labelLayout = shaper.ShapeTruncated(label, sectionStyle, maxWidth)
			}
			layouts[i].height = maxFloat(m.cachedSectionHeight, text.Height(layouts[i].labelLayout)+m.cachedPadY)
			layouts[i].width = text.Width(layouts[i].labelLayout) + m.cachedPadX*2
			if layouts[i].width < resolved.Density.Scale(120) {
				layouts[i].width = resolved.Density.Scale(120)
			}
		default:
			label := strings.TrimSpace(entry.Label)
			short := strings.TrimSpace(entry.Shortcut)
			if shaper != nil && label != "" {
				layouts[i].labelLayout = shaper.ShapeTruncated(label, itemStyle, maxWidth)
			}
			if shaper != nil && short != "" {
				layouts[i].shortcutLayout = shaper.ShapeTruncated(short, shortcutStyle, maxWidth)
			}
			leadW := float32(0)
			if entry.Selected {
				leadW += m.cachedCheckSize
				leadW += m.cachedGap
			}
			if strings.TrimSpace(entry.IconRef) != "" {
				if leadW > 0 {
					leadW += m.cachedGap
				}
				leadW += m.cachedMenuIconSize
				leadW += m.cachedGap
			}
			labelW := text.Width(layouts[i].labelLayout)
			shortW := text.Width(layouts[i].shortcutLayout)
			rowW := m.cachedPadX*2 + leadW + labelW
			if shortW > 0 {
				rowW += m.cachedGap + shortW
			}
			if rowW < resolved.Density.Scale(160) {
				rowW = resolved.Density.Scale(160)
			}
			rowH := maxFloat(m.cachedRowHeight, maxFloat(text.Height(layouts[i].labelLayout), text.Height(layouts[i].shortcutLayout)))
			if rowH < m.cachedMenuIconSize+m.cachedPadY {
				rowH = m.cachedMenuIconSize + m.cachedPadY
			}
			layouts[i].width = rowW
			layouts[i].height = rowH
		}
		if layouts[i].width > maxEntryW {
			maxEntryW = layouts[i].width
		}
		totalMenuH += layouts[i].height
	}
	m.cachedEntryLayouts = layouts

	triggerContentW := m.cachedPadX * 2
	if m.TriggerIconRef != "" {
		triggerContentW += m.cachedTriggerIconSize + m.cachedGap
	}
	triggerContentW += text.Width(triggerLabelLayout)
	if text.Width(triggerLabelLayout) > 0 {
		triggerContentW += m.cachedGap
	}
	triggerContentW += m.cachedChevronSize
	triggerW := maxFloat(resolved.Density.Scale(120), triggerContentW)
	triggerH := maxFloat(m.cachedTriggerHeight, maxFloat(text.Height(triggerLabelLayout), m.cachedTriggerIconSize))
	triggerH = maxFloat(triggerH, m.cachedChevronSize)
	triggerH += m.cachedPadY * 2
	m.cachedTriggerMeasuredW = triggerW
	m.cachedTriggerMeasuredH = triggerH

	menuW := maxFloat(maxEntryW, triggerW)
	if len(m.Entries) > 0 {
		menuW = maxFloat(menuW, resolved.Density.Scale(160))
	}
	m.cachedMenuMeasuredW = menuW
	menuH := float32(0)
	if m.Open && len(m.Entries) > 0 {
		menuH = totalMenuH
		if len(m.Entries) > 1 {
			menuH += m.cachedRowGap * float32(len(m.Entries)-1)
		}
	}
	m.cachedMenuMeasuredH = menuH
	if m.Open {
		m.syncFocusIndex()
	}

	size := gfx.Size{
		W: maxFloat(triggerW, menuW) + m.cachedPadX*2,
		H: m.cachedPadY*2 + triggerH,
	}
	if m.Open && len(m.Entries) > 0 {
		size.H += m.cachedGap + menuH
	}
	size = constraints.Constrain(size)
	m.layoutRole.MeasuredSize = size
	m.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	return m.layoutRole.MeasuredResult
}

func (m *MenuButton) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return m.measure(ctx, constraints).Size
}

func (m *MenuButton) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	m.cachedRootBounds = bounds
	m.cachedTriggerBounds = gfx.Rect{}
	m.cachedTriggerLabelBounds = gfx.Rect{}
	m.cachedTriggerIconBounds = gfx.Rect{}
	m.cachedChevronBounds = gfx.Rect{}
	m.cachedMenuBounds = gfx.Rect{}
	m.cachedFocusBounds = gfx.Rect{}
	m.layoutRole.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	inner := bounds.Inset(m.cachedPadX, m.cachedPadY)
	if inner.IsEmpty() {
		inner = bounds
	}
	rtl := m.cachedWritingDirection == facet.WritingDirectionRTL
	triggerW := m.cachedTriggerMeasuredW
	if triggerW <= 0 {
		triggerW = m.cachedTriggerLabelBounds.Width()
	}
	if triggerW <= 0 {
		triggerW = maxFloat(bounds.Width()-m.cachedPadX*2, m.cachedChevronSize+m.cachedPadX*2)
	}
	triggerH := m.cachedTriggerMeasuredH
	if triggerH <= 0 {
		triggerH = maxFloat(bounds.Height(), m.cachedTriggerHeight)
	}
	startX := inner.Min.X
	if rtl {
		startX = inner.Max.X
	}
	triggerY := inner.Min.Y
	if rtl {
		m.cachedTriggerBounds = gfx.RectFromXYWH(startX-triggerW, triggerY, triggerW, triggerH)
	} else {
		m.cachedTriggerBounds = gfx.RectFromXYWH(startX, triggerY, triggerW, triggerH)
	}
	if m.cachedTriggerLabelLayout != nil {
		textH := text.Height(m.cachedTriggerLabelLayout)
		contentY := text.CenterY(m.cachedTriggerBounds, textH)
		if rtl {
			x := m.cachedTriggerBounds.Max.X - m.cachedPadX
			if m.cachedChevronSize > 0 {
				x -= m.cachedChevronSize + m.cachedGap
				m.cachedChevronBounds = text.CenterRect(gfx.RectFromXYWH(x, m.cachedTriggerBounds.Min.Y, m.cachedChevronSize, m.cachedTriggerBounds.Height()), m.cachedChevronSize, m.cachedChevronSize)
				x -= m.cachedGap
			}
			labelW := m.cachedTriggerLabelLayout.Bounds.Width()
			x -= labelW
			m.cachedTriggerLabelBounds = gfx.RectFromXYWH(x, contentY, labelW, textH)
			x -= m.cachedGap
			if strings.TrimSpace(m.TriggerIconRef) != "" {
				x -= m.cachedTriggerIconSize
				m.cachedTriggerIconBounds = text.CenterRect(gfx.RectFromXYWH(x, m.cachedTriggerBounds.Min.Y, m.cachedTriggerIconSize, m.cachedTriggerBounds.Height()), m.cachedTriggerIconSize, m.cachedTriggerIconSize)
			}
		} else {
			x := m.cachedTriggerBounds.Min.X + m.cachedPadX
			if strings.TrimSpace(m.TriggerIconRef) != "" {
				m.cachedTriggerIconBounds = text.CenterRect(gfx.RectFromXYWH(x, m.cachedTriggerBounds.Min.Y, m.cachedTriggerIconSize, m.cachedTriggerBounds.Height()), m.cachedTriggerIconSize, m.cachedTriggerIconSize)
				x += m.cachedTriggerIconSize + m.cachedGap
			}
			labelW := m.cachedTriggerLabelLayout.Bounds.Width()
			m.cachedTriggerLabelBounds = gfx.RectFromXYWH(x, contentY, labelW, textH)
			x += labelW + m.cachedGap
			m.cachedChevronBounds = text.CenterRect(gfx.RectFromXYWH(m.cachedTriggerBounds.Max.X-m.cachedPadX-m.cachedChevronSize, m.cachedTriggerBounds.Min.Y, m.cachedChevronSize, m.cachedTriggerBounds.Height()), m.cachedChevronSize, m.cachedChevronSize)
		}
	} else {
		if rtl {
			x := m.cachedTriggerBounds.Max.X - m.cachedPadX
			if m.cachedChevronSize > 0 {
				x -= m.cachedChevronSize
				m.cachedChevronBounds = text.CenterRect(gfx.RectFromXYWH(x, m.cachedTriggerBounds.Min.Y, m.cachedChevronSize, m.cachedTriggerBounds.Height()), m.cachedChevronSize, m.cachedChevronSize)
			}
		} else {
			m.cachedChevronBounds = text.CenterRect(gfx.RectFromXYWH(m.cachedTriggerBounds.Max.X-m.cachedPadX-m.cachedChevronSize, m.cachedTriggerBounds.Min.Y, m.cachedChevronSize, m.cachedTriggerBounds.Height()), m.cachedChevronSize, m.cachedChevronSize)
		}
	}
	menuY := m.cachedTriggerBounds.Max.Y + m.cachedGap
	if m.Open && len(m.cachedEntryLayouts) > 0 {
		menuW := m.cachedMenuMeasuredW
		if menuW <= 0 {
			menuW = maxFloat(m.cachedTriggerBounds.Width(), m.cachedRootBounds.Width()-m.cachedPadX*2)
		}
		for i := range m.cachedEntryLayouts {
			if m.cachedEntryLayouts[i].width > menuW {
				menuW = m.cachedEntryLayouts[i].width
			}
		}
		menuH := m.cachedMenuMeasuredH
		if menuH <= 0 {
			menuH = sumEntryHeights(m.cachedEntryLayouts, m.cachedRowGap)
		}
		if rtl {
			m.cachedMenuBounds = gfx.RectFromXYWH(m.cachedTriggerBounds.Max.X-menuW, menuY, menuW, menuH)
		} else {
			m.cachedMenuBounds = gfx.RectFromXYWH(m.cachedTriggerBounds.Min.X, menuY, menuW, menuH)
		}
		rowY := m.cachedMenuBounds.Min.Y
		for i := range m.cachedEntryLayouts {
			entry := &m.cachedEntryLayouts[i]
			switch entry.entry.Kind {
			case MenuButtonEntryDivider:
				entry.bounds = gfx.RectFromXYWH(m.cachedMenuBounds.Min.X+m.cachedPadX, rowY+m.cachedRowGap*0.5, m.cachedMenuBounds.Width()-m.cachedPadX*2, m.cachedDividerHeight)
				rowY += entry.height + m.cachedRowGap
			case MenuButtonEntrySection:
				entry.bounds = gfx.RectFromXYWH(m.cachedMenuBounds.Min.X+m.cachedPadX, rowY, m.cachedMenuBounds.Width()-m.cachedPadX*2, entry.height)
				labelH := text.Height(entry.labelLayout)
				labelW := text.Width(entry.labelLayout)
				labelY := text.CenterY(gfx.RectFromXYWH(m.cachedMenuBounds.Min.X, rowY, m.cachedMenuBounds.Width(), entry.height), labelH)
				if rtl {
					entry.labelBounds = gfx.RectFromXYWH(m.cachedMenuBounds.Max.X-m.cachedPadX-labelW, labelY, labelW, labelH)
				} else {
					entry.labelBounds = gfx.RectFromXYWH(m.cachedMenuBounds.Min.X+m.cachedPadX, labelY, labelW, labelH)
				}
				rowY += entry.height + m.cachedRowGap
			default:
				entry.bounds = gfx.RectFromXYWH(m.cachedMenuBounds.Min.X, rowY, m.cachedMenuBounds.Width(), entry.height)
				labelH := text.Height(entry.labelLayout)
				shortH := text.Height(entry.shortcutLayout)
				labelW := text.Width(entry.labelLayout)
				shortW := text.Width(entry.shortcutLayout)
				leadX := entry.bounds.Min.X + m.cachedPadX
				trailX := entry.bounds.Max.X - m.cachedPadX
				rowMidY := entry.bounds.Min.Y + entry.bounds.Height()*0.5
				if rtl {
					if shortW > 0 {
						trailX -= shortW
						entry.shortcutBounds = gfx.RectFromXYWH(trailX, rowMidY-shortH*0.5, shortW, shortH)
						trailX -= m.cachedGap
					}
					labelX := trailX - labelW
					entry.labelBounds = gfx.RectFromXYWH(labelX, rowMidY-labelH*0.5, labelW, labelH)
					leadX = entry.bounds.Max.X - m.cachedPadX
					if entry.entry.Selected {
						leadX -= m.cachedCheckSize
						entry.checkBounds = gfx.RectFromXYWH(leadX, rowMidY-m.cachedCheckSize*0.5, m.cachedCheckSize, m.cachedCheckSize)
						leadX -= m.cachedGap
					}
					if strings.TrimSpace(entry.entry.IconRef) != "" {
						leadX -= m.cachedMenuIconSize
						entry.iconBounds = gfx.RectFromXYWH(leadX, rowMidY-m.cachedMenuIconSize*0.5, m.cachedMenuIconSize, m.cachedMenuIconSize)
					}
				} else {
					if entry.entry.Selected {
						entry.checkBounds = gfx.RectFromXYWH(leadX, rowMidY-m.cachedCheckSize*0.5, m.cachedCheckSize, m.cachedCheckSize)
						leadX += m.cachedCheckSize + m.cachedGap
					}
					if strings.TrimSpace(entry.entry.IconRef) != "" {
						entry.iconBounds = gfx.RectFromXYWH(leadX, rowMidY-m.cachedMenuIconSize*0.5, m.cachedMenuIconSize, m.cachedMenuIconSize)
						leadX += m.cachedMenuIconSize + m.cachedGap
					}
					entry.labelBounds = gfx.RectFromXYWH(leadX, rowMidY-labelH*0.5, labelW, labelH)
					if shortW > 0 {
						entry.shortcutBounds = gfx.RectFromXYWH(trailX-shortW, rowMidY-shortH*0.5, shortW, shortH)
					}
				}
				rowY += entry.height + m.cachedRowGap
			}
		}
	}
	m.cachedFocusBounds = m.cachedTriggerBounds.Inset(maxFloat(1, m.cachedTriggerBounds.Height()*0.08), maxFloat(1, m.cachedTriggerBounds.Height()*0.08))
}

func (m *MenuButton) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.MenuButtonSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: m.cachedTokens}, m.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, m.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uiaction.ResolveMenuButtonRecipe(style)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: m.cachedTokens}, m.cachedRecipe
}

func (m *MenuButton) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if m == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := m.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := m.interactionState()
	if m.Open && state == theme.StateDefault {
		state = theme.StateSelected
	}
	root := slots.Root.Resolve(state, tokens)
	trigger := slots.Trigger.Resolve(state, tokens)
	label := slots.TriggerLabel.Resolve(state, tokens)
	triggerIcon := slots.TriggerIcon.Resolve(state, tokens)
	chevron := slots.Chevron.Resolve(state, tokens)
	menuSurface := slots.FloatingMenuSurface.Resolve(theme.StateSelected, tokens)
	menuItems := slots.MenuItems.Resolve(theme.StateDefault, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)

	cmds := make([]gfx.Command, 0, 128)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(trigger) {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(m.cachedTriggerBounds, m.cachedRadius), trigger)...)
	}
	if m.Open && !m.cachedMenuBounds.IsEmpty() && !isTransparentMaterial(menuSurface) {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(m.cachedMenuBounds, m.cachedRadius), menuSurface)...)
	}
	if m.cachedTriggerLabelLayout != nil && !isTransparentMaterial(label) {
		cmds = append(cmds, labelCommands(m.cachedTriggerLabelLayout, m.cachedTriggerLabelBounds, label)...)
	}
	if !m.cachedTriggerIconBounds.IsEmpty() && m.TriggerIconRef != "" {
		if iconCmds := iconAssetCommands(runtimeServicesOrNil(runtime), m.TriggerIconRef, m.cachedTriggerIconBounds, triggerIcon); len(iconCmds) > 0 {
			cmds = append(cmds, iconCmds...)
		}
	}
	if !m.cachedChevronBounds.IsEmpty() && !isTransparentMaterial(chevron) {
		cmds = append(cmds, materialCommands(menuButtonChevronPath(m.cachedChevronBounds), chevron)...)
	}
	if m.Open && len(m.cachedEntryLayouts) > 0 {
		for i := range m.cachedEntryLayouts {
			entry := &m.cachedEntryLayouts[i]
			if entry.bounds.IsEmpty() {
				continue
			}
			rowState := m.entryState(i)
			switch entry.entry.Kind {
			case MenuButtonEntryDivider:
				div := theme.MarkStyle{Base: theme.Material{Fills: []theme.Fill{{Type: theme.FillSolid, Color: tintColor(tokens.Color.OnSurfaceVariant, 0.25)}}, Opacity: 1}}
				cmds = append(cmds, materialCommands(gfx.RectPath(entry.bounds), div.Resolve(theme.StateDefault, tokens))...)
			case MenuButtonEntrySection:
				if !isTransparentMaterial(menuItems) && entry.labelLayout != nil {
					cmds = append(cmds, labelCommands(entry.labelLayout, entry.labelBounds, menuItems)...)
				}
			default:
				rowMaterial := theme.Material{Opacity: 0}
				switch rowState {
				case theme.StateHover:
					rowMaterial = theme.FromToken(tintColor(tokens.Color.Primary, 0.08))
				case theme.StatePressed:
					rowMaterial = theme.FromToken(tintColor(tokens.Color.Primary, 0.14))
				case theme.StateSelected:
					rowMaterial = theme.FromToken(tintColor(tokens.Color.Primary, 0.10))
				case theme.StateFocused:
					rowMaterial = theme.FromToken(tintColor(tokens.Color.Primary, 0.06))
				}
				if entry.entry.Destructive {
					switch rowState {
					case theme.StateHover:
						rowMaterial = theme.FromToken(tokens.Color.Error)
					case theme.StatePressed:
						rowMaterial = theme.FromToken(tokens.Color.Error)
					case theme.StateSelected:
						rowMaterial = theme.FromToken(tokens.Color.Error)
					case theme.StateFocused:
						rowMaterial = theme.FromToken(tokens.Color.Error)
					}
				}
				if !isTransparentMaterial(rowMaterial) {
					cmds = append(cmds, materialCommands(gfx.RoundedRectPath(entry.bounds, maxFloat(0, m.cachedRadius*0.5)), rowMaterial)...)
				}
				if entry.entry.Selected && !entry.checkBounds.IsEmpty() {
					checkMaterial := theme.MarkStyle{Base: theme.FromToken(tokens.Color.Primary)}.Resolve(rowState, tokens)
					cmds = append(cmds, materialCommands(menuButtonCheckmarkPath(entry.checkBounds), checkMaterial)...)
				}
				if entry.entry.IconRef != "" && !entry.iconBounds.IsEmpty() {
					iconMat := menuItems
					if entry.entry.Destructive {
						iconMat = theme.MarkStyle{Base: theme.FromToken(tokens.Color.Error)}.Resolve(rowState, tokens)
					}
					if iconCmds := iconAssetCommands(runtimeServicesOrNil(runtime), entry.entry.IconRef, entry.iconBounds, iconMat); len(iconCmds) > 0 {
						cmds = append(cmds, iconCmds...)
					}
				}
				if entry.labelLayout != nil && !isTransparentMaterial(menuItems) {
					cmds = append(cmds, labelCommands(entry.labelLayout, entry.labelBounds, menuItems)...)
				}
				if entry.shortcutLayout != nil && !entry.shortcutBounds.IsEmpty() {
					cmds = append(cmds, labelCommands(entry.shortcutLayout, entry.shortcutBounds, menuItems)...)
				}
			}
		}
	}
	if m.focusedVisible && !isTransparentMaterial(focus) {
		inset := maxFloat(1, m.cachedTriggerBounds.Height()*0.08)
		ringBounds := m.cachedTriggerBounds.Inset(-inset, -inset)
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ringBounds, m.cachedRadius+inset), focus)...)
	}
	return cmds
}

func (m *MenuButton) hitTest(p gfx.Point) facet.HitResult {
	if m == nil || m.layoutRole.ArrangedBounds.IsEmpty() || !m.layoutRole.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := m.cursorShape()
	if m.focusedVisible && m.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: menuButtonMarkIDFocusRing, Cursor: cursor}
	}
	if idx := m.indexAt(p); idx >= 0 {
		return facet.HitResult{Hit: true, MarkID: menuButtonMarkIDMenuItems, Cursor: cursor}
	}
	if m.Open && m.cachedMenuBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: menuButtonMarkIDFloatingMenuSurface, Cursor: cursor}
	}
	if m.cachedChevronBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: menuButtonMarkIDChevron, Cursor: cursor}
	}
	if m.cachedTriggerIconBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: menuButtonMarkIDTriggerIcon, Cursor: cursor}
	}
	if m.cachedTriggerLabelBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: menuButtonMarkIDTriggerLabel, Cursor: cursor}
	}
	if m.cachedTriggerBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: menuButtonMarkIDTrigger, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: menuButtonMarkIDRoot, Cursor: cursor}
}

func (m *MenuButton) cursorShape() facet.CursorShape {
	if m.Disabled {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (m *MenuButton) onPointer(e facet.PointerEvent) bool {
	if m.Disabled {
		return false
	}
	idx := m.indexAt(e.Position)
	switch e.Kind {
	case platform.PointerEnter, platform.PointerMove:
		m.hovered = true
		if idx != m.hoveredIndex {
			m.hoveredIndex = idx
			m.invalidate(facet.DirtyProjection)
		}
		return true
	case platform.PointerLeave:
		m.hovered = false
		m.hoveredIndex = -1
		if !m.pressed {
			m.focusFromPointer = false
		}
		m.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		m.hovered = true
		m.focusFromPointer = true
		m.focusedVisible = false
		if idx >= 0 && m.entryIsSelectable(idx) {
			m.pressedIndex = idx
			m.invalidate(facet.DirtyProjection)
			return true
		}
		if m.cachedTriggerBounds.Contains(e.Position) {
			m.pressed = true
			m.invalidate(facet.DirtyProjection)
			return true
		}
		return false
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := m.pressed
		m.pressed = false
		if idx >= 0 && m.entryIsSelectable(idx) {
			wasPressed = wasPressed || m.pressedIndex == idx
			m.pressedIndex = -1
			m.invalidate(facet.DirtyProjection)
			if wasPressed {
				m.activateEntry(idx)
				return true
			}
			return false
		}
		if m.cachedTriggerBounds.Contains(e.Position) && wasPressed {
			m.toggleOpen()
			return true
		}
		m.pressedIndex = -1
		m.invalidate(facet.DirtyProjection)
		return wasPressed
	default:
		return false
	}
}

func (m *MenuButton) onKey(e facet.KeyEvent) bool {
	if m.Disabled {
		return false
	}
	if m.Open {
		switch e.Key {
		case platform.KeyUp, platform.KeyDown, platform.KeyHome, platform.KeyEnd:
			if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
				m.navigateOpen(e.Key)
				return true
			}
		case platform.KeyEnter, platform.KeySpace:
			if e.Kind == platform.KeyRelease {
				if m.focusedIndex >= 0 {
					m.activateEntry(m.focusedIndex)
					return true
				}
			}
			return e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat || e.Kind == platform.KeyRelease
		case platform.KeyEscape:
			if e.Kind == platform.KeyPress {
				m.toggleOpen()
				return true
			}
		}
	}
	switch e.Key {
	case platform.KeyEnter, platform.KeySpace:
		if e.Kind == platform.KeyRelease {
			m.toggleOpen()
			return true
		}
		return e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat
	case platform.KeyDown:
		if e.Kind == platform.KeyPress {
			if !m.Open {
				m.Open = true
				m.focusedIndex = m.firstSelectableIndex()
				m.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
				return true
			}
		}
	case platform.KeyUp:
		if e.Kind == platform.KeyPress {
			if !m.Open {
				m.Open = true
				m.focusedIndex = m.lastSelectableIndex()
				m.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
				return true
			}
		}
	}
	return false
}

func (m *MenuButton) onDismiss(e facet.DismissEvent) bool {
	_ = e
	if m.Disabled || !m.Open {
		return false
	}
	m.SetOpen(false)
	return true
}

func (m *MenuButton) onFocusGained() {
	m.focusedVisible = !m.focusFromPointer
	m.focusFromPointer = false
	m.invalidate(facet.DirtyProjection)
}

func (m *MenuButton) onFocusLost() {
	m.focusedVisible = false
	m.pressed = false
	m.focusFromPointer = false
	m.hoveredIndex = -1
	m.pressedIndex = -1
	m.invalidate(facet.DirtyProjection)
}

func (m *MenuButton) interactionState() theme.InteractionState {
	switch {
	case m.Disabled:
		return theme.StateDisabled
	case m.pressed:
		return theme.StatePressed
	case m.hovered:
		return theme.StateHover
	case m.focusedVisible:
		return theme.StateFocused
	case m.Open:
		return theme.StateSelected
	default:
		return theme.StateDefault
	}
}

func (m *MenuButton) pointInFocusRing(p gfx.Point) bool {
	if !m.cachedTriggerBounds.Contains(p) {
		return false
	}
	inset := maxFloat(1, m.cachedTriggerBounds.Height()*0.08)
	inner := m.cachedTriggerBounds.Inset(inset, inset)
	if inner.IsEmpty() {
		return true
	}
	return !inner.Contains(p)
}

func (m *MenuButton) indexAt(p gfx.Point) int {
	for i := range m.cachedEntryLayouts {
		if m.cachedEntryLayouts[i].bounds.Contains(p) {
			return i
		}
	}
	return -1
}

func (m *MenuButton) entryIsSelectable(index int) bool {
	if index < 0 || index >= len(m.cachedEntryLayouts) {
		return false
	}
	entry := m.cachedEntryLayouts[index].entry
	return entry.Kind == MenuButtonEntryItem && !entry.Disabled
}

func (m *MenuButton) activateEntry(index int) {
	if !m.entryIsSelectable(index) {
		return
	}
	entry := m.cachedEntryLayouts[index].entry
	m.Activated.Emit(entryKey(entry))
	m.SetOpen(false)
}

func (m *MenuButton) toggleOpen() {
	m.SetOpen(!m.Open)
}

func (m *MenuButton) syncFocusIndex() {
	if !m.Open {
		m.focusedIndex = -1
		return
	}
	if m.focusedIndex >= 0 && m.focusedIndex < len(m.cachedEntryLayouts) && m.entryIsSelectable(m.focusedIndex) {
		return
	}
	m.focusedIndex = m.firstSelectableIndex()
}

func (m *MenuButton) firstSelectableIndex() int {
	for i := range m.cachedEntryLayouts {
		if m.entryIsSelectable(i) {
			return i
		}
	}
	return -1
}

func (m *MenuButton) lastSelectableIndex() int {
	for i := len(m.cachedEntryLayouts) - 1; i >= 0; i-- {
		if m.entryIsSelectable(i) {
			return i
		}
	}
	return -1
}

func (m *MenuButton) navigateOpen(key platform.Key) {
	if len(m.cachedEntryLayouts) == 0 {
		return
	}
	if m.focusedIndex < 0 {
		m.focusedIndex = m.firstSelectableIndex()
	}
	switch key {
	case platform.KeyHome:
		m.focusedIndex = m.firstSelectableIndex()
	case platform.KeyEnd:
		m.focusedIndex = m.lastSelectableIndex()
	case platform.KeyUp:
		for i := m.focusedIndex - 1; i >= 0; i-- {
			if m.entryIsSelectable(i) {
				m.focusedIndex = i
				break
			}
		}
	case platform.KeyDown:
		for i := m.focusedIndex + 1; i < len(m.cachedEntryLayouts); i++ {
			if m.entryIsSelectable(i) {
				m.focusedIndex = i
				break
			}
		}
	}
	m.invalidate(facet.DirtyProjection)
}

func (m *MenuButton) entryState(index int) theme.InteractionState {
	if index < 0 || index >= len(m.cachedEntryLayouts) {
		return theme.StateDefault
	}
	entry := m.cachedEntryLayouts[index].entry
	switch {
	case entry.Disabled:
		return theme.StateDisabled
	case m.pressedIndex == index:
		return theme.StatePressed
	case m.hoveredIndex == index:
		return theme.StateHover
	case m.Open && m.focusedIndex == index:
		return theme.StateFocused
	case entry.Selected:
		return theme.StateSelected
	default:
		return theme.StateDefault
	}
}

func (m *MenuButton) newShaper(runtime any) *text.Shaper {
	registry := m.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (m *MenuButton) fontRegistry(runtime any) *text.FontRegistry {
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

func normalizeMenuButtonEntries(entries []MenuButtonEntry) []MenuButtonEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]MenuButtonEntry, len(entries))
	for i := range entries {
		out[i] = normalizeMenuButtonEntry(entries[i])
	}
	return out
}

func normalizeMenuButtonEntry(entry MenuButtonEntry) MenuButtonEntry {
	entry.Key = strings.TrimSpace(entry.Key)
	entry.Label = strings.TrimSpace(entry.Label)
	entry.AccessibleLabel = strings.TrimSpace(entry.AccessibleLabel)
	entry.IconRef = strings.TrimSpace(entry.IconRef)
	entry.Shortcut = strings.TrimSpace(entry.Shortcut)
	if entry.Kind != MenuButtonEntryItem {
		entry.Key = ""
		entry.Disabled = false
		entry.Selected = false
		entry.Destructive = false
		return entry
	}
	if entry.Key == "" {
		switch {
		case entry.AccessibleLabel != "":
			entry.Key = entry.AccessibleLabel
		case entry.Label != "":
			entry.Key = entry.Label
		case entry.Shortcut != "":
			entry.Key = entry.Shortcut
		}
	}
	if entry.AccessibleLabel == "" {
		if entry.Label != "" {
			entry.AccessibleLabel = entry.Label
		} else {
			entry.AccessibleLabel = entry.Key
		}
	}
	return entry
}

func entryKey(entry MenuButtonEntry) string {
	if name := strings.TrimSpace(entry.Key); name != "" {
		return name
	}
	if name := strings.TrimSpace(entry.AccessibleLabel); name != "" {
		return name
	}
	return strings.TrimSpace(entry.Label)
}

func iconAssetCommands(runtime any, ref string, bounds gfx.Rect, material theme.Material) []gfx.Command {
	if runtime == nil || ref == "" || bounds.IsEmpty() || isTransparentMaterial(material) {
		return nil
	}
	type iconProvider interface {
		IconResolver() runtimepkg.IconResolver
	}
	var (
		asset runtimepkg.IconAsset
		ok    bool
	)
	if provider, okProvider := runtime.(iconProvider); okProvider {
		if resolver := provider.IconResolver(); resolver != nil {
			asset, ok = resolver.ResolveIcon(ref)
		}
	}
	if !ok {
		if resolver, okResolver := runtime.(interface {
			ResolveIcon(string) (runtimepkg.IconAsset, bool)
		}); okResolver {
			asset, ok = resolver.ResolveIcon(ref)
		}
	}
	if !ok || len(asset.Path.Segments) == 0 {
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

func menuButtonChevronPath(bounds gfx.Rect) gfx.Path {
	if bounds.IsEmpty() {
		return gfx.Path{}
	}
	return gfx.NewPath().
		MoveTo(gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y + bounds.Height()*0.35}).
		LineTo(gfx.Point{X: bounds.Min.X + bounds.Width()*0.5, Y: bounds.Max.Y}).
		LineTo(gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y + bounds.Height()*0.35}).
		Build()
}

func menuButtonCheckmarkPath(bounds gfx.Rect) gfx.Path {
	if bounds.IsEmpty() {
		return gfx.Path{}
	}
	return gfx.NewPath().
		MoveTo(gfx.Point{X: bounds.Min.X + bounds.Width()*0.12, Y: bounds.Min.Y + bounds.Height()*0.55}).
		LineTo(gfx.Point{X: bounds.Min.X + bounds.Width()*0.38, Y: bounds.Min.Y + bounds.Height()*0.80}).
		LineTo(gfx.Point{X: bounds.Min.X + bounds.Width()*0.84, Y: bounds.Min.Y + bounds.Height()*0.24}).
		Build()
}

func tintColor(color gfx.Color, alpha float32) gfx.Color {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	color.A = alpha
	return color
}

func sumEntryHeights(entries []menuButtonEntryLayout, gap float32) float32 {
	if len(entries) == 0 {
		return 0
	}
	total := float32(0)
	for i := range entries {
		total += entries[i].height
		if i > 0 {
			total += gap
		}
	}
	return total
}

type menuButtonGroupPolicy struct {
	button *MenuButton
}

func (menuButtonGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (p menuButtonGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.button == nil {
		return facet.GroupMeasureResult{}, nil
	}
	return facet.GroupMeasureResult{Size: p.button.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}}).Size}, nil
}

func (p menuButtonGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.button == nil {
		return nil, nil
	}
	p.button.arrange(ctx.ArrangeContext, ctx.Bounds)
	return nil, nil
}
