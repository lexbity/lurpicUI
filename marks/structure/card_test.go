package structure

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

type cardRuntimeStub struct {
	fonts *text.FontRegistry
}

func (s cardRuntimeStub) Schedule(j job.AnyJob)                                              {}
func (s cardRuntimeStub) CancelJob(id job.JobID)                                             {}
func (s cardRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {}
func (s cardRuntimeStub) FontRegistry() *text.FontRegistry                                   { return s.fonts }

func TestCardMeasureProjectAnchorsAndAccessibility(t *testing.T) {
	card := newCardFixture()
	rt := cardRuntimeStub{fonts: mustCardFontRegistry(t)}
	ctx := cardResolvedContext(cardTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)

	facet.Attach(card, facet.AttachContext{Runtime: rt, Theme: ctx})
	result := card.layoutRole.Measure(facet.MeasureContext{
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
	card.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: card.layoutRole.Parent,
		ChildGroup:  card.layoutRole.Child,
	}, bounds)

	if got := card.AccessibilityRole(); got != "group" {
		t.Fatalf("accessibility role = %q, want group", got)
	}
	if got := card.AccessibleName(); got != "Default size card" {
		t.Fatalf("accessible name = %q, want Default size card", got)
	}
	if card.LayoutMode != CardLayoutGrid {
		t.Fatalf("layout mode = %v, want grid", card.LayoutMode)
	}
	if len(card.Children()) < 5 {
		t.Fatalf("expected composed child facets, got %d", len(card.Children()))
	}
	if card.cachedBounds.IsEmpty() || len(card.cachedChildBounds) == 0 {
		t.Fatalf("expected arranged geometry, got card=%#v children=%#v", card.cachedBounds, card.cachedChildBounds)
	}

	anchors := card.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := card.projectionRole.Project(facet.ProjectionContext{Runtime: rt, Bounds: bounds, ContentScale: 1})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands")
	}
	var sawGlyphRun, sawFillPath bool
	for _, cmd := range cmds.Commands {
		switch cmd.(type) {
		case gfx.DrawGlyphRun:
			sawGlyphRun = true
		case gfx.FillPath:
			sawFillPath = true
		}
	}
	if !sawGlyphRun {
		t.Fatal("expected text glyph commands")
	}
	if !sawFillPath {
		t.Fatal("expected surface/fill commands")
	}
}

func TestCardLayoutModes(t *testing.T) {
	rt := cardRuntimeStub{fonts: mustCardFontRegistry(t)}
	ctx := cardResolvedContext(cardTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	for _, mode := range []CardLayoutMode{CardLayoutGrid, CardLayoutVertical, CardLayoutHorizontal} {
		card := newCardFixture()
		card.SetLayoutMode(mode)
		facet.Attach(card, facet.AttachContext{Runtime: rt, Theme: ctx})
		result := card.layoutRole.Measure(facet.MeasureContext{
			Runtime:          rt,
			Theme:            ctx,
			ContentScale:     1,
			Density:          facet.DensityID(theme.DensityIDComfortable),
			WritingDirection: facet.WritingDirectionLTR,
		}, facet.Constraints{MaxSize: gfx.Size{W: 960, H: 720}})
		if result.Size.W <= 0 || result.Size.H <= 0 {
			t.Fatalf("mode %v produced invalid size %#v", mode, result.Size)
		}
	}
}

func TestCardGoldenDefault(t *testing.T) {
	AssertCardGolden(t, "default", cardTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Card) {})
}

func TestCardGoldenCompact(t *testing.T) {
	AssertCardGolden(t, "compact", cardTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(c *Card) {})
}

func TestCardGoldenComfortable(t *testing.T) {
	AssertCardGolden(t, "comfortable", cardTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Card) {})
}

func TestCardGoldenDisabled(t *testing.T) {
	AssertCardGolden(t, "disabled", cardTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Card) {
		c.SetDisabled(true)
	})
}

func TestCardGoldenHighContrast(t *testing.T) {
	AssertCardGolden(t, "high_contrast", highContrastCardTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Card) {})
}

func TestCardGoldenRTL(t *testing.T) {
	AssertCardGolden(t, "rtl", cardTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(c *Card) {})
}

func AssertCardGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Card)) {
	t.Helper()
	card := newCardFixture()
	if mutate != nil {
		mutate(card)
	}
	rt := cardRuntimeStub{fonts: mustCardFontRegistry(t)}
	ctx := cardResolvedContext(tokens, density, direction)
	facet.Attach(card, facet.AttachContext{Runtime: rt, Theme: ctx})
	canvas := gfx.RectFromXYWH(12, 12, 616, 336)
	_ = card.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	bounds := canvas
	card.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: ctx, ParentGroup: card.layoutRole.Parent, ChildGroup: card.layoutRole.Child}, bounds)
	cmds := card.projectionRole.Project(facet.ProjectionContext{Runtime: rt, Bounds: bounds, ContentScale: 1})
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
	testkit.AssertGolden(t, surface, "card_"+name)
}

func newCardFixture() *Card {
	card := NewCard("Default size card")
	card.SetGrid(3, 3)
	card.SetChildren([]CardChild{
		{
			Key:    "icon",
			Facet:  primitive.NewIcon(cardTestHeaderIcon()),
			Grid:   facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1},
			MarkID: cardMarkIDFirstChild,
		},
		{
			Key:    "title",
			Facet:  primitive.NewText("Default size card"),
			Grid:   facet.GridPlacement{ColStart: 1, RowStart: 0, ColSpan: 2, RowSpan: 1},
			MarkID: cardMarkIDFirstChild + 1,
		},
		{
			Key:    "action",
			Facet:  action.NewButton("Action", uiinput.ButtonOutlined),
			Grid:   facet.GridPlacement{ColStart: 0, RowStart: 1, ColSpan: 1, RowSpan: 1},
			MarkID: cardMarkIDFirstChild + 2,
		},
		{
			Key:    "field",
			Facet:  input.NewTextField("Notes", uiinput.TextInputOutlined),
			Grid:   facet.GridPlacement{ColStart: 1, RowStart: 1, ColSpan: 2, RowSpan: 1},
			MarkID: cardMarkIDFirstChild + 3,
		},
		{
			Key:    "body",
			Facet:  primitive.NewText("Card content\nCard content\nCard content"),
			Grid:   facet.GridPlacement{ColStart: 0, RowStart: 2, ColSpan: 3, RowSpan: 1},
			MarkID: cardMarkIDFirstChild + 4,
		},
	})
	return card
}

func cardTestHeaderIcon() primitive.IconSource {
	return primitive.IconSVG(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><circle cx="12" cy="12" r="10"/></svg>`)
}

func cardTokens() theme.Tokens {
	tokens := theme.DefaultTokens()
	tokens.Color.Background = gfx.ColorFromRGBA8(247, 247, 249, 255)
	tokens.Color.Surface = gfx.ColorFromRGBA8(255, 255, 255, 255)
	tokens.Color.SurfaceVariant = gfx.ColorFromRGBA8(243, 244, 247, 255)
	tokens.Color.OnBackground = gfx.ColorFromRGBA8(27, 28, 33, 255)
	tokens.Color.OnSurface = gfx.ColorFromRGBA8(27, 28, 33, 255)
	tokens.Color.OnSurfaceVariant = gfx.ColorFromRGBA8(99, 102, 119, 255)
	tokens.Color.Primary = gfx.ColorFromRGBA8(79, 101, 216, 255)
	tokens.Color.OnPrimary = gfx.ColorFromRGBA8(255, 255, 255, 255)
	tokens.Color.Secondary = gfx.ColorFromRGBA8(103, 86, 188, 255)
	tokens.Color.OnSecondary = gfx.ColorFromRGBA8(255, 255, 255, 255)
	return tokens
}

func highContrastCardTokens() theme.Tokens {
	tokens := cardTokens()
	tokens.Color.SurfaceVariant = gfx.ColorFromRGBA8(232, 232, 235, 255)
	tokens.Color.OnSurfaceVariant = gfx.ColorFromRGBA8(0, 0, 0, 255)
	tokens.Color.Primary = gfx.ColorFromRGBA8(0, 87, 184, 255)
	tokens.Color.Secondary = gfx.ColorFromRGBA8(0, 87, 184, 255)
	return tokens
}

func mustCardFontRegistry(t *testing.T) *text.FontRegistry {
	t.Helper()
	reg, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("new font registry: %v", err)
	}
	data := mustReadCardFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	if err := reg.LoadFontBytes(data, "noto-sans-regular"); err != nil {
		t.Fatalf("load font: %v", err)
	}
	return reg
}

func mustReadCardFont(t *testing.T, rel string) []byte {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatalf("go env GOMODCACHE: %v", err)
	}
	path := filepath.Join(string(bytesTrim(out)), rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read font %q: %v", path, err)
	}
	return data
}

func bytesTrim(in []byte) []byte {
	for len(in) > 0 {
		switch in[len(in)-1] {
		case '\n', '\r', '\t', ' ':
			in = in[:len(in)-1]
		default:
			return in
		}
	}
	return in
}

func cardResolvedContext(tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) theme.ResolvedContext {
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
