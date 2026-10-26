package gfx

import (
	"crypto/sha256"
	"errors"
	"image"
	"math"
)

// ImageHandle identifies immutable CPU pixels by content generation. The
// backing pixels are copied once when the source asset is registered and are
// never exposed as a mutable image.Image.
type ImageHandle struct {
	asset *imageAsset
}

type imageAsset struct {
	generation uint64
	width      int
	height     int
	stride     int
	pixels     []byte
	digest     [sha256.Size]byte
}

// NewImageHandle copies a validated RGBA image into immutable storage.
func NewImageHandle(src *image.RGBA, generation uint64) (ImageHandle, error) {
	if src == nil {
		return ImageHandle{}, errors.New("gfx: nil image source")
	}
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 || width > math.MaxInt/4 || src.Stride < width*4 {
		return ImageHandle{}, errors.New("gfx: invalid RGBA image dimensions or stride")
	}
	if height > math.MaxInt/src.Stride || len(src.Pix) < src.Stride*height {
		return ImageHandle{}, errors.New("gfx: RGBA image backing is truncated or overflows")
	}
	pixels := make([]byte, src.Stride*height)
	for y := 0; y < height; y++ {
		off := src.PixOffset(bounds.Min.X, bounds.Min.Y+y)
		if off < 0 || off > len(src.Pix)-width*4 {
			return ImageHandle{}, errors.New("gfx: RGBA image row lies outside backing")
		}
		copy(pixels[y*src.Stride:y*src.Stride+width*4], src.Pix[off:off+width*4])
	}
	return NewImageHandlePixels(pixels, width, height, src.Stride, generation)
}

// NewImageHandlePixels registers an already-owned pixel buffer by copying it.
func NewImageHandlePixels(pixels []byte, width, height, stride int, generation uint64) (ImageHandle, error) {
	if width <= 0 || height <= 0 || width > math.MaxInt/4 || stride < width*4 || height > math.MaxInt/stride || len(pixels) != height*stride {
		return ImageHandle{}, errors.New("gfx: invalid immutable pixel dimensions or backing")
	}
	owned := append([]byte(nil), pixels...)
	return ImageHandle{asset: &imageAsset{
		generation: generation,
		width:      width,
		height:     height,
		stride:     stride,
		pixels:     owned,
		digest:     sha256.Sum256(owned),
	}}, nil
}

func (h ImageHandle) IsZero() bool { return h.asset == nil }

func (h ImageHandle) Generation() uint64 {
	if h.asset == nil {
		return 0
	}
	return h.asset.generation
}

func (h ImageHandle) Width() int {
	if h.asset == nil {
		return 0
	}
	return h.asset.width
}

func (h ImageHandle) Height() int {
	if h.asset == nil {
		return 0
	}
	return h.asset.height
}

func (h ImageHandle) Stride() int {
	if h.asset == nil {
		return 0
	}
	return h.asset.stride
}

// PixelAt returns one immutable pixel in RGBA byte order.
func (h ImageHandle) PixelAt(x, y int) (r, g, b, a byte, ok bool) {
	if h.asset == nil || x < 0 || y < 0 || x >= h.asset.width || y >= h.asset.height {
		return 0, 0, 0, 0, false
	}
	off := y*h.asset.stride + x*4
	pixels := h.asset.pixels
	return pixels[off], pixels[off+1], pixels[off+2], pixels[off+3], true
}

// CopyPixels returns a detached copy suitable for an upload API that requires
// caller-owned writable bytes.
func (h ImageHandle) CopyPixels() []byte {
	if h.asset == nil {
		return nil
	}
	return append([]byte(nil), h.asset.pixels...)
}

func (h ImageHandle) ContentDigest() [sha256.Size]byte {
	if h.asset == nil {
		return [sha256.Size]byte{}
	}
	return h.asset.digest
}
