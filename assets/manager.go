package assets

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Manager is the runtime-facing asset access surface.
type Manager interface {
	fs.FS

	LoadSVG(path string) Handle
	LoadImage(path string) Handle
	LoadTexture(path string) Handle
	LoadFont(path string) Handle
	LoadConfig(path string, dst any) Handle
	Prefetch(paths ...string)
	Invalidate(path string)
	DrainCompleted() int
	Stats() ManagerStats
	Close() error
}

// PathIDRegistry resolves canonical asset paths to stable IDs.
type PathIDRegistry interface {
	Lookup(canonicalPath string) AssetID
}

// CanonicalizePath normalizes an asset path to the canonical slash-separated
// form used as the lookup key for path→ID resolution. The cook pipeline
// (build time, via UUIDRegistry) and the runtime PathIDRegistry MUST share
// this single function so that the keys written into uuid_registry.json match
// the queries issued at runtime — otherwise lookups silently miss and every
// path-based asset load returns an empty handle.
func CanonicalizePath(p string) (string, error) {
	cleaned := filepath.Clean(p)
	if cleaned == "." || cleaned == string(filepath.Separator) || cleaned == "" {
		return "", fmt.Errorf("invalid canonical path %q", p)
	}
	return filepath.ToSlash(cleaned), nil
}

// JSONPathRegistry implements PathIDRegistry from a JSON file produced by
// the cook pipeline's UUIDRegistry. The file format matches cook.AssetIDRecord.
type JSONPathRegistry struct {
	mu    sync.RWMutex
	paths map[string]AssetID
}

// pathIDRecord is the JSON structure for one entry in the registry.
type pathIDRecord struct {
	ID            AssetID `json:"id"`
	CreatedAt     int64   `json:"created_at,omitempty"`
	CanonicalPath string  `json:"canonical_path"`
}

// pathIDFile is the top-level JSON structure.
type pathIDFile struct {
	Version int            `json:"version"`
	Records []pathIDRecord `json:"records"`
}

// ParseJSONPathRegistry parses UUID registry JSON bytes and returns a
// PathIDRegistry backed by them. Keys are canonicalized via CanonicalizePath
// so they match runtime queries. The format matches the cook pipeline's
// uuid_registry.json output.
func ParseJSONPathRegistry(data []byte) (*JSONPathRegistry, error) {
	var f pathIDFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	r := &JSONPathRegistry{paths: make(map[string]AssetID, len(f.Records))}
	for _, rec := range f.Records {
		if rec.CanonicalPath == "" || rec.ID.IsZero() {
			continue
		}
		key, err := CanonicalizePath(rec.CanonicalPath)
		if err != nil {
			continue
		}
		r.paths[key] = rec.ID
	}
	return r, nil
}

// LoadJSONPathRegistry reads a UUID registry JSON file and returns a
// PathIDRegistry backed by it. The format matches the cook pipeline's
// uuid_registry.json output.
func LoadJSONPathRegistry(path string) (*JSONPathRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseJSONPathRegistry(data)
}

// NewMapPathRegistry creates a PathIDRegistry from a static map. Keys are
// canonicalized so lookups resolve regardless of the caller's path form.
func NewMapPathRegistry(m map[string]AssetID) *JSONPathRegistry {
	paths := make(map[string]AssetID, len(m))
	for k, v := range m {
		key, err := CanonicalizePath(k)
		if err != nil {
			continue
		}
		paths[key] = v
	}
	return &JSONPathRegistry{paths: paths}
}

func (r *JSONPathRegistry) Lookup(canonicalPath string) AssetID {
	if r == nil {
		return AssetID{}
	}
	key, err := CanonicalizePath(canonicalPath)
	if err != nil {
		return AssetID{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.paths[key]
}

// Handle is a lightweight reference to a registry entry.
type Handle struct {
	ID       AssetID
	registry *AssetRegistryStore
}

// NewHandle constructs a handle bound to a registry.
func NewHandle(id AssetID, registry *AssetRegistryStore) Handle {
	return Handle{ID: id, registry: registry}
}

// Registry exposes the registry backing the handle.
func (h Handle) Registry() *AssetRegistryStore {
	return h.registry
}

// AvailableLOD reports the best available LOD for the asset.
func (h Handle) AvailableLOD() int {
	if h.registry == nil || h.ID == (AssetID{}) {
		return -1
	}
	entry := h.registry.Get(h.ID)
	if entry == nil {
		return -1
	}
	return entry.HighestReadyLOD
}

// State returns the aggregate load state of the asset.
func (h Handle) State() AssetState {
	if h.registry == nil || h.ID == (AssetID{}) {
		return AssetStateAbsent
	}
	entry := h.registry.Get(h.ID)
	if entry == nil {
		return AssetStateAbsent
	}
	return entry.State
}

// Err returns the latest asset load error, if any.
func (h Handle) Err() error {
	if h.registry == nil || h.ID == (AssetID{}) {
		return nil
	}
	entry := h.registry.Get(h.ID)
	if entry == nil {
		return nil
	}
	return entry.Err
}

// IsZero reports whether the handle references an asset.
func (h Handle) IsZero() bool {
	return h.ID == (AssetID{}) || h.registry == nil
}

// ManagerStats summarizes the current asset system state.
type ManagerStats struct {
	TotalEntries       int
	LoadingEntries     int
	ReadyEntries       int
	PartialEntries     int
	FailedEntries      int
	CPUUsedBytes       int64
	CPUBudgetBytes     int64
	GPUUsedBytes       int64
	GPUBudgetBytes     int64
	EvictionsThisFrame int
	UploadsThisFrame   int
	JobsInFlight       int
	CacheHitRate       float64
	WaitingOnDeps      int
	Entries            []AssetDiagEntry
}

// AssetDiagEntry is a diagnostic snapshot of one asset entry.
type AssetDiagEntry struct {
	ID              AssetID
	Path            string
	State           AssetState
	HighestReadyLOD int
	RefCounts       [3]int32
	SizeBytes       [3]int64
	LODGPUReady     [3]bool
	LODGPUBytes     [3]int64
	LoadTimeNs      [3]int64
	LastUsedFrame   int64
}

// ManagerConfig controls budgets and concurrency.
type ManagerConfig struct {
	MemoryBudgetBytes         int64
	GPUMemoryBudgetBytes      int64
	UploadBudgetBytesPerFrame int
	WorkerCount               int
}
