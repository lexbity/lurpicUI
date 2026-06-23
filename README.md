# lurpicUI

A Go-based UX framework for realtime UI, mixed media, and data visualization
applications.

It provides:

- a retained facet tree and projection system
- a cross-platform rendering pipeline
- asset bundling and streaming support
- Android application build tooling

## Requirements

- Go 1.25 or newer
- CMake for the build, test, and lint workflow
- Android SDK, NDK, and JDK for Android builds
- Rust toolchain and `cargo-ndk`
- `adb` if you want to install or inspect builds on a device or emulator

**Platform notes:**

- Platform support is currently Linux desktop and Android.

Run the Android doctor first if your toolchain is not already configured:

```sh
lurpic doctor android
```

## Quick Demo

```sh
go test ./...
go run ./demos/quick_square_app
```

The demo is a minimal smoke test that exercises the app startup path and the
software renderer.

## Building An App

To ship an application with lurpicUI, create a project directory with:

- a `lurpic.toml` file at the project root
- a Go entry point, usually under `cmd/<app-name>/main.go`
- any assets under `assets/` or your own asset directory

For framework development, build verification, and contributor workflow, see
[CONTRIBUTING.md](CONTRIBUTING.md) and [Documentation/README.md](Documentation/README.md).
