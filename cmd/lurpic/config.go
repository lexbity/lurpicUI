package main

import (
	"fmt"
	"os"
	"path/filepath"

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

// KeystoreConfig contains release signing configuration
// For security, the password can also be provided via:
// - Environment variable: LURPIC_KEYSTORE_PASSWORD
// - Command line flag: --keystore-password
type KeystoreConfig struct {
	Path     string `toml:"path"`
	Alias    string `toml:"alias"`
	Password string `toml:"password"`
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

	// Validate required fields
	if config.App.ID == "" {
		return nil, fmt.Errorf("app.id is required in lurpic.toml")
	}
	if config.App.Name == "" {
		return nil, fmt.Errorf("app.name is required in lurpic.toml")
	}

	return &config, nil
}
