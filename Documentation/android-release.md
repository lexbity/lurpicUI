# Android Release Guide

This document covers everything needed to submit a lurpicUI app to Google Play Store.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Building for Release](#building-for-release)
3. [Google Play Listing](#google-play-listing)
   - [Data Safety](#data-safety)
   - [Permissions](#permissions)
   - [Content Rating](#content-rating)
4. [App Signing](#app-signing)
5. [AAB Validation](#aab-validation)
6. [Pre-launch Report](#pre-launch-report)
7. [Troubleshooting](#troubleshooting)

---

## Prerequisites

- Android SDK, NDK, JDK installed (run `lurpic doctor android` to verify)
- Release keystore generated
- Google Play Console account
- `bundletool` installed (`sdkmanager "extras;google;bundletool"`)

## Building for Release

```sh
# Build a release AAB for Play Store submission
lurpic build android --release --aab

# Build a release APK for sideload testing
lurpic build android --release
```

### Build output

| Artifact | Path | Purpose |
|---|---|---|
| `build/android/*.aab` | Signed Android App Bundle | Play Store upload |
| `build/android/native-debug-symbols.zip` | Unstripped debug symbols | Play Console upload for crash symbolication |
| `build/android/*.apk` | Signed universal APK | Sideload testing |

### Configuration (`lurpic.toml`)

```toml
[app]
id = "com.example.app"
name = "My App"
version = "1.0.0"

[android]
min_sdk = 24
target_sdk = 36
abis = ["arm64-v8a", "x86_64"]

# Optional: pin toolchain versions for reproducible builds
[android.sdk]
version = "35"
[android.ndk]
version = "27.0.12077973"

[android.keystore]
path = "release.keystore"
alias = "my-key"

[android.permissions]
required = [
    "android.permission.INTERNET",
    "android.permission.POST_NOTIFICATIONS",
]
optional = [
    "android.permission.CAMERA",
    "android.permission.ACCESS_FINE_LOCATION",
]
```

---

## Google Play Listing

### Data Safety

Google Play requires a Data Safety section describing what data your app
collects and shares. Below is the mapping for lurpicUI framework features:

| Data type | Collected? | Shared? | Purpose | Required permission |
|---|---|---|---|---|
| **Location (coarse)** | Optional | No | User-facing features (maps, weather) | `ACCESS_COARSE_LOCATION` |
| **Location (precise)** | Optional | No | Navigation, proximity features | `ACCESS_FINE_LOCATION` |
| **Camera** | Optional | No | Camera features in app | `CAMERA` |
| **Photos / Media** | Optional | No | User picks files via SAF | `READ_EXTERNAL_STORAGE` (legacy) |
| **Audio recording** | Optional | No | Voice input, recording | `RECORD_AUDIO` |
| **Notifications** | Optional | N/A | App notifications | `POST_NOTIFICATIONS` |
| **Device ID** | No | No | — | — |
| **App diagnostics** | No | No | — | — |

**Framework defaults:** lurpicUI does **not** collect, share, or transmit any
user data by default. The above permissions are only required if your app
uses those specific features. You can omit optional permissions from
`lurpic.toml` if your app does not use them.

### Permissions

The generated manifest includes `<uses-permission>` entries for all
permissions listed in `[android.permissions]`:

- **Required permissions**: Always included in the manifest. The app requests
  these at runtime.
- **Optional permissions**: Included in the manifest. The app may request
  them conditionally.

Android 13+ notification permission (`POST_NOTIFICATIONS`) must be requested
at runtime before posting notifications. See Phase 20 for the permission
request API.

### Content Rating

Google Play assigns a content rating based on a questionnaire. The lurpicUI
framework does not enforce any content restrictions. The rating depends
entirely on your app's content.

---

## App Signing

Two signing flows are supported:

### 1. Direct signing (no Play App Signing)

```sh
lurpic build android --release --aab
```

The AAB is signed with your keystore using v1+v2+v3+v4 schemes.

### 2. Play App Signing (recommended)

See [android-release-signing.md](android-release-signing.md) for detailed
instructions on setting up Play App Signing with upload keys.

---

## AAB Validation

Before uploading to Play Console, validate the AAB:

```sh
# Validate AAB structure
bundletool validate --bundle build/android/com.example.app-release.aab

# Check signing
apksigner verify --verbose build/android/com.example.app-release.aab
```

Expected verification output:
```
Verified using v1 scheme (JAR signing): true
Verified using v2 scheme (APK Signature Scheme v2): true
Verified using v3 scheme (APK Signature Scheme v3): true
Verified using v4 scheme (APK Signature Scheme v4): true
Number of signers: 1
```

---

## Pre-launch Report

Google Play's pre-launch report tests your app on a range of devices. To
prepare:

1. **Build a release AAB**: `lurpic build android --release --aab`
2. **Upload to Play Console**: Internal testing → Create new release → Upload AAB
3. **Review pre-launch report**: Check for crashes, ANRs, and compatibility issues

### Common pre-launch issues

| Issue | Resolution |
|---|---|
| `extractNativeLibs="true"` warning | Already set to `false` by default — OK |
| Missing `dataExtractionRules` | Already included by default |
| Unused permissions | Remove unused permissions from `lurpic.toml` |
| Missing privacy policy | Add a privacy policy link in Play Console |
| `targetSdk < 35` | Already set to 36 by default |

---

## Troubleshooting

| Problem | Solution |
|---|---|
| `bundletool validate` fails | Ensure `bundletool.jar` is installed: `sdkmanager "extras;google;bundletool"` |
| `apksigner verify` shows no v3/v4 | Run with `--release` flag to enable v3+v4 signing |
| Play Console rejects AAB as "invalid" | Run `bundletool validate` first, check the output |
| Pre-launch report shows crash | Check the crash stack trace and symbolicate with `lurpic crash` |
| INVALID_SIGNATURE error | Verify your upload certificate matches Play Console |
