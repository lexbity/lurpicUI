package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"codeburg.org/lexbit/lurpicui/platform/android"
)

// androidBuilder orchestrates the Android APK build process
type androidBuilder struct {
	runner      Runner
	sdk         string
	ndk         string
	projectRoot string
	buildDir    string
	config      *Config
	release     bool
	outputPath  string
	apiLevel    int
}

// build executes the full Android build pipeline
func (b *androidBuilder) build() error {
	fmt.Println("=== Building Android APK ===")

	if err := b.selectAndroidAPI(); err != nil {
		return err
	}

	// Step 1: Build Go shared library for each configured ABI
	for _, abi := range b.config.Android.ABIs {
		arch, ok := ArchitectureByABI(abi)
		if !ok {
			return fmt.Errorf("unsupported ABI: %s", abi)
		}
		if err := b.buildGoLibrary(arch); err != nil {
			return fmt.Errorf("Go build failed for %s: %w", abi, err)
		}
	}

	// Step 2: Build Rust library
	if err := b.buildRustLibrary(); err != nil {
		return fmt.Errorf("Rust build failed: %w", err)
	}

	// Step 3: Generate AndroidManifest.xml
	if err := b.generateManifest(); err != nil {
		return fmt.Errorf("manifest generation failed: %w", err)
	}

	// Step 4: Bundle assets
	if err := b.bundleAssets(); err != nil {
		return fmt.Errorf("asset bundling failed: %w", err)
	}

	// Step 5: Assemble APK
	if err := b.assembleAPK(); err != nil {
		return fmt.Errorf("APK assembly failed: %w", err)
	}

	// Step 6: Sign APK
	if err := b.signAPK(); err != nil {
		return fmt.Errorf("APK signing failed: %w", err)
	}

	return nil
}

func (b *androidBuilder) selectAndroidAPI() error {
	impl, ok := android.SelectImplementation(b.config.Android.TargetSDK)
	if !ok || impl == nil {
		return fmt.Errorf("no Android API implementation registered for target SDK %d", b.config.Android.TargetSDK)
	}
	b.apiLevel = impl.APILevel()
	fmt.Printf("Selected Android API level %d for target SDK %d\n", b.apiLevel, b.config.Android.TargetSDK)
	return nil
}

// buildGoLibrary cross-compiles the Go code for Android for the given architecture.
func (b *androidBuilder) buildGoLibrary(arch Architecture) error {
	fmt.Printf("Building Go library for %s...\n", arch.ABI)

	// Find NDK compiler using the triple+api-level clang name
	clang := b.findNDKCompiler(arch)
	if clang == "" {
		return fmt.Errorf("cannot find NDK clang compiler for %s", arch.ABI)
	}

	// Create output directory for native libs
	libDir := filepath.Join(b.buildDir, "lib", arch.ABI)
	if err := os.MkdirAll(libDir, 0755); err != nil {
		return err
	}

	// Set up environment for cross-compilation
	env := os.Environ()
	env = append(env,
		"GOOS=android",
		"GOARCH="+arch.GOARCH,
		"CGO_ENABLED=1",
		fmt.Sprintf("CC=%s", clang),
	)
	if arch.GOARM != "" {
		env = append(env, "GOARM="+arch.GOARM)
	}

	// Find the main package
	mainPath := filepath.Join(b.projectRoot, "main.go")
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		mainPath = filepath.Join(b.projectRoot, "cmd", b.config.App.ID, "main.go")
	}

	// Missing main.go is a fatal error
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		return fmt.Errorf("no main.go found at %s or cmd/%s/main.go", filepath.Join(b.projectRoot, "main.go"), b.config.App.ID)
	}

	output := filepath.Join(libDir, "libgo.so")
	args := []string{
		"build",
		"-buildmode=c-shared",
		"-o", output,
	}

	// If main.go is in cmd/subdir, we need to build that package
	if filepath.Base(filepath.Dir(mainPath)) != b.projectRoot {
		args = append(args, filepath.Join("cmd", b.config.App.ID))
	} else {
		args = append(args, ".")
	}

	if err := b.runner.Run(CommandSpec{
		Path:   "go",
		Args:   args,
		Dir:    b.projectRoot,
		Env:    env,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	fmt.Printf("  Built: %s\n", output)
	return nil
}

// buildRustLibrary cross-compiles the Rust code for Android
func (b *androidBuilder) buildRustLibrary() error {
	fmt.Println("Building Rust library...")

	// Check if there's a Rust crate to build
	cargoToml := filepath.Join(b.projectRoot, "Cargo.toml")
	if _, err := os.Stat(cargoToml); os.IsNotExist(err) {
		// Check for crates/ subdirectory
		cratesDir := filepath.Join(b.projectRoot, "crates")
		if entries, err := os.ReadDir(cratesDir); err == nil && len(entries) > 0 {
			// Build each crate
			for _, entry := range entries {
				if entry.IsDir() {
					cratePath := filepath.Join(cratesDir, entry.Name())
					if _, err := os.Stat(filepath.Join(cratePath, "Cargo.toml")); err == nil {
						if err := b.buildRustCrate(cratePath, entry.Name()); err != nil {
							return err
						}
					}
				}
			}
			return nil
		}
		fmt.Println("  No Cargo.toml found, skipping Rust build")
		return nil
	}

	return b.buildRustCrate(b.projectRoot, "main")
}

// buildRustCrate builds a single Rust crate for Android
func (b *androidBuilder) buildRustCrate(cratePath, name string) error {
	libDir := filepath.Join(b.buildDir, "lib", "arm64-v8a")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		return err
	}

	// Set up NDK environment for cargo
	target := "aarch64-linux-android"
	env := os.Environ()

	// Find NDK toolchain
	toolchain := b.findNDKToolchain(target)
	if toolchain != "" {
		env = append(env,
			fmt.Sprintf("CC_%s=%s", target, filepath.Join(toolchain, "clang")),
			fmt.Sprintf("CXX_%s=%s", target, filepath.Join(toolchain, "clang++")),
			fmt.Sprintf("AR_%s=%s", target, filepath.Join(toolchain, "llvm-ar")),
		)
	}

	// Use cargo-ndk if available, otherwise manual configuration
	cargoNdk, err := b.runner.Look("cargo-ndk")
	if err == nil {
		if err := b.runner.Run(CommandSpec{
			Path:   cargoNdk,
			Args:   []string{"-t", "arm64-v8a", "build", "--release"},
			Dir:    cratePath,
			Env:    env,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}); err != nil {
			return fmt.Errorf("cargo-ndk build failed: %w", err)
		}
	} else {
		// Manual cargo build with target
		if err := b.runner.Run(CommandSpec{
			Path:   "cargo",
			Args:   []string{"build", "--release", "--target", target},
			Dir:    cratePath,
			Env:    env,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}); err != nil {
			return fmt.Errorf("cargo build failed: %w", err)
		}
	}

	// Copy the resulting library to libDir
	// TODO: Find and copy the actual built .so file
	fmt.Printf("  Built Rust crate: %s\n", name)
	return nil
}

// ManifestData contains data for the manifest template
type ManifestData struct {
	Package            string
	VersionCode        int
	VersionName        string
	MinSDK             int
	TargetSDK          int
	Permissions        []string
	AppName            string
	HasIcon            bool
	UsesLurpicActivity bool
}

const manifestTemplate = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="{{.Package}}"
    android:versionCode="{{.VersionCode}}"
    android:versionName="{{.VersionName}}">

    <uses-sdk android:minSdkVersion="{{.MinSDK}}" android:targetSdkVersion="{{.TargetSDK}}" />
{{range .Permissions}}
    <uses-permission android:name="{{.}}" />{{end}}

    <application
        android:label="{{.AppName}}"
        android:hasCode="true"
        android:extractNativeLibs="true"
        {{if .HasIcon}}android:icon="@mipmap/ic_launcher"{{end}}>
        <activity android:name="org.lurpicui.bridge.LurpicNativeActivity"
            android:exported="true"
            android:configChanges="orientation|screenSize|smallestScreenSize|density|keyboard|keyboardHidden">
            <meta-data android:name="android.app.lib_name" android:value="go" />
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>`

// generateManifest creates the AndroidManifest.xml using a template
func (b *androidBuilder) generateManifest() error {
	fmt.Println("Generating AndroidManifest.xml...")

	// Collect all permissions
	var permissions []string
	permissions = append(permissions, b.config.Android.Permissions.Required...)
	permissions = append(permissions, b.config.Android.Permissions.Optional...)

	// Parse version code from version string (e.g., "1.2.3" -> 1)
	versionCode := 1
	parts := strings.Split(b.config.App.Version, ".")
	if len(parts) > 0 {
		fmt.Sscanf(parts[0], "%d", &versionCode)
	}

	data := ManifestData{
		Package:            b.config.App.ID,
		VersionCode:        versionCode,
		VersionName:        b.config.App.Version,
		MinSDK:             b.config.Android.MinSDK,
		TargetSDK:          b.config.Android.TargetSDK,
		Permissions:        permissions,
		AppName:            b.config.App.Name,
		HasIcon:            b.config.App.HasIcon(),
		UsesLurpicActivity: true,
	}

	tmpl, err := template.New("manifest").Parse(manifestTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse manifest template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute manifest template: %w", err)
	}

	manifestPath := filepath.Join(b.buildDir, "AndroidManifest.xml")
	if err := os.WriteFile(manifestPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	fmt.Printf("  Generated: %s\n", manifestPath)
	return nil
}

// bundleAssets copies project assets into the build directory
func (b *androidBuilder) bundleAssets() error {
	fmt.Println("Bundling assets...")

	assetsDir := filepath.Join(b.projectRoot, "assets")
	if _, err := os.Stat(assetsDir); os.IsNotExist(err) {
		fmt.Println("  No assets directory found, skipping")
		return nil
	}

	// Copy assets to build directory
	destDir := filepath.Join(b.buildDir, "assets")
	if err := copyDir(assetsDir, destDir); err != nil {
		return fmt.Errorf("failed to copy assets: %w", err)
	}

	fmt.Printf("  Bundled assets to: %s\n", destDir)
	return nil
}

// compileResources uses aapt2 to compile resources and link the APK
func (b *androidBuilder) compileResources() (string, error) {
	fmt.Println("Compiling resources with aapt2...")

	aapt2, err := findSDKTool(b.sdk, "aapt2")
	if err != nil {
		return "", fmt.Errorf("aapt2 not found: %w", err)
	}

	compiledResDir := filepath.Join(b.buildDir, "resCompiled")
	os.MkdirAll(compiledResDir, 0755)

	// Compile icons if present
	if b.config.App.HasIcon() {
		if err := b.compileIcons(aapt2, compiledResDir); err != nil {
			fmt.Printf("  Warning: icon compilation failed: %v\n", err)
		}
	}

	// Link resources and create base APK
	baseApk := filepath.Join(b.buildDir, "base.apk")
	manifestPath := filepath.Join(b.buildDir, "AndroidManifest.xml")

	// Find android.jar for the target SDK
	androidJar := filepath.Join(b.sdk, "platforms", fmt.Sprintf("android-%d", b.config.Android.TargetSDK), "android.jar")
	if _, err := os.Stat(androidJar); err != nil {
		// Fall back to any available android.jar
		androidJar = filepath.Join(b.sdk, "platforms", fmt.Sprintf("android-%d", b.config.Android.MinSDK), "android.jar")
	}

	linkArgs := []string{
		"link",
		"-o", baseApk,
		"-I", androidJar,
		"--manifest", manifestPath,
		"--auto-add-overlay",
	}

	// Add compiled resources if any exist
	entries, _ := os.ReadDir(compiledResDir)
	if len(entries) > 0 {
		linkArgs = append(linkArgs, "-R")
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".flat") {
				linkArgs = append(linkArgs, filepath.Join(compiledResDir, entry.Name()))
			}
		}
	}

	if err := b.runner.Run(CommandSpec{
		Path:   aapt2,
		Args:   linkArgs,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return "", fmt.Errorf("aapt2 link failed: %w", err)
	}

	fmt.Printf("  Compiled resources: %s\n", baseApk)
	return baseApk, nil
}

// compileIcons compiles icon resources using aapt2
func (b *androidBuilder) compileIcons(aapt2, compiledResDir string) error {
	iconPath := b.config.App.Icon

	// Check if icon path is absolute or relative
	if !filepath.IsAbs(iconPath) {
		iconPath = filepath.Join(b.projectRoot, iconPath)
	}

	if _, err := os.Stat(iconPath); err != nil {
		return fmt.Errorf("icon not found at %s: %w", iconPath, err)
	}

	// Create mipmap directories and copy icons
	mipmapDir := filepath.Join(b.buildDir, "res", "mipmap-anydpi-v26")
	os.MkdirAll(mipmapDir, 0755)

	// For now, copy the icon as ic_launcher.png
	// In a full implementation, we'd generate multiple densities
	destIcon := filepath.Join(mipmapDir, "ic_launcher.png")
	if err := copyFile(iconPath, destIcon); err != nil {
		return err
	}

	// Compile the resource
	return b.runner.Run(CommandSpec{
		Path:   aapt2,
		Args:   []string{"compile", "-o", compiledResDir, destIcon},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
}

// assembleAPK creates the final APK by combining base APK with native libs and assets
func (b *androidBuilder) assembleAPK() error {
	fmt.Println("Assembling APK...")

	// Step 1: Compile resources and get base APK
	baseApk, err := b.compileResources()
	if err != nil {
		// Fall back to manual assembly if aapt2 fails
		fmt.Printf("  Resource compilation failed, falling back to manual assembly: %v\n", err)
		return b.assembleAPKManual()
	}

	unsignedApk := filepath.Join(b.buildDir, "unsigned.apk")

	// Step 2: Copy base APK to unsigned APK
	if err := copyFile(baseApk, unsignedApk); err != nil {
		return fmt.Errorf("failed to copy base APK: %w", err)
	}

	// Step 3: Add native libraries to APK
	libSrc := filepath.Join(b.buildDir, "lib")
	if _, err := os.Stat(libSrc); err == nil {
		if err := b.addToApk(unsignedApk, libSrc, "lib"); err != nil {
			return fmt.Errorf("failed to add native libs: %w", err)
		}
	}

	// Step 4: Add assets to APK
	assetsSrc := filepath.Join(b.buildDir, "assets")
	if _, err := os.Stat(assetsSrc); err == nil {
		if err := b.addToApk(unsignedApk, assetsSrc, "assets"); err != nil {
			return fmt.Errorf("failed to add assets: %w", err)
		}
	}

	fmt.Printf("  Assembled: %s\n", unsignedApk)
	return nil
}

// assembleAPKManual creates APK manually without aapt2 (fallback)
func (b *androidBuilder) assembleAPKManual() error {
	unsignedApk := filepath.Join(b.buildDir, "unsigned.apk")

	apkDir := filepath.Join(b.buildDir, "apk-staging")
	os.RemoveAll(apkDir)
	os.MkdirAll(apkDir, 0755)

	// Copy AndroidManifest.xml
	manifestSrc := filepath.Join(b.buildDir, "AndroidManifest.xml")
	copyFile(manifestSrc, filepath.Join(apkDir, "AndroidManifest.xml"))

	// Copy native libraries
	libSrc := filepath.Join(b.buildDir, "lib")
	if _, err := os.Stat(libSrc); err == nil {
		copyDir(libSrc, filepath.Join(apkDir, "lib"))
	}

	// Copy assets
	assetsSrc := filepath.Join(b.buildDir, "assets")
	if _, err := os.Stat(assetsSrc); err == nil {
		copyDir(assetsSrc, filepath.Join(apkDir, "assets"))
	}

	// Create APK zip
	os.Remove(unsignedApk)
	if err := b.runner.Run(CommandSpec{
		Path:   "zip",
		Args:   []string{"-r", unsignedApk, "."},
		Dir:    apkDir,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("failed to create APK: %w", err)
	}

	fmt.Printf("  Assembled: %s\n", unsignedApk)
	return nil
}

// addToApk adds files to an existing APK using zip
func (b *androidBuilder) addToApk(apkPath, srcDir, destPath string) error {
	return b.runner.Run(CommandSpec{
		Path:   "zip",
		Args:   []string{"-r", apkPath, destPath},
		Dir:    b.buildDir,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
}

// signAPK signs the APK with debug or release keystore
func (b *androidBuilder) signAPK() error {
	fmt.Println("Signing APK...")

	unsignedApk := filepath.Join(b.buildDir, "unsigned.apk")
	if _, err := os.Stat(unsignedApk); os.IsNotExist(err) {
		return fmt.Errorf("unsigned APK not found")
	}

	// Find apksigner
	apksigner, err := findSDKTool(b.sdk, "apksigner")
	if err != nil {
		return fmt.Errorf("apksigner not found: %w", err)
	}

	// Step 1: Align the APK (required before signing)
	alignedApk := filepath.Join(b.buildDir, "aligned.apk")
	if err := b.alignAPK(unsignedApk, alignedApk); err != nil {
		fmt.Printf("  Warning: zipalign failed, proceeding with unaligned APK: %v\n", err)
		alignedApk = unsignedApk
	}

	// Step 2: Sign the APK
	var keystorePath, keystoreAlias, keystorePass string

	if b.release {
		// Release signing - requires keystore configuration
		keystorePath = b.config.Android.Keystore.Path
		keystoreAlias = b.config.Android.Keystore.Alias
		keystorePass = b.getKeystorePassword()

		if keystorePath == "" {
			return fmt.Errorf("release signing requires keystore.path in lurpic.toml or --keystore flag")
		}
		if keystoreAlias == "" {
			return fmt.Errorf("release signing requires keystore.alias in lurpic.toml or --ks-alias flag")
		}
		if keystorePass == "" {
			return fmt.Errorf("release signing requires keystore password. Set in lurpic.toml, use --ks-pass flag, or set LURPIC_KEYSTORE_PASSWORD environment variable")
		}

		// Validate keystore exists
		if _, err := os.Stat(keystorePath); err != nil {
			return fmt.Errorf("keystore not found at %s: %w", keystorePath, err)
		}

		fmt.Printf("  Using release keystore: %s (alias: %s)\n", keystorePath, keystoreAlias)
	} else {
		// Debug signing
		keystorePath = b.getDebugKeystore()
		keystoreAlias = "androiddebugkey"
		keystorePass = "android"
		fmt.Printf("  Using debug keystore: %s\n", keystorePath)
	}

	// Build apksigner command
	signArgs := []string{
		"sign",
		"--ks", keystorePath,
		"--ks-key-alias", keystoreAlias,
		"--ks-pass", "pass:" + keystorePass,
		"--key-pass", "pass:" + keystorePass,
		"--in", alignedApk,
		"--out", b.outputPath,
	}

	// For release builds, add additional verification
	if b.release {
		signArgs = append(signArgs, "--v1-signing-enabled", "true", "--v2-signing-enabled", "true")
	}

	if err := b.runner.Run(CommandSpec{
		Path:   apksigner,
		Args:   signArgs,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("apk signing failed: %w", err)
	}

	fmt.Printf("  Signed: %s\n", b.outputPath)

	// Step 3: Verify the signed APK
	if err := b.verifyAPK(); err != nil {
		fmt.Printf("  Warning: APK verification failed: %v\n", err)
	}

	return nil
}

// alignAPK aligns the APK using zipalign
func (b *androidBuilder) alignAPK(input, output string) error {
	zipalign, err := findSDKTool(b.sdk, "zipalign")
	if err != nil {
		return fmt.Errorf("zipalign not found: %w", err)
	}

	// Remove output if it exists
	os.Remove(output)

	// zipalign -p 4 = page alignment (4 bytes)
	if err := b.runner.Run(CommandSpec{
		Path:   zipalign,
		Args:   []string{"-p", "4", input, output},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("zipalign failed: %w", err)
	}

	return nil
}

// getKeystorePassword returns the keystore password from config, env, or prompt
func (b *androidBuilder) getKeystorePassword() string {
	// Priority: 1) Config file, 2) Environment variable, 3) Empty (will fail validation)
	if b.config.Android.Keystore.Password != "" {
		return b.config.Android.Keystore.Password
	}

	if pass := os.Getenv("LURPIC_KEYSTORE_PASSWORD"); pass != "" {
		return pass
	}

	return ""
}

// verifyAPK verifies the signed APK using apksigner
func (b *androidBuilder) verifyAPK() error {
	apksigner, err := findSDKTool(b.sdk, "apksigner")
	if err != nil {
		return fmt.Errorf("apksigner not found: %w", err)
	}

	output, err := b.runner.Output(CommandSpec{
		Path: apksigner,
		Args: []string{"verify", "--verbose", b.outputPath},
	})
	if err != nil {
		return fmt.Errorf("verification failed: %w\n%s", err, output)
	}

	// Only show verification output in verbose mode or on error
	if !b.release {
		// For debug builds, verification passes silently
		return nil
	}

	// For release builds, show full verification output
	fmt.Println("  APK verification:")
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) != "" {
			fmt.Printf("    %s\n", line)
		}
	}

	return nil
}

// findNDKCompiler finds the NDK clang compiler for the given architecture.
// The compiler binary is named <triple><api>-clang (e.g. aarch64-linux-android33-clang).
func (b *androidBuilder) findNDKCompiler(arch Architecture) string {
	toolchain := b.findNDKToolchain(arch.NDKTriple)
	if toolchain == "" {
		return ""
	}

	compilerName := fmt.Sprintf("%s%d-clang", arch.NDKTriple, b.apiLevel)
	compilerPath := filepath.Join(toolchain, compilerName)
	if runtime.GOOS == "windows" {
		compilerPath += ".exe"
	}

	if _, err := os.Stat(compilerPath); err == nil {
		return compilerPath
	}

	return ""
}

// findNDKToolchain returns the path to the NDK toolchain for a target
func (b *androidBuilder) findNDKToolchain(target string) string {
	// NDK r21+ layout
	toolchain := filepath.Join(b.ndk, "toolchains", "llvm", "prebuilt")

	// Determine host prebuilt directory
	host := runtime.GOOS
	if host == "darwin" {
		host = "darwin-x86_64"
	} else if host == "linux" {
		host = "linux-x86_64"
	} else if host == "windows" {
		host = "windows-x86_64"
	}

	toolchain = filepath.Join(toolchain, host, "bin")
	if _, err := os.Stat(toolchain); err == nil {
		return toolchain
	}

	return ""
}

// getDebugKeystore returns the path to the debug keystore, creating it if necessary
func (b *androidBuilder) getDebugKeystore() string {
	// Use user-local debug keystore
	home, _ := os.UserHomeDir()
	keystoreDir := filepath.Join(home, ".android")
	keystore := filepath.Join(keystoreDir, "debug.keystore")

	if _, err := os.Stat(keystore); err == nil {
		return keystore
	}

	// Create the debug keystore
	os.MkdirAll(keystoreDir, 0755)

	// Find keytool
	keytool := filepath.Join(os.Getenv("JAVA_HOME"), "bin", "keytool")
	if runtime.GOOS == "windows" {
		keytool += ".exe"
	}
	if _, err := os.Stat(keytool); err != nil {
		// Try to find keytool in PATH
		if found, lookErr := b.runner.Look("keytool"); lookErr == nil {
			keytool = found
		}
	}

	if keytool != "" {
		_ = b.runner.Run(CommandSpec{
			Path:   keytool,
			Args:   []string{"-genkey", "-v", "-keystore", keystore, "-alias", "androiddebugkey", "-storepass", "android", "-keypass", "android", "-keyalg", "RSA", "-validity", "10000", "-dname", "CN=Android Debug,O=Android,C=US"},
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
	}

	return keystore
}

// Helper functions

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(dst, data, 0644)
}
