package ui

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
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

// ContentFacet displays the main content area with a grid of cards.
type ContentFacet struct {
	facet.Facet
	layout       facet.LayoutRole
	render       facet.RenderRole
	input        facet.InputRole
	th           theme.Context
	shaper       *text.Shaper
	filterSub    signal.SubscriptionID
	selectionSub signal.SubscriptionID
	cards        []*CardFacet
	viewMode     ViewMode
	focusedCard  int  // Index of currently focused card for keyboard navigation
	loading      bool // Show loading state
}

// NewContentFacet creates a new content facet.
func NewContentFacet(th theme.Context, shaper *text.Shaper) *ContentFacet {
	c := &ContentFacet{
		Facet:  facet.NewFacet(),
		th:     th,
		shaper: shaper,
	}

	c.layout.OnMeasure = func(cons facet.Constraints) gfx.Size {
		return gfx.Size{W: cons.MaxSize.W, H: cons.MaxSize.H}
	}

	c.layout.OnArrange = func(bounds gfx.Rect) {
		c.layout.ArrangedBounds = bounds
		c.updateCardPositions(bounds)
	}
	c.AddRole(&c.layout)

	c.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		c.renderContent(list, bounds)
	}
	c.AddRole(&c.render)

	// Keyboard navigation
	c.input.OnKey = func(e facet.KeyEvent) bool {
		return c.handleKeyEvent(e)
	}
	c.AddRole(&c.input)

	return c
}

// Base returns the base facet.
func (f *ContentFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

// OnAttach handles attachment.
func (f *ContentFacet) OnAttach(ctx facet.AttachContext) {
	// Subscribe to filter changes
	f.filterSub = store.FilterStore.OnChange.Subscribe(func(change signal.Change[store.FilterState]) {
		f.syncCards()
		f.Invalidate(facet.DirtyProjection)
	})
	// Subscribe to selection changes
	f.selectionSub = store.SelectionStore.OnChange.Subscribe(func(change signal.Change[string]) {
		f.Invalidate(facet.DirtyProjection)
	})
	f.syncCards()
}

// OnDetach handles detachment.
func (f *ContentFacet) OnDetach() {
	store.FilterStore.OnChange.Unsubscribe(f.filterSub)
	store.SelectionStore.OnChange.Unsubscribe(f.selectionSub)
	// Clean up cards
	for _, card := range f.cards {
		card.OnDetach()
	}
	f.cards = nil
}

// SetViewMode switches between grid and detail views.
func (f *ContentFacet) SetViewMode(mode ViewMode) {
	if f.viewMode != mode {
		f.viewMode = mode
		f.Invalidate(facet.DirtyProjection)
	}
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

	// Build new card list
	newCards := make([]*CardFacet, 0, len(entries))
	for _, entry := range entries {
		if card, ok := existingCards[entry.ID]; ok {
			// Update selection state
			card.SetSelected(entry.ID == selectedID)
			newCards = append(newCards, card)
			delete(existingCards, entry.ID)
		} else {
			// Create new card
			card := NewCardFacet(f.th, f.shaper, entry)
			card.SetSelected(entry.ID == selectedID)
			card.SetOnClick(func() {
				store.SelectEntry(entry.ID)
			})
			newCards = append(newCards, card)
		}
	}

	// Dispose unused cards
	for _, card := range existingCards {
		card.OnDetach()
	}

	f.cards = newCards
	f.updateCardPositions(f.layout.ArrangedBounds)
}

// updateCardPositions calculates grid layout for cards.
func (f *ContentFacet) updateCardPositions(bounds gfx.Rect) {
	if bounds.IsEmpty() {
		return
	}

	inner := Inset(bounds, contentPadding)
	if inner.IsEmpty() {
		return
	}

	// Calculate grid dimensions
	availableWidth := inner.Width()
	cols := int(availableWidth / (cardWidth + cardMargin))
	if cols < 1 {
		cols = 1
	}

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
	headerHeight := float32(24)

	for _, family := range familyOrder {
		cards := cardsByFamily[family]
		if len(cards) == 0 {
			continue
		}

		// Family header position (rendered in renderGridView)
		_ = gfx.RectFromXYWH(inner.Min.X, y, inner.Width(), headerHeight)
		y += headerHeight + 4

		// Layout cards in this family
		x := inner.Min.X
		for i, card := range cards {
			if i > 0 && i%cols == 0 {
				// New row
				x = inner.Min.X
				y += cardHeight + cardMargin
			}

			cardBounds := gfx.RectFromXYWH(x, y, cardWidth, cardHeight)
			card.layout.ArrangedBounds = cardBounds

			x += cardWidth + cardMargin
		}

		// Add spacing after family group
		y += cardHeight + cardMargin + 16
	}
}

func (f *ContentFacet) renderContent(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	inner := Inset(bounds, contentPadding)
	if inner.IsEmpty() {
		return
	}

	if f.shaper == nil {
		return
	}

	// Loading state
	if f.loading {
		f.renderLoadingState(list, inner)
		return
	}

	// Empty state
	if len(f.cards) == 0 {
		f.renderEmptyState(list, inner)
		return
	}

	switch f.viewMode {
	case ViewDetail:
		f.renderDetailView(list, inner)
	default:
		f.renderGridView(list, inner)
	}
}

func (f *ContentFacet) renderGridView(list *gfx.CommandList, bounds gfx.Rect) {
	// Render header with count
	countText := fmt.Sprintf("%d entries", len(f.cards))
	countStyle := f.th.TextStyle(theme.TextBodyM)
	countLayout := f.shaper.ShapeSimple(countText, countStyle)
	if countLayout != nil && len(countLayout.Lines) > 0 {
		line := countLayout.Lines[0]
		f.drawTextLine(list, bounds.Min.X, bounds.Min.Y, line, f.th.Color(theme.ColorText))
	}

	// Render cards
	for _, card := range f.cards {
		card.render.OnCollect(list, card.layout.ArrangedBounds)
	}
}

func (f *ContentFacet) renderDetailView(list *gfx.CommandList, bounds gfx.Rect) {
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
		f.renderGridView(list, bounds)
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
		f.drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
		y += hintLayout.Bounds.Height() + 16
	}

	// Render entry ID as title
	titleStyle := f.th.TextStyle(theme.TextHeadingS)
	titleLayout := f.shaper.ShapeSimple(selectedEntry.ID, titleStyle)
	if titleLayout != nil && len(titleLayout.Lines) > 0 {
		line := titleLayout.Lines[0]
		f.drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorPrimary))
		y += titleLayout.Bounds.Height() + 8
	}

	// Render display name
	nameStyle := f.th.TextStyle(theme.TextBodyM)
	nameLayout := f.shaper.ShapeSimple(selectedEntry.DisplayName, nameStyle)
	if nameLayout != nil && len(nameLayout.Lines) > 0 {
		line := nameLayout.Lines[0]
		f.drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorText))
		y += nameLayout.Bounds.Height() + 16
	}

	// Render properties
	y = f.renderDetailProperty(list, bounds, y, "Family", selectedEntry.Family.DisplayName())
	y = f.renderDetailProperty(list, bounds, y, "Coverage", selectedEntry.Coverage.DisplayName())
	y = f.renderDetailProperty(list, bounds, y, "Interactive", fmt.Sprintf("%v", selectedEntry.Interactive))
	y = f.renderDetailProperty(list, bounds, y, "Theme Sensitive", fmt.Sprintf("%v", selectedEntry.ThemeSensitive))

	// Notes
	if selectedEntry.Notes != "" {
		y += 16
		notesLabelStyle := f.th.TextStyle(theme.TextLabelS)
		notesLabelLayout := f.shaper.ShapeSimple("Notes:", notesLabelStyle)
		if notesLabelLayout != nil && len(notesLabelLayout.Lines) > 0 {
			line := notesLabelLayout.Lines[0]
			f.drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
			y += notesLabelLayout.Bounds.Height() + 4
		}

		notesStyle := f.th.TextStyle(theme.TextBodyS)
		notesLayout := f.shaper.ShapeSimple(selectedEntry.Notes, notesStyle)
		if notesLayout != nil {
			for _, line := range notesLayout.Lines {
				f.drawTextLine(list, bounds.Min.X+16, y, line, f.th.Color(theme.ColorText))
				y += notesLayout.Bounds.Height()
			}
		}
	}
}

func (f *ContentFacet) renderDetailProperty(list *gfx.CommandList, bounds gfx.Rect, y float32, name, value string) float32 {
	// Name
	nameStyle := f.th.TextStyle(theme.TextLabelS)
	nameText := name + ":"
	nameLayout := f.shaper.ShapeSimple(nameText, nameStyle)
	if nameLayout != nil && len(nameLayout.Lines) > 0 {
		line := nameLayout.Lines[0]
		f.drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
	}

	// Value
	valueStyle := f.th.TextStyle(theme.TextBodyS)
	valueLayout := f.shaper.ShapeSimple(value, valueStyle)
	if valueLayout != nil && len(valueLayout.Lines) > 0 {
		line := valueLayout.Lines[0]
		x := bounds.Min.X + 120
		f.drawTextLine(list, x, y, line, f.th.Color(theme.ColorText))
		if valueLayout.Bounds.Height() > nameLayout.Bounds.Height() {
			return y + valueLayout.Bounds.Height() + 8
		}
	}

	return y + nameLayout.Bounds.Height() + 8
}

func (f *ContentFacet) renderLoadingState(list *gfx.CommandList, bounds gfx.Rect) {
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
		y := centerY + spinnerRadius + 20
		f.drawTextLine(list, x, y, line, f.th.Color(theme.ColorTextSecondary))
	}
}

func (f *ContentFacet) renderEmptyState(list *gfx.CommandList, bounds gfx.Rect) {
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
		y := centerY + iconSize/2 + 16
		f.drawTextLine(list, x, y, line, f.th.Color(theme.ColorTextSecondary))
	}
}

func (f *ContentFacet) drawTextLine(list *gfx.CommandList, x, y float32, line text.ShapedLine, color gfx.Color) {
	origin := gfx.Point{X: x + line.Bounds.Min.X, Y: y + line.Baseline}
	for _, run := range line.Runs {
		// Draw glyphs...
		_ = run
		_ = origin
	}
}

// handleKeyEvent handles keyboard navigation for grid view
func (f *ContentFacet) handleKeyEvent(e facet.KeyEvent) bool {
	entries := store.FilteredEntries.Get()
	if len(entries) == 0 {
		return false
	}

	// Calculate columns based on card width and margin
	bounds := f.layout.ArrangedBounds
	availableWidth := bounds.Width() - 2*contentPadding
	columns := int(availableWidth / (cardWidth + cardMargin))
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

// renderCompareView renders the selected entry in side-by-side comparison mode.
func (f *ContentFacet) renderCompareView(list *gfx.CommandList, bounds gfx.Rect, entry *model.CatalogEntry) {
	// Back button hint
	y := bounds.Min.Y
	hintText := "← Grid view | Compare Mode"
	hintStyle := f.th.TextStyle(theme.TextLabelS)
	hintLayout := f.shaper.ShapeSimple(hintText, hintStyle)
	if hintLayout != nil && len(hintLayout.Lines) > 0 {
		line := hintLayout.Lines[0]
		f.drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
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
		f.drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorPrimary))
		y += titleLayout.Bounds.Height() + 16
	}

	// Calculate available space for cards
	availableHeight := bounds.Max.Y - y - 16
	if availableHeight < cardHeight {
		availableHeight = cardHeight
	}

	switch compareMode {
	case store.CompareSideBySide:
		// Side by side: current theme | compare theme
		halfWidth := (bounds.Width() - 32) / 2

		// Left panel - current theme
		leftBounds := gfx.RectFromXYWH(bounds.Min.X+8, y, halfWidth, availableHeight)
		list.Add(gfx.FillRect{
			Rect:  leftBounds,
			Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
		})
		f.drawTextLine(list, leftBounds.Min.X+8, y+8,
			f.shaper.ShapeSimple(store.GetTheme().String(), f.th.TextStyle(theme.TextLabelS)).Lines[0],
			f.th.Color(theme.ColorText))
		// Draw a simplified card representation
		cardBounds := gfx.RectFromXYWH(leftBounds.Min.X+8, y+24, cardWidth, cardHeight)
		f.renderMiniCard(list, cardBounds, entry, f.th)

		// Right panel - compare theme
		rightBounds := gfx.RectFromXYWH(bounds.Min.X+halfWidth+24, y, halfWidth, availableHeight)
		list.Add(gfx.FillRect{
			Rect:  rightBounds,
			Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
		})
		f.drawTextLine(list, rightBounds.Min.X+8, y+8,
			f.shaper.ShapeSimple(compareTheme.String(), f.th.TextStyle(theme.TextLabelS)).Lines[0],
			f.th.Color(theme.ColorText))
		// Draw the same card with different theme indication
		compareBounds := gfx.RectFromXYWH(rightBounds.Min.X+8, y+24, cardWidth, cardHeight)
		f.renderMiniCard(list, compareBounds, entry, f.th)

	case store.CompareStacked:
		// Stacked: current on top, compare below
		halfHeight := (availableHeight - 16) / 2

		// Top panel - current theme
		topBounds := gfx.RectFromXYWH(bounds.Min.X+8, y, bounds.Width()-16, halfHeight)
		list.Add(gfx.FillRect{
			Rect:  topBounds,
			Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
		})
		f.drawTextLine(list, topBounds.Min.X+8, y+8,
			f.shaper.ShapeSimple(store.GetTheme().String(), f.th.TextStyle(theme.TextLabelS)).Lines[0],
			f.th.Color(theme.ColorText))
		cardBounds := gfx.RectFromXYWH(topBounds.Min.X+8, y+24, cardWidth, cardHeight)
		f.renderMiniCard(list, cardBounds, entry, f.th)

		// Bottom panel - compare theme
		bottomY := y + halfHeight + 16
		bottomBounds := gfx.RectFromXYWH(bounds.Min.X+8, bottomY, bounds.Width()-16, halfHeight)
		list.Add(gfx.FillRect{
			Rect:  bottomBounds,
			Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
		})
		f.drawTextLine(list, bottomBounds.Min.X+8, bottomY+8,
			f.shaper.ShapeSimple(compareTheme.String(), f.th.TextStyle(theme.TextLabelS)).Lines[0],
			f.th.Color(theme.ColorText))
		compareBounds := gfx.RectFromXYWH(bottomBounds.Min.X+8, bottomY+24, cardWidth, cardHeight)
		f.renderMiniCard(list, compareBounds, entry, f.th)
	}
}

// renderMiniCard renders a simplified card view for compare mode.
func (f *ContentFacet) renderMiniCard(list *gfx.CommandList, bounds gfx.Rect, entry *model.CatalogEntry, th theme.Context) {
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
		f.drawTextLine(list, bounds.Min.X+8, bounds.Min.Y+12, line, th.Color(theme.ColorText))
	}

	// Family badge
	familyText := entry.Family.String()
	familyStyle := th.TextStyle(theme.TextLabelS)
	familyLayout := f.shaper.ShapeSimple(familyText, familyStyle)
	if familyLayout != nil && len(familyLayout.Lines) > 0 {
		line := familyLayout.Lines[0]
		f.drawTextLine(list, bounds.Min.X+8, bounds.Min.Y+32, line, th.Color(theme.ColorPrimary))
	}
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
