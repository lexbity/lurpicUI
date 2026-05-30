package assets

import "testing"

type recordingBackend struct {
	freed []TextureID
}

func (b *recordingBackend) FreeTexture(id TextureID) {
	b.freed = append(b.freed, id)
}

func TestAssetCacheEvictsLowestLastUse(t *testing.T) {
	reg := NewAssetRegistryStore()
	backend := &recordingBackend{}
	cache := newAssetCache(reg, backend, 100, 0)

	a := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc001")
	b := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc002")
	c := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc003")

	reg.SetLODReady(a, 0, &DecodedSVGLOD0{Data: []byte("a")}, 1)
	reg.SetLODReady(b, 0, &DecodedSVGLOD0{Data: []byte("b")}, 1)
	reg.SetLODReady(c, 0, &DecodedSVGLOD0{Data: []byte("c")}, 1)

	cache.trackLOD(a, 0, 40, 10, TextureID(1), 10)
	cache.trackLOD(b, 0, 40, 10, TextureID(2), 20)
	cache.trackLOD(c, 0, 40, 10, TextureID(3), 30)

	cache.evictIfNeeded(1)

	if got := cache.usedBytes; got != 80 {
		t.Fatalf("unexpected used bytes: %d", got)
	}
	if got := cache.gpuUsed; got != 20 {
		t.Fatalf("unexpected gpu bytes: %d", got)
	}
	if got := backend.freed; len(got) != 1 || got[0] != TextureID(1) {
		t.Fatalf("unexpected freed textures: %#v", got)
	}
	if got := reg.Get(a); got == nil || got.LODHandles[0] != nil {
		t.Fatal("expected oldest asset to be cleared")
	}
	if got := reg.Get(b); got == nil || got.LODHandles[0] == nil {
		t.Fatal("expected second asset to remain")
	}
	if got := reg.Get(c); got == nil || got.LODHandles[0] == nil {
		t.Fatal("expected third asset to remain")
	}
	if cache.evictionsThisFrame != 1 {
		t.Fatalf("unexpected eviction count: %d", cache.evictionsThisFrame)
	}
}

func TestAssetCacheSkipsReferencedEntries(t *testing.T) {
	reg := NewAssetRegistryStore()
	backend := &recordingBackend{}
	cache := newAssetCache(reg, backend, 50, 0)

	keep := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc010")
	evict := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc011")

	reg.SetLODReady(keep, 0, &DecodedSVGLOD0{Data: []byte("keep")}, 1)
	reg.SetLODReady(evict, 0, &DecodedSVGLOD0{Data: []byte("evict")}, 1)

	reg.GetOrCreate(keep).LODRefCounts[0] = 1

	cache.trackLOD(keep, 0, 60, 0, TextureID(10), 1)
	cache.trackLOD(evict, 0, 60, 0, TextureID(11), 2)

	cache.evictIfNeeded(1)

	if got := backend.freed; len(got) != 1 || got[0] != TextureID(11) {
		t.Fatalf("unexpected freed textures: %#v", got)
	}
	if got := reg.Get(keep); got == nil || got.LODHandles[0] == nil {
		t.Fatal("expected referenced asset to remain loaded")
	}
	if got := reg.Get(evict); got == nil || got.LODHandles[0] != nil {
		t.Fatal("expected unreferenced asset to be evicted")
	}
	if cache.usedBytes != 60 {
		t.Fatalf("unexpected used bytes after partial eviction: %d", cache.usedBytes)
	}
}

func TestAssetCacheCheckDeviceGenerationClearsGPULODs(t *testing.T) {
	reg := NewAssetRegistryStore()
	backend := &recordingBackend{}
	cache := newAssetCache(reg, backend, 1000, 500)

	a := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc001")
	b := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc002")

	reg.SetLODReady(a, 0, &DecodedSVGLOD0{Data: []byte("a")}, 1)
	reg.SetLODReady(b, 0, &DecodedSVGLOD0{Data: []byte("b")}, 1)

	// Track with GPU data — simulates uploaded textures.
	cache.trackLOD(a, 0, 100, 50, TextureID(10), 1)
	cache.trackLOD(b, 0, 200, 100, TextureID(20), 2)

	if cache.gpuUsed != 150 {
		t.Fatalf("gpuUsed = %d, want 150", cache.gpuUsed)
	}
	if len(backend.freed) != 0 {
		t.Fatal("expected no frees before generation check")
	}

	// First check with matching generation — no change.
	if cache.CheckDeviceGeneration(0) {
		t.Fatal("expected false for matching generation")
	}
	if len(backend.freed) != 0 {
		t.Fatal("expected no frees on matching generation")
	}

	// Bump generation — GPU LODs should be cleared.
	if !cache.CheckDeviceGeneration(1) {
		t.Fatal("expected true for generation change")
	}
	if cache.gpuUsed != 0 {
		t.Fatalf("gpuUsed = %d, want 0 after generation change", cache.gpuUsed)
	}
	if len(backend.freed) != 2 {
		t.Fatalf("expected 2 frees, got %d: %v", len(backend.freed), backend.freed)
	}
	freedSet := map[TextureID]bool{backend.freed[0]: true, backend.freed[1]: true}
	if !freedSet[10] || !freedSet[20] {
		t.Fatalf("unexpected freed textures: %v", backend.freed)
	}

	// CPU data should survive (usedBytes > 0 for entries with sizeBytes).
	// Both entries had sizeBytes > 0, so they're fully evicted.
	if cache.usedBytes != 0 {
		t.Fatalf("usedBytes = %d, want 0 after generation change for GPU-only entries",
			cache.usedBytes)
	}
}
