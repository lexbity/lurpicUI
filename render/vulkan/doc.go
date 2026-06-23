// Package vulkan is the Rust-backed Vulkan bridge.
//
// Maturity banner: bridge behavior is partially verified, and platform coverage
// is intentionally conservative in the docs. The FFI boundary uses explicit
// result codes, opaque handles, and panic-catching conventions documented in
// render/vulkan/crates/lurpic_render/CONVENTIONS.md.
//
// Use this package as a shape reference for the current backend bridge, not as
// a promise of stable cross-platform Vulkan behavior.
package vulkan
