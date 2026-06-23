# Android Release and Signing

Canonical release guide for Android APK and AAB builds.

## Prerequisites

- Android SDK, NDK, and JDK installed.
- `lurpic doctor android` passes for the local toolchain.
- A release keystore and alias are configured in `lurpic.toml` or supplied
  with command-line flags.
- `bundletool` is available for AAB validation.

## Build Commands

```sh
lurpic build android --release
lurpic build android --release --aab
```

The build command also supports these release-relevant overrides:

| Flag | Purpose |
|---|---|
| `--keystore PATH` | Override `android.keystore.path`. |
| `--ks-alias ALIAS` | Override `android.keystore.alias`. |
| `--ks-pass VALUE` | Provide the release keystore password directly. |
| `--output PATH` / `-o PATH` | Override the output artifact path. |
| `--abi ABI` | Restrict the build to one ABI. |
| `--project DIR` | Point at a specific project root. |
| `--sdk-path`, `--ndk-path`, `--jdk-path` | Override toolchain detection. |

## Build Behavior

- Release APKs and AABs are signed with v1, v2, v3, and v4 schemes.
- Debug builds use v1 and v2 only.
- Release signing requires `android.keystore.path`, `android.keystore.alias`,
  and a password.
- Password lookup order is `--ks-pass`, then `LURPIC_KEYSTORE_PASSWORD`, then
  interactive prompt.
- Release builds emit `build/android/native-debug-symbols.zip` alongside the
  APK or AAB so `lurpic crash` can symbolize release crashes.
- Zip timestamps are derived from `SOURCE_DATE_EPOCH` when it is set.

## `lurpic.toml`

Minimal release signing config:

```toml
[app]
id = "com.example.app"
name = "My App"

[android]
min_sdk = 24
target_sdk = 36

[android.keystore]
path = "release.keystore"
alias = "my-key"
```

Toolchain paths can be set in project config or user config. `cmd/lurpic`
resolves them in this order: command-line flag, project config, user config,
environment, then auto-detection.

## Signing Flows

### Direct signing

Use your app signing keystore directly:

```sh
LURPIC_KEYSTORE_PASSWORD=secret lurpic build android --release --aab
```

This is the simplest path when you control the release keystore.

### Play App Signing

This repository does not automate Play Console enrollment. Keep the Play-side
upload certificate workflow outside the repo, and submit the signed AAB
produced by `lurpic build android --release --aab`.

The release build enables the v3 signing scheme, which is the code-backed piece
needed for signing key rotation.

### CI signing

`cmd/lurpic` accepts the password via environment variable, which keeps it out
of the process arguments:

```yaml
- name: Build release bundle
  env:
    LURPIC_KEYSTORE_PASSWORD: ${{ secrets.KEYSTORE_PASSWORD }}
  run: lurpic build android --release --aab
```

If `--ks-pass` is an absolute path, the signer treats it as a file-backed
secret and uses `pass:file:` internally.

## Verification

Validate the signed artifact after the build:

```sh
apksigner verify --verbose build/android/com.example.app-release.apk
bundletool validate --bundle build/android/com.example.app-release.aab
```

`lurpic` also runs `apksigner verify` after signing a release build.

## Outputs

| Artifact | Path |
|---|---|
| Release APK | `build/android/<app-id>-release.apk` |
| Release AAB | `build/android/<app-id>-release.aab` |
| Release debug symbols | `build/android/native-debug-symbols.zip` |

## Troubleshooting

| Problem | What to check |
|---|---|
| `keystore not found` | `android.keystore.path` or `--keystore`. |
| `release signing requires keystore password` | `--ks-pass`, `LURPIC_KEYSTORE_PASSWORD`, or stdin prompt. |
| `bundletool validate` fails | Confirm the `bundletool` installation and bundle path. |
| `INVALID_SIGNATURE` on upload | Verify the Play Console upload certificate matches the keystore in use. |
| Missing `v3` or `v4` verification | Confirm the build used `--release`. |

## Related Commands

- `lurpic doctor android --verbose` to inspect SDK, NDK, and JDK detection.
- `lurpic crash` to symbolize release crashes with the emitted debug symbols.
