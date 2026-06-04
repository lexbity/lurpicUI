package selection

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/templates"
)

func TestTurnDialMeasureProjectHitAndAccessibility(t *testing.T) {
	td, rt, measureCtx := newTurnDialTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	td.Label = marks.Const("Volume")
	td.Min = 0
	td.Max = 100
	td.Step = 1

	facet.Attach(td, facet.AttachContext{Runtime: rt, Theme: measureCtx})

	result := td.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 500, H: 400}})

	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(10, 10, result.Size.W, result.Size.H)
	td.LayoutRole().Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: td.LayoutRole().Parent,
		ChildGroup:  td.LayoutRole().Child,
	}, bounds)

	if got := td.AccessibilityRole(); got != "slider" {
		t.Fatalf("accessibility role = %q, want slider", got)
	}
	if got := td.AccessibleName(); got != "Volume" {
		t.Fatalf("accessible name = %q, want Volume", got)
	}

	if td.cachedLabelBounds.IsEmpty() || td.cachedDialBounds.IsEmpty() || td.cachedValueLabelBounds.IsEmpty() {
		t.Fatalf("expected turn dial geometry, got label=%#v dial=%#v value=%#v", td.cachedLabelBounds, td.cachedDialBounds, td.cachedValueLabelBounds)
	}

	// Hit testing
	labelHit := td.HitRole().HitTest(gfx.Point{
		X: td.cachedLabelBounds.Min.X + td.cachedLabelBounds.Width()*0.5,
		Y: td.cachedLabelBounds.Min.Y + td.cachedLabelBounds.Height()*0.5,
	})
	if !labelHit.Hit || labelHit.MarkID != turnDialMarkIDLabel {
		t.Fatalf("expected label hit, got %#v", labelHit)
	}

	knobHit := td.HitRole().HitTest(gfx.Point{
		X: td.cachedDialBounds.Min.X + td.cachedDialBounds.Width()*0.5,
		Y: td.cachedDialBounds.Min.Y + td.cachedDialBounds.Height()*0.5,
	})
	if !knobHit.Hit || knobHit.MarkID != turnDialMarkIDKnob {
		t.Fatalf("expected knob hit, got %#v", knobHit)
	}

	cmds := td.ProjectionRole().Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands")
	}

	var sawGlyphRun, sawCircle bool
	for _, cmd := range cmds.Commands {
		switch cmd.(type) {
		case gfx.DrawGlyphRun:
			sawGlyphRun = true
		case gfx.FillPath, gfx.StrokePath:
			sawCircle = true
		}
	}
	if !sawGlyphRun {
		t.Fatal("expected glyph commands")
	}
	if !sawCircle {
		t.Fatal("expected circle rendering commands")
	}
}

func TestTurnDialPointerInteraction(t *testing.T) {
	td, rt, measureCtx := newTurnDialTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	td.Min = 0
	td.Max = 100
	td.Step = 10
	td.Value.Set(50)

	facet.Attach(td, facet.AttachContext{Runtime: rt, Theme: measureCtx})

	result := td.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 500, H: 400}})

	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	td.LayoutRole().Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: td.LayoutRole().Parent,
		ChildGroup:  td.LayoutRole().Child,
	}, bounds)

	centerX := (td.cachedDialBounds.Min.X + td.cachedDialBounds.Max.X) * 0.5
	centerY := (td.cachedDialBounds.Min.Y + td.cachedDialBounds.Max.Y) * 0.5

	if !td.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: gfx.Point{X: centerX, Y: centerY}}) {
		t.Fatal("expected enter handled")
	}
	if !td.hovered {
		t.Fatal("expected hovered true")
	}

	// Press at 135 deg relative to center (dx < 0, dy > 0)
	// dx = -cos(45)*R = -0.707 * 20, dy = sin(45)*R = 0.707 * 20
	pressPos := gfx.Point{X: centerX - 14, Y: centerY + 14}
	if !td.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: pressPos, Button: platform.PointerLeft}) {
		t.Fatal("expected press handled")
	}
	if !td.dragging {
		t.Fatal("expected dragging true")
	}
	if math.Abs(td.currentValue()-0.0) > 5.0 {
		t.Fatalf("expected pointer drag at start to yield close to 0, got %v", td.currentValue())
	}

	// Drag to 270 deg relative to start (i.e. 45 deg relative to center: dx > 0, dy > 0)
	dragPos := gfx.Point{X: centerX + 14, Y: centerY + 14}
	if !td.onPointer(facet.PointerEvent{Kind: platform.PointerMove, Position: dragPos}) {
		t.Fatal("expected drag move handled")
	}
	if math.Abs(td.currentValue()-100.0) > 5.0 {
		t.Fatalf("expected pointer drag at end to yield close to 100, got %v", td.currentValue())
	}

	// Release to click
	clicked := false
	td.Activated.Subscribe(func(signal.Unit) {
		clicked = true
	})

	if !td.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: dragPos, Button: platform.PointerLeft}) {
		t.Fatal("expected release handled")
	}
	if td.dragging || td.pressed {
		t.Fatal("expected dragging and pressed false after release")
	}
	if !clicked {
		t.Fatal("expected activation signal on release")
	}

	// Keyboard inputs
	td.Value.Set(50)
	if !td.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyLeft}) {
		t.Fatal("expected key left handled")
	}
	if td.currentValue() != 40 {
		t.Fatalf("expected key left to decrement value, got %v", td.currentValue())
	}

	if !td.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected key right handled")
	}
	if td.currentValue() != 50 {
		t.Fatalf("expected key right to increment value, got %v", td.currentValue())
	}
}

func TestTurnDialGoldenDefault(t *testing.T) {
	AssertTurnDialGolden(t, "default", defaultSliderTokens(), theme.DensityIDComfortable, func(td *TurnDial) {})
}

func TestTurnDialGoldenPressed(t *testing.T) {
	AssertTurnDialGolden(t, "pressed", defaultSliderTokens(), theme.DensityIDComfortable, func(td *TurnDial) {
		centerX := (td.cachedDialBounds.Min.X + td.cachedDialBounds.Max.X) * 0.5
		centerY := (td.cachedDialBounds.Min.Y + td.cachedDialBounds.Max.Y) * 0.5
		td.onPointer(facet.PointerEvent{
			Kind:     platform.PointerPress,
			Position: gfx.Point{X: centerX, Y: centerY},
			Button:   platform.PointerLeft,
		})
	})
}

func TestTurnDialGoldenDark(t *testing.T) {
	AssertTurnDialGolden(t, "dark", darkSliderTokens(), theme.DensityIDComfortable, func(td *TurnDial) {})
}

func TestTurnDialGoldenDarkPressed(t *testing.T) {
	AssertTurnDialGolden(t, "dark_pressed", darkSliderTokens(), theme.DensityIDComfortable, func(td *TurnDial) {
		centerX := (td.cachedDialBounds.Min.X + td.cachedDialBounds.Max.X) * 0.5
		centerY := (td.cachedDialBounds.Min.Y + td.cachedDialBounds.Max.Y) * 0.5
		td.onPointer(facet.PointerEvent{
			Kind:     platform.PointerPress,
			Position: gfx.Point{X: centerX, Y: centerY},
			Button:   platform.PointerLeft,
		})
	})
}


func AssertTurnDialGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, mutate func(*TurnDial)) {
	t.Helper()
	td, rt, measureCtx := newTurnDialTestFixture(t, tokens, density, layout.WritingDirectionLTR)
	td.Label = marks.Const("Turn Dial")
	td.Min = 0
	td.Max = 100
	td.Value.Set(30)

	facet.Attach(td, facet.AttachContext{Runtime: rt, Theme: measureCtx})

	result := td.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(layout.WritingDirectionLTR),
	}, facet.Constraints{MaxSize: gfx.Size{W: 320, H: 320}})

	surfaceW := 160
	surfaceH := 160
	x := maxFloat(0, float32(surfaceW)-result.Size.W) * 0.5
	y := maxFloat(0, float32(surfaceH)-result.Size.H) * 0.5
	bounds := gfx.RectFromXYWH(x, y, result.Size.W, result.Size.H)

	td.LayoutRole().Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: td.LayoutRole().Parent,
		ChildGroup:  td.LayoutRole().Child,
	}, bounds)

	if mutate != nil {
		mutate(td)
	}

	cmds := td.ProjectionRole().Project(facet.ProjectionContext{
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
	// Premium matte dark charcoal background color for synthesizer layout
	bgColor := gfx.ColorFromRGBA8(26, 29, 36, 255) // #1a1d24
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

	testkit.AssertGolden(t, surface, "turn_dial_"+name)
}

func newTurnDialTestFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*TurnDial, sliderRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	fonts := testkit.TestFontRegistry(t)
	rtTokens := tokens
	rtTokens.Density.Mode = densityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.NewResolvedContext(tokens).WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	td := NewTurnDial("Label", 0, 100, 1)
	rt := sliderRuntimeStub{rootStyle: rootStyle, fonts: fonts}
	return td, rt, resolved
}

func darkSliderTokens() theme.Tokens {
	return toThemeTokens(templates.UneNuit().Tokens)
}

