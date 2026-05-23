package structure

import (
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	gfxsvg "codeburg.org/lexbit/lurpicui/gfx/svg"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/templates"
)

func TestListGeometryContracts(t *testing.T) {
	list := NewList("Geometry list", []ListEntry{
		{Key: "one", Label: "One"},
		{Key: "two", Label: "Two", SupportingText: strings.Join([]string{"Line one", "Line two"}, "\n")},
	})
	list.SetSectionHeader("Heading")
	rt := listRuntimeStub{
		cardRuntimeStub: cardRuntimeStub{fonts: mustCardFontRegistry(t)},
		icons:           map[string]runtimepkg.IconAsset{},
	}
	ctx := listResolvedContext(listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)

	facet.Attach(list, facet.AttachContext{Runtime: rt, Theme: ctx})
	result := list.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 360, H: 220}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}
	bounds := gfx.RectFromXYWH(12, 12, 320, 180)
	list.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: list.layoutRole.Parent,
		ChildGroup:  list.layoutRole.Child,
	}, bounds)

	headerBounds := list.cachedHeaderBounds
	rowOneBounds := list.cachedRowBounds["one"]
	rowTwoBounds := list.cachedRowBounds["two"]
	if headerBounds.IsEmpty() || rowOneBounds.IsEmpty() || rowTwoBounds.IsEmpty() {
		t.Fatalf("expected arranged row bounds, got header=%#v row1=%#v row2=%#v", headerBounds, rowOneBounds, rowTwoBounds)
	}
	if rowOneBounds.Min.Y <= headerBounds.Max.Y {
		t.Fatalf("expected row spacing below header, got header=%#v row1=%#v", headerBounds, rowOneBounds)
	}
	if rowTwoBounds.Min.Y <= rowOneBounds.Max.Y {
		t.Fatalf("expected row spacing between items, got row1=%#v row2=%#v", rowOneBounds, rowTwoBounds)
	}
	if rowTwoBounds.Height() <= rowOneBounds.Height() {
		t.Fatalf("expected multiline supporting text to expand row height, got row1=%#v row2=%#v", rowOneBounds, rowTwoBounds)
	}
}

type listRuntimeStub struct {
	cardRuntimeStub
	icons map[string]runtimepkg.IconAsset
}

func (s listRuntimeStub) IconResolver() runtimepkg.IconResolver {
	return listIconResolverStub{icons: s.icons}
}

type listIconResolverStub struct {
	icons map[string]runtimepkg.IconAsset
}

func (r listIconResolverStub) ResolveIcon(ref string) (runtimepkg.IconAsset, bool) {
	if r.icons == nil {
		return runtimepkg.IconAsset{}, false
	}
	asset, ok := r.icons[ref]
	if !ok {
		return runtimepkg.IconAsset{}, false
	}
	return asset.Clone(), true
}

func TestListMeasureProjectAnchorsAndAccessibility(t *testing.T) {
	list := newListFixture()
	rt := listRuntimeStub{
		cardRuntimeStub: cardRuntimeStub{fonts: mustCardFontRegistry(t)},
		icons:           map[string]runtimepkg.IconAsset{},
	}
	ctx := listResolvedContext(listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)

	facet.Attach(list, facet.AttachContext{Runtime: rt, Theme: ctx})
	result := list.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 960, H: 720}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	list.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: list.layoutRole.Parent,
		ChildGroup:  list.layoutRole.Child,
	}, bounds)

	if got := list.AccessibilityRole(); got != "list" {
		t.Fatalf("accessibility role = %q, want list", got)
	}
	if got := list.AccessibleName(); got != "Sample list" {
		t.Fatalf("accessible name = %q, want Sample list", got)
	}
	if len(list.Children()) != 1 {
		t.Fatalf("expected scroll-region child, got %d", len(list.Children()))
	}
	if list.cachedBounds.IsEmpty() || len(list.cachedRowBounds) == 0 {
		t.Fatalf("expected arranged geometry, got bounds=%#v rows=%#v", list.cachedBounds, list.cachedRowBounds)
	}

	anchors := list.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	expectBoundsAnchors(t, anchors, bounds)
	if got, ok := anchors["section_header"]; !ok {
		t.Fatal("missing section_header anchor")
	} else if want := rectCenter(list.cachedHeaderBounds); got != want {
		t.Fatalf("section_header anchor = %#v, want %#v", got, want)
	}

	cmds := list.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands")
	}
}

func TestListStoreChangeInvalidatesStructure(t *testing.T) {
	list := newListFixture()
	oldVersion := list.Data.Version()
	list.SetEntries([]ListEntry{
		{Key: "a", Label: "test-item-1"},
		{Key: "b", Label: "test-item-2"},
		{Key: "c", Label: "test-item-3"},
		{Key: "d", Label: "test-item-4"},
	})
	if got := list.Data.Version(); got == oldVersion {
		t.Fatal("expected store version to change")
	}
	if len(list.Children()) != 1 {
		t.Fatalf("expected scroll-region child, got %d", len(list.Children()))
	}
}

func TestListGoldenDefault(t *testing.T) {
	AssertListGolden(t, "default", listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(l *List) {})
}

func TestListGoldenCompact(t *testing.T) {
	AssertListGolden(t, "compact", listTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(l *List) {})
}

func TestListGoldenComfortable(t *testing.T) {
	AssertListGolden(t, "comfortable", listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(l *List) {})
}

func TestListGoldenDisabled(t *testing.T) {
	AssertListGolden(t, "disabled", listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(l *List) {
		l.SetDisabled(true)
	})
}

func TestListGoldenHighContrast(t *testing.T) {
	AssertListGolden(t, "high_contrast", highContrastListTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(l *List) {})
}

func TestListGoldenRTL(t *testing.T) {
	AssertListGolden(t, "rtl", listTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(l *List) {})
}

func TestListGoldenEmpty(t *testing.T) {
	AssertListGolden(t, "empty", listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(l *List) {
		l.SetEntries(nil)
	})
}

func AssertListGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*List)) {
	t.Helper()
	list := newListFixture()
	if mutate != nil {
		mutate(list)
	}
	rt := listRuntimeStub{
		cardRuntimeStub: cardRuntimeStub{fonts: mustCardFontRegistry(t)},
		icons:           map[string]runtimepkg.IconAsset{},
	}
	ctx := listResolvedContext(tokens, density, direction)
	facet.Attach(list, facet.AttachContext{Runtime: rt, Theme: ctx})
	canvas := gfx.RectFromXYWH(12, 12, 616, 336)
	_ = list.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	bounds := canvas
	list.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: ctx, ParentGroup: list.layoutRole.Parent, ChildGroup: list.layoutRole.Child}, bounds)
	cmds := list.projectionRole.Project(facet.ProjectionContext{Runtime: rt, Bounds: bounds, ContentScale: 1})
	if cmds == nil {
		t.Fatal("expected projected commands")
	}
	surface := testkit.NewMemorySurface(640, 360)
	renderer := softwarerenderer.NewSoftwareRenderer()
	if err := renderer.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:          1,
			Bounds:      bounds,
			Opacity:     1,
			Commands:    *cmds,
			CommandHash: 1,
		}},
	}
	if err := renderer.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "list_"+name)
}

func newListFixture() *List {
	list := NewList("Sample list", []ListEntry{
		{Key: "one", Label: "List item text"},
		{Key: "two", Label: "List item text"},
		{Key: "three", Label: "List item text"},
	})
	list.SetSectionHeader("Heading")
	list.SetEmptyState("No items")
	return list
}

func mustListIconAsset(t *testing.T, ref string, src string) runtimepkg.IconAsset {
	t.Helper()
	doc, err := gfxsvg.ParseSVGString(src)
	if err != nil {
		t.Fatalf("parse svg %q: %v", ref, err)
	}
	return runtimepkg.NewIconAsset(ref, 1, gfxsvgPathFromDocument(doc), doc.ViewBox)
}

func listTokens() theme.Tokens {
	return toThemeTokens(templates.Notes().Tokens)
}

func highContrastListTokens() theme.Tokens {
	return toThemeTokens(templates.UneNuit().Tokens)
}

func listResolvedContext(tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) theme.ResolvedContext {
	ctx := theme.DefaultResolvedContext()
	rv := reflect.ValueOf(&ctx).Elem()
	field := rv.FieldByName("defaultContext")
	fieldCopy := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	tokensField := fieldCopy.FieldByName("tokens")
	reflect.NewAt(tokensField.Type(), unsafe.Pointer(tokensField.UnsafeAddr())).Elem().Set(reflect.ValueOf(tokens))
	ctx = ctx.WithDensity(theme.DefaultDensityScale(density, tokens))
	ctx = ctx.WithWritingDirection(direction)
	return ctx
}

func gfxsvgPathFromDocument(doc gfxsvg.SVGDocument) gfx.Path {
	if len(doc.Elements) == 0 {
		return gfx.Path{}
	}
	return doc.Elements[0].Path
}
