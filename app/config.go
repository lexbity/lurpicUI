package app

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// FontSource describes a font input for application startup.
type FontSource = text.FontSource

// WindowConfig configures the application window.
type WindowConfig struct {
	Title     string
	Width     int
	Height    int
	MinSize   gfx.Size
	MaxSize   gfx.Size
	Resizable bool
}

// RenderBackendKind selects the preferred renderer implementation.
type RenderBackendKind uint8

const (
	RenderBackendVulkan RenderBackendKind = iota
	RenderBackendSoftware
)

func (k RenderBackendKind) String() string {
	switch k {
	case RenderBackendVulkan:
		return "vulkan"
	case RenderBackendSoftware:
		return "software"
	default:
		return "unknown"
	}
}

// Config configures the application entry point.
type Config struct {
	Window  WindowConfig
	Runtime runtime.Config
	Fonts   []FontSource
	Theme   theme.Context
	// Render selects the preferred renderer. Vulkan is the default; software
	// is used as a fallback when Vulkan initialization fails.
	Render RenderBackendKind
	// OnBackendSelected reports the renderer that actually initialized.
	OnBackendSelected func(RenderBackendKind)
}

// BuildContext is passed to the root builder after engine startup.
type BuildContext struct {
	FontRegistry *text.FontRegistry
	WindowSize   gfx.Size
	ContentScale float32
	Theme        theme.Context
}

// RootBuilder constructs the root facet after all engine systems are ready.
type RootBuilder func(ctx BuildContext) facet.FacetImpl

// DefaultConfig returns a usable default app configuration.
func DefaultConfig(title string, w, h int) Config {
	return Config{
		Window: WindowConfig{
			Title:     title,
			Width:     w,
			Height:    h,
			Resizable: true,
		},
		Runtime: runtime.DefaultConfig(),
		Render:  RenderBackendVulkan,
	}
}
