package assets

// TextureUploadRequest describes pixels to upload to the GPU.
// The consumer (render backend) reads Pixels, Width, Height, MipLevels,
// and Format to produce a GPU texture handle.
type TextureUploadRequest struct {
	AssetID   AssetID
	LOD       int
	Pixels    []byte
	Width     int
	Height    int
	MipLevels int
	Format    uint32
}

// TextureUploadResult carries the GPU texture handle for a completed upload.
type TextureUploadResult struct {
	AssetID   AssetID
	LOD       int
	TextureID TextureID
	GPUBytes  int64
	OK        bool
}

// TextureUploader is the seam between the asset manager and the render
// backend's GPU upload queue. Implemented by the runtime over the existing
// render.UploadQueue. A nil uploader or one returning Budget() == 0 means
// the backend is not GPU-capable; the manager stays CPU-only.
type TextureUploader interface {
	Enqueue(req TextureUploadRequest) bool
	Results() <-chan TextureUploadResult
	Budget() int
}
