package cook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/assets"
)

func TestLoadManifestAndResolveDependencyTree(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "cook.toml")
	manifestSrc := []byte(`
[cook]
targets = ["linux", "android"]

[ids]
registry = ".lurpic-cache/asset-ids.json"

[font."assets/fonts/inter.ttf"]
ranges = ["U+0020-U+007E"]

[config."assets/config/theme.toml"]
deps = ["assets/fonts/inter.ttf", "assets/images/logo.png"]
`)
	if err := os.WriteFile(manifestPath, manifestSrc, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if got := manifest.Font["assets/fonts/inter.ttf"].Ranges; len(got) != 1 || got[0] != "U+0020-U+007E" {
		t.Fatalf("unexpected font rule: %+v", manifest.Font["assets/fonts/inter.ttf"])
	}
	if got := manifest.RegistryPath(); got != filepath.Join(dir, ".lurpic-cache", "asset-ids.json") {
		t.Fatalf("unexpected registry path: %q", got)
	}

	reg := NewUUIDRegistry()
	sources := []AssetSource{
		{Path: "assets/fonts/inter.ttf", Type: assets.AssetTypeFont, LODs: []CompiledLOD{{Level: 0, Data: []byte("font")}}},
		{Path: "assets/images/logo.png", Type: assets.AssetTypeImage, LODs: []CompiledLOD{{Level: 0, Data: []byte("image")}}},
		{Path: "assets/config/theme.toml", Type: assets.AssetTypeConfig, LODs: []CompiledLOD{{Level: 0, Data: []byte("config")}}},
	}
	tree, err := ResolveDependencyTree(manifest, reg, sources)
	if err != nil {
		t.Fatalf("resolve dependency tree: %v", err)
	}
	if len(tree.Leaves) != 2 {
		t.Fatalf("unexpected leaf count: %d", len(tree.Leaves))
	}
	if len(tree.Configs) != 1 {
		t.Fatalf("unexpected config count: %d", len(tree.Configs))
	}
	if got := len(tree.Configs[0].Deps); got != 2 {
		t.Fatalf("unexpected config dep count: %d", got)
	}

	fontID := reg.Lookup("assets/fonts/inter.ttf")
	imageID := reg.Lookup("assets/images/logo.png")
	if fontID.IsZero() || imageID.IsZero() {
		t.Fatal("expected resolved asset ids")
	}
	wantDeps := map[assets.AssetID]bool{fontID: true, imageID: true}
	for _, dep := range tree.Configs[0].Deps {
		if !wantDeps[dep] {
			t.Fatalf("unexpected dependency id: %s", dep)
		}
		delete(wantDeps, dep)
	}
	if len(wantDeps) != 0 {
		t.Fatalf("missing dependencies: %+v", wantDeps)
	}

	if _, err := os.Stat(manifest.RegistryPath()); err != nil {
		t.Fatalf("expected registry file: %v", err)
	}
	loaded, err := LoadUUIDRegistry(manifest.RegistryPath())
	if err != nil {
		t.Fatalf("load uuid registry: %v", err)
	}
	if got := len(loaded.Records()); got != 3 {
		t.Fatalf("unexpected registry record count: %d", got)
	}
}

func TestResolveDependencyTreeRejectsNestedConfigDeps(t *testing.T) {
	manifest := &Manifest{
		Config: map[string]ConfigRule{
			"assets/config/base.toml":  {},
			"assets/config/child.toml": {Deps: []string{"assets/config/base.toml"}},
		},
	}
	reg := NewUUIDRegistry()
	sources := []AssetSource{
		{Path: "assets/config/base.toml", Type: assets.AssetTypeConfig},
		{Path: "assets/config/child.toml", Type: assets.AssetTypeConfig},
	}
	_, err := ResolveDependencyTree(manifest, reg, sources)
	if err == nil {
		t.Fatal("expected nested config dependency error")
	}
	if !strings.Contains(err.Error(), "nested config") {
		t.Fatalf("unexpected error: %v", err)
	}
}
