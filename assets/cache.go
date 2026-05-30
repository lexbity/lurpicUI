package assets

import "container/heap"

// TextureID identifies an uploaded texture resource.
type TextureID uint64

// textureReleaser is the minimal backend contract needed for eviction.
type textureReleaser interface {
	FreeTexture(TextureID)
}

type assetLODKey struct {
	id  AssetID
	lod int
}

// assetCache manages the in-memory budget for decoded asset data.
// Runtime-thread-only; no synchronisation needed.
type assetCache struct {
	registry    *AssetRegistryStore
	backend     textureReleaser
	budgetBytes int64
	usedBytes   int64
	gpuBudget   int64
	gpuUsed     int64

	lruHeap lodLRUHeap
	entries map[assetLODKey]*lodCacheEntry

	evictionsThisFrame int
}

type lodCacheEntry struct {
	assetID   AssetID
	lod       int
	lastUse   int64
	sizeBytes int64
	gpuBytes  int64
	textureID TextureID
	index     int
}

// lodLRUHeap is a min-heap ordered by LastUse, with oldest entries first.
type lodLRUHeap []*lodCacheEntry

func (h lodLRUHeap) Len() int { return len(h) }

func (h lodLRUHeap) Less(i, j int) bool {
	if h[i].lastUse != h[j].lastUse {
		return h[i].lastUse < h[j].lastUse
	}
	if h[i].assetID != h[j].assetID {
		return h[i].assetID.String() < h[j].assetID.String()
	}
	return h[i].lod < h[j].lod
}

func (h lodLRUHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *lodLRUHeap) Push(x any) {
	entry := x.(*lodCacheEntry)
	entry.index = len(*h)
	*h = append(*h, entry)
}

func (h *lodLRUHeap) Pop() any {
	old := *h
	n := len(old)
	entry := old[n-1]
	old[n-1] = nil
	entry.index = -1
	*h = old[:n-1]
	return entry
}

// newAssetCache returns an initialized cache controller.
func newAssetCache(registry *AssetRegistryStore, backend textureReleaser, budgetBytes, gpuBudget int64) *assetCache {
	return &assetCache{
		registry:    registry,
		backend:     backend,
		budgetBytes: budgetBytes,
		gpuBudget:   gpuBudget,
		entries:     make(map[assetLODKey]*lodCacheEntry),
	}
}

// trackLOD records a cached LOD in the eviction heap and accounting totals.
func (c *assetCache) trackLOD(assetID AssetID, lod int, sizeBytes, gpuBytes int64, textureID TextureID, lastUse int64) {
	if c == nil {
		return
	}
	key := assetLODKey{id: assetID, lod: lod}
	if existing := c.entries[key]; existing != nil {
		c.usedBytes -= existing.sizeBytes
		c.gpuUsed -= existing.gpuBytes
		existing.lastUse = lastUse
		existing.sizeBytes = sizeBytes
		existing.gpuBytes = gpuBytes
		existing.textureID = textureID
		c.usedBytes += sizeBytes
		c.gpuUsed += gpuBytes
		heap.Fix(&c.lruHeap, existing.index)
	} else {
		entry := &lodCacheEntry{
			assetID:   assetID,
			lod:       lod,
			lastUse:   lastUse,
			sizeBytes: sizeBytes,
			gpuBytes:  gpuBytes,
			textureID: textureID,
		}
		c.entries[key] = entry
		c.usedBytes += sizeBytes
		c.gpuUsed += gpuBytes
		heap.Push(&c.lruHeap, entry)
	}

	if c.registry != nil {
		entry := c.registry.GetOrCreate(assetID)
		if entry != nil && lod >= 0 && lod < len(entry.SizeBytes) {
			entry.SizeBytes[lod] = sizeBytes
			entry.LastUse = lastUse
		}
	}
}

func (c *assetCache) currentRefCount(entry *lodCacheEntry) int32 {
	if c == nil || c.registry == nil || entry == nil {
		return 0
	}
	reg := c.registry.Get(entry.assetID)
	if reg == nil || entry.lod < 0 || entry.lod >= len(reg.LODRefCounts) {
		return 0
	}
	return reg.LODRefCounts[entry.lod]
}

func (c *assetCache) popMinEvictable() *lodCacheEntry {
	if c == nil {
		return nil
	}
	var skipped []*lodCacheEntry
	for c.lruHeap.Len() > 0 {
		entry := heap.Pop(&c.lruHeap).(*lodCacheEntry)
		if c.currentRefCount(entry) > 0 {
			skipped = append(skipped, entry)
			continue
		}
		for _, keep := range skipped {
			heap.Push(&c.lruHeap, keep)
		}
		return entry
	}
	for _, keep := range skipped {
		heap.Push(&c.lruHeap, keep)
	}
	return nil
}

// EvictToWatermark evicts cache entries until used bytes are at or below the
// given fraction of the original budget (0.0–1.0). Setting fraction to 0 evicts
// all unpinned entries. The returned count is the number of LODs evicted.
func (c *assetCache) EvictToWatermark(fraction float64) int {
	if c == nil || c.budgetBytes <= 0 {
		return 0
	}
	targetBytes := int64(float64(c.budgetBytes) * fraction)
	targetGPU := int64(float64(c.gpuBudget) * fraction)
	count := 0
	for (c.usedBytes > targetBytes) || (c.gpuBudget > 0 && c.gpuUsed > targetGPU) {
		victim := c.popMinEvictable()
		if victim == nil {
			break
		}
		if c.backend != nil && victim.textureID != 0 {
			c.backend.FreeTexture(victim.textureID)
		}
		c.usedBytes -= victim.sizeBytes
		c.gpuUsed -= victim.gpuBytes
		delete(c.entries, assetLODKey{id: victim.assetID, lod: victim.lod})
		if c.registry != nil {
			c.registry.ClearLOD(victim.assetID, victim.lod)
		}
		count++
		c.evictionsThisFrame++
	}
	return count
}

// TrimLevelFraction maps Android onTrimMemory levels to a budget fraction.
// A higher fraction means less aggressive eviction.
func TrimLevelFraction(level int) float64 {
	switch {
	case level >= 80: // TRIM_MEMORY_COMPLETE
		return 0.0
	case level >= 60: // TRIM_MEMORY_MODERATE
		return 0.1
	case level >= 40: // TRIM_MEMORY_BACKGROUND
		return 0.25
	case level >= 20: // TRIM_MEMORY_UI_HIDDEN
		return 0.50
	case level >= 15: // TRIM_MEMORY_RUNNING_CRITICAL
		return 0.0
	case level >= 10: // TRIM_MEMORY_RUNNING_LOW
		return 0.25
	case level >= 5: // TRIM_MEMORY_RUNNING_MODERATE
		return 0.50
	default:
		return 0.75
	}
}

// evictIfNeeded runs at the end of Phase 1 after job commits for the frame.
func (c *assetCache) evictIfNeeded(currentFrame int64) {
	if c == nil {
		return
	}
	_ = currentFrame
	for c.usedBytes > c.budgetBytes || (c.gpuBudget > 0 && c.gpuUsed > c.gpuBudget) {
		victim := c.popMinEvictable()
		if victim == nil {
			return
		}
		if c.backend != nil && victim.textureID != 0 {
			c.backend.FreeTexture(victim.textureID)
		}
		c.usedBytes -= victim.sizeBytes
		c.gpuUsed -= victim.gpuBytes
		delete(c.entries, assetLODKey{id: victim.assetID, lod: victim.lod})
		if c.registry != nil {
			c.registry.ClearLOD(victim.assetID, victim.lod)
		}
		c.evictionsThisFrame++
	}
}
