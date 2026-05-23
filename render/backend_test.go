package render

import (
	"bytes"
	"testing"
)

type dummyTextureBackend struct {
	budget int
	count  int
}

func (b *dummyTextureBackend) UploadTexture(req TextureUploadRequest) (TextureID, error) {
	b.count++
	if req.ResultCh != nil {
		req.ResultCh <- TextureUploadResult{AssetID: req.AssetID, TextureID: TextureID(b.count), LOD: 0}
	}
	return TextureID(b.count), nil
}

func (b *dummyTextureBackend) FreeTexture(TextureID)          {}
func (b *dummyTextureBackend) UploadBudgetBytesPerFrame() int { return b.budget }
func (b *dummyTextureBackend) TranscodeTarget() TextureFormat { return TextureFormatRGBA8 }

func TestSoftwareBackendReusesFreedIDs(t *testing.T) {
	var backend SoftwareBackend
	req := TextureUploadRequest{
		AssetID:   1,
		PixelData: bytes.Repeat([]byte{0xff, 0x00, 0x00, 0xff}, 4),
		Width:     2,
		Height:    2,
		Format:    TextureFormatRGBA8,
		MipLevels: 1,
	}

	id1, err := backend.UploadTexture(req)
	if err != nil {
		t.Fatalf("upload 1: %v", err)
	}
	if got := backend.textureCount(); got != 1 {
		t.Fatalf("unexpected texture count: %d", got)
	}

	backend.FreeTexture(id1)
	if got := backend.textureCount(); got != 0 {
		t.Fatalf("expected pool cleared, got %d", got)
	}

	id2, err := backend.UploadTexture(req)
	if err != nil {
		t.Fatalf("upload 2: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("expected freed id reuse, got %d want %d", id2, id1)
	}
}

func TestUploadQueueRespectsBudgetAndResults(t *testing.T) {
	backend := &dummyTextureBackend{budget: 8}
	queue := NewUploadQueue(backend, 4)
	results := make(chan TextureUploadResult, 4)
	for i := 0; i < 2; i++ {
		if !queue.Enqueue(TextureUploadRequest{
			AssetID:   uint64(i + 1),
			PixelData: []byte{1, 2, 3, 4, 5},
			Width:     1,
			Height:    1,
			Format:    TextureFormatRGBA8,
			ResultCh:  results,
		}) {
			t.Fatal("enqueue failed")
		}
	}

	queue.DrainBudget()
	if backend.count != 1 {
		t.Fatalf("expected one upload within budget, got %d", backend.count)
	}
	if got := len(results); got != 1 {
		t.Fatalf("unexpected result count: %d", got)
	}
	if got := (<-results).AssetID; got != 1 {
		t.Fatalf("unexpected result asset: %d", got)
	}
}

func BenchmarkUploadQueueDrainBudget(b *testing.B) {
	backend := &dummyTextureBackend{budget: 1 << 20}
	queue := NewUploadQueue(backend, 1024)
	payload := bytes.Repeat([]byte{0, 1, 2, 3}, 256)
	for i := 0; i < 512; i++ {
		_ = queue.Enqueue(TextureUploadRequest{
			AssetID:   uint64(i + 1),
			PixelData: payload,
			Width:     32,
			Height:    32,
			Format:    TextureFormatRGBA8,
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.DrainBudget()
	}
}

func TestVulkanBackendDelegatesToRegisteredHooks(t *testing.T) {
	orig := currentVulkanHooks()
	defer RegisterVulkanTextureHooks(orig)

	var uploads, frees int
	RegisterVulkanTextureHooks(VulkanTextureHooks{
		Upload: func(req TextureUploadRequest) (TextureID, error) {
			uploads++
			if req.Format != TextureFormatRGBA8 {
				t.Fatalf("unexpected request format: %v", req.Format)
			}
			return 99, nil
		},
		Free: func(id TextureID) {
			if id != 99 {
				t.Fatalf("unexpected free id: %d", id)
			}
			frees++
		},
		Budget: func(target TextureFormat) int {
			if target != TextureFormatBC7 {
				t.Fatalf("unexpected target: %v", target)
			}
			return 123
		},
		Target: func() TextureFormat {
			return TextureFormatBC7
		},
	})

	backend := NewVulkanBackend(0)
	if got := backend.TranscodeTarget(); got != TextureFormatBC7 {
		t.Fatalf("unexpected target: %v", got)
	}
	if got := backend.UploadBudgetBytesPerFrame(); got != 123 {
		t.Fatalf("unexpected budget: %d", got)
	}
	id, err := backend.UploadTexture(TextureUploadRequest{
		AssetID:   7,
		PixelData: bytes.Repeat([]byte{1, 2, 3, 4}, 4),
		Width:     2,
		Height:    2,
		Format:    TextureFormatRGBA8,
	})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if id != 99 || uploads != 1 {
		t.Fatalf("unexpected upload result: id=%d uploads=%d", id, uploads)
	}
	backend.FreeTexture(id)
	if frees != 1 {
		t.Fatalf("unexpected free count: %d", frees)
	}
}
