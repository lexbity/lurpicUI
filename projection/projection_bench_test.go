package projection

import (
	"fmt"
	goruntime "runtime"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func BenchmarkProjectionSequential(b *testing.B) {
	for _, depth := range []int{4, 8, 16} {
		for _, procs := range []int{1, 2, 4} {
			b.Run(fmt.Sprintf("depth=%d/procs=%d", depth, procs), func(b *testing.B) {
				benchmarkProjectionTree(b, buildBenchmarkProjectionChain(depth), procs)
			})
		}
	}
}

func BenchmarkProjectionParallel(b *testing.B) {
	for _, depth := range []int{2, 3, 4} {
		for _, procs := range []int{1, 2, 4} {
			b.Run(fmt.Sprintf("depth=%d/procs=%d", depth, procs), func(b *testing.B) {
				benchmarkProjectionTree(b, buildBenchmarkProjectionWide(depth, 3), procs)
			})
		}
	}
}

func benchmarkProjectionTree(b *testing.B, root *projectionTestFacet, procs int) {
	b.Helper()
	if root == nil {
		b.Fatal("expected benchmark root")
	}
	attachTree(root)
	sys := NewSystem()
	sys.SetRuntime(projectionStateRuntimeStub{})

	oldProcs := goruntime.GOMAXPROCS(procs)
	defer goruntime.GOMAXPROCS(oldProcs)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sys.MarkDirty(root.Base().ID())
		sys.Run(root, FrameInfo{Number: uint64(i + 1)})
	}
}

func buildBenchmarkProjectionChain(depth int) *projectionTestFacet {
	if depth < 1 {
		depth = 1
	}
	root := newProjectionTestFacet("chain_root", gfx.RectFromXYWH(0, 0, 32, 16))
	cur := root
	for i := 1; i < depth; i++ {
		child := newProjectionTestFacet(fmt.Sprintf("chain_%d", i), gfx.RectFromXYWH(float32(i*4), float32(i*2), 24, 12))
		cur.Base().AddChild(&child.Facet)
		cur = child
	}
	return root
}

func buildBenchmarkProjectionWide(depth, fanout int) *projectionTestFacet {
	if depth < 1 {
		depth = 1
	}
	if fanout < 1 {
		fanout = 1
	}
	counter := 0
	var build func(level int) *projectionTestFacet
	build = func(level int) *projectionTestFacet {
		counter++
		node := newProjectionTestFacet(
			fmt.Sprintf("wide_%d", counter),
			gfx.RectFromXYWH(float32(level*8), float32(level*4), 28, 14),
		)
		if level >= depth {
			return node
		}
		for i := 0; i < fanout; i++ {
			child := build(level + 1)
			node.Base().AddChild(&child.Facet)
		}
		return node
	}
	return build(1)
}

var _ facet.FacetImpl = (*projectionTestFacet)(nil)
