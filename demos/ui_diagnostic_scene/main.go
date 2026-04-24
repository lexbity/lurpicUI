package main

import (
	"flag"
	"log"
	"os"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
	"codeburg.org/lexbit/ui_diagnostic_scene/scenes"
	"codeburg.org/lexbit/ui_diagnostic_scene/shell"
)

func main() {
	var (
		windowWidth  = flag.Int("width", 1400, "window width")
		windowHeight = flag.Int("height", 900, "window height")
	)
	flag.Parse()

	config := app.DefaultConfig("UI Diagnostic Scene", *windowWidth, *windowHeight)
	config.Fonts = defaultFontSources()
	config.Theme = theme.Default()

	registry := scene.NewRegistry()
	registerScenes(registry)

	if err := app.Run(config, func(ctx app.BuildContext) facet.FacetImpl {
		shaper := text.NewShaper(ctx.FontRegistry)
		shaper.SetContentScale(ctx.ContentScale)
		return shell.NewRootFacet(ctx.Theme, shaper, registry)
	}); err != nil {
		log.Fatal(err)
	}
}

func defaultFontSources() []app.FontSource {
	candidates := []string{
		"/usr/share/fonts/noto/NotoSans-Regular.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return []app.FontSource{{Path: path, Name: "Noto Sans"}}
		}
	}
	return nil
}

func registerScenes(r *scene.Registry) {
	// Phase 3: Register implemented scenes

	r.Register(scene.Definition{
		ID:          "catalog-lite",
		DisplayName: "Catalog Lite",
		Description: "Basic primitives in a nested layout, with text and surface contrast",
		Families:    []string{"basic", "structure"},
		Factory:     func() scene.Scene { return scenes.NewCatalogLiteScene() },
	})

	r.Register(scene.Definition{
		ID:          "interaction",
		DisplayName: "Interaction",
		Description: "Hover, press, drag, click, selection, focus, and disabled input states",
		Families:    []string{"uiinput"},
		Factory:     func() scene.Scene { return scenes.NewInteractionScene() },
	})

	r.Register(scene.Definition{
		ID:          "layout",
		DisplayName: "Layout",
		Description: "Constraint extremes, nested groups, clipping, and overflow handling",
		Families:    []string{"structure"},
		Factory:     func() scene.Scene { return scenes.NewLayoutScene() },
	})

	r.Register(scene.Definition{
		ID:          "input-focus",
		DisplayName: "Input / Focus",
		Description: "Keyboard routing, tab order, caret visibility, and disabled focus targets",
		Families:    []string{"uiinput"},
		Factory:     func() scene.Scene { return scenes.NewInputFocusScene() },
	})

	r.Register(scene.Definition{
		ID:          "stress",
		DisplayName: "Stress",
		Description: "Survives repeated resize, theme, mount, and unmount churn",
		Families:    []string{"basic", "structure", "uiinput"},
		Factory:     func() scene.Scene { return scenes.NewStressScene() },
	})

	// Phase 5: Projection and layout debugging
	r.Register(scene.Definition{
		ID:          "projection",
		DisplayName: "Projection",
		Description: "Child transforms, anchor forwarding, hit regions, and viewport projection",
		Families:    []string{"structure"},
		Factory:     func() scene.Scene { return scenes.NewProjectionScene() },
	})

	// Phase 6: Stress and portability (enhanced scenes)
	r.Register(scene.Definition{
		ID:          "animation",
		DisplayName: "Animation / Ticking",
		Description: "Tick delivery, timeline progression, and frame jank detection",
		Families:    []string{"basic"},
		Factory:     func() scene.Scene { return scenes.NewAnimationScene() },
	})

	r.Register(scene.Definition{
		ID:          "theme",
		DisplayName: "Theme",
		Description: "Token propagation, state colors, and density changes",
		Families:    []string{"basic"},
		Factory:     func() scene.Scene { return scenes.NewThemeScene() },
	})

	r.Register(scene.Definition{
		ID:          "store-signal",
		DisplayName: "Store / Signal",
		Description: "Invalidation, store fanout, and state replay",
		Families:    []string{"basic"},
		Factory:     func() scene.Scene { return scenes.NewStoreSignalScene() },
	})

	r.Register(scene.Definition{
		ID:          "text-ime",
		DisplayName: "Text / IME",
		Description: "Text entry, composing state, and caret movement",
		Families:    []string{"uiinput"},
		Factory:     func() scene.Scene { return scenes.NewTextIMEScene() },
	})

	r.Register(scene.Definition{
		ID:          "annotation",
		DisplayName: "Annotation",
		Description: "Labels, connectors, badges, callouts, and handles",
		Families:    []string{"annotation"},
		Factory:     func() scene.Scene { return scenes.NewAnnotationScene() },
	})

	r.Register(scene.Definition{
		ID:          "chart",
		DisplayName: "Chart",
		Description: "Axes, scaling, and density-aware chart layout",
		Families:    []string{"chart"},
		Factory:     func() scene.Scene { return scenes.NewChartScene() },
	})

	r.Register(scene.Definition{
		ID:          "uinav",
		DisplayName: "UI Navigation",
		Description: "Tabs, drawer, menus, pagination, scrollbars, and speed-dial marks",
		Families:    []string{"uinav"},
		Factory:     func() scene.Scene { return scenes.NewUINavScene() },
	})

	r.Register(scene.Definition{
		ID:          "uinotification",
		DisplayName: "UI Notification",
		Description: "Snackbar, dialog, and progress notification marks",
		Families:    []string{"uinotification"},
		Factory:     func() scene.Scene { return scenes.NewUINotificationScene() },
	})
}
