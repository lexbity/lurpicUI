# Android Release Signing

This document describes how to configure and use release signing for lurpicUI Android applications.

## Debug vs Release Builds

### Debug Builds (Default)

Debug builds are automatically signed with a debug keystore that the build tool generates on first use:

```bash
lurpic build android
```

The debug keystore is stored at `~/.android/debug.keystore` and uses:
- **Alias**: `androiddebugkey`
- **Password**: `android`

### Release Builds

Release builds require a valid release keystore:

```bash
lurpic build android --release
```

## Creating a Release Keystore

### Using keytool

Create a new keystore using Java's keytool:

```bash
keytool -genkey -v \
    -keystore release.keystore \
    -alias myapp \
    -keyalg RSA \
    -keysize 2048 \
    -validity 10000
```

You will be prompted for:
- Keystore password
- Key password (can be same as keystore)
- Certificate information (name, organization, etc.)

### Using Android Studio

You can also create keystores through Android Studio:
1. Build → Generate Signed Bundle / APK
2. Create new keystore
3. Fill in the required fields

## Configuring Release Signing

### Method 1: lurpic.toml (Recommended)

Add the keystore configuration to your `lurpic.toml`:

```toml
[app]
id = "com.example.myapp"
name = "My App"
version = "1.0.0"

[android]
min_sdk = 29
target_sdk = 33

[android.keystore]
path = "release.keystore"
alias = "myapp"
password = "your-password-here"
```

**Security Note**: Storing passwords in `lurpic.toml` is convenient but not secure. Consider using environment variables for CI/CD pipelines.

### Method 2: Environment Variable

Set the keystore password via environment variable:

```bash
export LURPIC_KEYSTORE_PASSWORD="your-password"
lurpic build android --release
```

This allows you to omit the password from `lurpic.toml`:

```toml
[android.keystore]
path = "release.keystore"
alias = "myapp"
# Password is set via LURPIC_KEYSTORE_PASSWORD
```

### Method 3: Command Line Flags

Override configuration at build time:

```bash
lurpic build android --release \
    --keystore /path/to/keystore.jks \
    --ks-alias myapp \
    --ks-pass "your-password"
```

## Build Verification

### APK Verification

After building, verify the APK signature:

```bash
# Using apksigner (from Android SDK)
apksigner verify --verbose myapp-release.apk

# Expected output includes:
# - Verifies with v1/v2/v3 scheme
# - Certificate information
# - Signature is valid
```

### Debug vs Release Verification

**Debug APK**:
```bash
$ apksigner verify -v myapp-debug.apk
Verifies
Verified using v1 scheme (JAR signing): true
Verified using v2 scheme (APK Signature Scheme v2): true
Certificate DN: C=US, O=Android, CN=Android Debug
```

**Release APK**:
```bash
$ apksigner verify -v myapp-release.apk
Verifies
Verified using v1 scheme (JAR signing): true
Verified using v2 scheme (APK Signature Scheme v2): true
Certificate DN: CN=Your Name, O=Your Organization, C=US
```

## APK Alignment

The build tool automatically aligns APKs using `zipalign` before signing. This is required for Google Play Store submission.

Alignment ensures uncompressed data starts on specific byte boundaries (4-byte alignment), reducing RAM usage when the APK is loaded.

## Complete Release Build Example

1. **Create keystore** (one-time):
   ```bash
   keytool -genkey -v -keystore release.keystore -alias myapp -keyalg RSA -validity 10000
   ```

2. **Configure lurpic.toml**:
   ```toml
   [app]
   id = "com.example.myapp"
   name = "My App"
   version = "1.0.0"

   [android.keystore]
   path = "release.keystore"
   alias = "myapp"
   ```

3. **Build with environment variable**:
   ```bash
   export LURPIC_KEYSTORE_PASSWORD="your-password"
   lurpic build android --release
   ```

4. **Verify the APK**:
   ```bash
   apksigner verify --verbose build/android/com.example.myapp-release.apk
   ```

## Troubleshooting

### "keystore not found"

```
Error: keystore not found at /path/to/keystore.jks
```

- Check that the path in `lurpic.toml` is correct
- Use absolute path if relative path doesn't work
- Ensure the keystore file exists and is readable

### "release signing requires keystore password"

```
Error: release signing requires keystore password. Set in lurpic.toml, use --ks-pass flag, or set LURPIC_KEYSTORE_PASSWORD environment variable
```

You must provide the keystore password via one of:
- `lurpic.toml`: `[android.keystore] password = "..."`
- Command line: `--ks-pass "your-password"`
- Environment: `LURPIC_KEYSTORE_PASSWORD="your-password"`

### APK verification fails

```
Warning: APK verification failed: ...
```

This warning doesn't necessarily mean the APK is broken. Check:
1. Is `apksigner` available in your Android SDK?
2. Are all build tools up to date?
3. Try manually verifying: `apksigner verify -v your.apk`

### zipalign fails

```
Warning: zipalign failed, proceeding with unaligned APK
```

While the build will succeed, you should fix zipalign for Play Store submission:
- Ensure `zipalign` is in your Android SDK build-tools
- Update build-tools: `sdkmanager "build-tools;33.0.0"`

## Security Best Practices

1. **Keep your keystore secure** - Anyone with your keystore can sign apps as you
2. **Use strong passwords** - For both keystore and key
3. **Backup your keystore** - Losing it means you can't update your app
4. **Don't commit passwords** - Use environment variables in CI/CD
5. **Different keystores** - Use different keystores for different apps
6. **Rotate keys periodically** - For new app versions

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Build Release APK

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Setup Android SDK
        uses: android-actions/setup-android@v2

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.22'

      - name: Decode keystore
        run: |
          echo "${{ secrets.RELEASE_KEYSTORE }}" | base64 -d > release.keystore

      - name: Build release APK
        env:
          LURPIC_KEYSTORE_PASSWORD: ${{ secrets.KEYSTORE_PASSWORD }}
        run: |
          go run ./cmd/lurpic build android --release

      - name: Upload APK
        uses: actions/upload-artifact@v3
        with:
          name: release-apk
          path: build/android/*.apk
```

Store your keystore base64-encoded in GitHub Secrets:
```bash
base64 -i release.keystore | pbcopy  # Copy to clipboard, paste in GitHub Secrets
```
