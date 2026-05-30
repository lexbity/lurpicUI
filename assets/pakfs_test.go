package assets_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/assets/cook"
)

// newPathIDs returns an assets.PathIDRegistry backed by the given map.
func newPathIDs(t *testing.T, m map[string]assets.AssetID) assets.PathIDRegistry {
	t.Helper()
	return &pathIDsStub{paths: m}
}

type pathIDsStub struct {
	mu    sync.Mutex
	paths map[string]assets.AssetID
}

func (r *pathIDsStub) Lookup(path string) assets.AssetID {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.paths[path]
}

func TestNewPakFSReadsRawAndDecodedBlocks(t *testing.T) {
	imageID, err := assets.ParseAssetID("01234567-89ab-cdef-0123-456789abcdef")
	if err != nil {
		t.Fatalf("parse image id: %v", err)
	}
	fontID, err := assets.ParseAssetID("01234567-89ab-cdef-0123-456789abcdee")
	if err != nil {
		t.Fatalf("parse font id: %v", err)
	}

	tree := &cook.DependencyTree{
		Leaves: []cook.AssetNode{
			{ID: imageID, Path: "assets/image.png", Type: assets.AssetTypeImage, LODs: []cook.CompiledLOD{{Level: 0, Data: []byte("image-lod0")}}},
			{ID: fontID, Path: "assets/font.ttf", Type: assets.AssetTypeFont, LODs: []cook.CompiledLOD{{Level: 0, Data: []byte("font-lod0-data")}}},
		},
	}

	pak, err := (&cook.Packer{}).Pack(tree)
	if err != nil {
		t.Fatalf("pack tree: %v", err)
	}

	dir := t.TempDir()
	pakPath := filepath.Join(dir, "assets.pak")
	if err := os.WriteFile(pakPath, pak, 0o644); err != nil {
		t.Fatalf("write pak: %v", err)
	}

	pfs, err := assets.NewPakFS(pakPath)
	if err != nil {
		t.Fatalf("new pak fs: %v", err)
	}
	defer pfs.Close()

	stat, err := fs.Stat(pfs, ".")
	if err != nil {
		t.Fatalf("stat root: %v", err)
	}
	if !stat.IsDir() {
		t.Fatal("expected root to be a directory")
	}

	entries, err := fs.ReadDir(pfs, ".")
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("unexpected root entry count: %d", len(entries))
	}
	if entries[0].Name() != imageID.String() && entries[1].Name() != imageID.String() {
		t.Fatalf("expected image id in root entries: %+v", entries)
	}

	rawImage, err := pfs.ReadLOD(imageID, 0)
	if err != nil {
		t.Fatalf("read raw image lod: %v", err)
	}
	if !bytes.Equal(rawImage, []byte("image-lod0")) {
		t.Fatalf("unexpected raw image bytes: %q", rawImage)
	}

	rawFont, err := pfs.ReadLOD(fontID, 0)
	if err != nil {
		t.Fatalf("read raw font lod: %v", err)
	}
	if bytes.Equal(rawFont, []byte("font-lod0-data")) {
		t.Fatal("expected compressed font raw bytes to differ from source")
	}

	imageFile, err := fs.ReadFile(pfs, imageID.String())
	if err != nil {
		t.Fatalf("read image via fs: %v", err)
	}
	if !bytes.Equal(imageFile, []byte("image-lod0")) {
		t.Fatalf("unexpected decoded image bytes: %q", imageFile)
	}

	fontFile, err := fs.ReadFile(pfs, fontID.String())
	if err != nil {
		t.Fatalf("read font via fs: %v", err)
	}
	if !bytes.Equal(fontFile, []byte("font-lod0-data")) {
		t.Fatalf("unexpected decoded font bytes: %q", fontFile)
	}

	statFont, err := fs.Stat(pfs, fontID.String())
	if err != nil {
		t.Fatalf("stat font: %v", err)
	}
	if statFont.Size() != int64(len("font-lod0-data")) {
		t.Fatalf("unexpected stat size: %d", statFont.Size())
	}
}

func TestPakFSConcurrentLoadAndClose(t *testing.T) {
	imageID := mustParseID(t, "01234567-89ab-cdef-0123-456789000001")
	fontID := mustParseID(t, "01234567-89ab-cdef-0123-456789000002")
	configID := mustParseID(t, "01234567-89ab-cdef-0123-456789000003")

	tree := &cook.DependencyTree{
		Leaves: []cook.AssetNode{
			{ID: imageID, Path: "img/a.png", Type: assets.AssetTypeImage, LODs: []cook.CompiledLOD{
				{Level: 0, Data: []byte("image-lod0")},
				{Level: 1, Data: []byte("image-lod1")},
			}},
			{ID: fontID, Path: "fonts/a.ttf", Type: assets.AssetTypeFont, LODs: []cook.CompiledLOD{
				{Level: 0, Data: make([]byte, 10000)},
			}},
			{ID: configID, Path: "cfg/a.toml", Type: assets.AssetTypeConfig, LODs: []cook.CompiledLOD{
				{Level: 0, Data: []byte("key=val")},
			}},
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

	pfs, err := assets.NewPakFS(pakPath)
	if err != nil {
		t.Fatalf("NewPakFS: %v", err)
	}

	// Launch concurrent readers.
	var readWg sync.WaitGroup
	errs := make(chan error, 20)
	readIDs := []assets.AssetID{imageID, fontID, configID}
	for i := 0; i < 10; i++ {
		readWg.Add(1)
		go func(n int) {
			defer readWg.Done()
			id := readIDs[n%len(readIDs)]
			// Only LOD 0 is guaranteed to exist for all three types.
			_, err := pfs.ReadLOD(id, 0)
			if err != nil && err.Error() != "pakfs closed" {
				errs <- err
			}
		}(i)
	}

	// Close while reads are in-flight.
	time.Sleep(time.Millisecond)
	closeErr := pfs.Close()
	if closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	readWg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent read error: %v", err)
	}

	// Reads after close must fail.
	_, err = pfs.ReadLOD(imageID, 0)
	if err == nil {
		t.Error("expected error reading after close")
	}

	// Double-close must not panic.
	if err := pfs.Close(); err != nil {
		t.Errorf("double close: %v", err)
	}
}

func TestPakFSManagerRoundTrip(t *testing.T) {
	imageID, err := assets.ParseAssetID("01234567-89ab-cdef-0123-456789abcdef")
	if err != nil {
		t.Fatalf("parse image id: %v", err)
	}
	tree := &cook.DependencyTree{
		Leaves: []cook.AssetNode{
			{ID: imageID, Path: "assets/image.png", Type: assets.AssetTypeImage, LODs: []cook.CompiledLOD{{Level: 0, Data: []byte("image-lod0")}}},
		},
	}
	pak, err := (&cook.Packer{}).Pack(tree)
	if err != nil {
		t.Fatalf("pack tree: %v", err)
	}
	dir := t.TempDir()
	pakPath := filepath.Join(dir, "assets.pak")
	if err := os.WriteFile(pakPath, pak, 0o644); err != nil {
		t.Fatalf("write pak: %v", err)
	}

	pfs, err := assets.NewPakFS(pakPath)
	if err != nil {
		t.Fatalf("new pak fs: %v", err)
	}
	defer pfs.Close()

	reg := assets.NewAssetRegistryStore()
	pathIDs := newPathIDs(t, map[string]assets.AssetID{
		"assets/image.png": imageID,
	})
	manager := assets.NewManager(reg, pfs, assets.BackendSoftware, nil, pathIDs)
	defer func() {
		if n := manager.DrainCompleted(); n != 0 {
			t.Errorf("DrainCompleted() = %d, want 0", n)
		}
	}()
	if n := manager.DrainCompleted(); n != 0 {
		t.Errorf("DrainCompleted() = %d, want 0", n)
	}
}
