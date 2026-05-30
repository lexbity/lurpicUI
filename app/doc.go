// Package app contains the application entry points and app-level wiring.
//
// The package is intentionally thin: it bootstraps the runtime, platform, and
// rendering stack used by the demos and examples.
//
// App startup prefers the Vulkan renderer and falls back to the software
// renderer when Vulkan initialization fails on the current machine. Callers can
// observe the final renderer choice through the Config callback.
//
// # Asset loading boundary
//
// Two asset pathways exist and must not be confused:
//
//   - Asset() — bootstrap-only, whole-file, no cache/budget/streaming.
//     Use for small files needed before the runtime starts (fonts, configs).
//     Reads >1 MiB produce a diagnostic warning.
//
//   - Manager (Runtime.AssetManager) — streaming, cached, budgeted, evictable.
//     Use for all media and user-facing content. Facets load assets through
//     Manager.LoadImage / LoadSVG / LoadFont / LoadTexture / LoadConfig.
package app
