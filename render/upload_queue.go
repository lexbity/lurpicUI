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
			if req.ResultCh != nil {
				_ = id
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
