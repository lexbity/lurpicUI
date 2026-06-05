package runtime

import (
	"sync"
	"sync/atomic"
	"testing"

	"codeburg.org/lexbit/lurpicui/render"
)

type fakeBackend struct {
	initCount     atomic.Int32
	recreateCount atomic.Int32
	submitCount   atomic.Int32
	destroyCount  atomic.Int32

	initializeErr error
	submitErr     error

	freed      []render.TextureID
	freedMu    sync.Mutex

	uploads          []uploadRecord
	uploadsMu        sync.Mutex

	supportsTexture  bool
}

type uploadRecord struct {
	AssetID  uint64
	LOD      int
	Data     []byte
	Width    uint16
	Height   uint16
	Format   render.TextureFormat
}

func (b *fakeBackend) Initialize(surface render.Surface) error {
	b.initCount.Add(1)
	return b.initializeErr
}

func (b *fakeBackend) Submit(frame *render.Frame) error {
	b.submitCount.Add(1)
	return b.submitErr
}

func (b *fakeBackend) Resize(width, height int) error {
	return nil
}

func (b *fakeBackend) Destroy() {
	b.destroyCount.Add(1)
}

func (b *fakeBackend) Recreate(surface render.Surface) error {
	b.recreateCount.Add(1)
	return nil
}

func (b *fakeBackend) UploadTexture(req render.TextureUploadRequest) (render.TextureID, error) {
	if !b.supportsTexture {
		return 0, nil
	}
	b.uploadsMu.Lock()
	b.uploads = append(b.uploads, uploadRecord{
		AssetID: req.AssetID,
		LOD:     req.LOD,
		Data:    append([]byte(nil), req.PixelData...),
		Width:   req.Width,
		Height:  req.Height,
		Format:  req.Format,
	})
	b.uploadsMu.Unlock()
	return render.TextureID(len(b.uploads)), nil
}

func (b *fakeBackend) FreeTexture(id render.TextureID) {
	if b == nil {
		return
	}
	b.freedMu.Lock()
	b.freed = append(b.freed, id)
	b.freedMu.Unlock()
}

func (b *fakeBackend) UploadBudgetBytesPerFrame() int {
	if !b.supportsTexture {
		return 0
	}
	return 4096
}

func (b *fakeBackend) TranscodeTarget() render.TextureFormat {
	return render.TextureFormatRGBA8
}

var _ render.Backend = (*fakeBackend)(nil)
var _ render.TextureBackend = (*fakeBackend)(nil)
var _ render.RecreatableBackend = (*fakeBackend)(nil)

func TestFakeBackend_counts_smoke(t *testing.T) {
	b := &fakeBackend{}

	if err := b.Initialize(nil); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if b.initCount.Load() != 1 {
		t.Fatalf("initCount = %d, want 1", b.initCount.Load())
	}

	if err := b.Submit(nil); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if b.submitCount.Load() != 1 {
		t.Fatalf("submitCount = %d, want 1", b.submitCount.Load())
	}

	b.Destroy()
	if b.destroyCount.Load() != 1 {
		t.Fatalf("destroyCount = %d, want 1", b.destroyCount.Load())
	}

	if err := b.Recreate(nil); err != nil {
		t.Fatalf("Recreate: %v", err)
	}
	if b.recreateCount.Load() != 1 {
		t.Fatalf("recreateCount = %d, want 1", b.recreateCount.Load())
	}
}

func TestFakeBackend_texture_free_smoke(t *testing.T) {
	b := &fakeBackend{supportsTexture: true}
	b.FreeTexture(render.TextureID(7))
	if len(b.freed) != 1 || b.freed[0] != 7 {
		t.Fatalf("freed = %v, want [7]", b.freed)
	}
}

func TestFakeBackend_supportsTexture_controls_budget(t *testing.T) {
	b1 := &fakeBackend{supportsTexture: true}
	if got := b1.UploadBudgetBytesPerFrame(); got == 0 {
		t.Fatal("expected non-zero budget with texture support")
	}

	b2 := &fakeBackend{supportsTexture: false}
	if got := b2.UploadBudgetBytesPerFrame(); got != 0 {
		t.Fatalf("expected zero budget without texture, got %d", got)
	}
}
