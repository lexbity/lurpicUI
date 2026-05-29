package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

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
	_ = filepath.Join(sdkDir, "platform-tools", "adb")
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

	// Set HOME so debug keystore resolves to a temp location
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

	// zipalign and apksigner sign should succeed
	f.When(MatchCommand(zipalignPath)).Then("", "", nil)
	f.When(MatchCommand(apksignerPath)).Then("", "", nil)

	if err := b.signAPK(); err != nil {
		t.Fatalf("signAPK: %v", err)
	}

	calls := f.Calls()
	// We expect: zipalign, apksigner sign, apksigner verify
	if len(calls) < 2 {
		t.Fatalf("expected at least 2 calls, got %d", len(calls))
	}
	// First call should be zipalign
	if calls[0].Path != zipalignPath {
		t.Fatalf("call 0 expected %q, got %q", zipalignPath, calls[0].Path)
	}
	// Second call should be apksigner sign
	if calls[1].Path != apksignerPath {
		t.Fatalf("call 1 expected %q, got %q", apksignerPath, calls[1].Path)
	}
	if len(calls[1].Args) < 2 || calls[1].Args[0] != "sign" {
		t.Fatalf("call 1 expected args starting with 'sign', got %v", calls[1].Args)
	}
}

func TestAndroidBuilder_buildGoLibrary_argv(t *testing.T) {
	sdkDir := t.TempDir()
	ndkDir := t.TempDir()
	projectRoot := t.TempDir()

	// Create a minimal main.go in the project root
	mainGo := filepath.Join(projectRoot, "main.go")
	if err := os.WriteFile(mainGo, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create NDK toolchain directory
	toolchainDir := filepath.Join(ndkDir, "toolchains", "llvm", "prebuilt", "linux-x86_64", "bin")
	if err := os.MkdirAll(toolchainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	clangPath := filepath.Join(toolchainDir, "clang")
	if err := os.WriteFile(clangPath, []byte("#!/bin/sh\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	f := newFakeRunner()
	b := &androidBuilder{
		runner:      f,
		sdk:         sdkDir,
		ndk:         ndkDir,
		projectRoot: projectRoot,
		buildDir:    filepath.Join(projectRoot, "build", "android"),
		config: &Config{
			App:     AppConfig{ID: "org.test.app", Name: "TestApp"},
			Android: AndroidConfig{TargetSDK: 33, MinSDK: 29},
		},
		release:    false,
		outputPath: filepath.Join(projectRoot, "build", "android", "output.apk"),
	}

	// buildGoLibrary spawns "go build -buildmode=c-shared -o ..."
	f.When(MatchCommand("go")).Then("", "", nil)

	if err := b.buildGoLibrary(); err != nil {
		t.Fatalf("buildGoLibrary: %v", err)
	}

	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Path != "go" {
		t.Fatalf("expected 'go', got %q", calls[0].Path)
	}
	// Verify it's using c-shared build mode
	if calls[0].Args[0] != "build" {
		t.Fatalf("expected build subcommand, got %v", calls[0].Args)
	}

	// Verify the CC env is set
	ccSet := false
	for _, e := range calls[0].Env {
		if e == "GOOS=android" {
			ccSet = true
			break
		}
	}
	if !ccSet {
		t.Fatal("expected GOOS=android in environment")
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

	_, err := b.compileResources()
	if err == nil {
		t.Fatal("expected error when aapt2 is missing")
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

	err := b.buildGoLibrary()
	if err == nil {
		t.Fatal("expected error for missing main.go, got nil")
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
