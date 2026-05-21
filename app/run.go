package app

import (
	"errors"
	"fmt"
	"path/filepath"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/platform/linux"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

var newPlatformApp = linux.NewApp
var newBackend = func(kind RenderBackendKind) render.Backend {
	switch kind {
	case RenderBackendVulkan:
		return &vulkan.Backend{}
	case RenderBackendSoftware:
		return software.NewSoftwareRenderer()
	default:
		return software.NewSoftwareRenderer()
	}
}
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

	platformApp := config.PlatformApp
	createdPlatformApp := false
	if platformApp == nil {
		var err error
		platformApp, err = newPlatformApp()
		if err != nil {
			return fmt.Errorf("app: platform: %w", err)
		}
		createdPlatformApp = true
	}
	if createdPlatformApp {
		defer platformApp.Destroy()
	}

	provider, ok := platform.WindowCapableOf(platformApp)
	if !ok {
		return errors.New("app: platform does not provide window creation")
	}

	window, err := provider.NewWindow(platform.WindowOptions{
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
	contentScale := config.Runtime.ContentScale
	if contentScale <= 0 {
		contentScale = window.ContentScale()
	}
	if contentScale <= 0 {
		contentScale = 1
	}

	fontRegistry, err := loadFontRegistry(config.Fonts)
	if err != nil {
		return err
	}

	surface := window.Surface()
	if surface == nil {
		return errors.New("app: window surface is nil")
	}
	backend, selectedRender, err := initBackend(config.Render, surface, config.Runtime.Logger)
	if err != nil {
		return err
	}
	if config.OnBackendSelected != nil {
		config.OnBackendSelected(selectedRender)
	}
	backendOwnedByRuntime := false
	defer func() {
		if !backendOwnedByRuntime && backend != nil {
			backend.Destroy()
		}
	}()

	w, h := window.Size()
	themeContext := config.Theme
	if themeContext.Resolver == nil && themeContext.Materials == nil && themeContext.ContentScale == 0 && themeContext.Depth == 0 {
		themeContext = theme.DefaultResolvedContext()
	}
	themeContext = themeContext.WithFontRegistry(fontRegistry)
	if err := themeContext.TokenSet().Fonts.Validate(); err != nil {
		return err
	}

	root := builder(BuildContext{
		FontRegistry: fontRegistry,
		WindowSize:   gfx.Size{W: float32(w), H: float32(h)},
		ContentScale: contentScale,
		Theme:        themeContext,
	})
	if root == nil {
		return errors.New("app: RootBuilder returned nil")
	}

	rtConfig := config.Runtime
	rtConfig.FontRegistry = fontRegistry
	if rtConfig.LayerRegistry == nil {
		layerRegistry, err := layout.StandardLayerRegistry()
		if err != nil {
			return fmt.Errorf("app: layer registry: %w", err)
		}
		rtConfig.LayerRegistry = layerRegistry
	}
	rt, err := newRuntime(rtConfig, platformApp, window, backend, root)
	if err != nil {
		return fmt.Errorf("app: runtime: %w", err)
	}
	backendOwnedByRuntime = true

	primeRuntime(rt)
	window.Show()
	return runRuntime(rt)
}

func initBackend(preferred RenderBackendKind, surface render.Surface, logger runtime.Logger) (render.Backend, RenderBackendKind, error) {
	if surface == nil {
		return nil, preferred, errors.New("app: window surface is nil")
	}
	attempt := func(kind RenderBackendKind) (render.Backend, error) {
		backend := newBackend(kind)
		if backend == nil {
			return nil, errors.New("app: backend constructor returned nil")
		}
		if err := backend.Initialize(surface); err != nil {
			backend.Destroy()
			return nil, err
		}
		return backend, nil
	}

	backend, err := attempt(preferred)
	if err == nil {
		return backend, preferred, nil
	}
	if preferred != RenderBackendVulkan {
		return nil, preferred, fmt.Errorf("app: render: %w", err)
	}
	if _, ok := surface.(render.SoftwareSurface); !ok {
		return nil, preferred, fmt.Errorf("app: render: %w", err)
	}
	if logger != nil {
		logger.Warn("app: vulkan backend unavailable; falling back to software", "error", err)
	}
	fallback, fallbackErr := attempt(RenderBackendSoftware)
	if fallbackErr != nil {
		return nil, RenderBackendSoftware, fmt.Errorf("app: vulkan init failed: %w; software fallback failed: %v", err, fallbackErr)
	}
	return fallback, RenderBackendSoftware, nil
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
