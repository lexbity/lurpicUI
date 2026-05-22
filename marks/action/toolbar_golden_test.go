package action

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestToolbarGoldenDefault(t *testing.T) {
	AssertToolbarGolden(t, "default", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(toolbar *Toolbar) {})
}

func TestToolbarGoldenCompact(t *testing.T) {
	AssertToolbarGolden(t, "compact", defaultActionBarTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(toolbar *Toolbar) {})
}

func TestToolbarGoldenComfortable(t *testing.T) {
	AssertToolbarGolden(t, "comfortable", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(toolbar *Toolbar) {})
}

func TestToolbarGoldenDisabled(t *testing.T) {
	AssertToolbarGolden(t, "disabled", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(toolbar *Toolbar) {
		toolbar.SetDisabled(true)
	})
}

func TestToolbarGoldenHighContrast(t *testing.T) {
	AssertToolbarGolden(t, "high_contrast", highContrastActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(toolbar *Toolbar) {})
}

func TestToolbarGoldenHovered(t *testing.T) {
	AssertToolbarGolden(t, "hovered", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(toolbar *Toolbar) {
		toolbar.hovered = true
	})
}

func TestToolbarGoldenPressed(t *testing.T) {
	AssertToolbarGolden(t, "pressed", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(toolbar *Toolbar) {
		toolbar.pressed = true
	})
}

func TestToolbarGoldenFocused(t *testing.T) {
	AssertToolbarGolden(t, "focused", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(toolbar *Toolbar) {
		toolbar.onFocusGained()
	})
}

func TestToolbarGoldenRTL(t *testing.T) {
	AssertToolbarGolden(t, "rtl", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(toolbar *Toolbar) {})
}

func AssertToolbarGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Toolbar)) {
	t.Helper()
	toolbar, rt, measureCtx := newToolbarGoldenFixture(t, tokens, density, direction)
	if mutate != nil {
		mutate(toolbar)
	}
	renderToolbarToSurface(t, toolbar, rt, measureCtx, density, direction, name)
}

func renderToolbarToSurface(t *testing.T, toolbar *Toolbar, rt buttonRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(toolbar, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := toolbar.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 320}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	surfaceW := 1920
	surfaceH := 528
	x := maxFloat(0, float32(surfaceW)-result.Size.W) * 0.5
	y := maxFloat(0, float32(surfaceH)-result.Size.H) * 0.5
	bounds := gfx.RectFromXYWH(x, y, result.Size.W, result.Size.H)
	toolbar.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: toolbar.layoutRole.Parent,
		ChildGroup:  toolbar.layoutRole.Child,
	}, bounds)

	cmds := toolbar.projectionRole.Project(facet.ProjectionContext{
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
	testkit.AssertGolden(t, surface, "toolbar_"+goldenName)
}

func newToolbarGoldenFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*Toolbar, buttonRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	rtTokens := tokens
	rtTokens.Density.Mode = actionBarDensityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	toolbar := NewToolbar("Actions", []ToolbarGroup{
		{
			Key: "primary",
			Actions: []ActionGroupAction{
				{Key: "close", AccessibleLabel: "Close", IconRef: "close"},
				{Key: "edit", Label: "Edit", IconRef: "edit", Active: true},
			},
		},
		{
			Key: "secondary",
			Actions: []ActionGroupAction{
				{Key: "copy", Label: "Copy", IconRef: "copy"},
				{Key: "delete", Label: "Delete", IconRef: "delete"},
			},
		},
	}, &ToolbarOverflow{
		AccessibleLabel: "More options",
		TriggerIconRef:  "more",
		Entries: []MenuButtonEntry{
			{Key: "rename", Label: "Rename", IconRef: "edit"},
			{Key: "duplicate", Label: "Duplicate", IconRef: "copy"},
		},
	})
	rt := buttonRuntimeStub{
		rootStyle: rootStyle,
		fonts:     mustButtonTextRegistry(t),
		icons: buttonIconResolverStub{
			"close":    mustActionGroupIconAsset("edit"),
			"edit":     mustActionGroupIconAsset("edit"),
			"copy":     mustActionGroupIconAsset("copy"),
			"delete":   mustActionGroupIconAsset("delete"),
			"more":     mustActionGroupIconAsset("more"),
			"rename":   mustActionGroupIconAsset("edit"),
			"duplicate": mustActionGroupIconAsset("copy"),
		},
	}
	return toolbar, rt, resolved
}
