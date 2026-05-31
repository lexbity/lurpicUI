package assets

import (
	"fmt"
	"image"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// Runtime is the narrow seam the drawable resolver needs from the runtime.
type Runtime interface {
	AssetRegistry() *AssetRegistryStore
}

// gpuReuploadFn is the optional callback set by the runtime, called by
// ResolveDrawable when a GPU-eligible LOD is CPU-ready but not GPU-ready,
// requesting a re-upload (e.g. after device-loss or eviction). Must be
// set once at startup and never unset. Nil means no re-upload is requested.
var gpuReuploadFn func(id AssetID, lod int)

// SetGPUReuploadCallback sets the function called by ResolveDrawable when
// a GPU-eligible asset LOD is CPU-ready but not GPU-ready, requesting a
// re-upload. This is used by the runtime after device-loss recovery and
// must be called on the runtime thread during initialisation.
func SetGPUReuploadCallback(fn func(id AssetID, lod int)) {
	gpuReuploadFn = fn
}

// ResolveDrawable picks the highest-residency representation available for
// handle: GPUTexture > CPUBitmap/Vector, falling back gracefully while a
// GPU upload is still in flight. Pure; no allocation of GPU resources.
func ResolveDrawable(rt Runtime, handle Handle, typ AssetType) (gfx.DrawableRef, bool) {
	reg := rt.AssetRegistry()
	if reg == nil {
		reg = handle.Registry()
	}
	if reg == nil {
		return gfx.DrawableRef{}, false
	}

	entry := reg.Get(handle.ID)
	if entry == nil {
		return gfx.DrawableRef{}, false
	}

	lod := entry.HighestReadyLOD
	if lod < 0 || lod >= len(entry.LODHandles) {
		return gfx.DrawableRef{}, false
	}

	// GPU path: highest priority.
	if entry.LODGPUReady[lod] && entry.LODTextureIDs[lod] != 0 {
		return gfx.DrawableRef{
			Kind:      gfx.DrawableGPUTexture,
			TextureID: uint64(entry.LODTextureIDs[lod]),
			LOD:       lod,
			SrcBox:    srcBoxForEntry(entry, lod),
			Key:       drawableKey(entry.ID, lod, entry.EntryVersion),
		}, true
	}

	// CPU fallback: decoded data.
	if decoded := entry.LODHandles[lod]; decoded != nil {
		switch typ {
		case AssetTypeImage:
			if img, ok := decoded.(*DecodedImageLOD); ok && len(img.Data) > 0 {
				// Request GPU re-upload per asset-type policy when the
				// LOD is CPU-ready but not GPU-ready. Non-eligible types
				// (SVG, config, font) never trigger re-upload here.
				if gpuReuploadFn != nil && !entry.LODGPUReady[lod] && GPUUploadEligible(typ) {
					gpuReuploadFn(entry.ID, lod)
				}
				return gfx.DrawableRef{
					Kind:   gfx.DrawableCPUBitmap,
					Bitmap: imageFromDecoded(img),
					LOD:    lod,
					SrcBox: fullSrcBox(img),
					Key:    drawableKey(entry.ID, lod, entry.EntryVersion),
				}, true
			}
		case AssetTypeSVG:
			if _, ok := decoded.(*DecodedSVGLOD0); ok {
				return gfx.DrawableRef{
					Kind:   gfx.DrawableVector,
					Vector: decoded,
					LOD:    lod,
					Key:    drawableKey(entry.ID, lod, entry.EntryVersion),
				}, true
			}
		}
	}

	return gfx.DrawableRef{}, false
}

func drawableKey(id AssetID, lod int, version uint64) string {
	return fmt.Sprintf("asset:%s:%d:%d", id, lod, version)
}

func fullSrcBox(img *DecodedImageLOD) gfx.Rect {
	if img.Width > 0 && img.Height > 0 {
		return gfx.RectFromXYWH(0, 0, float32(img.Width), float32(img.Height))
	}
	return gfx.Rect{}
}

func srcBoxForEntry(entry *RegistryEntry, lod int) gfx.Rect {
	if decoded := entry.LODHandles[lod]; decoded != nil {
		if img, ok := decoded.(*DecodedImageLOD); ok {
			return fullSrcBox(img)
		}
	}
	return gfx.Rect{}
}

func imageFromDecoded(img *DecodedImageLOD) *image.RGBA {
	w := int(img.Width)
	h := int(img.Height)
	if w <= 0 || h <= 0 || len(img.Data) == 0 {
		return nil
	}
	if len(img.Data) == w*h*4 {
		return &image.RGBA{
			Pix:    img.Data,
			Stride: w * 4,
			Rect:   image.Rect(0, 0, w, h),
		}
	}
	return nil
}
