package runtime

import (
	"errors"
	"image/color"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
)

func TestRuntimeNew_validation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FontRegistry = nil
	if _, err := New(cfg, nil, nil, &stubBackend{}, &facet.Facet{}); err == nil {
		t.Fatal("expected error for nil font registry")
	}
	cfg = DefaultConfig()
	cfg.TargetFPS = 0
	if _, err := New(cfg, nil, nil, &stubBackend{}, &facet.Facet{}); err == nil {
		t.Fatal("expected error for zero target fps")
	}
	cfg = DefaultConfig()
	if _, err := New(cfg, nil, nil, &stubBackend{}, nil); err == nil {
		t.Fatal("expected error for nil root")
	}
}

func TestFrameTimer_basics(t *testing.T) {
	timer := NewFrameTimer(60)
	timer.RequestFrame()
	before := time.Now()
	_ = timer.Wait()
	if time.Since(before) > 20*time.Millisecond {
		t.Fatal("expected immediate wake")
	}
}

func TestRenderPipeline_submit_blocks_on_full(t *testing.T) {
	pipe := newRenderPipeline(&stubBackend{})
	pipe.Submit(&render.Frame{})
	done := make(chan struct{})
	go func() {
		pipe.Submit(&render.Frame{})
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("second submit should block")
	case <-time.After(20 * time.Millisecond):
	}
	<-pipe.handoffCh
	select {
	case <-done:
	case <-time.After(20 * time.Millisecond):
		t.Fatal("expected second submit to unblock after drain")
	}
}

func TestRenderPipeline_fatalch_readable(t *testing.T) {
	pipe := newRenderPipeline(&stubBackend{})
	err := errors.New("boom")
	pipe.fatalCh <- err
	select {
	case got := <-pipe.fatalCh:
		if got == nil || got.Error() != "boom" {
			t.Fatalf("got %v", got)
		}
	default:
		t.Fatal("expected readable fatal channel")
	}
}

func TestRuntime_assembleFrame_prepends_transform(t *testing.T) {
	rt := mustRuntime(t)
	output := &projection.FrameOutput{
		RenderBatchs: []projection.RenderBatchOutput{
			{
				FacetID:   1,
				Bounds:    gfx.RectFromXYWH(0, 0, 10, 10),
				Transform: gfx.Translation(12, 18),
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(255, 0, 0, 255))},
				}},
			},
		},
	}
	frame := rt.assembleFrame(output, map[facet.FacetID]facet.DirtyFlags{1: facet.DirtyAll})
	if frame == nil || len(frame.RenderBatchs) != 1 {
		t.Fatalf("frame = %#v", frame)
	}
	cmds := frame.RenderBatchs[0].Commands.Commands
	if len(cmds) < 3 {
		t.Fatalf("commands = %#v", cmds)
	}
	if _, ok := cmds[0].(gfx.PushTransform); !ok {
		t.Fatalf("first command = %T", cmds[0])
	}
	if _, ok := cmds[len(cmds)-1].(gfx.PopTransform); !ok {
		t.Fatalf("last command = %T", cmds[len(cmds)-1])
	}
	if frame.RenderBatchs[0].CommandHash == 0 {
		t.Fatal("expected command hash")
	}
}

func TestRuntime_assembleFrame_dirty_regions(t *testing.T) {
	rt := mustRuntime(t)
	output := &projection.FrameOutput{
		RenderBatchs: []projection.RenderBatchOutput{
			{FacetID: 1, Bounds: gfx.RectFromXYWH(0, 0, 10, 10)},
			{FacetID: 2, Bounds: gfx.RectFromXYWH(10, 10, 10, 10)},
		},
	}
	frame := rt.assembleFrame(output, map[facet.FacetID]facet.DirtyFlags{2: facet.DirtyProjection})
	if got := len(frame.DirtyRegions); got != 1 {
		t.Fatalf("dirty regions = %d, want 1", got)
	}
	if frame.DirtyRegions[0] != (gfx.RectFromXYWH(10, 10, 10, 10)) {
		t.Fatalf("dirty regions = %#v", frame.DirtyRegions)
	}
}

func TestRuntime_addfacet_visible_next_frame(t *testing.T) {
	root, child := newRuntimeRenderTree()
	backend := &recordingBackend{}
	rt := mustRuntimeWithBackend(t, root, backend)
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.RunOneFrame()
	before := rt.LastFrameStats().RenderBatchCount
	rt.AddFacet(root, child, layout.ChildAttachment{})
	rt.RunOneFrame()
	if got := rt.LastFrameStats().RenderBatchCount; got <= before {
		t.Fatalf("RenderBatch count = %d, before = %d", got, before)
	}
	if backend.last == nil || len(backend.last.RenderBatchs) != 2 {
		t.Fatalf("backend frame = %#v", backend.last)
	}
	rt.Shutdown()
}

func TestRuntime_run_returns_render_error(t *testing.T) {
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	rt := mustRuntimeWithBackend(t, root, &stubBackend{submitErr: errors.New("boom")})
	errCh := make(chan error, 1)
	go func() {
		errCh <- rt.Run()
	}()
	select {
	case err := <-errCh:
		if err == nil || err.Error() == "" {
			t.Fatal("expected render error")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for runtime error")
	}
}
