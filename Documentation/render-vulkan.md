# Render Vulkan

> Maturity banner: Rust-backed Vulkan bridge. The FFI contract is documented in
> `render/vulkan/crates/lurpic_render/CONVENTIONS.md`. Platform coverage and
> failure-mode behavior are only partially verified in-repo, so this page keeps
> the scope narrow.

`render/vulkan` is the Go-side bridge for the Rust Vulkan backend.

## Verified From Code

- `render/vulkan/doc.go` documents the bridge package.
- `render/vulkan/vulkan.go` exposes the core backend methods:
  `Initialize`, `Submit`, `Recreate`, `Resize`, `Destroy`, `DeviceInfo`,
  `DeviceGeneration`, and `EvictCaches`.
- `render/vulkan/ffi_linux.go` and `render/vulkan/ffi_android.go` provide the
  platform-specific CGO bindings.
- `render/vulkan/ffi_unavailable.go` returns explicit errors on unsupported
  builds.
- `render/vulkan/ffi_conventions_test.go` verifies result translation and the
  handle registry contract.
- `render/vulkan/phase3_test.go` exercises init/shutdown and capability
  queries.
- `app/run.go:initBackend` prefers Vulkan by default and falls back to software
  when Vulkan initialization fails and the surface can support software.

## What the Bridge Does

- Loads the Rust renderer and exposes its result-code contract to Go.
- Translates Rust errors into typed Go errors.
- Creates or recreates platform surfaces where the build tag supports it.
- Tracks device generation so dead GPU resources can be invalidated upstream.
- Resets the Rust-side atlas when the Go side evicts caches.

## FFI Boundary

The Rust-side conventions file defines the boundary shape:

- result codes are explicit
- handles are opaque and non-zero
- panics are caught at the boundary
- test-only exports exist for validating the contract

The Go-side wrapper follows those conventions and the tests in
`ffi_conventions_test.go` and `vulkan_test.go` exercise the translation layer.

## Unverified / Not Yet Documented

Unknown - not verifiable from the current repository:

- exact platform coverage beyond the `linux && cgo` and `android && cgo`
  build-tagged entry points
- which driver, loader, and ICD failure cases always trigger software fallback
- performance characteristics and tuning guidance
- whether every Android lifecycle path recreates the surface identically to
  desktop surface recreation

Treat those as open until the code or tests explicitly prove them.

## Related Code and Tests

- `render/vulkan/crates/lurpic_render/CONVENTIONS.md`
- `render/vulkan/ffi_conventions_test.go`
- `render/vulkan/phase3_test.go`
- `render/vulkan/vulkan_test.go`
- `app/run.go`
- `app/run_test.go`
