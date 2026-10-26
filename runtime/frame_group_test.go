package runtime

import (
	"bytes"
	"image/png"
	"sync"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testframe"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/software"
)

func TestCaptureFreezesSoftwarePixelsBeforeEncoding(t *testing.T) {
	surface := newPipelineSurface(8, 8)
	backend := software.NewSoftwareRenderer()
	if err := backend.Initialize(surface); err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	frame := testframe.Seal(render.FrameInput{Batches: []render.BatchInput{{Bounds: gfx.RectFromXYWH(0, 0, 8, 8), Opacity: 1, Commands: gfx.CommandList{Commands: []gfx.Command{gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 8, 8), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})}}}}}})
	defer frame.Release()
	if err := backend.Submit(frame); err != nil {
		t.Fatal(err)
	}
	if err := surface.Lock(); err != nil {
		t.Fatal(err)
	}
	frozen, err := copySurfaceImage(surface.Buffer(), surface.Stride(), 8, 8)
	_ = surface.Unlock(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := surface.Lock(); err != nil {
		t.Fatal(err)
	}
	clear(surface.Buffer())
	_ = surface.Unlock(nil)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, frozen); err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(&encoded)
	if err != nil {
		t.Fatal(err)
	}
	red, _, _, alpha := decoded.At(0, 0).RGBA()
	if red != 65535 || alpha != 65535 {
		t.Fatal("asynchronous encoding observed later surface mutation")
	}
	if _, err := copySurfaceImage(nil, 1, 16384, 16384); err == nil {
		t.Fatal("invalid capture backing accepted")
	}
}

func (s *pipelineSurface) Scale() float32 { return 1 }

type packetWindow struct {
	testWindow
	pixels *pipelineSurface
}

func (w *packetWindow) Surface() platform.Surface { return w.pixels }

type windowBarrierBackend struct {
	*software.SoftwareRenderer
	started chan struct{}
	release chan struct{}
}

func (b *windowBarrierBackend) SubmitWindow(id string, frame *render.Frame) error {
	if id == "tools" {
		b.started <- struct{}{}
		<-b.release
	}
	return b.SoftwareRenderer.SubmitWindow(id, frame)
}

func TestMultiwindowReadySetExact(t *testing.T) {
	primary, tools := newPipelineSurface(8, 8), newPipelineSurface(8, 8)
	backend := &windowBarrierBackend{SoftwareRenderer: software.NewSoftwareRenderer(), started: make(chan struct{}, 1), release: make(chan struct{})}
	if err := backend.Initialize(primary); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t)
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.WindowBindings = map[string]platform.Window{"tools": &packetWindow{pixels: tools}}
	root := facet.NewFacet()
	rt, err := New(cfg, nil, &packetWindow{pixels: primary}, backend, &root)
	if err != nil {
		t.Fatal(err)
	}
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(backend.release) }) }
	t.Cleanup(func() { unblock(); rt.Shutdown() })
	rt.renderPipeline.start()
	input := func(id string, color gfx.Color) render.FrameInput {
		return render.FrameInput{WindowID: id, SurfaceGeneration: 0, Batches: []render.BatchInput{{ID: 1, Bounds: gfx.RectFromXYWH(0, 0, 8, 8), Opacity: 1, Commands: gfx.CommandList{Commands: []gfx.Command{gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 8, 8), Brush: gfx.SolidBrush(color)}}}}}}
	}
	group, err := rt.sealFrameGroup(map[string]render.FrameInput{render.PrimaryWindow: input(render.PrimaryWindow, gfx.Color{R: 1, A: 1}), "tools": input("tools", gfx.Color{B: 1, A: 1})}, false)
	if err != nil {
		t.Fatal(err)
	}
	view, err := group.Retain()
	if err != nil {
		t.Fatal(err)
	}
	rt.pendingFrameViews[1] = view
	if !rt.renderPipeline.TryReserveFrame() {
		t.Fatal("frame credit unavailable")
	}
	rt.renderPipeline.SubmitReserved(group, 1)
	select {
	case <-backend.started:
	case <-time.After(5 * time.Second):
		t.Fatal("named window renderer did not start")
	}
	if primary.Buffer()[0] != 255 {
		t.Fatal("primary window was not actually rendered")
	}
	select {
	case completion := <-rt.renderPipeline.Completions():
		t.Fatalf("partial window set reported completion: %+v", completion)
	default:
	}
	if err := rt.RunControlTurn(); err != nil {
		t.Fatal(err)
	}
	if frames, err := rt.RetainWindowFrames(); err != nil || len(frames) != 0 {
		t.Fatalf("partial window set became ready: frames=%d err=%v", len(frames), err)
	}
	unblock()
	var completion RenderCompletion
	select {
	case completion = <-rt.renderPipeline.Completions():
	case <-time.After(5 * time.Second):
		t.Fatal("window set did not complete")
	}
	if !completion.Ready || completion.Err != nil || completion.WindowCount != 2 {
		t.Fatalf("window set completion=%+v", completion)
	}
	rt.driveHandleRenderCompletion(completion)
	frames, err := rt.RetainWindowFrames()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, frame := range frames {
			_ = frame.Release()
		}
	}()
	if len(frames) != 2 || frames[render.PrimaryWindow] == nil || frames["tools"] == nil {
		t.Fatalf("ready window set is incomplete: %v", frames)
	}
	if tools.Buffer()[2] != 255 {
		t.Fatal("named window ready acknowledgment preceded its actual pixels")
	}
	rt.driveHandleRenderCompletion(completion) // duplicate must not release the retained generations.
	if frames["tools"].BatchCount() != 1 {
		t.Fatal("duplicate acknowledgment released named-window storage")
	}
}

func TestFrameGroupAdmissionUsesAggregatePacketCap(t *testing.T) {
	bytes, err := render.EstimateFrameBytes(render.FrameInput{WindowID: render.PrimaryWindow})
	if err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{frameStorage: testFrameStorage(t, 1<<20, bytes+128)}
	if _, err := rt.sealFrameGroup(map[string]render.FrameInput{render.PrimaryWindow: {}, "tools": {}}, false); err == nil {
		t.Fatal("per-window packets bypassed aggregate cap")
	}
	_, reserved, live, retiring := rt.frameStorage.wallet.Snapshot()
	if reserved+live+retiring != 0 {
		t.Fatalf("partial multiwindow admission retained %d bytes", reserved+live+retiring)
	}
}

func TestSurfaceLossInvalidatesDelayedReadySet(t *testing.T) {
	rt := &Runtime{frameStorage: testFrameStorage(t, 8<<20, 1<<20), pendingFrameViews: make(map[uint64]*render.FrameGroup), lastWindowFrames: make(map[string]*render.Frame)}
	group, err := rt.sealFrameGroup(map[string]render.FrameInput{render.PrimaryWindow: {SurfaceGeneration: 3}}, false)
	if err != nil {
		t.Fatal(err)
	}
	rt.pendingFrameViews[1] = group
	rt.surfaceLifecycleGeneration = 4
	descriptor, _ := group.WindowAt(0)
	completion, accepted := rt.completeFrameView(RenderCompletion{Frame: 1, ResourceGeneration: group.Generation(), Ready: true, WindowCount: 1, Windows: [render.MaxFrameWindows]WindowCompletion{{WindowDescriptor: descriptor, Ready: true}}})
	if !accepted || completion.Ready || completion.Err == nil || len(rt.lastWindowFrames) != 0 {
		t.Fatalf("ended surface reported ready: %+v accepted=%v", completion, accepted)
	}
	_, reserved, live, retiring := rt.frameStorage.wallet.Snapshot()
	if reserved+live+retiring != 0 {
		t.Fatal("invalidated group retained charges")
	}
}
