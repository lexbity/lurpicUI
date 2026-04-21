// Package animation provides time-based interpolation, easing, and timelines.
//
// The package is designed to sit between authored values and runtime-driven
// state. It exposes interpolatable values, easing registries, animated value
// wrappers, and timeline playback controls. Runtime-bound timelines register
// phase-1 hooks on the runtime; standalone timelines may be ticked by tests or
// legacy callers through the nil-runtime registry.
package animation
