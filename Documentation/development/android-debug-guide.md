# Android Debugging Guide

How to build, install, launch, and pull logs for a lurpicUI Android app
(example: `cmd/quick_square_app`, package `org.lurpicui.quicksquare`).

The `lurpic run android --emulator` command boots an emulator, builds,
installs, streams logcat, **and shuts the emulator down when it exits**
(including on a timeout or Ctrl-C). That auto-shutdown destroys the crash
state before you can inspect it. For debugging a crash, drive the emulator
and `adb` manually as below so the device stays up.

## 0. Environment

```bash
export PATH="$PATH:$HOME/Android/Sdk/platform-tools:$HOME/Android/Sdk/emulator"
adb version
emulator -list-avds            # e.g. lurpic_api33_google_apis_x86_64
```

App coordinates used throughout:

- Package:  `org.lurpicui.quicksquare`
- Activity: `org.lurpicui.bridge.LurpicNativeActivity`

## 1. Boot an emulator that persists

Start it detached so it survives across commands (do **not** use
`lurpic run --emulator`, which manages and kills it):

```bash
emulator -avd lurpic_api33_google_apis_x86_64 \
    -no-snapshot-save -no-audio -no-boot-anim &

# Wait until fully booted
adb wait-for-device
until [ "$(adb shell getprop sys.boot_completed | tr -d '\r')" = "1" ]; do sleep 2; done
adb devices            # should list emulator-5554  device
```

## 2. Build the APK (without touching the running emulator)

Use the `build` subcommand (no `--emulator`), so the tool only produces the
APK and leaves your device alone:

```bash
go run ./cmd/lurpic build android --project ./cmd/quick_square_app
```

Output APK:
`cmd/quick_square_app/build/android/org.lurpicui.quicksquare-debug.apk`

To cut through the Rust/cgo warning noise and see only real errors:

```bash
go run ./cmd/lurpic build android --project ./cmd/quick_square_app 2>&1 \
  | grep -vE "warning:|deprecated|note:|^\s*\||\^|has been explicitly|-->"
```

## 3. Install + launch

```bash
adb install -r cmd/quick_square_app/build/android/org.lurpicui.quicksquare-debug.apk
adb logcat -c          # clear the log buffers first
adb shell am start -W -n org.lurpicui.quicksquare/org.lurpicui.bridge.LurpicNativeActivity
```

`am start -W` blocks until the launch completes and prints `TotalTime` /
`WaitTime`. Check whether the process survived:

```bash
adb shell pidof org.lurpicui.quicksquare && echo RUNNING || echo "NOT RUNNING (crashed/exited)"
```

## 4. Pull crash logs

### a. App lifecycle + our own logs

The native bridge logs under tags `LurpicNativeActivity`, `LurpicBridge`,
`LurpicAudio`. This shows how far startup got:

```bash
adb logcat -d | grep -E "Lurpic" | sed 's/.*[0-9] [IWE] //'
```

A healthy startup prints: `Loaded Go shared library` → `onCreate` →
`ANativeActivity_onCreate` → `onStart` → `onResume` → `onInputQueueCreated`
→ `onNativeWindowCreated` → `onNativeWindowResized` → `onNativeWindowRedrawNeeded`.

### b. The Java crash (FATAL EXCEPTION)

```bash
adb logcat -d | grep -E "AndroidRuntime:" | sed 's/.*AndroidRuntime: //'
```

This gives the `java.lang.*` exception class and the Java stack. For a
`StackOverflowError`, count repeated frames to find a Java recursion:

```bash
adb logcat -d | grep "E AndroidRuntime" \
  | grep -oE "at [a-zA-Z0-9_.$]+\([A-Za-z0-9_.]+:[0-9-]+\)" \
  | sort | uniq -c | sort -rn | head
```

If the Java stack is **shallow** (only a handful of frames, bottoming out at
`Looper.loop`/`ActivityThread.main`) then the stack was exhausted by **native**
frames, not Java recursion — look at the native side (below).

### c. The native crash (tombstone)

Native SIGSEGV/SIGABRT crashes go to the dedicated `crash` buffer and the
`DEBUG` tag:

```bash
adb logcat -d -b crash | tail -80
```

Look for:
- `signal 11 (SIGSEGV)` / `signal 6 (SIGABRT)`
- `Abort message: '...'` — for ART aborts this contains the originating Java
  exception (e.g. `No pending exception expected: java.lang.StackOverflowError`).
- `backtrace:` — note this often shows the **abort/exception-handling** path
  (libart `Runtime::Abort`, `ThrowStackOverflowError`), not the frames that
  actually overflowed the stack.

### d. Full ART thread dump (all threads, interleaved native+Java)

On an ART abort the runtime dumps every thread under tag matching the process
name (`F <procname>: runtime.cc:...`). The `"main" prio=10 tid=1` section is
the most useful — it interleaves native and managed frames:

```bash
adb logcat -d > /tmp/lurpic_crash.log
awk '/"main" prio=10 tid=1/{f=1} f{print}' /tmp/lurpic_crash.log \
  | sed 's/.*runtime.cc:[0-9]*] //' | grep -E "native: #|at " | head -60
```

### e. Symbolicating native frames

The build keeps unstripped `.so` files under
`cmd/quick_square_app/build/android/lib/x86_64/`
(`libgo.so`, `liblurpic_render.so`). Pipe a raw tombstone through `ndk-stack`:

```bash
NDK=$HOME/Android/Sdk/ndk/30.0.14904198
adb logcat -d -b crash > /tmp/tombstone.txt
$NDK/ndk-stack -sym cmd/quick_square_app/build/android/lib/x86_64 -dump /tmp/tombstone.txt
```

Inspect what dynamic libs a `.so` pulls in (catches missing `-l...` link flags
that surface as `UnsatisfiedLinkError: cannot locate symbol ... at dlopen`):

```bash
$NDK/toolchains/llvm/prebuilt/linux-x86_64/bin/llvm-readelf -d \
  cmd/quick_square_app/build/android/lib/x86_64/libgo.so | grep NEEDED
```

## 5. Useful one-liners

```bash
# Live-follow only our logs while reproducing
adb logcat LurpicNativeActivity:V LurpicBridge:V LurpicAudio:V AndroidRuntime:E '*:S'

# Most recent native tombstone files on device
adb shell ls -t /data/tombstones/ | head
adb shell cat /data/tombstones/tombstone_00

# Uninstall (clean slate)
adb uninstall org.lurpicui.quicksquare

# Shut the emulator down when done
adb emu kill
```

## 6. Known gotchas (observed)

- **Auto-shutdown:** `lurpic run android --emulator` kills the emulator on
  exit. Use a manually-started emulator + `lurpic build` for crash debugging.
- **`UnsatisfiedLinkError: cannot locate symbol "SL_IID_*"` at dlopen of
  `libgo.so`:** the audio cgo package must link OpenSL ES and the log lib —
  `#cgo LDFLAGS: -lOpenSLES -llog`. A shared lib links fine with undefined
  symbols but fails at load time. Verify with `llvm-readelf -d ... | grep NEEDED`.
- **`zipalign` warning** during signing (`'-P <pagesize_kb>' and '-p' cannot be
  used in combination`) with build-tools 37.x: the APK is still produced
  (unaligned) and installs/runs on the emulator, but should be fixed in the
  packager before release.
