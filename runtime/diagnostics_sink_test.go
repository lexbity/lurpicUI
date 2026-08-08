package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// recordingDirtySink implements DiagnosticsHook and opts into DirtySnapshotSink
// to record every frame's snapshot.
type recordingDirtySink struct {
	snapshots []DirtySnapshot
}

func (s *recordingDirtySink) OnFrame(diagnostics.FrameStats) {}

func (s *recordingDirtySink) OnDirtySnapshot(snap DirtySnapshot) {
	s.snapshots = append(s.snapshots, snap)
}

// plainDiagHook implements only DiagnosticsHook — it must be skipped cleanly
// by the sink type assertion.
type plainDiagHook struct{ frames int }

func (h *plainDiagHook) OnFrame(diagnostics.FrameStats) { h.frames++ }

// sinkDirtyFacet is a minimal store-driven facet that invalidates with a
// recorded source tag when its store changes.
type sinkDirtyFacet struct {
	facet.Facet
	layout facet.LayoutRole
	proj   facet.ProjectionRole
}

func newSinkDirtyFacet() (*sinkDirtyFacet, *store.ValueStore[string]) {
	f := &sinkDirtyFacet{}
	f.Facet = facet.NewFacet()
	s := store.NewValueStore("a")
	f.layout = facet.LayoutRole{
		OnMeasure: func(_ facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return facet.MeasureResult{Size: c.MaxSize}
		},
		OnArrange: func(_ facet.ArrangeContext, bounds gfx.Rect) { f.layout.ArrangedBounds = bounds },
	}
	f.proj = facet.ProjectionRole{
		OnProject: func(_ facet.ProjectionContext) *gfx.CommandList { return &gfx.CommandList{} },
	}
	f.AddRole(&f.layout)
	f.AddRole(&f.proj)
	facet.Store(facet.Subscribe(f), &s.OnChange, s.Version, func(signal.Change[string]) {
		f.InvalidateWithSource(facet.DirtyProjection, "sink-test")
	})
	return f, s
}

func (f *sinkDirtyFacet) Base() *facet.Facet { f.BindImpl(f); return &f.Facet }

// TestRuntime_DirtySnapshotSinkFiresOncePerFrame asserts the sink receives one
// snapshot per frame at the dirty-set snapshot point, with the Dirty set and
// the per-facet source matching the runtime's own dirty bookkeeping.
func TestRuntime_DirtySnapshotSinkFiresOncePerFrame(t *testing.T) {
	f, s := newSinkDirtyFacet()
	rt := mustRuntimeTree(t, f)
	sink := &recordingDirtySink{}
	rt.EnableDiagnostics(sink)

	rt.RunOneFrame() // initial attach/project frame
	rt.RunOneFrame() // quiescent frame (possibly empty snapshot)

	s.Set("b")
	rt.RunOneFrame() // the store change dirties the facet

	if got := len(sink.snapshots); got != 3 {
		t.Fatalf("OnDirtySnapshot fired %d times, want 3 (one per frame)", got)
	}

	last := sink.snapshots[len(sink.snapshots)-1]
	id := f.Base().ID()
	flags, ok := last.Dirty[id]
	if !ok || flags&facet.DirtyProjection == 0 {
		t.Fatalf("last snapshot missing the store-dirtied facet: %v", last.Dirty)
	}
	if src := last.Sources[id]; src != "sink-test" {
		t.Fatalf("dirty source = %q, want sink-test", src)
	}
	if last.FrameNumber != rt.frameNumber {
		t.Fatalf("snapshot frame %d != runtime frame %d", last.FrameNumber, rt.frameNumber)
	}
	if sink.snapshots[1].FrameNumber <= sink.snapshots[0].FrameNumber {
		t.Fatal("frame numbers did not advance between snapshots")
	}
}

// TestRuntime_DirtySnapshotSinkNilDirtySet asserts a quiescent frame delivers a
// snapshot with nil Dirty/Sources (not an empty-map allocation per frame).
func TestRuntime_DirtySnapshotSinkNilDirtySet(t *testing.T) {
	root := &facet.Facet{}
	root.BindImpl(root)
	rt := mustRuntimeTree(t, root)
	sink := &recordingDirtySink{}
	rt.EnableDiagnostics(sink)

	rt.RunOneFrame()
	rt.RunOneFrame()

	if got := len(sink.snapshots); got != 2 {
		t.Fatalf("snapshots = %d, want 2", got)
	}
	last := sink.snapshots[len(sink.snapshots)-1]
	if last.Dirty != nil || last.Sources != nil {
		t.Fatalf("quiescent snapshot has non-nil dirty bookkeeping: %+v", last)
	}
}

// TestRuntime_DirtySnapshotSinkSkipsNonSinkHook asserts a DiagnosticsHook that
// does not implement DirtySnapshotSink is skipped cleanly by the type
// assertion (the interface is not widened).
func TestRuntime_DirtySnapshotSinkSkipsNonSinkHook(t *testing.T) {
	root := &facet.Facet{}
	root.BindImpl(root)
	rt := mustRuntimeTree(t, root)
	hook := &plainDiagHook{}
	rt.EnableDiagnostics(hook)

	rt.RunOneFrame()
	rt.RunOneFrame()

	if hook.frames != 2 {
		t.Fatalf("OnFrame fired %d times, want 2 (hook still receives stats)", hook.frames)
	}
}
