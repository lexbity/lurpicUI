// Package render defines backend-agnostic frame and surface types.
//
// Rendering backends consume render.Frame values produced by the runtime.
// Backends that need direct pixel access may request render.SoftwareSurface,
// while GPU backends can request render.VulkanSurface. The platform core
// stays on the neutral render.Surface contract.
package render
