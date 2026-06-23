# Testing

This document covers the current test gates, golden-image workflow, and the
known sharp edges that affect local verification.

## Verified Gates

- `cmake --build build --target test-unit` passed after redirecting `GOCACHE`
  and `GOTMPDIR` to writable temp directories.
- `cmake --build build --target lint` currently fails in `golangci-lint` with 37
  `unused` findings in `platform/linux/linux_app.go` and
  `platform/linux/linux_testhelpers.go`.
- `cmake --build build --target build-lurpic-cli` and
  `cmake --build build --target build-lurpiclint` both completed successfully.

## CMake Test Targets

| Target | Coverage | Status |
|---|---|---|
| `test-unit` | Host tests via `lurpic-host-tests` (`vet`, `build`, `test` with `CGO_ENABLED=0`). | Verified passed. |
| `test-all` | Aggregates `test-pure`, `test-headless`, `test-render-software`, and `test-integration`. | Present, not run in this pass. |
| `test-pure` | Core package tests for `gfx`, `signal`, `store`, `job`, `layout`, `text`, `theme`, `facet`, `runtime`, `graph/index`, `internal/hashutil`, `internal/renderutil`, `render/vulkan`, and `marks`. | Present, not run in this pass. |
| `test-headless` | `internal/testkit`, `projection`, `diagnostics`, `graph/canvas`. | Present, not run in this pass. |
| `test-render-software` | `render/software` tests when the software backend is enabled. | Present in this configuration. |
| `test-integration` | `marks/integration` tests. | Present, not run in this pass. |
| `golden-verify` | Golden-image verification for `internal/testkit`. | Present in CMake, not run in this pass. |
| `golden-update` | Golden-image regeneration for `internal/testkit`. | Conditional on `LURPIC_UPDATE_GOLDENS`; not run in this pass. |

## Golden Images

`internal/testkit/golden.go` implements the golden workflow.

| Mechanism | Effect |
|---|---|
| `LURPICUI_UPDATE_GOLDEN=1` | Regenerates the expected golden image files. |
| `-update-golden` | Flag form of the same behavior. |
| `TESTKIT_GOLDEN_DEBUG=1` | Prints extra diagnostic output, including base directory and image digests. |

The policy in `Documentation/development/golden-policy.md` says goldens are
assertions, not auto-generated baselines. Missing goldens fail tests instead of
creating files implicitly.

## Coverage Notes

- `job/job_test.go` is the only test file under `job/`, so the async job model
  remains relatively thinly covered compared with the larger packages.
- `store/projection_test.go` and `store/projection_test_helper_test.go` exercise
  the projection-phase mutation guard that panics when stores mutate during a
  projection pass.
- The repository still has a lot of marks coverage, but the current docs and
  tests should be read as "present behavior" rather than a guarantee of stable
  public API.

## Practical Workflow

1. Run `cmake --build build --target test-unit` for a fast contributor gate.
2. Run `cmake --build build --target lint` when you want the stricter lint
   gate.
3. Use `LURPICUI_UPDATE_GOLDEN=1` when an intentional visual change needs new
   goldens.
4. Set `TESTKIT_GOLDEN_DEBUG=1` when a golden comparison is hard to diagnose.

## Known Friction

- The documented lint target currently fails on existing `unused` findings in
  `platform/linux`.
- The Go toolchain in this workspace needs writable temp caches when invoked
  through CMake.
- The repository contains a large number of marks tests, but the audit notes
  still treat marks as the most change-sensitive surface.
