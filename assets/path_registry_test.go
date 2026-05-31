package assets_test

import (
	"os"
	"testing"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/assets/cook"
)

// TestJSONPathRegistryCanonicalizesQueries verifies that a query path is
// canonicalized the same way the stored key is, so non-canonical forms still
// resolve. This is the consistency guarantee between the cook pipeline (which
// writes canonical keys) and the runtime (which receives arbitrary paths).
func TestJSONPathRegistryCanonicalizesQueries(t *testing.T) {
	id := mustParseID(t, "01234567-89ab-cdef-0123-4567890aaaaa")
	reg := assets.NewMapPathRegistry(map[string]assets.AssetID{
		"ui/button.png": id,
	})

	for _, query := range []string{
		"ui/button.png",
		"./ui/button.png",
		"ui//button.png",
		"ui/../ui/button.png",
	} {
		if got := reg.Lookup(query); got != id {
			t.Errorf("Lookup(%q) = %v, want %v", query, got, id)
		}
	}

	if got := reg.Lookup("ui/missing.png"); !got.IsZero() {
		t.Errorf("Lookup(missing) = %v, want zero", got)
	}
}

// TestJSONPathRegistryMatchesCookCanonicalization verifies that a key written
// by the cook UUIDRegistry resolves through the runtime JSONPathRegistry using
// the same shared canonicalization. The registry JSON round-trips through the
// on-disk format produced by SaveTo.
func TestJSONPathRegistryMatchesCookCanonicalization(t *testing.T) {
	dir := t.TempDir()
	regPath := dir + "/uuid_registry.json"

	ureg := cook.NewUUIDRegistry()
	// Assign with a non-canonical form; cook canonicalizes on store.
	id, err := ureg.Assign("./textures/wall.png")
	if err != nil {
		t.Fatalf("assign: %v", err)
	}
	if err := ureg.SaveTo(regPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	runtimeReg, err := assets.LoadJSONPathRegistry(regPath)
	if err != nil {
		t.Fatalf("load runtime registry: %v", err)
	}
	// The runtime resolves the same asset by its plain path.
	if got := runtimeReg.Lookup("textures/wall.png"); got != id {
		t.Fatalf("runtime Lookup = %v, want %v (cook/runtime canonicalization diverged)", got, id)
	}
}

// TestManagerResolvesPackedAssetByPath is the end-to-end guard for the
// build-but-unwired regression: a packed asset must resolve from a path to a
// non-zero handle carrying the correct ID. If path→ID resolution is broken,
// LoadImage returns an empty handle and this fails.
func TestManagerResolvesPackedAssetByPath(t *testing.T) {
	imageID := mustParseID(t, "01234567-89ab-cdef-0123-456789abcdef")

	tree := &cook.DependencyTree{
		Leaves: []cook.AssetNode{
			{
				ID:   imageID,
				Path: "sprites/hero.png",
				Type: assets.AssetTypeImage,
				LODs: []cook.CompiledLOD{{Level: 0, Data: []byte("hero-lod0")}},
			},
		},
	}

	pakBytes, err := (&cook.Packer{}).Pack(tree)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	pakPath := t.TempDir() + "/assets.pak"
	if err := os.WriteFile(pakPath, pakBytes, 0o644); err != nil {
		t.Fatalf("write pak: %v", err)
	}

	pak, err := assets.NewPakFS(pakPath)
	if err != nil {
		t.Fatalf("new pak fs: %v", err)
	}

	idReg := assets.NewMapPathRegistry(map[string]assets.AssetID{
		"sprites/hero.png": imageID,
	})
	mgr := assets.NewManager(assets.NewAssetRegistryStore(), pak, assets.BackendSoftware, nil, idReg)
	defer mgr.Close()

	h := mgr.LoadImage("sprites/hero.png")
	if h.IsZero() {
		t.Fatal("LoadImage returned an empty handle: path→ID resolution is not wired")
	}
	if h.ID != imageID {
		t.Fatalf("resolved handle ID = %v, want %v", h.ID, imageID)
	}

	// A path with no registry entry must stay zero, not alias another asset.
	if missing := mgr.LoadImage("sprites/unknown.png"); !missing.IsZero() {
		t.Fatalf("unknown path resolved to %v, want empty handle", missing.ID)
	}
}
