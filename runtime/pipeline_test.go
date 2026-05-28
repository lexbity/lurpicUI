package runtime

import (
	"testing"
)

func TestRenderPipeline_destroy_is_idempotent(t *testing.T) {
	p := newRenderPipeline(nil)
	p.destroy()
	p.destroy()
}
