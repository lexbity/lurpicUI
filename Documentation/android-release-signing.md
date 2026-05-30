# Android Release Signing Guide

## Overview

lurpicUI supports multiple signing schemes for Android APKs and AABs:

| Scheme | Purpose | Release | Debug |
|--------|---------|---------|-------|
| **v1** (JAR) | Legacy signing (pre-API 24) | ✓ | ✓ |
| **v2** (APK) | Full APK signing (API 24+) | ✓ | ✓ |
| **v3** (Key rotation) | Supports signing key rotation for Play App Signing | ✓ | — |
| **v4** (Incremental) | Enables incremental APK installs via .idsig file | ✓ | — |

---

## Quick Start

### 1. Generate a keystore (one-time)

```sh
keytool -genkey -v \
  -keystore release.keystore \
  -alias my-key \
  -keyalg RSA \
  -keysize 2048 \
  -validity 10000 \
  -dname "CN=Your Name,O=Your Org,C=US"
```

### 2. Configure lurpic.toml

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

### 3. Build a release AAB

```sh
# Password via env var (recommended for CI):
export LURPIC_KEYSTORE_PASSWORD=your-password
lurpic build android --release --aab

# Or prompt interactively:
lurpic build android --release --aab

# Or pass via flag (less secure — visible in ps):
lurpic build android --release --aab --ks-pass your-password
```

The output `.aab` is signed with v1+v2+v3+v4 schemes.

---

## Play App Signing

Play App Signing is Google's recommended approach where the app signing key is
managed by Google Play, and developers upload using an **upload key**.

### Setup

1. Generate an upload keystore (separate from the app signing key):
   ```sh
   keytool -genkey -v \
     -keystore upload.keystore \
     -alias upload-key \
     -keyalg RSA \
     -keysize 2048 \
     -validity 10000
   ```

2. Extract the upload certificate:
   ```sh
   keytool -exportcert \
     -keystore upload.keystore \
     -alias upload-key \
     -file upload_cert.pem \
     -rfc
   ```

3. In Google Play Console:
   - Go to Release → App Integrity
   - Choose "Let Google manage your app signing key"
   - Upload `upload_cert.pem` as the upload certificate

4. Configure lurpic.toml to use the upload keystore:
   ```toml
   [android.keystore]
   path = "upload.keystore"
   alias = "upload-key"
   ```

5. Build and upload the AAB to Play Console:
   ```sh
   LURPIC_KEYSTORE_PASSWORD=your-upload-password lurpic build android --release --aab
   ```

### Key rotation

The v3 signing scheme enables key rotation. When you need to change your
signing key, apksigner can generate a new key signed by the old key:

```sh
apksigner rotate \
  --out rotation_history \
  --old-signer --ks old.keystore --ks-key-alias old-key \
  --new-signer --ks new.keystore --ks-key-alias new-key
```

Then sign with the new key and include the rotation history.

---

## CI Signing

### GitHub Actions

```yaml
- name: Sign release AAB
  run: |
    echo "${{ secrets.KEYSTORE_BASE64 }}" | base64 -d > release.keystore
    LURPIC_KEYSTORE_PASSWORD="${{ secrets.KEYSTORE_PASSWORD }}" \
      lurpic build android --release --aab
```

### Password security

The password can be supplied via:

| Method | Security | Usage |
|--------|----------|-------|
| `LURPIC_KEYSTORE_PASSWORD` env var | Good — not in process args | CI secrets |
| Interactive prompt | Best — never stored | Local dev |
| `--ks-pass` flag | Poor — visible in `ps` | Avoid |
| `pass:file:<path>` | Excellent — file read by apksigner | CI with mounted secrets |

For maximum security in CI, write the password to a temp file and use
`pass:file:`:

```sh
echo -n "$KEYSTORE_PASSWORD" > /tmp/ks_pass.txt
lurpic build android --release --aab --ks-pass /tmp/ks_pass.txt
```

The `buildSignArgs` function detects absolute paths and automatically uses
`pass:file:` to prevent the secret from appearing in the process listing.

---

## Verification

After signing, verify the schemes are present:

```sh
# Check APK signing
apksigner verify --verbose my-app.apk

# Expected output includes:
#   Verified using v1 scheme (JAR signing): true
#   Verified using v2 scheme (APK Signature Scheme v2): true
#   Verified using v3 scheme (APK Signature Scheme v3): true
#   Verified using v4 scheme (APK Signature Scheme v4): true
#   Number of signers: 1

# Check AAB signing (bundletool)
bundletool validate --bundle my-app.aab
```

For release builds, `lurpic` automatically runs `apksigner verify` after signing.

---

## Debug Builds

Debug builds use the auto-generated debug keystore at
`~/.android/debug.keystore` (alias `androiddebugkey`, password `android`).
They are signed with v1+v2 only.

```sh
lurpic build android           # debug APK (v1+v2)
lurpic build android --aab     # debug AAB (v1+v2)
```

Debug AABs cannot be uploaded to Play Console — they will be rejected. Use
`--release` for Play submission.

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `keystore not found` | Check `android.keystore.path` in `lurpic.toml` |
| `password missing` | Set `LURPIC_KEYSTORE_PASSWORD` or use `--ks-pass` |
| `v3 signing not supported` | Update `build-tools` to version 30.0.0+ |
| `AAB rejected by Play` | Ensure you're using `--release` — debug AABs are rejected |
| `INVALID_SIGNATURE on upload` | Upload certificate must match the one in Play Console |
