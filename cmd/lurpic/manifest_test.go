package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestManifestTemplate_ValidData(t *testing.T) {
	data := ManifestData{
		Package:            "com.example.testapp",
		VersionCode:        42,
		VersionName:        "1.2.3",
		MinSDK:             29,
		TargetSDK:          33,
		Permissions:        []string{"android.permission.INTERNET", "android.permission.CAMERA"},
		AppName:            "Test App",
		HasIcon:            true,
		UsesLurpicActivity: true,
	}

	tmpl, err := template.New("manifest").Parse(manifestTemplate)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("failed to execute template: %v", err)
	}

	manifest := buf.String()

	// Check key elements are present
	if !strings.Contains(manifest, `package="com.example.testapp"`) {
		t.Error("manifest missing correct package")
	}
	if !strings.Contains(manifest, `android:versionCode="42"`) {
		t.Error("manifest missing correct version code")
	}
	if !strings.Contains(manifest, `android:versionName="1.2.3"`) {
		t.Error("manifest missing correct version name")
	}
	if !strings.Contains(manifest, `minSdkVersion="29"`) {
		t.Error("manifest missing correct min SDK")
	}
	if !strings.Contains(manifest, `targetSdkVersion="33"`) {
		t.Error("manifest missing correct target SDK")
	}
	if !strings.Contains(manifest, `android:label="Test App"`) {
		t.Error("manifest missing correct app name")
	}
	if !strings.Contains(manifest, `android.permission.INTERNET`) {
		t.Error("manifest missing internet permission")
	}
	if !strings.Contains(manifest, `android.permission.CAMERA`) {
		t.Error("manifest missing camera permission")
	}
	if !strings.Contains(manifest, `android:icon="@mipmap/ic_launcher"`) {
		t.Error("manifest missing icon reference")
	}
	if !strings.Contains(manifest, `org.lurpicui.bridge.LurpicNativeActivity`) {
		t.Error("manifest missing LurpicNativeActivity")
	}
	if !strings.Contains(manifest, `android:exported="true"`) {
		t.Error("manifest missing exported attribute")
	}
}

func TestManifestTemplate_NoIcon(t *testing.T) {
	data := ManifestData{
		Package:            "com.example.noicon",
		VersionCode:        1,
		VersionName:        "1.0.0",
		MinSDK:             29,
		TargetSDK:          33,
		Permissions:        []string{},
		AppName:            "No Icon App",
		HasIcon:            false,
		UsesLurpicActivity: true,
	}

	tmpl, err := template.New("manifest").Parse(manifestTemplate)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("failed to execute template: %v", err)
	}

	manifest := buf.String()

	// Should NOT contain icon reference
	if strings.Contains(manifest, `android:icon`) {
		t.Error("manifest should not contain icon when HasIcon is false")
	}
}

func TestManifestTemplate_NoPermissions(t *testing.T) {
	data := ManifestData{
		Package:            "com.example.noperm",
		VersionCode:        1,
		VersionName:        "1.0.0",
		MinSDK:             29,
		TargetSDK:          33,
		Permissions:        []string{},
		AppName:            "No Permissions App",
		HasIcon:            false,
		UsesLurpicActivity: true,
	}

	tmpl, err := template.New("manifest").Parse(manifestTemplate)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("failed to execute template: %v", err)
	}

	manifest := buf.String()

	// Should not contain any uses-permission lines
	if strings.Contains(manifest, `uses-permission`) {
		t.Error("manifest should not contain any permissions")
	}
}

func TestDeriveVersionCode_1_2_3(t *testing.T) {
	code, err := deriveVersionCode("1.2.3")
	if err != nil {
		t.Fatalf("deriveVersionCode: %v", err)
	}
	if code != 1002003 {
		t.Fatalf("expected 1002003, got %d", code)
	}
}

func TestDeriveVersionCode_1_9_0_vs_1_2_3(t *testing.T) {
	code1, _ := deriveVersionCode("1.9.0")
	code2, _ := deriveVersionCode("1.2.3")
	if code1 == code2 {
		t.Fatal("1.9.0 and 1.2.3 must produce distinct version codes")
	}
	if code1 != 1009000 {
		t.Fatalf("1.9.0 expected 1009000, got %d", code1)
	}
	if code2 != 1002003 {
		t.Fatalf("1.2.3 expected 1002003, got %d", code2)
	}
}

func TestDeriveVersionCode_0_0_0(t *testing.T) {
	code, err := deriveVersionCode("0.0.0")
	if err != nil {
		t.Fatalf("deriveVersionCode: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
}

func TestDeriveVersionCode_malformedTooFewParts(t *testing.T) {
	_, err := deriveVersionCode("1.2")
	if err == nil {
		t.Fatal("expected error for '1.2'")
	}
}

func TestDeriveVersionCode_malformedTooManyParts(t *testing.T) {
	_, err := deriveVersionCode("1.2.3.4")
	if err == nil {
		t.Fatal("expected error for '1.2.3.4'")
	}
}

func TestDeriveVersionCode_malformedNonNumeric(t *testing.T) {
	_, err := deriveVersionCode("1.x.3")
	if err == nil {
		t.Fatal("expected error for '1.x.3'")
	}
}

func TestDeriveVersionCode_malformedEmpty(t *testing.T) {
	_, err := deriveVersionCode("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
}

func TestDeriveVersionCode_oversizedComponent(t *testing.T) {
	_, err := deriveVersionCode("1000.0.0")
	if err == nil {
		t.Fatal("expected error for major > 999")
	}
	_, err = deriveVersionCode("0.1000.0")
	if err == nil {
		t.Fatal("expected error for minor > 999")
	}
	_, err = deriveVersionCode("0.0.1000")
	if err == nil {
		t.Fatal("expected error for patch > 999")
	}
}

func TestDeriveVersionCode_monotonicity(t *testing.T) {
	// Verify ordering: higher semver → higher version code
	v1, _ := deriveVersionCode("1.0.0")
	v2, _ := deriveVersionCode("1.0.1")
	v3, _ := deriveVersionCode("1.1.0")
	v4, _ := deriveVersionCode("2.0.0")
	if !(v1 < v2 && v2 < v3 && v3 < v4) {
		t.Fatal("version codes must be monotonically increasing with semver")
	}
}

func TestAppConfig_HasIcon(t *testing.T) {
	// Icon configured
	config := AppConfig{
		ID:   "com.example.test",
		Name: "Test",
		Icon: "assets/icon.png",
	}
	if !config.HasIcon() {
		t.Error("HasIcon should return true when Icon is set")
	}

	// No icon
	config2 := AppConfig{
		ID:   "com.example.test",
		Name: "Test",
		Icon: "",
	}
	if config2.HasIcon() {
		t.Error("HasIcon should return false when Icon is empty")
	}
}

func TestConfig_WithIcon(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `[app]
id = "com.example.iconapp"
name = "Icon App"
version = "2.0.0"
icon = "assets/app-icon.png"

[android]
min_sdk = 30
target_sdk = 34

[android.permissions]
required = ["android.permission.INTERNET"]
`
	configPath := filepath.Join(tmpDir, "lurpic.toml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	config, err := loadConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if config.App.Icon != "assets/app-icon.png" {
		t.Errorf("expected icon 'assets/app-icon.png', got '%s'", config.App.Icon)
	}

	if !config.App.HasIcon() {
		t.Error("HasIcon should return true")
	}

	if config.Android.MinSDK != 30 {
		t.Errorf("expected min_sdk 30, got %d", config.Android.MinSDK)
	}

	if config.Android.TargetSDK != 34 {
		t.Errorf("expected target_sdk 34, got %d", config.Android.TargetSDK)
	}
}

func TestManifestTemplate_WellFormedXML(t *testing.T) {
	data := ManifestData{
		Package:            "com.example.xmltest",
		VersionCode:        1,
		VersionName:        "1.0",
		MinSDK:             29,
		TargetSDK:          33,
		Permissions:        []string{"android.permission.INTERNET"},
		AppName:            "XML Test",
		HasIcon:            true,
		UsesLurpicActivity: true,
	}

	tmpl, err := template.New("manifest").Parse(manifestTemplate)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("failed to execute template: %v", err)
	}

	manifest := buf.String()

	// Basic XML structure checks
	if !strings.HasPrefix(manifest, `<?xml version="1.0" encoding="utf-8"?>`) {
		t.Error("manifest missing XML declaration")
	}
	if !strings.Contains(manifest, "<manifest") {
		t.Error("manifest missing manifest element")
	}
	if !strings.Contains(manifest, "</manifest>") {
		t.Error("manifest missing closing manifest tag")
	}
	if !strings.Contains(manifest, "<application") {
		t.Error("manifest missing application element")
	}
	if !strings.Contains(manifest, "</application>") {
		t.Error("manifest missing closing application tag")
	}
}
