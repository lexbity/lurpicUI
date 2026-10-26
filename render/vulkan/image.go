package vulkan

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/gfx"
)

type imageAsset struct {
	handle uint64
}

type imageCache struct {
	mu     sync.Mutex
	assets map[[32]byte]imageAsset
}

func newImageCache() *imageCache {
	return &imageCache{assets: make(map[[32]byte]imageAsset)}
}

func (c *imageCache) ensureImage(img gfx.ImageHandle) (uint64, error) {
	if c == nil || img.IsZero() {
		return 0, nil
	}
	key := img.ContentDigest()
	c.mu.Lock()
	defer c.mu.Unlock()
	if asset, ok := c.assets[key]; ok {
		return asset.handle, nil
	}
	handle, err := createImageAsset(img)
	if err != nil {
		return 0, err
	}
	c.assets[key] = imageAsset{handle: handle}
	return handle, nil
}

func (c *imageCache) destroyAll() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, asset := range c.assets {
		_ = destroyImageAsset(asset.handle)
		delete(c.assets, key)
	}
}

func createImageAsset(img gfx.ImageHandle) (uint64, error) {
	if img.IsZero() {
		return 0, nil
	}
	pixels := img.CopyPixels()
	defer clear(pixels)
	return UploadImage(pixels, img.Width(), img.Height(), img.Stride(), 0)
}

func destroyImageAsset(handle uint64) error {
	if handle == 0 {
		return nil
	}
	return DestroyImage(handle)
}
