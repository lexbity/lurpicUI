package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

var (
	errNoConfigDir  = errors.New("cannot determine user config directory")
	errNoConfigFile = errors.New("user config file does not exist")
)

// UserConfig represents the user-level configuration from ~/.config/lurpic/config.toml
type UserConfig struct {
	Android UserAndroidConfig `toml:"android"`
}

// UserAndroidConfig contains user-level Android toolchain configuration
type UserAndroidConfig struct {
	SDKPath string `toml:"sdk-path"`
	NDKPath string `toml:"ndk-path"`
	JDKPath string `toml:"jdk-path"`
}

// loadUserConfig loads the user configuration from the platform-appropriate config directory.
// Returns nil config if no user config exists (not an error).
func loadUserConfig() (*UserConfig, error) {
	configDir := getUserConfigDir()
	if configDir == "" {
		return nil, errNoConfigDir
	}

	configPath := filepath.Join(configDir, "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No user config is not an error
			return nil, errNoConfigFile
		}
		return nil, fmt.Errorf("cannot read user config file: %w", err)
	}

	var config UserConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("cannot parse user config file: %w", err)
	}

	return &config, nil
}

// getUserConfigDir returns the platform-appropriate user config directory for lurpic.
func getUserConfigDir() string {
	// Check XDG_CONFIG_HOME first (Linux/macOS/Windows with XDG)
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "lurpic")
	}

	switch runtime.GOOS {
	case "linux":
		// XDG default: ~/.config/lurpic
		home, _ := os.UserHomeDir()
		if home != "" {
			return filepath.Join(home, ".config", "lurpic")
		}
	case "darwin":
		// macOS: ~/Library/Application Support/lurpic
		home, _ := os.UserHomeDir()
		if home != "" {
			return filepath.Join(home, "Library", "Application Support", "lurpic")
		}
	case "windows":
		// Windows: %APPDATA%\lurpic (Roaming)
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "lurpic")
		}
		// Fallback to LOCALAPPDATA
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "lurpic")
		}
	}

	return ""
}

// saveUserConfig saves the user configuration to the platform-appropriate config directory.
func saveUserConfig(config *UserConfig) error {
	configDir := getUserConfigDir()
	if configDir == "" {
		return fmt.Errorf("cannot determine user config directory")
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.toml")

	// Marshal to TOML
	data, err := toml.Marshal(config)
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("cannot write config file: %w", err)
	}

	return nil
}
