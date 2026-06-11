package runtime

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/render"
)

type frameHandoff struct {
	frame  *render.Frame
	doneCh chan struct{}
}

// RenderPipeline hands frames to the render backend with capacity-1 backpressure.
type RenderPipeline struct {
	backend     render.Backend
	uploadQueue *render.UploadQueue
	handoffCh   chan frameHandoff
	fatalCh     chan error
	destroyOnce sync.Once
}

type renderThread struct {
	pipeline *RenderPipeline
}

func newRenderPipeline(backend render.Backend) *RenderPipeline {
	p := &RenderPipeline{
		backend:   backend,
		handoffCh: make(chan frameHandoff, 1),
		fatalCh:   make(chan error, 1),
	}
	if tb, ok := backend.(render.TextureBackend); ok {
		p.uploadQueue = render.NewUploadQueue(tb, 1024)
	}
	return p
}

// Submit sends a frame to the render backend pipeline.
func (p *RenderPipeline) Submit(frame *render.Frame) {
	if p.backend == nil {
		return
	}
	p.handoffCh <- frameHandoff{frame: frame}
}

// SubmitAndWait hands a frame to the renderer and waits for completion.
func (p *RenderPipeline) SubmitAndWait(frame *render.Frame) {
	if p.backend == nil {
		return
	}
	done := make(chan struct{})
	p.handoffCh <- frameHandoff{frame: frame, doneCh: done}
	<-done
}

func (rt *renderThread) run() {

	for handoff := range rt.pipeline.handoffCh {
		if rt.pipeline.backend == nil || handoff.frame == nil {
			if handoff.doneCh != nil {
				close(handoff.doneCh)
			}
			continue
		}
		if rt.pipeline.uploadQueue != nil {
			rt.pipeline.uploadQueue.DrainBudget()
		}
		if err := rt.pipeline.backend.Submit(handoff.frame); err != nil {
			select {
			case rt.pipeline.fatalCh <- err:
			default:
			}
		}
		if handoff.doneCh != nil {
			close(handoff.doneCh)
		}
	}
}

func (p *RenderPipeline) destroy() {

	p.destroyOnce.Do(func() {
		close(p.handoffCh)
	})
}

// UploadQueue exposes the pipeline's upload queue for wiring into the
// asset manager's uploader bridge. Returns nil when the backend does not
// support textures or the pipeline has not been fully initialised.
func (p *RenderPipeline) UploadQueue() *render.UploadQueue {
	if p == nil {
		return nil
	}
	return p.uploadQueue
}
