package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

// defaultSDKSearchPaths returns the default SDK search paths for the current OS.
func defaultSDKSearchPaths() []string {
	switch runtime.GOOS {
	case platformDarwin:
		return []string{
			"$HOME/Library/Android/sdk",
			"/opt/android-sdk",
		}
	case platformLinux:
		return []string{
			"$HOME/Android/Sdk",
			"/usr/lib/android-sdk",
			"/opt/android-sdk",
		}
	case platformWindows:
		return []string{
			"%LOCALAPPDATA%\\Android\\Sdk",
			"C:\\Android\\Sdk",
		}
	}
	return nil
}

// detectAndroidSDK finds the Android SDK from ANDROID_HOME environment variable
// or common installation paths.
func detectAndroidSDK() (string, error) {
	return detectAndroidSDKWithPaths(defaultSDKSearchPaths())
}

// detectAndroidSDKWithPaths finds the Android SDK using the provided search paths.
// This allows tests to inject controlled paths while production uses default paths.
func detectAndroidSDKWithPaths(commonPaths []string) (string, error) {
	// Check environment variable first
	if sdk := os.Getenv("ANDROID_HOME"); sdk != "" {
		if isValidSDK(sdk) {
			return sdk, nil
		}
		return "", fmt.Errorf("ANDROID_HOME (%s) does not point to a valid Android SDK", sdk)
	}

	if sdk := os.Getenv("ANDROID_SDK"); sdk != "" {
		if isValidSDK(sdk) {
			return sdk, nil
		}
		return "", fmt.Errorf("ANDROID_SDK (%s) does not point to a valid Android SDK", sdk)
	}

	// Check common installation paths
	for _, path := range commonPaths {
		path = os.ExpandEnv(path)
		if isValidSDK(path) {
			return path, nil
		}
	}

	return "", fmt.Errorf("Android SDK not found. Set ANDROID_HOME environment variable to your SDK path")
}

// isValidSDK checks if a directory looks like a valid Android SDK
func isValidSDK(path string) bool {
	// Check for key SDK components
	requiredFiles := []string{
		"platform-tools/adb",
		"build-tools",
		"platforms",
	}

	// On Windows, adb has .exe extension
	if runtime.GOOS == platformWindows {
		requiredFiles[0] = "platform-tools/adb.exe"
	}

	for _, file := range requiredFiles {
		fullPath := filepath.Join(path, file)
		if _, err := os.Stat(fullPath); err != nil { //nolint:gosec // path from user config
			return false
		}
	}

	return true
}

// detectAndroidNDK finds the Android NDK from ANDROID_NDK_HOME or infers it from SDK
func detectAndroidNDK(sdk string) (string, error) {
	// Check environment variable first
	if ndk := os.Getenv("ANDROID_NDK_HOME"); ndk != "" {
		if isValidNDK(ndk) {
			return ndk, nil
		}
		return "", fmt.Errorf("ANDROID_NDK_HOME (%s) does not point to a valid Android NDK", ndk)
	}

	if ndk := os.Getenv("NDK_HOME"); ndk != "" {
		if isValidNDK(ndk) {
			return ndk, nil
		}
		return "", fmt.Errorf("NDK_HOME (%s) does not point to a valid Android NDK", ndk)
	}

	// Check if NDK is inside the SDK (common layout)
	ndkInSdk := filepath.Join(sdk, "ndk")
	if entries, err := os.ReadDir(ndkInSdk); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				candidate := filepath.Join(ndkInSdk, entry.Name())
				if isValidNDK(candidate) {
					return candidate, nil
				}
			}
		}
	}

	// Check for ndk-bundle (older SDK layout)
	ndkBundle := filepath.Join(sdk, "ndk-bundle")
	if isValidNDK(ndkBundle) {
		return ndkBundle, nil
	}

	return "", fmt.Errorf("Android NDK not found. Set ANDROID_NDK_HOME or install NDK via SDK Manager")
}

// isValidNDK checks if a directory looks like a valid Android NDK.
//
// Validity is keyed on the markers that both identify an NDK package and that
// the build actually depends on, rather than legacy layout. source.properties
// is the canonical NDK package descriptor (carries Pkg.Revision), and
// toolchains/llvm/prebuilt holds the Clang toolchain the cross-compile resolves
// via findNDKToolchain. The "platforms" directory is intentionally NOT checked:
// it was removed in NDK r19 (2019) when the per-API sysroot was unified into
// toolchains/llvm/prebuilt/<host>/sysroot, so requiring it rejects every modern NDK.
func isValidNDK(path string) bool {
	requiredFiles := []string{
		"source.properties",
		filepath.Join("toolchains", "llvm", "prebuilt"),
	}

	for _, file := range requiredFiles {
		fullPath := filepath.Join(path, file)
		if _, err := os.Stat(fullPath); err != nil { //nolint:gosec // path from user config
			return false
		}
	}

	return true
}

// findSDKTool locates a specific tool in the Android SDK
func findSDKTool(sdk, tool string) (string, error) {
	// Check build-tools directories (versioned)
	buildToolsDir := filepath.Join(sdk, "build-tools")
	entries, err := os.ReadDir(buildToolsDir)
	if err == nil {
		// Sort entries to get newest version first
		for i := len(entries) - 1; i >= 0; i-- {
			if entries[i].IsDir() {
				candidate := filepath.Join(buildToolsDir, entries[i].Name(), tool)
				if runtime.GOOS == platformWindows {
					candidate += ".exe"
				}
				if _, err := os.Stat(candidate); err == nil {
					return candidate, nil
				}
			}
		}
	}

	// Check platform-tools
	candidate := filepath.Join(sdk, "platform-tools", tool)
	if runtime.GOOS == platformWindows {
		candidate += ".exe"
	}
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	// Check cmdline-tools (sdkmanager, avdmanager, ...) and the legacy tools/bin.
	if found, err := findCmdlineTool(sdk, tool); err == nil {
		return found, nil
	}

	return "", fmt.Errorf("tool '%s' not found in SDK", tool)
}

// findEmulatorTool finds the emulator binary in the SDK.
func findEmulatorTool(sdk string) (string, error) {
	candidate := filepath.Join(sdk, "emulator", "emulator")
	if runtime.GOOS == platformWindows {
		candidate += ".exe"
	}
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", fmt.Errorf("emulator binary not found in Android SDK")
}

// findCmdlineTool finds a tool (e.g. sdkmanager, avdmanager) from the "Android
// SDK Command-line Tools" package. These live under cmdline-tools/<channel>/bin,
// where <channel> is "latest" or a version like "13.0"; older SDKs placed them in
// tools/bin. "latest" is preferred, then versioned directories in descending
// order, then the legacy location.
func findCmdlineTool(sdk, tool string) (string, error) {
	name := tool
	if runtime.GOOS == platformWindows {
		name += ".bat"
	}

	var binDirs []string
	cmdlineRoot := filepath.Join(sdk, "cmdline-tools")
	if entries, err := os.ReadDir(cmdlineRoot); err == nil {
		var versioned []string
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if e.Name() == "latest" {
				binDirs = append(binDirs, filepath.Join(cmdlineRoot, "latest", "bin"))
			} else {
				versioned = append(versioned, e.Name())
			}
		}
		sort.Sort(sort.Reverse(sort.StringSlice(versioned)))
		for _, v := range versioned {
			binDirs = append(binDirs, filepath.Join(cmdlineRoot, v, "bin"))
		}
	}
	binDirs = append(binDirs, filepath.Join(sdk, "tools", "bin")) // legacy layout

	for _, dir := range binDirs {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("tool %q not found; install the \"Android SDK Command-line Tools\" package (e.g. via Android Studio's SDK Manager, or sdkmanager \"cmdline-tools;latest\")", tool)
}
