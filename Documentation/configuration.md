# Configuration

This document covers the verified `lurpic.toml` schema, the user-level config
file, and the environment variables the code actually reads.

## Configuration Sources

`lurpic` resolves settings in this order:

1. command-line flags
2. project `lurpic.toml`
3. user config
4. environment variables
5. auto-detection

The CLI prints that ordering in `cmd/lurpic/main.go`, and the Android toolchain
detector implements the same precedence in `cmd/lurpic/toolchain.go`.

## Project Config: `lurpic.toml`

### `app`

| Key | Type | Default | Notes |
|---|---|---|---|
| `app.id` | string | required | Android applicationId. Must contain at least two dot-separated segments and start each segment with a letter. |
| `app.name` | string | required | Display name. |
| `app.version` | string | `1.0.0` | Used to derive `android.version_code` when that field is omitted. |
| `app.icon` | string | empty | Optional icon path. |
| `app.main` | string | `.` | Go package to cross-compile, relative to the project root. Must not escape the root. |

### `android`

| Key | Type | Default | Notes |
|---|---|---|---|
| `android.min_sdk` | int | `24` | Minimum supported API level. Release builds require at least 21. |
| `android.target_sdk` | int | `36` | Used for packaging and release validation. Release builds require at least 35. |
| `android.version_code` | int | derived from `app.version` | Derived as `major*1_000_000 + minor*1_000 + patch` when omitted. |
| `android.abis` | list[string] | `["arm64-v8a"]` | Release builds must include `arm64-v8a`; `x86_64` is allowed for emulator use when explicitly added. |
| `android.permissions.required` | list[string] | empty | Required Android permissions. |
| `android.permissions.optional` | list[string] | empty | Optional Android permissions. |
| `android.keystore.path` | string | required for release | Release signing keystore path. |
| `android.keystore.alias` | string | required for release | Keystore alias. |
| `android.sdk.path` | string | empty | Project-level Android SDK override. |
| `android.sdk.version` | string | empty | Optional SDK version pin. |
| `android.ndk.path` | string | empty | Project-level Android NDK override. |
| `android.ndk.version` | string | empty | Optional NDK version pin. |
| `android.jdk.path` | string | empty | Project-level JDK override. |
| `android.jdk.version` | string | empty | Optional JDK version pin. |
| `android.assets.no_compress` | list[string] | `["*.pak"]` | Files stored uncompressed in the APK. |
| `android.assets.packs` | list | empty | Play Asset Delivery pack definitions. |
| `android.network_security_config` | string | empty | Optional XML resource path for Android network security config. |

### `assets`

| Key | Type | Default | Notes |
|---|---|---|---|
| `assets.residency_mode` | string | `auto` | Accepts `auto`, `cpu`, `cpuonly`, `gpu`, and `gpuresident`. |
| `assets.cpu_budget_mb` | int | `256` | CPU-side decoded LOD cache cap. |
| `assets.gpu_budget_mb` | int | `192` in `auto` mode | Must be non-negative; `0` disables GPU residency. |
| `assets.upload_budget_kb_frame` | int | `4096` | Per-frame GPU upload budget. |

## User Config

The user config file lives at:

- Linux: `~/.config/lurpic/config.toml`, or `$XDG_CONFIG_HOME/lurpic/config.toml`
- macOS: `~/Library/Application Support/lurpic/config.toml`
- Windows: `%APPDATA%\lurpic\config.toml`, falling back to `%LOCALAPPDATA%\lurpic\config.toml`

`cmd/lurpic/user_config.go` reads:

| Key | Type | Notes |
|---|---|---|
| `android.sdk-path` | string | User-level Android SDK override. |
| `android.ndk-path` | string | User-level Android NDK override. |
| `android.jdk-path` | string | User-level JDK override. |

## Environment Variables

The following environment variables are read by the code today.

| Name | Purpose |
|---|---|
| `ANDROID_HOME` | Android SDK path detection. |
| `ANDROID_SDK` | Alternate Android SDK path detection. |
| `ANDROID_NDK_HOME` | Android NDK path detection. |
| `NDK_HOME` | Alternate Android NDK path detection. |
| `ANDROID_AVD_NAME` | Emulator AVD override when no explicit `LURPIC_ANDROID_AVD` is set. |
| `LURPIC_ANDROID_AVD` | Emulator AVD override used by `lurpic run android --emulator`. |
| `JAVA_HOME` | JDK detection and `keytool` lookup. |
| `LURPIC_KEYSTORE_PASSWORD` | Release keystore password. |
| `LURPICUI_UPDATE_GOLDEN` | Enables golden-image regeneration in tests. |
| `TESTKIT_GOLDEN_DEBUG` | Enables extra golden-test diagnostics. |
| `SOURCE_DATE_EPOCH` | Deterministic ZIP timestamps for release packaging. |
| `HOME` | Used for managed AVD location and some test isolation paths. |
| `PATH` | Used for executable lookup when `java` or other tools are discovered from the shell path. |
| `XDG_CONFIG_HOME` | User config directory override. |
| `APPDATA` | Windows roaming user config directory. |
| `LOCALAPPDATA` | Windows fallback user config directory. |

## Notes

- `LURPIC_KEYSTORE_PASSWORD` is documented in `cmd/lurpic/main.go` and used by
  release signing in `cmd/lurpic/android_builder.go`.
- `SOURCE_DATE_EPOCH` is used only when release packaging writes ZIP timestamps.
- If a value is not called out above, the repository does not currently verify
  it as part of the config load path.
