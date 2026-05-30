package assets

import (
	"encoding/binary"
	"fmt"
	"io/fs"
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
	scheduler   JobScheduler
	results     chan *AssetLoadJob
	depTree     ConfigDependencyTree

	mu      sync.Mutex
	waiting waitingOn
}

// NewManager returns an asset manager that wraps the given source.
// When scheduler is nil, a default async goroutine scheduler is created.
func NewManager(registry *AssetRegistryStore, source AssetSource, backend BackendType, scheduler JobScheduler, idReg PathIDRegistry) *ManagerImpl {
	m := &ManagerImpl{
		registry:    registry,
		source:      source,
		idReg:       idReg,
		backendType: backend,
		results:     make(chan *AssetLoadJob, 32),
		waiting:     make(waitingOn),
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

// Stats returns a snapshot of the registry and cache state.
func (m *ManagerImpl) Stats() ManagerStats {
	if m == nil || m.registry == nil {
		return ManagerStats{}
	}
	stats := ManagerStats{}
	m.registry.mu.RLock()
	defer m.registry.mu.RUnlock()
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

// DecodedImageLOD contains cooked texture bytes.
type DecodedImageLOD struct {
	Data []byte
}

// DecodedFontLOD contains CFNT bytes.
type DecodedFontLOD struct {
	Data []byte
}

// DecodedConfigLOD contains gob-encoded config bytes.
type DecodedConfigLOD struct {
	Data []byte
}
