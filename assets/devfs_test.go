package assets

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type pathRegistryStub struct {
	mu    sync.RWMutex
	paths map[string]AssetID
}

func newPathRegistryStub() *pathRegistryStub {
	return &pathRegistryStub{paths: make(map[string]AssetID)}
}

func (r *pathRegistryStub) Lookup(path string) AssetID {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.paths[path]
}

func (r *pathRegistryStub) Assign(path string, id AssetID) {
	r.mu.Lock()
	r.paths[path] = id
	r.mu.Unlock()
}

func TestDevFSInvalidateHotReload(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "icons", "check.svg")
	if err := os.MkdirAll(filepath.Dir(assetPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(assetPath, []byte("<svg/>"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	idReg := newPathRegistryStub()
	id := AssetID{1}
	idReg.Assign("icons/check.svg", id)

	reg := NewAssetRegistryStore()
	reg.SetLODReady(id, 0, &DecodedSVGLOD0{Data: []byte("ready")}, 1)
	before := reg.Get(id).EntryVersion

	invalidation := make(chan AssetInvalidatedSignal, 1)
	release := reg.SubscribeAsset(id, nil, func(sig AssetInvalidatedSignal) {
		select {
		case invalidation <- sig:
		default:
		}
	})
	defer release()

	devfs, err := NewDevFS(os.DirFS(dir), reg, idReg, WithDevWatchRoot(dir))
	if err != nil {
		t.Fatalf("new devfs: %v", err)
	}
	defer devfs.Close()

	if err := os.WriteFile(assetPath, []byte("<svg><!-- change --></svg>"), 0o644); err != nil {
		t.Fatalf("rewrite asset: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var drained int
	for time.Now().Before(deadline) {
		drained = devfs.DrainInvalidations()
		if drained > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if drained == 0 {
		t.Fatal("expected fsnotify invalidation to be drained")
	}

	entry := reg.Get(id)
	if entry == nil {
		t.Fatal("missing registry entry")
	}
	if entry.State != AssetStateAbsent {
		t.Fatalf("state = %v, want absent", entry.State)
	}
	if entry.EntryVersion <= before {
		t.Fatalf("entry version = %d, want > %d", entry.EntryVersion, before)
	}
	select {
	case <-invalidation:
	default:
		t.Fatal("expected invalidation signal")
	}
}

func TestDevFSDrainCompleted(t *testing.T) {
	dir := t.TempDir()
	idReg := newPathRegistryStub()
	reg := NewAssetRegistryStore()
	
	devfs, err := NewDevFS(os.DirFS(dir), reg, idReg)
	if err != nil {
		t.Fatalf("new devfs: %v", err)
	}
	defer devfs.Close()

	mgr := NewManager(reg, devfs, BackendSoftware, nil, idReg)
	if n := mgr.DrainCompleted(); n != 0 {
		t.Errorf("DrainCompleted() = %d, want 0", n)
	}
}
