package assets

import "sync"

// DecodedAsset is the in-memory form committed into the registry.
// Later phases will use concrete backend-specific values behind this alias.
type DecodedAsset any

// AssetState enumerates the lifecycle state of one asset.
type AssetState uint8

const (
	AssetStateAbsent AssetState = iota
	AssetStateLoading
	AssetStateReady
	AssetStateFailed
)

// AssetReadySignal is emitted when an asset LOD transitions to ready.
type AssetReadySignal struct {
	ID  AssetID
	LOD int
}

// AssetInvalidatedSignal is emitted when an asset is invalidated.
type AssetInvalidatedSignal struct {
	ID AssetID
}

// AssetRegistryStore holds the load state of all active assets.
// Mutations are expected on the runtime thread; a mutex keeps the implementation safe.
type AssetRegistryStore struct {
	mu            sync.RWMutex
	entries       map[AssetID]*RegistryEntry
	signals       map[AssetID]*AssetSignalSet
	globalVersion uint64
}

// RegistryEntry tracks per-asset, per-LOD state including GPU residency.
type RegistryEntry struct {
	ID              AssetID
	Path            string
	Type            AssetType
	State           AssetState
	LODHandles      [3]DecodedAsset
	LODRefCounts    [3]int32
	LODInFlight     [3]bool
	LODFailed       [3]bool
	LODGPUReady     [3]bool
	LODTextureIDs   [3]TextureID
	LODGPUBytes     [3]int64
	SizeBytes       [3]int64
	HighestReadyLOD int
	EntryVersion    uint64
	Err             error
	LastUse         int64
	LoadTimeNs      [3]int64
}

// NewAssetRegistryStore returns an empty registry store.
func NewAssetRegistryStore() *AssetRegistryStore {
	return &AssetRegistryStore{
		entries: make(map[AssetID]*RegistryEntry),
		signals: make(map[AssetID]*AssetSignalSet),
	}
}

// GlobalVersion returns the current store version.
func (r *AssetRegistryStore) GlobalVersion() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.globalVersion
}

// GetOrCreate returns the entry for id, creating it if needed.
func (r *AssetRegistryStore) GetOrCreate(id AssetID) *RegistryEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.getOrCreateLocked(id)
}

// Get returns the entry for id, or nil if it does not exist.
func (r *AssetRegistryStore) Get(id AssetID) *RegistryEntry {
	r.mu.RLock()
	entry := r.entries[id]
	r.mu.RUnlock()
	return entry
}

// SetLODReady commits a decoded LOD result.
func (r *AssetRegistryStore) SetLODReady(id AssetID, lod int, handle DecodedAsset, loadNs int64) {
	if lod < 0 || lod >= len((&RegistryEntry{}).LODHandles) {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.getOrCreateLocked(id)
	entry.LODHandles[lod] = handle
	entry.LODInFlight[lod] = false
	entry.LODFailed[lod] = false
	entry.LoadTimeNs[lod] = loadNs
	entry.SizeBytes[lod] = 0
	entry.Err = nil
	if lod < entry.HighestReadyLOD || entry.HighestReadyLOD == -1 {
		entry.HighestReadyLOD = lod
	}
	if lod == 0 {
		entry.State = AssetStateReady
	} else if entry.State != AssetStateReady {
		entry.State = AssetStateLoading
	}
	entry.EntryVersion++
	r.globalVersion++

	if sig := r.signalSetLocked(id); sig != nil {
		sig.Ready.Emit(AssetReadySignal{ID: id, LOD: lod})
	}
}

// SetLODGPUReady marks a LOD as having a GPU texture uploaded. The texture
// ID and GPU byte size are recorded so consumers can select the GPU draw
// path and the cache can account for GPU memory.
func (r *AssetRegistryStore) SetLODGPUReady(id AssetID, lod int, textureID TextureID, gpuBytes int64) {
	if lod < 0 || lod >= len((&RegistryEntry{}).LODHandles) {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.entries[id]
	if entry == nil {
		return
	}
	if lod >= len(entry.LODGPUReady) {
		return
	}
	entry.LODGPUReady[lod] = true
	entry.LODTextureIDs[lod] = textureID
	entry.LODGPUBytes[lod] = gpuBytes
	entry.EntryVersion++
	r.globalVersion++
}

// ClearLOD clears one LOD result and updates state/version bookkeeping.
func (r *AssetRegistryStore) ClearLOD(id AssetID, lod int) {
	if lod < 0 || lod >= len((&RegistryEntry{}).LODHandles) {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.entries[id]
	if entry == nil {
		return
	}
	entry.LODHandles[lod] = nil
	entry.LODInFlight[lod] = false
	entry.LODFailed[lod] = false
	entry.LODGPUReady[lod] = false
	entry.LODTextureIDs[lod] = 0
	entry.LODGPUBytes[lod] = 0
	entry.LoadTimeNs[lod] = 0
	entry.SizeBytes[lod] = 0
	entry.Err = nil
	entry.HighestReadyLOD = recomputeHighestReadyLOD(entry.LODHandles)
	switch {
	case entry.HighestReadyLOD == -1:
		entry.State = AssetStateAbsent
	case entry.HighestReadyLOD == 0:
		entry.State = AssetStateReady
	default:
		entry.State = AssetStateLoading
	}
	entry.EntryVersion++
	r.globalVersion++
}

// Invalidate clears all LODs for the given asset and emits an invalidation signal.
func (r *AssetRegistryStore) Invalidate(id AssetID) {
	r.mu.Lock()
	entry := r.getOrCreateLocked(id)
	for i := range entry.LODHandles {
		entry.LODHandles[i] = nil
		entry.LODInFlight[i] = false
		entry.LODFailed[i] = false
		entry.LODGPUReady[i] = false
		entry.LODTextureIDs[i] = 0
		entry.LODGPUBytes[i] = 0
		entry.LoadTimeNs[i] = 0
		entry.SizeBytes[i] = 0
	}
	entry.HighestReadyLOD = -1
	entry.State = AssetStateAbsent
	entry.Err = nil
	entry.EntryVersion++
	r.globalVersion++
	sig := r.signalSetLocked(id)
	r.mu.Unlock()

	if sig != nil {
		sig.Invalidated.Emit(AssetInvalidatedSignal{ID: id})
	}
}

// SubscribeAsset registers callbacks for asset ready and invalidation signals.
// The returned release function is idempotent and decrements the asset refcount.
func (r *AssetRegistryStore) SubscribeAsset(id AssetID, onReady func(AssetReadySignal), onInvalidated func(AssetInvalidatedSignal)) func() {
	r.mu.Lock()
	entry := r.getOrCreateLocked(id)
	for i := range entry.LODRefCounts {
		entry.LODRefCounts[i]++
	}
	sig := r.signalSetLocked(id)
	if sig == nil {
		sig = newAssetSignalSet(id)
		r.signals[id] = sig
	}
	r.mu.Unlock()

	readyID := sig.Ready.Subscribe(func(sig AssetReadySignal) {
		if onReady != nil {
			onReady(sig)
		}
	})
	invalidID := sig.Invalidated.Subscribe(func(sig AssetInvalidatedSignal) {
		if onInvalidated != nil {
			onInvalidated(sig)
		}
	})

	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			if entry := r.entries[id]; entry != nil {
				for i := range entry.LODRefCounts {
					if entry.LODRefCounts[i] > 0 {
						entry.LODRefCounts[i]--
					}
				}
			}
			r.mu.Unlock()
			sig.Ready.Unsubscribe(readyID)
			sig.Invalidated.Unsubscribe(invalidID)
		})
	}
}

func (r *AssetRegistryStore) getOrCreateLocked(id AssetID) *RegistryEntry {
	if entry, ok := r.entries[id]; ok {
		return entry
	}
	entry := &RegistryEntry{
		ID:              id,
		State:           AssetStateAbsent,
		HighestReadyLOD: -1,
	}
	r.entries[id] = entry
	return entry
}

func (r *AssetRegistryStore) signalSetLocked(id AssetID) *AssetSignalSet {
	if sig, ok := r.signals[id]; ok {
		return sig
	}
	return nil
}

func recomputeHighestReadyLOD(handles [3]DecodedAsset) int {
	for i := range handles {
		if handles[i] != nil {
			return i
		}
	}
	return -1
}
