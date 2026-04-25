// Package linux contains the Linux platform backend.
//
// The public surface is intentionally thin: NewApp is the only constructor and
// all X11/XCB, SHM, epoll, and clipboard work lives in internal packages.
//
// Linux apps implement platform.WindowCapable and platform.ClipboardCapable
// even though those capabilities are not part of platform.App. Linux surfaces
// satisfy render.SoftwareSurface for the software renderer, and the cgo
// display backend also exposes render.VulkanSurface for Vulkan presentation.
//
// Internal packages:
//   - internal/display: cgo-backed X11 window, surface, event, and clipboard handling
//   - internal/input: Linux-local wrappers around shared key/modifier translation
//   - platform/internal/common: reusable platform helpers shared across backends
package linux
