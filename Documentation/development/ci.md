# CI

This repository uses a dedicated Android CI workflow in `.github/workflows/android-ci.yml`.

## What it runs

- `go test ./...` on every push and pull request.
- `go run ./cmd/lurpic validate demos` to run the shared marks packages and each demo module's validation suite.
- An Android build job against `test_apps/android_hello`.
- An emulator smoke job that installs the debug APK, launches `org.lurpic.hello`, and checks that the process stays alive long enough to capture logs.
- A log artifact from the smoke run, uploaded even if the smoke step fails.
- A frame-capture check that screenshots the launched app and validates the image remains near the expected launch state.
- A touch replay check that synthesizes tap and swipe input and validates the resulting touch log sequence.

## Local reproduction

From the repository root:

```bash
go test ./...
go run ./cmd/lurpic validate demos
cd test_apps/android_hello
go run ../../cmd/lurpic doctor android
go run ../../cmd/lurpic build android
```

If you want to reproduce the smoke test locally, install the APK on a connected device or emulator and launch:

```bash
adb install -r build/android/org.lurpic.hello-debug.apk
adb shell am start -n org.lurpic.hello/org.lurpicui.bridge.LurpicNativeActivity
```

## Required toolchain

- Go 1.25+
- Rust with `cargo-ndk`
- Android SDK platform 33
- Android NDK
- JDK 17+

## Notes

- The build job targets the sample app in `test_apps/android_hello`.
- The emulator smoke job uses API 33 so it exercises the current target SDK path.
- The smoke job writes `android-smoke/logcat.txt` and uploads it as the `android-smoke-logs` artifact.
- The frame check writes `android-smoke/launch.png` and validates it with `lurpic android-ci frame`.
- The replay check writes `android-smoke/replay-logcat.txt` and validates it with `lurpic android-ci replay`.
