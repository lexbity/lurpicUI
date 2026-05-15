package ui

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/store"
)

type contentRuntimeStub struct {
	attachments map[facet.FacetID]layout.ChildAttachment
	added       []layout.ChildAttachment
	updated     []layout.ChildAttachment
	removed     int
}

func (s *contentRuntimeStub) AddFacet(_, child facet.FacetImpl, attachment layout.ChildAttachment) {
	if s.attachments == nil {
		s.attachments = make(map[facet.FacetID]layout.ChildAttachment)
	}
	s.attachments[child.Base().ID()] = attachment
	s.added = append(s.added, attachment)
}

func (s *contentRuntimeStub) RemoveFacet(child facet.FacetImpl) {
	if child == nil || child.Base() == nil {
		return
	}
	if s.attachments != nil {
		delete(s.attachments, child.Base().ID())
	}
	s.removed++
}

func (s *contentRuntimeStub) ResolveChildAttachment(id facet.FacetID) (layout.ChildAttachment, bool) {
	if s.attachments == nil {
		return layout.ChildAttachment{}, false
	}
	attachment, ok := s.attachments[id]
	return attachment, ok
}

func (s *contentRuntimeStub) UpdateChildAttachment(child facet.FacetImpl, attachment layout.ChildAttachment) {
	if child == nil || child.Base() == nil {
		return
	}
	if s.attachments == nil {
		s.attachments = make(map[facet.FacetID]layout.ChildAttachment)
	}
	s.attachments[child.Base().ID()] = attachment
	s.updated = append(s.updated, attachment)
}

func (s *contentRuntimeStub) RequestFrame() {}

func TestContentFacet_ReflowCachesViewport(t *testing.T) {
	th := theme.Default()
	shaper := newTestShaper(t)

	resetCatalogStores(t)

	content := NewContentFacet(th, shaper)
	bounds := gfx.RectFromXYWH(0, 0, 1280, 800)
	content.layout.Arrange(bounds)
	content.syncCards()

	state := content.LayoutState()
	if state.bounds != bounds {
		t.Fatalf("layout bounds = %+v, want %+v", state.bounds, bounds)
	}
	if state.inner.IsEmpty() {
		t.Fatal("expected inner viewport bounds to be cached")
	}
	if state.columns < 1 {
		t.Fatalf("columns = %d, want at least 1", state.columns)
	}
	if len(state.sections) == 0 {
		t.Fatal("expected family sections to be cached")
	}

	first := content.cards[0]
	cardBounds, ok := content.CardBounds(first.Entry().ID)
	if !ok {
		t.Fatalf("CardBounds(%q) not found", first.Entry().ID)
	}
	if !state.inner.Contains(cardBounds.Min) {
		t.Fatalf("card bounds %v not placed inside inner viewport %v", cardBounds, state.inner)
	}
}

func TestContentFacet_ReflowRespondsToResize(t *testing.T) {
	th := theme.Default()
	shaper := newTestShaper(t)

	resetCatalogStores(t)

	content := NewContentFacet(th, shaper)

	wide := gfx.RectFromXYWH(0, 0, 1280, 800)
	content.layout.Arrange(wide)
	content.syncCards()
	wideColumns := content.LayoutState().columns
	wideBounds, ok := content.CardBounds("basic.rect")
	if !ok {
		t.Fatal("wide card bounds not found")
	}

	narrow := gfx.RectFromXYWH(0, 0, 720, 800)
	content.layout.Arrange(narrow)
	content.syncCards()
	narrowColumns := content.LayoutState().columns
	narrowBounds, ok := content.CardBounds("basic.rect")
	if !ok {
		t.Fatal("narrow card bounds not found")
	}

	if wideColumns < narrowColumns {
		t.Fatalf("wide columns = %d, narrow columns = %d; expected wide to be >= narrow", wideColumns, narrowColumns)
	}
	if wideBounds == narrowBounds {
		t.Fatal("card bounds did not change after resize")
	}
}

func TestCalculateShellBounds_NarrowWindow(t *testing.T) {
	shell := CalculateShellBounds(gfx.RectFromXYWH(0, 0, 560, 400), 240, 280)
	if shell.Content.Width() < 0 {
		t.Fatalf("content width = %v, want non-negative", shell.Content.Width())
	}
	if shell.Sidebar.IsEmpty() {
		t.Fatal("sidebar bounds should still be present")
	}
	if shell.Inspector.IsEmpty() {
		t.Fatal("inspector bounds should still be present")
	}
}

func TestContentFacet_DensityAffectsCardSizing(t *testing.T) {
	th := theme.Default()
	shaper := newTestShaper(t)

	resetCatalogStores(t)

	content := NewContentFacet(th, shaper)
	bounds := gfx.RectFromXYWH(0, 0, 1280, 800)

	store.SetDensity(store.DensityCompact)
	content.SetLayoutProfile(LayoutProfileForDensity(store.DensityCompact))
	content.layout.Arrange(bounds)
	content.syncCards()
	compactCard, ok := content.CardBounds("basic.rect")
	if !ok {
		t.Fatal("compact card bounds not found")
	}

	store.SetDensity(store.DensityComfortable)
	content.SetLayoutProfile(LayoutProfileForDensity(store.DensityComfortable))
	content.layout.Arrange(bounds)
	content.syncCards()
	comfortableCard, ok := content.CardBounds("basic.rect")
	if !ok {
		t.Fatal("comfortable card bounds not found")
	}

	if compactCard.Width() >= comfortableCard.Width() {
		t.Fatalf("card width did not increase with density: compact=%v comfortable=%v", compactCard.Width(), comfortableCard.Width())
	}
	if compactCard.Height() >= comfortableCard.Height() {
		t.Fatalf("card height did not increase with density: compact=%v comfortable=%v", compactCard.Height(), comfortableCard.Height())
	}
}

func TestContentFacet_AttachesCardsIntoRuntimeTree(t *testing.T) {
	th := theme.Default()
	shaper := newTestShaper(t)

	resetCatalogStores(t)

	content := NewContentFacet(th, shaper)
	rt := &contentRuntimeStub{}
	content.runtime = rt
	content.syncCards()

	bounds := gfx.RectFromXYWH(0, 0, 1280, 800)
	content.layout.Arrange(bounds)

	if len(rt.added) == 0 {
		t.Fatal("expected cards to be attached to the runtime tree")
	}
	if len(rt.updated) == 0 {
		t.Fatal("expected card attachments to be updated during reflow")
	}
	first := rt.updated[0]
	if first.LayerID != contentCardsLayerID {
		t.Fatalf("layer id = %d, want %d", first.LayerID, contentCardsLayerID)
	}
	if first.Placement.FreeAnchor != layout.FreeTopLeft {
		t.Fatalf("free anchor = %v, want top-left", first.Placement.FreeAnchor)
	}
}

func TestContentFacet_SelectsCardOnPointerRelease(t *testing.T) {
	th := theme.Default()
	shaper := newTestShaper(t)

	resetCatalogStores(t)

	content := NewContentFacet(th, shaper)
	bounds := gfx.RectFromXYWH(0, 0, 1280, 800)
	content.layout.Arrange(bounds)
	content.syncCards()

	if len(content.cards) < 2 {
		t.Fatal("expected at least two catalog cards")
	}

	target := content.cards[1]
	center := gfx.Point{
		X: target.layout.ArrangedBounds.Min.X + target.layout.ArrangedBounds.Width()/2,
		Y: target.layout.ArrangedBounds.Min.Y + target.layout.ArrangedBounds.Height()/2,
	}
	if !content.input.OnPointer(facet.PointerEvent{
		Kind:     platform.PointerRelease,
		Position: center,
		Button:   platform.PointerLeft,
	}) {
		t.Fatal("expected pointer release to be handled")
	}
	if got := store.SelectionStore.Get(); got != target.Entry().ID {
		t.Fatalf("selection = %q, want %q", got, target.Entry().ID)
	}
}

func TestContentFacet_ArrowKeyMovesSelection(t *testing.T) {
	th := theme.Default()
	shaper := newTestShaper(t)

	resetCatalogStores(t)

	content := NewContentFacet(th, shaper)
	content.layout.Arrange(gfx.RectFromXYWH(0, 0, 1280, 800))
	content.syncCards()
	if len(content.cards) < 2 {
		t.Fatal("expected at least two catalog cards")
	}

	store.SelectEntry(content.cards[0].Entry().ID)
	if content.focus.OnFocusGained != nil {
		content.focus.OnFocusGained()
	}

	if !content.input.OnKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected right arrow to be handled")
	}
	if got := store.SelectionStore.Get(); got != content.cards[1].Entry().ID {
		t.Fatalf("selection = %q, want %q", got, content.cards[1].Entry().ID)
	}
}
