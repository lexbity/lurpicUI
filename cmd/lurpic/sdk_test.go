package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsValidSDK(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Empty directory should not be valid
	if isValidSDK(tmpDir) {
		t.Error("empty directory should not be a valid SDK")
	}

	// Create required directories
	os.MkdirAll(filepath.Join(tmpDir, "platform-tools"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "build-tools"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "platforms"), 0755)

	// Create adb executable
	adb := "adb"
	if runtime.GOOS == "windows" {
		adb = "adb.exe"
	}
	os.WriteFile(filepath.Join(tmpDir, "platform-tools", adb), []byte(""), 0755)

	// Now it should be valid
	if !isValidSDK(tmpDir) {
		t.Error("directory with SDK structure should be valid")
	}
}

func TestIsValidNDK(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Empty directory should not be valid
	if isValidNDK(tmpDir) {
		t.Error("empty directory should not be a valid NDK")
	}

	// Create the markers a modern NDK (r19+) actually carries: the
	// source.properties package descriptor and the LLVM prebuilt toolchain.
	os.MkdirAll(filepath.Join(tmpDir, "toolchains", "llvm", "prebuilt"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "source.properties"), []byte("Pkg.Revision = 30.0.14904198\n"), 0644)

	// Now it should be valid
	if !isValidNDK(tmpDir) {
		t.Error("directory with NDK structure should be valid")
	}
}

func TestFindProjectRoot(t *testing.T) {
	// Create a temporary project structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "myproject")
	os.MkdirAll(projectDir, 0755)

	// Create lurpic.toml
	configContent := `[app]
id = "com.test.app"
name = "Test App"
`
	os.WriteFile(filepath.Join(projectDir, "lurpic.toml"), []byte(configContent), 0644)

	// Create a subdirectory
	subDir := filepath.Join(projectDir, "cmd", "app")
	os.MkdirAll(subDir, 0755)

	// Change to subdirectory and find project root
	originalWd, _ := os.Getwd()
	os.Chdir(subDir)
	defer os.Chdir(originalWd)

	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot failed: %v", err)
	}

	// Should find the project directory (not the subdirectory)
	absProject, _ := filepath.Abs(projectDir)
	absRoot, _ := filepath.Abs(root)
	if absRoot != absProject {
		t.Errorf("expected project root %s, got %s", absProject, absRoot)
	}
}

func TestFindProjectRoot_NoConfig(t *testing.T) {
	// Create a temporary directory without config
	tmpDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	_, err = findProjectRoot()
	if err == nil {
		t.Error("expected error when no lurpic.toml found")
	}
}

func TestDetectAndroidSDK_NotSet(t *testing.T) {
	// Clear environment variables
	originalHome := os.Getenv("ANDROID_HOME")
	originalSdk := os.Getenv("ANDROID_SDK")
	defer func() {
		os.Setenv("ANDROID_HOME", originalHome)
		os.Setenv("ANDROID_SDK", originalSdk)
	}()
	os.Unsetenv("ANDROID_HOME")
	os.Unsetenv("ANDROID_SDK")

	// Use empty paths to ensure no system SDK is found
	_, err := detectAndroidSDKWithPaths([]string{})
	if err == nil {
		t.Error("expected error when ANDROID_HOME not set and no common paths exist")
	}
}

func TestDetectAndroidSDK_FromEnv(t *testing.T) {
	// Create a fake SDK
	tmpDir := t.TempDir()

	// Create SDK structure
	os.MkdirAll(filepath.Join(tmpDir, "platform-tools"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "build-tools"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "platforms"), 0755)

	adb := "adb"
	if runtime.GOOS == "windows" {
		adb = "adb.exe"
	}
	os.WriteFile(filepath.Join(tmpDir, "platform-tools", adb), []byte(""), 0755)

	// Set environment variable
	os.Setenv("ANDROID_HOME", tmpDir)

	sdk, err := detectAndroidSDK()
	if err != nil {
		t.Fatalf("detectAndroidSDK failed: %v", err)
	}

	if sdk != tmpDir {
		t.Errorf("expected SDK path %s, got %s", tmpDir, sdk)
	}
}

func TestDetectAndroidNDK_FromEnv(t *testing.T) {
	// Create a fake NDK
	tmpDir := t.TempDir()

	// Create NDK structure (modern r19+ layout)
	os.MkdirAll(filepath.Join(tmpDir, "toolchains", "llvm", "prebuilt"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "source.properties"), []byte("Pkg.Revision = 30.0.14904198\n"), 0644)

	// Set environment variable
	os.Setenv("ANDROID_NDK_HOME", tmpDir)

	ndk, err := detectAndroidNDK(tmpDir)
	if err != nil {
		t.Fatalf("detectAndroidNDK failed: %v", err)
	}

	if ndk != tmpDir {
		t.Errorf("expected NDK path %s, got %s", tmpDir, ndk)
	}
}

func TestDetectAndroidNDK_InsideSDK(t *testing.T) {
	// Create a fake SDK with NDK inside
	sdkDir := t.TempDir()
	ndkVersion := "25.2.9519653"
	ndkDir := filepath.Join(sdkDir, "ndk", ndkVersion)

	// Create NDK structure (modern r19+ layout)
	os.MkdirAll(filepath.Join(ndkDir, "toolchains", "llvm", "prebuilt"), 0755)
	os.WriteFile(filepath.Join(ndkDir, "source.properties"), []byte("Pkg.Revision = 30.0.14904198\n"), 0644)

	// Clear NDK environment variables
	os.Unsetenv("ANDROID_NDK_HOME")
	os.Unsetenv("NDK_HOME")

	ndk, err := detectAndroidNDK(sdkDir)
	if err != nil {
		t.Fatalf("detectAndroidNDK failed: %v", err)
	}

	if ndk != ndkDir {
		t.Errorf("expected NDK path %s, got %s", ndkDir, ndk)
	}
}
