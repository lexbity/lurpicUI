package runtime

import (
	"errors"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/software"
)

func TestCleanupPacketCapacitySurvivesPinnedOrdinaryPackets(t *testing.T) {
	input := render.FrameInput{WindowID: render.PrimaryWindow, Batches: []render.BatchInput{{
		Bounds: gfx.RectFromXYWH(0, 0, 8, 8), Opacity: 1,
		Commands: gfx.CommandList{Commands: []gfx.Command{gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 8, 8), Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1})}}},
	}}}
	packetBytes, err := render.EstimateFrameBytes(input)
	if err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{frameStorage: testFrameStorage(t, 8*packetBytes, packetBytes)}
	packets := map[string]render.FrameInput{render.PrimaryWindow: input}
	var pinned []*render.Frame
	t.Cleanup(func() {
		for _, frame := range pinned {
			_ = frame.Release()
		}
	})
	for i := 0; i < 3; i++ {
		group, err := rt.sealFrameGroup(packets, false)
		if err != nil {
			t.Fatal(err)
		}
		frame, err := group.RetainWindow(0)
		if err != nil {
			t.Fatal(err)
		}
		pinned = append(pinned, frame)
		if err := group.Release(); err != nil {
			t.Fatal(err)
		}
	}
	_, err = rt.sealFrameGroup(packets, false)
	var limit *membudget.ResourceLimit
	if !errors.As(err, &limit) {
		t.Fatalf("ordinary packet exhaustion = %v", err)
	}
	cleanup, err := rt.sealFrameGroup(packets, true)
	if err != nil {
		t.Fatal("pinned ordinary pages consumed cleanup backing", err)
	}
	for _, frame := range pinned {
		if cleanup.Generation() == frame.Generation() {
			t.Fatal("cleanup reused a pinned generation")
		}
	}
	backend := software.NewSoftwareRenderer()
	surface := newPipelineSurface(8, 8)
	if err := backend.Initialize(surface); err != nil {
		t.Fatal(err)
	}
	pipeline := newRenderPipeline(backend)
	pipeline.start()
	t.Cleanup(func() {
		if !pipeline.shutdown(5 * time.Second) {
			t.Error("render worker did not stop")
		}
	})
	if !pipeline.TryReserveCleanupFrame() {
		t.Fatal("cleanup credit unavailable")
	}
	if err := pipeline.SubmitCleanupReserved(cleanup, 1); err != nil {
		t.Fatal(err)
	}
	select {
	case completion := <-pipeline.Completions():
		if !completion.Ready || completion.Err != nil {
			t.Fatalf("cleanup completion = %+v", completion)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not render")
	}
	if surface.Buffer()[1] != 255 {
		t.Fatal("cleanup did not produce the required ready pixels")
	}
	_, reserved, live, retiring := rt.frameStorage.cleanup.Snapshot()
	if reserved+live+retiring != 0 {
		t.Fatal("cleanup charge survived its last acknowledgment")
	}
	for _, frame := range pinned {
		if frame.BatchCount() != 1 {
			t.Fatal("cleanup released a pinned ordinary generation")
		}
	}
}

func TestResourcePolicyRejectsMissingCleanupCapacity(t *testing.T) {
	for _, profile := range []membudget.Profile{membudget.Compact, membudget.Standard} {
		policy := DefaultResourcePolicy(profile)
		policy.Plan.Caps[membudget.Frame] = 4*framePacketCapacity(profile) + frameSealScratchBytes - 1
		if err := policy.Validate(); err == nil {
			t.Fatal("startup accepted an incomplete packet/scratch partition")
		}
	}
}
