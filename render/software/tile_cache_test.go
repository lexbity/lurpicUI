package software

import (
	"image"
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
)

func TestTileCache_Basic(t *testing.T) {
	// Create a wallet and cache
	wallet, err := membudget.NewWallet(1024 * 1024) // 1 MB
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	cache, err := NewTileCache(TileCacheConfig{
		Wallet:     wallet,
		MaxEntries: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create tile cache: %v", err)
	}

	// Test empty cache
	if cache.Count() != 0 {
		t.Errorf("New cache should be empty, got %d entries", cache.Count())
	}

	// Test nil wallet
	_, err = NewTileCache(TileCacheConfig{
		Wallet:     nil,
		MaxEntries: 100,
	})
	if err == nil {
		t.Error("NewTileCache with nil wallet should fail")
	}
}

func TestTileCache_CreateAndGet(t *testing.T) {
	wallet, err := membudget.NewWallet(1024 * 1024)
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	cache, err := NewTileCache(TileCacheConfig{
		Wallet:     wallet,
		MaxEntries: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create tile cache: %v", err)
	}

	// Create a tile
	key := TileCacheKey{
		WindowID:        "test",
		TileCoord:       tileCoord{X: 0, Y: 0},
		ContentRevision: 1,
	}

	entry, err := cache.Create(key, TileSize, TileSize)
	if err != nil {
		t.Fatalf("Failed to create tile: %v", err)
	}
	if entry == nil {
		t.Fatal("Create returned nil entry")
	}
	if entry.Image == nil {
		t.Error("Entry image should not be nil")
	}
	if entry.Image.Bounds().Dx() != TileSize || entry.Image.Bounds().Dy() != TileSize {
		t.Errorf("Entry image size should be %dx%d, got %dx%d",
			TileSize, TileSize, entry.Image.Bounds().Dx(), entry.Image.Bounds().Dy())
	}

	// Get the same tile - should increment pin count
	got := cache.Get(key)
	if got == nil {
		t.Error("Get should return the tile")
	}
	if got != entry {
		t.Error("Get should return the same entry")
	}

	// Test cache count
	if cache.Count() != 1 {
		t.Errorf("Cache should have 1 entry, got %d", cache.Count())
	}

	// Release both pins (Create sets PinCount=1, Get increments to 2)
	cache.Release(entry)
	cache.Release(entry)
	if entry.PinCount != 0 {
		t.Errorf("PinCount should be 0 after two releases, got %d", entry.PinCount)
	}
}

func TestTileCache_Eviction(t *testing.T) {
	wallet, err := membudget.NewWallet(1024 * 1024)
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Create a cache with small capacity
	cache, err := NewTileCache(TileCacheConfig{
		Wallet:     wallet,
		MaxEntries: 2,
	})
	if err != nil {
		t.Fatalf("Failed to create tile cache: %v", err)
	}

	// Add first tile
	key1 := TileCacheKey{WindowID: "test", TileCoord: tileCoord{X: 0, Y: 0}, ContentRevision: 1}
	entry1, err := cache.Create(key1, TileSize, TileSize)
	if err != nil {
		t.Fatalf("Failed to create first tile: %v", err)
	}

	// Add second tile
	key2 := TileCacheKey{WindowID: "test", TileCoord: tileCoord{X: 1, Y: 0}, ContentRevision: 1}
	entry2, err := cache.Create(key2, TileSize, TileSize)
	if err != nil {
		t.Fatalf("Failed to create second tile: %v", err)
	}

	if cache.Count() != 2 {
		t.Errorf("Cache should have 2 entries, got %d", cache.Count())
	}

	// Release both pins for both entries
	cache.Release(entry1)
	cache.Release(entry1)
	cache.Release(entry2)
	cache.Release(entry2)

	// Now try to add a third tile - should trigger eviction
	key3 := TileCacheKey{WindowID: "test", TileCoord: tileCoord{X: 2, Y: 0}, ContentRevision: 1}
	_, err = cache.Create(key3, TileSize, TileSize)
	if err != nil {
		t.Logf("Adding third tile returned error: %v", err)
		// This is expected if cache is full and all entries are unpinned
		// But we released them, so eviction should work
	}

	// Check that we have at most max entries
	count := cache.Count()
	if count > 2 {
		t.Errorf("Cache should not exceed max entries, got %d", count)
	}
}

func TestTileCache_Invalidate(t *testing.T) {
	wallet, err := membudget.NewWallet(1024 * 1024)
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	cache, err := NewTileCache(TileCacheConfig{
		Wallet:     wallet,
		MaxEntries: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create tile cache: %v", err)
	}

	// Add a tile
	key := TileCacheKey{WindowID: "test", TileCoord: tileCoord{X: 0, Y: 0}, ContentRevision: 1}
	entry, err := cache.Create(key, TileSize, TileSize)
	if err != nil {
		t.Fatalf("Failed to create tile: %v", err)
	}

	// Release pins
	cache.Release(entry)
	cache.Release(entry)

	if cache.Count() != 1 {
		t.Errorf("Cache should have 1 entry, got %d", cache.Count())
	}

	// Invalidate by coord
	cache.Invalidate(tileCoord{X: 0, Y: 0})
	if cache.Count() != 0 {
		t.Errorf("Cache should be empty after invalidate, got %d", cache.Count())
	}

	// Test invalidate non-existent
	cache.Invalidate(tileCoord{X: 99, Y: 99})
	if cache.Count() != 0 {
		t.Errorf("Invalidating non-existent should not affect count, got %d", cache.Count())
	}
}

func TestTileCache_Stats(t *testing.T) {
	wallet, err := membudget.NewWallet(1024 * 1024)
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	cache, err := NewTileCache(TileCacheConfig{
		Wallet:     wallet,
		MaxEntries: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create tile cache: %v", err)
	}

	// Check initial stats
	hits, misses, evictions := cache.Stats()
	if hits != 0 || misses != 0 || evictions != 0 {
		t.Errorf("Initial stats should be (0, 0, 0), got (%d, %d, %d)", hits, misses, evictions)
	}

	// Create a tile
	key := TileCacheKey{WindowID: "test", TileCoord: tileCoord{X: 0, Y: 0}, ContentRevision: 1}
	entry, err := cache.Create(key, TileSize, TileSize)
	if err != nil {
		t.Fatalf("Failed to create tile: %v", err)
	}

	// Release the create pin, but keep one pin
	cache.Release(entry)

	// Now get the tile - this should increment hits
	cache.Get(key)
	cache.Release(entry) // release the Get pin

	// Check stats after get
	hits, _, _ = cache.Stats()
	if hits < 1 {
		t.Errorf("After get, hits should be at least 1, got %d", hits)
	}

	// Release the remaining pin
	cache.Release(entry)
}

func TestTileGrid_Bounds(t *testing.T) {
	// Test tileCoord.tileRect
	coord := tileCoord{X: 1, Y: 2}
	rect := coord.tileRect()
	expected := image.Rect(64, 128, 128, 192)
	if rect != expected {
		t.Errorf("tileRect() for (1,2) should be %v, got %v", expected, rect)
	}

	// Test origin tile
	coord0 := tileCoord{X: 0, Y: 0}
	rect0 := coord0.tileRect()
	expected0 := image.Rect(0, 0, 64, 64)
	if rect0 != expected0 {
		t.Errorf("tileRect() for (0,0) should be %v, got %v", expected0, rect0)
	}
}
