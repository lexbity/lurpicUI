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
		Description: "Reduced catalog rendering without diagnostic noise",
		Families:    []string{"basic", "structure"},
		Factory:     func() scene.Scene { return scenes.NewCatalogLiteScene() },
	})

	r.Register(scene.Definition{
		ID:          "interaction",
		DisplayName: "Interaction",
		Description: "Hover, press, drag, click, selection, and focus transitions",
		Families:    []string{"uiinput"},
		Factory:     func() scene.Scene { return scenes.NewInteractionScene() },
	})

	r.Register(scene.Definition{
		ID:          "layout",
		DisplayName: "Layout",
		Description: "Constraint extremes, nesting, clipping, overflow handling",
		Families:    []string{"structure"},
		Factory:     func() scene.Scene { return scenes.NewLayoutScene() },
	})

	r.Register(scene.Definition{
		ID:          "stress",
		DisplayName: "Stress",
		Description: "Survives repeated resize/theme/mount/unmount churn",
		Families:    []string{"basic", "structure", "uiinput"},
		Factory:     func() scene.Scene { return scenes.NewStressScene() },
	})

	// Phase 5: Projection and layout debugging
	r.Register(scene.Definition{
		ID:          "projection",
		DisplayName: "Projection",
		Description: "Child transforms, hit regions, and viewport projection",
		Families:    []string{"structure"},
		Factory:     func() scene.Scene { return scenes.NewProjectionScene() },
	})

	// Phase 6: Stress and portability (enhanced scenes)
	r.Register(scene.Definition{
		ID:          "animation",
		DisplayName: "Animation / Ticking",
		Description: "Tick delivery, timeline progression, frame jank detection",
		Families:    []string{"basic"},
		Factory:     func() scene.Scene { return scenes.NewAnimationScene() },
	})

	// Phase 7+ Placeholder scenes (to be implemented)
	registerPlaceholderScene(r, "theme", "Theme", "Token propagation, state colors, density changes", []string{"basic"})
	registerPlaceholderScene(r, "store-signal", "Store / Signal", "Invalidation, signal fanout", []string{"basic"})
	registerPlaceholderScene(r, "text-ime", "Text / IME", "Text entry, composing state", []string{"uiinput"})
	registerPlaceholderScene(r, "annotation", "Annotation", "Labels, connectors, badges", []string{"annotation"})
	registerPlaceholderScene(r, "chart", "Chart", "Axes and scale-driven layout", []string{"chart"})
}

func registerPlaceholderScene(r *scene.Registry, id, name, desc string, families []string) {
	r.Register(scene.Definition{
		ID:          id,
		DisplayName: name,
		Description: desc,
		Families:    families,
		Factory:     func() scene.Scene { return &placeholderScene{id: id, name: name} },
	})
}

// placeholderScene is a minimal scene implementation for unimplemented phases
type placeholderScene struct {
	id   string
	name string
}

func (p *placeholderScene) SceneID() string                   { return p.id }
func (p *placeholderScene) DisplayName() string               { return p.name }
func (p *placeholderScene) BuildRoot() facet.FacetImpl        { return nil }
func (p *placeholderScene) Reset()                            {}
func (p *placeholderScene) ApplyTheme(theme.Context)          {}
func (p *placeholderScene) ApplyDensity(float32)              {}
func (p *placeholderScene) Capabilities() scene.CapabilitySet { return scene.CapabilitySet{} }
func (p *placeholderScene) ExportState() map[string]any       { return nil }
func (p *placeholderScene) ImportState(map[string]any)        {}
