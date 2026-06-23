# CLI Reference

This document covers the `lurpic` build tool and the `lurpiclint` analyzer as
implemented in the repository today.

## `lurpic`

Source: [`cmd/lurpic/main.go`](../cmd/lurpic/main.go)

### Commands

| Command | Purpose | Notes |
|---|---|---|
| `build android` | Build an Android APK or AAB. | Accepts `--release`, `--aab`, `--output`, `--sdk-path`, `--ndk-path`, and `--jdk-path`. |
| `run android` | Build, install, and run on Android. | Accepts `--emulator`, `--release`, `--device`, `--abi`, `--gpu`, `--force-software`, `--project`, and `--no-logcat`. |
| `doctor [android]` | Diagnose the Android toolchain. | `--verbose` prints detailed checks. |
| `validate demos` | Run the demo validation suites. | Uses the shared marks suites plus the demo app suites. |
| `logcat` | Stream or clear device logs. | Implemented in `cmd/lurpic/diagnostics.go` and dispatched from `cmd/lurpic/main.go`. |
| `crash` | Pull tombstones and symbolize crash dumps. | Implemented in `cmd/lurpic/diagnostics.go` and dispatched from `cmd/lurpic/main.go`. |
| `screenshot` | Capture device screenshots for golden testing. | Implemented in `cmd/lurpic/diagnostics.go` and dispatched from `cmd/lurpic/main.go`. |
| `clean` | Remove build artifacts. | No extra flags documented in the usage text. |
| `version` | Print version information. | Returns the binary version string. |
| `help` / `-h` / `--help` | Print the usage text. | Any unknown top-level command also prints usage and exits non-zero. |

### Build and Run Flags

- `--release` builds a release-signed artifact.
- `--aab` selects Android App Bundle output instead of APK.
- `--output PATH` sets the output path.
- `--sdk-path PATH` overrides Android SDK detection.
- `--ndk-path PATH` overrides Android NDK detection.
- `--jdk-path PATH` overrides JDK detection.
- `--emulator` starts or reuses an emulator before launch.
- `--force-software` forces the software renderer at runtime.
- `--project DIR` points the CLI at a project root containing `lurpic.toml`.
- `--abi ABI` limits Android packaging to one ABI.
- `--gpu MODE` selects emulator GPU mode.
- `--boot-timeout DURATION` sets the emulator boot timeout.
- `--device SERIAL` selects a specific connected device.
- `--avd NAME` selects a specific Android Virtual Device.
- `--no-logcat` suppresses logcat streaming after launch.

### Configuration Precedence

The binary documents this precedence in its usage text:

1. command-line flags
2. project `lurpic.toml`
3. user config
4. environment variables
5. auto-detection

### Environment Variables

`lurpic` documents these environment variables in its usage text:

- `ANDROID_HOME`
- `ANDROID_NDK_HOME`
- `JAVA_HOME`
- `LURPIC_KEYSTORE_PASSWORD`

## `lurpiclint`

Source: [`cmd/lurpiclint/main.go`](../cmd/lurpiclint/main.go)

### Subcommands

| Subcommand | Purpose | Output |
|---|---|---|
| `check` | Run the registered rules over one or more packages. | Text, JSON, or GitHub annotation output. |
| `capabilities` | Emit the uxauthoring capability index used by LL004. | Text or JSON. |
| `explain <rule-id>` | Print a rule rationale and fix guidance. | Text. |
| `version` | Print the analyzer version string. | Text. |
| `help` / `-h` / `--help` | Print usage. | Text. |

### `check` Flags

- `--format text|json|github`
- `--severity info|warn|error`
- `--fail-on info|warn|error`
- `--config PATH`
- `--baseline PATH`
- `--rules ID1,ID2,...`
- `--no-suggest`
- `--include-tests`
- `--root PATH`

### Exit Codes

`lurpiclint` documents these exit codes in its usage text:

- `0` no findings at or above `--fail-on`
- `1` findings at or above `--fail-on`
- `2` usage error
- `3` internal error

### Notes

- `check` defaults to `--format text`, `--severity warn`, and `--fail-on error`.
- `capabilities` defaults to text output.
- `explain` requires a rule ID argument.
- Behavior not explicitly covered above is documented as "present in source, not
  separately verified during this pass."
