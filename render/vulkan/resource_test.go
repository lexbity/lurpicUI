//go:build linux && cgo

package vulkan

import (
	"errors"
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/testframe"
	"codeburg.org/lexbit/lurpicui/render"
)

func TestVulkanActualAllocationBudget(t *testing.T) {
	b := new(Backend)
	if err := b.Initialize(nil); err != nil {
		t.Skipf("Vulkan not available: %v", err)
	}
	defer b.Destroy()

	initial, err := ResourceSnapshot()
	if err != nil {
		t.Fatalf("failed to get initial resource snapshot: %v", err)
	}
	if initial.BudgetBytes == 0 || initial.TotalManaged() > initial.BudgetBytes {
		t.Fatalf("invalid Vulkan budget accounting: managed=%d budget=%d", initial.TotalManaged(), initial.BudgetBytes)
	}
	if initial.TotalManaged() == 0 {
		t.Fatal("Vulkan initialization did not account for its live renderer resources")
	}
	if err := SetResourceBudget(initial.TotalManaged()); err != nil {
		t.Fatalf("set exact current-usage budget: %v", err)
	}
	tempBackend := render.NewVulkanBackend(render.TextureFormatBC7)
	if _, err := tempBackend.UploadTexture(render.TextureUploadRequest{
		AssetID: 99, LOD: 0, PixelData: make([]byte, 64*64*4), Width: 64, Height: 64,
		Format: render.TextureFormatRGBA8, MipLevels: 1,
	}); err == nil {
		t.Fatal("texture allocation succeeded with no unreserved GPU capacity")
	}
	if err := SetResourceBudget(initial.BudgetBytes); err != nil {
		t.Fatalf("restore GPU resource budget after refusal: %v", err)
	}

	imgData := make([]byte, 256*256*4)
	for i := range imgData {
		imgData[i] = byte(i % 256)
	}
	textureID, err := tempBackend.UploadTexture(render.TextureUploadRequest{
		AssetID: 1, LOD: 0, PixelData: imgData, Width: 256, Height: 256,
		Format: render.TextureFormatRGBA8, MipLevels: 1,
	})
	if err != nil {
		t.Fatalf("upload texture: %v", err)
	}
	defer tempBackend.FreeTexture(textureID)

	updated, err := ResourceSnapshot()
	if err != nil {
		t.Fatalf("failed to get updated resource snapshot: %v", err)
	}
	if updated.TotalLive() <= initial.TotalLive() {
		t.Fatalf("texture upload did not increase tracked GPU bytes: before=%d after=%d", initial.TotalLive(), updated.TotalLive())
	}
	if updated.TotalManaged() > updated.BudgetBytes {
		t.Fatalf("Vulkan resource cap exceeded: managed=%d budget=%d", updated.TotalManaged(), updated.BudgetBytes)
	}
	if updated.TotalReserved() != 0 {
		t.Fatalf("successful upload left reservations outstanding: %d", updated.TotalReserved())
	}
	if updated.DeviceGeneration < initial.DeviceGeneration {
		t.Errorf("device generation decreased: was %d, now %d", initial.DeviceGeneration, updated.DeviceGeneration)
	}
}

func TestVulkanFenceRetainsPhysicalCharge(t *testing.T) {
	b := new(Backend)
	if err := b.Initialize(nil); err != nil {
		t.Skipf("Vulkan not available: %v", err)
	}
	defer b.Destroy()
	initial, err := ResourceSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	tempBackend := render.NewVulkanBackend(render.TextureFormatBC7)
	var handles []render.TextureID
	for i := 0; i < 5; i++ {
		handle, err := tempBackend.UploadTexture(render.TextureUploadRequest{
			AssetID: uint64(i), LOD: 0, PixelData: make([]byte, 128*128*4),
			Width: 128, Height: 128, Format: render.TextureFormatRGBA8, MipLevels: 1,
		})
		if err != nil {
			t.Fatalf("upload texture %d: %v", i, err)
		}
		handles = append(handles, handle)
	}
	withResources, err := ResourceSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, handle := range handles {
		tempBackend.FreeTexture(handle)
	}
	afterFree, err := ResourceSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if withResources.TotalManaged() <= initial.TotalManaged() {
		t.Fatalf("texture uploads did not increase managed bytes: baseline=%d current=%d", initial.TotalManaged(), withResources.TotalManaged())
	}
	if afterFree.TotalManaged() != initial.TotalManaged() {
		t.Errorf("synchronous image retirement did not return to baseline: baseline=%d afterFree=%d", initial.TotalManaged(), afterFree.TotalManaged())
	}
}

func TestVulkanResizeGenerationAndCapture(t *testing.T) {
	b := new(Backend)
	if err := b.Initialize(nil); err != nil {
		t.Skipf("Vulkan not available: %v", err)
	}
	defer b.Destroy()
	initialGen := b.DeviceGeneration()
	if initialGen == 0 {
		t.Fatal("expected non-zero initial device generation")
	}

	frame := testFrame(t)
	defer frame.Release()
	packet, err := EncodeFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{64, 48}, {96, 72}} {
		pixels, err := SubmitAndReadback(packet, size[0], size[1])
		if err != nil {
			t.Fatalf("offscreen readback at %dx%d: %v", size[0], size[1], err)
		}
		if want := size[0] * size[1] * 4; len(pixels) != want {
			t.Fatalf("readback returned %d bytes, want %d", len(pixels), want)
		}
	}
	if err := b.Resize(1024, 768); err != nil {
		t.Fatalf("headless resize: %v", err)
	}
	if finalGen := b.DeviceGeneration(); finalGen != initialGen {
		t.Fatalf("headless resize changed device generation: %d -> %d", initialGen, finalGen)
	}
}

func TestVulkanDeviceLossNoFalseReady(t *testing.T) {
	b := new(Backend)
	if err := b.Initialize(nil); err != nil {
		t.Skipf("Vulkan not available: %v", err)
	}
	defer b.Destroy()
	frame := testFrame(t)
	defer frame.Release()
	packet, err := EncodeFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SubmitAndReadback(packet, 32, 32); err != nil {
		t.Fatalf("baseline readback: %v", err)
	}
	initial, err := ResourceSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	// InjectDeviceLost is a deterministic failure hook; physical device-loss
	// behavior still requires the Vulkan device acceptance run.
	if err := InjectDeviceLost(); err != nil {
		t.Fatal(err)
	}
	_, err = SubmitAndReadback(packet, 32, 32)
	var gpuFatal *render.ErrGPUFatal
	if !errors.As(err, &gpuFatal) {
		t.Fatalf("injected device loss must fail the submitted render as GPU fatal, got %v", err)
	}
	currentGen := b.DeviceGeneration()
	if currentGen <= initial.DeviceGeneration {
		t.Fatal("expected device generation to increase after device loss")
	}
	final, err := ResourceSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if final.DeviceGeneration != currentGen {
		t.Errorf("resource snapshot generation %d != backend generation %d", final.DeviceGeneration, currentGen)
	}
	if _, err := SubmitAndReadback(packet, 32, 32); err != nil {
		t.Fatalf("one-shot device-loss fault remained armed after failure: %v", err)
	}
}

func testFrame(t *testing.T) *render.Frame {
	t.Helper()
	frame := testframe.Seal(render.FrameInput{Batches: []render.BatchInput{}})
	return frame
}
