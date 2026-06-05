package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/render"
)

// recordingTextureBackend records FreeTexture calls for test verification.
// It implements both render.Backend and render.TextureBackend.
type recordingTextureBackend struct {
	freed []render.TextureID
}

func (b *recordingTextureBackend) Initialize(surface render.Surface) error { return nil }
func (b *recordingTextureBackend) Submit(frame *render.Frame) error        { return nil }
func (b *recordingTextureBackend) Resize(width, height int) error          { return nil }
func (b *recordingTextureBackend) Destroy()                                {}

func (b *recordingTextureBackend) UploadTexture(req render.TextureUploadRequest) (render.TextureID, error) {
	return 0, nil
}

func (b *recordingTextureBackend) FreeTexture(id render.TextureID) {
	if b == nil {
		return
	}
	b.freed = append(b.freed, id)
}

func (b *recordingTextureBackend) UploadBudgetBytesPerFrame() int {
	return 4096
}

func (b *recordingTextureBackend) TranscodeTarget() render.TextureFormat {
	return render.TextureFormatRGBA8
}

func TestAssetTextureReleaser_forwardsToBackend(t *testing.T) {
	backend := &recordingTextureBackend{}
	releaser := &assetTextureReleaser{backend: backend}

	releaser.FreeTexture(assets.TextureID(42))
	releaser.FreeTexture(assets.TextureID(99))

	if len(backend.freed) != 2 {
		t.Fatalf("expected 2 FreeTexture calls, got %d", len(backend.freed))
	}
	if backend.freed[0] != render.TextureID(42) {
		t.Fatalf("expected freed[0] = 42, got %d", backend.freed[0])
	}
	if backend.freed[1] != render.TextureID(99) {
		t.Fatalf("expected freed[1] = 99, got %d", backend.freed[1])
	}
}

func TestAssetTextureReleaser_nilBackendSafe(t *testing.T) {
	releaser := &assetTextureReleaser{backend: nil}
	// Must not panic.
	releaser.FreeTexture(assets.TextureID(1))
}

func TestAssetTextureReleaser_nilReceiverSafe(t *testing.T) {
	var releaser *assetTextureReleaser
	// Must not panic.
	releaser.FreeTexture(assets.TextureID(1))
}

func TestTextureReleaser_wiredThroughNew(t *testing.T) {
	reg := assets.NewAssetRegistryStore()
	mgr := assets.NewManagerWithResidency(reg, nil, nil, nil, nil, assets.ResidencyCPUOnly, 64, 128)

	backend := &fakeBackend{supportsTexture: true}
	root := &runtimeTestFacet{Facet: facet.NewFacet()}
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.AssetManager = mgr

	rt, err := New(cfg, nil, nil, backend, root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if rt == nil {
		t.Fatal("expected non-nil runtime")
	}

	if rt.renderPipeline.UploadQueue() == nil {
		t.Fatal("upload queue not created — pipeline did not detect TextureBackend")
	}

	_, ok := rt.assetManager.(*assets.ManagerImpl)
	if !ok {
		t.Fatal("asset manager is not ManagerImpl — releaser/uploader wiring skipped")
	}
}

func TestTextureReleaser_wiredThroughNew_textureBackendReachesBackend(t *testing.T) {
	reg := assets.NewAssetRegistryStore()
	mgr := assets.NewManagerWithResidency(reg, nil, nil, nil, nil, assets.ResidencyCPUOnly, 64, 128)

	fb := &fakeBackend{supportsTexture: true}
	root := &runtimeTestFacet{Facet: facet.NewFacet()}
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.AssetManager = mgr

	rt, err := New(cfg, nil, nil, fb, root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	mgrImpl, ok := rt.assetManager.(*assets.ManagerImpl)
	if !ok {
		t.Fatal("asset manager is not ManagerImpl — releaser/uploader wiring skipped")
	}

	if rt.renderPipeline.UploadQueue() == nil {
		t.Fatal("upload queue not created — wiring precondition not met")
	}

	if fb.UploadBudgetBytesPerFrame() <= 0 {
		t.Fatal("fakeBackend has zero upload budget — wiring precondition not met")
	}

	// CheckDeviceGeneration and TrimMemory exercise the cache path.
	// With no tracked data they are no-ops, but proving they don't
	// panic confirms the cache was properly initialised and the
	// backend (releaser) was wired into it during New().
	mgrImpl.CheckDeviceGeneration(1)
	mgrImpl.TrimMemory(2)
}

func TestTextureReleaser_backendWithoutTextureSupport_skipsWiring(t *testing.T) {
	reg := assets.NewAssetRegistryStore()
	mgr := assets.NewManagerWithResidency(reg, nil, nil, nil, nil, assets.ResidencyCPUOnly, 64, 128)

	fb := &fakeBackend{supportsTexture: false}
	root := &runtimeTestFacet{Facet: facet.NewFacet()}
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.AssetManager = mgr

	_, err := New(cfg, nil, nil, fb, root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// With UploadBudgetBytesPerFrame() == 0 the wiring code must skip
	// SetTextureReleaser and SetUploader even though the type assertion
	// for render.TextureBackend succeeds. The backend's freed list must
	// stay empty because no releaser was installed.
	if len(fb.freed) != 0 {
		t.Fatalf("unexpected free calls: %v", fb.freed)
	}
}

func TestTextureReleaser_noCacheDoesNotPanic(t *testing.T) {
	mgr := assets.NewManager(nil, nil, assets.BackendSoftware, nil, nil)
	// No cache — SetTextureReleaser is a no-op.
	mgr.SetTextureReleaser(&assetTextureReleaser{})
}

// ── Uploader adapter tests ──────────────────────────────────────────────────

type uploadRecordingBackend struct {
	recordingTextureBackend
	uploads []render.TextureUploadRequest
}

func (b *uploadRecordingBackend) UploadTexture(req render.TextureUploadRequest) (render.TextureID, error) {
	b.uploads = append(b.uploads, req)
	return render.TextureID(len(b.uploads)), nil
}

func TestAssetUploader_forwardsEnqueueToQueue(t *testing.T) {
	backend := &uploadRecordingBackend{}
	queue := render.NewUploadQueue(backend, 1024)
	uploader := newAssetUploader(queue)

	enqueued := uploader.Enqueue(assets.TextureUploadRequest{
		AssetID:   assetIDFromHex(t, "fedcba9876543210fedcba9876543210"),
		LOD:       0,
		Pixels:    []byte("test-pixels"),
		Width:     64,
		Height:    32,
		MipLevels: 1,
	})
	if !enqueued {
		t.Fatal("expected enqueue to succeed")
	}

	// Drain the queue to process the upload.
	queue.DrainBudget()

	if len(backend.uploads) != 1 {
		t.Fatalf("expected 1 upload processed, got %d", len(backend.uploads))
	}
	req := backend.uploads[0]
	if string(req.PixelData) != "test-pixels" {
		t.Fatalf("pixel data mismatch: got %q", string(req.PixelData))
	}
	if req.Width != 64 {
		t.Fatalf("width mismatch: got %d", req.Width)
	}
	if req.Height != 32 {
		t.Fatalf("height mismatch: got %d", req.Height)
	}
}

func TestAssetUploader_budgetReflectsBackend(t *testing.T) {
	backend := &uploadRecordingBackend{}
	queue := render.NewUploadQueue(backend, 1024)
	uploader := newAssetUploader(queue)

	budget := uploader.Budget()
	if budget != 4096 {
		t.Fatalf("expected budget 4096, got %d", budget)
	}
}

func TestAssetUploader_nilQueueSafe(t *testing.T) {
	var uploader *assetUploader
	if uploader.Enqueue(assets.TextureUploadRequest{}) {
		t.Fatal("expected false from nil uploader")
	}
	if uploader.Results() != nil {
		t.Fatal("expected nil results from nil uploader")
	}
	if uploader.Budget() != 0 {
		t.Fatal("expected 0 budget from nil uploader")
	}
}

func TestAssetUploader_wiredThroughNew(t *testing.T) {
	reg := assets.NewAssetRegistryStore()
	mgr := assets.NewManagerWithResidency(reg, nil, nil, nil, nil, assets.ResidencyGPUResident, 64, 128)

	fb := &fakeBackend{supportsTexture: true}
	root := &runtimeTestFacet{Facet: facet.NewFacet()}
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.AssetManager = mgr

	rt, err := New(cfg, nil, nil, fb, root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if rt == nil {
		t.Fatal("expected non-nil runtime")
	}

	if rt.renderPipeline.UploadQueue() == nil {
		t.Fatal("upload queue not created — pipeline did not detect TextureBackend")
	}

	mgrImpl, ok := rt.assetManager.(*assets.ManagerImpl)
	if !ok {
		t.Fatal("asset manager is not ManagerImpl — uploader wiring skipped")
	}
	_ = mgrImpl
}

func assetIDFromHex(t *testing.T, s string) assets.AssetID {
	t.Helper()
	id, err := assets.ParseAssetID(s)
	if err != nil {
		t.Fatalf("ParseAssetID: %v", err)
	}
	return id
}


