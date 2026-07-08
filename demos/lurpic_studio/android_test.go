package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAndroidCrossCompile(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not found in PATH")
	}
	// Android NDK is required for CGo-enabled compilation. Without NDK,
	// platform/android packages won't link. This test is a smoke check
	// that runs only when the LURPIC_ANDROID_NDK env var is set (CI).
	if os.Getenv("LURPIC_ANDROID_NDK") == "" {
		t.Skip("LURPIC_ANDROID_NDK not set; skipping Android cross-compile")
	}

	root := findModuleRoot()
	if root == "" {
		t.Fatal("could not find module root")
	}
	cmd := exec.Command("go", "build", "-tags", "android", "./demos/lurpic_studio")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS=android", "GOARCH=arm64", "CGO_ENABLED=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("android arm64 cross-compile failed: %v\n%s", err, string(out))
	}
}

func TestAssetsCSVPresent(t *testing.T) {
	paths := []string{
		"demos/lurpic_studio/assets/metrics.csv",
		"assets/metrics.csv",
	}
	found := false
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("metrics.csv not found in expected asset locations")
	}
}

func TestLurpicTomlPresent(t *testing.T) {
	paths := []string{
		"demos/lurpic_studio/lurpic.toml",
		"lurpic.toml",
	}
	found := false
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("lurpic.toml not found")
	}
}

func findModuleRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
