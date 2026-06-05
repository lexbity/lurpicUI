package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
)

func TestRenderPipeline_destroy_is_idempotent(t *testing.T) {
	fb := &fakeBackend{}
	p := newRenderPipeline(fb)

	p.destroy()
	if fb.destroyCount.Load() != 0 {
		t.Fatalf("backend Destroy called on pipeline destroy: count=%d", fb.destroyCount.Load())
	}

	p.destroy()
	if fb.destroyCount.Load() != 0 {
		t.Fatalf("backend Destroy called on double pipeline destroy: count=%d", fb.destroyCount.Load())
	}
}

func TestRenderPipeline_destroy_backend_destroyed_on_shutdown(t *testing.T) {
	fb := &fakeBackend{}
	root := &runtimeTestFacet{Facet: facet.NewFacet()}
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	rt, err := New(cfg, nil, nil, fb, root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rt.Shutdown()

	if fb.destroyCount.Load() != 1 {
		t.Fatalf("backend Destroy called %d times, want 1", fb.destroyCount.Load())
	}
}
