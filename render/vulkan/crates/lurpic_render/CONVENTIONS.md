# FFI Conventions

This crate is the Rust side of the `render/vulkan` bridge.

## Result codes

The Rust boundary uses the `RenderResult` enum:

- `Ok`
- `InitFailed`
- `OutOfMemory`
- `InvalidHandle`
- `VulkanError`
- `Panic`
- `Unknown`

These values are mirrored in Go by `render/vulkan/internal.ResultCode`.

## Error translation

Rust functions set a last-error string on failure. Go reads that message and
maps the result code to typed Go errors:

- `InitFailed` -> `*InitFailedError`
- `OutOfMemory` -> `*OutOfMemoryError`
- `InvalidHandle` -> `*InvalidHandleError`
- `VulkanError` -> `*VulkanError`
- `Panic` -> `*PanicError`
- unknown codes -> `*UnknownError`

## Panic catching

Every exported Rust function goes through `catch_render_result(...)`, which
wraps the body in `std::panic::catch_unwind`. Panics are converted to the
`Panic` result code and a diagnostic string.

## Opaque handles

`RenderHandle` is a `u64`.

Handle allocation:

- zero is reserved as an invalid sentinel
- handles start at `1`
- the registry owns handle lifetime

Handle validation:

- using a missing handle returns `InvalidHandle`
- explicit destruction removes the handle from the registry

Resource lifetime:

- explicitly destroyed resources increment the destroy counter
- resources that leave the registry without explicit destruction increment the
  drop counter during cleanup

## Test exports

The crate exposes test-only FFI functions for exercising the conventions before
real Vulkan resources exist. These functions are not a public Vulkan API; they
exist to validate the boundary shape for future phases.

## Vulkan lifecycle

The phase 3 Vulkan entry points follow the same result-code contract:

- `lurpic_render_init()` creates the Vulkan loader state, instance, device, and queue.
- `lurpic_render_shutdown()` tears that state down.
- `lurpic_render_query_capabilities()` copies the selected device summary into a caller-provided buffer.

`render/vulkan` treats `Init` and `Shutdown` as idempotent where possible so repeated backend setup in tests does not leak state.
