package theme

import (
	"errors"
	"image"
	"math/rand"
	"sync"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// TextureData represents a loaded or procedurally generated texture.
type TextureData struct {
	Image   gfx.ImageHandle
	Name    string
	IsNoise bool
}

var (
	registryMu sync.RWMutex
	textures              = make(map[TextureRef]TextureData)
	nextRef    TextureRef = 1
)

// RegisterTexture registers a new image asset and returns its unique handle.
func RegisterTexture(img *image.RGBA, name string) (TextureRef, error) {
	if img == nil {
		return 0, errors.New("theme: texture image is required")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	ref := nextRef
	if ref == 0 {
		return 0, errors.New("theme: texture identity space exhausted")
	}
	handle, err := gfx.NewImageHandle(img, uint64(ref))
	if err != nil {
		return 0, err
	}
	nextRef++
	textures[ref] = TextureData{
		Image: handle,
		Name:  name,
	}
	return ref, nil
}

// GetTexture retrieves a texture from the registry, falling back to procedural generators if necessary.
func GetTexture(ref TextureRef) (TextureData, bool) {
	registryMu.RLock()
	data, ok := textures[ref]
	registryMu.RUnlock()
	if ok {
		return data, true
	}
	return generateProcedural(ref)
}

// Procedural presets mapping
const (
	TextureBrushedMetal TextureRef = 1000
	TextureMicroNoise   TextureRef = 1001
)

func generateProcedural(ref TextureRef) (TextureData, bool) {
	registryMu.Lock()
	defer registryMu.Unlock()

	// Double-check if generated while waiting for the lock
	if data, ok := textures[ref]; ok {
		return data, true
	}

	switch ref {
	case TextureBrushedMetal:
		img := generateBrushedMetal()
		handle, err := gfx.NewImageHandle(img, uint64(ref))
		if err != nil {
			return TextureData{}, false
		}
		data := TextureData{Image: handle, Name: "brushed_metal"}
		textures[ref] = data
		return data, true
	case TextureMicroNoise:
		img := generateMicroNoise()
		handle, err := gfx.NewImageHandle(img, uint64(ref))
		if err != nil {
			return TextureData{}, false
		}
		data := TextureData{Image: handle, Name: "micro_noise", IsNoise: true}
		textures[ref] = data
		return data, true
	default:
		return TextureData{}, false
	}
}

func generateBrushedMetal() *image.RGBA {
	w, h := 256, 256
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewSource(42)) //nolint:gosec // non-security random
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Brushed aluminum effect using horizontal micro-streaks
			noise := r.Float64() * 20
			v := uint8(220 + noise)
			idx := (y*w + x) * 4
			img.Pix[idx] = v
			img.Pix[idx+1] = v
			img.Pix[idx+2] = v
			img.Pix[idx+3] = 255
		}
	}
	return img
}

func generateMicroNoise() *image.RGBA {
	w, h := 128, 128
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewSource(1337)) //nolint:gosec // non-security random
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Fine tactile surface noise
			noise := r.Float64() * 15
			v := uint8(240 + noise)
			idx := (y*w + x) * 4
			img.Pix[idx] = v
			img.Pix[idx+1] = v
			img.Pix[idx+2] = v
			img.Pix[idx+3] = 255
		}
	}
	return img
}
