package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Valid(t *testing.T) {
	// Create a temporary directory with valid config
	tmpDir := t.TempDir()

	configContent := `[app]
id = "com.example.app"
name = "Example App"
version = "1.2.3"

[android]
min_sdk = 29
target_sdk = 33

[android.permissions]
required = ["android.permission.INTERNET"]
optional = ["android.permission.CAMERA"]
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	config, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if config.App.ID != "com.example.app" {
		t.Errorf("expected ID 'com.example.app', got '%s'", config.App.ID)
	}
	if config.App.Name != "Example App" {
		t.Errorf("expected name 'Example App', got '%s'", config.App.Name)
	}
	if config.App.Version != "1.2.3" {
		t.Errorf("expected version '1.2.3', got '%s'", config.App.Version)
	}
	if config.Android.MinSDK != 29 {
		t.Errorf("expected min_sdk 29, got %d", config.Android.MinSDK)
	}
	if config.Android.TargetSDK != 33 {
		t.Errorf("expected target_sdk 33, got %d", config.Android.TargetSDK)
	}
	if len(config.Android.Permissions.Required) != 1 || config.Android.Permissions.Required[0] != "android.permission.INTERNET" {
		t.Errorf("expected required permissions [android.permission.INTERNET], got %v", config.Android.Permissions.Required)
	}
	if len(config.Android.Permissions.Optional) != 1 || config.Android.Permissions.Optional[0] != "android.permission.CAMERA" {
		t.Errorf("expected optional permissions [android.permission.CAMERA], got %v", config.Android.Permissions.Optional)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	// Create a temporary directory with minimal config
	tmpDir := t.TempDir()

	configContent := `[app]
id = "com.test.minimal"
name = "Minimal App"
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	config, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	// Check defaults
	if config.App.Version != "1.0.0" {
		t.Errorf("expected default version '1.0.0', got '%s'", config.App.Version)
	}
	if config.Android.MinSDK != 29 {
		t.Errorf("expected default min_sdk 29, got %d", config.Android.MinSDK)
	}
	if config.Android.TargetSDK != 33 {
		t.Errorf("expected default target_sdk 33, got %d", config.Android.TargetSDK)
	}
}

func TestLoadConfig_MissingID(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `[app]
name = "No ID App"
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	_, err := loadConfig(tmpDir)
	if err == nil {
		t.Error("expected error when app.id is missing")
	}
}

func TestLoadConfig_MissingName(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `[app]
id = "com.test.noname"
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	_, err := loadConfig(tmpDir)
	if err == nil {
		t.Error("expected error when app.name is missing")
	}
}

func TestLoadConfig_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := loadConfig(tmpDir)
	if err == nil {
		t.Error("expected error when config file not found")
	}
}

func TestLoadConfig_InvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `this is not valid toml [[[
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	_, err := loadConfig(tmpDir)
	if err == nil {
		t.Error("expected error when TOML is invalid")
	}
}
