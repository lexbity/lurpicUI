package primitive

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/software"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"github.com/go-text/typesetting/font"
)

func TestPrimitiveTextGolden(t *testing.T) {
	mark := NewText("Hello ☺ world _ AaBbCcDdEeFfGgHhIiJjLlMmNnOoPpQqRrSsTtUuVvWwXxYyZz")
	mark.SetTypography(theme.TextBodyM)
	rt := textRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, primitiveGoldenTokens(), nil),
		fonts:     mustPrimitiveTextRegistry(t),
	}
	surface := renderPrimitiveText(t, mark, rt, "primitive_text_symbol", 320, 320, 120)
	testkit.AssertGolden(t, surface, "primitive_text_symbol")
}

func TestPrimitiveTextGoldenAlphabet(t *testing.T) {
	renderAndAssertPrimitiveTextGolden(t, "primitive_text_alphabet", "AaBbCcDdEeFfGgHhIiJjLlMmNnOoPpQqRrSsTtUuVvWwXxYyZz", 960, 1040, 160)
}

func TestPrimitiveTextGoldenChinese(t *testing.T) {
	renderAndAssertPrimitiveTextGolden(t, "primitive_text_chinese", "你好，世界", 320, 320, 120)
}

func TestPrimitiveTextGoldenJapanese(t *testing.T) {
	renderAndAssertPrimitiveTextGolden(t, "primitive_text_japanese", "こんにちは世界", 320, 320, 120)
}

func TestPrimitiveTextGoldenArabic(t *testing.T) {
	renderAndAssertPrimitiveTextGolden(t, "primitive_text_arabic", "مرحبا بالعالم", 320, 320, 120)
}

func TestPrimitiveTextGoldenDescenders(t *testing.T) {
	renderAndAssertPrimitiveTextGolden(t, "primitive_text_descenders", "Agjpqy", 320, 320, 120)
}

func TestPrimitiveTextGoldenAnatomy(t *testing.T) {
	renderAndAssertPrimitiveTextGoldenWithGuides(t, "primitive_text_anatomy", "ACGJQSUt", 320, 320, 120)
}

func TestPrimitiveIconGolden(t *testing.T) {
	mark := NewIcon(IconSVG(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2 3 7v10l9 5 9-5V7z"/><path d="M12 6v12" stroke="#fff" stroke-width="2" stroke-linecap="round"/><path d="M8 10h8" stroke="#fff" stroke-width="2" stroke-linecap="round"/></svg>`))
	mark.SetDecorative(false)
	mark.SetAccessibleName("Primitive icon")
	mark.SetColorSlot(theme.ColorPrimary)
	tokens := primitiveGoldenTokens()
	tokens.Color.Primary = gfx.ColorFromRGBA8(38, 82, 122, 255)
	rt := iconRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		icons:     map[string]runtimepkg.IconAsset{},
	}
	surface := renderPrimitiveIcon(t, mark, rt, "primitive_icon_inline")
	testkit.AssertGolden(t, surface, "primitive_icon_inline")
}

func renderAndAssertPrimitiveTextGolden(t *testing.T, goldenName, content string, maxWidth float32, surfaceWidth, surfaceHeight int) {
	t.Helper()
	mark := NewText(content)
	mark.SetTypography(theme.TextBodyM)
	rt := textRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, primitiveGoldenTokens(), nil),
		fonts:     mustPrimitiveTextRegistry(t),
	}
	surface := renderPrimitiveText(t, mark, rt, goldenName, maxWidth, surfaceWidth, surfaceHeight)
	testkit.AssertGolden(t, surface, goldenName)
}

func renderAndAssertPrimitiveTextGoldenWithGuides(t *testing.T, goldenName, content string, maxWidth float32, surfaceWidth, surfaceHeight int) {
	t.Helper()
	mark := NewText(content)
	mark.SetTypography(theme.TextBodyM)
	rt := textRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, primitiveGoldenTokens(), nil),
		fonts:     mustPrimitiveTextRegistry(t),
	}
	surface := renderPrimitiveTextWithGuides(t, mark, rt, goldenName, maxWidth, surfaceWidth, surfaceHeight)
	testkit.AssertGolden(t, surface, goldenName)
}

func renderPrimitiveText(t *testing.T, mark *Text, rt textRuntimeStub, goldenName string, maxWidth float32, surfaceWidth, surfaceHeight int) *testkit.MemorySurface {
	t.Helper()
	facet.Attach(mark, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	if maxWidth > 0 {
		mark.SetMaxWidth(maxWidth)
	}
	result := mark.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: float32(surfaceWidth), H: float32(surfaceHeight)}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable primitive text for %s, got %#v", goldenName, result.Size)
	}
	bounds := gfx.RectFromXYWH(16, 16, result.Size.W, result.Size.H)
	mark.layoutRole.Arrange(facet.ArrangeContext{}, bounds)
	cmds := mark.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatalf("expected projected commands for %s", goldenName)
	}
	surface := testkit.NewMemorySurface(surfaceWidth, surfaceHeight)
	submitCommands(t, surface, bounds, cmds.Commands)
	return surface
}

func renderPrimitiveTextWithGuides(t *testing.T, mark *Text, rt textRuntimeStub, goldenName string, maxWidth float32, surfaceWidth, surfaceHeight int) *testkit.MemorySurface {
	t.Helper()
	facet.Attach(mark, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	if maxWidth > 0 {
		mark.SetMaxWidth(maxWidth)
	}
	result := mark.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: float32(surfaceWidth), H: float32(surfaceHeight)}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable primitive text for %s, got %#v", goldenName, result.Size)
	}
	bounds := gfx.RectFromXYWH(16, 16, result.Size.W, result.Size.H)
	mark.layoutRole.Arrange(facet.ArrangeContext{}, bounds)
	cmds := mark.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatalf("expected projected commands for %s", goldenName)
	}
	if mark.cachedLayout != nil && len(mark.cachedLayout.Lines) > 0 {
		line := mark.cachedLayout.Lines[0]
		style := mark.cachedStyle
		face := rt.fonts.Resolve(style).GoFace()
		if face != nil {
			if capUnits := face.LineMetric(font.CapHeight); capUnits > 0 && style.Size > 0 {
				capHeight := capUnits * style.Size / float32(face.Upem())
				baselineY := bounds.Min.Y + line.Baseline
				capY := baselineY - capHeight
				cmds.Commands = append(cmds.Commands,
					gfx.FillRect{
						Rect:  gfx.RectFromXYWH(bounds.Min.X, float32(math.Round(float64(capY))), bounds.Width(), 1),
						Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(84, 123, 255, 255)),
					},
					gfx.FillRect{
						Rect:  gfx.RectFromXYWH(bounds.Min.X, float32(math.Round(float64(baselineY))), bounds.Width(), 1),
						Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(255, 94, 94, 255)),
					},
				)
			}
		}
	}
	surface := testkit.NewMemorySurface(surfaceWidth, surfaceHeight)
	submitCommands(t, surface, bounds, cmds.Commands)
	return surface
}

func renderPrimitiveIcon(t *testing.T, mark *Icon, rt iconRuntimeStub, goldenName string) *testkit.MemorySurface {
	t.Helper()
	facet.Attach(mark, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := mark.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 160, H: 160}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable primitive icon for %s, got %#v", goldenName, result.Size)
	}
	bounds := gfx.RectFromXYWH(20, 20, result.Size.W, result.Size.H)
	mark.layoutRole.Arrange(facet.ArrangeContext{}, bounds)
	cmds := mark.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatalf("expected projected commands for %s", goldenName)
	}
	surface := testkit.NewMemorySurface(160, 160)
	submitCommands(t, surface, bounds, cmds.Commands)
	return surface
}

func submitCommands(t *testing.T, surface *testkit.MemorySurface, bounds gfx.Rect, cmds []gfx.Command) {
	t.Helper()
	r := software.NewSoftwareRenderer()
	if err := r.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	if err := r.Submit(&render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      bounds,
				Opacity:     1,
				CommandHash: 1,
				Commands:    gfx.CommandList{Commands: cmds},
			},
		},
	}); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
}

func mustPrimitiveTextRegistry(t *testing.T) *text.FontRegistry {
	t.Helper()
	reg, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("new font registry: %v", err)
	}
	for _, rel := range []string{
		"github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf",
	} {
		data := mustReadFont(t, rel)
		if err := reg.LoadFontBytes(data, filepath.Base(rel)); err != nil {
			t.Fatalf("load font %q: %v", rel, err)
		}
	}
	for _, family := range []string{
		"Noto Color Emoji",
		"Noto Sans Symbols 2",
		"Noto Sans CJK SC",
		"Noto Sans CJK JP",
		"Noto Naskh Arabic",
	} {
		path := mustSystemFontPath(t, family)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read system font %q: %v", path, err)
		}
		if err := reg.LoadFontBytes(data, filepath.Base(path)); err != nil {
			t.Fatalf("load system font %q: %v", path, err)
		}
	}
	return reg
}

func mustSystemFontPath(t *testing.T, family string) string {
	t.Helper()
	out, err := exec.Command("fc-match", "-f", "%{file}\n", family).Output()
	if err != nil {
		t.Fatalf("fc-match %q: %v", family, err)
	}
	path := string(bytesTrimSpace(out))
	if path == "" {
		t.Fatalf("fc-match %q returned empty path", family)
	}
	return path
}

func primitiveGoldenTokens() theme.Tokens {
	tokens := theme.DefaultTokens()
	tokens.Color.Primary = gfx.ColorFromRGBA8(38, 82, 122, 255)
	return tokens
}

func bytesTrimSpace(in []byte) []byte {
	start := 0
	for start < len(in) {
		switch in[start] {
		case '\n', '\r', '\t', ' ':
			start++
		default:
			goto trimEnd
		}
	}
trimEnd:
	end := len(in)
	for end > start {
		switch in[end-1] {
		case '\n', '\r', '\t', ' ':
			end--
		default:
			return in[start:end]
		}
	}
	return in[start:end]
}
