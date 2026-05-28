package status

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
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/templates"
)

type badgeRuntimeStub struct {
	rootStyle any
	fonts     *text.FontRegistry
	icons     map[string]runtimepkg.IconAsset
}

func (s badgeRuntimeStub) Schedule(j job.AnyJob)  {}
func (s badgeRuntimeStub) CancelJob(id job.JobID) {}
func (s badgeRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s badgeRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s badgeRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s badgeRuntimeStub) FontRegistry() *text.FontRegistry { return s.fonts }
func (s badgeRuntimeStub) ResolveIcon(ref string) (runtimepkg.IconAsset, bool) {
	if s.icons == nil {
		return runtimepkg.IconAsset{}, false
	}
	asset, ok := s.icons[ref]
	if !ok {
		return runtimepkg.IconAsset{}, false
	}
	return asset.Clone(), true
}

func TestBadgeMeasureProjectAnchorsAndAccessibility(t *testing.T) {
	badge := newBadgeFixture()
	tokens := badgeTokens()
	rt := badgeRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     mustBadgeFontRegistry(t),
		icons: map[string]runtimepkg.IconAsset{
			"status-dot": runtimepkg.NewIconAsset("status-dot", 1, gfx.CirclePath(gfx.Point{X: 12, Y: 12}, 6), gfx.RectFromXYWH(0, 0, 24, 24)),
		},
	}
	ctx := badgeResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)

	facet.Attach(badge, facet.AttachContext{Runtime: rt, Theme: ctx})
	result := badge.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 240, H: 160}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}
	bounds := gfx.RectFromXYWH(16, 16, result.Size.W, result.Size.H)
	badge.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: badge.layoutRole.Parent,
		ChildGroup:  badge.layoutRole.Child,
		Placement:   facet.Placement{Mode: facet.PlacementLinear},
	}, bounds)
	if got := badge.AccessibilityRole(); got != "status" {
		t.Fatalf("accessibility role = %q, want status", got)
	}
	if got := badge.AccessibleName(); got != "New" {
		t.Fatalf("accessible name = %q, want New", got)
	}
	if len(badge.Children()) != 2 {
		t.Fatalf("expected icon and label children, got %d", len(badge.Children()))
	}
	if badge.cachedContainerBounds.IsEmpty() || badge.cachedLabelBounds.IsEmpty() || badge.cachedIconBounds.IsEmpty() {
		t.Fatalf("expected arranged geometry, got container=%#v label=%#v icon=%#v", badge.cachedContainerBounds, badge.cachedLabelBounds, badge.cachedIconBounds)
	}
	anchors := badge.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline", "label", "optional_icon"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
	cmds := badge.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands")
	}
	var capture testkit.CommandCapture
	capture.Capture(cmds)
	capture.AssertHasGlyphRun(t)
	capture.AssertHasFillPath(t)
}

func TestBadgeGoldenDefault(t *testing.T) {
	AssertBadgeGolden(t, "default", badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(b *Badge) {})
}

func TestBadgeGoldenCompact(t *testing.T) {
	AssertBadgeGolden(t, "compact", badgeTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(b *Badge) {})
}

func TestBadgeGoldenComfortable(t *testing.T) {
	AssertBadgeGolden(t, "comfortable", badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(b *Badge) {})
}

func TestBadgeGoldenDisabled(t *testing.T) {
	AssertBadgeGolden(t, "disabled", badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(b *Badge) {
		b.SetDisabled(true)
	})
}

func TestBadgeGoldenHighContrast(t *testing.T) {
	AssertBadgeGolden(t, "high_contrast", highContrastBadgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(b *Badge) {})
}

func TestBadgeGoldenRTL(t *testing.T) {
	AssertBadgeGolden(t, "rtl", badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(b *Badge) {})
}

func AssertBadgeGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Badge)) {
	t.Helper()
	badge := newBadgeFixture()
	if mutate != nil {
		mutate(badge)
	}
	rt := badgeRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     mustBadgeFontRegistry(t),
		icons: map[string]runtimepkg.IconAsset{
			"status-dot": runtimepkg.NewIconAsset("status-dot", 1, gfx.CirclePath(gfx.Point{X: 12, Y: 12}, 6), gfx.RectFromXYWH(0, 0, 24, 24)),
		},
	}
	ctx := badgeResolvedContext(tokens, density, direction)
	facet.Attach(badge, facet.AttachContext{Runtime: rt, Theme: ctx})
	canvas := gfx.RectFromXYWH(16, 16, 240, 160)
	_ = badge.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	badge.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: ctx, ParentGroup: badge.layoutRole.Parent, ChildGroup: badge.layoutRole.Child, Placement: facet.Placement{Mode: facet.PlacementLinear}}, canvas)
	cmds := badge.projectionRole.Project(facet.ProjectionContext{Runtime: rt, Bounds: canvas, ContentScale: 1})
	if cmds == nil {
		t.Fatal("expected projected commands")
	}
	surface := testkit.NewMemorySurface(272, 192)
	renderer := softwarerenderer.NewSoftwareRenderer()
	if err := renderer.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:          1,
			Bounds:      canvas,
			Opacity:     1,
			Commands:    *cmds,
			CommandHash: 1,
		}},
	}
	if err := renderer.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "badge_"+name)
}

func newBadgeFixture() *Badge {
	badge := NewBadge("New")
	badge.SetIconRef("status-dot")
	return badge
}

func badgeTokens() theme.Tokens {
	return toThemeTokens(templates.Notes().Tokens)
}

func highContrastBadgeTokens() theme.Tokens {
	return toThemeTokens(templates.UneNuit().Tokens)
}

func badgeResolvedContext(tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) theme.ResolvedContext {
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

func mustBadgeFontRegistry(t *testing.T) *text.FontRegistry {
	t.Helper()
	reg, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("new font registry: %v", err)
	}
	data := mustReadBadgeFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	if err := reg.LoadFontBytes(data, "noto-sans-regular"); err != nil {
		t.Fatalf("load font: %v", err)
	}
	return reg
}

func mustReadBadgeFont(t *testing.T, rel string) []byte {
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

func toThemeTokens(t templates.Tokens) theme.Tokens {
	tokens := theme.DefaultTokens()
	tokens.Color.Background = t.Color.Background
	tokens.Color.Surface = t.Color.Surface
	tokens.Color.SurfaceVariant = t.Color.SurfaceVariant
	tokens.Color.SurfaceInverse = t.Color.SurfaceInverse
	tokens.Color.OnBackground = t.Color.OnBackground
	tokens.Color.OnSurface = t.Color.OnSurface
	tokens.Color.OnSurfaceVariant = t.Color.OnSurfaceVariant
	tokens.Color.Primary = t.Color.Primary
	tokens.Color.OnPrimary = t.Color.OnPrimary
	tokens.Color.Secondary = t.Color.Secondary
	tokens.Color.OnSecondary = t.Color.OnSecondary
	tokens.Color.Error = t.Color.Error
	tokens.Color.Warning = t.Color.Warning
	tokens.Color.Success = t.Color.Success
	tokens.Color.OnError = t.Color.OnError
	tokens.Color.DisabledOpacity = t.Color.DisabledOpacity
	tokens.Color.HoverLighten = t.Color.HoverOpacity
	tokens.Color.PressedDarken = t.Color.PressedOpacity
	tokens.Color.SelectedOverlay = t.Color.SelectionOpacity

	tokens.Typography.DisplayLarge = t.Typography.DisplayLarge
	tokens.Typography.DisplayMedium = t.Typography.DisplayMedium
	tokens.Typography.DisplaySmall = t.Typography.DisplaySmall
	tokens.Typography.HeadlineLarge = t.Typography.HeadlineLarge
	tokens.Typography.HeadlineMedium = t.Typography.HeadlineMedium
	tokens.Typography.HeadlineSmall = t.Typography.HeadlineSmall
	tokens.Typography.TitleLarge = t.Typography.TitleLarge
	tokens.Typography.TitleMedium = t.Typography.TitleMedium
	tokens.Typography.TitleSmall = t.Typography.TitleSmall
	tokens.Typography.LabelLarge = t.Typography.LabelLarge
	tokens.Typography.LabelMedium = t.Typography.LabelMedium
	tokens.Typography.LabelSmall = t.Typography.LabelSmall
	tokens.Typography.BodyLarge = t.Typography.BodyLarge
	tokens.Typography.BodyMedium = t.Typography.BodyMedium
	tokens.Typography.BodySmall = t.Typography.BodySmall

	tokens.Radius.None = t.Shape.RadiusNone
	tokens.Radius.XS = t.Shape.RadiusXS
	tokens.Radius.SM = t.Shape.RadiusSM
	tokens.Radius.MD = t.Shape.RadiusMD
	tokens.Radius.LG = t.Shape.RadiusLG
	tokens.Radius.Full = t.Shape.RadiusFull

	return tokens
}
