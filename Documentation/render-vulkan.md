# Render Vulkan

`render/vulkan` is the Go side of the Vulkan GPU rasterization backend. The
actual GPU pipeline lives in the Rust crate
(`render/vulkan/crates/lurpic_render`), built on the `ash` Vulkan bindings; the
Go side is a thin bridge: frame encoding, FFI, and the `render.Backend`
contract.

This page describes the pipeline as implemented (Slice 10). The equivalence
baseline and measured numbers live in
`devdocs/notes/vulkan-equivalence-baseline.md` and
`devdocs/notes/vulkan-gpu-pipeline-benchmarks.md`.

## Relationship to the software backend

`render/vulkan` and `render/software` are feature-equivalent dynamic peers of
the same `render.Backend` contract. Software rendering is the always-present,
dependency-free baseline and the correctness oracle for the equivalence harness
(`render/equivalence`); GPU rendering is optional acceleration. The runtime is
unaware which backend is active — `Backend.Submit` is the only call site.

Backend selection is honest (FR-11): `app/run.go:initBackend` initializes the
Vulkan backend, then builds a real graphics pipeline through it
(`lurpic_render_build_pipeline_probe`). A device that cannot construct the
renderer's pipelines selects the software backend upfront — a feature or symbol
check alone is not enough. A Vulkan runtime failure (`*render.ErrGPUFatal`,
FR-10) triggers a one-shot GPU→software swap for the rest of the session (Q9).

## The GPU pipeline (Rust crate)

The crate renders `gfx` commands through real Vulkan 1.3 pipelines:

- **Solid fills/strokes** — instanced quads; the 2×3 affine transform travels
  as a push constant (Q4), never baked into vertices; the fragment shader
  computes analytic coverage AA.
- **Path fill** — stencil-buffer winding accumulation over CPU-flattened
  contours; the cover shader evaluates the nonzero winding with a
  supersample coverage grid (Slice 7). Self-intersections and holes render
  correctly (FR-5).
- **Strokes** — the Go encoder expands every stroke to the fill path of its
  outline via `gfx.ExpandStroke`/`gfx.OffsetContour` (Slice 8); the full
  `StrokeStyle` (caps, joins, miter limit, dash) is honored.
- **Linear gradients** — a fragment-shader gradient brush over a per-group UBO
  of stops (Slice 6).
- **Text** — a packed glyph atlas (bitmap below 24 px, SDF above) with a
  `smoothstep` reconstruction shader (Slice 5).
- **Textures** — `DrawImage`/`DrawTexture` sample real `VkImage`s via
  combined image samplers (Slice 4).
- **Blurred shadows** — `DrawBlurredShadow` renders the path's coverage mask
  into an R8 scratch, applies a separable Gaussian (H then V), and composites
  the tinted mask at the shadow's offset; `Inner` inverts the mask (Slice 9).
  The software oracle gained full parity in-slice.

### Dirty-region redraw (Q13)

The runtime computes `Frame.DirtyRegions` (merged bounds of the frame's changed
facets); the GPU consumes them on non-tile-based devices: every draw is
scissor-restricted to the dirty union (off-region draws are culled), and a
present-side target preserves its prior content with per-region clears. A
multi-buffered swapchain's content is stale by `image_count` frames, so the
renderer tracks per-image accumulated dirty regions. Tile-based mobile GPUs
gate the feature off initially (`LurpicRenderPipelineFeatures.tile_based`).

## The Go bridge

- `render/vulkan/vulkan.go` — the `render.Backend`, `render.RecreatableBackend`,
  `render.TextureBackend`, `render.CacheEvictor`, and
  `render.DeviceGenerationProvider` implementations. `Submit` returns
  `*render.ErrGPUFatal` (FR-10) for device-lost / out-of-memory, which the
  runtime answers with the one-shot software fallback.
- `render/vulkan/packet.go` — the packet v2 wire format encoder (every `gfx`
  command, full `StrokeStyle`, both brush kinds, per-batch transform/clip,
  dirty regions).
- `render/vulkan/ffi_linux.go`, `ffi_android.go` — the CGO bindings; the
  dlsym table and C declarations are generated from the Rust FFI inventory
  (`ffi_gen_test.go`, `TestFFISymbols_InSync`).
- `render/vulkan/toolchain.go` — builds the Rust crate on demand.
- `render/vulkan/equivalence` harness and `render/equivalence/` corpus verify
  GPU output against the software oracle within the Q1 perceptual tolerance.

## FFI boundary

The Rust-side `CONVENTIONS.md` defines the boundary shape: explicit result
codes, opaque handles, `catch_unwind` at every export. The Go side mirrors the
result codes in `render/vulkan/internal`; a `RenderResult::DeviceLost` maps to
`*render.ErrGPUFatal` (via `translateSubmitError`).

## Failure modes

| Condition | Behavior |
|---|---|
| No Vulkan loader / no 1.3 device | `Backend.Initialize` fails; `initBackend` selects software (FR-11). |
| Pipeline build fails on an initializing device | `lurpic_render_build_pipeline_probe` fails; software selected upfront (FR-11). |
| Device lost / GPU OOM during `Submit` | `*render.ErrGPUFatal`; the render thread swaps to software for the session (Q9/FR-12), emits `OnBackendFallback`. |
| Surface out of date | Handled inside the Rust side (`VK_ERROR_OUT_OF_DATE_KHR` → swapchain recreate); the Go `Recreate` path rebuilds the surface/swapchain on lifecycle events. |
