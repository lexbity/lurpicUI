// Package app contains the application entry points and app-level wiring.
//
// The package is intentionally thin: it bootstraps the runtime, platform, and
// rendering stack used by the demos and examples.
//
// App startup prefers the Vulkan renderer and falls back to the software
// renderer when Vulkan initialization fails on the current machine. Callers can
// observe the final renderer choice through the Config callback.
package app
