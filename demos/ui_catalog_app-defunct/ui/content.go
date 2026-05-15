package ui

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/model"
	"codeburg.org/lexbit/ui_catalog/store"
)

// ViewMode determines how content is displayed.
type ViewMode int

const (
	ViewGrid   ViewMode = iota // Grid of cards
	ViewDetail                 // Detail view for selected entry
)

type contentLayoutState struct {
	bounds   gfx.Rect
	inner    gfx.Rect
	columns  int
	sections []contentSectionLayout
}

type contentSectionLayout struct {
	family model.Family
	header gfx.Rect
	cards  []gfx.Rect
}

const contentCardsLayerID layout.LayerID = 1

type contentRuntime interface {
	AddFacet(parent, child facet.FacetImpl, attachment layout.ChildAttachment)
	RemoveFacet(child facet.FacetImpl)
	ResolveChildAttachment(id facet.FacetID) (layout.ChildAttachment, bool)
	UpdateChildAttachment(child facet.FacetImpl, attachment layout.ChildAttachment)
	RequestFrame()
}

// ContentFacet displays the main content area with a grid of cards.
type ContentFacet struct {
	facet.Facet
	layout        facet.LayoutRole
	render        facet.RenderRole
	input         facet.InputRole
	hit           facet.HitRole
	focus         facet.FocusRole
	th            theme.Context
	shaper        *text.Shaper
	filterSub     signal.SubscriptionID
	selectionSub  signal.SubscriptionID
	cards         []*CardFacet
	viewMode      ViewMode
	focusedCard   int  // Index of currently focused card for keyboard navigation
	loading       bool // Show loading state
	scrollOffset  float32
	maxScroll     float32
	layoutState   contentLayoutState
	layoutProfile LayoutProfile
	runtime       contentRuntime
}

// NewContentFacet creates a new content facet.
func NewContentFacet(th theme.Context, shaper *text.Shaper) *ContentFacet {
	c := &ContentFacet{
		Facet:         facet.NewFacet(),
		th:            th,
		shaper:        shaper,
		layoutProfile: DefaultLayoutProfile(),
	}

	c.layout.OnMeasure = func(cons facet.Constraints) gfx.Size {
		return gfx.Size{W: cons.MaxSize.W, H: cons.MaxSize.H}
	}

	c.layout.OnArrange = func(bounds gfx.Rect) {
		c.layout.ArrangedBounds = bounds
		c.reflow(bounds)
	}
	c.AddRole(&c.layout)

	c.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		c.renderContent(list, bounds)
	}
	c.AddRole(&c.render)

	c.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		if c.layout.ArrangedBounds.Contains(p) {
			cursor := facet.CursorDefault
			if c.hitCardAt(p) >= 0 {
				cursor = facet.CursorPointer
			}
			return facet.HitResult{Hit: true, Cursor: cursor}
		}
		return facet.HitResult{}
	}
	c.AddRole(&c.hit)

	// Keyboard navigation
	c.input.OnKey = func(e facet.KeyEvent) bool {
		return c.handleKeyEvent(e)
	}
	c.input.OnPointer = func(e facet.PointerEvent) bool {
		if e.Kind != platform.PointerRelease || e.Button != platform.PointerLeft {
			return false
		}
		if idx := c.hitCardAt(e.Position); idx >= 0 {
			c.selectCardIndex(idx)
			return true
		}
		return c.layout.ArrangedBounds.Contains(e.Position)
	}
	c.input.OnScroll = func(e facet.ScrollEvent) bool {
		return c.handleScrollEvent(e)
	}
	c.AddRole(&c.input)

	c.focus.Focusable = func() bool { return true }
	c.focus.OnFocusGained = func() {
		c.syncFocusedCardFromSelection()
	}
	c.focus.OnFocusLost = func() {}
	c.AddRole(&c.focus)

	return c
}

// Base returns the base facet.
func (f *ContentFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

// OnAttach handles attachment.
func (f *ContentFacet) OnAttach(ctx facet.AttachContext) {
	if rt, ok := ctx.Runtime.(contentRuntime); ok {
		f.runtime = rt
	}
	// Subscribe to filter changes
	f.filterSub = store.FilterStore.OnChange.Subscribe(func(change signal.Change[store.FilterState]) {
		f.syncCards()
		f.Invalidate(facet.DirtyProjection)
	})
	// Subscribe to selection changes
	f.selectionSub = store.SelectionStore.OnChange.Subscribe(func(change signal.Change[string]) {
		f.syncCards()
		f.Invalidate(facet.DirtyProjection)
	})
	f.syncCards()
}

// OnDetach handles detachment.
func (f *ContentFacet) OnDetach() {
	store.FilterStore.OnChange.Unsubscribe(f.filterSub)
	store.SelectionStore.OnChange.Unsubscribe(f.selectionSub)
	if f.runtime != nil {
		for _, card := range f.cards {
			if card != nil {
				f.runtime.RemoveFacet(card)
			}
		}
	}
	f.cards = nil
	f.runtime = nil
}

// SetViewMode switches between grid and detail views.
func (f *ContentFacet) SetViewMode(mode ViewMode) {
	if f.viewMode != mode {
		f.viewMode = mode
		f.Invalidate(facet.DirtyProjection)
	}
}

// SetLayoutProfile updates density-driven layout geometry.
func (f *ContentFacet) SetLayoutProfile(profile LayoutProfile) {
	if f == nil {
		return
	}
	f.layoutProfile = profile
	for _, card := range f.cards {
		card.SetLayoutProfile(profile)
	}
	f.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// OnLayerSpecs exposes a child layer for the live card facets.
func (f *ContentFacet) OnLayerSpecs() []layout.LayerSpec {
	bounds := f.layout.ArrangedBounds
	if bounds.IsEmpty() {
		return nil
	}
	inner := Inset(bounds, f.layoutProfile.ContentPadding)
	if inner.IsEmpty() {
		return nil
	}
	return []layout.LayerSpec{{
		ID:          contentCardsLayerID,
		Placement:   layout.PlacementFree,
		Measurement: layout.MeasureNonStructural,
		CoordSpace:  layout.CoordParentLayout,
		CoordLimits: layout.CoordLimits{Bounds: inner},
		HitPolicy:   layout.HitNormal,
		RenderOrder: 10,
		ClipPolicy:  layout.ClipToParent,
	}}
}

// OnActivate handles activation.
func (f *ContentFacet) OnActivate() {}

// OnDeactivate handles deactivation.
func (f *ContentFacet) OnDeactivate() {}

// syncCards updates the card facets to match filtered entries.
func (f *ContentFacet) syncCards() {
	entries := store.FilteredEntries.Get()

	// Reuse existing cards where possible
	existingCards := make(map[string]*CardFacet)
	for _, card := range f.cards {
		if card.Entry() != nil {
			existingCards[card.Entry().ID] = card
		}
	}

	selectedID := store.SelectionStore.Get()
	profile := f.layoutProfile
	if profile.CardWidth <= 0 || profile.CardHeight <= 0 {
		profile = DefaultLayoutProfile()
	}

	// Build new card list
	newCards := make([]*CardFacet, 0, len(entries))
	for _, entry := range entries {
		if card, ok := existingCards[entry.ID]; ok {
			// Update selection state
			card.SetLayoutProfile(profile)
			card.SetSelected(entry.ID == selectedID)
			card.SetFocused(false)
			newCards = append(newCards, card)
			delete(existingCards, entry.ID)
		} else {
			// Create new card
			card := NewCardFacet(f.th, f.shaper, entry)
			card.SetLayoutProfile(profile)
			card.SetSelected(entry.ID == selectedID)
			card.SetFocused(false)
			card.SetOnClick(func() {
				store.SelectEntry(entry.ID)
			})
			if f.runtime != nil {
				f.runtime.AddFacet(f, card, contentCardAttachment(f.layout.ArrangedBounds, card.layout.ArrangedBounds))
			}
			newCards = append(newCards, card)
		}
	}

	// Dispose unused cards
	for _, card := range existingCards {
		if f.runtime != nil {
			f.runtime.RemoveFacet(card)
		} else {
			card.OnDetach()
		}
	}

	f.cards = newCards
	f.reflow(f.layout.ArrangedBounds)
	f.syncFocusedCardFromSelection()
	selectedID = store.SelectionStore.Get()
	f.syncCardStates(selectedID)
}

// reflow calculates the local content viewport and card placement.
func (f *ContentFacet) reflow(bounds gfx.Rect) {
	f.layoutState = contentLayoutState{bounds: bounds}
	profile := f.layoutProfile
	if profile.CardWidth <= 0 || profile.CardHeight <= 0 {
		profile = DefaultLayoutProfile()
	}
	if bounds.IsEmpty() {
		return
	}

	inner := Inset(bounds, profile.ContentPadding)
	if inner.IsEmpty() {
		return
	}
	f.layoutState.inner = inner

	// Calculate grid dimensions
	availableWidth := inner.Width()
	cols := int(availableWidth / (profile.CardWidth + profile.CardMargin))
	if cols < 1 {
		cols = 1
	}
	f.layoutState.columns = cols

	// Group cards by family
	cardsByFamily := make(map[model.Family][]*CardFacet)
	familyOrder := make([]model.Family, 0)
	for _, card := range f.cards {
		family := card.entry.Family
		if _, exists := cardsByFamily[family]; !exists {
			familyOrder = append(familyOrder, family)
		}
		cardsByFamily[family] = append(cardsByFamily[family], card)
	}

	// Layout cards with family headers
	y := inner.Min.Y
	sections := make([]contentSectionLayout, 0, len(familyOrder))

	for _, family := range familyOrder {
		cards := cardsByFamily[family]
		if len(cards) == 0 {
			continue
		}

		section := contentSectionLayout{
			family: family,
			header: gfx.RectFromXYWH(inner.Min.X, y, inner.Width(), profile.FamilyHeaderHeight),
		}
		y += profile.FamilyHeaderHeight + profile.FieldGap

		// Layout cards in this family
		x := inner.Min.X
		for i, card := range cards {
			if i > 0 && i%cols == 0 {
				// New row
				x = inner.Min.X
				y += profile.CardHeight + profile.CardMargin
			}

			cardBounds := gfx.RectFromXYWH(x, y, profile.CardWidth, profile.CardHeight)
			card.layout.ArrangedBounds = cardBounds
			card.bounds = cardBounds
			f.updateCardAttachment(card, inner, cardBounds)
			section.cards = append(section.cards, cardBounds)

			x += profile.CardWidth + profile.CardMargin
		}

		// Add spacing after family group
		y += profile.CardHeight + profile.CardMargin + profile.FieldGap*4
		sections = append(sections, section)
	}

	f.layoutState.sections = sections
	f.maxScroll = totalContentHeight(inner, y)
	f.clampScrollOffset()
	f.applyScrollOffset()
}

func (f *ContentFacet) renderContent(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	profile := f.layoutProfile
	if profile.CardWidth <= 0 || profile.CardHeight <= 0 {
		profile = DefaultLayoutProfile()
	}

	inner := Inset(bounds, profile.ContentPadding)
	if inner.IsEmpty() {
		return
	}

	if f.shaper == nil {
		return
	}

	// Loading state
	if f.loading {
		f.renderLoadingState(list, inner, profile)
		return
	}

	// Empty state
	if len(f.cards) == 0 {
		f.renderEmptyState(list, inner, profile)
		return
	}

	switch f.viewMode {
	case ViewDetail:
		f.renderDetailView(list, inner, profile)
	default:
		f.renderGridView(list, inner, profile)
	}
}

func (f *ContentFacet) renderGridView(list *gfx.CommandList, bounds gfx.Rect, profile LayoutProfile) {
	// Render header with count
	countText := fmt.Sprintf("%d entries", len(f.cards))
	countStyle := f.th.TextStyle(theme.TextBodyM)
	countLayout := f.shaper.ShapeSimple(countText, countStyle)
	if countLayout != nil && len(countLayout.Lines) > 0 {
		line := countLayout.Lines[0]
		drawTextLine(list, bounds.Min.X, bounds.Min.Y, line, f.th.Color(theme.ColorText))
	}

	for _, section := range f.layoutState.sections {
		if section.header.IsEmpty() {
			continue
		}
		label := section.family.DisplayName()
		labelLayout := f.shaper.ShapeSimple(label, f.th.TextStyle(theme.TextLabelS))
		if labelLayout != nil && len(labelLayout.Lines) > 0 {
			line := labelLayout.Lines[0]
			drawTextLine(list, section.header.Min.X, section.header.Min.Y, line, f.th.Color(theme.ColorTextSecondary))
		}
	}
}

func (f *ContentFacet) applyScrollOffset() {
	offset := f.scrollOffset
	for _, section := range f.layoutState.sections {
		section.header = section.header.Offset(0, -offset)
		for i := range section.cards {
			section.cards[i] = section.cards[i].Offset(0, -offset)
		}
	}
	for _, card := range f.cards {
		if card == nil || card.layout.ArrangedBounds.IsEmpty() {
			continue
		}
		card.layout.ArrangedBounds = card.layout.ArrangedBounds.Offset(0, -offset)
		card.bounds = card.layout.ArrangedBounds
		f.updateCardAttachment(card, f.layoutState.inner, card.layout.ArrangedBounds)
	}
}

func (f *ContentFacet) clampScrollOffset() {
	if f.maxScroll < 0 {
		f.maxScroll = 0
	}
	if f.scrollOffset < 0 {
		f.scrollOffset = 0
	}
	if f.scrollOffset > f.maxScroll {
		f.scrollOffset = f.maxScroll
	}
}

func (f *ContentFacet) handleScrollEvent(e facet.ScrollEvent) bool {
	if f == nil || f.layout.ArrangedBounds.IsEmpty() {
		return false
	}
	if e.DeltaY == 0 {
		return false
	}
	next := f.scrollOffset - e.DeltaY*32
	if next < 0 {
		next = 0
	}
	if next > f.maxScroll {
		next = f.maxScroll
	}
	if next == f.scrollOffset {
		return false
	}
	f.scrollOffset = next
	f.applyScrollOffset()
	f.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	return true
}

func (f *ContentFacet) syncCardStates(selectedID string) {
	focusedIndex := f.focusedCard
	if selectedID != "" {
		for i, card := range f.cards {
			if card.Entry() != nil && card.Entry().ID == selectedID {
				focusedIndex = i
				break
			}
		}
	}
	for i, card := range f.cards {
		selected := card.Entry() != nil && card.Entry().ID == selectedID
		card.SetSelected(selected)
		card.SetFocused(i == focusedIndex && focusedIndex >= 0)
	}
	if focusedIndex >= 0 && focusedIndex < len(f.cards) {
		f.focusedCard = focusedIndex
	}
}

func contentCardAttachment(layerBounds, cardBounds gfx.Rect) layout.ChildAttachment {
	offset := gfx.Point{}
	if !layerBounds.IsEmpty() && !cardBounds.IsEmpty() {
		offset = gfx.Point{
			X: cardBounds.Min.X - layerBounds.Min.X,
			Y: cardBounds.Min.Y - layerBounds.Min.Y,
		}
	}
	return layout.ChildAttachment{
		LayerID: contentCardsLayerID,
		Placement: layout.PlacementHints{
			FreeAnchor: layout.FreeTopLeft,
			Offset:     offset,
		},
	}
}

func (f *ContentFacet) updateCardAttachment(card *CardFacet, layerBounds, cardBounds gfx.Rect) {
	if f == nil || f.runtime == nil || card == nil || card.Base() == nil {
		return
	}
	next := contentCardAttachment(layerBounds, cardBounds)
	if current, ok := f.runtime.ResolveChildAttachment(card.Base().ID()); ok && current == next {
		return
	}
	f.runtime.UpdateChildAttachment(card, next)
}

func (f *ContentFacet) syncFocusedCardFromSelection() {
	selectedID := store.SelectionStore.Get()
	if selectedID == "" {
		if f.focusedCard >= len(f.cards) {
			f.focusedCard = -1
		}
		return
	}
	for i, card := range f.cards {
		if card.Entry() != nil && card.Entry().ID == selectedID {
			f.focusedCard = i
			return
		}
	}
}

func (f *ContentFacet) renderDetailView(list *gfx.CommandList, bounds gfx.Rect, profile LayoutProfile) {
	// Get selected entry
	selectedID := store.SelectionStore.Get()
	var selectedEntry *model.CatalogEntry
	for _, card := range f.cards {
		if card.Entry().ID == selectedID {
			selectedEntry = card.Entry()
			break
		}
	}

	if selectedEntry == nil {
		// No selection, show grid instead
		f.renderGridView(list, bounds, profile)
		return
	}

	// Check if we're in compare mode
	if store.IsCompareMode() {
		f.renderCompareView(list, bounds, selectedEntry)
		return
	}

	// Render back button hint
	y := bounds.Min.Y
	hintText := "← Grid view"
	hintStyle := f.th.TextStyle(theme.TextLabelS)
	hintLayout := f.shaper.ShapeSimple(hintText, hintStyle)
	if hintLayout != nil && len(hintLayout.Lines) > 0 {
		line := hintLayout.Lines[0]
		drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
		y += hintLayout.Bounds.Height() + 16
	}

	// Render entry ID as title
	titleStyle := f.th.TextStyle(theme.TextHeadingS)
	titleLayout := f.shaper.ShapeSimple(selectedEntry.ID, titleStyle)
	if titleLayout != nil && len(titleLayout.Lines) > 0 {
		line := titleLayout.Lines[0]
		drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorPrimary))
		y += titleLayout.Bounds.Height() + 8
	}

	// Render display name
	nameStyle := f.th.TextStyle(theme.TextBodyM)
	nameLayout := f.shaper.ShapeSimple(selectedEntry.DisplayName, nameStyle)
	if nameLayout != nil && len(nameLayout.Lines) > 0 {
		line := nameLayout.Lines[0]
		drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorText))
		y += nameLayout.Bounds.Height() + 16
	}

	// Render properties
	y = f.renderDetailProperty(list, bounds, y, "Family", selectedEntry.Family.DisplayName(), profile)
	y = f.renderDetailProperty(list, bounds, y, "Coverage", selectedEntry.Coverage.DisplayName(), profile)
	y = f.renderDetailProperty(list, bounds, y, "Interactive", fmt.Sprintf("%v", selectedEntry.Interactive), profile)
	y = f.renderDetailProperty(list, bounds, y, "Theme Sensitive", fmt.Sprintf("%v", selectedEntry.ThemeSensitive), profile)

	// Notes
	if selectedEntry.Notes != "" {
		y += profile.FieldGap * 4
		notesLabelStyle := f.th.TextStyle(theme.TextLabelS)
		notesLabelLayout := f.shaper.ShapeSimple("Notes:", notesLabelStyle)
		if notesLabelLayout != nil && len(notesLabelLayout.Lines) > 0 {
			line := notesLabelLayout.Lines[0]
			drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
			y += notesLabelLayout.Bounds.Height() + profile.FieldGap
		}

		notesStyle := f.th.TextStyle(theme.TextBodyS)
		notesLayout := f.shaper.ShapeSimple(selectedEntry.Notes, notesStyle)
		if notesLayout != nil {
			for _, line := range notesLayout.Lines {
				drawTextLine(list, bounds.Min.X+profile.FieldLabelWidth/5, y, line, f.th.Color(theme.ColorText))
				y += notesLayout.Bounds.Height()
			}
		}
	}
}

func (f *ContentFacet) renderDetailProperty(list *gfx.CommandList, bounds gfx.Rect, y float32, name, value string, profile LayoutProfile) float32 {
	// Name
	nameStyle := f.th.TextStyle(theme.TextLabelS)
	nameText := name + ":"
	nameLayout := f.shaper.ShapeSimple(nameText, nameStyle)
	if nameLayout != nil && len(nameLayout.Lines) > 0 {
		line := nameLayout.Lines[0]
		drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
	}

	// Value
	valueStyle := f.th.TextStyle(theme.TextBodyS)
	valueLayout := f.shaper.ShapeSimple(value, valueStyle)
	if valueLayout != nil && len(valueLayout.Lines) > 0 {
		line := valueLayout.Lines[0]
		x := bounds.Min.X + profile.FieldLabelWidth
		drawTextLine(list, x, y, line, f.th.Color(theme.ColorText))
		if valueLayout.Bounds.Height() > nameLayout.Bounds.Height() {
			return y + valueLayout.Bounds.Height() + profile.FieldGap*2
		}
	}

	return y + nameLayout.Bounds.Height() + profile.FieldGap*2
}

func (f *ContentFacet) renderLoadingState(list *gfx.CommandList, bounds gfx.Rect, profile LayoutProfile) {
	// Loading spinner (simple rotating indicator)
	centerX := bounds.Min.X + bounds.Width()/2
	centerY := bounds.Min.Y + bounds.Height()/2
	spinnerRadius := float32(20)
	spinnerColor := f.th.Color(theme.ColorPrimary)

	// Draw a simple circle outline as loading indicator
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(centerX-spinnerRadius, centerY-spinnerRadius, spinnerRadius*2, spinnerRadius*2),
		Brush: gfx.SolidBrush(spinnerColor),
	})

	// Loading text
	message := "Loading..."
	msgStyle := f.th.TextStyle(theme.TextBodyM)
	msgLayout := f.shaper.ShapeSimple(message, msgStyle)
	if msgLayout != nil && len(msgLayout.Lines) > 0 {
		line := msgLayout.Lines[0]
		x := bounds.Min.X + (bounds.Width()-line.Bounds.Width())/2
		y := centerY + spinnerRadius + profile.FieldGap*4
		drawTextLine(list, x, y, line, f.th.Color(theme.ColorTextSecondary))
	}
}

func (f *ContentFacet) renderEmptyState(list *gfx.CommandList, bounds gfx.Rect, profile LayoutProfile) {
	// Empty icon (simple square outline)
	centerX := bounds.Min.X + bounds.Width()/2
	centerY := bounds.Min.Y + bounds.Height()/2 - 20
	iconSize := float32(48)
	iconColor := f.th.Color(theme.ColorTextSecondary)

	// Draw empty box icon
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(centerX-iconSize/2, centerY-iconSize/2, iconSize, 2),
		Brush: gfx.SolidBrush(iconColor),
	})
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(centerX-iconSize/2, centerY-iconSize/2, 2, iconSize),
		Brush: gfx.SolidBrush(iconColor),
	})
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(centerX+iconSize/2-2, centerY-iconSize/2, 2, iconSize),
		Brush: gfx.SolidBrush(iconColor),
	})
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(centerX-iconSize/2, centerY+iconSize/2-2, iconSize, 2),
		Brush: gfx.SolidBrush(iconColor),
	})

	// Empty message
	message := "No entries match the current filters"
	msgStyle := f.th.TextStyle(theme.TextBodyM)
	msgLayout := f.shaper.ShapeSimple(message, msgStyle)
	if msgLayout != nil && len(msgLayout.Lines) > 0 {
		line := msgLayout.Lines[0]
		x := bounds.Min.X + (bounds.Width()-line.Bounds.Width())/2
		y := centerY + iconSize/2 + profile.FieldGap*4
		drawTextLine(list, x, y, line, f.th.Color(theme.ColorTextSecondary))
	}
}

// handleKeyEvent handles keyboard navigation for grid view
func (f *ContentFacet) handleKeyEvent(e facet.KeyEvent) bool {
	entries := store.FilteredEntries.Get()
	if len(entries) == 0 {
		return false
	}
	if f.focusedCard < 0 || f.focusedCard >= len(entries) {
		selectedID := store.SelectionStore.Get()
		f.focusedCard = -1
		if selectedID != "" {
			for i, entry := range entries {
				if entry.ID == selectedID {
					f.focusedCard = i
					break
				}
			}
		}
		if f.focusedCard < 0 {
			f.focusedCard = 0
		}
	}

	// Calculate columns based on card width and margin
	bounds := f.layout.ArrangedBounds
	profile := f.layoutProfile
	if profile.CardWidth <= 0 || profile.CardHeight <= 0 {
		profile = DefaultLayoutProfile()
	}
	availableWidth := bounds.Width() - 2*profile.ContentPadding
	columns := int(availableWidth / (profile.CardWidth + profile.CardMargin))
	if columns < 1 {
		columns = 1
	}

	switch e.Key {
	case platform.KeyDown:
		// Move down in grid
		if f.focusedCard+columns < len(entries) {
			f.focusedCard += columns
			f.selectFocusedCard(entries)
			return true
		}
	case platform.KeyUp:
		// Move up in grid
		if f.focusedCard-columns >= 0 {
			f.focusedCard -= columns
			f.selectFocusedCard(entries)
			return true
		}
	case platform.KeyRight:
		// Move right in grid
		if f.focusedCard+1 < len(entries) {
			f.focusedCard++
			f.selectFocusedCard(entries)
			return true
		}
	case platform.KeyLeft:
		// Move left in grid
		if f.focusedCard > 0 {
			f.focusedCard--
			f.selectFocusedCard(entries)
			return true
		}
	case platform.KeyEnter:
		// Select focused card
		if f.focusedCard < len(entries) {
			f.selectFocusedCard(entries)
			return true
		}
	}
	return false
}

func (f *ContentFacet) hitCardAt(p gfx.Point) int {
	for i, card := range f.cards {
		if card != nil && card.layout.ArrangedBounds.Contains(p) {
			return i
		}
	}
	return -1
}

// renderCompareView renders the selected entry in side-by-side comparison mode.
func (f *ContentFacet) renderCompareView(list *gfx.CommandList, bounds gfx.Rect, entry *model.CatalogEntry) {
	// Back button hint
	y := bounds.Min.Y
	hintText := "← Grid view | Compare Mode"
	hintStyle := f.th.TextStyle(theme.TextLabelS)
	hintLayout := f.shaper.ShapeSimple(hintText, hintStyle)
	if hintLayout != nil && len(hintLayout.Lines) > 0 {
		line := hintLayout.Lines[0]
		drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
		y += hintLayout.Bounds.Height() + 16
	}

	// Title with compare info
	compareMode := store.GetCompareMode()
	compareTheme := store.GetCompareTheme()
	title := fmt.Sprintf("%s - %s vs %s", entry.ID, store.GetTheme().String(), compareTheme.String())
	titleStyle := f.th.TextStyle(theme.TextHeadingS)
	titleLayout := f.shaper.ShapeSimple(title, titleStyle)
	if titleLayout != nil && len(titleLayout.Lines) > 0 {
		line := titleLayout.Lines[0]
		drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorPrimary))
		y += titleLayout.Bounds.Height() + 16
	}

	// Calculate available space for cards
	availableHeight := bounds.Max.Y - y - 16
	profile := f.layoutProfile
	if profile.CardWidth <= 0 || profile.CardHeight <= 0 {
		profile = DefaultLayoutProfile()
	}
	if availableHeight < profile.CardHeight {
		availableHeight = profile.CardHeight
	}
	currentThemeLayout := f.shaper.ShapeSimple(store.GetTheme().String(), f.th.TextStyle(theme.TextLabelS))
	compareThemeLayout := f.shaper.ShapeSimple(compareTheme.String(), f.th.TextStyle(theme.TextLabelS))

	switch compareMode {
	case store.CompareSideBySide:
		// Side by side: current theme | compare theme
		availableWidth := bounds.Width() - (profile.CompareInnerPad * 2) - profile.ComparePanelGap
		halfWidth := availableWidth / 2

		// Left panel - current theme
		leftBounds := gfx.RectFromXYWH(bounds.Min.X+profile.CompareInnerPad, y, halfWidth, availableHeight)
		list.Add(gfx.FillRect{
			Rect:  leftBounds,
			Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
		})
		if currentThemeLayout != nil && len(currentThemeLayout.Lines) > 0 {
			drawTextLine(list, leftBounds.Min.X+profile.CompareInnerPad, y+profile.CompareInnerPad,
				currentThemeLayout.Lines[0],
				f.th.Color(theme.ColorText))
		}
		// Draw a simplified card representation
		cardBounds := gfx.RectFromXYWH(leftBounds.Min.X+profile.CompareInnerPad, y+profile.HeaderInset*1.5, profile.CardWidth, profile.CardHeight)
		f.renderMiniCard(list, cardBounds, entry, f.th, profile)

		// Right panel - compare theme
		rightBounds := gfx.RectFromXYWH(bounds.Min.X+profile.CompareInnerPad+halfWidth+profile.ComparePanelGap, y, halfWidth, availableHeight)
		list.Add(gfx.FillRect{
			Rect:  rightBounds,
			Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
		})
		if compareThemeLayout != nil && len(compareThemeLayout.Lines) > 0 {
			drawTextLine(list, rightBounds.Min.X+profile.CompareInnerPad, y+profile.CompareInnerPad,
				compareThemeLayout.Lines[0],
				f.th.Color(theme.ColorText))
		}
		// Draw the same card with different theme indication
		compareBounds := gfx.RectFromXYWH(rightBounds.Min.X+profile.CompareInnerPad, y+profile.HeaderInset*1.5, profile.CardWidth, profile.CardHeight)
		f.renderMiniCard(list, compareBounds, entry, f.th, profile)

	case store.CompareStacked:
		// Stacked: current on top, compare below
		halfHeight := (availableHeight - profile.ComparePanelGap) / 2

		// Top panel - current theme
		topBounds := gfx.RectFromXYWH(bounds.Min.X+profile.CompareInnerPad, y, bounds.Width()-profile.CompareInnerPad*2, halfHeight)
		list.Add(gfx.FillRect{
			Rect:  topBounds,
			Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
		})
		if currentThemeLayout != nil && len(currentThemeLayout.Lines) > 0 {
			drawTextLine(list, topBounds.Min.X+profile.CompareInnerPad, y+profile.CompareInnerPad,
				currentThemeLayout.Lines[0],
				f.th.Color(theme.ColorText))
		}
		cardBounds := gfx.RectFromXYWH(topBounds.Min.X+profile.CompareInnerPad, y+profile.HeaderInset*1.5, profile.CardWidth, profile.CardHeight)
		f.renderMiniCard(list, cardBounds, entry, f.th, profile)

		// Bottom panel - compare theme
		bottomY := y + halfHeight + profile.ComparePanelGap
		bottomBounds := gfx.RectFromXYWH(bounds.Min.X+profile.CompareInnerPad, bottomY, bounds.Width()-profile.CompareInnerPad*2, halfHeight)
		list.Add(gfx.FillRect{
			Rect:  bottomBounds,
			Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
		})
		if compareThemeLayout != nil && len(compareThemeLayout.Lines) > 0 {
			drawTextLine(list, bottomBounds.Min.X+profile.CompareInnerPad, bottomY+profile.CompareInnerPad,
				compareThemeLayout.Lines[0],
				f.th.Color(theme.ColorText))
		}
		compareBounds := gfx.RectFromXYWH(bottomBounds.Min.X+profile.CompareInnerPad, bottomY+profile.HeaderInset*1.5, profile.CardWidth, profile.CardHeight)
		f.renderMiniCard(list, compareBounds, entry, f.th, profile)
	}
}

// renderMiniCard renders a simplified card view for compare mode.
func (f *ContentFacet) renderMiniCard(list *gfx.CommandList, bounds gfx.Rect, entry *model.CatalogEntry, th theme.Context, profile LayoutProfile) {
	if profile.CardWidth <= 0 || profile.CardHeight <= 0 {
		profile = DefaultLayoutProfile()
	}
	// Card background
	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(th.Color(theme.ColorSurfaceVariant)),
	})
	// Card border
	borderColor := th.Color(theme.ColorBorder)
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), 1),
		Brush: gfx.SolidBrush(borderColor),
	})
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Max.Y-1, bounds.Width(), 1),
		Brush: gfx.SolidBrush(borderColor),
	})
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, 1, bounds.Height()),
		Brush: gfx.SolidBrush(borderColor),
	})
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Max.X-1, bounds.Min.Y, 1, bounds.Height()),
		Brush: gfx.SolidBrush(borderColor),
	})

	// Entry ID
	textStyle := th.TextStyle(theme.TextBodyS)
	textLayout := f.shaper.ShapeSimple(entry.ID, textStyle)
	if textLayout != nil && len(textLayout.Lines) > 0 {
		line := textLayout.Lines[0]
		drawTextLine(list, bounds.Min.X+profile.CompareInnerPad, bounds.Min.Y+profile.HeaderInset, line, th.Color(theme.ColorText))
	}

	// Family badge
	familyText := entry.Family.String()
	familyStyle := th.TextStyle(theme.TextLabelS)
	familyLayout := f.shaper.ShapeSimple(familyText, familyStyle)
	if familyLayout != nil && len(familyLayout.Lines) > 0 {
		line := familyLayout.Lines[0]
		drawTextLine(list, bounds.Min.X+profile.CompareInnerPad, bounds.Min.Y+profile.HeaderInset*2, line, th.Color(theme.ColorPrimary))
	}
}

// LayoutState returns the cached local layout model for tests.
func (f *ContentFacet) LayoutState() contentLayoutState {
	return f.layoutState
}

// CardBounds returns the arranged bounds for the card with the given ID.
func (f *ContentFacet) CardBounds(id string) (gfx.Rect, bool) {
	for _, card := range f.cards {
		if card.Entry() != nil && card.Entry().ID == id {
			return card.layout.ArrangedBounds, true
		}
	}
	return gfx.Rect{}, false
}

// selectFocusedCard selects the currently focused card
func (f *ContentFacet) selectFocusedCard(entries []*model.CatalogEntry) {
	if f.focusedCard >= 0 && f.focusedCard < len(entries) {
		store.SelectEntry(entries[f.focusedCard].ID)
		// Update card focus states
		for i, card := range f.cards {
			card.SetFocused(i == f.focusedCard)
		}
	}
}

func totalContentHeight(inner gfx.Rect, y float32) float32 {
	if inner.IsEmpty() {
		return 0
	}
	h := y - inner.Min.Y
	if h < 0 {
		return 0
	}
	return h
}

func (f *ContentFacet) selectCardIndex(index int) {
	if index < 0 || index >= len(f.cards) {
		return
	}
	f.focusedCard = index
	if entry := f.cards[index].Entry(); entry != nil {
		store.SelectEntry(entry.ID)
	}
	for i, card := range f.cards {
		card.SetFocused(i == index)
	}
}
