package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ToolchainDetector holds all the sources for toolchain detection.
// Precedence (highest to lowest):
// 1. Command-line flags (FlagSDK, FlagNDK, FlagJDK)
// 2. Project config (Config.Android.SDK.Path, etc.)
// 3. User config (UserConfig.Android.SDKPath, etc.)
// 4. Environment variables (ANDROID_HOME, ANDROID_NDK_HOME, JAVA_HOME)
// 5. Auto-detection (common install paths)
type ToolchainDetector struct {
	// Runner for executing external commands (optional; used only for PATH lookups).
	Runner Runner

	// Command-line flags (highest priority)
	FlagSDK string
	FlagNDK string
	FlagJDK string

	// Project config (from lurpic.toml)
	Config *Config

	// User config (from ~/.config/lurpic/config.toml)
	UserConfig *UserConfig
}

// DetectSDK finds the Android SDK using the full precedence chain.
func (d *ToolchainDetector) DetectSDK() (string, string, error) {
	sources := []struct {
		name string
		path string
	}{
		{"command-line flag", d.FlagSDK},
		{"project config", ""},
		{"user config", ""},
		{"environment variable", ""},
		{"auto-detection", ""},
	}

	// Populate project config source
	if d.Config != nil && d.Config.Android.SDK.Path != "" {
		sources[1].path = d.Config.Android.SDK.Path
	}

	// Populate user config source
	if d.UserConfig != nil && d.UserConfig.Android.SDKPath != "" {
		sources[2].path = d.UserConfig.Android.SDKPath
	}

	// Populate environment source
	if env := os.Getenv("ANDROID_HOME"); env != "" {
		sources[3].path = env
	} else if env := os.Getenv("ANDROID_SDK"); env != "" {
		sources[3].path = env
	}

	// Try each source in order
	for _, source := range sources {
		if source.path == "" {
			continue
		}

		if isValidSDK(source.path) {
			return source.path, source.name, nil
		}

		// If explicitly specified but invalid, report error
		if source.name != "auto-detection" && source.name != "environment variable" {
			return "", "", fmt.Errorf("SDK from %s (%s) is not valid", source.name, source.path)
		}
	}

	// Auto-detection: check common paths
	for _, path := range defaultSDKSearchPaths() {
		path = os.ExpandEnv(path)
		if isValidSDK(path) {
			return path, "auto-detection", nil
		}
	}

	return "", "", fmt.Errorf("Android SDK not found\n\nTo fix this, you can (in order of recommendation):\n1. Set ANDROID_HOME environment variable\n2. Add to your user config: lurpic config set android.sdk-path /path/to/sdk\n3. Add to your project lurpic.toml: [android.sdk] path = \"/path/to/sdk\"\n4. Use command-line flag: --sdk-path /path/to/sdk")
}

// DetectNDK finds the Android NDK using the full precedence chain.
func (d *ToolchainDetector) DetectNDK(sdk string) (string, string, error) {
	sources := []struct {
		name string
		path string
	}{
		{"command-line flag", d.FlagNDK},
		{"project config", ""},
		{"user config", ""},
		{"environment variable", ""},
		{"auto-detection (in SDK)", ""},
	}

	// Populate project config source
	if d.Config != nil && d.Config.Android.NDK.Path != "" {
		sources[1].path = d.Config.Android.NDK.Path
	}

	// Populate user config source
	if d.UserConfig != nil && d.UserConfig.Android.NDKPath != "" {
		sources[2].path = d.UserConfig.Android.NDKPath
	}

	// Populate environment source
	if env := os.Getenv("ANDROID_NDK_HOME"); env != "" {
		sources[3].path = env
	} else if env := os.Getenv("NDK_HOME"); env != "" {
		sources[3].path = env
	}

	// Try each source in order
	for _, source := range sources {
		if source.path == "" {
			continue
		}

		if isValidNDK(source.path) {
			return source.path, source.name, nil
		}

		// If explicitly specified but invalid, report error
		if source.name != "auto-detection (in SDK)" && source.name != "environment variable" {
			return "", "", fmt.Errorf("NDK from %s (%s) is not valid", source.name, source.path)
		}
	}

	// Auto-detection: check if NDK is inside the SDK
	if sdk != "" {
		// Check for NDK inside SDK (common layout)
		ndkInSdk := filepath.Join(sdk, "ndk")
		if entries, err := os.ReadDir(ndkInSdk); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					candidate := filepath.Join(ndkInSdk, entry.Name())
					if isValidNDK(candidate) {
						return candidate, "auto-detection (in SDK)", nil
					}
				}
			}
		}

		// Check for ndk-bundle (older SDK layout)
		ndkBundle := filepath.Join(sdk, "ndk-bundle")
		if isValidNDK(ndkBundle) {
			return ndkBundle, "auto-detection (ndk-bundle)", nil
		}
	}

	return "", "", fmt.Errorf("Android NDK not found\n\nTo fix this, you can (in order of recommendation):\n1. Set ANDROID_NDK_HOME environment variable\n2. Add to your user config: lurpic config set android.ndk-path /path/to/ndk\n3. Install NDK via Android Studio SDK Manager")
}

// DetectJDK finds the JDK using the full precedence chain.
func (d *ToolchainDetector) DetectJDK() (string, string, error) {
	sources := []struct {
		name string
		path string
	}{
		{"command-line flag", d.FlagJDK},
		{"project config", ""},
		{"user config", ""},
		{"environment variable", ""},
		{"auto-detection", ""},
	}

	// Populate project config source
	if d.Config != nil && d.Config.Android.JDK.Path != "" {
		sources[1].path = d.Config.Android.JDK.Path
	}

	// Populate user config source
	if d.UserConfig != nil && d.UserConfig.Android.JDKPath != "" {
		sources[2].path = d.UserConfig.Android.JDKPath
	}

	// Populate environment source
	if env := os.Getenv("JAVA_HOME"); env != "" {
		sources[3].path = env
	}

	// Try each source in order
	for _, source := range sources {
		if source.path == "" {
			continue
		}

		if isValidJDK(source.path) {
			return source.path, source.name, nil
		}

		// If explicitly specified but invalid, report error
		if source.name != "auto-detection" && source.name != "environment variable" {
			return "", "", fmt.Errorf("JDK from %s (%s) is not valid", source.name, source.path)
		}
	}

	// Auto-detection: check common paths
	for _, path := range defaultJDKSearchPaths() {
		path = os.ExpandEnv(path)
		if isValidJDK(path) {
			return path, "auto-detection", nil
		}
	}

	// Check if java is in PATH and try to find JAVA_HOME from it
	var javaPath string
	if d.Runner != nil {
		javaPath, _ = d.Runner.Look("java")
	} else {
		javaPath, _ = lookPathFallback("java")
	}
	if javaPath != "" {
		javaPath, _ = filepath.EvalSymlinks(javaPath)
		possibleJDK := filepath.Dir(filepath.Dir(javaPath))
		if isValidJDK(possibleJDK) {
			return possibleJDK, "auto-detection (from PATH)", nil
		}
	}

	return "", "", fmt.Errorf("JDK not found\n\nTo fix this, you can (in order of recommendation):\n1. Set JAVA_HOME environment variable\n2. Add to your user config: lurpic config set android.jdk-path /path/to/jdk\n3. Install JDK via your package manager or from https://adoptium.net/")
}

// lookPathFallback finds an executable on PATH without relying on os/exec.
func lookPathFallback(name string) (string, error) {
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		candidate := filepath.Join(dir, name)
		if runtime.GOOS == "windows" {
			candidate += ".exe"
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("executable %q not found on PATH", name)
}

// isValidJDK checks if a directory looks like a valid JDK
func isValidJDK(path string) bool {
	if path == "" {
		return false
	}

	// Check for key JDK components
	javaBin := filepath.Join(path, "bin", "java")
	javacBin := filepath.Join(path, "bin", "javac")

	if runtime.GOOS == "windows" {
		javaBin += ".exe"
		javacBin += ".exe"
	}

	// java is required
	if _, err := os.Stat(javaBin); err != nil {
		return false
	}

	return true
}

// defaultJDKSearchPaths returns the default JDK search paths for the current OS.
func defaultJDKSearchPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Library/Java/JavaVirtualMachines/*/Contents/Home",
			"/System/Library/Java/JavaVirtualMachines/*/Contents/Home",
			"/usr/local/opt/openjdk",
			"/opt/homebrew/opt/openjdk",
		}
	case "linux":
		return []string{
			"/usr/lib/jvm/default-java",
			"/usr/lib/jvm/java-*-openjdk*",
			"/usr/lib/jvm/java-*-oracle*",
			"/usr/local/java/jdk*",
		}
	case "windows":
		return []string{
			"C:\\Program Files\\Java\\jdk*",
			"C:\\Program Files\\Eclipse Adoptium\\jdk*",
		}
	}
	return nil
}

// GetToolchainReport returns a report of all detected toolchains for diagnostics.
func (d *ToolchainDetector) GetToolchainReport() (*ToolchainReport, error) {
	report := &ToolchainReport{}

	// Detect SDK
	sdk, source, err := d.DetectSDK()
	if err != nil {
		report.SDK = ToolchainStatus{Error: err.Error()}
	} else {
		report.SDK = ToolchainStatus{Path: sdk, Source: source, OK: true}
	}

	// Detect NDK (needs SDK path for auto-detection)
	ndk, source, err := d.DetectNDK(sdk)
	if err != nil {
		report.NDK = ToolchainStatus{Error: err.Error()}
	} else {
		report.NDK = ToolchainStatus{Path: ndk, Source: source, OK: true}
	}

	// Detect JDK
	jdk, source, err := d.DetectJDK()
	if err != nil {
		report.JDK = ToolchainStatus{Error: err.Error()}
	} else {
		report.JDK = ToolchainStatus{Path: jdk, Source: source, OK: true}
	}

	return report, nil
}

// ToolchainReport contains the detection status of all toolchains.
type ToolchainReport struct {
	SDK ToolchainStatus `json:"sdk"`
	NDK ToolchainStatus `json:"ndk"`
	JDK ToolchainStatus `json:"jdk"`
}

// ToolchainStatus represents the status of a single toolchain.
type ToolchainStatus struct {
	OK     bool   `json:"ok"`
	Path   string `json:"path,omitempty"`
	Source string `json:"source,omitempty"`
	Error  string `json:"error,omitempty"`
}

// CanBuild reports whether all required toolchains are available.
func (r *ToolchainReport) CanBuild() bool {
	return r.SDK.OK && r.NDK.OK && r.JDK.OK
}

// String returns a formatted report string.
func (r *ToolchainReport) String() string {
	var sb strings.Builder

	printStatus := func(name string, status ToolchainStatus) {
		if status.OK {
			sb.WriteString(fmt.Sprintf("✓ %s at %s\n", name, status.Path))
			sb.WriteString(fmt.Sprintf("  (found via: %s)\n", status.Source))
		} else {
			sb.WriteString(fmt.Sprintf("✗ %s not found\n", name))
			sb.WriteString(fmt.Sprintf("  %s\n", status.Error))
		}
	}

	printStatus("Android SDK", r.SDK)
	printStatus("Android NDK", r.NDK)
	printStatus("JDK", r.JDK)

	if r.CanBuild() {
		sb.WriteString("\nReady to build for Android.\n")
	} else {
		sb.WriteString("\nCannot build for Android until issues are resolved.\n")
	}

	return sb.String()
}
