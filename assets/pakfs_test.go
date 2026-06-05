package assets_test

import (
	"bytes"
	"encoding/binary"
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

	gate := assets.NewReadGate(pfs)
	readDone := make(chan error, 1)
	go func() {
		_, err := gate.ReadLOD(imageID, 0)
		readDone <- err
	}()

	<-gate.Started

	pfs.Close()

	close(gate.Release)

	readErr := <-readDone
	if readErr == nil {
		t.Fatal("expected read during close to return an error, got nil")
	}

	if _, err := pfs.ReadLOD(imageID, 0); err == nil {
		t.Error("expected error reading after close")
	}

	if err := pfs.Close(); err != nil {
		t.Errorf("double close: %v", err)
	}
}

func TestNewPakFSFromFDReadsLODs(t *testing.T) {
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

	// Open the file to get an fd, then create PakFS from fd.
	f, err := os.Open(pakPath)
	if err != nil {
		t.Fatalf("open pak: %v", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	pfs, err := assets.NewPakFSFromFD(int(f.Fd()), 0, fi.Size())
	if err != nil {
		t.Fatalf("NewPakFSFromFD: %v", err)
	}
	defer pfs.Close()

	// ReadLOD must work on fd-based PakFS.
	rawImage, err := pfs.ReadLOD(imageID, 0)
	if err != nil {
		t.Fatalf("read raw image lod: %v", err)
	}
	if !bytes.Equal(rawImage, []byte("image-lod0")) {
		t.Fatalf("unexpected raw image bytes: %q", rawImage)
	}

	// fs.FS must work on fd-based PakFS.
	imageFile, err := fs.ReadFile(pfs, imageID.String())
	if err != nil {
		t.Fatalf("read image via fs: %v", err)
	}
	if !bytes.Equal(imageFile, []byte("image-lod0")) {
		t.Fatalf("unexpected decoded image bytes: %q", imageFile)
	}

	// Root directory listing.
	entries, err := fs.ReadDir(pfs, ".")
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("unexpected root entry count: %d", len(entries))
	}

	// Close and verify no double-close panic.
	if err := pfs.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := pfs.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}

	// Reads after close must fail.
	_, err = pfs.ReadLOD(imageID, 0)
	if err == nil {
		t.Fatal("expected error reading after close")
	}
}

func TestNewPakFSFromFDRejectsInvalid(t *testing.T) {
	dir := t.TempDir()
	pakPath := filepath.Join(dir, "assets.pak")
	if err := os.WriteFile(pakPath, []byte{}, 0o644); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	f, err := os.Open(pakPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	if _, err := assets.NewPakFSFromFD(int(f.Fd()), 0, 0); err == nil {
		t.Fatal("expected error for zero length")
	}

	if _, err := assets.NewPakFSFromFD(int(f.Fd()), 0, -1); err == nil {
		t.Fatal("expected error for negative length")
	}
}

func writePakBytes(t *testing.T, data []byte) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "test-*.pak")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestNewPakFS_fromBytes_rejectsBadMagic(t *testing.T) {
	f := writePakBytes(t, []byte("not a pak"))
	_, err := assets.NewPakFSFromFD(int(f.Fd()), 0, 9)
	if err == nil {
		t.Fatal("expected error for bad magic, got nil")
	}
}

func TestNewPakFS_fromBytes_handlesTruncatedHeader(t *testing.T) {
	f := writePakBytes(t, []byte("LURP")) // valid magic but truncated before full header
	_, err := assets.NewPakFSFromFD(int(f.Fd()), 0, 4)
	if err == nil {
		t.Fatal("expected error for truncated header, got nil")
	}
}

func TestNewPakFS_fromBytes_handlesTruncatedTOCTable(t *testing.T) {
	hdr := make([]byte, 36) // 36 bytes = full PakHeader on 64-bit
	copy(hdr[0:4], []byte("LURP"))
	hdr[4] = 2 // version
	// TOCOffset at bytes 8-15, set to 36 (right after header)
	// TOCCount at bytes 16-19, set to 1000 (too many entries for data)
	binary.LittleEndian.PutUint64(hdr[8:16], 36)
	binary.LittleEndian.PutUint32(hdr[16:20], 1000)
	f := writePakBytes(t, hdr)
	_, err := assets.NewPakFSFromFD(int(f.Fd()), 0, int64(len(hdr)))
	if err == nil {
		t.Fatal("expected error for truncated TOC table, got nil")
	}
}

func TestNewPakFS_fromBytes_rejectsTOCOffsetPastEOF(t *testing.T) {
	hdr := make([]byte, 36)
	copy(hdr[0:4], []byte("LURP"))
	hdr[4] = 2
	binary.LittleEndian.PutUint64(hdr[8:16], 999999) // TOC offset past EOF
	f := writePakBytes(t, hdr)
	_, err := assets.NewPakFSFromFD(int(f.Fd()), 0, int64(len(hdr)))
	if err == nil {
		t.Fatal("expected error for TOC offset past EOF, got nil")
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

	handle := manager.LoadImage("assets/image.png")
	if handle.IsZero() {
		t.Fatal("LoadImage returned zero handle")
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if n := manager.DrainCompleted(); n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	entry := reg.Get(imageID)
	if entry == nil {
		t.Fatal("expected registry entry after load")
	}
	if entry.State != assets.AssetStateReady {
		t.Fatalf("asset state = %v, want Ready", entry.State)
	}
	if entry.HighestReadyLOD < 0 {
		t.Fatal("expected at least one ready LOD")
	}

	manager.DrainCompleted()
}

// FuzzNewPakFSFromFD tests that NewPakFSFromFD never panics on arbitrary input.
// PakFS files ship from disk/network on Android and are a robustness boundary.
func FuzzNewPakFSFromFD(f *testing.F) {
	f.Add([]byte("LURP\x02\x00\x00\x00" + string(make([]byte, 28))))
	f.Fuzz(func(t *testing.T, data []byte) {
		f, err := os.CreateTemp(t.TempDir(), "fuzz-*.pak")
		if err != nil {
			t.Skipf("create temp: %v", err)
		}
		if _, err := f.Write(data); err != nil {
			f.Close()
			t.Skipf("write: %v", err)
		}
		if _, err := f.Seek(0, 0); err != nil {
			f.Close()
			t.Skipf("seek: %v", err)
		}
		pfs, err := assets.NewPakFSFromFD(int(f.Fd()), 0, int64(len(data)))
		f.Close()
		if err == nil {
			pfs.Close()
		}
	})
}
