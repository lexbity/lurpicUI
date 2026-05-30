package main

import (
	"os"
	"path/filepath"
	"strings"
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
target_sdk = 36

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
	if config.Android.TargetSDK != 36 {
		t.Errorf("expected target_sdk 36, got %d", config.Android.TargetSDK)
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
	if config.Android.MinSDK != 24 {
		t.Errorf("expected default min_sdk 24, got %d", config.Android.MinSDK)
	}
	if config.Android.TargetSDK != 36 {
		t.Errorf("expected default target_sdk 36, got %d", config.Android.TargetSDK)
	}
}

func TestLoadConfig_ABIDefault(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `[app]
id = "com.test.abis"
name = "ABI Test"
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	config, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if len(config.Android.ABIs) != 2 {
		t.Fatalf("expected 2 default ABIs, got %d", len(config.Android.ABIs))
	}
	if config.Android.ABIs[0] != "x86_64" {
		t.Fatalf("expected ABIs[0] = x86_64, got %q", config.Android.ABIs[0])
	}
	if config.Android.ABIs[1] != "arm64-v8a" {
		t.Fatalf("expected ABIs[1] = arm64-v8a, got %q", config.Android.ABIs[1])
	}
}

func TestValidateAndroidPackageName(t *testing.T) {
	valid := []string{"org.example.app", "com.a.b", "org.lurpicui.quicksquare", "a.b.c_d", "x.y2"}
	for _, id := range valid {
		if err := validateAndroidPackageName(id); err != nil {
			t.Errorf("expected %q valid, got error: %v", id, err)
		}
	}
	invalid := []string{
		"",                 // empty
		"app",              // single segment
		"quick_square_app", // single segment with underscores
		"com.",             // empty trailing segment
		".com.app",         // empty leading segment
		"com.1abc.app",     // segment starts with digit
		"com.ab-c.app",     // hyphen not allowed
		"com.a b.app",      // space not allowed
	}
	for _, id := range invalid {
		if err := validateAndroidPackageName(id); err == nil {
			t.Errorf("expected %q invalid, got no error", id)
		}
	}
}

func TestLoadConfig_RejectsInvalidAppID(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `[app]
id = "quick_square_app"
name = "Bad ID"
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := loadConfig(tmpDir); err == nil {
		t.Fatal("expected loadConfig to reject single-segment app.id")
	}
}

func TestLoadConfig_MainDefault(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `[app]
id = "com.test.app"
name = "Main Default Test"
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	config, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if config.App.Main != "." {
		t.Fatalf("expected app.main default %q, got %q", ".", config.App.Main)
	}
}

func TestLoadConfig_MainExplicit(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `[app]
id = "org.lurpicui.quicksquare"
name = "Quick Square"
main = "cmd/quick_square_app/"
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	config, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	// Cleaned: trailing separator removed, independent of the reverse-DNS id.
	if config.App.Main != filepath.Join("cmd", "quick_square_app") {
		t.Fatalf("expected cleaned app.main, got %q", config.App.Main)
	}
}

func TestLoadConfig_MainRejectsEscape(t *testing.T) {
	cases := map[string]string{
		"absolute": "/etc",
		"parent":   "..",
		"escaping": "../sibling",
	}
	for name, main := range cases {
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configContent := "[app]\nid = \"com.test.app\"\nname = \"Escape Test\"\nmain = \"" + main + "\"\n"
			configPath := filepath.Join(tmpDir, "lurpic.toml")
			if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
				t.Fatalf("write config: %v", err)
			}
			if _, err := loadConfig(tmpDir); err == nil {
				t.Fatalf("expected error for app.main = %q", main)
			}
		})
	}
}

func TestLoadConfig_ABIExplicit(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `[app]
id = "com.test.abis"
name = "Explicit ABI Test"

[android]
abis = ["x86_64"]
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	config, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if len(config.Android.ABIs) != 1 {
		t.Fatalf("expected 1 ABI, got %d", len(config.Android.ABIs))
	}
	if config.Android.ABIs[0] != "x86_64" {
		t.Fatalf("expected ABIs[0] = x86_64, got %q", config.Android.ABIs[0])
	}
}

func TestLoadConfig_ABIAllArches(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `[app]
id = "com.test.abis"
name = "All ABIs"

[android]
abis = ["x86_64", "arm64-v8a", "armeabi-v7a"]
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	config, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if len(config.Android.ABIs) != 3 {
		t.Fatalf("expected 3 ABIs, got %d", len(config.Android.ABIs))
	}
	if config.Android.ABIs[2] != "armeabi-v7a" {
		t.Fatalf("expected ABIs[2] = armeabi-v7a, got %q", config.Android.ABIs[2])
	}
}

func TestLoadConfig_versionCodeDerived(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `[app]
id = "com.test.vc"
name = "VC Test"
version = "3.2.1"

[android]
min_sdk = 29
target_sdk = 36
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	config, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if config.Android.VersionCode != 3002001 {
		t.Fatalf("expected versionCode 3002001 from '3.2.1', got %d", config.Android.VersionCode)
	}
}

func TestLoadConfig_versionCodeExplicit(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `[app]
id = "com.test.vc"
name = "VC Explicit"
version = "1.0.0"

[android]
min_sdk = 29
target_sdk = 36
version_code = 99999
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	config, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if config.Android.VersionCode != 99999 {
		t.Fatalf("expected versionCode 99999 (explicit), got %d", config.Android.VersionCode)
	}
}

func TestLoadConfig_malformedVersionIsError(t *testing.T) {
	cases := []string{"1", "1.2", "1.2.3.4", "x.y.z", "1.x.3"}
	for _, ver := range cases {
		tmpDir2 := t.TempDir()
		cfg := `[app]
id = "com.test.bad"
name = "Bad"
version = "` + ver + `"

[android]
min_sdk = 29
target_sdk = 36
`
		configPath := filepath.Join(tmpDir2, "lurpic.toml")
		os.WriteFile(configPath, []byte(cfg), 0644)
		_, err := loadConfig(tmpDir2)
		if err == nil {
			t.Fatalf("expected error for version %q", ver)
		}
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

func TestAssetConfig_defaultNoCompressIncludesPak(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `[app]
id = "com.test.defaults"
name = "Defaults Test"

[android]
target_sdk = 35
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	config, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if config.Android.Assets.NoCompress == nil {
		t.Fatal("expected NoCompress to have a default value")
	}
	foundPak := false
	for _, p := range config.Android.Assets.NoCompress {
		if p == "*.pak" {
			foundPak = true
			break
		}
	}
	if !foundPak {
		t.Fatalf("expected NoCompress to contain '*.pak', got %v", config.Android.Assets.NoCompress)
	}
}

func TestAssetConfig_noCompressParsing(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `[app]
id = "com.test.assets"
name = "Asset Test"

[android]
target_sdk = 35

[android.assets]
no_compress = ["*.png", "*.mp4", "fonts/*.ttf"]
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	config, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if len(config.Android.Assets.NoCompress) != 3 {
		t.Fatalf("expected 3 no_compress patterns, got %d", len(config.Android.Assets.NoCompress))
	}
	if config.Android.Assets.NoCompress[0] != "*.png" {
		t.Errorf("expected no_compress[0]='*.png', got %q", config.Android.Assets.NoCompress[0])
	}
	if config.Android.Assets.NoCompress[1] != "*.mp4" {
		t.Errorf("expected no_compress[1]='*.mp4', got %q", config.Android.Assets.NoCompress[1])
	}
}

func TestIsNoCompress_matchesGlob(t *testing.T) {
	globs := []string{"*.png", "*.mp4"}
	if !isNoCompress("icon.png", globs) {
		t.Fatal("expected icon.png to match *.png")
	}
	if !isNoCompress("video.mp4", globs) {
		t.Fatal("expected video.mp4 to match *.mp4")
	}
	if isNoCompress("data.bin", globs) {
		t.Fatal("expected data.bin not to match")
	}
	if isNoCompress("icon.PNG", globs) {
		t.Fatal("expected case-sensitive no match")
	}
}

func TestCheckToolchainPins_missingBuildTools(t *testing.T) {
	sdkDir := t.TempDir()
	ndkDir := t.TempDir()

	warnings := checkToolchainPins(&Config{
		Android: AndroidConfig{TargetSDK: 35},
	}, sdkDir, ndkDir)

	found := false
	for _, w := range warnings {
		if strings.Contains(w, "build-tools 35.0.0") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected warning about missing build-tools 35.0.0")
	}
}

func TestCheckToolchainPins_quietWhenBuildToolsExist(t *testing.T) {
	sdkDir := t.TempDir()
	btDir := filepath.Join(sdkDir, "build-tools", "35.0.0")
	os.MkdirAll(btDir, 0755)
	ndkDir := t.TempDir()

	warnings := checkToolchainPins(&Config{
		Android: AndroidConfig{TargetSDK: 35},
	}, sdkDir, ndkDir)

	for _, w := range warnings {
		if strings.Contains(w, "build-tools") {
			t.Fatalf("unexpected build-tools warning: %s", w)
		}
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
