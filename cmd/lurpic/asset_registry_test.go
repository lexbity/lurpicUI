package main

import (
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/assets"
)

// TestGenerateAssetRegistry verifies the build produces a uuid_registry.json
// that the runtime can load and that resolves every asset file to a non-zero
// ID, with stable IDs across repeated builds.
func TestGenerateAssetRegistry(t *testing.T) {
	projectAssets := t.TempDir()
	staged := t.TempDir()

	files := []string{"fonts/regular.ttf", "sprites/hero.png", "ui/button.svg"}
	for _, rel := range files {
		p := filepath.Join(staged, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("data:"+rel), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := generateAssetRegistry(projectAssets, staged); err != nil {
		t.Fatalf("generateAssetRegistry: %v", err)
	}

	// The staged registry ships in the APK; load it the way the runtime does.
	stagedReg := filepath.Join(staged, assetRegistryName)
	reg, err := assets.LoadJSONPathRegistry(stagedReg)
	if err != nil {
		t.Fatalf("load staged registry: %v", err)
	}

	ids := make(map[string]assets.AssetID)
	for _, rel := range files {
		id := reg.Lookup(rel)
		if id.IsZero() {
			t.Errorf("asset %q resolved to an empty ID", rel)
		}
		ids[rel] = id
	}

	// The project copy is the checked-in source of truth.
	if _, err := os.Stat(filepath.Join(projectAssets, assetRegistryName)); err != nil {
		t.Errorf("project registry not written: %v", err)
	}

	// The registry itself must not be assigned an ID.
	if !reg.Lookup(assetRegistryName).IsZero() {
		t.Errorf("%s should not be registered as an asset", assetRegistryName)
	}

	// Re-running preserves IDs (stable across builds).
	if err := generateAssetRegistry(projectAssets, staged); err != nil {
		t.Fatalf("second generateAssetRegistry: %v", err)
	}
	reg2, err := assets.LoadJSONPathRegistry(stagedReg)
	if err != nil {
		t.Fatalf("reload registry: %v", err)
	}
	for rel, want := range ids {
		if got := reg2.Lookup(rel); got != want {
			t.Errorf("ID for %q changed across builds: %v -> %v", rel, want, got)
		}
	}
}
