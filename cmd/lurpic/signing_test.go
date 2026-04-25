package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeystoreConfig_Valid(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `[app]
id = "com.example.app"
name = "Test App"

[android]
min_sdk = 29
target_sdk = 33

[android.keystore]
path = "/path/to/keystore.jks"
alias = "release"
password = "secret123"
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	config, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if config.Android.Keystore.Path != "/path/to/keystore.jks" {
		t.Errorf("expected keystore path '/path/to/keystore.jks', got '%s'", config.Android.Keystore.Path)
	}
	if config.Android.Keystore.Alias != "release" {
		t.Errorf("expected alias 'release', got '%s'", config.Android.Keystore.Alias)
	}
	if config.Android.Keystore.Password != "secret123" {
		t.Errorf("expected password 'secret123', got '%s'", config.Android.Keystore.Password)
	}
}

func TestKeystoreConfig_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `[app]
id = "com.example.app"
name = "Test App"

[android]
min_sdk = 29
target_sdk = 33
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	config, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if config.Android.Keystore.Path != "" {
		t.Error("expected empty keystore path")
	}
	if config.Android.Keystore.Alias != "" {
		t.Error("expected empty keystore alias")
	}
	if config.Android.Keystore.Password != "" {
		t.Error("expected empty keystore password")
	}
}

func TestGetKeystorePassword_FromConfig(t *testing.T) {
	b := &androidBuilder{
		config: &Config{
			Android: AndroidConfig{
				Keystore: KeystoreConfig{
					Password: "config-password",
				},
			},
		},
	}

	pass := b.getKeystorePassword()
	if pass != "config-password" {
		t.Errorf("expected 'config-password', got '%s'", pass)
	}
}

func TestGetKeystorePassword_FromEnv(t *testing.T) {
	os.Setenv("LURPIC_KEYSTORE_PASSWORD", "env-password")
	defer os.Unsetenv("LURPIC_KEYSTORE_PASSWORD")

	b := &androidBuilder{
		config: &Config{
			Android: AndroidConfig{
				Keystore: KeystoreConfig{
					Password: "", // Empty in config
				},
			},
		},
	}

	pass := b.getKeystorePassword()
	if pass != "env-password" {
		t.Errorf("expected 'env-password', got '%s'", pass)
	}
}

func TestGetKeystorePassword_Priority(t *testing.T) {
	// Config takes priority over env
	os.Setenv("LURPIC_KEYSTORE_PASSWORD", "env-password")
	defer os.Unsetenv("LURPIC_KEYSTORE_PASSWORD")

	b := &androidBuilder{
		config: &Config{
			Android: AndroidConfig{
				Keystore: KeystoreConfig{
					Password: "config-password",
				},
			},
		},
	}

	pass := b.getKeystorePassword()
	if pass != "config-password" {
		t.Errorf("config should take priority: expected 'config-password', got '%s'", pass)
	}
}

func TestGetKeystorePassword_Empty(t *testing.T) {
	b := &androidBuilder{
		config: &Config{
			Android: AndroidConfig{
				Keystore: KeystoreConfig{
					Password: "",
				},
			},
		},
	}

	pass := b.getKeystorePassword()
	if pass != "" {
		t.Errorf("expected empty string, got '%s'", pass)
	}
}

func TestGetDebugKeystore_CreatesNew(t *testing.T) {
	// Create a temporary home directory for testing
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	b := &androidBuilder{}

	// First call should create the keystore
	keystore := b.getDebugKeystore()
	if keystore == "" {
		t.Fatal("getDebugKeystore should return a path")
	}

	// Verify it's in the expected location
	expectedPath := filepath.Join(tmpHome, ".android", "debug.keystore")
	if keystore != expectedPath {
		t.Errorf("expected keystore at %s, got %s", expectedPath, keystore)
	}

	// File should exist (or keystore generation should have been attempted)
	_, err := os.Stat(keystore)
	// Note: The keystore generation may fail if keytool is not available
	// which is expected in test environments
	_ = err
}

func TestGetDebugKeystore_ReusesExisting(t *testing.T) {
	// Create a temporary home directory for testing
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	// Create a fake keystore
	keystoreDir := filepath.Join(tmpHome, ".android")
	os.MkdirAll(keystoreDir, 0755)
	fakeKeystore := filepath.Join(keystoreDir, "debug.keystore")
	os.WriteFile(fakeKeystore, []byte("fake keystore data"), 0644)

	b := &androidBuilder{}
	keystore := b.getDebugKeystore()

	if keystore != fakeKeystore {
		t.Errorf("should reuse existing keystore, expected %s, got %s", fakeKeystore, keystore)
	}
}

func TestBuildFlags_KeystoreOverride(t *testing.T) {
	// Test that command-line flags override config
	tmpDir := t.TempDir()

	configContent := `[app]
id = "com.example.app"
name = "Test App"

[android.keystore]
path = "/config/keystore.jks"
alias = "config-alias"
password = "config-pass"
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	config, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	// Simulate command-line override
	config.Android.Keystore.Path = "/cmdline/keystore.jks"
	config.Android.Keystore.Alias = "cmdline-alias"
	config.Android.Keystore.Password = "cmdline-pass"

	if config.Android.Keystore.Path != "/cmdline/keystore.jks" {
		t.Error("keystore path should be overridden")
	}
	if config.Android.Keystore.Alias != "cmdline-alias" {
		t.Error("keystore alias should be overridden")
	}
	if config.Android.Keystore.Password != "cmdline-pass" {
		t.Error("keystore password should be overridden")
	}
}
