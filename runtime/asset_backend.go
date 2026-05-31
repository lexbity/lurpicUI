package runtime

import (
	"encoding/binary"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/render"
)

// assetTextureReleaser adapts a render.TextureBackend to the
// assets.TextureReleaser interface so the asset cache can free
// GPU textures through the real backend. The bridge is a pure
// cast: both types are uint64 aliases.
type assetTextureReleaser struct {
	backend render.TextureBackend
}

func (a *assetTextureReleaser) FreeTexture(id assets.TextureID) {
	if a == nil || a.backend == nil {
		return
	}
	a.backend.FreeTexture(render.TextureID(id))
}

// assetUploader adapts a render.UploadQueue to the assets.TextureUploader
// interface. It converts between the asset and render type spaces (AssetID,
// TextureID, pixel format) and forwards results through a conversion channel.
type assetUploader struct {
	queue   *render.UploadQueue
	results chan assets.TextureUploadResult
	done    chan struct{}
}

func newAssetUploader(queue *render.UploadQueue) *assetUploader {
	u := &assetUploader{
		queue:   queue,
		results: make(chan assets.TextureUploadResult, 32),
		done:    make(chan struct{}),
	}
	go u.forwardResults()
	return u
}

func (u *assetUploader) Enqueue(req assets.TextureUploadRequest) bool {
	if u == nil || u.queue == nil {
		return false
	}
	// Convert assets.AssetID ([16]byte) → uint64 via the first 8 bytes.
	assetID := binary.LittleEndian.Uint64(req.AssetID[:8])
	rreq := render.TextureUploadRequest{
		AssetID:   assetID,
		LOD:       req.LOD,
		PixelData: req.Pixels,
		Width:     uint16(req.Width),
		Height:    uint16(req.Height),
		MipLevels: uint8(req.MipLevels),
		// ResultCh: nil → defaults to the upload queue's shared results channel.
	}
	return u.queue.Enqueue(rreq)
}

func (u *assetUploader) Results() <-chan assets.TextureUploadResult {
	if u == nil {
		return nil
	}
	return u.results
}

func (u *assetUploader) Budget() int {
	if u == nil || u.queue == nil {
		return 0
	}
	return u.queue.Budget()
}

func (u *assetUploader) Close() {
	close(u.done)
}

func (u *assetUploader) forwardResults() {
	for {
		select {
		case r, ok := <-u.queue.Results():
			if !ok {
				close(u.results)
				return
			}
			var id assets.AssetID
			binary.LittleEndian.PutUint64(id[:8], r.AssetID)
			select {
			case u.results <- assets.TextureUploadResult{
				AssetID:   id,
				LOD:       r.LOD,
				TextureID: assets.TextureID(r.TextureID),
				OK:        r.Err == nil,
			}:
			case <-u.done:
				return
			}
		case <-u.done:
			return
		}
	}
}
