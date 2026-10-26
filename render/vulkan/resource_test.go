//go:build linux && cgo

package vulkan

import (
	"errors"
	"runtime"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
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
	tempBackend := render.NewVulkanBackend(render.TextureFormatBC7)
	empty := testframe.Seal(render.FrameInput{Batches: []render.BatchInput{}})
	emptyPacket, err := EncodeFrame(empty)
	empty.Release()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SubmitAndReadback(emptyPacket, 32, 32); err != nil {
		t.Fatalf("warm readback resources: %v", err)
	}
	baseline, err := ResourceSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	textureID, err := tempBackend.UploadTexture(render.TextureUploadRequest{
		AssetID: 77, LOD: 0, PixelData: make([]byte, 16*16*4),
		Width: 16, Height: 16, Format: render.TextureFormatRGBA8, MipLevels: 1,
	})
	if err != nil {
		t.Fatalf("upload texture: %v", err)
	}
	frame := testframe.Seal(render.FrameInput{Batches: []render.BatchInput{{
		Bounds: gfx.RectFromXYWH(0, 0, 16, 16), Opacity: 1,
		Commands: gfx.CommandList{Commands: []gfx.Command{gfx.DrawTexture{
			TextureID: uint64(textureID), DestRect: gfx.RectFromXYWH(0, 0, 16, 16),
			SrcRect: gfx.RectFromXYWH(0, 0, 1, 1), Sampling: gfx.SamplingNearest, Opacity: 1,
		}}},
	}}})
	defer frame.Release()
	packet, err := EncodeFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	if err := TestPauseNextFenceWait(); err != nil {
		t.Fatalf("arm real submit fence barrier: %v", err)
	}
	releaseFence := true
	defer func() {
		if releaseFence {
			if err := TestReleaseFenceWait(); err != nil {
				t.Errorf("release fence barrier during cleanup: %v", err)
			}
		}
	}()
	submitDone := make(chan error, 1)
	go func() {
		_, err := SubmitAndReadback(packet, 32, 32)
		submitDone <- err
	}()
	waitForFenceTestState(t, true)
	withTexture, err := ResourceSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if withTexture.TotalManaged() <= baseline.TotalManaged() {
		t.Fatalf("submitted texture did not retain a physical allocation charge: baseline=%d current=%d", baseline.TotalManaged(), withTexture.TotalManaged())
	}
	freeStarted := make(chan struct{})
	freeDone := make(chan struct{})
	go func() {
		close(freeStarted)
		tempBackend.FreeTexture(textureID)
		close(freeDone)
	}()
	<-freeStarted
	select {
	case <-freeDone:
		t.Fatal("texture was destroyed before the queue submission fence completed")
	case <-time.After(25 * time.Millisecond):
	}
	stillPinned, err := ResourceSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if stillPinned.TotalManaged() != withTexture.TotalManaged() {
		t.Fatalf("fence-blocked texture charge changed: before=%d while blocked=%d", withTexture.TotalManaged(), stillPinned.TotalManaged())
	}
	if err := TestReleaseFenceWait(); err != nil {
		t.Fatal(err)
	}
	releaseFence = false
	select {
	case err := <-submitDone:
		if err != nil {
			t.Fatalf("render submitted texture: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("render did not complete after releasing its real queue fence")
	}
	select {
	case <-freeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("texture destruction did not resume after queue completion")
	}
	afterFree, err := ResourceSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if afterFree.TotalManaged() != baseline.TotalManaged() {
		t.Errorf("texture charge did not retire after the queue fence: baseline=%d after=%d", baseline.TotalManaged(), afterFree.TotalManaged())
	}
}

func waitForFenceTestState(t *testing.T, want bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if TestFenceWaitPaused() == want {
			return
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for Vulkan fence barrier paused=%v", want)
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
