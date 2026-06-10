# Android Development Setup

This document describes how to set up your development environment for building lurpicUI applications for Android.

## Prerequisites

- Go 1.22 or later
- Rust 1.70 or later (with cargo-ndk for Android builds)
- Android SDK (API 29+, target API 33)
- Android NDK (r21 or later)
- Java JDK 11 or later (for signing and dex generation)

## Installing the Android SDK and NDK

### Option 1: Android Studio (Recommended)

1. Download and install [Android Studio](https://developer.android.com/studio)
2. Open Android Studio and go to SDK Manager (Tools > SDK Manager)
3. Install the following:
   - **SDK Platforms**: Android 10.0 (API 29) and Android 13.0 (API 33)
   - **SDK Tools**:
     - Android SDK Build-Tools
     - Android SDK Platform-Tools
     - Android SDK Tools
     - NDK (Side by side) - version 25.x or later
     - CMake (optional, for native builds)

### Option 2: Command Line Tools Only

1. Download [Android command line tools](https://developer.android.com/studio#command-line-tools-only)
2. Extract to a location like `~/Android/cmdline-tools`
3. Run `sdkmanager` to install components:

```bash
sdkmanager "platforms;android-29"
sdkmanager "platforms;android-33"
sdkmanager "build-tools;33.0.0"
sdkmanager "platform-tools"
sdkmanager "ndk;25.2.9519653"
```

## Environment Variables

Set these environment variables in your shell profile:

```bash
# Android SDK location
export ANDROID_HOME=$HOME/Android/Sdk

# Android NDK (optional - will be auto-detected from SDK)
export ANDROID_NDK_HOME=$ANDROID_HOME/ndk/25.2.9519653

# Add SDK tools to PATH
export PATH=$PATH:$ANDROID_HOME/platform-tools
export PATH=$PATH:$ANDROID_HOME/cmdline-tools/latest/bin

# Java (required for signing)
export JAVA_HOME=/usr/lib/jvm/java-11-openjdk  # Adjust for your system
```

## Installing Rust Android Targets

Install the Android targets for Rust:

```bash
rustup target add aarch64-linux-android
rustup target add armv7-linux-androideabi
```

Install cargo-ndk (optional but recommended):

```bash
cargo install cargo-ndk
```

## Verifying Your Setup

Run the lurpic build tool to verify everything is configured:

```bash
cd test_apps/android_hello
lurpic version
```

You should see:
```
lurpic version 0.1.0-dev
lurpicUI build tool
```

## Building Your First APK

Navigate to a project with a `lurpic.toml` file and run:

```bash
lurpic build android
```

This will:
1. Detect your Android SDK and NDK
2. Cross-compile the Go code for Android arm64
3. Cross-compile any Rust crates
4. Generate `AndroidManifest.xml`
5. Bundle assets (if present)
6. Assemble and sign the APK

The output APK will be in `./build/android/`.

## Installing on a Device

Connect an Android device with USB debugging enabled:

```bash
adb devices  # Verify device is connected
lurpic run android  # Build, install, and launch
```

Or manually:

```bash
adb install build/android/org.lurpic.hello-debug.apk
adb shell am start -n org.lurpic.hello/android.app.NativeActivity
```

To launch the sample app on an emulator, create or start an AVD first and then use:

```bash
cd test_apps/android_hello
lurpic run android --emulator
```

## Troubleshooting

### "Android SDK not found"

- Verify `ANDROID_HOME` is set correctly
- Check that the SDK directory contains `platform-tools/`, `build-tools/`, and `platforms/`

### "Android NDK not found"

- Set `ANDROID_NDK_HOME` explicitly, or
- Ensure the NDK is installed in `$ANDROID_HOME/ndk/`

### "apksigner not found"

- Install build-tools via SDK Manager: `sdkmanager "build-tools;33.0.0"`

### Go cross-compile fails

- Verify you have a recent Go version (1.22+)
- Ensure the NDK clang compiler is at the expected path

### Rust build fails

- Install Android targets: `rustup target add aarch64-linux-android`
- Try using cargo-ndk: `cargo install cargo-ndk`

## Project Configuration

Create a `lurpic.toml` file in your project root:

```toml
[app]
id = "com.example.myapp"
name = "My App"
version = "1.0.0"

[android]
min_sdk = 29
target_sdk = 33

[android.permissions]
required = ["android.permission.INTERNET"]
optional = ["android.permission.CAMERA"]
```

See `test_apps/android_hello/lurpic.toml` for a minimal example.
