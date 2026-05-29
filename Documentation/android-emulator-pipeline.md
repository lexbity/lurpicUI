# Android Emulator Pipeline

## One command

```sh
lurpic run android --emulator
```

This performs the following steps automatically:

1. **Build** — cross-compiles Go (`-buildmode=c-shared`) and Rust (`cargo build --release --target <triple>`) for the emulator ABI (x86_64 by default)
2. **Provision** — accepts SDK licenses, downloads the `system-images;android-<api>;google_apis;x86_64` system image, and creates a managed AVD named `lurpic_api<api>_google_apis_x86_64`
3. **Boot** — launches the emulator with `-no-snapshot -no-boot-anim -gpu auto -port 5554`, waits for `sys.boot_completed == 1` and `pm path android` to succeed
4. **Install** — `adb install -r <apk>`
5. **Launch** — `adb shell am start -n <package>/org.lurpicui.bridge.LurpicNativeActivity`

## Usage

```sh
# Build only
lurpic build android
lurpic build android --release
lurpic build android --abi x86_64

# Build and run on emulator
lurpic run android --emulator
lurpic run android --emulator --gpu host
lurpic run android --emulator --force-software
lurpic run android --emulator --boot-timeout 10m

# Build and run on connected device
lurpic run android --device emulator-5554
lurpic run android --device <serial>

# AVD selection (highest to lowest priority):
#   1. --avd flag
#   2. LURPIC_ANDROID_AVD env
#   3. ANDROID_AVD_NAME env
#   4. Managed AVD (lurpic_api<api>_google_apis_x86_64, created on demand)
lurpic run android --emulator --avd MyCustomAVD

# Renderer override:
#   LURPIC_RENDER_BACKEND=vulkan   (default)
#   LURPIC_RENDER_BACKEND=software (override)
lurpic run android --emulator --force-software
```

## Doctor

```sh
lurpic doctor android
lurpic doctor android --verbose
```

Reports the Android toolchain status including:

- Go, Rust, cargo-ndk versions
- Android SDK, NDK, JDK detection
- Emulator binary, sdkmanager, avdmanager
- x86_64 google_apis system image presence
- Managed AVD presence

## Manual visual test

1. Ensure an Android emulator AVD exists:
   ```sh
   avdmanager create avd -n test_lurpic -k "system-images;android-33;google_apis;x86_64" -d pixel_6
   ```

2. Start the emulator:
   ```sh
   emulator -avd test_lurpic -no-snapshot -no-boot-anim -gpu auto -port 5554 &
   ```

3. Wait for boot:
   ```sh
   adb -s emulator-5554 wait-for-device
   adb -s emulator-5554 shell getprop sys.boot_completed
   ```

4. Build and install:
   ```sh
   lurpic build android --abi x86_64
   adb -s emulator-5554 install -r ./build/android/*-debug.apk
   ```

5. Launch:
   ```sh
   adb -s emulator-5554 shell am start -n <app-id>/org.lurpicui.bridge.LurpicNativeActivity
   ```

6. Accept the visual outcome manually — the app should appear in the emulator window.

## Renderers

| Mode | Emulator `-gpu` | App env |
|------|-----------------|---------|
| Default | `auto` (workstation GPU) | Vulkan, fallback to software |
| `--gpu host` | `host` | Same |
| `--gpu swiftshader_indirect` | `swiftshader_indirect` | Same |
| `--force-software` | `auto` | `LURPIC_RENDER_BACKEND=software` |

## CMake convenience targets

```sh
cmake --preset host-tests
cmake --build --preset build-host-tests

cmake --preset android-emulator
cmake --build --preset build-android-emulator
```
