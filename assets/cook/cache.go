package cook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const cookCacheFileVersion = 1

// CookCache tracks source content hashes to skip unchanged assets.
// Persisted as .lurpic-cache/cook-cache.json. Not committed to version control.
type CookCache struct {
	CookerVersion string
	Target        string
	SourceHashes  map[string][32]byte
	CookedHashes  map[string][32]byte

	manifestHash [32]byte
}

type cookCacheDisk struct {
	Version       int                  `json:"version"`
	CookerVersion string               `json:"cooker_version"`
	Target        string               `json:"target"`
	ManifestHash  string               `json:"manifest_hash"`
	SourceHashes  []cookCacheHashEntry `json:"source_hashes"`
	CookedHashes  []cookCacheHashEntry `json:"cooked_hashes"`
}

type cookCacheHashEntry struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

// IncrementalSource is one source asset handled by the incremental cooker helper.
type IncrementalSource struct {
	Path    string
	Read    func() ([]byte, error)
	Compile func([]byte) ([]CompiledLOD, error)
}

// IncrementalCooker reuses previous cooked outputs when source hashes are unchanged.
type IncrementalCooker struct {
	Cache   *CookCache
	outputs map[string][]CompiledLOD
}

// NewCookCache returns an initialized cache.
func NewCookCache() *CookCache {
	return &CookCache{
		SourceHashes: make(map[string][32]byte),
		CookedHashes: make(map[string][32]byte),
	}
}

// DefaultCookCachePath returns the repository-local cache file path.
func DefaultCookCachePath() string {
	return filepath.Join(".lurpic-cache", "cook-cache.json")
}

// LoadCookCache loads the persisted cache from path.
func LoadCookCache(path string) (*CookCache, error) {
	cache := NewCookCache()

	data, err := os.ReadFile(path) //nolint:gosec // path from user config
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cache, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return cache, nil
	}

	var disk cookCacheDisk
	if err := json.Unmarshal(data, &disk); err != nil {
		return nil, fmt.Errorf("parse cook cache: %w", err)
	}
	if disk.Version != 0 && disk.Version != cookCacheFileVersion {
		return nil, fmt.Errorf("unsupported cook cache version %d", disk.Version)
	}

	cache.CookerVersion = disk.CookerVersion
	cache.Target = disk.Target
	if disk.ManifestHash != "" {
		manifestHash, err := decodeCacheHash(disk.ManifestHash)
		if err != nil {
			return nil, fmt.Errorf("decode manifest hash: %w", err)
		}
		cache.manifestHash = manifestHash
	}
	if cache.SourceHashes == nil {
		cache.SourceHashes = make(map[string][32]byte)
	}
	if cache.CookedHashes == nil {
		cache.CookedHashes = make(map[string][32]byte)
	}
	for _, entry := range disk.SourceHashes {
		canonical, err := canonicalizePath(entry.Path)
		if err != nil {
			return nil, fmt.Errorf("canonicalize source hash path %q: %w", entry.Path, err)
		}
		hash, err := decodeCacheHash(entry.Hash)
		if err != nil {
			return nil, fmt.Errorf("decode source hash for %q: %w", entry.Path, err)
		}
		cache.SourceHashes[canonical] = hash
	}
	for _, entry := range disk.CookedHashes {
		canonical, err := canonicalizePath(entry.Path)
		if err != nil {
			return nil, fmt.Errorf("canonicalize cooked hash path %q: %w", entry.Path, err)
		}
		hash, err := decodeCacheHash(entry.Hash)
		if err != nil {
			return nil, fmt.Errorf("decode cooked hash for %q: %w", entry.Path, err)
		}
		cache.CookedHashes[canonical] = hash
	}
	return cache, nil
}

// Save writes the cache to path.
func (c *CookCache) Save(path string) error {
	if c == nil {
		return errors.New("nil cook cache")
	}
	if path == "" {
		return errors.New("empty cook cache path")
	}
	if c.SourceHashes == nil {
		c.SourceHashes = make(map[string][32]byte)
	}
	if c.CookedHashes == nil {
		c.CookedHashes = make(map[string][32]byte)
	}

	disk := cookCacheDisk{
		Version:       cookCacheFileVersion,
		CookerVersion: c.CookerVersion,
		Target:        c.Target,
		ManifestHash:  encodeCacheHash(c.manifestHash),
		SourceHashes:  marshalCacheHashes(c.SourceHashes),
		CookedHashes:  marshalCacheHashes(c.CookedHashes),
	}
	payload, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')

	//nolint:gosec // cache dir
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// EnsureInvalidation clears the cache when cooker metadata changes.
// It returns true when a full invalidation occurred.
func (c *CookCache) EnsureInvalidation(version, target string, manifestHash [32]byte) bool {
	if c == nil {
		return false
	}
	if c.SourceHashes == nil {
		c.SourceHashes = make(map[string][32]byte)
	}
	if c.CookedHashes == nil {
		c.CookedHashes = make(map[string][32]byte)
	}
	if c.CookerVersion == version && c.Target == target && c.manifestHash == manifestHash {
		return false
	}
	c.CookerVersion = version
	c.Target = target
	c.manifestHash = manifestHash
	clear(c.SourceHashes)
	clear(c.CookedHashes)
	return true
}

// HashBytes returns the SHA-256 hash of src.
func HashBytes(src []byte) [32]byte {
	return sha256.Sum256(src)
}

// HashFile returns the SHA-256 hash of the file at path.
func HashFile(path string) ([32]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path from user config
	if err != nil {
		return [32]byte{}, err
	}
	return HashBytes(data), nil
}

// SourceHash returns the cached source hash for path.
func (c *CookCache) SourceHash(path string) ([32]byte, bool) {
	if c == nil {
		return [32]byte{}, false
	}
	canonical, err := canonicalizePath(path)
	if err != nil {
		return [32]byte{}, false
	}
	h, ok := c.SourceHashes[canonical]
	return h, ok
}

// ShouldReuse reports whether path can reuse the previously cooked output.
func (c *CookCache) ShouldReuse(path string, sourceHash [32]byte) bool {
	if c == nil {
		return false
	}
	canonical, err := canonicalizePath(path)
	if err != nil {
		return false
	}
	old, ok := c.SourceHashes[canonical]
	if !ok || old != sourceHash {
		return false
	}
	_, ok = c.CookedHashes[canonical]
	return ok
}

// Record stores the latest source and cooked hashes for path.
func (c *CookCache) Record(path string, sourceHash, cookedHash [32]byte) {
	if c == nil {
		return
	}
	canonical, err := canonicalizePath(path)
	if err != nil {
		return
	}
	if c.SourceHashes == nil {
		c.SourceHashes = make(map[string][32]byte)
	}
	if c.CookedHashes == nil {
		c.CookedHashes = make(map[string][32]byte)
	}
	c.SourceHashes[canonical] = sourceHash
	c.CookedHashes[canonical] = cookedHash
}

// Delete removes an asset from the cache.
func (c *CookCache) Delete(path string) {
	if c == nil {
		return
	}
	canonical, err := canonicalizePath(path)
	if err != nil {
		return
	}
	delete(c.SourceHashes, canonical)
	delete(c.CookedHashes, canonical)
}

// NewIncrementalCooker returns a cooker helper that reuses outputs across runs.
func NewIncrementalCooker(cache *CookCache) *IncrementalCooker {
	if cache == nil {
		cache = NewCookCache()
	}
	return &IncrementalCooker{
		Cache:   cache,
		outputs: make(map[string][]CompiledLOD),
	}
}

// Cook compiles sources only when their source hash changed or the cache was invalidated.
func (c *IncrementalCooker) Cook(version string, target Platform, manifestHash [32]byte, sources []IncrementalSource) (map[string][]CompiledLOD, error) {
	if c == nil {
		return nil, errors.New("nil incremental cooker")
	}
	if c.Cache == nil {
		c.Cache = NewCookCache()
	}
	if c.outputs == nil {
		c.outputs = make(map[string][]CompiledLOD)
	}

	if c.Cache.EnsureInvalidation(version, string(target), manifestHash) {
		clear(c.outputs)
	}

	results := make(map[string][]CompiledLOD, len(sources))
	seen := make(map[string]struct{}, len(sources))

	sorted := append([]IncrementalSource(nil), sources...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	for _, src := range sorted {
		canonical, err := canonicalizePath(src.Path)
		if err != nil {
			return nil, fmt.Errorf("canonicalize %q: %w", src.Path, err)
		}
		if src.Read == nil {
			return nil, fmt.Errorf("source %q has no reader", src.Path)
		}
		if src.Compile == nil {
			return nil, fmt.Errorf("source %q has no compiler", src.Path)
		}

		data, err := src.Read()
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", src.Path, err)
		}
		sourceHash := HashBytes(data)
		seen[canonical] = struct{}{}

		if c.Cache.ShouldReuse(canonical, sourceHash) {
			if cached, ok := c.outputs[canonical]; ok {
				results[canonical] = cloneCompiledLODs(cached)
				continue
			}
		}

		lods, err := src.Compile(data)
		if err != nil {
			return nil, fmt.Errorf("compile %q: %w", src.Path, err)
		}
		if len(lods) == 0 {
			return nil, fmt.Errorf("compile %q produced no LODs", src.Path)
		}
		cached := cloneCompiledLODs(lods)
		c.outputs[canonical] = cached
		results[canonical] = cloneCompiledLODs(cached)
		c.Cache.Record(canonical, sourceHash, HashBytes(lods[0].Data))
	}

	for path := range c.outputs {
		if _, ok := seen[path]; !ok {
			delete(c.outputs, path)
			c.Cache.Delete(path)
		}
	}
	return results, nil
}

func cloneCompiledLODs(src []CompiledLOD) []CompiledLOD {
	if len(src) == 0 {
		return nil
	}
	out := make([]CompiledLOD, len(src))
	for i := range src {
		out[i].Level = src[i].Level
		out[i].Data = append([]byte(nil), src[i].Data...)
	}
	return out
}

func marshalCacheHashes(m map[string][32]byte) []cookCacheHashEntry {
	if len(m) == 0 {
		return nil
	}
	entries := make([]cookCacheHashEntry, 0, len(m))
	for path, hash := range m {
		entries = append(entries, cookCacheHashEntry{
			Path: path,
			Hash: encodeCacheHash(hash),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}

func encodeCacheHash(hash [32]byte) string {
	return hex.EncodeToString(hash[:])
}

func decodeCacheHash(spec string) ([32]byte, error) {
	var out [32]byte
	if spec == "" {
		return out, nil
	}
	data, err := hex.DecodeString(spec)
	if err != nil {
		return out, err
	}
	if len(data) != len(out) {
		return out, fmt.Errorf("expected %d hash bytes, got %d", len(out), len(data))
	}
	copy(out[:], data)
	return out, nil
}
