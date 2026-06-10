package action

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	gfxsvg "codeburg.org/lexbit/lurpicui/gfx/svg"
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
	"codeburg.org/lexbit/lurpicui/theme/templates"
)

// Flowbite icons used for the skeuomorphic button golden.
// floppy-disk (leading = "Save") and check (trailing = "Confirm").
const skeuoButtonFloppyDiskSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" fill="none" viewBox="0 0 24 24">
  <path stroke="currentColor" stroke-linejoin="round" stroke-width="2" d="M4 5a1 1 0 0 1 1-1h11.586a1 1 0 0 1 .707.293l2.414 2.414a1 1 0 0 1 .293.707V19a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5Z"/>
  <path stroke="currentColor" stroke-linejoin="round" stroke-width="2" d="M8 4h8v4H8V4Zm7 10a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"/>
</svg>`

const skeuoButtonCheckSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" fill="none" viewBox="0 0 24 24">
  <path stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 11.917 9.724 16.5 19 7.5"/>
</svg>`

func mustSkeuoButtonIconAsset(ref, svgSrc string) runtimepkg.IconAsset {
	doc, err := gfxsvg.ParseSVGString(svgSrc)
	if err != nil {
		panic("button_skeuomorphic_golden_test: parse svg " + ref + ": " + err.Error())
	}
	var path gfx.Path
	for _, el := range doc.Elements {
		path.Segments = append(path.Segments, el.Path.Segments...)
	}
	return runtimepkg.NewIconAsset(ref, 1, path, doc.ViewBox)
}

func TestButtonGoldenSkeuomorphic(t *testing.T) {
	AssertButtonSkeuomorphicGolden(t, "skeuomorphic", func(btn *Button) {
		btn.Variant = marks.Const(uiinput.ButtonSkeuomorphic)
	})
}

func TestButtonGoldenSkeuomorphicPressed(t *testing.T) {
	AssertButtonSkeuomorphicGolden(t, "skeuomorphic_pressed", func(btn *Button) {
		btn.Variant = marks.Const(uiinput.ButtonSkeuomorphic)
		btn.onPointer(facet.PointerEvent{
			Kind:     platform.PointerPress,
			Position: gfx.Point{X: 1, Y: 1},
			Button:   platform.PointerLeft,
		})
	})
}

func AssertButtonSkeuomorphicGolden(t *testing.T, name string, mutate func(*Button)) {
	t.Helper()

	// Build button with real flowbite icon paths rather than placeholder rects.
	btn := NewButton(marks.Const("Save changes"), marks.Const(uiinput.ButtonFilled))
	btn.LeadingIconRef = marks.Const("floppy-disk")
	btn.TrailingIconRef = marks.Const("check")

	rt := buttonRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, toThemeTokens(templates.Notes().Tokens), nil),
		fonts:     testkit.TestFontRegistry(t),
		icons: buttonIconResolverStub{
			"floppy-disk": mustSkeuoButtonIconAsset("floppy-disk", skeuoButtonFloppyDiskSVG),
			"check":       mustSkeuoButtonIconAsset("check", skeuoButtonCheckSVG),
		},
	}

	if mutate != nil {
		mutate(btn)
	}

	measureCtx := theme.DefaultResolvedContext()
	facet.Attach(btn, facet.AttachContext{Runtime: rt, Theme: measureCtx})

	result := btn.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirection(layout.WritingDirectionLTR),
	}, facet.Constraints{MaxSize: gfx.Size{W: 320, H: 120}})

	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	surfaceW := 360
	surfaceH := 160
	x := mathutil.Max(0, float32(surfaceW)-result.Size.W) * 0.5
	y := mathutil.Max(0, float32(surfaceH)-result.Size.H) * 0.5
	bounds := gfx.RectFromXYWH(x, y, result.Size.W, result.Size.H)

	btn.Layout.Arrange(facet.ArrangeContext{
		Runtime: rt,
		Theme:   measureCtx,
	}, bounds)

	cmds := btn.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})

	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands for golden")
	}

	surface := testkit.NewMemorySurface(surfaceW, surfaceH)
	r := softwarerenderer.NewSoftwareRenderer()
	if err := r.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}

	bgPath := gfx.RectPath(gfx.RectFromXYWH(0, 0, float32(surfaceW), float32(surfaceH)))
	bgColor := measureCtx.TokenSet().Color.Background
	bgBrush := gfx.SolidBrush(bgColor)
	bgCmd := gfx.FillPath{Path: bgPath, Brush: bgBrush}

	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:          1,
			Bounds:      gfx.RectFromXYWH(0, 0, float32(surfaceW), float32(surfaceH)),
			Opacity:     1,
			CommandHash: 1,
			Commands:    gfx.CommandList{Commands: append([]gfx.Command{bgCmd}, cmds.Commands...)},
		}},
	}

	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}

	testkit.AssertGolden(t, surface, "button_"+name)
}
