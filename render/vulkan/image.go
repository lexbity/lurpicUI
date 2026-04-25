package vulkan

import (
	"image"
	"sync"
)

type imageAsset struct {
	handle uint64
	hash   uint64
}

type imageCache struct {
	mu     sync.Mutex
	assets map[uint64]imageAsset
}

func newImageCache() *imageCache {
	return &imageCache{assets: make(map[uint64]imageAsset)}
}

func (c *imageCache) ensureImage(img *image.RGBA) (uint64, error) {
	if c == nil || img == nil {
		return 0, nil
	}
	key := hashImage(img)
	c.mu.Lock()
	defer c.mu.Unlock()
	if asset, ok := c.assets[key]; ok {
		return asset.handle, nil
	}
	handle, err := createImageAsset(img)
	if err != nil {
		return 0, err
	}
	c.assets[key] = imageAsset{handle: handle, hash: key}
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

func createImageAsset(img *image.RGBA) (uint64, error) {
	if img == nil {
		return 0, nil
	}
	return UploadImage(
		img.Pix,
		img.Bounds().Dx(),
		img.Bounds().Dy(),
		img.Stride,
		0,
	)
}

func destroyImageAsset(handle uint64) error {
	if handle == 0 {
		return nil
	}
	return DestroyImage(handle)
}
