// Package platform defines the cross-platform event, surface, and input abstractions.
//
// The runtime depends on this package for event delivery, input typing, and
// the neutral surface contract. Desktop-style window creation and clipboard
// access are exposed as optional capabilities via WindowCapable and
// ClipboardCapable. Software-specific pixel access is expressed by
// render.SoftwareSurface, while GPU-specific surface hooks live behind
// optional renderer interfaces such as render.VulkanSurface rather than the
// platform core.
//
// Platform code should keep host-specific glue in platform/<platform>/internal
// packages. Shared helper code that has a concrete reuse path across backends
// may live in platform/internal/common.
//
// Android API-level behavior is registered from platform/android/apiNN
// subpackages and resolved through the parent platform/android package.
package platform
