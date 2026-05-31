package assets

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// fakeRuntime implements Runtime for testing.
type fakeRuntime struct {
	reg *AssetRegistryStore
}

func (r *fakeRuntime) AssetRegistry() *AssetRegistryStore {
	return r.reg
}

func TestResolveDrawable_returnsGPUTextureWhenGPUReady(t *testing.T) {
	reg := NewAssetRegistryStore()
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc001")
	handle := NewHandle(id, reg)

	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: []byte("rgba-data"), Width: 32, Height: 32}, 100)
	reg.SetLODGPUReady(id, 0, TextureID(42), 4096)

	rt := &fakeRuntime{reg: reg}
	ref, ok := ResolveDrawable(rt, handle, AssetTypeImage)
	if !ok {
		t.Fatal("expected resolve to succeed")
	}
	if ref.Kind != gfx.DrawableGPUTexture {
		t.Fatalf("expected GPUTexture, got %v", ref.Kind)
	}
	if ref.TextureID != uint64(42) {
		t.Fatalf("expected TextureID 42, got %d", ref.TextureID)
	}
	if ref.LOD != 0 {
		t.Fatalf("expected LOD 0, got %d", ref.LOD)
	}
}

func TestResolveDrawable_returnsCPUBitmapWhenOnlyCPU(t *testing.T) {
	reg := NewAssetRegistryStore()
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc002")
	handle := NewHandle(id, reg)

	rgba := make([]byte, 64*64*4)
	for i := range rgba {
		rgba[i] = byte(i)
	}
	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: rgba, Width: 64, Height: 64}, 100)

	rt := &fakeRuntime{reg: reg}
	ref, ok := ResolveDrawable(rt, handle, AssetTypeImage)
	if !ok {
		t.Fatal("expected resolve to succeed")
	}
	if ref.Kind != gfx.DrawableCPUBitmap {
		t.Fatalf("expected CPUBitmap, got %v", ref.Kind)
	}
	if ref.Bitmap == nil {
		t.Fatal("expected non-nil Bitmap")
	}
	if ref.Bitmap.Bounds().Dx() != 64 || ref.Bitmap.Bounds().Dy() != 64 {
		t.Fatalf("expected 64x64 bitmap, got %dx%d", ref.Bitmap.Bounds().Dx(), ref.Bitmap.Bounds().Dy())
	}
	if ref.LOD != 0 {
		t.Fatalf("expected LOD 0, got %d", ref.LOD)
	}
}

func TestResolveDrawable_returnsVectorForSVG(t *testing.T) {
	reg := NewAssetRegistryStore()
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc003")
	handle := NewHandle(id, reg)

	reg.SetLODReady(id, 0, &DecodedSVGLOD0{Data: []byte("<svg>...</svg>")}, 100)

	rt := &fakeRuntime{reg: reg}
	ref, ok := ResolveDrawable(rt, handle, AssetTypeSVG)
	if !ok {
		t.Fatal("expected resolve to succeed")
	}
	if ref.Kind != gfx.DrawableVector {
		t.Fatalf("expected Vector, got %v", ref.Kind)
	}
	if ref.Vector == nil {
		t.Fatal("expected non-nil Vector")
	}
}

func TestResolveDrawable_returnsNoneWhenAbsent(t *testing.T) {
	reg := NewAssetRegistryStore()
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc004")
	handle := NewHandle(id, reg)

	rt := &fakeRuntime{reg: reg}
	_, ok := ResolveDrawable(rt, handle, AssetTypeImage)
	if ok {
		t.Fatal("expected resolve to fail for absent asset")
	}
}

func TestResolveDrawable_returnsNoneForNilRegistry(t *testing.T) {
	rt := &fakeRuntime{reg: nil}
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc005")
	handle := NewHandle(id, nil)

	_, ok := ResolveDrawable(rt, handle, AssetTypeImage)
	if ok {
		t.Fatal("expected resolve to fail with nil registry")
	}
}

func TestResolveDrawable_midUploadReturnsCPUBitmap(t *testing.T) {
	reg := NewAssetRegistryStore()
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc006")
	handle := NewHandle(id, reg)

	rgba := make([]byte, 16*16*4)
	for i := range rgba {
		rgba[i] = 128
	}
	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: rgba, Width: 16, Height: 16}, 100)

	rt := &fakeRuntime{reg: reg}
	ref, ok := ResolveDrawable(rt, handle, AssetTypeImage)
	if !ok {
		t.Fatal("expected resolve to succeed (CPU fallback during mid-upload)")
	}
	if ref.Kind != gfx.DrawableCPUBitmap {
		t.Fatalf("expected CPUBitmap fallback during mid-upload, got %v", ref.Kind)
	}
	if ref.Bitmap == nil {
		t.Fatal("expected non-nil Bitmap from CPU fallback")
	}
}

func TestResolveDrawable_returnsGPUTextureOverCPUBitmap(t *testing.T) {
	reg := NewAssetRegistryStore()
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc007")
	handle := NewHandle(id, reg)

	rgba := make([]byte, 32*32*4)
	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: rgba, Width: 32, Height: 32}, 100)
	reg.SetLODGPUReady(id, 0, TextureID(99), 2048)

	rt := &fakeRuntime{reg: reg}
	ref, ok := ResolveDrawable(rt, handle, AssetTypeImage)
	if !ok {
		t.Fatal("expected resolve to succeed")
	}
	if ref.Kind != gfx.DrawableGPUTexture {
		t.Fatalf("expected GPUTexture over CPUBitmap, got %v", ref.Kind)
	}
	if ref.TextureID != uint64(99) {
		t.Fatalf("expected TextureID 99, got %d", ref.TextureID)
	}
}

func TestResolveDrawable_keyIsStable(t *testing.T) {
	reg := NewAssetRegistryStore()
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc008")
	handle := NewHandle(id, reg)

	rgba := make([]byte, 8*8*4)
	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: rgba, Width: 8, Height: 8}, 100)

	rt := &fakeRuntime{reg: reg}
	ref1, ok1 := ResolveDrawable(rt, handle, AssetTypeImage)
	if !ok1 {
		t.Fatal("first resolve failed")
	}

	ref2, ok2 := ResolveDrawable(rt, handle, AssetTypeImage)
	if !ok2 {
		t.Fatal("second resolve failed")
	}

	if ref1.Key != ref2.Key {
		t.Fatalf("expected stable key, got %q vs %q", ref1.Key, ref2.Key)
	}
	if ref1.Key == "" {
		t.Fatal("expected non-empty key")
	}
}

func TestResolveDrawable_returnsNoneForZeroHandle(t *testing.T) {
	reg := NewAssetRegistryStore()
	rt := &fakeRuntime{reg: reg}

	_, ok := ResolveDrawable(rt, Handle{}, AssetTypeImage)
	if ok {
		t.Fatal("expected resolve to fail for zero handle")
	}
}

func TestResolveDrawable_triggersReuploadCallbackWhenNotGPUReady(t *testing.T) {
	reg := NewAssetRegistryStore()
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc010")
	handle := NewHandle(id, reg)

	rgba := make([]byte, 16*16*4)
	for i := range rgba {
		rgba[i] = 128
	}
	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: rgba, Width: 16, Height: 16}, 100)

	// Register a callback that records the call.
	var calledID AssetID
	var calledLOD int
	prev := gpuReuploadFn
	SetGPUReuploadCallback(func(aid AssetID, lod int) {
		calledID = aid
		calledLOD = lod
	})
	defer SetGPUReuploadCallback(prev)

	rt := &fakeRuntime{reg: reg}
	ref, ok := ResolveDrawable(rt, handle, AssetTypeImage)
	if !ok {
		t.Fatal("expected resolve to succeed (CPU fallback triggers re-upload)")
	}
	if ref.Kind != gfx.DrawableCPUBitmap {
		t.Fatalf("expected CPUBitmap fallback, got %v", ref.Kind)
	}

	if calledID != id {
		t.Fatalf("expected callback with ID %v, got %v", id, calledID)
	}
	if calledLOD != 0 {
		t.Fatalf("expected callback with LOD 0, got %d", calledLOD)
	}
}

func TestResolveDrawable_doesNotTriggerReuploadWhenGPUReady(t *testing.T) {
	reg := NewAssetRegistryStore()
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc011")
	handle := NewHandle(id, reg)

	rgba := make([]byte, 32*32*4)
	reg.SetLODReady(id, 0, &DecodedImageLOD{Data: rgba, Width: 32, Height: 32}, 100)
	reg.SetLODGPUReady(id, 0, TextureID(99), 2048)

	var callbackCalled bool
	prev := gpuReuploadFn
	SetGPUReuploadCallback(func(aid AssetID, lod int) {
		callbackCalled = true
	})
	defer SetGPUReuploadCallback(prev)

	rt := &fakeRuntime{reg: reg}
	ref, ok := ResolveDrawable(rt, handle, AssetTypeImage)
	if !ok {
		t.Fatal("expected resolve to succeed")
	}
	if ref.Kind != gfx.DrawableGPUTexture {
		t.Fatalf("expected GPUTexture, got %v", ref.Kind)
	}
	if callbackCalled {
		t.Fatal("expected no re-upload callback when LOD is already GPU-ready")
	}
}
