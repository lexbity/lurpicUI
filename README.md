# lurpicUI - Facet Projection UX framework

A golang native UX framework for realtime 
UI , mixed media, and data visualization applications.

Features:

- Graph based multi-platform realtime rendering engine
- Asset bundling and streaming pipeline 
- Standard collection of UI elements 
- central tool for mobile app packaging 

## Requirements

- Go 1.25 or newer
- Android SDK, NDK, and JDK for Android builds
- Rust toolchain and `cargo-ndk`
- `adb` if you want to install or inspect builds on a device or emulator

**Platform notes:**

- Platform support for Linux Desktop (X11/wayland) , Android
- Android SDK can be installed through Android studio 

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

## Android build Commands

```sh
# Build a debug APK
lurpic build android

# Build a release APK
lurpic build android --release

# Build a release AAB for Play Console
lurpic build android --release --aab

# Run on a connected device
lurpic run android --device <serial>

# Run on an emulator, creating or reusing one if needed
lurpic run android --emulator

# Validate the demo suites
lurpic validate demos

# Remove generated build output
lurpic clean
```

### Build and Run Flags

- `--project <dir>` points `lurpic` at a specific project root containing
  `lurpic.toml`
- `--abi <abi>` limits the Android build to one ABI
- `--sdk-path`, `--ndk-path`, and `--jdk-path` override toolchain detection
- `--release` switches to release signing and release validation
- `--aab` produces an Android App Bundle instead of an APK
- `--emulator` starts or reuses an emulator before installing and launching
- `--force-software` disables Vulkan at runtime for the launched app

## Configuration Hierarchy

The CLI resolves Android toolchain settings in this order:

1. command-line flags
2. project `lurpic.toml`
3. user config in `~/.config/lurpic/config.toml`
4. environment variables such as `ANDROID_HOME`, `ANDROID_NDK_HOME`, and
   `JAVA_HOME`
5. auto-detection from common install paths

## Developer Notes

- `lurpic validate demos` runs the shared marks suite plus the demo module
  validation suites.
- `lurpic doctor android --verbose` prints detailed Android SDK, NDK, JDK,
  emulator, and build-tooling checks.
- Asset loading has two paths: bootstrap assets for startup-only files, and the
  runtime asset manager for cached, streaming content.
- The Android release guide documents signing, AAB validation, and Play Store
  requirements.
