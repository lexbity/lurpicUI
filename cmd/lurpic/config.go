package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config represents the lurpic.toml configuration file
type Config struct {
	App     AppConfig     `toml:"app"`
	Android AndroidConfig `toml:"android"`
}

// AppConfig contains general application settings
type AppConfig struct {
	ID      string `toml:"id"`
	Name    string `toml:"name"`
	Version string `toml:"version"`
	Icon    string `toml:"icon"`
	// Main is the Go package to cross-compile, relative to the project root
	// (e.g. "cmd/quick_square_app"). It is independent of ID, which is the
	// Android applicationId. Defaults to "." (the project root package).
	Main string `toml:"main"`
}

// IconConfig contains icon paths for different densities
type IconConfig struct {
	LDPI    string `toml:"ldpi"`
	MDPI    string `toml:"mdpi"`
	HDPI    string `toml:"hdpi"`
	XHDPI   string `toml:"xhdpi"`
	XXHDPI  string `toml:"xxhdpi"`
	XXXHDPI string `toml:"xxxhdpi"`
}

// AndroidConfig contains Android-specific settings
type AndroidConfig struct {
	MinSDK      int              `toml:"min_sdk"`
	TargetSDK   int              `toml:"target_sdk"`
	VersionCode int              `toml:"version_code"`
	ABIs        []string         `toml:"abis"`
	Permissions PermissionConfig `toml:"permissions"`
	Keystore    KeystoreConfig   `toml:"keystore"`
	SDK         SDKConfig        `toml:"sdk"`
	NDK         NDKConfig        `toml:"ndk"`
	JDK         JDKConfig        `toml:"jdk"`
	Assets      AssetConfig      `toml:"assets"`
	// NetworkSecurityConfig optionally points to an XML resource file
	// that configures Android's network security policy (cleartext traffic,
	// certificate pinning, etc.). If set, the manifest references it via
	// android:networkSecurityConfig.
	NetworkSecurityConfig string `toml:"network_security_config,omitempty"`
}

// AssetConfig controls how project assets are packaged.
type AssetConfig struct {
	// NoCompress lists glob patterns for files that should be stored
	// uncompressed in the APK. Patterns are matched against the asset
	// path relative to the assets directory. Example: ["*.mp4", "*.png"]
	NoCompress []string `toml:"no_compress"`
	// Packs defines named asset packs for Play Asset Delivery.
	// Each pack becomes a separate AAB module with its own delivery type.
	// Example:
	//
	//	[[android.assets.packs]]
	//	name = "media"
	//	delivery = "install_time"
	Packs []AssetPackConfig `toml:"packs"`
}

// AssetPackConfig defines one Play Asset Delivery pack.
type AssetPackConfig struct {
	// Name is the pack identifier used in the AAB module and at runtime.
	Name string `toml:"name"`
	// Delivery is one of "install_time", "fast_follow", or "on_demand".
	Delivery string `toml:"delivery"`
}

// SDKConfig contains Android SDK path configuration and optional version pins
// for reproducible builds.
type SDKConfig struct {
	Path    string `toml:"path"`
	Version string `toml:"version,omitempty"` // pinned SDK version (e.g. "35")
}

// NDKConfig contains Android NDK path configuration and optional version pins
// for reproducible builds.
type NDKConfig struct {
	Path    string `toml:"path"`
	Version string `toml:"version,omitempty"` // pinned NDK version (e.g. "27.0.12077973")
}

// JDKConfig contains JDK path configuration for Java tooling and optional
// version pins for reproducible builds.
type JDKConfig struct {
	Path    string `toml:"path"`
	Version string `toml:"version,omitempty"` // pinned JDK version (e.g. "17")
}

// KeystoreConfig contains release signing configuration.
// The password must be supplied via --ks-pass flag, LURPIC_KEYSTORE_PASSWORD env,
// or an interactive secure prompt — never stored in lurpic.toml.
type KeystoreConfig struct {
	Path  string `toml:"path"`
	Alias string `toml:"alias"`
}

// HasIcon returns true if any icon path is configured
func (c *AppConfig) HasIcon() bool {
	return c.Icon != ""
}

// PermissionConfig defines Android permissions
type PermissionConfig struct {
	Required []string `toml:"required"`
	Optional []string `toml:"optional"`
}

func loadConfig(projectRoot string) (*Config, error) {
	configPath := filepath.Join(projectRoot, "lurpic.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}

	var config Config
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("cannot parse config file: %w", err)
	}

	// Set defaults
	if config.App.Version == "" {
		config.App.Version = "1.0.0"
	}
	if config.Android.MinSDK == 0 {
		config.Android.MinSDK = 24
	}
	if config.Android.TargetSDK == 0 {
		config.Android.TargetSDK = 36
	}

	if config.Android.ABIs == nil {
		// arm64-v8a is the mandatory release ABI. x86_64 is available for
		// emulator and Chromebook testing but must be explicitly configured.
		config.Android.ABIs = []string{"arm64-v8a"}
	}

	// *.pak files must be stored uncompressed so the APK fd can be mmap'd
	// directly for zero-copy asset access. Compressed paks require extraction.
	if config.Android.Assets.NoCompress == nil {
		config.Android.Assets.NoCompress = []string{"*.pak"}
	}

	// Derive versionCode from semver if not explicitly set
	if config.Android.VersionCode == 0 {
		code, err := deriveVersionCode(config.App.Version)
		if err != nil {
			return nil, fmt.Errorf("app.version %q: %w", config.App.Version, err)
		}
		config.Android.VersionCode = code
	}

	// Normalize the Go entrypoint package. Default to the project root package.
	if config.App.Main == "" {
		config.App.Main = "."
	} else {
		config.App.Main = filepath.Clean(config.App.Main)
		if filepath.IsAbs(config.App.Main) {
			return nil, fmt.Errorf("app.main %q must be a path relative to the project root", config.App.Main)
		}
		if config.App.Main == ".." || strings.HasPrefix(config.App.Main, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("app.main %q must not escape the project root", config.App.Main)
		}
	}

	// Validate required fields
	if config.App.ID == "" {
		return nil, fmt.Errorf("app.id is required in lurpic.toml")
	}
	if err := validateAndroidPackageName(config.App.ID); err != nil {
		return nil, fmt.Errorf("app.id %q is not a valid Android applicationId: %w", config.App.ID, err)
	}
	if config.App.Name == "" {
		return nil, fmt.Errorf("app.name is required in lurpic.toml")
	}

	return &config, nil
}

// checkToolchainPins verifies that the configured SDK, NDK, and build-tools
// versions match the pinned versions in config (when set). Returns a list of
// warnings; an empty list means all pins match.
func checkToolchainPins(config *Config, sdkPath, ndkPath string) []string {
	var warnings []string

	// Check SDK version pin.
	if config != nil && config.Android.SDK.Version != "" {
		// SDK versions are stored in platforms/android-<api>/source.properties
		sdkProp := filepath.Join(sdkPath, "platforms", fmt.Sprintf("android-%d", config.Android.TargetSDK), "source.properties")
		if prop, err := os.ReadFile(sdkProp); err == nil {
			if !strings.Contains(string(prop), "Pkg.Revision="+config.Android.SDK.Version) {
				warnings = append(warnings, fmt.Sprintf(
					"SDK platform version %s configured but installed version does not match (check %s)",
					config.Android.SDK.Version, sdkProp,
				))
			}
		}
	}

	// Check NDK version pin.
	if config != nil && config.Android.NDK.Version != "" {
		ndkProp := filepath.Join(ndkPath, "source.properties")
		if prop, err := os.ReadFile(ndkProp); err == nil {
			if !strings.Contains(string(prop), "Pkg.Revision="+config.Android.NDK.Version) {
				warnings = append(warnings, fmt.Sprintf(
					"NDK version %s configured but installed version does not match (check %s)",
					config.Android.NDK.Version, ndkProp,
				))
			}
		}
	}

	// Check build-tools version pin (inferred from SDK path).
	if config != nil && config.Android.TargetSDK > 0 {
		btDir := filepath.Join(sdkPath, "build-tools", fmt.Sprintf("%d.0.0", config.Android.TargetSDK))
		if _, err := os.Stat(btDir); os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf(
				"build-tools %d.0.0 not found at %s (install via sdkmanager \"build-tools;%d.0.0\")",
				config.Android.TargetSDK, btDir, config.Android.TargetSDK,
			))
		}
	}

	return warnings
}

// validateAndroidConfigForRelease checks that the Android config meets
// Google Play policy requirements. Returns an error suitable for display
// during release builds.
func validateAndroidConfigForRelease(config *Config) error {
	if config == nil {
		return fmt.Errorf("android config is nil")
	}
	if config.Android.TargetSDK < 35 {
		return fmt.Errorf(
			"android.target_sdk = %d is too low for release builds; "+
				"Google Play requires targetSdk >= 35 (set android.target_sdk to 35 or higher in lurpic.toml)",
			config.Android.TargetSDK,
		)
	}
	if config.Android.MinSDK < 21 {
		return fmt.Errorf(
			"android.min_sdk = %d is too low; lurpicUI requires at least API 21 (Android 5.0)",
			config.Android.MinSDK,
		)
	}
	if config.Android.VersionCode <= 0 {
		return fmt.Errorf("android.version_code must be positive (got %d)", config.Android.VersionCode)
	}
	if config.App.ID == "" {
		return fmt.Errorf("app.id is required for release builds")
	}
	if config.App.Name == "" {
		return fmt.Errorf("app.name is required for release builds")
	}
	if !containsABI(config.Android.ABIs, "arm64-v8a") {
		return fmt.Errorf(
			"release builds must include arm64-v8a in android.abis; "+
				"x86_64 is available for emulator/Chromebook testing but must be explicitly added",
		)
	}
	return nil
}

// validateAndroidPackageName enforces the Android applicationId / manifest
// package rules: at least two dot-separated segments, each starting with a
// letter and containing only letters, digits, or underscores. aapt2 rejects
// anything else, so this fails fast with a clear message instead of deep in the
// build.
func containsABI(abis []string, target string) bool {
	for _, a := range abis {
		if a == target {
			return true
		}
	}
	return false
}

func validateAndroidPackageName(id string) error {
	segments := strings.Split(id, ".")
	if len(segments) < 2 {
		return fmt.Errorf("must contain at least two segments separated by '.' (e.g. org.example.app)")
	}
	for _, seg := range segments {
		if seg == "" {
			return fmt.Errorf("segments must not be empty")
		}
		for i, r := range seg {
			isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
			isDigit := r >= '0' && r <= '9'
			if i == 0 && !isLetter {
				return fmt.Errorf("segment %q must start with a letter", seg)
			}
			if !isLetter && !isDigit && r != '_' {
				return fmt.Errorf("segment %q contains invalid character %q (allowed: letters, digits, underscore)", seg, r)
			}
		}
	}
	return nil
}

// deriveVersionCode parses a semver string and produces a monotonic version code.
// Format: "major.minor.patch" → major*1_000_000 + minor*1_000 + patch.
// Returns an error if the version is not valid semver or any component exceeds 999.
func deriveVersionCode(version string) (int, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return 0, fmt.Errorf("expected semver \"major.minor.patch\", got %q", version)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("major version %q is not a number", parts[0])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("minor version %q is not a number", parts[1])
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, fmt.Errorf("patch version %q is not a number", parts[2])
	}
	if major < 0 || major > 999 {
		return 0, fmt.Errorf("major version %d out of range (0-999)", major)
	}
	if minor < 0 || minor > 999 {
		return 0, fmt.Errorf("minor version %d out of range (0-999)", minor)
	}
	if patch < 0 || patch > 999 {
		return 0, fmt.Errorf("patch version %d out of range (0-999)", patch)
	}
	return major*1_000_000 + minor*1_000 + patch, nil
}
