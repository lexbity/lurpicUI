# Android Debugging Guide

## Overview

This document describes the debugging workflow for lurpicUI Android builds. The
tooling covers three scenarios:

1. **Log streaming** — watching app output in real time.
2. **Crash analysis** — pulling native tombstones and symbolicating them.
3. **Generic `adb` diagnostics** — device logs, tombstones, and symbol bundles.

---

## Prerequisites

- Android SDK (set `ANDROID_HOME` or detect via auto-detection).
- Android NDK (set `ANDROID_NDK_HOME` or detect from SDK) — required for
  `ndk-stack` symbolication.
- A connected device or running emulator.

---

## Commands

### `lurpic logcat`

Stream the device log buffer with lurpicUI-specific filters:

```sh
lurpic logcat
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--clear` | `false` | Clear the log buffer and exit. |
| `--filter` | `LurpicBridge:V LurpicNativeActivity:V AndroidRuntime:V *:W` | Logcat filter expression. |
| `--serial` | auto | Target device serial (e.g. `emulator-5554`). |

Examples:

```sh
# Clear the buffer before a test run
lurpic logcat --clear

# Stream with a custom filter (verbose debug for your tag + errors only)
lurpic logcat --filter "MyTag:V *:E"
```

### `lurpic crash`

Pull native crash tombstones from the device and symbolicate them:

```sh
lurpic crash
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--serial` | auto | Target device serial. |
| `--build-dir` | `<project>/build` | Build directory containing `android/lib/<abi>/*.so`. |
| `--pull-dir` | temp dir | Local directory to pull tombstones into. |
| `--abi` | auto | Filter analysis to a single ABI (e.g. `arm64-v8a`). |

Workflow:

```sh
# Run the app on the emulator until it crashes
lurpic run android --emulator

# After the crash, analyse it
lurpic crash
```

The command:
1. Locates the `android/lib/<abi>/*.so` symbol files from the build directory.
2. Pulls `/data/tombstones` from the device.
3. Runs `ndk-stack -sym <symbol-dir> -dump <tombstone>` for each tombstone.
4. Falls back to scanning `logcat -d` for crash entries if no tombstones exist.

When debug symbols are explicitly retained via
`build/android/native-debug-symbols/<abi>/*.so`, those take precedence over
the (potentially stripped) lib copies.

---

## Symbol Bundle for Release Builds

Before a release build, ensure unstripped copies are retained:

1. Configure the build tool to emit `native-debug-symbols.zip` containing the
   unstripped `.so` set per ABI.
2. Upload this zip as a Play Console debug symbol artifact.

The `lurpic crash` command checks the build directory for both
`android/lib/<abi>` and `android/native-debug-symbols/<abi>`, preferring the
latter.

---

## Manual Workflow (without lurpic)

If the `lurpic` CLI is unavailable, the manual workflow is:

```sh
# 1. Pull tombstones
adb pull /data/tombstones ./tombstones

# 2. Symbolicate
ndk-stack -sym build/android/lib/arm64-v8a -dump ./tombstones/tombstone_00

# 3. Check logcat for Java crashes
adb logcat -d -v time AndroidRuntime:V *:E
```

---

## Expected Output

A successful `lurpic crash` invocation produces output like:

```
Device: emulator-5554
Build:  /home/user/project/build
Symbol sets:
  arm64-v8a:
    /home/user/project/build/android/lib/arm64-v8a/libgo.so
    /home/user/project/build/android/lib/arm64-v8a/liblurpic_render.so

Pulling tombstones to /tmp/lurpic-tombstones-abc123 ...

Found 1 tombstone(s):
  /tmp/lurpic-tombstones-abc123/tombstones/tombstone_00

────────────────────────────────────────────────────────────
Tombstone: /tmp/lurpic-tombstones-abc123/tombstones/tombstone_00
────────────────────────────────────────────────────────────
*** *** *** *** *** *** *** *** *** *** *** *** *** *** *** ***
ABI: 'arm64-v8a'
...
signal 11 (SIGSEGV), code 1 (SEGV_MAPERR), fault addr 0x0

Symbolicated stack trace:
  #00 pc 0000000000012345  libgo.so!crashyFunction+0x100
  #01 pc 0000000000012467  libgo.so!main.someInit+0x47
```

This identifies the crashing library, the signal, and the function name
+ offset, making the root cause immediately debuggable.
