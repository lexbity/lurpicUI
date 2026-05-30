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
}

// SDKConfig contains Android SDK path configuration
type SDKConfig struct {
	Path string `toml:"path"`
}

// NDKConfig contains Android NDK path configuration
type NDKConfig struct {
	Path string `toml:"path"`
}

// JDKConfig contains JDK path configuration for Java tooling
type JDKConfig struct {
	Path string `toml:"path"`
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
		config.Android.MinSDK = 29
	}
	if config.Android.TargetSDK == 0 {
		config.Android.TargetSDK = 33
	}

	if config.Android.ABIs == nil {
		config.Android.ABIs = []string{"x86_64", "arm64-v8a"}
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

// validateAndroidPackageName enforces the Android applicationId / manifest
// package rules: at least two dot-separated segments, each starting with a
// letter and containing only letters, digits, or underscores. aapt2 rejects
// anything else, so this fails fast with a clear message instead of deep in the
// build.
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
