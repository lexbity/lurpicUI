package app

import (
	"errors"
	"fmt"
	"path/filepath"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/platform/linux"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

var newPlatformApp = linux.NewApp
var newBackend = func() render.Backend { return software.NewSoftwareRenderer() }
var newFontRegistry = func() (*text.FontRegistry, error) {
	return text.NewFontRegistry()
}
var newRuntime = runtime.New
var primeRuntime = func(rt *runtime.Runtime) {
	if rt != nil {
		rt.RunOneFrame()
	}
}
var runRuntime = func(rt *runtime.Runtime) error {
	if rt == nil {
		return errors.New("app: runtime is nil")
	}
	defer rt.Shutdown()
	return rt.Run()
}

// Run initialises the engine, builds the root facet, and starts the main loop.
func Run(config Config, builder RootBuilder) error {
	if builder == nil {
		return errors.New("app: RootBuilder is required")
	}
	normalizeConfig(&config)

	platformApp, err := newPlatformApp()
	if err != nil {
		return fmt.Errorf("app: platform: %w", err)
	}
	defer platformApp.Destroy()

	window, err := platformApp.NewWindow(platform.WindowOptions{
		Title:     config.Window.Title,
		Width:     config.Window.Width,
		Height:    config.Window.Height,
		Resizable: config.Window.Resizable,
		MinSize:   config.Window.MinSize,
		MaxSize:   config.Window.MaxSize,
	})
	if err != nil {
		return fmt.Errorf("app: window: %w", err)
	}
	defer window.Destroy()
	window.SetTitle(config.Window.Title)

	fontRegistry, err := loadFontRegistry(config.Fonts)
	if err != nil {
		return err
	}

	backend := newBackend()
	if backend == nil {
		return errors.New("app: backend constructor returned nil")
	}
	backendOwnedByRuntime := false
	defer func() {
		if !backendOwnedByRuntime && backend != nil {
			backend.Destroy()
		}
	}()
	surface := window.Surface()
	if surface == nil {
		return errors.New("app: window surface is nil")
	}
	renderSurface, ok := surface.(render.Surface)
	if !ok {
		return errors.New("app: window surface does not implement render.Surface")
	}
	if err := backend.Initialize(renderSurface); err != nil {
		return fmt.Errorf("app: render: %w", err)
	}

	w, h := window.Size()
	root := builder(BuildContext{
		FontRegistry: fontRegistry,
		WindowSize:   gfx.Size{W: float32(w), H: float32(h)},
		Theme:        theme.Default(),
	})
	if root == nil {
		return errors.New("app: RootBuilder returned nil")
	}

	rtConfig := config.Runtime
	rtConfig.FontRegistry = fontRegistry
	rt, err := newRuntime(rtConfig, platformApp, window, backend, root)
	if err != nil {
		return fmt.Errorf("app: runtime: %w", err)
	}
	backendOwnedByRuntime = true

	primeRuntime(rt)
	window.Show()
	return runRuntime(rt)
}

func normalizeConfig(config *Config) {
	if config == nil {
		return
	}
	if config.Window.Width <= 0 {
		config.Window.Width = 800
	}
	if config.Window.Height <= 0 {
		config.Window.Height = 600
	}
}

func loadFontRegistry(sources []FontSource) (*text.FontRegistry, error) {
	registry, err := newFontRegistry()
	if err != nil {
		return nil, err
	}
	for _, src := range sources {
		if src.Path != "" && len(src.Data) > 0 {
			return nil, fmt.Errorf("app: font source %q cannot specify both Path and Data", src.Name)
		}
		if src.Path != "" {
			if err := registry.LoadFontFile(src.Path); err != nil {
				return nil, fmt.Errorf("app: font %q: %w", filepath.Clean(src.Path), err)
			}
			continue
		}
		if len(src.Data) > 0 {
			if src.Name == "" {
				return nil, errors.New("app: font data source requires Name")
			}
			if err := registry.LoadFontBytes(src.Data, src.Name); err != nil {
				return nil, err
			}
		}
	}
	return registry, nil
}
