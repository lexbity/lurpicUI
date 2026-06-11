package render

import (
	"errors"
	"math"
	"sync"
)

type softwareTexture struct {
	pixels []byte
	width  uint16
	height uint16
	mips   uint8
}

// SoftwareBackend implements TextureBackend for CPU-side texture storage.
type SoftwareBackend struct {
	mu      sync.RWMutex
	pool    []*softwareTexture
	freeIDs []TextureID
}

var _ TextureBackend = (*SoftwareBackend)(nil)

func (b *SoftwareBackend) UploadTexture(req TextureUploadRequest) (TextureID, error) {
	if req.Width == 0 || req.Height == 0 {
		return 0, errors.New("software backend: invalid texture dimensions")
	}
	if len(req.PixelData) == 0 {
		return 0, errors.New("software backend: empty pixel data")
	}
	tex := &softwareTexture{
		pixels: append([]byte(nil), req.PixelData...),
		width:  req.Width,
		height: req.Height,
		mips:   req.MipLevels,
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	var id TextureID
	if n := len(b.freeIDs); n > 0 {
		id = b.freeIDs[n-1]
		b.freeIDs = b.freeIDs[:n-1]
		b.pool[id] = tex
	} else {
		id = TextureID(len(b.pool))
		b.pool = append(b.pool, tex)
	}

	if req.ResultCh != nil {
		req.ResultCh <- TextureUploadResult{AssetID: req.AssetID, TextureID: id, LOD: 0}
	}
	return id, nil
}

func (b *SoftwareBackend) FreeTexture(id TextureID) {
	b.mu.Lock()
	defer b.mu.Unlock()
	idx := int(id) //nolint:gosec // integer overflow conversion
	if idx < 0 || idx >= len(b.pool) || b.pool[idx] == nil {
		return
	}
	b.pool[idx] = nil
	b.freeIDs = append(b.freeIDs, id)
}

func (b *SoftwareBackend) UploadBudgetBytesPerFrame() int { return math.MaxInt }
func (b *SoftwareBackend) TranscodeTarget() TextureFormat { return TextureFormatRGBA8 }

func (b *SoftwareBackend) GetTexture(id TextureID) ([]byte, uint16, uint16, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	idx := int(id) //nolint:gosec // integer overflow conversion
	if idx < 0 || idx >= len(b.pool) || b.pool[idx] == nil {
		return nil, 0, 0, false
	}
	t := b.pool[idx]
	return t.pixels, t.width, t.height, true
}

func (b *SoftwareBackend) textureCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	count := 0
	for _, tex := range b.pool {
		if tex != nil {
			count++
		}
	}
	return count
}
