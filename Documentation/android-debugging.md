# Android Debugging

Canonical debugging guide for Android app bring-up, live logs, crash analysis,
and screenshot capture.

If you need to preserve crash state after a failure, start the emulator
manually and use `lurpic build android` plus `adb`. The `lurpic run android
--emulator` path is convenient for iterative bring-up, but spawned emulators are
left running when the command exits.

## Prerequisites

- Android SDK with `adb` available.
- Android NDK for `ndk-stack` symbolication.
- A connected device or a running emulator.

## Live Run Loop

`lurpic run android --emulator` builds the app, installs it, launches it, and
streams logcat. Useful flags from `cmd/lurpic/run.go` are:

- `--no-logcat` to launch without streaming logs.
- `--force-software` to force `LURPIC_RENDER_BACKEND=software`.
- `--device`, `--avd`, `--abi`, `--gpu`, and `--boot-timeout` for emulator
  selection and startup control.
- `--release` to build the release variant before running.

The command keeps a spawned emulator running after exit so the next run can
reuse it.

## `lurpic logcat`

Stream or clear the device log buffer.

```sh
lurpic logcat
```

Flags:

| Flag | Default | Description |
|---|---|---|
| `--clear` | `false` | Clear the log buffer and exit. |
| `--filter` | `LurpicAsset:V LurpicBridge:V LurpicNativeActivity:V AndroidRuntime:V *:W` | Logcat filter expression. |
| `--serial` | auto | Target device serial, such as `emulator-5554`. |

Use `--clear` before reproducing a problem if you want the log stream to start
from a clean buffer.

## `lurpic crash`

Pull tombstones from the device and symbolicate them.

```sh
lurpic crash
```

Flags:

| Flag | Default | Description |
|---|---|---|
| `--serial` | auto | Target device serial. |
| `--build-dir` | `<project>/build` | Build directory containing `android/lib/<abi>/*.so`. |
| `--pull-dir` | temp dir | Local directory for pulled tombstones. |
| `--abi` | auto | Restrict analysis to one ABI, such as `arm64-v8a`. |

Behavior verified in `cmd/lurpic/diagnostics.go`:

1. It looks for `android/native-debug-symbols/<abi>` first when present.
2. Otherwise it uses `android/lib/<abi>` for the symbol set.
3. It pulls `/data/tombstones` from the device.
4. It runs `ndk-stack -sym <symbol-dir> -dump <tombstone>` for each tombstone.
5. If no tombstones are found, it scans `adb logcat -d -v time` for crash
   entries.

## `lurpic screenshot`

Capture a device screenshot or compare it against a golden reference.

```sh
lurpic screenshot -o current.png
```

Flags:

| Flag | Default | Description |
|---|---|---|
| `-o` | `screenshot_<timestamp>.png` | Output path for the captured PNG. |
| `--serial` | auto | Target device serial. |
| `--golden` | empty | Compare against a reference screenshot. |
| `--diff` | empty | Report a difference-image path on mismatch. |
| `--tolerance` | `0.01` | Maximum size-ratio delta for the golden check. |

The current golden comparison is size-ratio based, not pixel-diff based, so it
is a smoke check rather than a visual-diff engine.

## Manual Workflow

Use this flow when you need the emulator and device state to stay visible across
commands:

```sh
emulator -avd lurpic_api33_google_apis_x86_64 -no-snapshot-save -no-audio -no-boot-anim &
adb wait-for-device
until [ "$(adb shell getprop sys.boot_completed | tr -d '\r')" = "1" ]; do sleep 2; done

lurpic build android --project ./cmd/quick_square_app
adb install -r cmd/quick_square_app/build/android/org.lurpicui.quicksquare-debug.apk
adb shell am start -W -n org.lurpicui.quicksquare/org.lurpicui.bridge.LurpicNativeActivity
```

For crash work:

```sh
adb logcat -d -b crash
adb pull /data/tombstones ./tombstones
ndk-stack -sym cmd/quick_square_app/build/android/lib/x86_64 -dump ./tombstones/tombstone_00
```

## Troubleshooting

- If `lurpic run android --emulator` exits and you need the device kept alive,
  start the emulator yourself and use `lurpic build android`.
- If Vulkan initialization fails on an emulator or headless machine, rerun with
  `--force-software`.
- If `ndk-stack` cannot find symbols, check for
  `build/android/native-debug-symbols/<abi>` in a release build.
