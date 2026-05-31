package assets

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"
	"time"

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

// asyncJobScheduler executes jobs on a background goroutine and forwards them to results.
type asyncJobScheduler struct {
	results chan<- *AssetLoadJob
}

// NewAsyncJobScheduler returns a scheduler that runs each job in its own goroutine.
func NewAsyncJobScheduler(results chan<- *AssetLoadJob) JobScheduler {
	return &asyncJobScheduler{results: results}
}

func (s *asyncJobScheduler) Schedule(job *AssetLoadJob) error {
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

	mu      sync.Mutex
	waiting waitingOn

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
// This is a thin back-compat wrapper; new code should use NewManagerWithResidency.
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
		registry:    registry,
		source:      source,
		idReg:       idReg,
		backendType: bt,
		uploader:    uploader,
		residency:   mode,
		results:     make(chan *AssetLoadJob, 32),
		waiting:     make(waitingOn),
	}

	cpuBytes := cpuBudgetMB * 1024 * 1024
	gpuBytes := gpuBudgetMB * 1024 * 1024
	if cpuBytes > 0 || gpuBytes > 0 {
		m.cache = newAssetCache(registry, nil, cpuBytes, gpuBytes)
	}

	if scheduler == nil {
		m.scheduler = NewAsyncJobScheduler(m.results)
	} else {
		m.scheduler = scheduler
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
func (m *ManagerImpl) LoadConfig(path string, _ any) Handle { return m.loadByPath(path, AssetTypeConfig) }

// Prefetch schedules load work for the given paths.
func (m *ManagerImpl) Prefetch(paths ...string) {
	for _, path := range paths {
		m.loadByPath(path, assetTypeForPath(path))
	}
}

// Invalidate marks an asset stale and clears ready LODs from the registry.
func (m *ManagerImpl) Invalidate(path string) {
	if m == nil || m.idReg == nil || m.registry == nil {
		return
	}
	if id := m.idReg.Lookup(path); id != (AssetID{}) {
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
// A nil receiver is safe-to-call.
func (m *ManagerImpl) TrimMemory(level int) int {
	if m == nil {
		return 0
	}
	if m.cache != nil {
		return m.cache.EvictToWatermark(TrimLevelFraction(level))
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

	raw, err := j.Source.ReadLOD(j.ID, j.LOD)
	if err != nil {
		j.Err = err
		return
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
				Format:    img.Format,
			}
			m.uploader.Enqueue(req)
		}
	}
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
			return &DecodedSVGLOD1{RGBA: append([]byte(nil), data...)}, nil
		case 2:
			if len(data) < 4 {
				return nil, fmt.Errorf("svg lod2 payload too small: %d", len(data))
			}
			return &DecodedSVGLOD2{DominantColor: binary.LittleEndian.Uint32(data[:4])}, nil
		default:
			return &DecodedSVGLOD0{Data: append([]byte(nil), data...)}, nil
		}
	case AssetTypeImage:
		return &DecodedImageLOD{Data: append([]byte(nil), data...)}, nil
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
	RGBA []byte
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
