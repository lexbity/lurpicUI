package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"codeburg.org/lexbit/lurpicui/platform/android"
	"golang.org/x/term"
)

// androidEntrySource is injected into the app's main package (android build only)
// via go build -overlay. Under -buildmode=c-shared the Go runtime runs package
// init at library load but never calls main(); this starts main() on a dedicated
// goroutine so app.Run executes and waits for the NativeActivity surface.
const androidEntrySource = `//go:build android

package main

func init() {
	go main()
}
`

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
	ksPassword  string
	jdk         string

	cachedModuleDir string
}

// build executes the full Android build pipeline
func (b *androidBuilder) build() error {
	fmt.Println("=== Building Android APK ===")

	if err := b.selectAndroidAPI(); err != nil {
		return err
	}

	// Step 1: Build the framework render Rust lib, then the Go shared library
	// (which links against it), for each configured ABI.
	for _, abi := range b.config.Android.ABIs {
		arch, ok := ArchitectureByABI(abi)
		if !ok {
			return fmt.Errorf("unsupported ABI: %s", abi)
		}
		if err := b.buildRenderRustLib(arch); err != nil {
			return fmt.Errorf("render Rust build failed for %s: %w", abi, err)
		}
		if err := b.buildGoLibrary(arch); err != nil {
			return fmt.Errorf("Go build failed for %s: %w", abi, err)
		}
	}

	// Step 2: Build the app's own Rust library (if present) for each ABI.
	for _, abi := range b.config.Android.ABIs {
		arch, ok := ArchitectureByABI(abi)
		if !ok {
			return fmt.Errorf("unsupported ABI: %s", abi)
		}
		if err := b.buildRustLibrary(arch); err != nil {
			return fmt.Errorf("Rust build failed for %s: %w", abi, err)
		}
	}

	// Step 3: Generate AndroidManifest.xml
	if err := b.generateManifest(); err != nil {
		return fmt.Errorf("manifest generation failed: %w", err)
	}

	// Step 4: Bundle assets
	if err := b.bundleAssets(); err != nil {
		return fmt.Errorf("asset bundling failed: %w", err)
	}

	// Step 5: Compile + dex the Java NativeActivity
	if err := b.buildJavaDex(); err != nil {
		return fmt.Errorf("java/dex build failed: %w", err)
	}

	// Step 6: Assemble APK
	if err := b.assembleAPK(); err != nil {
		return fmt.Errorf("APK assembly failed: %w", err)
	}

	// Step 7: Sign APK
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
	// Link against the framework render lib bundled in lib/<abi> so libgo.so
	// resolves the lurpic_render_* symbols (DT_NEEDED liblurpic_render.so).
	env = append(env, fmt.Sprintf("CGO_LDFLAGS=-L%s -llurpic_render", libDir))

	// The Go entrypoint package is configured via app.main (relative to the
	// project root), independent of the Android applicationId (app.id).
	mainPkg := b.config.App.Main
	if mainPkg == "" {
		mainPkg = "."
	}

	// Missing main.go in the configured package is a fatal error.
	mainPath := filepath.Join(b.projectRoot, mainPkg, "main.go")
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		return fmt.Errorf("no main.go found at %s (app.main = %q)", mainPath, mainPkg)
	}

	// Under -buildmode=c-shared, main() is not invoked at load; only package
	// init runs. Inject an android-only init that starts main() on a goroutine,
	// via -overlay so the app's source tree is never modified.
	overlayPath, err := b.writeAndroidEntryOverlay(mainPkg)
	if err != nil {
		return err
	}

	output := filepath.Join(libDir, "libgo.so")
	args := []string{
		"build",
		"-buildmode=c-shared",
		"-overlay", overlayPath,
		"-o", output,
		"./" + filepath.ToSlash(mainPkg),
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

// writeAndroidEntryOverlay writes the android entry source and a go build
// overlay that maps it into the app's main package without touching the source
// tree. It returns the overlay JSON path.
func (b *androidBuilder) writeAndroidEntryOverlay(mainPkg string) (string, error) {
	contentPath := filepath.Join(b.buildDir, "android_entry.go")
	if err := os.WriteFile(contentPath, []byte(androidEntrySource), 0644); err != nil {
		return "", fmt.Errorf("write android entry: %w", err)
	}

	// The injected (virtual) file lives inside the main package directory.
	mainDir := filepath.Join(b.projectRoot, mainPkg)
	injected := filepath.Join(mainDir, "zz_lurpic_android_entry.go")
	if _, err := os.Stat(injected); err == nil {
		return "", fmt.Errorf("cannot inject android entry: %s already exists", injected)
	}

	overlay := struct {
		Replace map[string]string `json:"Replace"`
	}{Replace: map[string]string{injected: contentPath}}

	data, err := json.Marshal(overlay)
	if err != nil {
		return "", err
	}
	overlayPath := filepath.Join(b.buildDir, "overlay.json")
	if err := os.WriteFile(overlayPath, data, 0644); err != nil {
		return "", fmt.Errorf("write overlay: %w", err)
	}
	return overlayPath, nil
}

// moduleDir resolves the on-disk directory of the lurpicui framework module, so
// the build can locate framework assets (the render crate) regardless of where
// the user's project lives.
func (b *androidBuilder) moduleDir() (string, error) {
	if b.cachedModuleDir != "" {
		return b.cachedModuleDir, nil
	}
	out, err := b.runner.Output(CommandSpec{
		Path: "go",
		Args: []string{"list", "-m", "-f", "{{.Dir}}", "codeburg.org/lexbit/lurpicui"},
		Dir:  b.projectRoot,
	})
	if err != nil {
		return "", fmt.Errorf("locate lurpicui module: %w\n%s", err, out)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", fmt.Errorf("could not resolve lurpicui module directory")
	}
	b.cachedModuleDir = dir
	return dir, nil
}

// buildRenderRustLib builds the framework's lurpic_render crate for the given ABI
// with cargo-ndk and stages liblurpic_render.so into lib/<abi> so the Go shared
// library can link and load it.
func (b *androidBuilder) buildRenderRustLib(arch Architecture) error {
	fmt.Printf("Building render Rust library for %s...\n", arch.ABI)

	moduleDir, err := b.moduleDir()
	if err != nil {
		return err
	}
	crateDir := filepath.Join(moduleDir, "render", "vulkan", "crates", "lurpic_render")
	if _, err := os.Stat(filepath.Join(crateDir, "Cargo.toml")); err != nil {
		return fmt.Errorf("render crate not found at %s: %w", crateDir, err)
	}

	outDir := filepath.Join(b.buildDir, "rustout")
	env := os.Environ()
	env = append(env, "ANDROID_NDK_HOME="+b.ndk, "ANDROID_NDK_ROOT="+b.ndk)

	cargoArgs := []string{"ndk", "-t", arch.ABI, "-o", outDir, "build"}
	if b.release {
		cargoArgs = append(cargoArgs, "--release")
	}
	// On debug builds (no --release), the Rust dev profile is used, which
	// enables debug assertions, panic=unwind, and no optimizations — matching
	// the debuggability expectations for development APKs. Release builds
	// use the release profile (panic=unwind, LTO thin, opt-level 2).
	if err := b.runner.Run(CommandSpec{
		Path:   "cargo",
		Args:   cargoArgs,
		Dir:    crateDir,
		Env:    env,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("cargo ndk build failed: %w", err)
	}

	produced := filepath.Join(outDir, arch.ABI, "liblurpic_render.so")
	if _, err := os.Stat(produced); err != nil {
		return fmt.Errorf("cargo ndk did not produce %s: %w", produced, err)
	}

	libDir := filepath.Join(b.buildDir, "lib", arch.ABI)
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		return err
	}
	if err := copyFile(produced, filepath.Join(libDir, "liblurpic_render.so")); err != nil {
		return fmt.Errorf("stage liblurpic_render.so: %w", err)
	}
	fmt.Printf("  Built: %s\n", filepath.Join(libDir, "liblurpic_render.so"))
	return nil
}

// buildRustLibrary cross-compiles Rust code for Android for the given architecture.
func (b *androidBuilder) buildRustLibrary(arch Architecture) error {
	fmt.Printf("Building Rust library for %s...\n", arch.ABI)

	cargoToml := filepath.Join(b.projectRoot, "Cargo.toml")
	if _, err := os.Stat(cargoToml); os.IsNotExist(err) {
		cratesDir := filepath.Join(b.projectRoot, "crates")
		if entries, err := os.ReadDir(cratesDir); err == nil && len(entries) > 0 {
			for _, entry := range entries {
				if entry.IsDir() {
					cratePath := filepath.Join(cratesDir, entry.Name())
					if _, err := os.Stat(filepath.Join(cratePath, "Cargo.toml")); err == nil {
						if err := b.buildRustCrate(arch, cratePath, entry.Name()); err != nil {
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

	return b.buildRustCrate(arch, b.projectRoot, "main")
}

// buildRustCrate builds a single Rust crate for Android for the given architecture.
func (b *androidBuilder) buildRustCrate(arch Architecture, cratePath, name string) error {
	libDir := filepath.Join(b.buildDir, "lib", arch.ABI)
	if err := os.MkdirAll(libDir, 0755); err != nil {
		return err
	}

	target := arch.CargoTarget
	env := os.Environ()

	toolchain := b.findNDKToolchain(arch.NDKTriple)
	if toolchain != "" {
		clangName := fmt.Sprintf("%s%d-clang", arch.NDKTriple, b.apiLevel)
		clangPath := filepath.Join(toolchain, clangName)
		env = append(env,
			fmt.Sprintf("CC_%s=%s", target, clangPath),
			fmt.Sprintf("CXX_%s=%s", target, filepath.Join(toolchain, strings.Replace(clangName, "-clang", "++", 1))),
			fmt.Sprintf("AR_%s=%s", target, filepath.Join(toolchain, "llvm-ar")),
		)
	}

	// For the user's Rust crate, debug builds use the dev profile
	// (debug assertions + unwind), release builds use release (unwind + LTO).
	profileFlag := ""
	if b.release {
		profileFlag = "--release"
	}

	cargoNdk, err := b.runner.Look("cargo-ndk")
	if err == nil {
		args := []string{"-t", arch.ABI, "build"}
		if profileFlag != "" {
			args = append(args, profileFlag)
		}
		if err := b.runner.Run(CommandSpec{
			Path:   cargoNdk,
			Args:   args,
			Dir:    cratePath,
			Env:    env,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}); err != nil {
			return fmt.Errorf("cargo-ndk build failed: %w", err)
		}
	} else {
		args := []string{"build"}
		if profileFlag != "" {
			args = append(args, profileFlag)
		}
		args = append(args, "--target", target)
		if err := b.runner.Run(CommandSpec{
			Path:   "cargo",
			Args:   args,
			Dir:    cratePath,
			Env:    env,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}); err != nil {
			return fmt.Errorf("cargo build failed: %w", err)
		}
	}

	// Copy the built .so artifact(s) to the jniLibs directory.
	// Without --release, Cargo builds to the debug profile directory.
	profileDir := "release"
	if !b.release {
		profileDir = "debug"
	}
	pattern := filepath.Join(cratePath, "target", target, profileDir, "lib*.so")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return fmt.Errorf("no Rust shared library found after building crate %q (expected pattern: %s)", name, pattern)
	}
	for _, src := range matches {
		dst := filepath.Join(libDir, filepath.Base(src))
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("failed to copy Rust library %s: %w", src, err)
		}
		fmt.Printf("  Copied: %s\n", dst)
	}

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

	data := ManifestData{
		Package:            b.config.App.ID,
		VersionCode:        b.config.Android.VersionCode,
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

// compileResources uses aapt2 to compile resources and link the APK.
// The caller must resolve the aapt2 path first.
func (b *androidBuilder) compileResources(aapt2 string) (string, error) {
	fmt.Println("Compiling resources with aapt2...")

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

// assembleAPK creates the final APK by combining base APK with native libs and assets.
// Requires aapt2 — there is no fallback assembly path.
func (b *androidBuilder) assembleAPK() error {
	fmt.Println("Assembling APK...")

	aapt2, err := findSDKTool(b.sdk, "aapt2")
	if err != nil {
		return fmt.Errorf("aapt2 not found: %w", err)
	}

	// Step 1: Compile resources and get base APK
	baseApk, err := b.compileResources(aapt2)
	if err != nil {
		return fmt.Errorf("resource compilation failed: %w", err)
	}

	unsignedApk := filepath.Join(b.buildDir, "unsigned.apk")

	// Step 2: Copy base APK to unsigned APK
	if err := copyFile(baseApk, unsignedApk); err != nil {
		return fmt.Errorf("failed to copy base APK: %w", err)
	}

	// Step 3: Add native libraries to the APK (lib/<abi>/...).
	libSrc := filepath.Join(b.buildDir, "lib")
	if _, err := os.Stat(libSrc); err == nil {
		if err := addTreeToAPK(unsignedApk, b.buildDir, "lib"); err != nil {
			return fmt.Errorf("failed to add native libs to APK: %w", err)
		}
	}

	// Step 4: Add assets to the APK (assets/...).
	assetsSrc := filepath.Join(b.buildDir, "assets")
	if _, err := os.Stat(assetsSrc); err == nil {
		if err := addTreeToAPK(unsignedApk, b.buildDir, "assets"); err != nil {
			return fmt.Errorf("failed to add assets to APK: %w", err)
		}
	}

	// Step 5: Add the dexed Java code at the APK root (classes.dex).
	dexPath := filepath.Join(b.buildDir, "classes.dex")
	if _, err := os.Stat(dexPath); err != nil {
		return fmt.Errorf("classes.dex not found (java/dex stage must run before assembly): %w", err)
	}
	if err := addFileToAPK(unsignedApk, dexPath, "classes.dex"); err != nil {
		return fmt.Errorf("failed to add classes.dex to APK: %w", err)
	}

	fmt.Printf("  Assembled: %s\n", unsignedApk)
	return nil
}

// addFileToAPK injects a single file into the APK at the given entry name (e.g.
// "classes.dex" at the archive root), rewriting the archive atomically.
func addFileToAPK(apkPath, srcPath, entryName string) error {
	src, err := zip.OpenReader(apkPath)
	if err != nil {
		return fmt.Errorf("open apk: %w", err)
	}
	defer src.Close()

	tmpPath := apkPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer os.Remove(tmpPath)
	zw := zip.NewWriter(out)

	for _, f := range src.File {
		if f.Name == entryName {
			continue // replace any existing entry
		}
		if err := zw.Copy(f); err != nil {
			out.Close()
			return fmt.Errorf("copy entry %q: %w", f.Name, err)
		}
	}

	w, err := zw.Create(entryName)
	if err != nil {
		out.Close()
		return err
	}
	in, err := os.Open(srcPath)
	if err != nil {
		out.Close()
		return err
	}
	if _, err := io.Copy(w, in); err != nil {
		in.Close()
		out.Close()
		return err
	}
	in.Close()

	if err := zw.Close(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, apkPath)
}

// addTreeToAPK injects a staged directory tree (e.g. "lib" or "assets") into the
// linked APK at the matching top-level entry path. An APK is a zip archive and
// aapt2 has no "add" subcommand, so the tree is merged in-process with
// archive/zip — avoiding a dependency on an external zip binary. Entry paths are
// taken relative to baseDir so they carry the correct prefix (lib/<abi>/libgo.so,
// assets/...). The APK is rewritten atomically via a temp file + rename.
func addTreeToAPK(apkPath, baseDir, treeName string) error {
	src, err := zip.OpenReader(apkPath)
	if err != nil {
		return fmt.Errorf("open apk: %w", err)
	}
	defer src.Close()

	tmpPath := apkPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer os.Remove(tmpPath)
	zw := zip.NewWriter(out)

	// Preserve existing entries verbatim (no recompression).
	for _, f := range src.File {
		if err := zw.Copy(f); err != nil {
			out.Close()
			return fmt.Errorf("copy entry %q: %w", f.Name, err)
		}
	}

	// Add the staged tree.
	root := filepath.Join(baseDir, treeName)
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		return err
	}); err != nil {
		out.Close()
		return err
	}

	if err := zw.Close(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, apkPath)
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
		// Release signing - requires keystore configuration or command-line overrides
		keystorePath = b.config.Android.Keystore.Path
		keystoreAlias = b.config.Android.Keystore.Alias
		keystorePass = b.ksPassword
		if keystorePass == "" {
			keystorePass = os.Getenv("LURPIC_KEYSTORE_PASSWORD")
		}
		if keystorePass == "" {
			keystorePass = b.promptPassword()
		}

		if keystorePath == "" {
			return fmt.Errorf("release signing requires keystore.path in lurpic.toml or --keystore flag")
		}
		if keystoreAlias == "" {
			return fmt.Errorf("release signing requires keystore.alias in lurpic.toml or --ks-alias flag")
		}
		if keystorePass == "" {
			return fmt.Errorf("release signing requires keystore password. Use --ks-pass flag, or set LURPIC_KEYSTORE_PASSWORD environment variable")
		}

		// Validate keystore exists
		if _, err := os.Stat(keystorePath); err != nil {
			return fmt.Errorf("keystore not found at %s: %w", keystorePath, err)
		}

		fmt.Printf("  Using release keystore: %s (alias: %s)\n", keystorePath, keystoreAlias)
	} else {
		// Debug signing
		var err error
		keystorePath, err = b.getDebugKeystore()
		if err != nil {
			return fmt.Errorf("debug keystore: %w", err)
		}
		keystoreAlias = "androiddebugkey"
		keystorePass = "android"
		fmt.Printf("  Using debug keystore: %s\n", keystorePath)
	}

	// Build apksigner command — password is passed on the command line to apksigner,
	// but we take care never to log it or store it in config.
	signArgs := []string{
		"sign",
		"--ks", keystorePath,
		"--ks-key-alias", keystoreAlias,
		"--ks-pass", "pass:" + keystorePass,
		"--in", alignedApk,
		"--out", b.outputPath,
	}

	// For release builds, enable v1/v2 signing schemes
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

	// Step 3: Verify the signed APK (fatal on failure)
	if err := b.verifyAPK(); err != nil {
		return fmt.Errorf("APK verification failed: %w", err)
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

// promptPassword reads a keystore password from the terminal with echo disabled.
// Returns empty string if the terminal is not available.
func (b *androidBuilder) promptPassword() string {
	fmt.Fprint(os.Stderr, "Enter keystore password: ")
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return ""
	}
	fmt.Fprintln(os.Stderr)
	passStr := string(pass)
	b.ksPassword = passStr
	return passStr
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

// getDebugKeystore returns the path to the debug keystore, creating it if necessary.
// Returns an error if keytool is not found or the keystore cannot be created.
func (b *androidBuilder) getDebugKeystore() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	keystoreDir := filepath.Join(home, ".android")
	keystore := filepath.Join(keystoreDir, "debug.keystore")

	if _, err := os.Stat(keystore); err == nil {
		return keystore, nil
	}

	// Create the debug keystore
	if err := os.MkdirAll(keystoreDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create .android directory: %w", err)
	}

	// Find keytool
	keytool := filepath.Join(os.Getenv("JAVA_HOME"), "bin", "keytool")
	if runtime.GOOS == "windows" {
		keytool += ".exe"
	}
	if _, err := os.Stat(keytool); err != nil {
		if found, lookErr := b.runner.Look("keytool"); lookErr == nil {
			keytool = found
		} else {
			return "", fmt.Errorf("keytool not found: install a JDK or set JAVA_HOME")
		}
	}

	if err := b.runner.Run(CommandSpec{
		Path:   keytool,
		Args:   []string{"-genkey", "-v", "-keystore", keystore, "-alias", "androiddebugkey", "-storepass", "android", "-keypass", "android", "-keyalg", "RSA", "-validity", "10000", "-dname", "CN=Android Debug,O=Android,C=US"},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return "", fmt.Errorf("failed to generate debug keystore with keytool: %w", err)
	}

	return keystore, nil
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
