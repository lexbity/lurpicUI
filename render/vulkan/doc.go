// Package vulkan is the Rust-backed Vulkan bridge.
//
// The package currently exposes the Go-side backend skeleton plus the
// crates/lurpic_render bridge crate. The FFI boundary uses explicit result
// codes, opaque handles, and panic catching conventions documented in
// crates/lurpic_render/CONVENTIONS.md.
//
// Phase 4 adds optional surface creation and a clear-color swapchain present
// path for Linux/XCB surfaces. The Go side can still initialize headless for
// diagnostics, but real rendering now requests render.VulkanSurface support.
//
// Phase 11 adds Android support so the same Vulkan backend can create
// ANativeWindow-backed surfaces and recreate swapchains across Android
// lifecycle changes.
package vulkan
