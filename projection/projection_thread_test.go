package projection

import (
	"sync/atomic"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestProjectionOutputPartitions_FlattenPreservesOrder(t *testing.T) {
	var partitions ProjectionOutputPartitions
	left := partitions.NewPartition()
	right := partitions.NewPartition()

	a := &ProjectionOutput{FacetID: 1, Bounds: gfx.RectFromXYWH(0, 0, 10, 10)}
	b := &ProjectionOutput{FacetID: 2, Bounds: gfx.RectFromXYWH(10, 0, 10, 10)}
	c := &ProjectionOutput{FacetID: 3, Bounds: gfx.RectFromXYWH(20, 0, 10, 10)}

	left.Append(a)
	left.Append(b)
	right.Append(c)

	got := partitions.Flatten()
	want := []*ProjectionOutput{a, b, c}

	if len(got) != len(want) {
		t.Fatalf("flatten len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("flatten[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestProjectionSystem_Run_collectsOutputsInPreOrder(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 80, 24))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(0, 0, 32, 24))
	root.Base().AddChild(&child.Facet)

	attachTree(root)

	sys := NewSystem()
	sys.SetRuntime(projectionStateRuntimeStub{})

	out := sys.Run(root, FrameInfo{})
	if out == nil {
		t.Fatal("expected frame output")
	}
	if root.projectCalls != 1 {
		t.Fatalf("root project calls = %d, want 1", root.projectCalls)
	}
	if child.projectCalls != 1 {
		t.Fatalf("child project calls = %d, want 1", child.projectCalls)
	}
	if len(sys.frameOutputs) != 2 {
		t.Fatalf("frame outputs = %d, want 2", len(sys.frameOutputs))
	}
	if sys.frameOutputs[0] == nil || sys.frameOutputs[0].FacetID != root.Base().ID() {
		t.Fatalf("first output = %#v, want root", sys.frameOutputs[0])
	}
	if sys.frameOutputs[1] == nil || sys.frameOutputs[1].FacetID != child.Base().ID() {
		t.Fatalf("second output = %#v, want child", sys.frameOutputs[1])
	}
	if len(out.RenderBatchs) != 2 {
		t.Fatalf("render batches = %d, want 2", len(out.RenderBatchs))
	}
}

func TestProjectionSystem_Run_forksSiblingSubtrees(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 320, 32))
	release := make(chan struct{})
	concurrentStarted := make(chan struct{}, 1)
	var active int32
	children := make([]*projectionTestFacet, 0, 9)
	for i := 0; i < 9; i++ {
		child := newProjectionTestFacet("child", gfx.RectFromXYWH(float32(i*16), 0, 12, 12))
		childIdx := i
		child.projection.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
			if atomic.AddInt32(&active, 1) == 2 {
				select {
				case concurrentStarted <- struct{}{}:
				default:
				}
			}
			defer atomic.AddInt32(&active, -1)
			<-release
			list := &gfx.CommandList{}
			list.Add(gfx.FillRect{
				Rect:  gfx.RectFromXYWH(float32(childIdx*16), 0, 12, 12),
				Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(0, 0, 255, 255)),
			})
			return list
		}
		root.Base().AddChild(&child.Facet)
		children = append(children, child)
	}

	attachTree(root)

	sys := NewSystem()
	sys.SetRuntime(projectionStateRuntimeStub{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		sys.Run(root, FrameInfo{})
	}()

	select {
	case <-concurrentStarted:
	case <-time.After(time.Second):
		close(release)
		<-done
		t.Fatal("expected sibling projection to fork concurrently")
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for concurrent projection run")
	}

	if got := len(sys.frameOutputs); got != 10 {
		t.Fatalf("frame outputs = %d, want 10", got)
	}
	if sys.frameOutputs[0] == nil || sys.frameOutputs[0].FacetID != root.Base().ID() {
		t.Fatalf("first output = %#v, want root", sys.frameOutputs[0])
	}
	for i, child := range children {
		if sys.frameOutputs[i+1] == nil || sys.frameOutputs[i+1].FacetID != child.Base().ID() {
			t.Fatalf("output %d = %#v, want child %d", i+1, sys.frameOutputs[i+1], i)
		}
	}
}

func TestProjectionSystem_Run_mergesForkedSubtreesInPreOrder(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 480, 32))
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	var active int32
	children := make([]*projectionTestFacet, 0, 9)
	grandchildren := make([]*projectionTestFacet, 0, 9)
	for i := 0; i < 9; i++ {
		child := newProjectionTestFacet("child", gfx.RectFromXYWH(float32(i*40), 0, 20, 12))
		grandchild := newProjectionTestFacet("grandchild", gfx.RectFromXYWH(float32(i*40), 12, 16, 8))
		childIdx := i
		grandchild.projection.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
			if atomic.AddInt32(&active, 1) == 2 {
				select {
				case started <- struct{}{}:
				default:
				}
			}
			defer atomic.AddInt32(&active, -1)
			<-release
			list := &gfx.CommandList{}
			list.Add(gfx.FillRect{
				Rect:  gfx.RectFromXYWH(float32(childIdx*40), 12, 16, 8),
				Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(255, 0, 0, 255)),
			})
			return list
		}
		child.Base().AddChild(&grandchild.Facet)
		root.Base().AddChild(&child.Facet)
		children = append(children, child)
		grandchildren = append(grandchildren, grandchild)
	}

	attachTree(root)

	sys := NewSystem()
	sys.SetRuntime(projectionStateRuntimeStub{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		sys.Run(root, FrameInfo{})
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		close(release)
		<-done
		t.Fatal("expected forked subtree projection to overlap")
	}

	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for merge run")
	}

	want := make([]facet.FacetID, 0, 19)
	want = append(want, root.Base().ID())
	for i := range children {
		want = append(want, children[i].Base().ID())
		want = append(want, grandchildren[i].Base().ID())
	}
	if len(sys.frameOutputs) != len(want) {
		t.Fatalf("frame outputs = %d, want %d", len(sys.frameOutputs), len(want))
	}
	for i := range want {
		if sys.frameOutputs[i] == nil || sys.frameOutputs[i].FacetID != want[i] {
			t.Fatalf("output %d = %#v, want facet %d", i, sys.frameOutputs[i], want[i])
		}
	}
}
