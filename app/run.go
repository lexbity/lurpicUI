package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"time"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/log"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// initAssetManager populates rtConfig.AssetManager and AssetRegistry when they
// have not been set by the caller. The default implementation tries the
// environment variables LURPIC_ASSETS_PAK and LURPIC_ASSETS_DIR, then falls
// back to ./assets.pak and ./assets/. Platform-specific builds (Android)
// override this variable to inject the platform asset manager.
var initAssetManager = func(rtConfig *runtime.Config) {
	if rtConfig.AssetManager != nil {
		return
	}
	pakPath := os.Getenv("LURPIC_ASSETS_PAK")
	if pakPath == "" {
		if _, err := os.Stat("assets.pak"); err == nil {
			pakPath = "assets.pak"
		}
	}
	if pakPath != "" {
		pak, err := assets.NewPakFS(pakPath)
		if err == nil {
			reg := assets.NewAssetRegistryStore()
			idReg := loadIDRegistry("assets/uuid_registry.json")
			rtConfig.AssetManager = assets.NewManager(reg, pak, assets.BackendSoftware, nil, idReg)
			rtConfig.AssetRegistry = reg
			return
		}
	}
	assetsDir := os.Getenv("LURPIC_ASSETS_DIR")
	if assetsDir == "" {
		if _, err := os.Stat("assets"); err == nil {
			assetsDir = "assets"
		}
	}
	if assetsDir != "" {
		root := os.DirFS(assetsDir)
		reg := assets.NewAssetRegistryStore()
		dev, err := assets.NewDevFS(root, reg, nil)
		if err == nil {
			idReg := loadIDRegistry(filepath.Join(assetsDir, "uuid_registry.json"))
			rtConfig.AssetManager = assets.NewManager(reg, dev, assets.BackendSoftware, nil, idReg)
			rtConfig.AssetRegistry = reg
		}
	}
}

// loadIDRegistry tries to load a UUID registry JSON file. Returns nil if
// the file doesn't exist or can't be parsed — the manager falls back to
// returning empty handles for path-based lookups.
func loadIDRegistry(path string) assets.PathIDRegistry {
	reg, err := assets.LoadJSONPathRegistry(path)
	if err != nil {
		return nil
	}
	return reg
}

// surfaceProvider is implemented by platforms (e.g. Android) that provide a
// render surface directly through their lifecycle instead of through a window.
type surfaceProvider interface {
	Surface() platform.Surface
}

// newPlatformApp constructs the platform App for the current build target.
// It is assigned in build-tagged files (run_default.go for desktop,
// run_android.go for Android) so platform packages are only linked where valid.
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

// surfaceWaitTimeout is how long Run waits for a surface to become available
// on lifecycle-based platforms (e.g. Android) before giving up.
const surfaceWaitTimeout = 15 * time.Second

// Run initialises the engine, builds the root facet, and starts the main loop.
// It supports two platform models:
//   - Desktop: platform provides WindowCapable (Linux, Windows, macOS)
//   - Lifecycle-based: platform provides a Surface() method (Android)
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

	// ── Resolve window / surface ──

	var (
		window       platform.Window
		surface      render.Surface
		contentScale float32
		w, h         int
	)

	if provider, ok := platform.WindowCapableOf(platformApp); ok {
		// Desktop path: create a window and get its surface.
		var err error
		window, err = provider.NewWindow(platform.WindowOptions{
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
		contentScale = config.Runtime.ContentScale
		if contentScale <= 0 {
			contentScale = window.ContentScale()
		}
		if contentScale <= 0 {
			contentScale = 1
		}
		surface = window.Surface()
		if surface == nil {
			return errors.New("app: window surface is nil")
		}
		w, h = window.Size()
	} else if sp, ok := platformApp.(surfaceProvider); ok {
		// Lifecycle-based path (e.g. Android): the surface is created by the
		// system and delivered through the event queue. Poll events once to
		// dispatch any already-queued WindowCreated events, then wait.
		surface = sp.Surface()
		if surface == nil {
			// Drain any already-queued WindowCreated event.
			platformApp.Events().Poll()
			surface = sp.Surface()
		}
		if surface == nil {
			// Pump the event queue until the system delivers the surface. The
			// adapter sets the current surface as a side effect of draining the
			// WindowCreated event, so we must keep draining — blocking on a bare
			// callback here would deadlock, since nothing else drains the queue.
			deadline := time.Now().Add(surfaceWaitTimeout)
			for surface == nil && time.Now().Before(deadline) {
				platformApp.Events().Wait(surfaceWaitTimeout)
				surface = sp.Surface()
			}
			if surface == nil {
				return errors.New("app: timeout waiting for platform surface")
			}
		}
		w, h = surface.Size()
		contentScale = config.Runtime.ContentScale
		if contentScale <= 0 {
			contentScale = 1
		}
	} else {
		return errors.New("app: platform does not provide window or surface")
	}

	fontRegistry, err := loadFontRegistry(config.Fonts)
	if err != nil {
		return err
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
	initAssetManager(&rtConfig)
	rt, err := newRuntime(rtConfig, platformApp, window, backend, root)
	if err != nil {
		return fmt.Errorf("app: runtime: %w", err)
	}
	backendOwnedByRuntime = true

	primeRuntime(rt)
	if window != nil {
		window.Show()
	}
	return runRuntime(rt)
}

func initBackend(preferred RenderBackendKind, surface render.Surface, logger log.Logger) (render.Backend, RenderBackendKind, error) {
	if surface == nil {
		return nil, preferred, errors.New("app: window surface is nil")
	}

	// LURPIC_RENDER_BACKEND env overrides the config default.
	// On Android emulators without a Vulkan ICD, setting this to "software"
	// avoids the costly Vulkan init failure path entirely.
	if env := os.Getenv("LURPIC_RENDER_BACKEND"); env != "" {
		switch env {
		case "vulkan":
			preferred = RenderBackendVulkan
		case "software":
			preferred = RenderBackendSoftware
		default:
			return nil, preferred, fmt.Errorf("app: LURPIC_RENDER_BACKEND=%q is invalid (use \"vulkan\" or \"software\")", env)
		}
	}

	// On Android, if the surface does not support Vulkan at all, skip the
	// expensive init attempt and go straight to software.
	if preferred == RenderBackendVulkan && goruntime.GOOS == "android" {
		if vs, ok := surface.(render.VulkanSurface); ok {
			if len(vs.VulkanInstanceExtensions()) == 0 {
				if logger != nil {
					logger.Info("app: surface reports no Vulkan extensions; using software renderer")
				}
				preferred = RenderBackendSoftware
			}
		}
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
	// Check if this surface can fall back to software.
	hasSoftware, surfaceSupportsSW := surface.(render.SoftwareSurface)
	if !surfaceSupportsSW {
		return nil, preferred, fmt.Errorf("app: render: %w", err)
	}
	_ = hasSoftware

	if logger != nil {
		isUnsupported := vulkan.IsUnsupported(err)
		switch {
		case isUnsupported:
			logger.Info("app: vulkan not supported on this device; falling back to software",
				"error", err)
		default:
			logger.Warn("app: vulkan backend unavailable; falling back to software",
				"error", err)
		}
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
