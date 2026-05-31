package gfx

import "image"

// DrawableKind identifies which representation a resolved asset uses.
type DrawableKind uint8

const (
	DrawableNone DrawableKind = iota
	DrawableGPUTexture
	DrawableCPUBitmap
	DrawableVector
)

// DrawableRef is a backend-neutral representation of a resolved managed asset.
// A mark feeds this into its existing paint/command emission.
type DrawableRef struct {
	Kind      DrawableKind
	TextureID uint64      // when GPUTexture (render.TextureID as uint64)
	Bitmap    *image.RGBA // when CPUBitmap
	Vector    any         // when Vector (decoded SVG doc)
	LOD       int
	SrcBox    Rect
	Key       string
}
