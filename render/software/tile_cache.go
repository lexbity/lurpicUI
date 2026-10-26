package software

import (
	"image"
	"sync"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
)

// TileCacheKey uniquely identifies a cached tile.
type TileCacheKey struct {
	// WindowID identifies which window this tile belongs to.
	WindowID string
	// TileCoord is the tile coordinate.
	TileCoord tileCoord
	// ContentRevision is a hash of the content that affects this tile.
	ContentRevision uint64
	// GeometryRevision accounts for geometry changes.
	GeometryRevision uint64
	// ClipRevision accounts for clip changes.
	ClipRevision uint64
	// FontGeneration accounts for font changes.
	FontGeneration uint64
	// ImageGeneration accounts for image/texture changes.
	ImageGeneration uint64
	// EffectState accounts for effect state.
	EffectState uint64
}

// TileEntry represents a cached raster tile.
type TileEntry struct {
	// Image is the rasterized tile content (pre-multiplied RGBA).
	Image *image.RGBA
	// Key is the cache key for this entry.
	Key TileCacheKey
	// PinCount is the number of active references to this tile.
	PinCount int
	// LastUsed tracks when this entry was last accessed (for LRU).
	LastUsed uint64
	// Bytes is the size of this entry in bytes.
	Bytes uint64
}

// TileCache is a byte-budgeted LRU cache for raster tiles.
type TileCache struct {
	mu sync.RWMutex

	// tiles maps tile coordinates to cache entries.
	tiles map[tileCoord]*TileEntry

	// keyMap maps cache keys to tile entries.
	keyMap map[TileCacheKey]*TileEntry

	// wallet is the resource wallet for this cache.
	wallet *membudget.Wallet

	// maxEntries is the maximum number of entries in the cache.
	maxEntries int

	// nextAccess is a counter for tracking access order.
	nextAccess uint64

	// stats
	hits      uint64
	misses    uint64
	evictions uint64
}

// TileCacheConfig configures a tile cache.
type TileCacheConfig struct {
	// Wallet is the resource wallet (required).
	Wallet *membudget.Wallet
	// MaxEntries is the maximum number of entries.
	MaxEntries int
}

// NewTileCache creates a new tile cache with the given configuration.
func NewTileCache(config TileCacheConfig) (*TileCache, error) {
	if config.Wallet == nil {
		return nil, errNoWallet
	}
	if config.MaxEntries <= 0 {
		config.MaxEntries = 1024 // Default
	}

	return &TileCache{
		tiles:      make(map[tileCoord]*TileEntry),
		keyMap:     make(map[TileCacheKey]*TileEntry),
		wallet:     config.Wallet,
		maxEntries: config.MaxEntries,
	}, nil
}

var errNoWallet = &ResourceError{Message: "tile cache requires a wallet"}

// ResourceError represents a resource-related error.
type ResourceError struct {
	Message string
}

func (e *ResourceError) Error() string {
	return e.Message
}

// Get returns a tile from the cache if available.
// The caller must call Release when done with the tile.
func (tc *TileCache) Get(key TileCacheKey) *TileEntry {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if entry, ok := tc.keyMap[key]; ok {
		tc.nextAccess++
		entry.LastUsed = tc.nextAccess
		tc.hits++
		entry.PinCount++
		return entry
	}

	tc.misses++
	return nil
}

// Create creates a new tile entry and adds it to the cache.
// The caller owns the returned tile and must call Release when done.
func (tc *TileCache) Create(key TileCacheKey, width, height int) (*TileEntry, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Check if we already have this key
	if entry, ok := tc.keyMap[key]; ok {
		entry.PinCount++
		tc.hits++
		tc.nextAccess++
		entry.LastUsed = tc.nextAccess
		return entry, nil
	}

	// Ensure capacity
	if err := tc.ensureCapacityLocked(); err != nil {
		return nil, err
	}

	// Create the image
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	bytesUsed := uint64(len(img.Pix))

	// Create the entry
	entry := &TileEntry{
		Image:    img,
		Key:      key,
		PinCount: 1,
		Bytes:    bytesUsed,
	}

	// Add to cache
	tc.tiles[key.TileCoord] = entry
	tc.keyMap[key] = entry
	tc.nextAccess++
	entry.LastUsed = tc.nextAccess

	return entry, nil
}

// ensureCapacityLocked ensures there is capacity for a new entry.
// Must be called with tc.mu held.
func (tc *TileCache) ensureCapacityLocked() error {
	currentCount := len(tc.tiles)
	if currentCount < tc.maxEntries {
		return nil
	}

	// Find the least recently used unpinned entry
	var toEvict *TileEntry
	var oldestAccess uint64 = 0

	for _, entry := range tc.tiles {
		if entry.PinCount == 0 && (toEvict == nil || entry.LastUsed < oldestAccess) {
			toEvict = entry
			oldestAccess = entry.LastUsed
		}
	}

	if toEvict == nil {
		return errCacheFull
	}

	// Evict the entry
	tc.evictLocked(toEvict)
	return nil
}

// evictLocked removes an entry from the cache.
func (tc *TileCache) evictLocked(entry *TileEntry) {
	delete(tc.tiles, entry.Key.TileCoord)
	delete(tc.keyMap, entry.Key)
	tc.evictions++
}

var errCacheFull = &ResourceError{Message: "tile cache is full and all entries are pinned"}

// Release decrements the pin count for a tile.
func (tc *TileCache) Release(entry *TileEntry) {
	if entry == nil {
		return
	}

	tc.mu.Lock()
	defer tc.mu.Unlock()

	entry.PinCount--
	if entry.PinCount < 0 {
		entry.PinCount = 0
	}
}

// Invalidate removes a specific tile from the cache.
func (tc *TileCache) Invalidate(coord tileCoord) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if entry, ok := tc.tiles[coord]; ok {
		tc.evictLocked(entry)
	}
}

// Clear removes all entries from the cache.
func (tc *TileCache) Clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.tiles = make(map[tileCoord]*TileEntry)
	tc.keyMap = make(map[TileCacheKey]*TileEntry)
}

// Stats returns cache statistics.
func (tc *TileCache) Stats() (hits, misses, evictions uint64) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.hits, tc.misses, tc.evictions
}

// Count returns the number of entries in the cache.
func (tc *TileCache) Count() int {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return len(tc.tiles)
}

// ByteCount returns the total bytes used by the cache.
func (tc *TileCache) ByteCount() uint64 {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	var total uint64
	for _, entry := range tc.tiles {
		total += entry.Bytes
	}
	return total
}
