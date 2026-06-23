# Build System

`CMakeLists.txt` is the orchestration layer for lurpicUI. Go remains the source
of truth for package behavior, but CMake wires together toolchain detection,
test grouping, packaging, and convenience targets.

## Verified Execution

The following target runs were executed during this documentation pass:

- `cmake --build build --target test-unit` - passed after redirecting `GOCACHE`
  and `GOTMPDIR` to writable temp directories.
- `cmake --build build --target lint` - failed in `golangci-lint` with 37
  `unused` findings in `platform/linux/linux_app.go` and
  `platform/linux/linux_testhelpers.go`.
- `cmake --build build --target build-lurpic-cli` - completed successfully.
- `cmake --build build --target build-lurpiclint` - completed successfully.
- `cmake --build build --target help` - listed the available CMake targets.

## Core Targets

| Target | Purpose | Status in this pass |
|---|---|---|
| `test-unit` | Runs host tests via `lurpic-host-tests`: `vet`, `build`, and `test` with `CGO_ENABLED=0`. | Verified passed. |
| `test-all` | Aggregates `test-pure`, `test-headless`, `test-render-software`, and `test-integration`. | Present in CMake, not run in this pass. |
| `lint` | Runs `lint-golangci` and `lint-lurpiclint`. | Verified, but failed because `golangci-lint` reported unused-symbol issues. |
| `lint-golangci` | Runs `golangci-lint run ./...` with `CGO_ENABLED=0`. | Verified, failed for the same unused-symbol issues. |
| `lint-lurpiclint` | Runs `lurpiclint check --fail-on error ./... ./demos/quick_square_app/...`. | Present and reachable; built as part of `lint`. |
| `lint-lurpiclint-ci` | Same as `lint-lurpiclint` but emits GitHub annotations. | Present in CMake, not run in this pass. |
| `build-lurpic-cli` | Builds the `lurpic` binary. | Verified passed. |
| `build-lurpiclint` | Builds the `lurpiclint` binary. | Verified passed. |
| `build-render-vulkan-rust` | Builds the Rust bridge crate for the Vulkan backend. | Present only when `LURPIC_BACKEND_VULKAN=ON`; not run in this pass because the current build configured Vulkan off. |
| `run-demo-quick_square_app` | Runs the quick-square demo. | Present in CMake, not run in this pass. |
| `android-emulator` | Launches the emulator workflow through `lurpic run android --emulator`. | Present in CMake, not run in this pass. |

## Target Groups

The CMake build groups packages and features around the active backend
configuration:

- `build-backend-software` builds `./render/software/...` when the software
  backend is enabled.
- `build-backend-vulkan` builds `./render/vulkan/...` and depends on
  `build-render-vulkan-rust` when the Vulkan backend is enabled.
- `test-pure` covers core packages such as `./gfx/...`, `./signal/...`,
  `./store/...`, `./job/...`, `./layout/...`, `./text/...`, `./theme/...`,
  `./facet/...`, `./runtime/...`, `./graph/index/...`, `./internal/hashutil/...`,
  `./internal/renderutil/...`, `./render/vulkan/...`, and `./marks`.
- `test-headless` covers `./internal/testkit/...`, `./projection/...`,
  `./diagnostics/...`, and `./graph/canvas/...`.
- `test-render-software` covers `./render/software/...` and is only added when
  the software backend is enabled.
- `test-integration` covers `./marks/integration/...`.

## Practical Notes

- `test-unit` is the default contributor gate for Go code changes.
- `lint` is stricter than `test-unit` and currently blocks on repository lint
  debt in `platform/linux`.
- `build-render-vulkan-rust` is conditional on Vulkan detection during CMake
  configure.
- The generated `build/` tree already contains the configured toolchain summary
  and target list.
