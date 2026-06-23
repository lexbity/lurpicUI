# Contributing to lurpicUI

This repository uses Go as the source of truth for package behavior and CMake as
the build orchestration layer.

## Prerequisites

- Go 1.25 or newer
- CMake
- Rust toolchain and `cargo-ndk` for the Vulkan backend
- Android SDK, NDK, and JDK for Android builds
- `adb` for device and emulator workflows

## Local Workflow

1. Configure a build directory if needed.
   - `cmake -S . -B build`
2. Run the repo lint gate before handing off Go changes.
   - `cmake --build build --target lint`
3. Run the unit-test target for the framework.
   - `cmake --build build --target test-unit`
4. Run broader checks when the change touches more than one subsystem.
   - `cmake --build build --target test-all`

## Common Build Targets

- `build-lurpic-cli` builds the `lurpic` application builder.
- `build-lurpiclint` builds the static analyzer binary.
- `lint-lurpiclint` runs `lurpiclint` over the framework and demo app.
- `lint-lurpiclint-ci` runs the same analyzer with GitHub annotation output.
- `run-demo-quick_square_app` runs the verified demo application.
- `android-emulator` prepares the emulator workflow used by the Android build
  pipeline.

## Review Expectations

- Keep documentation claims backed by code, tests, config, or scripts that are
  present in the repository.
- Prefer tracked documentation under `Documentation/` for canonical material.
- Use `devdocs/` for working notes, drafts, and plans.
- If a change touches Go code, verify it with `cmake --build build --target lint`
  before handoff.
- If a change touches runtime behavior, add or update tests alongside it.

## Docs Changes

- Keep the root `README.md` focused on the first impression and quick start.
- Link framework contributors to [Documentation/README.md](Documentation/README.md)
  instead of duplicating deep build or architecture notes in the root readme.
