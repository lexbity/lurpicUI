package assets

import "io/fs"

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
	Stats() ManagerStats
}

// PathIDRegistry resolves canonical asset paths to stable IDs.
type PathIDRegistry interface {
	Lookup(canonicalPath string) AssetID
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
