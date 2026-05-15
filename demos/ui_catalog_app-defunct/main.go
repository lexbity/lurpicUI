package main

import (
	"flag"
	"log"
	"os"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/ui_catalog/model"
	"codeburg.org/lexbit/ui_catalog/ui"
)

func main() {
	var (
		windowWidth  = flag.Int("width", 1280, "window width")
		windowHeight = flag.Int("height", 800, "window height")
	)
	flag.Parse()

	config := app.DefaultConfig("UI Catalog", *windowWidth, *windowHeight)
	config.Fonts = defaultFontSources()
	config.Theme = ui.NewCatalogThemeContext()

	meta := model.DefaultBuildMetadata()

	if err := app.Run(config, func(ctx app.BuildContext) facet.FacetImpl {
		shaper := text.NewShaper(ctx.FontRegistry)
		shaper.SetContentScale(ctx.ContentScale)
		return ui.NewCatalogRootFacet(ctx.Theme, shaper, meta)
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
