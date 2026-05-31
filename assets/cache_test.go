package assets

import (
	"fmt"
	"math"
	"testing"
)

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

	// Bump generation — GPU LODs should be cleared, CPU data survives.
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

	// CPU data survives → entries with sizeBytes > 0 stay in the cache,
	// usedBytes is unchanged, registry LOD handles are preserved.
	if cache.usedBytes != 300 {
		t.Fatalf("usedBytes = %d, want 300 (CPU data survives)", cache.usedBytes)
	}
	if reg.Get(a).LODHandles[0] == nil {
		t.Fatal("expected CPU LOD data for 'a' to survive")
	}
	if reg.Get(b).LODHandles[0] == nil {
		t.Fatal("expected CPU LOD data for 'b' to survive")
	}

	// Registry entries should have GPU flags cleared but CPU data intact.
	entryA := reg.Get(a)
	if entryA.LODGPUReady[0] {
		t.Fatal("expected LODGPUReady to be cleared after device loss")
	}
	if entryA.LODTextureIDs[0] != 0 {
		t.Fatal("expected LODTextureID to be cleared after device loss")
	}
	if entryA.LODGPUBytes[0] != 0 {
		t.Fatal("expected LODGPUBytes to be cleared after device loss")
	}
}

func TestAssetCacheCheckDeviceGenerationGPUOnlyEntriesFullyEvicted(t *testing.T) {
	reg := NewAssetRegistryStore()
	backend := &recordingBackend{}
	cache := newAssetCache(reg, backend, 1000, 500)

	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc003")

	// Track a GPU-only entry (sizeBytes = 0, gpuBytes > 0).
	// This simulates a texture uploaded without CPU data tracking.
	cache.trackLOD(id, 0, 0, 200, TextureID(30), 1)

	if cache.gpuUsed != 200 {
		t.Fatalf("gpuUsed = %d, want 200", cache.gpuUsed)
	}
	if cache.usedBytes != 0 {
		t.Fatalf("usedBytes = %d, want 0 for GPU-only entry", cache.usedBytes)
	}

	if !cache.CheckDeviceGeneration(2) {
		t.Fatal("expected device generation change")
	}

	// GPU-only entry should be fully evicted.
	if len(backend.freed) != 1 || backend.freed[0] != TextureID(30) {
		t.Fatalf("expected texture 30 freed, got %v", backend.freed)
	}
	if cache.gpuUsed != 0 {
		t.Fatalf("gpuUsed = %d, want 0", cache.gpuUsed)
	}
	if len(cache.entries) != 0 {
		t.Fatalf("expected 0 cache entries after GPU-only eviction, got %d", len(cache.entries))
	}
}

// ── GPU-first eviction tests ────────────────────────────────────────────────

func TestEvictGPUToWatermark_freesOldestGPU(t *testing.T) {
	reg := NewAssetRegistryStore()
	backend := &recordingBackend{}
	cache := newAssetCache(reg, backend, 1000, 500)

	a := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc010")
	b := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc011")
	c := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc012")

	reg.SetLODReady(a, 0, &DecodedSVGLOD0{Data: []byte("a")}, 1)
	reg.SetLODReady(b, 0, &DecodedSVGLOD0{Data: []byte("b")}, 1)
	reg.SetLODReady(c, 0, &DecodedSVGLOD0{Data: []byte("c")}, 1)

	reg.SetLODGPUReady(a, 0, TextureID(1), 50)
	reg.SetLODGPUReady(b, 0, TextureID(2), 50)
	reg.SetLODGPUReady(c, 0, TextureID(3), 50)

	cache.trackLOD(a, 0, 100, 50, TextureID(1), 10)
	cache.trackLOD(b, 0, 100, 50, TextureID(2), 20)
	cache.trackLOD(c, 0, 100, 50, TextureID(3), 30)

	// Evict GPU to 25% watermark → targetGPU = 500 * 0.25 = 125.
	// gpuUsed starts at 150. Freeing oldest (a: 50) → 100. Below 125.
	count := cache.EvictGPUToWatermark(0.25)
	if count != 1 {
		t.Fatalf("expected 1 GPU eviction, got %d", count)
	}
	if cache.gpuUsed != 100 {
		t.Fatalf("gpuUsed = %d, want 100", cache.gpuUsed)
	}
	if len(backend.freed) != 1 || backend.freed[0] != TextureID(1) {
		t.Fatalf("expected texture 1 freed, got %v", backend.freed)
	}

	// CPU data survives — entries stay in cache.
	if len(cache.entries) != 3 {
		t.Fatalf("expected 3 entries in cache (CPU data survives), got %d", len(cache.entries))
	}
	if cache.usedBytes != 300 {
		t.Fatalf("usedBytes = %d, want 300 (CPU data unchanged)", cache.usedBytes)
	}

	// Registry: GPU fields cleared for 'a', CPU data intact.
	entryA := reg.Get(a)
	if entryA == nil {
		t.Fatal("expected entry A to survive GPU-only eviction")
	}
	if entryA.LODHandles[0] == nil {
		t.Fatal("expected CPU LOD data for A to survive")
	}
	if entryA.LODGPUReady[0] {
		t.Fatal("expected LODGPUReady to be cleared for A")
	}
	if entryA.LODTextureIDs[0] != 0 {
		t.Fatal("expected LODTextureID to be cleared for A")
	}

	// B and C are still GPU-ready.
	entryB := reg.Get(b)
	if !entryB.LODGPUReady[0] {
		t.Fatal("expected B to remain GPU-ready")
	}
	entryC := reg.Get(c)
	if !entryC.LODGPUReady[0] {
		t.Fatal("expected C to remain GPU-ready")
	}
}

func TestEvictGPUToWatermark_skipsReferencedEntries(t *testing.T) {
	reg := NewAssetRegistryStore()
	backend := &recordingBackend{}
	cache := newAssetCache(reg, backend, 1000, 100)

	keep := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc013")
	evict := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc014")

	reg.SetLODReady(keep, 0, &DecodedSVGLOD0{Data: []byte("keep")}, 1)
	reg.SetLODReady(evict, 0, &DecodedSVGLOD0{Data: []byte("evict")}, 1)

	reg.GetOrCreate(keep).LODRefCounts[0] = 1

	cache.trackLOD(keep, 0, 50, 50, TextureID(10), 1)
	cache.trackLOD(evict, 0, 50, 50, TextureID(11), 2)

	// GPU watermark at 0% → evict everything GPU.
	count := cache.EvictGPUToWatermark(0)
	if count != 1 {
		t.Fatalf("expected 1 GPU eviction (only unreferenced), got %d", count)
	}
	if len(backend.freed) != 1 || backend.freed[0] != TextureID(11) {
		t.Fatalf("expected only texture 11 freed (referenced skipped), got %v", backend.freed)
	}
	if cache.gpuUsed != 50 {
		t.Fatalf("gpuUsed = %d, want 50 (referenced entry's GPU survives)", cache.gpuUsed)
	}
}

func TestEvictGPUToWatermark_noOpWhenNoGPUData(t *testing.T) {
	reg := NewAssetRegistryStore()
	backend := &recordingBackend{}
	cache := newAssetCache(reg, backend, 1000, 100)

	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc015")
	reg.SetLODReady(id, 0, &DecodedSVGLOD0{Data: []byte("data")}, 1)
	cache.trackLOD(id, 0, 50, 0, TextureID(0), 1)

	count := cache.EvictGPUToWatermark(0)
	if count != 0 {
		t.Fatalf("expected 0 GPU evictions when no GPU data, got %d", count)
	}
	if len(backend.freed) != 0 {
		t.Fatal("expected no frees when no GPU data")
	}
}

func TestEvictGPUToWatermark_stopsAtWatermark(t *testing.T) {
	reg := NewAssetRegistryStore()
	backend := &recordingBackend{}
	cache := newAssetCache(reg, backend, 1000, 500)

	for i := 0; i < 5; i++ {
		id := mustAssetID(t, fmt.Sprintf("01234567-89ab-cdef-0123-456789abc02%d", i))
		reg.SetLODReady(id, 0, &DecodedSVGLOD0{Data: []byte{byte(i)}}, 1)
		cache.trackLOD(id, 0, 100, 100, TextureID(100+i), int64(i*10))
	}

	if cache.gpuUsed != 500 {
		t.Fatalf("gpuUsed = %d, want 500", cache.gpuUsed)
	}

	// Evict to 50% watermark → targetGPU = 250. Need to free 250 bytes.
	// Freeing oldest 3 entries (100+100+100=300) would overshoot.
	// gpuUsed after 2 freed (100+100=200): 300. Still above 250.
	// gpuUsed after 3 freed (100+100+100=300): 200. Below 250. Stop at 3.
	count := cache.EvictGPUToWatermark(0.50)
	// Need exactly 3 frees: 500 - 300 = 200 <= 250.
	if count != 3 {
		t.Fatalf("expected 3 GPU evictions to reach 50%% watermark, got %d", count)
	}
	if cache.gpuUsed > 250 {
		t.Fatalf("gpuUsed = %d, want <= 250 after eviction to 50%%", cache.gpuUsed)
	}
	if len(backend.freed) != 3 {
		t.Fatalf("expected 3 textures freed, got %d", len(backend.freed))
	}
	// CPU data for all 5 entries survives (usedBytes unchanged).
	if cache.usedBytes != 500 {
		t.Fatalf("usedBytes = %d, want 500 (CPU data unchanged)", cache.usedBytes)
	}
	if len(cache.entries) != 5 {
		t.Fatalf("expected all 5 entries to survive GPU-only eviction, got %d", len(cache.entries))
	}
}

// ── TrimLevelFraction mapping tests ──────────────────────────────────────────

func TestTrimLevelFraction_mapping(t *testing.T) {
	tests := []struct {
		level int
		want  float64
		desc  string
	}{
		{0, 0.75, "unknown"},
		{4, 0.75, "just below RUNNING_MODERATE"},
		{5, 0.50, "TRIM_MEMORY_RUNNING_MODERATE"},
		{9, 0.50, "just below RUNNING_LOW"},
		{10, 0.25, "TRIM_MEMORY_RUNNING_LOW"},
		{14, 0.25, "just below RUNNING_CRITICAL"},
		{15, 0.0, "TRIM_MEMORY_RUNNING_CRITICAL"},
		{19, 0.0, "just below UI_HIDDEN"},
		{20, 0.50, "TRIM_MEMORY_UI_HIDDEN"},
		{39, 0.50, "just below BACKGROUND"},
		{40, 0.25, "TRIM_MEMORY_BACKGROUND"},
		{59, 0.25, "just below MODERATE"},
		{60, 0.10, "TRIM_MEMORY_MODERATE"},
		{79, 0.10, "just below COMPLETE"},
		{80, 0.0, "TRIM_MEMORY_COMPLETE"},
		{100, 0.0, "above COMPLETE"},
	}
	for _, tt := range tests {
		got := TrimLevelFraction(tt.level)
		if math.Abs(got-tt.want) > 0.001 {
			t.Errorf("TrimLevelFraction(%d) = %.2f, want %.2f (%s)", tt.level, got, tt.want, tt.desc)
		}
	}
}
