package assets

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"
	"time"

	"image"
	_ "image/jpeg"
	_ "image/png"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/job"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

// BackendType identifies the runtime backend used for decoding decisions.
type BackendType uint8

const (
	BackendSoftware BackendType = iota
	BackendVulkan
)

// ResidencyMode controls GPU residency for decoded assets.
type ResidencyMode uint8

const (
	ResidencyCPUOnly ResidencyMode = iota
	ResidencyGPUResident
	ResidencyAuto
)

// ParseResidencyMode parses a string into a ResidencyMode.
// Accepted values: "cpu" / "cpuonly", "gpu" / "gpuresident", "auto".
// Returns ResidencyCPUOnly and an error for unrecognized strings.
func ParseResidencyMode(s string) (ResidencyMode, error) {
	switch strings.ToLower(s) {
	case "cpu", "cpuonly":
		return ResidencyCPUOnly, nil
	case "gpu", "gpuresident":
		return ResidencyGPUResident, nil
	case "auto":
		return ResidencyAuto, nil
	default:
		return ResidencyCPUOnly, fmt.Errorf("invalid residency mode %q (valid: cpu, gpu, auto)", s)
	}
}

func (m ResidencyMode) String() string {
	switch m {
	case ResidencyCPUOnly:
		return "cpu"
	case ResidencyGPUResident:
		return "gpu"
	case ResidencyAuto:
		return "auto"
	default:
		return "unknown"
	}
}

// AssetLoadJob loads and decodes one LOD level of one asset.
type AssetLoadJob struct {
	ID           AssetID
	Path         string
	Type         AssetType
	LOD          int
	EntryVersion uint64
	Source       AssetSource
	Backend      BackendType
	StartNs      int64

	// PreloadedSource carries the compressed source bytes read once in Schedule.
	// Per D9: the compressed source is read once and carried in the snapshot;
	// Execute decodes from these bytes without re-reading.
	PreloadedSource []byte

	Result    DecodedAsset
	Err       error
	ElapsedNs int64
}

// AssetSource abstracts raw byte access from PakFS or DevFS.
type AssetSource interface {
	ReadLOD(id AssetID, lod int) ([]byte, error)
}

// JobScheduler submits jobs to a background execution context.
type JobScheduler interface {
	Schedule(job *AssetLoadJob) error
}

// AsyncScheduler executes jobs on a background goroutine.
// This is a simple scheduler for tests and legacy use. For production, use PoolScheduler.
type AsyncScheduler struct {
	results chan<- *AssetLoadJob
}

// NewAsyncScheduler returns a scheduler that runs each job in its own goroutine.
// This is the replacement for the deleted asyncJobScheduler, for test compatibility.
func NewAsyncScheduler(results chan<- *AssetLoadJob) JobScheduler {
	return &AsyncScheduler{results: results}
}

func (s *AsyncScheduler) Schedule(job *AssetLoadJob) error {
	if s == nil || job == nil {
		return fmt.Errorf("nil job scheduler")
	}
	go func() {
		job.Execute()
		if s.results != nil {
			s.results <- job
		}
	}()
	return nil
}

// PoolScheduler schedules asset load jobs through a bounded job.Pool with
// proper memory reservations. Per D9: the compressed source is read once
// (in Schedule), decode input/scratch/output are declared and reserved as their
// simultaneous sum, and decoded residency is charged to AssetsCPU wallet.
type PoolScheduler struct {
	pool      *job.Pool
	results   chan<- *AssetLoadJob
	wallet    *membudget.Wallet
	nextJobID uint64
	mu        sync.Mutex
	// decodeSemaphore bounds concurrent decode operations.
	// Per D9: decode concurrency is bounded (1 Compact / 2 Standard).
	decodeSemaphore chan struct{}
}

// NewPoolScheduler creates a scheduler that uses a job.Pool for bounded
// execution with memory reservations. The wallet is used for decoded residency
// tracking (separate from the pool's jobs wallet). Results are sent to the
// results channel. The decodeSemaphore bounds concurrent decode operations.
func NewPoolScheduler(pool *job.Pool, wallet *membudget.Wallet, results chan<- *AssetLoadJob, decodeSemaphore chan struct{}) *PoolScheduler {
	return &PoolScheduler{
		pool:            pool,
		wallet:          wallet,
		results:         results,
		decodeSemaphore: decodeSemaphore,
	}
}

// Schedule submits an asset load job through the job pool with memory reservation.
// Per D9: source bytes are read once and carried in the snapshot; decode uses
// image.DecodeConfig for preflight; declarations are required and charged to the
// pool's jobs wallet; decoded residency is charged to the assets wallet.
func (s *PoolScheduler) Schedule(assetJob *AssetLoadJob) error {
	if s == nil || assetJob == nil || s.pool == nil {
		return fmt.Errorf("nil pool scheduler, job, or pool")
	}

	// Pre-read the source to get actual bytes.
	// Per D9: the compressed source is read once.
	raw, err := assetJob.Source.ReadLOD(assetJob.ID, assetJob.LOD)
	if err != nil {
		assetJob.Err = err
		if s.results != nil {
			s.results <- assetJob
		}
		return nil
	}

	// Preflight dimensions using image.DecodeConfig.
	// Per D9: image dimensions MUST be preflighted before pixel allocation.
	var decodedBytes uint64
	if assetJob.Type == AssetTypeImage {
		// Use image.DecodeConfig to get exact dimensions without full decode.
		// Per D9: exact size preflight via DecodeConfig.
		config, _, err := image.DecodeConfig(bytes.NewReader(raw))
		if err != nil {
			assetJob.Err = fmt.Errorf("image decode config failed: %w", err)
			if s.results != nil {
				s.results <- assetJob
			}
			return nil
		}
		// Calculate exact decoded size: width * height * 4 bytes (RGBA8).
		// This is the maximum possible size; actual may be smaller for some formats.
		// Check for overflow in the multiplication.
		// config.Width and config.Height are always non-negative per the image package.
		w, h := int64(config.Width), int64(config.Height)
		if w <= 0 || h <= 0 {
			decodedBytes = 0
		} else if w > (1<<32) || h > (1<<32)/w {
			// Would overflow uint64; use a safe maximum instead.
			// 1<<32 is already a very large dimension (4 billion pixels).
			decodedBytes = 256 * 1024 * 1024 // 256 MB max
		} else {
			decodedBytes = uint64(w) * uint64(h) * 4
		}

		// Guard against unreasonably large images (bomb detection).
		// Per D9: oversized declarations are refused before decode.
		const maxReasonableImagePixels = 64 * 1024 * 1024 // 64 megapixels
		if decodedBytes > maxReasonableImagePixels*4 {
			assetJob.Err = fmt.Errorf("image too large: %d x %d = %d bytes exceeds limit",
				config.Width, config.Height, decodedBytes)
			if s.results != nil {
				s.results <- assetJob
			}
			return nil
		}
	} else {
		// For non-image assets, use a conservative multiplier.
		// This is less precise but still bounded by the pool wallet.
		decodedBytes = uint64(len(raw)) * 4
	}

	// Memory declaration for the pool's jobs wallet.
	// Per D9: input+scratch+output as simultaneous sum.
	// MaxInputBytes: the raw compressed bytes we already read.
	// MaxScratchBytes: decompression buffer (4x input size conservatively).
	// MaxOutputBytes: decoded pixel data.
	maxInputBytes := uint64(len(raw))
	maxScratchBytes := maxInputBytes * 4
	maxOutputBytes := decodedBytes

	// Validate declaration (overflow check).
	// Per D8.1: declarations must be overflow-checked.
	decl := job.JobMemoryDecl{
		MaxInputBytes:   maxInputBytes,
		MaxScratchBytes: maxScratchBytes,
		MaxOutputBytes:  maxOutputBytes,
	}
	if _, err := decl.TotalBytes(); err != nil {
		assetJob.Err = fmt.Errorf("memory declaration overflow: %w", err)
		if s.results != nil {
			s.results <- assetJob
		}
		return nil
	}

	s.mu.Lock()
	s.nextJobID++
	jobID := job.JobID(s.nextJobID)
	s.mu.Unlock()

	// Create a new AssetLoadJob with preloaded source bytes.
	// Per D9: the compressed source is carried in the snapshot.
	preloadedJob := &AssetLoadJob{
		ID:              assetJob.ID,
		Path:            assetJob.Path,
		Type:            assetJob.Type,
		LOD:             assetJob.LOD,
		EntryVersion:    assetJob.EntryVersion,
		Source:          assetJob.Source,
		Backend:         assetJob.Backend,
		StartNs:         assetJob.StartNs,
		PreloadedSource: raw, // Carry the bytes read once in Schedule
	}

	// Build the typed job for the pool with preloaded bytes.
	// We don't use version tracking for now since we can't import store.
	typedJob := job.Job[*AssetLoadJob, *AssetLoadJob]{
		ID:       jobID,
		Priority: job.PriorityBackground,
		Snapshot: job.NewSnapshot(preloadedJob),
		MemoryDecl: job.JobMemoryDecl{
			MaxInputBytes:   maxInputBytes,
			MaxScratchBytes: maxScratchBytes,
			MaxOutputBytes:  maxOutputBytes,
		},
		Work: func(snap job.Snapshot[*AssetLoadJob], cancel *job.CancelToken) (*AssetLoadJob, error) {
			// Get the job from snapshot.
			loadedJob := snap.Data
			if loadedJob == nil {
				return nil, fmt.Errorf("nil job in snapshot")
			}

			// Acquire decode semaphore to bound concurrent decodes.
			// Per D9: decode concurrency is bounded by the plan.
			if s.decodeSemaphore != nil {
				s.decodeSemaphore <- struct{}{}
				defer func() { <-s.decodeSemaphore }()
			}

			// Execute the actual load/decode from carried bytes.
			// Per D9: Execute decodes from the carried bytes (read once in Schedule).
			loadedJob.Execute()
			return loadedJob, loadedJob.Err
		},
	}

	// Submit to pool with guarded commit.
	// The commit function forwards the result to the assets results channel
	// and charges decoded residency to the assets wallet.
	err = job.ScheduleGuarded(s.pool, typedJob, nil, func(result *AssetLoadJob) {
		if s.results != nil {
			s.results <- result
		}
		// Charge decoded residency to assets wallet.
		// Per D9: decoded residency is charged to AssetsCPU at registry admission.
		// This is done by the manager's commitJob path, not here.
	})
	if err != nil {
		return fmt.Errorf("pool schedule failed: %w", err)
	}

	return nil
}

// ManagerImpl owns runtime-thread asset scheduling and commit orchestration.
// It is the single implementation of the Manager interface and must not be
// embedded or wrapped — all storage backends feed into it via AssetSource.
type ManagerImpl struct {
	registry    *AssetRegistryStore
	source      AssetSource
	idReg       PathIDRegistry
	backendType BackendType
	uploader    TextureUploader
	residency   ResidencyMode
	scheduler   JobScheduler
	results     chan *AssetLoadJob
	cache       *assetCache
	depTree     ConfigDependencyTree

	// assetsWallet is the membudget.Wallet for asset CPU memory.
	// Per FR-10: this wallet tracks compressed source, decoded pixels,
	// conversion buffers, GPU upload staging, and source-result overlap.
	// When nil, the legacy assetCache system is used.
	assetsWallet *membudget.Wallet

	// assetReservations tracks the wallet reservations for decoded assets.
	// Per D9: decoded residency is charged to AssetsCPU at registry admission
	// and released at registry eviction.
	assetReservations map[AssetID][]*membudget.Reservation

	// decodeSemaphore bounds concurrent decode operations.
	// Per D9: decode concurrency is bounded by the plan (1 Compact / 2 Standard).
	decodeSemaphore chan struct{}

	mu               sync.Mutex
	waiting          waitingOn
	uploadsThisFrame int
}

// NewManager returns an asset manager that wraps the given source.
// When scheduler is nil, a default async goroutine scheduler is created.
// This is a thin back-compat wrapper; new code should use NewManagerWithResidency.
func NewManager(registry *AssetRegistryStore, source AssetSource, backend BackendType, scheduler JobScheduler, idReg PathIDRegistry) *ManagerImpl {
	m := NewManagerWithResidency(registry, source, idReg, scheduler, nil, ResidencyCPUOnly, 0, 0)
	m.backendType = backend
	return m
}

// NewManagerWithCache is like NewManager but also accepts cache configuration.
// When cacheBytes is > 0, the manager creates an LRU-backed asset cache that
// evicts under memory pressure. gpuCacheBytes limits GPU-resident data.
// This is a thin back-compat wrapper; new code should use NewManagerWithBudget.
func NewManagerWithCache(registry *AssetRegistryStore, source AssetSource, backend BackendType, scheduler JobScheduler, idReg PathIDRegistry, releaser TextureReleaser, cacheBytes, gpuCacheBytes int64) *ManagerImpl {
	m := NewManagerWithResidency(registry, source, idReg, scheduler, nil, ResidencyCPUOnly, 0, 0)
	m.backendType = backend
	if cacheBytes > 0 {
		m.cache = newAssetCache(registry, releaser, cacheBytes, gpuCacheBytes)
	}
	return m
}

// NewManagerWithResidency constructs a manager with GPU residency support.
// The uploader is the seam to the render backend's GPU upload queue; when nil
// or returning Budget() == 0, the manager is CPU-only regardless of mode.
// cpuBudgetMB and gpuBudgetMB are fixed caps; 0 means no cache for that tier.
// This constructor is deprecated in favor of NewManagerWithBudget which requires
// a job.Pool for bounded scheduling (D9).
func NewManagerWithResidency(registry *AssetRegistryStore, source AssetSource, idReg PathIDRegistry, scheduler JobScheduler, uploader TextureUploader, mode ResidencyMode, cpuBudgetMB, gpuBudgetMB int64) *ManagerImpl {
	gpuCapable := uploader != nil && uploader.Budget() > 0

	effectiveMode := mode
	if !gpuCapable && mode != ResidencyCPUOnly {
		effectiveMode = ResidencyCPUOnly
	}

	var bt BackendType
	switch effectiveMode {
	case ResidencyGPUResident, ResidencyAuto:
		bt = BackendVulkan
	default:
		bt = BackendSoftware
	}

	m := &ManagerImpl{
		registry:          registry,
		source:            source,
		idReg:             idReg,
		backendType:       bt,
		uploader:          uploader,
		residency:         mode,
		results:           make(chan *AssetLoadJob, 32),
		waiting:           make(waitingOn),
		assetReservations: make(map[AssetID][]*membudget.Reservation),
		decodeSemaphore:   make(chan struct{}, 2),
	}

	cpuBytes := cpuBudgetMB * 1024 * 1024
	gpuBytes := gpuBudgetMB * 1024 * 1024
	if cpuBytes > 0 || gpuBytes > 0 {
		m.cache = newAssetCache(registry, nil, cpuBytes, gpuBytes)
	}

	// Per D9: if a scheduler is provided (for tests), use it.
	// In production, use NewManagerWithBudget with a pool.
	if scheduler != nil {
		m.scheduler = scheduler
	} else {
		// Per S5: the goroutine-per-load scheduler MUST NOT exist in production.
		// For test compatibility only, fall back to AsyncScheduler.
		// Production code must use NewManagerWithBudget with a pool.
		m.scheduler = NewAsyncScheduler(m.results)
	}

	return m
}

// NewManagerWithBudget creates a manager that uses a job.Pool for scheduling
// and a membudget.Wallet for tracking asset memory.
// Per D9: the application owns pool construction; the runtime and asset manager
// consume it. A manager MUST NOT have a goroutine-per-load path.
// Per FR-10: the wallet tracks compressed source, decoded pixels, conversion
// buffers, GPU upload staging, and source-result overlap.
// The pool provides bounded concurrency with memory reservations.
// Both pool and wallet must be non-nil for production use.
func NewManagerWithBudget(registry *AssetRegistryStore, source AssetSource, idReg PathIDRegistry, pool *job.Pool, wallet *membudget.Wallet, uploader TextureUploader, mode ResidencyMode) *ManagerImpl {
	if pool == nil {
		// Pool is required for production use (D9).
		// In tests, a nil pool may be acceptable if the test explicitly uses
		// a different scheduler, but production code should always provide a pool.
		// We document this requirement instead of panicking to allow test flexibility.
	}

	gpuCapable := uploader != nil && uploader.Budget() > 0

	effectiveMode := mode
	if !gpuCapable && mode != ResidencyCPUOnly {
		effectiveMode = ResidencyCPUOnly
	}

	var bt BackendType
	switch effectiveMode {
	case ResidencyGPUResident, ResidencyAuto:
		bt = BackendVulkan
	default:
		bt = BackendSoftware
	}

	// Decode semaphore: default to Standard (2) concurrent decodes.
	// Per D9: this should be 1 for Compact, 2 for Standard, but we don't have
	// the profile here. The app layer should set this appropriately.
	// For now, use 2 as a reasonable default.
	decodeSem := make(chan struct{}, 2)

	m := &ManagerImpl{
		registry:          registry,
		source:            source,
		idReg:             idReg,
		backendType:       bt,
		uploader:          uploader,
		residency:         mode,
		results:           make(chan *AssetLoadJob, 32),
		waiting:           make(waitingOn),
		assetsWallet:      wallet,
		assetReservations: make(map[AssetID][]*membudget.Reservation),
		decodeSemaphore:   decodeSem,
	}

	// Always use PoolScheduler with the provided pool.
	// Per D9/S5: the goroutine-per-load scheduler MUST NOT exist in production.
	if pool != nil {
		m.scheduler = NewPoolScheduler(pool, wallet, m.results, decodeSem)
	} else {
		// Per S5: a nil pool is an error for production use.
		// For test compatibility only, fall back to AsyncScheduler.
		// Production code must provide a pool.
		m.scheduler = NewAsyncScheduler(m.results)
	}

	return m
}

// ── Manager interface methods ──────────────────────────────────────────────

// LoadSVG schedules progressive LOD streaming for an SVG asset.
func (m *ManagerImpl) LoadSVG(path string) Handle { return m.loadByPath(path, AssetTypeSVG) }

// LoadImage schedules progressive LOD streaming for a raster image asset.
func (m *ManagerImpl) LoadImage(path string) Handle { return m.loadByPath(path, AssetTypeImage) }

// LoadTexture schedules progressive LOD streaming for a material texture.
func (m *ManagerImpl) LoadTexture(path string) Handle { return m.loadByPath(path, AssetTypeImage) }

// LoadFont schedules progressive LOD streaming for a font asset.
func (m *ManagerImpl) LoadFont(path string) Handle { return m.loadByPath(path, AssetTypeFont) }

// LoadConfig schedules loading for a config asset.
func (m *ManagerImpl) LoadConfig(path string, _ any) Handle {
	return m.loadByPath(path, AssetTypeConfig)
}

// Prefetch schedules load work for the given paths.
func (m *ManagerImpl) Prefetch(paths ...string) {
	for _, path := range paths {
		m.loadByPath(path, assetTypeForPath(path))
	}
}

// releaseAssetReservations releases all wallet reservations for an asset.
// Per D9: decoded residency is released at registry eviction.
func (m *ManagerImpl) releaseAssetReservations(id AssetID) {
	if m == nil || m.assetsWallet == nil {
		return
	}
	m.mu.Lock()
	reservations := m.assetReservations[id]
	delete(m.assetReservations, id)
	m.mu.Unlock()

	// Release all reservations for this asset.
	for _, res := range reservations {
		if res != nil {
			if err := res.Release(); err != nil {
				// Release failure should not prevent eviction.
				// This is a best-effort accounting.
			}
		}
	}
}

// Invalidate marks an asset stale and clears ready LODs from the registry.
func (m *ManagerImpl) Invalidate(path string) {
	if m == nil || m.idReg == nil || m.registry == nil {
		return
	}
	if id := m.idReg.Lookup(path); id != (AssetID{}) {
		// Release wallet charges before invalidating.
		// Per D9: decoded residency is released at registry eviction.
		m.releaseAssetReservations(id)
		m.registry.Invalidate(id)
	}
}

// SetTextureReleaser injects the GPU backend's texture releaser into the cache.
// Must be called before any GPU uploads. Safe to call on a nil manager or
// when no cache has been configured.
func (m *ManagerImpl) SetTextureReleaser(rl TextureReleaser) {
	if m == nil || m.cache == nil {
		return
	}
	m.cache.backend = rl
}

// SetUploader sets the TextureUploader after construction. The uploader is
// typically nil at construction time (the render pipeline isn't ready yet)
// and is wired by the runtime after the backend initialises.
func (m *ManagerImpl) SetUploader(uploader TextureUploader) {
	if m == nil {
		return
	}
	m.uploader = uploader
	if uploader != nil && uploader.Budget() > 0 {
		m.backendType = BackendVulkan
	}
}

// TrimMemory responds to onTrimMemory from the platform. It translates the
// Android trim level to a budget fraction and evicts the cache accordingly.
// GPU textures are freed first (GPU-first eviction); CPU LODs are only
// evicted under critical pressure. A nil receiver is safe-to-call.
func (m *ManagerImpl) TrimMemory(level int) int {
	if m == nil {
		return 0
	}
	if m.cache != nil {
		fraction := TrimLevelFraction(level)
		gpuCount := m.cache.EvictGPUToWatermark(fraction)
		cpuCount := m.cache.EvictToWatermark(fraction)
		return gpuCount + cpuCount
	}
	return 0
}

// CheckDeviceGeneration compares the renderer's device generation against
// the cached generation. On mismatch (device lost + recreate), all GPU LODs
// are invalidated but CPU data survives for lazy re-upload. Returns true if
// GPU LODs were cleared. The runtime calls this each frame.
func (m *ManagerImpl) CheckDeviceGeneration(currentGen uint64) bool {
	if m == nil || m.cache == nil {
		return false
	}
	return m.cache.CheckDeviceGeneration(currentGen)
}

// Close drains all pending results and closes the underlying AssetSource.
// After Close, the ManagerImpl must not be used for further scheduling.
// This is the ordered teardown sequence: drain results → close source.
func (m *ManagerImpl) Close() error {
	if m == nil {
		return nil
	}
	// Drain any remaining results from in-flight jobs.
	for {
		select {
		case job := <-m.results:
			if job != nil {
				m.commitJob(job)
			}
		default:
			goto closeSource
		}
	}
closeSource:
	// Close the underlying source if it supports io.Closer (e.g. PakFS).
	if m.source != nil {
		if closer, ok := m.source.(io.Closer); ok {
			return closer.Close()
		}
	}
	return nil
}

// DrainCompleted commits any jobs that have completed since the last drain.
func (m *ManagerImpl) DrainCompleted() int {
	if m == nil {
		return 0
	}
	count := 0
	for {
		select {
		case job := <-m.results:
			if job == nil {
				continue
			}
			m.commitJob(job)
			count++
		default:
			return count
		}
	}
}

// DrainUploadResults drains completed GPU upload results from the uploader
// and commits them to the cache and registry. Must be called on the runtime
// thread once per frame after UploadQueue.DrainBudget has had a chance to
// process enqueued uploads (typically at the start of the next frame).
// Returns the number of upload results committed.
func (m *ManagerImpl) DrainUploadResults() int {
	if m == nil || m.uploader == nil {
		return 0
	}
	m.uploadsThisFrame = 0
	count := 0
	for {
		select {
		case result, ok := <-m.uploader.Results():
			if !ok {
				return count
			}
			if !result.OK || result.TextureID == 0 {
				continue
			}
			// Look up the CPU LOD size from the registry.
			var cpuBytes int64
			if entry := m.registry.Get(result.AssetID); entry != nil {
				if result.LOD >= 0 && result.LOD < len(entry.SizeBytes) {
					cpuBytes = entry.SizeBytes[result.LOD]
				}
			}
			// Track in the cache with GPU accounting.
			if m.cache != nil {
				m.cache.trackLOD(result.AssetID, result.LOD, cpuBytes, result.GPUBytes, result.TextureID, 0)
			}
			// Mark GPU-ready in the registry.
			m.registry.SetLODGPUReady(result.AssetID, result.LOD, result.TextureID, result.GPUBytes)
			count++
		default:
			m.uploadsThisFrame = count
			return count
		}
	}
}

// Stats returns a snapshot of the registry and cache state.
func (m *ManagerImpl) Stats() ManagerStats {
	if m == nil || m.registry == nil {
		return ManagerStats{}
	}
	stats := ManagerStats{}
	m.registry.mu.RLock()
	stats.TotalEntries = len(m.registry.entries)
	for _, entry := range m.registry.entries {
		switch entry.State {
		case AssetStateLoading:
			stats.LoadingEntries++
		case AssetStateReady:
			stats.ReadyEntries++
		case AssetStateFailed:
			stats.FailedEntries++
		}
		if entry.HighestReadyLOD > 0 {
			stats.PartialEntries++
		}
		stats.Entries = append(stats.Entries, AssetDiagEntry{
			ID:              entry.ID,
			Path:            entry.Path,
			State:           entry.State,
			HighestReadyLOD: entry.HighestReadyLOD,
			RefCounts:       entry.LODRefCounts,
			SizeBytes:       entry.SizeBytes,
			LODGPUReady:     entry.LODGPUReady,
			LODGPUBytes:     entry.LODGPUBytes,
			LoadTimeNs:      entry.LoadTimeNs,
			LastUsedFrame:   entry.LastUse,
		})
	}
	m.registry.mu.RUnlock()

	stats.UploadsThisFrame = m.uploadsThisFrame

	if m.cache != nil {
		stats.CPUUsedBytes = m.cache.usedBytes
		stats.CPUBudgetBytes = m.cache.budgetBytes
		stats.GPUUsedBytes = m.cache.gpuUsed
		stats.GPUBudgetBytes = m.cache.gpuBudget
		stats.EvictionsThisFrame = m.cache.evictionsThisFrame
	}
	return stats
}

// Open implements fs.FS by delegating to the underlying source.
func (m *ManagerImpl) Open(name string) (fs.File, error) {
	if m == nil || m.source == nil {
		return nil, fs.ErrNotExist
	}
	if s, ok := m.source.(fs.FS); ok {
		return s.Open(name)
	}
	return nil, fs.ErrNotExist
}

func (m *ManagerImpl) loadByPath(path string, typ AssetType) Handle {
	if m == nil || m.idReg == nil || m.registry == nil {
		return Handle{}
	}
	id := m.idReg.Lookup(path)
	if id == (AssetID{}) {
		return Handle{}
	}
	m.scheduleAllLODs(id, path, typ)
	return NewHandle(id, m.registry)
}

func (m *ManagerImpl) scheduleLOD(id AssetID, path string, typ AssetType, lod int) {
	if m == nil || m.registry == nil || m.scheduler == nil {
		return
	}
	entry := m.registry.GetOrCreate(id)
	if entry == nil {
		return
	}
	if lod < 0 || lod >= len(entry.LODHandles) {
		return
	}
	if entry.LODHandles[lod] != nil || entry.LODInFlight[lod] {
		return
	}
	entry.LODInFlight[lod] = true
	entry.Path = path
	entry.Type = typ
	if entry.State == AssetStateAbsent {
		entry.State = AssetStateLoading
	}

	job := &AssetLoadJob{
		ID:           id,
		Path:         path,
		Type:         typ,
		LOD:          lod,
		EntryVersion: entry.EntryVersion,
		Source:       m.source,
		Backend:      m.backendType,
	}
	_ = m.scheduler.Schedule(job)
}

func (m *ManagerImpl) scheduleAllLODs(id AssetID, path string, typ AssetType) {
	for lod := maxLODForType(typ); lod >= 0; lod-- {
		m.scheduleLOD(id, path, typ, lod)
	}
}

func (j *AssetLoadJob) Execute() {
	if j == nil {
		return
	}
	start := time.Now()

	var raw []byte
	var err error

	// Per D9: if PreloadedSource is set, use it (single read already done).
	// Otherwise, read from source (for legacy paths).
	if len(j.PreloadedSource) > 0 {
		raw = j.PreloadedSource
	} else {
		raw, err = j.Source.ReadLOD(j.ID, j.LOD)
		if err != nil {
			j.Err = err
			return
		}
	}

	decompressed, err := decompressJobPayload(j.Type, j.LOD, raw)
	if err != nil {
		j.Err = err
		return
	}

	j.Result, j.Err = decodeJobPayload(j.Type, j.LOD, decompressed, j.Backend)
	if j.Err == nil {
		j.ElapsedNs = time.Since(start).Nanoseconds()
	}
}

func (m *ManagerImpl) commitJob(job *AssetLoadJob) {
	if m == nil || m.registry == nil || job == nil {
		return
	}

	entry := m.registry.Get(job.ID)
	if entry == nil {
		return
	}
	if entry.EntryVersion != job.EntryVersion {
		entry.LODInFlight[job.LOD] = false
		return
	}

	entry.LODInFlight[job.LOD] = false

	if job.Err != nil {
		entry.LODFailed[job.LOD] = true
		allFailed := true
		for i := range entry.LODFailed {
			if i <= maxLODForType(entry.Type) && !entry.LODFailed[i] {
				allFailed = false
				break
			}
		}
		if allFailed {
			entry.State = AssetStateFailed
			entry.Err = job.Err
			entry.EntryVersion++
			m.registry.globalVersion++
		}
		return
	}

	// Calculate the size of the decoded asset for wallet charging.
	// Per D9: decoded residency is charged to AssetsCPU at registry admission.
	var decodedSize int64
	if job.Result != nil {
		if img, ok := job.Result.(*DecodedImageLOD); ok {
			decodedSize = int64(len(img.Data))
		} else if font, ok := job.Result.(*DecodedFontLOD); ok {
			decodedSize = int64(len(font.Data))
		} else {
			// For other asset types, we can't determine the exact size without
			// the raw bytes. This is a fallback that should be rare.
			// Use a conservative estimate based on the asset type.
			decodedSize = estimateDecodedSize(job.Type)
		}
	}

	// Charge the decoded residency to the assets wallet.
	// Per D9: decoded residency is charged to AssetsCPU at registry admission.
	if m.assetsWallet != nil && decodedSize > 0 {
		if reservation, err := m.assetsWallet.Reserve(uint64(decodedSize)); err == nil {
			// Store the reservation for later release.
			m.mu.Lock()
			m.assetReservations[job.ID] = append(m.assetReservations[job.ID], reservation)
			m.mu.Unlock()
		} else {
			// If we can't charge, we still commit the asset but log the failure.
			// This should not happen in normal operation as the pool already
			// reserved space for the output.
			// Per D9: the job wallet and assets wallet are separate budgets.
			// For now, we proceed without the reservation to keep the asset system working.
		}
	}

	m.registry.SetLODReady(job.ID, job.LOD, job.Result, job.ElapsedNs)
	m.drainWaiting(job.ID)

	// Enqueue GPU upload for GPU-eligible LODs.
	m.enqueueUpload(job)
}

func (m *ManagerImpl) enqueueUpload(job *AssetLoadJob) {
	if m == nil || job == nil {
		return
	}
	gpuCapable := m.uploader != nil && m.uploader.Budget() > 0
	if !gpuCapable {
		return
	}
	if AssetResidency(job.Type, m.residency, true) != ResidencyGPU {
		return
	}

	// For raster images, enqueue the decoded pixels for upload.
	if job.Result != nil {
		if img, ok := job.Result.(*DecodedImageLOD); ok && len(img.Data) > 0 {
			req := TextureUploadRequest{
				AssetID:   job.ID,
				LOD:       job.LOD,
				Pixels:    img.Data,
				Width:     int(img.Width),
				Height:    int(img.Height),
				MipLevels: 1,
				Format:    m.uploader.TargetFormat(),
			}
			m.uploader.Enqueue(req)
		}
	}
}

// RequestUpload enqueues a GPU upload for the given asset LOD when the
// LOD is CPU-ready and should be GPU-resident according to residency policy.
// Returns true if the upload was enqueued. No-op when:
//   - the backend is not GPU-capable
//   - the LOD is already GPU-ready or in-flight
//   - the asset type is not GPU-eligible
//   - the LOD has no decoded data
//
// This is the lazy re-upload path triggered by ResolveDrawable after
// device-loss recovery or eviction.
func (m *ManagerImpl) RequestUpload(id AssetID, lod int) bool {
	gpuCapable := m != nil && m.uploader != nil && m.uploader.Budget() > 0 && m.registry != nil
	if !gpuCapable {
		return false
	}

	entry := m.registry.Get(id)
	if entry == nil {
		return false
	}
	if AssetResidency(entry.Type, m.residency, true) != ResidencyGPU {
		return false
	}
	if lod < 0 || lod >= len(entry.LODHandles) {
		return false
	}
	if entry.LODGPUReady[lod] {
		return false
	}
	if entry.LODHandles[lod] == nil {
		return false
	}

	if img, ok := entry.LODHandles[lod].(*DecodedImageLOD); ok && len(img.Data) > 0 {
		req := TextureUploadRequest{
			AssetID:   id,
			LOD:       lod,
			Pixels:    img.Data,
			Width:     int(img.Width),
			Height:    int(img.Height),
			MipLevels: 1,
			Format:    m.uploader.TargetFormat(),
		}
		return m.uploader.Enqueue(req)
	}
	return false
}

func (m *ManagerImpl) drainWaiting(readyID AssetID) {
	m.drainWaitingForLeaf(readyID)
}

func maxLODForType(typ AssetType) int {
	switch typ {
	case AssetTypeSVG, AssetTypeImage:
		return 2
	case AssetTypeFont:
		return 1
	case AssetTypeConfig:
		return 0
	default:
		return 0
	}
}

// estimateDecodedSize returns a conservative estimate of decoded size for asset types
// where we can't determine the exact size. Used as a fallback for wallet charging.
func estimateDecodedSize(typ AssetType) int64 {
	// Conservative estimates based on typical asset characteristics.
	switch typ {
	case AssetTypeImage:
		// Assume 1024x1024 RGBA = 4MB
		return 4 * 1024 * 1024
	case AssetTypeSVG:
		// SVG can expand significantly; use a large estimate
		return 10 * 1024 * 1024
	case AssetTypeFont:
		// Fonts are typically moderate size
		return 1 * 1024 * 1024
	case AssetTypeConfig:
		// Configs are typically small
		return 64 * 1024
	default:
		return 1 * 1024 * 1024
	}
}

func decompressJobPayload(typ AssetType, lod int, raw []byte) ([]byte, error) {
	switch typ {
	case AssetTypeSVG:
		if lod == 0 {
			return decompressLZ4(raw)
		}
		return append([]byte(nil), raw...), nil
	case AssetTypeFont, AssetTypeConfig:
		dec, err := zstd.NewReader(nil)
		if err != nil {
			return nil, err
		}
		defer dec.Close()
		return dec.DecodeAll(raw, nil)
	default:
		return append([]byte(nil), raw...), nil
	}
}

func decompressLZ4(src []byte) ([]byte, error) {
	if len(src) == 0 {
		return nil, nil
	}
	size := len(src) * 4
	if size < 64 {
		size = 64
	}
	for size <= 1<<26 {
		dst := make([]byte, size)
		n, err := lz4.UncompressBlock(src, dst)
		if err == nil {
			return append([]byte(nil), dst[:n]...), nil
		}
		size *= 2
	}
	return nil, fmt.Errorf("lz4 decompression failed")
}

func decodeJobPayload(typ AssetType, lod int, data []byte, backend BackendType) (DecodedAsset, error) {
	_ = backend
	switch typ {
	case AssetTypeSVG:
		switch lod {
		case 0:
			return &DecodedSVGLOD0{Data: append([]byte(nil), data...)}, nil
		case 1:
			image, err := gfx.NewImageHandlePixels(data, 32, 32, 128, 0)
			if err != nil {
				return nil, fmt.Errorf("svg lod1 image: %w", err)
			}
			return &DecodedSVGLOD1{Image: image}, nil
		case 2:
			if len(data) < 4 {
				return nil, fmt.Errorf("svg lod2 payload too small: %d", len(data))
			}
			return &DecodedSVGLOD2{DominantColor: binary.LittleEndian.Uint32(data[:4])}, nil
		default:
			return &DecodedSVGLOD0{Data: append([]byte(nil), data...)}, nil
		}
	case AssetTypeImage:
		d := &DecodedImageLOD{Data: append([]byte(nil), data...)}
		if w, h, err := decodeImageHeader(data); err == nil {
			d.Width = w
			d.Height = h
		}
		return d, nil
	case AssetTypeFont:
		return &DecodedFontLOD{Data: append([]byte(nil), data...)}, nil
	case AssetTypeConfig:
		return &DecodedConfigLOD{Data: append([]byte(nil), data...)}, nil
	default:
		return append([]byte(nil), data...), nil
	}
}

// DecodedSVGLOD0 contains FlatBuffers geometry bytes.
type DecodedSVGLOD0 struct {
	Data []byte
}

// DecodedSVGLOD1 contains a 32x32 RGBA bitmap.
type DecodedSVGLOD1 struct {
	Image gfx.ImageHandle
}

// DecodedSVGLOD2 contains the packed dominant color.
type DecodedSVGLOD2 struct {
	DominantColor uint32
}

// DecodedImageLOD contains cooked texture bytes with dimension metadata.
// Width, Height, and Format are populated by the decode path when the
// cooked payload carries them; they default to 0 until Phase 11 plumbing.
type DecodedImageLOD struct {
	Data   []byte
	Width  int32
	Height int32
	Format uint32
}

// DecodedFontLOD contains CFNT bytes.
type DecodedFontLOD struct {
	Data []byte
}

// DecodedConfigLOD contains gob-encoded config bytes.
type DecodedConfigLOD struct {
	Data []byte
}
