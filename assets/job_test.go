package assets

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

type staticSource struct {
	data map[[2]uint64][]byte
}

func (s staticSource) ReadLOD(id AssetID, lod int) ([]byte, error) {
	return append([]byte(nil), s.data[[2]uint64{assetIDKey(id), uint64(lod)}]...), nil
}

type blockingSource struct {
	payload []byte
	started chan struct{}
	release chan struct{}
}

type captureScheduler struct {
	jobs chan *AssetLoadJob
}

func (s captureScheduler) Schedule(job *AssetLoadJob) error {
	s.jobs <- job
	return nil
}

func (s *blockingSource) ReadLOD(id AssetID, lod int) ([]byte, error) {
	close(s.started)
	<-s.release
	return append([]byte(nil), s.payload...), nil
}

func assetIDKey(id AssetID) uint64 {
	return binary.LittleEndian.Uint64(id[:8]) ^ binary.LittleEndian.Uint64(id[8:])
}

func TestAssetLoadJobExecuteDecodesPayloads(t *testing.T) {
	t.Run("svg lod0 lz4", func(t *testing.T) {
		raw := []byte("geometry-bytes")
		dst := make([]byte, lz4.CompressBlockBound(len(raw)))
		n, err := lz4.CompressBlock(raw, dst, nil)
		if err != nil {
			t.Fatalf("compress: %v", err)
		}
		job := &AssetLoadJob{
			ID:      mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdef"),
			Type:    AssetTypeSVG,
			LOD:     0,
			Source:  staticSource{data: map[[2]uint64][]byte{{assetIDKey(mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdef")), 0}: append([]byte(nil), dst[:n]...)}},
			Backend: BackendSoftware,
		}
		job.Execute()
		if job.Err != nil {
			t.Fatalf("execute: %v", job.Err)
		}
		got, ok := job.Result.(*DecodedSVGLOD0)
		if !ok {
			t.Fatalf("unexpected result type: %T", job.Result)
		}
		if !bytes.Equal(got.Data, raw) {
			t.Fatalf("unexpected payload: %q", got.Data)
		}
	})

	t.Run("font lod0 zstd", func(t *testing.T) {
		raw := []byte("font-bytes")
		enc := zstd.EncodeTo(nil, raw)
		job := &AssetLoadJob{
			ID:      mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdee"),
			Type:    AssetTypeFont,
			LOD:     0,
			Source:  staticSource{data: map[[2]uint64][]byte{{assetIDKey(mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdee")), 0}: append([]byte(nil), enc...)}},
			Backend: BackendSoftware,
		}
		job.Execute()
		if job.Err != nil {
			t.Fatalf("execute: %v", job.Err)
		}
		got, ok := job.Result.(*DecodedFontLOD)
		if !ok {
			t.Fatalf("unexpected result type: %T", job.Result)
		}
		if !bytes.Equal(got.Data, raw) {
			t.Fatalf("unexpected payload: %q", got.Data)
		}
	})

	t.Run("svg lod2 color", func(t *testing.T) {
		var payload [4]byte
		binary.LittleEndian.PutUint32(payload[:], 0x11223344)
		job := &AssetLoadJob{
			ID:      mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdf0"),
			Type:    AssetTypeSVG,
			LOD:     2,
			Source:  staticSource{data: map[[2]uint64][]byte{{assetIDKey(mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdf0")), 2}: append([]byte(nil), payload[:]...)}},
			Backend: BackendSoftware,
		}
		job.Execute()
		if job.Err != nil {
			t.Fatalf("execute: %v", job.Err)
		}
		got, ok := job.Result.(*DecodedSVGLOD2)
		if !ok {
			t.Fatalf("unexpected result type: %T", job.Result)
		}
		if got.DominantColor != 0x11223344 {
			t.Fatalf("unexpected color: %#x", got.DominantColor)
		}
	})
}

func TestManagerScheduleAndDrainAsync(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdef")
	reg := NewAssetRegistryStore()
	configPayload := zstd.EncodeTo(nil, []byte("geometry-bytes"))
	src := &blockingSource{
		payload: configPayload,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	mgr := NewManager(reg, src, BackendSoftware, nil, nil)

	start := time.Now()
	mgr.scheduleLOD(id, "assets/theme.toml", AssetTypeConfig, 0)
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("schedule blocked too long: %v", elapsed)
	}

	select {
	case <-src.started:
	case <-time.After(time.Second):
		t.Fatal("job never started")
	}

	close(src.release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := mgr.DrainCompleted(); got > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	entry := reg.Get(id)
	if entry == nil {
		t.Fatal("expected registry entry")
	}
	if entry.State != AssetStateReady {
		t.Fatalf("unexpected state: %v", entry.State)
	}
	got, ok := entry.LODHandles[0].(*DecodedConfigLOD)
	if !ok {
		t.Fatalf("unexpected result type: %T", entry.LODHandles[0])
	}
	if !bytes.Equal(got.Data, []byte("geometry-bytes")) {
		t.Fatalf("unexpected committed bytes: %q", got.Data)
	}
}

func TestCommitJobRejectsStaleVersion(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdee")
	reg := NewAssetRegistryStore()
	mgr := NewManager(reg, staticSource{}, BackendSoftware, nil, nil)

	reg.SetLODReady(id, 0, &DecodedSVGLOD0{Data: []byte("current")}, 10)
	entry := reg.Get(id)
	if entry == nil {
		t.Fatal("expected entry")
	}
	version := entry.EntryVersion

	job := &AssetLoadJob{
		ID:           id,
		Type:         AssetTypeSVG,
		LOD:          0,
		EntryVersion: version - 1,
		Result:       &DecodedSVGLOD0{Data: []byte("stale")},
		ElapsedNs:    20,
	}
	mgr.commitJob(job)

	entry = reg.Get(id)
	got, ok := entry.LODHandles[0].(*DecodedSVGLOD0)
	if !ok {
		t.Fatalf("unexpected type: %T", entry.LODHandles[0])
	}
	if string(got.Data) != "current" {
		t.Fatalf("stale commit overwrote payload: %q", got.Data)
	}
	if entry.EntryVersion != version {
		t.Fatalf("unexpected version after stale commit: %d", entry.EntryVersion)
	}
}

func TestWaitingOnSchedulesConfigAfterLastDependency(t *testing.T) {
	fontID := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc001")
	imageID := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc002")
	configID := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc003")

	reg := NewAssetRegistryStore()
	sched := captureScheduler{jobs: make(chan *AssetLoadJob, 4)}
	mgr := NewManager(reg, staticSource{}, BackendSoftware, sched, nil)
	mgr.SetDependencyTree(fakeConfigTree{nodes: map[AssetID]*ConfigNode{
		configID: {
			ID:   configID,
			Path: "assets/config/theme.toml",
			Deps: []AssetID{fontID, imageID},
		},
	}})

	mgr.scheduleConfig(configID, "assets/config/theme.toml")
	if got := len(sched.jobs); got != 0 {
		t.Fatalf("expected config to wait, got %d scheduled jobs", got)
	}

	reg.GetOrCreate(fontID)
	reg.GetOrCreate(imageID)

	mgr.commitJob(&AssetLoadJob{
		ID:           fontID,
		Type:         AssetTypeFont,
		LOD:          0,
		EntryVersion: 0,
		Result:       &DecodedFontLOD{Data: []byte("font")},
		ElapsedNs:    1,
	})
	if got := len(sched.jobs); got != 0 {
		t.Fatalf("expected config to still wait on image, got %d scheduled jobs", got)
	}

	mgr.commitJob(&AssetLoadJob{
		ID:           imageID,
		Type:         AssetTypeImage,
		LOD:          0,
		EntryVersion: 0,
		Result:       &DecodedImageLOD{Data: []byte("image")},
		ElapsedNs:    1,
	})

	select {
	case job := <-sched.jobs:
		if job.ID != configID {
			t.Fatalf("unexpected scheduled config: %s", job.ID)
		}
		if job.Type != AssetTypeConfig || job.LOD != 0 {
			t.Fatalf("unexpected config job: %+v", job)
		}
	case <-time.After(time.Second):
		t.Fatal("expected config to schedule after last dependency")
	}
}

type fakeConfigTree struct {
	nodes map[AssetID]*ConfigNode
}

func (f fakeConfigTree) ConfigNode(id AssetID) *ConfigNode {
	if f.nodes == nil {
		return nil
	}
	return f.nodes[id]
}

// ── Fake uploader for residency tests ──────────────────────────────────────

type fakeUploader struct {
	budget  int
	results chan TextureUploadResult
}

func (u *fakeUploader) Enqueue(req TextureUploadRequest) bool {
	return true
}

func (u *fakeUploader) Results() <-chan TextureUploadResult {
	if u.results == nil {
		u.results = make(chan TextureUploadResult, 8)
	}
	return u.results
}

func (u *fakeUploader) Budget() int                { return u.budget }
func (u *fakeUploader) TargetFormat() uint32       { return 0 }

// ── Residency constructor tests ─────────────────────────────────────────────

func TestNewManagerWithResidency_noBudgets_noCache(t *testing.T) {
	reg := NewAssetRegistryStore()
	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, nil, ResidencyCPUOnly, 0, 0)
	if m.cache != nil {
		t.Fatal("expected nil cache when both budgets are 0")
	}
	if m.uploader != nil {
		t.Fatal("expected nil uploader")
	}
	if m.backendType != BackendSoftware {
		t.Fatalf("expected BackendSoftware, got %v", m.backendType)
	}
}

func TestNewManagerWithResidency_cpuBudget_createsCache(t *testing.T) {
	reg := NewAssetRegistryStore()
	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, nil, ResidencyCPUOnly, 64, 0)
	if m.cache == nil {
		t.Fatal("expected non-nil cache with cpuBudgetMB > 0")
	}
	if m.cache.budgetBytes != 64*1024*1024 {
		t.Fatalf("expected budgetBytes %d, got %d", 64*1024*1024, m.cache.budgetBytes)
	}
	if m.cache.gpuBudget != 0 {
		t.Fatalf("expected gpuBudget 0, got %d", m.cache.gpuBudget)
	}
}

func TestNewManagerWithResidency_gpuBudget_createsCache(t *testing.T) {
	reg := NewAssetRegistryStore()
	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, nil, ResidencyCPUOnly, 0, 128)
	if m.cache == nil {
		t.Fatal("expected non-nil cache with gpuBudgetMB > 0")
	}
	if m.cache.gpuBudget != 128*1024*1024 {
		t.Fatalf("expected gpuBudget %d, got %d", 128*1024*1024, m.cache.gpuBudget)
	}
}

func TestNewManagerWithResidency_nilUploader_isCPUOnly(t *testing.T) {
	reg := NewAssetRegistryStore()
	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, nil, ResidencyGPUResident, 64, 128)
	if m.backendType != BackendSoftware {
		t.Fatal("expected BackendSoftware with nil uploader even in GPUResident mode")
	}
}

func TestNewManagerWithResidency_zeroBudgetUploader_isCPUOnly(t *testing.T) {
	reg := NewAssetRegistryStore()
	uploader := &fakeUploader{budget: 0}
	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)
	if m.backendType != BackendSoftware {
		t.Fatal("expected BackendSoftware when uploader.Budget() == 0")
	}
}

func TestNewManagerWithResidency_gpuMode_setsBackendVulkan(t *testing.T) {
	reg := NewAssetRegistryStore()
	uploader := &fakeUploader{budget: 4096}
	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)
	if m.backendType != BackendVulkan {
		t.Fatalf("expected BackendVulkan, got %v", m.backendType)
	}
	if m.uploader != uploader {
		t.Fatal("uploader not stored")
	}
	if m.residency != ResidencyGPUResident {
		t.Fatalf("expected residency GPUResident, got %v", m.residency)
	}
}

func TestNewManagerWithResidency_cpuMode_staysSoftware(t *testing.T) {
	reg := NewAssetRegistryStore()
	uploader := &fakeUploader{budget: 4096}
	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyCPUOnly, 64, 128)
	if m.backendType != BackendSoftware {
		t.Fatalf("expected BackendSoftware, got %v", m.backendType)
	}
}

func TestNewManagerWithResidency_autoMode_delegatesToGpuWhenCapable(t *testing.T) {
	reg := NewAssetRegistryStore()
	uploader := &fakeUploader{budget: 4096}
	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyAuto, 64, 128)
	if m.backendType != BackendVulkan {
		t.Fatalf("expected BackendVulkan in auto mode with capable uploader, got %v", m.backendType)
	}
}

func TestNewManagerWithResidency_autoMode_fallsBackToSoftware(t *testing.T) {
	reg := NewAssetRegistryStore()
	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, nil, ResidencyAuto, 64, 0)
	if m.backendType != BackendSoftware {
		t.Fatalf("expected BackendSoftware in auto mode with nil uploader, got %v", m.backendType)
	}
}

func TestNewManager_backCompat_respectsBackend(t *testing.T) {
	reg := NewAssetRegistryStore()
	m := NewManager(reg, staticSource{}, BackendVulkan, nil, nil)
	if m.backendType != BackendVulkan {
		t.Fatalf("expected BackendVulkan from back-compat wrapper, got %v", m.backendType)
	}
	if m.cache != nil {
		t.Fatal("expected nil cache from NewManager")
	}
}

func TestNewManagerWithCache_backCompat_createsCache(t *testing.T) {
	reg := NewAssetRegistryStore()
	backend := &recordingBackend{}
	m := NewManagerWithCache(reg, staticSource{}, BackendSoftware, nil, nil, backend, 1000, 500)
	if m.cache == nil {
		t.Fatal("expected non-nil cache")
	}
	if m.cache.budgetBytes != 1000 {
		t.Fatalf("expected budgetBytes 1000, got %d", m.cache.budgetBytes)
	}
	if m.cache.gpuBudget != 500 {
		t.Fatalf("expected gpuBudget 500, got %d", m.cache.gpuBudget)
	}
}

func TestNewManagerWithCache_backCompat_preservesReleaser(t *testing.T) {
	reg := NewAssetRegistryStore()
	backend := &recordingBackend{}
	m := NewManagerWithCache(reg, staticSource{}, BackendSoftware, nil, nil, backend, 100, 0)
	if m.cache == nil {
		t.Fatal("expected non-nil cache")
	}
	if m.cache.backend == nil {
		t.Fatal("expected TextureReleaser to be set on cache")
	}
	// Verify it's the same backend by calling FreeTexture through the chain.
	m.cache.backend.FreeTexture(TextureID(42))
	if len(backend.freed) != 1 || backend.freed[0] != TextureID(42) {
		t.Fatal("TextureReleaser not forwarded correctly")
	}
}

// ── AssetResidency policy tests ─────────────────────────────────────────────

// ── Upload result drain tests ───────────────────────────────────────────────

// resultUploader is a fake uploader that lets us inject results.
type resultUploader struct {
	results chan TextureUploadResult
	budget  int
}

func newResultUploader() *resultUploader {
	return &resultUploader{
		results: make(chan TextureUploadResult, 16),
		budget:  4096,
	}
}

func (u *resultUploader) Enqueue(req TextureUploadRequest) bool {
	return true
}

func (u *resultUploader) Results() <-chan TextureUploadResult {
	return u.results
}

func (u *resultUploader) Budget() int          { return u.budget }
func (u *resultUploader) TargetFormat() uint32 { return 0 }

func (u *resultUploader) send(id AssetID, lod int, texID TextureID, gpuBytes int64) {
	u.results <- TextureUploadResult{
		AssetID:   id,
		LOD:       lod,
		TextureID: texID,
		GPUBytes:  gpuBytes,
		OK:        true,
	}
}

func TestDrainUploadResults_increasesGPUBudget(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc001")
	reg := NewAssetRegistryStore()
	uploader := newResultUploader()

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)

	// Prepare a CPU-ready entry so SizeBytes is set.
	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: []byte("pixels")}, 100)
	entry := reg.Get(id)
	if entry == nil {
		t.Fatal("expected entry")
	}

	// Send a GPU upload result.
	uploader.send(id, 0, TextureID(42), 2048)

	count := m.DrainUploadResults()
	if count != 1 {
		t.Fatalf("expected 1 drain result, got %d", count)
	}

	// GPU budget accounting should reflect the upload.
	if m.cache == nil {
		t.Fatal("expected non-nil cache")
	}
	if m.cache.gpuUsed != 2048 {
		t.Fatalf("expected gpuUsed 2048, got %d", m.cache.gpuUsed)
	}

	// Registry should mark the LOD as GPU-ready.
	entry = reg.Get(id)
	if entry == nil {
		t.Fatal("expected entry")
	}
	if !entry.LODGPUReady[0] {
		t.Fatal("expected LOD 0 to be GPU-ready")
	}
	if entry.LODTextureIDs[0] != TextureID(42) {
		t.Fatalf("expected TextureID 42, got %d", entry.LODTextureIDs[0])
	}
	if entry.LODGPUBytes[0] != 2048 {
		t.Fatalf("expected GPUBytes 2048, got %d", entry.LODGPUBytes[0])
	}
}

func TestDrainUploadResults_statsReflectCount(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc002")
	reg := NewAssetRegistryStore()
	uploader := newResultUploader()

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)

	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: []byte("pixels")}, 100)
	reg.SetLODReady(id, 1, &DecodedImageLOD{Data: []byte("lod1"), Width: 32, Height: 16}, 50)

	uploader.send(id, 0, TextureID(10), 1024)
	uploader.send(id, 1, TextureID(11), 512)

	count := m.DrainUploadResults()
	if count != 2 {
		t.Fatalf("expected 2 drain results, got %d", count)
	}

	stats := m.Stats()
	if stats.UploadsThisFrame != 2 {
		t.Fatalf("expected UploadsThisFrame 2, got %d", stats.UploadsThisFrame)
	}
}

func TestDrainUploadResults_multipleFramesResetCount(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc003")
	reg := NewAssetRegistryStore()
	uploader := newResultUploader()

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)

	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: []byte("pixels")}, 100)
	uploader.send(id, 0, TextureID(7), 256)

	m.DrainUploadResults()
	stats1 := m.Stats()
	if stats1.UploadsThisFrame != 1 {
		t.Fatalf("expected UploadsThisFrame 1, got %d", stats1.UploadsThisFrame)
	}

	// Second drain with no new results should reset to 0.
	m.DrainUploadResults()
	stats2 := m.Stats()
	if stats2.UploadsThisFrame != 0 {
		t.Fatalf("expected UploadsThisFrame 0 after drain with no results, got %d", stats2.UploadsThisFrame)
	}
}

func TestDrainUploadResults_nilUploaderNoOp(t *testing.T) {
	reg := NewAssetRegistryStore()
	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, nil, ResidencyCPUOnly, 64, 0)

	count := m.DrainUploadResults()
	if count != 0 {
		t.Fatalf("expected 0 drain results with nil uploader, got %d", count)
	}
}

func TestDrainUploadResults_GPUEntrySurvivesCPUFallback(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc004")
	reg := NewAssetRegistryStore()
	uploader := newResultUploader()

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)

	// CPU LOD is set first.
	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: []byte("cpu-pixels"), Width: 64, Height: 32}, 100)

	// GPU upload completes.
	uploader.send(id, 0, TextureID(99), 4096)
	m.DrainUploadResults()

	// CPU LOD handle should still be present.
	entry := reg.Get(id)
	if entry == nil {
		t.Fatal("expected entry")
	}
	if entry.LODHandles[0] == nil {
		t.Fatal("expected CPU LOD handle to survive GPU upload")
	}
	if !entry.LODGPUReady[0] {
		t.Fatal("expected LOD to be GPU-ready")
	}
}

// ── Upload enqueue tests ────────────────────────────────────────────────────

type recordingUploader struct {
	enqueued []TextureUploadRequest
	budget   int
}

func (u *recordingUploader) Enqueue(req TextureUploadRequest) bool {
	u.enqueued = append(u.enqueued, req)
	return true
}

func (u *recordingUploader) Results() <-chan TextureUploadResult {
	return nil
}

func (u *recordingUploader) Budget() int {
	if u == nil {
		return 0
	}
	return u.budget
}

func (u *recordingUploader) TargetFormat() uint32 { return 0 }

func TestCommitJob_enqueuesRasterInGPUMode(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc001")
	reg := NewAssetRegistryStore()
	uploader := &recordingUploader{budget: 4096}

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)

	// Create and commit a raster image LOD.
	reg.GetOrCreate(id)
	job := &AssetLoadJob{
		ID:           id,
		Type:         AssetTypeImage,
		LOD:          0,
		EntryVersion: 0,
		Result:       &DecodedImageLOD{Data: []byte("pixel-data"), Width: 64, Height: 32},
		ElapsedNs:    100,
	}
	m.commitJob(job)

	if len(uploader.enqueued) != 1 {
		t.Fatalf("expected 1 enqueue, got %d", len(uploader.enqueued))
	}
	req := uploader.enqueued[0]
	if req.AssetID != id {
		t.Fatalf("expected AssetID %v, got %v", id, req.AssetID)
	}
	if req.LOD != 0 {
		t.Fatalf("expected LOD 0, got %d", req.LOD)
	}
	if string(req.Pixels) != "pixel-data" {
		t.Fatalf("expected pixels 'pixel-data', got %q", string(req.Pixels))
	}
	if req.Width != 64 {
		t.Fatalf("expected Width 64, got %d", req.Width)
	}
	if req.Height != 32 {
		t.Fatalf("expected Height 32, got %d", req.Height)
	}
}

func TestCommitJob_doesNotEnqueueInCPUMode(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc002")
	reg := NewAssetRegistryStore()
	uploader := &recordingUploader{budget: 4096}

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyCPUOnly, 64, 128)

	reg.GetOrCreate(id)
	job := &AssetLoadJob{
		ID:           id,
		Type:         AssetTypeImage,
		LOD:          0,
		EntryVersion: 0,
		Result:       &DecodedImageLOD{Data: []byte("pixel-data"), Width: 64, Height: 32},
		ElapsedNs:    100,
	}
	m.commitJob(job)

	if len(uploader.enqueued) != 0 {
		t.Fatalf("expected 0 enqueues in CPU mode, got %d", len(uploader.enqueued))
	}
}

func TestCommitJob_doesNotEnqueueSVG(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc003")
	reg := NewAssetRegistryStore()
	uploader := &recordingUploader{budget: 4096}

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)

	reg.GetOrCreate(id)
	job := &AssetLoadJob{
		ID:           id,
		Type:         AssetTypeSVG,
		LOD:          0,
		EntryVersion: 0,
		Result:       &DecodedSVGLOD0{Data: []byte("vector-data")},
		ElapsedNs:    100,
	}
	m.commitJob(job)

	if len(uploader.enqueued) != 0 {
		t.Fatalf("expected 0 enqueues for SVG, got %d", len(uploader.enqueued))
	}
}

func TestCommitJob_doesNotEnqueueWhenUploaderNil(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc004")
	reg := NewAssetRegistryStore()

	// Uploader is nil, so even GPUResident mode shouldn't enqueue.
	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, nil, ResidencyGPUResident, 64, 128)

	reg.GetOrCreate(id)
	job := &AssetLoadJob{
		ID:           id,
		Type:         AssetTypeImage,
		LOD:          0,
		EntryVersion: 0,
		Result:       &DecodedImageLOD{Data: []byte("pixel-data"), Width: 64, Height: 32},
		ElapsedNs:    100,
	}
	m.commitJob(job)

	if m.uploader != nil {
		t.Fatal("expected nil uploader")
	}
}

func TestCommitJob_doesNotEnqueueWhenUploaderBudgetZero(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc005")
	reg := NewAssetRegistryStore()
	uploader := &recordingUploader{budget: 0}

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)

	reg.GetOrCreate(id)
	job := &AssetLoadJob{
		ID:           id,
		Type:         AssetTypeImage,
		LOD:          0,
		EntryVersion: 0,
		Result:       &DecodedImageLOD{Data: []byte("pixel-data"), Width: 64, Height: 32},
		ElapsedNs:    100,
	}
	m.commitJob(job)

	if len(uploader.enqueued) != 0 {
		t.Fatalf("expected 0 enqueues when budget is 0, got %d", len(uploader.enqueued))
	}
}

func TestCommitJob_enqueuesWithCorrectAssetIDBytes(t *testing.T) {
	// Use a non-zero asset ID to verify the ID is passed through correctly.
	id := mustAssetID(t, "fedcba98-7654-3210-fedc-ba9876543210")
	reg := NewAssetRegistryStore()
	uploader := &recordingUploader{budget: 4096}

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)

	reg.GetOrCreate(id)
	job := &AssetLoadJob{
		ID:           id,
		Type:         AssetTypeImage,
		LOD:          1,
		EntryVersion: 0,
		Result:       &DecodedImageLOD{Data: []byte("more-pixels"), Width: 128, Height: 64},
		ElapsedNs:    200,
	}
	m.commitJob(job)

	if len(uploader.enqueued) != 1 {
		t.Fatalf("expected 1 enqueue, got %d", len(uploader.enqueued))
	}
	req := uploader.enqueued[0]
	if req.AssetID != id {
		t.Fatalf("AssetID mismatch: got %v, want %v", req.AssetID, id)
	}
	if req.LOD != 1 {
		t.Fatalf("LOD mismatch: got %d, want 1", req.LOD)
	}
}

// ── RequestUpload tests ──────────────────────────────────────────────────────

func TestRequestUpload_enqueuesImageLOD(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc010")
	reg := NewAssetRegistryStore()
	uploader := &recordingUploader{budget: 4096}

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)

	entry := reg.GetOrCreate(id)
	entry.Type = AssetTypeImage
	rgba := make([]byte, 32*32*4)
	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: rgba, Width: 32, Height: 32}, 100)

	ok := m.RequestUpload(id, 0)
	if !ok {
		t.Fatal("expected RequestUpload to return true")
	}
	if len(uploader.enqueued) != 1 {
		t.Fatalf("expected 1 enqueue, got %d", len(uploader.enqueued))
	}
	if uploader.enqueued[0].AssetID != id {
		t.Fatalf("unexpected AssetID: got %v, want %v", uploader.enqueued[0].AssetID, id)
	}
	if uploader.enqueued[0].LOD != 0 {
		t.Fatalf("expected LOD 0, got %d", uploader.enqueued[0].LOD)
	}
}

func TestRequestUpload_noOpWhenGPUReady(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc011")
	reg := NewAssetRegistryStore()
	uploader := &recordingUploader{budget: 4096}

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)

	entry := reg.GetOrCreate(id)
	entry.Type = AssetTypeImage
	rgba := make([]byte, 16*16*4)
	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: rgba, Width: 16, Height: 16}, 100)
	reg.SetLODGPUReady(id, 0, TextureID(1), 512)

	ok := m.RequestUpload(id, 0)
	if ok {
		t.Fatal("expected RequestUpload to return false for already GPU-ready LOD")
	}
	if len(uploader.enqueued) != 0 {
		t.Fatalf("expected 0 enqueues, got %d", len(uploader.enqueued))
	}
}

func TestRequestUpload_noOpForSVG(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc012")
	reg := NewAssetRegistryStore()
	uploader := &recordingUploader{budget: 4096}

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)

	reg.SetLODReady(id, 0, &DecodedSVGLOD0{Data: []byte("vector-data")}, 100)

	ok := m.RequestUpload(id, 0)
	if ok {
		t.Fatal("expected RequestUpload to return false for SVG asset")
	}
	if len(uploader.enqueued) != 0 {
		t.Fatalf("expected 0 enqueues for SVG, got %d", len(uploader.enqueued))
	}
}

func TestRequestUpload_noOpWhenNilUploader(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc013")
	reg := NewAssetRegistryStore()

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, nil, ResidencyGPUResident, 64, 128)

	rgba := make([]byte, 8*8*4)
	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: rgba, Width: 8, Height: 8}, 100)

	ok := m.RequestUpload(id, 0)
	if ok {
		t.Fatal("expected RequestUpload to return false with nil uploader")
	}
}

func TestRequestUpload_noOpWhenCPUMode(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc014")
	reg := NewAssetRegistryStore()
	uploader := &recordingUploader{budget: 4096}

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyCPUOnly, 64, 128)

	rgba := make([]byte, 32*32*4)
	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: rgba, Width: 32, Height: 32}, 100)

	ok := m.RequestUpload(id, 0)
	if ok {
		t.Fatal("expected RequestUpload to return false in CPU-only mode")
	}
	if len(uploader.enqueued) != 0 {
		t.Fatalf("expected 0 enqueues in CPU mode, got %d", len(uploader.enqueued))
	}
}

func TestRequestUpload_noOpForMissingLOD(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc015")
	reg := NewAssetRegistryStore()
	uploader := &recordingUploader{budget: 4096}

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)

	// Entry exists but no LOD data.
	reg.GetOrCreate(id)

	ok := m.RequestUpload(id, 0)
	if ok {
		t.Fatal("expected RequestUpload to return false when LOD has no data")
	}
	if len(uploader.enqueued) != 0 {
		t.Fatalf("expected 0 enqueues for missing LOD, got %d", len(uploader.enqueued))
	}
}

func TestRequestUpload_noOpForMissingEntry(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc016")
	reg := NewAssetRegistryStore()
	uploader := &recordingUploader{budget: 4096}

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)

	ok := m.RequestUpload(id, 0)
	if ok {
		t.Fatal("expected RequestUpload to return false for missing entry")
	}
}

func TestRequestUpload_deviceLossRecoveryFullCycle(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc017")
	reg := NewAssetRegistryStore()
	uploader := &recordingUploader{budget: 4096}
	backend := &recordingBackend{}

	m := NewManagerWithResidency(reg, staticSource{}, nil, nil, uploader, ResidencyGPUResident, 64, 128)
	m.SetTextureReleaser(backend)
	m.cache.deviceGeneration = 1

	entry := reg.GetOrCreate(id)
	entry.Type = AssetTypeImage
	rgba := make([]byte, 64*64*4)
	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: rgba, Width: 64, Height: 64}, 100)

	// Track in cache and mark GPU-ready (simulating initial upload).
	m.cache.trackLOD(id, 0, 16384, 4096, TextureID(42), 0)
	reg.SetLODGPUReady(id, 0, TextureID(42), 4096)

	// ── Device generation bump ───────────────────────────────────────────
	changed := m.CheckDeviceGeneration(2)
	if !changed {
		t.Fatal("expected device generation change detection")
	}

	// GPU texture freed exactly once.
	if len(backend.freed) != 1 || backend.freed[0] != TextureID(42) {
		t.Fatalf("expected texture 42 freed once, got %v", backend.freed)
	}

	// LOD is CPU-ready but not GPU-ready after device loss.
	entry = reg.Get(id)
	if entry == nil {
		t.Fatal("expected entry to survive device loss")
	}
	if entry.LODHandles[0] == nil {
		t.Fatal("expected CPU LOD data to survive device loss")
	}
	if entry.LODGPUReady[0] {
		t.Fatal("expected LOD to not be GPU-ready after device loss")
	}

	// ── Lazy re-upload ───────────────────────────────────────────────────
	ok := m.RequestUpload(id, 0)
	if !ok {
		t.Fatal("expected RequestUpload to re-enqueue after device loss")
	}
	if len(uploader.enqueued) != 1 {
		t.Fatalf("expected 1 re-enqueue, got %d", len(uploader.enqueued))
	}
	if uploader.enqueued[0].AssetID != id || uploader.enqueued[0].LOD != 0 {
		t.Fatalf("unexpected enqueue: %+v", uploader.enqueued[0])
	}
}

func TestAssetResidency_policyTable(t *testing.T) {
	tests := []struct {
		typ        AssetType
		mode       ResidencyMode
		gpuCapable bool
		want       Residency
	}{
		// Raster images → GPU when mode allows and backend is capable
		{AssetTypeImage, ResidencyGPUResident, true, ResidencyGPU},
		{AssetTypeImage, ResidencyAuto, true, ResidencyGPU},
		{AssetTypeImage, ResidencyCPUOnly, true, ResidencyCPU},
		{AssetTypeImage, ResidencyGPUResident, false, ResidencyCPU},

		// SVG → always CPU
		{AssetTypeSVG, ResidencyGPUResident, true, ResidencyCPU},
		{AssetTypeSVG, ResidencyAuto, true, ResidencyCPU},
		{AssetTypeSVG, ResidencyCPUOnly, true, ResidencyCPU},

		// Font → CPU (Phase 13 promotes to GPU)
		{AssetTypeFont, ResidencyGPUResident, true, ResidencyCPU},
		{AssetTypeFont, ResidencyAuto, true, ResidencyCPU},

		// Config → always CPU
		{AssetTypeConfig, ResidencyGPUResident, true, ResidencyCPU},
		{AssetTypeConfig, ResidencyCPUOnly, true, ResidencyCPU},

		// No backend capability → always CPU regardless of type/mode
		{AssetTypeImage, ResidencyGPUResident, false, ResidencyCPU},
	}
	for _, tt := range tests {
		got := AssetResidency(tt.typ, tt.mode, tt.gpuCapable)
		if got != tt.want {
			t.Errorf("AssetResidency(%v, %v, gpuCapable=%v) = %v, want %v",
				tt.typ, tt.mode, tt.gpuCapable, got, tt.want)
		}
	}
}

// ── TrimMemory GPU-first eviction tests ─────────────────────────────────────

func TestTrimMemory_gpuFirstEviction(t *testing.T) {
	reg := NewAssetRegistryStore()
	backend := &recordingBackend{}

	// Byte budgets: 5KB CPU, 3KB GPU. At 50%: targetCPU=2500, targetGPU=1500.
	// Entries: id1 (CPU=4096, GPU=2048), id2 (CPU=1024, GPU=512).
	// GPU used = 2560 > 1500 → GPU eviction.
	// CPU used = 5120 > 2500 → CPU eviction.
	m := NewManagerWithCache(reg, staticSource{}, BackendSoftware, nil, nil, backend, 5000, 3000)

	id1 := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc020")
	id2 := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc021")

	entry1 := reg.GetOrCreate(id1)
	entry1.Type = AssetTypeImage
	entry2 := reg.GetOrCreate(id2)
	entry2.Type = AssetTypeImage

	reg.SetLODReady(id1, 0, &DecodedImageLOD{Data: []byte("img1-data"), Width: 32, Height: 32}, 100)
	reg.SetLODReady(id2, 0, &DecodedImageLOD{Data: []byte("img2-data"), Width: 16, Height: 16}, 100)

	reg.SetLODGPUReady(id1, 0, TextureID(30), 2048)
	reg.SetLODGPUReady(id2, 0, TextureID(40), 512)

	m.cache.trackLOD(id1, 0, 4096, 2048, TextureID(30), 10)
	m.cache.trackLOD(id2, 0, 1024, 512, TextureID(40), 20)

	if m.cache.gpuUsed != 2560 {
		t.Fatalf("gpuUsed = %d, want 2560", m.cache.gpuUsed)
	}
	if m.cache.usedBytes != 5120 {
		t.Fatalf("usedBytes = %d, want 5120", m.cache.usedBytes)
	}

	// RUNNING_MODERATE (5) → 50% → targetGPU=1500, targetCPU=2500.
	// Both used > target → both eviction passes run.
	count := m.TrimMemory(5)
	if count == 0 {
		t.Fatal("expected TrimMemory(5) to evict entries")
	}
	if len(backend.freed) == 0 {
		t.Fatal("expected GPU textures to be freed during trim")
	}
	if m.cache.gpuUsed > 1500 {
		t.Fatalf("gpuUsed = %d, want <= 1500 after 50%% watermark", m.cache.gpuUsed)
	}
	if m.cache.usedBytes > 2500 {
		t.Fatalf("usedBytes = %d, want <= 2500 after 50%% watermark", m.cache.usedBytes)
	}
}

func TestTrimMemory_moderatePressureFreesGPUBeforeCPU(t *testing.T) {
	reg := NewAssetRegistryStore()
	backend := &recordingBackend{}

	// Byte budgets: 8KB CPU, 2KB GPU.
	// RUNNING_LOW (10) → 25% → targetCPU=2048, targetGPU=512.
	// GPU used: 2560 > 512 → GPU eviction.
	// CPU used: 5120 > 2048 → CPU eviction.
	m := NewManagerWithCache(reg, staticSource{}, BackendSoftware, nil, nil, backend, 8000, 2048)

	id1 := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc022")
	id2 := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc023")

	entry1 := reg.GetOrCreate(id1)
	entry1.Type = AssetTypeImage
	entry2 := reg.GetOrCreate(id2)
	entry2.Type = AssetTypeImage

	reg.SetLODReady(id1, 0, &DecodedImageLOD{Data: []byte("data1"), Width: 32, Height: 32}, 100)
	reg.SetLODReady(id2, 0, &DecodedImageLOD{Data: []byte("data2"), Width: 16, Height: 16}, 100)

	reg.SetLODGPUReady(id1, 0, TextureID(50), 2048)
	reg.SetLODGPUReady(id2, 0, TextureID(60), 512)

	m.cache.trackLOD(id1, 0, 4096, 2048, TextureID(50), 10)
	m.cache.trackLOD(id2, 0, 1024, 512, TextureID(60), 20)

	beforeGPU := m.cache.gpuUsed
	count := m.TrimMemory(10)
	if count == 0 {
		t.Fatal("expected TrimMemory(10) to evict data")
	}
	if m.cache.gpuUsed >= beforeGPU {
		t.Fatal("expected GPU used to decrease after trim")
	}
	if len(backend.freed) == 0 {
		t.Fatal("expected GPU textures to be freed")
	}
}

func TestTrimMemory_gpuFirstBeforeFullEviction(t *testing.T) {
	reg := NewAssetRegistryStore()
	backend := &recordingBackend{}

	// Byte budgets: large enough for CPU data to survive after GPU eviction.
	// id1: sizeBytes=4096. COMPLETE (80) → 0% → evict everything.
	m := NewManagerWithCache(reg, staticSource{}, BackendSoftware, nil, nil, backend, 8000, 3000)

	id1 := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc024")
	id2 := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc025")

	entry1 := reg.GetOrCreate(id1)
	entry1.Type = AssetTypeImage
	entry2 := reg.GetOrCreate(id2)
	entry2.Type = AssetTypeImage

	reg.SetLODReady(id1, 0, &DecodedImageLOD{Data: []byte("data1"), Width: 32, Height: 32}, 100)
	reg.SetLODReady(id2, 0, &DecodedImageLOD{Data: []byte("data2"), Width: 16, Height: 16}, 100)

	reg.SetLODGPUReady(id1, 0, TextureID(70), 2048)
	reg.SetLODGPUReady(id2, 0, TextureID(80), 512)

	// id1 has CPU+GPU, id2 has only GPU.
	m.cache.trackLOD(id1, 0, 4096, 2048, TextureID(70), 10)
	m.cache.trackLOD(id2, 0, 0, 512, TextureID(80), 20)

	if m.cache.gpuUsed != 2560 {
		t.Fatalf("gpuUsed = %d, want 2560", m.cache.gpuUsed)
	}

	// COMPLETE → 0% → GPU eviction clears GPU from both, then EvictToWatermark
	// evicts remaining CPU data (0% targetCPU = 0, usedBytes=4096).
	count := m.TrimMemory(80)
	if count == 0 {
		t.Fatal("expected TrimMemory(80) to evict entries")
	}

	// Both GPU textures freed.
	if len(backend.freed) != 2 {
		t.Fatalf("expected 2 GPU textures freed, got %d: %v", len(backend.freed), backend.freed)
	}

	// At 0%, everything gets evicted (both GPU-only and CPU+GPU entries).
	if m.cache.gpuUsed != 0 {
		t.Fatalf("expected gpuUsed = 0 after complete trim, got %d", m.cache.gpuUsed)
	}
	if m.cache.usedBytes != 0 {
		t.Fatalf("expected usedBytes = 0 after complete trim, got %d", m.cache.usedBytes)
	}
}

func mustAssetID(t *testing.T, s string) AssetID {
	t.Helper()
	id, err := ParseAssetID(s)
	if err != nil {
		t.Fatalf("parse asset id: %v", err)
	}
	return id
}
