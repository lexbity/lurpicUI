# Lurpic Studio — Full Marks Showcase Demo

## Quick Start

```bash
go run ./demos/lurpic_studio
```

Opens an 1280×800 window (software renderer) rendering a themed dashboard
with data sources, chart canvas, inspector controls, and status bar.

## Running Tests

```bash
go test ./demos/lurpic_studio/...
```

## Android Build

Requires the `lurpic` CLI tool and Android NDK (API 29+).

```bash
# Build APK from the demo's lurpic.toml
lurpic build android ./demos/lurpic_studio/lurpic.toml

# Install on connected device/emulator
adb install -r ./build/lurpic_studio.apk

# Run
adb shell am start -n org.lurpicui.lurpicstudio/android.app.NativeActivity
```

The software renderer backend is used on Android (automatic fallback in
`app/run.go` when Vulkan is unavailable). The bundled `assets/metrics.csv`
is loaded via `app.Asset()` which resolves from APK assets on Android.

## Cross-Compile Verification

```bash
GOOS=android GOARCH=arm64 go build ./demos/lurpic_studio/...
```

## Phased Build

See the spec document for the full 17-phase plan. This is currently at Phase 14:

- [x] Phase 0 — API re-verification & scaffolding
- [x] Phase 1 — Dataset pipeline
- [x] Phase 2 — State topology
- [x] Phase 3 — Root facet & responsive skeleton
- [x] Phase 4 — Primitive & structure substrate
- [x] Phase 5 — Top chrome (action family pt.1)
- [x] Phase 6 — Left sources panel
- [x] Phase 7 — Center Data tab (table + pagination + tabs)
- [x] Phase 8 — Chart canvas (viz family + scales + data binding)
- [x] Phase 9 — Inspector (input + selection families), static
- [x] Phase 10 — Live wiring (inspector → state → chart/table)
- [x] Phase 11 — Overlays & feedback family
- [x] Phase 12 — Status bar & simulated async job
- [x] Phase 13 — Responsive collapse (drawers + bottom sheet)
- [x] Phase 14 — Android packaging & verification
- [x] Phase 15 — Polish: theme, focus traversal, accessibility, tooltips
- [x] Phase 16 — Coverage audit, golden harness, QA checklist
