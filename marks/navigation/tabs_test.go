package navigation

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

type tabsRuntimeStub struct {
	rootStyle any
	fonts     *text.FontRegistry
}

func (s tabsRuntimeStub) Schedule(j job.AnyJob)  {}
func (s tabsRuntimeStub) CancelJob(id job.JobID) {}
func (s tabsRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s tabsRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s tabsRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s tabsRuntimeStub) FontRegistry() *text.FontRegistry { return s.fonts }

func TestTabsMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	tabs, rt, measureCtx := newTabsTestFixture(t, defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(tabs, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := tabs.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1400, H: 800}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(12, 18, result.Size.W, result.Size.H)
	tabs.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: tabs.layoutRole.Parent,
		ChildGroup:  tabs.layoutRole.Child,
	}, bounds)

	if got := tabs.AccessibilityRole(); got != "tablist" {
		t.Fatalf("accessibility role = %q, want tablist", got)
	}
	if got := tabs.AccessibleName(); got != "Primary navigation" {
		t.Fatalf("accessible name = %q, want Primary navigation", got)
	}
	if len(tabs.cachedTabBounds) != len(tabs.Items) {
		t.Fatalf("cached tab bounds = %d, want %d", len(tabs.cachedTabBounds), len(tabs.Items))
	}
	if tabs.cachedPanelBounds.IsEmpty() || tabs.cachedTabListBounds.IsEmpty() {
		t.Fatalf("expected panel/tab-list geometry, got tablist=%#v panel=%#v", tabs.cachedTabListBounds, tabs.cachedPanelBounds)
	}

	firstHit := tabs.hitRole.HitTest(gfx.Point{
		X: tabs.cachedTabBounds[0].Min.X + tabs.cachedTabBounds[0].Width()*0.5,
		Y: tabs.cachedTabBounds[0].Min.Y + tabs.cachedTabBounds[0].Height()*0.5,
	})
	if !firstHit.Hit || (firstHit.MarkID != tabsMarkIDTab && firstHit.MarkID != tabsMarkIDTabLabel) {
		t.Fatalf("expected tab or label hit, got %#v", firstHit)
	}
	panelHit := tabs.hitRole.HitTest(gfx.Point{
		X: tabs.cachedPanelBounds.Min.X + tabs.cachedPanelBounds.Width()*0.5,
		Y: tabs.cachedPanelBounds.Min.Y + tabs.cachedPanelBounds.Height()*0.5,
	})
	if !panelHit.Hit || panelHit.MarkID != tabsMarkIDPanelAnchor {
		t.Fatalf("expected panel anchor hit, got %#v", panelHit)
	}

	anchors := tabs.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := tabs.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
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
		t.Fatal("expected glyph commands")
	}
	if !sawFillPath {
		t.Fatal("expected fill path commands")
	}
}

func TestTabsPointerAndKeyboardInteraction(t *testing.T) {
	tabs, rt, measureCtx := newTabsTestFixture(t, defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(tabs, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := tabs.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1400, H: 800}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	tabs.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	activated := -1
	tabs.Activated.Subscribe(func(index int) {
		activated = index
	})

	secondCenter := gfx.Point{
		X: tabs.cachedTabBounds[1].Min.X + tabs.cachedTabBounds[1].Width()*0.5,
		Y: tabs.cachedTabBounds[1].Min.Y + tabs.cachedTabBounds[1].Height()*0.5,
	}
	if !tabs.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: secondCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !tabs.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: secondCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	if got := tabs.ActiveIndex; got != 1 {
		t.Fatalf("active index after pointer = %d, want 1", got)
	}
	if activated != 1 {
		t.Fatalf("expected activated signal for index 1, got %d", activated)
	}

	if !tabs.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected right key to be handled")
	}
	if got := tabs.ActiveIndex; got != 2 {
		t.Fatalf("active index after right = %d, want 2", got)
	}
	if !tabs.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyHome}) {
		t.Fatal("expected home key to be handled")
	}
	if got := tabs.ActiveIndex; got != 0 {
		t.Fatalf("active index after home = %d, want 0", got)
	}
	if !tabs.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnd}) {
		t.Fatal("expected end key to be handled")
	}
	if got := tabs.ActiveIndex; got != len(tabs.Items)-1 {
		t.Fatalf("active index after end = %d, want %d", got, len(tabs.Items)-1)
	}
	if !tabs.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected space key press to be handled")
	}
	if !tabs.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeySpace}) {
		t.Fatal("expected space key release to be handled")
	}

	tabs.onFocusLost()
	tabs.focusFromPointer = false
	tabs.onFocusGained()
	if !tabs.focusedVisible {
		t.Fatal("expected keyboard focus to show focus ring")
	}
	tabs.SetDisabled(true)
	if tabs.focusRole.Focusable() {
		t.Fatal("expected disabled tabs to be unfocusable")
	}
	if tabs.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: secondCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected disabled tabs to ignore pointer input")
	}
	if tabs.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected disabled tabs to ignore keyboard input")
	}
}

func TestTabsGoldenDefault(t *testing.T) {
	AssertTabsGolden(t, "default", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {})
}

func TestTabsGoldenCompact(t *testing.T) {
	AssertTabsGolden(t, "compact", defaultTabsTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(t *Tabs) {})
}

func TestTabsGoldenComfortable(t *testing.T) {
	AssertTabsGolden(t, "comfortable", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {})
}

func TestTabsGoldenDisabled(t *testing.T) {
	AssertTabsGolden(t, "disabled", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {
		t.SetDisabled(true)
	})
}

func TestTabsGoldenHighContrast(t *testing.T) {
	AssertTabsGolden(t, "high_contrast", highContrastTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {})
}

func TestTabsGoldenHovered(t *testing.T) {
	AssertTabsGolden(t, "hovered", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {
		t.hoveredIndex = 1
	})
}

func TestTabsGoldenPressed(t *testing.T) {
	AssertTabsGolden(t, "pressed", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {
		t.pressedIndex = 0
	})
}

func TestTabsGoldenFocused(t *testing.T) {
	AssertTabsGolden(t, "focused", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {
		t.focusedVisible = true
	})
}

func TestTabsGoldenRTL(t *testing.T) {
	AssertTabsGolden(t, "rtl", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(t *Tabs) {})
}

func TestTabsGoldenSelected(t *testing.T) {
	AssertTabsGolden(t, "selected", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {
		t.SetActiveIndex(1)
	})
}

func AssertTabsGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Tabs)) {
	t.Helper()
	tabs, rt, measureCtx := newTabsTestFixture(t, tokens, density, direction)
	if mutate != nil {
		mutate(tabs)
	}
	renderTabsToSurface(t, tabs, rt, measureCtx, density, direction, name)
}

func renderTabsToSurface(t *testing.T, tabs *Tabs, rt tabsRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(tabs, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := tabs.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 1800, H: 1200}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	tabs.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)
	cmds := tabs.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands for golden")
	}
	surface := testkit.NewMemorySurface(int(result.Size.W+1), int(result.Size.H+1))
	r := softwarerenderer.NewSoftwareRenderer()
	if err := r.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      bounds,
				Opacity:     1,
				CommandHash: 1,
				Commands:    gfx.CommandList{Commands: cmds.Commands},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "tabs_"+goldenName)
}

func newTabsTestFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*Tabs, tabsRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	fonts := mustTabsFontRegistry(t)
	rtTokens := tokens
	rtTokens.Density.Mode = densityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	tabs := NewTabs("Primary navigation", []TabItem{
		{Key: "dashboard", Label: "Dashboard", PanelText: "Tab Panel 1"},
		{Key: "monitoring", Label: "Monitoring", PanelText: "Monitoring panel"},
		{Key: "activity", Label: "Activity", PanelText: "Activity panel"},
		{Key: "settings", Label: "Settings", PanelText: "Settings panel"},
	})
	rt := tabsRuntimeStub{rootStyle: rootStyle, fonts: fonts}
	return tabs, rt, resolved
}

func mustTabsFontRegistry(t *testing.T) *text.FontRegistry {
	t.Helper()
	reg, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("new font registry: %v", err)
	}
	data := mustReadTabsFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	if err := reg.LoadFontBytes(data, "noto-sans-regular"); err != nil {
		t.Fatalf("load font: %v", err)
	}
	return reg
}

func mustReadTabsFont(t *testing.T, rel string) []byte {
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

func defaultTabsTokens() theme.Tokens {
	return theme.DefaultTokens()
}

func highContrastTabsTokens() theme.Tokens {
	tokens := theme.DefaultTokens()
	tokens.Color.Surface = gfx.ColorFromRGBA8(255, 255, 255, 255)
	tokens.Color.OnSurface = gfx.ColorFromRGBA8(0, 0, 0, 255)
	tokens.Color.OnSurfaceVariant = gfx.ColorFromRGBA8(40, 40, 40, 255)
	tokens.Color.Primary = gfx.ColorFromRGBA8(0, 94, 184, 255)
	tokens.Color.OnPrimary = gfx.ColorFromRGBA8(255, 255, 255, 255)
	return tokens
}

func densityToTemplateMode(density theme.DensityID) theme.DensityMode {
	switch density {
	case theme.DensityIDCompact:
		return theme.DensityCompact
	case theme.DensityIDTouch:
		return theme.DensityTouch
	default:
		return theme.DensityComfortable
	}
}
