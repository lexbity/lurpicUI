package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// defaultSDKSearchPaths returns the default SDK search paths for the current OS.
func defaultSDKSearchPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"$HOME/Library/Android/sdk",
			"/opt/android-sdk",
		}
	case "linux":
		return []string{
			"$HOME/Android/Sdk",
			"/usr/lib/android-sdk",
			"/opt/android-sdk",
		}
	case "windows":
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
	if runtime.GOOS == "windows" {
		requiredFiles[0] = "platform-tools/adb.exe"
	}

	for _, file := range requiredFiles {
		fullPath := filepath.Join(path, file)
		if _, err := os.Stat(fullPath); err != nil {
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

// isValidNDK checks if a directory looks like a valid Android NDK
func isValidNDK(path string) bool {
	// Check for key NDK components
	requiredFiles := []string{
		"ndk-build",
		"toolchains",
		"platforms",
	}

	// On Windows, ndk-build has .cmd extension
	if runtime.GOOS == "windows" {
		requiredFiles[0] = "ndk-build.cmd"
	}

	for _, file := range requiredFiles {
		fullPath := filepath.Join(path, file)
		if _, err := os.Stat(fullPath); err != nil {
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
				if runtime.GOOS == "windows" {
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
	if runtime.GOOS == "windows" {
		candidate += ".exe"
	}
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	// Check cmdline-tools
	cmdlineTools := filepath.Join(sdk, "cmdline-tools", "latest", "bin", tool)
	if runtime.GOOS == "windows" {
		cmdlineTools += ".bat"
	}
	if _, err := os.Stat(cmdlineTools); err == nil {
		return cmdlineTools, nil
	}

	return "", fmt.Errorf("tool '%s' not found in SDK", tool)
}

// findEmulatorTool finds the emulator binary in the SDK.
func findEmulatorTool(sdk string) (string, error) {
	candidate := filepath.Join(sdk, "emulator", "emulator")
	if runtime.GOOS == "windows" {
		candidate += ".exe"
	}
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", fmt.Errorf("emulator binary not found in Android SDK")
}

// findCmdlineTool finds a tool in the SDK's cmdline-tools directory.
func findCmdlineTool(sdk, tool string) (string, error) {
	candidate := filepath.Join(sdk, "cmdline-tools", "latest", "bin", tool)
	if runtime.GOOS == "windows" {
		candidate += ".bat"
	}
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", fmt.Errorf("tool '%s' not found in SDK cmdline-tools", tool)
}
