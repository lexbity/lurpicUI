package theme

import (
	"image"
	"math/rand"
	"sync"
)

// TextureData represents a loaded or procedurally generated texture.
type TextureData struct {
	Image   *image.RGBA
	Name    string
	IsNoise bool
}

var (
	registryMu sync.RWMutex
	textures              = make(map[TextureRef]TextureData)
	nextRef    TextureRef = 1
)

// RegisterTexture registers a new image asset and returns its unique handle.
func RegisterTexture(img *image.RGBA, name string) TextureRef {
	registryMu.Lock()
	defer registryMu.Unlock()
	ref := nextRef
	nextRef++
	textures[ref] = TextureData{
		Image: img,
		Name:  name,
	}
	return ref
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
		data := TextureData{Image: img, Name: "brushed_metal"}
		textures[ref] = data
		return data, true
	case TextureMicroNoise:
		img := generateMicroNoise()
		data := TextureData{Image: img, Name: "micro_noise", IsNoise: true}
		textures[ref] = data
		return data, true
	default:
		return TextureData{}, false
	}
}

func generateBrushedMetal() *image.RGBA {
	w, h := 256, 256
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewSource(42))
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
	r := rand.New(rand.NewSource(1337))
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
