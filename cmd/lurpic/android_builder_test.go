package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// argFlagValue returns the argument following flag in args (e.g. the value of
// "-o"), or "" if not present. Used so argv assertions tolerate added flags.
func argFlagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func createSDKWithTool(t *testing.T, sdkDir, tool string) (toolPath string) {
	t.Helper()
	buildTools := filepath.Join(sdkDir, "build-tools", "99.0.0")
	if err := os.MkdirAll(buildTools, 0o755); err != nil {
		t.Fatal(err)
	}
	toolPath = filepath.Join(buildTools, tool)
	if runtime.GOOS == "windows" {
		toolPath += ".exe"
	}
	if err := os.WriteFile(toolPath, []byte("#!/bin/sh\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}
	return toolPath
}

func createSDKWithPlatform(t *testing.T, sdkDir string, api int) {
	t.Helper()
	platformDir := filepath.Join(sdkDir, "platforms", "android-33")
	if err := os.MkdirAll(platformDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(platformDir, "android.jar")
	if err := os.WriteFile(jar, []byte("fake jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	adbDir := filepath.Join(sdkDir, "platform-tools")
	if err := os.MkdirAll(adbDir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func createNDKWithCompiler(t *testing.T, ndkDir, triple string, apiLevel int) (clangPath string) {
	t.Helper()
	host := "linux-x86_64"
	if runtime.GOOS == "darwin" {
		host = "darwin-x86_64"
	} else if runtime.GOOS == "windows" {
		host = "windows-x86_64"
	}
	toolchainDir := filepath.Join(ndkDir, "toolchains", "llvm", "prebuilt", host, "bin")
	if err := os.MkdirAll(toolchainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	clangName := fmt.Sprintf("%s%d-clang", triple, apiLevel)
	if runtime.GOOS == "windows" {
		clangName += ".exe"
	}
	clangPath = filepath.Join(toolchainDir, clangName)
	if err := os.WriteFile(clangPath, []byte("#!/bin/sh\necho \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return clangPath
}

func TestAndroidBuilder_selectAndroidAPI(t *testing.T) {
	b := &androidBuilder{
		config: &Config{
			Android: AndroidConfig{TargetSDK: 33},
		},
	}
	if err := b.selectAndroidAPI(); err != nil {
		t.Fatalf("selectAndroidAPI: %v", err)
	}
}

func TestAndroidBuilder_alignAPK_argv(t *testing.T) {
	sdkDir := t.TempDir()
	zipalignPath := createSDKWithTool(t, sdkDir, "zipalign")
	createSDKWithPlatform(t, sdkDir, 33)

	f := newFakeRunner()
	input := filepath.Join(sdkDir, "unsigned.apk")
	output := filepath.Join(sdkDir, "aligned.apk")
	if err := os.WriteFile(input, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	b := &androidBuilder{
		runner: f,
		sdk:    sdkDir,
		config: &Config{
			Android: AndroidConfig{TargetSDK: 33, MinSDK: 29},
		},
		buildDir:   sdkDir,
		outputPath: filepath.Join(sdkDir, "out.apk"),
	}

	f.When(MatchCommand(zipalignPath)).Then("", "", nil)

	if err := b.alignAPK(input, output); err != nil {
		t.Fatalf("alignAPK: %v", err)
	}

	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Path != zipalignPath {
		t.Fatalf("expected path %q, got %q", zipalignPath, calls[0].Path)
	}
	if len(calls[0].Args) != 4 || calls[0].Args[0] != "-p" || calls[0].Args[1] != "4" || calls[0].Args[2] != input || calls[0].Args[3] != output {
		t.Fatalf("unexpected zipalign args: %v", calls[0].Args)
	}
}

func TestAndroidBuilder_verifyAPK_argv(t *testing.T) {
	sdkDir := t.TempDir()
	apksignerPath := createSDKWithTool(t, sdkDir, "apksigner")
	outputPath := filepath.Join(sdkDir, "out.apk")
	if err := os.WriteFile(outputPath, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := newFakeRunner()
	b := &androidBuilder{
		runner:     f,
		sdk:        sdkDir,
		config:     &Config{Android: AndroidConfig{TargetSDK: 33, MinSDK: 29}},
		buildDir:   sdkDir,
		outputPath: outputPath,
	}

	f.When(MatchCommand(apksignerPath, "verify", "--verbose", outputPath)).Then("", "", nil)

	if err := b.verifyAPK(); err != nil {
		t.Fatalf("verifyAPK: %v", err)
	}

	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Path != apksignerPath {
		t.Fatalf("expected path %q, got %q", apksignerPath, calls[0].Path)
	}
}

func TestAndroidBuilder_signAPK_debug_argv(t *testing.T) {
	sdkDir := t.TempDir()
	apksignerPath := createSDKWithTool(t, sdkDir, "apksigner")
	zipalignPath := createSDKWithTool(t, sdkDir, "zipalign")
	createSDKWithPlatform(t, sdkDir, 33)

	unsignedApk := filepath.Join(sdkDir, "unsigned.apk")
	if err := os.WriteFile(unsignedApk, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	debugKeystore := filepath.Join(homeDir, ".android", "debug.keystore")
	if err := os.MkdirAll(filepath.Dir(debugKeystore), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(debugKeystore, []byte("fake keystore"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := newFakeRunner()
	b := &androidBuilder{
		runner:     f,
		sdk:        sdkDir,
		config:     &Config{Android: AndroidConfig{TargetSDK: 33, MinSDK: 29}},
		buildDir:   sdkDir,
		outputPath: filepath.Join(sdkDir, "out.apk"),
		release:    false,
	}

	f.When(MatchCommand(zipalignPath)).Then("", "", nil)
	f.When(MatchCommand(apksignerPath)).Then("", "", nil)

	if err := b.signAPK(); err != nil {
		t.Fatalf("signAPK: %v", err)
	}

	calls := f.Calls()
	if len(calls) < 2 {
		t.Fatalf("expected at least 2 calls, got %d", len(calls))
	}
	if calls[0].Path != zipalignPath {
		t.Fatalf("call 0 expected %q, got %q", zipalignPath, calls[0].Path)
	}
	if calls[1].Path != apksignerPath {
		t.Fatalf("call 1 expected %q, got %q", apksignerPath, calls[1].Path)
	}
	if len(calls[1].Args) < 2 || calls[1].Args[0] != "sign" {
		t.Fatalf("call 1 expected args starting with 'sign', got %v", calls[1].Args)
	}
}

func TestAndroidBuilder_buildGoLibrary_x86_64_argv(t *testing.T) {
	sdkDir := t.TempDir()
	ndkDir := t.TempDir()
	projectRoot := t.TempDir()

	mainGo := filepath.Join(projectRoot, "main.go")
	if err := os.WriteFile(mainGo, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create NDK clang for x86_64-linux-android
	createNDKWithCompiler(t, ndkDir, "x86_64-linux-android", 33)

	f := newFakeRunner()
	b := &androidBuilder{
		runner:      f,
		sdk:         sdkDir,
		ndk:         ndkDir,
		projectRoot: projectRoot,
		buildDir:    filepath.Join(projectRoot, "build", "android"),
		apiLevel:    33,
		config: &Config{
			App:     AppConfig{ID: "org.test.app", Name: "TestApp"},
			Android: AndroidConfig{TargetSDK: 33, MinSDK: 29},
		},
		outputPath: filepath.Join(projectRoot, "build", "android", "output.apk"),
	}

	f.When(MatchCommand("go")).Then("", "", nil)

	arch := DefaultEmulatorArchitecture()
	if err := b.buildGoLibrary(arch); err != nil {
		t.Fatalf("buildGoLibrary: %v", err)
	}

	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Path != "go" {
		t.Fatalf("expected 'go', got %q", calls[0].Path)
	}

	// Verify GOARCH=amd64 for x86_64 ABI
	foundGOARCH := false
	foundGOOS := false
	foundCC := false
	for _, e := range calls[0].Env {
		if e == "GOARCH=amd64" {
			foundGOARCH = true
		}
		if e == "GOOS=android" {
			foundGOOS = true
		}
		if e == "CGO_ENABLED=1" {
			foundCC = true
		}
	}
	if !foundGOARCH {
		t.Fatal("expected GOARCH=amd64 in environment")
	}
	if !foundGOOS {
		t.Fatal("expected GOOS=android in environment")
	}
	if !foundCC {
		t.Fatal("expected CGO_ENABLED=1 in environment")
	}

	// Verify output path uses the ABI directory
	expectedOutput := filepath.Join(projectRoot, "build", "android", "lib", "x86_64", "libgo.so")
	if got := argFlagValue(calls[0].Args, "-o"); got != expectedOutput {
		t.Fatalf("expected output %q, got args: %v", expectedOutput, calls[0].Args)
	}
}

func TestAndroidBuilder_buildGoLibrary_arm64_argv(t *testing.T) {
	sdkDir := t.TempDir()
	ndkDir := t.TempDir()
	projectRoot := t.TempDir()

	mainGo := filepath.Join(projectRoot, "main.go")
	if err := os.WriteFile(mainGo, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	createNDKWithCompiler(t, ndkDir, "aarch64-linux-android", 33)

	f := newFakeRunner()
	b := &androidBuilder{
		runner:      f,
		sdk:         sdkDir,
		ndk:         ndkDir,
		projectRoot: projectRoot,
		buildDir:    filepath.Join(projectRoot, "build", "android"),
		apiLevel:    33,
		config: &Config{
			App:     AppConfig{ID: "org.test.app", Name: "TestApp"},
			Android: AndroidConfig{TargetSDK: 33, MinSDK: 29},
		},
		outputPath: filepath.Join(projectRoot, "build", "android", "output.apk"),
	}

	f.When(MatchCommand("go")).Then("", "", nil)

	arch, ok := ArchitectureByABI("arm64-v8a")
	if !ok {
		t.Fatal("arm64-v8a architecture not found")
	}
	if err := b.buildGoLibrary(arch); err != nil {
		t.Fatalf("buildGoLibrary(arm64-v8a): %v", err)
	}

	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}

	foundGOARCH := false
	foundGOOS := false
	for _, e := range calls[0].Env {
		if e == "GOARCH=arm64" {
			foundGOARCH = true
		}
		if e == "GOOS=android" {
			foundGOOS = true
		}
	}
	if !foundGOARCH {
		t.Fatal("expected GOARCH=arm64 in environment")
	}
	if !foundGOOS {
		t.Fatal("expected GOOS=android in environment")
	}

	// Verify output path uses arm64-v8a ABI directory
	expectedOutput := filepath.Join(projectRoot, "build", "android", "lib", "arm64-v8a", "libgo.so")
	if got := argFlagValue(calls[0].Args, "-o"); got != expectedOutput {
		t.Fatalf("expected output %q, got args: %v", expectedOutput, calls[0].Args)
	}
}

func TestAndroidBuilder_buildGoLibrary_noMainIsFatal(t *testing.T) {
	projectRoot := t.TempDir()

	f := newFakeRunner()
	b := &androidBuilder{
		runner:      f,
		projectRoot: projectRoot,
		buildDir:    filepath.Join(projectRoot, "build", "android"),
		config: &Config{
			App:     AppConfig{ID: "org.test.app"},
			Android: AndroidConfig{TargetSDK: 33, MinSDK: 29},
		},
	}

	err := b.buildGoLibrary(DefaultEmulatorArchitecture())
	if err == nil {
		t.Fatal("expected error for missing main.go, got nil")
	}
}

func TestAndroidBuilder_compileResources_failsWithoutAapt2(t *testing.T) {
	sdkDir := t.TempDir()
	createSDKWithPlatform(t, sdkDir, 33)

	f := newFakeRunner()
	buildDir := t.TempDir()
	b := &androidBuilder{
		runner:   f,
		sdk:      sdkDir,
		config:   &Config{Android: AndroidConfig{TargetSDK: 33, MinSDK: 29}},
		buildDir: buildDir,
	}

	// Calling assembly as a whole should fail because aapt2 is missing
	err := b.assembleAPK()
	if err == nil {
		t.Fatal("expected error when aapt2 is missing")
	}
}

func TestAndroidBuilder_signAPK_releaseNeedsKeystoreConfig(t *testing.T) {
	sdkDir := t.TempDir()
	_ = createSDKWithTool(t, sdkDir, "apksigner")
	createSDKWithPlatform(t, sdkDir, 33)

	unsignedApk := filepath.Join(sdkDir, "unsigned.apk")
	if err := os.WriteFile(unsignedApk, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := newFakeRunner()
	b := &androidBuilder{
		runner:     f,
		sdk:        sdkDir,
		config:     &Config{Android: AndroidConfig{TargetSDK: 33, MinSDK: 29}},
		buildDir:   sdkDir,
		outputPath: filepath.Join(sdkDir, "out.apk"),
		release:    true,
	}

	err := b.signAPK()
	if err == nil {
		t.Fatal("expected error for missing keystore config in release mode")
	}
}

func TestAndroidBuilder_buildRustCrate_artifactCopied(t *testing.T) {
	projectRoot := t.TempDir()

	// Create a fake crate with Cargo.toml
	cratePath := filepath.Join(projectRoot, "crates", "lurpic_render")
	if err := os.MkdirAll(cratePath, 0o755); err != nil {
		t.Fatal(err)
	}
	cargoToml := filepath.Join(cratePath, "Cargo.toml")
	if err := os.WriteFile(cargoToml, []byte("[package]\nname = \"lurpic_render\"\n[lib]\ncrate-type = [\"cdylib\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create the Cargo.toml at project root to trigger the root crate path
	rootCargo := filepath.Join(projectRoot, "Cargo.toml")
	if err := os.WriteFile(rootCargo, []byte("[workspace]\nmembers = [\"crates/*\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create the expected build artifact before calling buildRustCrate
	arch := DefaultEmulatorArchitecture()
	targetDir := filepath.Join(cratePath, "target", arch.CargoTarget, "release")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(targetDir, "liblurpic_render.so")
	if err := os.WriteFile(artifactPath, []byte("fake rust so"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := newFakeRunner()
	b := &androidBuilder{
		runner:      f,
		projectRoot: projectRoot,
		buildDir:    filepath.Join(projectRoot, "build", "android"),
		config: &Config{
			App:     AppConfig{ID: "org.test.app"},
			Android: AndroidConfig{TargetSDK: 33, MinSDK: 29},
		},
		apiLevel: 33,
	}

	// Ensure cargo-ndk is not found so the test takes the plain cargo path
	f.WhenLook("cargo-ndk").Returns("", fmt.Errorf("not found"))
	f.When(MatchCommand("cargo")).Then("", "", nil)

	if err := b.buildRustCrate(arch, cratePath, "lurpic_render"); err != nil {
		t.Fatalf("buildRustCrate: %v", err)
	}

	// Verify the .so was copied to lib/<abi>/
	destPath := filepath.Join(projectRoot, "build", "android", "lib", arch.ABI, "liblurpic_render.so")
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("expected artifact at %s: %v", destPath, err)
	}
	if string(data) != "fake rust so" {
		t.Fatalf("expected artifact content 'fake rust so', got %q", string(data))
	}
}

func TestAndroidBuilder_buildRustCrate_missingArtifactIsFatal(t *testing.T) {
	projectRoot := t.TempDir()

	// Create a fake crate
	cratePath := filepath.Join(projectRoot, "crates", "missing_render")
	if err := os.MkdirAll(cratePath, 0o755); err != nil {
		t.Fatal(err)
	}
	cargoToml := filepath.Join(cratePath, "Cargo.toml")
	if err := os.WriteFile(cargoToml, []byte("[package]\nname = \"missing_render\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// DO NOT create the target build artifact

	f := newFakeRunner()
	b := &androidBuilder{
		runner:      f,
		projectRoot: projectRoot,
		buildDir:    filepath.Join(projectRoot, "build", "android"),
		config: &Config{
			App:     AppConfig{ID: "org.test.app"},
			Android: AndroidConfig{TargetSDK: 33, MinSDK: 29},
		},
	}

	// Ensure cargo-ndk is not found so the test takes the plain cargo path
	f.WhenLook("cargo-ndk").Returns("", fmt.Errorf("not found"))
	f.When(MatchCommand("cargo")).Then("", "", nil)
	f.When(MatchCommand("cargo-ndk")).Then("", "", nil)

	err := b.buildRustCrate(DefaultEmulatorArchitecture(), cratePath, "missing_render")
	if err == nil {
		t.Fatal("expected error for missing Rust artifact, got nil")
	}
}

func TestAndroidBuilder_aapt2Link_argv(t *testing.T) {
	sdkDir := t.TempDir()
	createSDKWithPlatform(t, sdkDir, 33)
	buildDir := t.TempDir()

	// Create aapt2 tool in build-tools
	aapt2Path := filepath.Join(sdkDir, "build-tools", "99.0.0", "aapt2")
	if err := os.MkdirAll(filepath.Dir(aapt2Path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aapt2Path, []byte("#!/bin/sh\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	f := newFakeRunner()
	b := &androidBuilder{
		runner:   f,
		sdk:      sdkDir,
		buildDir: buildDir,
		apiLevel: 33,
		config: &Config{
			App:     AppConfig{ID: "org.test.app", Name: "Test", Version: "1.0.0"},
			Android: AndroidConfig{TargetSDK: 33, MinSDK: 29},
		},
	}

	// Generate manifest so it exists for aapt2 link
	if err := b.generateManifest(); err != nil {
		t.Fatal(err)
	}

	// Register aapt2 to succeed
	f.When(MatchCommand(aapt2Path)).Then("", "", nil)

	_, err := b.compileResources(aapt2Path)
	if err != nil {
		t.Fatalf("compileResources: %v", err)
	}

	calls := f.Calls()
	if len(calls) < 1 {
		t.Fatalf("expected at least 1 call, got %d", len(calls))
	}

	linkCall := calls[0]
	if linkCall.Path != aapt2Path {
		t.Fatalf("expected aapt2 path, got %q", linkCall.Path)
	}
	if len(linkCall.Args) < 2 || linkCall.Args[0] != "link" {
		t.Fatalf("expected args starting with 'link', got %v", linkCall.Args)
	}

	hasManifest := false
	hasBaseApk := false
	hasAndroidJar := false
	for _, a := range linkCall.Args {
		if strings.HasSuffix(a, "AndroidManifest.xml") {
			hasManifest = true
		}
		if strings.HasSuffix(a, "base.apk") {
			hasBaseApk = true
		}
		if strings.HasSuffix(a, "android.jar") {
			hasAndroidJar = true
		}
	}
	if !hasManifest {
		t.Fatal("aapt2 link args missing --manifest")
	}
	if !hasBaseApk {
		t.Fatal("aapt2 link args missing -o base.apk")
	}
	if !hasAndroidJar {
		t.Fatal("aapt2 link args missing -I android.jar")
	}
}

func TestAndroidBuilder_assembleAPK_failsWithoutAapt2(t *testing.T) {
	sdkDir := t.TempDir()
	createSDKWithPlatform(t, sdkDir, 33)
	buildDir := t.TempDir()

	f := newFakeRunner()
	b := &androidBuilder{
		runner:     f,
		sdk:        sdkDir,
		buildDir:   buildDir,
		config:     &Config{App: AppConfig{ID: "org.test"}, Android: AndroidConfig{TargetSDK: 33, MinSDK: 29}},
		outputPath: filepath.Join(buildDir, "out.apk"),
	}

	err := b.assembleAPK()
	if err == nil {
		t.Fatal("expected error when aapt2 is missing")
	}
}

func TestAndroidBuilder_build_unsupportedABI(t *testing.T) {
	b := &androidBuilder{
		config: &Config{
			Android: AndroidConfig{
				TargetSDK: 33,
				ABIs:      []string{"riscv64"},
			},
		},
	}

	err := b.build()
	if err == nil {
		t.Fatal("expected error for unsupported ABI")
	}
}
