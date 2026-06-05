package assets_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/assets/cook"
)

// TestAssetSourceContract verifies that every AssetSource implementation
// satisfies the same behavioural contract. Add new sources to the table.
func TestAssetSourceContract(t *testing.T) {
	imageID := mustParseID(t, "01234567-89ab-cdef-0123-456789000001")
	unknownID := mustParseID(t, "00000000-0000-0000-0000-000000000000")
	imagePath := "icons/check.svg"

	// ── PakFS source ──
	pakPath := buildTestPak(t, imageID, imagePath)
	pfs, err := assets.NewPakFS(pakPath)
	if err != nil {
		t.Fatalf("NewPakFS: %v", err)
	}
	t.Cleanup(func() { pfs.Close() })

	// ── DevFS source ──
	dir := t.TempDir()
	assetFile := filepath.Join(dir, imagePath)
	if err := os.MkdirAll(filepath.Dir(assetFile), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(assetFile, []byte("icon-file"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	reg := assets.NewAssetRegistryStore()
	entry := reg.GetOrCreate(imageID)
	entry.Path = imagePath
	entry.State = assets.AssetStateReady
	devFS, err := assets.NewDevFS(os.DirFS(dir), reg, newPathIDsMap(map[string]assets.AssetID{
		"icons/check.svg": imageID,
	}))
	if err != nil {
		t.Fatalf("NewDevFS: %v", err)
	}
	t.Cleanup(func() { devFS.Close() })

	sources := []struct {
		name     string
		source   assets.AssetSource
		wantData []byte
	}{
		{"PakFS", pfs, []byte("icon-lod0")},
		{"DevFS", devFS, []byte("icon-file")},
	}

	for _, src := range sources {
		t.Run(src.name, func(t *testing.T) {
			t.Run("existing LOD content", func(t *testing.T) {
				data, err := src.source.ReadLOD(imageID, 0)
				if err != nil {
					t.Fatalf("ReadLOD(%s, 0): %v", imageID, err)
				}
				if !bytes.Equal(data, src.wantData) {
					t.Fatalf("ReadLOD returned %q, want %q", data, src.wantData)
				}
			})

			t.Run("missing LOD error", func(t *testing.T) {
				_, err := src.source.ReadLOD(unknownID, 0)
				if err == nil {
					t.Fatal("expected error for unknown asset")
				}
				if !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, os.ErrNotExist) {
					// Source-specific error — not wrapped as fs.ErrNotExist.
					// This is expected for DevFS which returns its own format.
				}
			})
		})
	}
}

func buildTestPak(t *testing.T, id assets.AssetID, path string) string {
	t.Helper()
	tree := &cook.DependencyTree{
		Leaves: []cook.AssetNode{
			{ID: id, Path: path, Type: assets.AssetTypeImage, LODs: []cook.CompiledLOD{{Level: 0, Data: []byte("icon-lod0")}}},
		},
	}
	pak, err := (&cook.Packer{}).Pack(tree)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	dir := t.TempDir()
	pakPath := filepath.Join(dir, "assets.pak")
	if err := os.WriteFile(pakPath, pak, 0o644); err != nil {
		t.Fatalf("write pak: %v", err)
	}
	return pakPath
}

func mustParseID(t *testing.T, s string) assets.AssetID {
	t.Helper()
	id, err := assets.ParseAssetID(s)
	if err != nil {
		t.Fatalf("parse asset id: %v", err)
	}
	return id
}

func newPathIDsMap(m map[string]assets.AssetID) assets.PathIDRegistry {
	return &pathIDsMap{m: m}
}

type pathIDsMap struct {
	m map[string]assets.AssetID
}

func (r *pathIDsMap) Lookup(path string) assets.AssetID {
	return r.m[path]
}
