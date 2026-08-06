//go:build linux && cgo

package vulkan_test

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

func checkerRGBA(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if (x+y)%2 == 0 {
				img.SetRGBA(x, y, color.RGBA{R: 220, G: 60, B: 60, A: 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{R: 60, G: 120, B: 220, A: 255})
			}
		}
	}
	return img
}

func renderFramePixels(t *testing.T, frame *render.Frame, width, height int) []byte {
	t.Helper()
	packet, err := vulkan.EncodeFrame(frame)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := vulkan.SubmitAndReadback(packet, width, height)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	return out
}

// TestDrawTexture_Rendered proves DrawTexture renders end-to-end through the
// Slice 4 texture pipeline: a texture uploaded via the FFI handle round-trips
// into the frame packet, decodes, and samples the same VkImage the equivalent
// DrawImage does. The outputs must be byte-identical.
func TestDrawTexture_Rendered(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() {
		if err := vulkan.Shutdown(); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}()

	img := checkerRGBA(8, 8)
	handle, err := vulkan.UploadImage(img.Pix, 8, 8, img.Stride, 0)
	if err != nil {
		t.Fatalf("upload texture: %v", err)
	}
	defer func() {
		if err := vulkan.DestroyImage(handle); err != nil {
			t.Fatalf("destroy texture: %v", err)
		}
	}()

	const width, height = 32, 32
	texFrame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:       1,
			Bounds:   gfx.RectFromXYWH(0, 0, width, height),
			Opacity:  1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.DrawTexture{
					TextureID: handle,
					DestRect:  gfx.RectFromXYWH(4, 4, 16, 16),
					SrcRect:   gfx.RectFromXYWH(0, 0, 8, 8),
					Sampling:  gfx.SamplingNearest,
					Opacity:   1,
				},
			}},
		}},
	}
	imgFrame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:       1,
			Bounds:   gfx.RectFromXYWH(0, 0, width, height),
			Opacity:  1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.DrawImage{
					Image:    img,
					DestRect: gfx.RectFromXYWH(4, 4, 16, 16),
					SrcRect:  gfx.RectFromXYWH(0, 0, 8, 8),
					Sampling: gfx.SamplingNearest,
					Opacity:  1,
				},
			}},
		}},
	}

	texOut := renderFramePixels(t, texFrame, width, height)
	imgOut := renderFramePixels(t, imgFrame, width, height)

	// The DrawTexture command must draw the same content as the equivalent
	// DrawImage (same texture, same geometry, same pipeline).
	if !bytes.Equal(texOut, imgOut) {
		t.Fatal("DrawTexture rendered differently from the equivalent DrawImage")
	}

	// The texture must actually be drawn: the dest region is opaque checker.
	if !regionRendered(texOut, width, height, 4, 4, 20, 20) {
		t.Fatal("DrawTexture output is blank where the texture should be drawn")
	}
}

func regionRendered(pixels []byte, width, height, x0, y0, x1, y1 int) bool {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			off := (y*width + x) * 4
			if off+3 >= len(pixels) {
				continue
			}
			if pixels[off+3] != 0 {
				return true
			}
		}
	}
	return false
}

// TestDrawImage_Bilinear verifies the Slice 4 GPU bilinear path directly. The
// software oracle renders textures nearest-only (its backend is unchanged in
// Slice 4), so the corpus bilinear fixtures stay deferred; this test proves the
// sampler's linear CLAMP_TO_EDGE path is wired by sampling a known 2x2 gradient
// at a known pixel and checking against the GPU linear-filter convention.
func TestDrawImage_Bilinear(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() {
		if err := vulkan.Shutdown(); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}()

	// 2x2 corner colors: red, green, blue, white (premultiplied, A=255).
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	img.SetRGBA(1, 0, color.RGBA{R: 0, G: 255, B: 0, A: 255})
	img.SetRGBA(0, 1, color.RGBA{R: 0, G: 0, B: 255, A: 255})
	img.SetRGBA(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	const width, height = 4, 4
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:       1,
			Bounds:   gfx.RectFromXYWH(0, 0, width, height),
			Opacity:  1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.DrawImage{
					Image:    img,
					DestRect: gfx.RectFromXYWH(0, 0, width, height),
					SrcRect:  gfx.RectFromXYWH(0, 0, 2, 2),
					Sampling: gfx.SamplingBilinear,
					Opacity:  1,
				},
			}},
		}},
	}
	out := renderFramePixels(t, frame, width, height)

	// At dest pixel (2,2) center (2.5,2.5), sx = sy = 2.5/4*2 = 1.25.
	// GPU linear convention: xf = 1.25-0.5 = 0.75, x0 = 0, x1 = 1, wx = 0.75.
	// top  = lerp(red,   green, 0.75) = (63.75, 191.25, 0)
	// bot  = lerp(blue,  white, 0.75) = (191.25, 191.25, 255)
	// out  = lerp(top,   bot,   0.75) = (159.4, 191.25, 191.25)
	expect := []uint8{159, 191, 191, 255}
	off := (2*width + 2) * 4
	got := out[off : off+4]
	for i := range expect {
		d := int(expect[i]) - int(got[i])
		if d < 0 {
			d = -d
		}
		if d > 3 {
			t.Fatalf("bilinear pixel (2,2) = %v, want %v (±3)", got, expect)
		}
	}

	// The bilinear result must differ from the nearest result (proving the
	// linear filter is actually applied, not falling back to nearest).
	nearestFrame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:       1,
			Bounds:   gfx.RectFromXYWH(0, 0, width, height),
			Opacity:  1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.DrawImage{
					Image:    img,
					DestRect: gfx.RectFromXYWH(0, 0, width, height),
					SrcRect:  gfx.RectFromXYWH(0, 0, 2, 2),
					Sampling: gfx.SamplingNearest,
					Opacity:  1,
				},
			}},
		}},
	}
	nearest := renderFramePixels(t, nearestFrame, width, height)
	if bytes.Equal(out, nearest) {
		t.Fatal("bilinear output equals nearest output; the linear filter was not applied")
	}
}

// TestTextureLifecycle_ReleasesResources verifies the Slice 4 image store
// tracks creation/destruction: uploading N textures grows the store by N, and
// destroying them returns it to the baseline with a matching destroy count
// (RAII on the GPU resources, NFR-8-style resource accounting).
func TestTextureLifecycle_ReleasesResources(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() {
		if err := vulkan.Shutdown(); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}()

	baseline, err := vulkan.TestImageCount()
	if err != nil {
		t.Skipf("test-exports unavailable: %v", err)
	}
	destroyBaseline, err := vulkan.TestImageDestroyCount()
	if err != nil {
		t.Skipf("test-exports unavailable: %v", err)
	}

	img := checkerRGBA(4, 4)
	handles := make([]uint64, 0, 3)
	for i := 0; i < 3; i++ {
		handle, err := vulkan.UploadImage(img.Pix, 4, 4, img.Stride, 0)
		if err != nil {
			t.Fatalf("upload %d: %v", i, err)
		}
		handles = append(handles, handle)
	}

	after, err := vulkan.TestImageCount()
	if err != nil {
		t.Fatalf("image count: %v", err)
	}
	if after != baseline+3 {
		t.Fatalf("image count = %d, want %d after 3 uploads", after, baseline+3)
	}

	for _, handle := range handles {
		if err := vulkan.DestroyImage(handle); err != nil {
			t.Fatalf("destroy %d: %v", handle, err)
		}
	}

	afterDestroy, err := vulkan.TestImageCount()
	if err != nil {
		t.Fatalf("image count after destroy: %v", err)
	}
	if afterDestroy != baseline {
		t.Fatalf("image count = %d, want baseline %d after destroying all uploads", afterDestroy, baseline)
	}
	destroyCount, err := vulkan.TestImageDestroyCount()
	if err != nil {
		t.Fatalf("destroy count: %v", err)
	}
	if destroyCount != destroyBaseline+3 {
		t.Fatalf("destroy count = %d, want %d after 3 destroys", destroyCount, destroyBaseline+3)
	}
}

// TestDrawTexture_UnknownHandleRejected: a DrawTexture referencing a handle the
// image store does not own must fail at render time rather than silently
// dropping the draw (no silent degradation).
func TestDrawTexture_UnknownHandleRejected(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() {
		if err := vulkan.Shutdown(); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}()

	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:       1,
			Bounds:   gfx.RectFromXYWH(0, 0, 32, 32),
			Opacity:  1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.DrawTexture{
					TextureID: 0xdead_beef,
					DestRect:  gfx.RectFromXYWH(4, 4, 16, 16),
					SrcRect:   gfx.RectFromXYWH(0, 0, 8, 8),
					Sampling:  gfx.SamplingNearest,
					Opacity:   1,
				},
			}},
		}},
	}
	packet, err := vulkan.EncodeFrame(frame)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := vulkan.SubmitAndReadback(packet, 32, 32); err == nil {
		t.Fatal("expected an error when rendering a DrawTexture with an unknown handle")
	}
}
