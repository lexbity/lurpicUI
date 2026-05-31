package render

// UploadQueue manages pending texture uploads across frames.
type UploadQueue struct {
	pending chan TextureUploadRequest
	backend TextureBackend
	results chan TextureUploadResult
}

// NewUploadQueue returns a bounded FIFO upload queue.
func NewUploadQueue(backend TextureBackend, depth int) *UploadQueue {
	if depth <= 0 {
		depth = 1
	}
	return &UploadQueue{
		pending: make(chan TextureUploadRequest, depth),
		backend: backend,
		results: make(chan TextureUploadResult, depth),
	}
}

// Enqueue adds a pending upload. Returns false if the queue is full.
func (q *UploadQueue) Enqueue(req TextureUploadRequest) bool {
	if q == nil {
		return false
	}
	select {
	case q.pending <- req:
		return true
	default:
		return false
	}
}

// DrainBudget processes uploads up to the per-frame byte budget.
func (q *UploadQueue) DrainBudget() {
	if q == nil || q.backend == nil {
		return
	}
	budget := q.backend.UploadBudgetBytesPerFrame()
	if budget <= 0 {
		return
	}

	var used int
	var deferred []TextureUploadRequest
	for used < budget {
		select {
		case req := <-q.pending:
			cost := len(req.PixelData)
			if used+cost > budget {
				deferred = append(deferred, req)
				used = budget
				continue
			}
			if cost > 0 {
				used += cost
			}
			if req.ResultCh == nil {
				req.ResultCh = q.results
			}
			id, err := q.backend.UploadTexture(req)
			if err != nil {
				if req.ResultCh != nil {
					req.ResultCh <- TextureUploadResult{AssetID: req.AssetID, Err: err}
				}
				continue
			}
			// Send success result with the computed GPU byte footprint.
			if req.ResultCh != nil {
				gpuBytes := uploadGPUBytes(req)
				req.ResultCh <- TextureUploadResult{
					AssetID:   req.AssetID,
					TextureID: id,
					LOD:       req.LOD,
					GPUBytes:  gpuBytes,
				}
			}
		default:
			for i := len(deferred) - 1; i >= 0; i-- {
				q.Enqueue(deferred[i])
			}
			return
		}
	}
	for i := len(deferred) - 1; i >= 0; i-- {
		q.Enqueue(deferred[i])
	}
}

// uploadGPUBytes computes the equivalent GPU memory footprint for an upload
// request based on its dimensions and target format. This is used for GPU
// budget accounting, not an exact transcode result.
func uploadGPUBytes(req TextureUploadRequest) int64 {
	w := int64(req.Width)
	h := int64(req.Height)
	if w <= 0 || h <= 0 {
		return int64(len(req.PixelData))
	}

	switch req.Format {
	case TextureFormatASTC4x4:
		// 8 bytes per 4x4 block, round up to block boundary.
		bw := (w + 3) / 4 * 4
		bh := (h + 3) / 4 * 4
		return (bw * bh / 16) * 8
	case TextureFormatBC7:
		bw := (w + 3) / 4 * 4
		bh := (h + 3) / 4 * 4
		return (bw * bh / 16) * 16
	default:
		return w * h * 4
	}
}

// TargetFormat returns the backend's preferred transcode target format.
func (q *UploadQueue) TargetFormat() TextureFormat {
	if q == nil || q.backend == nil {
		return TextureFormatRGBA8
	}
	return q.backend.TranscodeTarget()
}

// Budget returns the per-frame upload budget in bytes from the backend.
// 0 means the backend is not GPU-capable or the queue is nil.
func (q *UploadQueue) Budget() int {
	if q == nil || q.backend == nil {
		return 0
	}
	return q.backend.UploadBudgetBytesPerFrame()
}

// Results exposes the queue's fallback result channel.
func (q *UploadQueue) Results() <-chan TextureUploadResult {
	if q == nil {
		return nil
	}
	return q.results
}
