package projection

import (
	"fmt"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func BenchmarkProjectionQuietRetainedTopology(b *testing.B) {
	for _, count := range []int{512, 2048} {
		b.Run(fmt.Sprintf("nodes=%d", count), func(b *testing.B) {
			root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 1280, 800))
			for i := 1; i < count; i++ {
				child := newProjectionTestFacet(fmt.Sprintf("child-%d", i), gfx.RectFromXYWH(0, 0, 1, 1))
				root.Base().AddChild(&child.Facet)
			}
			attachTree(root)
			sys := NewSystem()
			sys.SetRuntime(projectionStateRuntimeStub{})
			sys.MarkDirtyFlags(root.Base().ID(), facet.DirtyAll)
			sys.BeginDirtyGeneration()
			sys.Run(root, FrameInfo{Number: 1})
			sys.SealDirtyGeneration()
			baselineRebuilds := sys.TopologyRebuilds
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sys.Run(root, FrameInfo{Number: uint64(i + 2)})
			}
			b.StopTimer()
			if sys.TopologyRebuilds != baselineRebuilds {
				b.Fatalf("quiet projection rebuilt topology %d times", sys.TopologyRebuilds-baselineRebuilds)
			}
			if sys.ProjectedFacets != 0 {
				b.Fatalf("quiet projection recomputed %d facets", sys.ProjectedFacets)
			}
			b.ReportMetric(float64(sys.TopologyRebuilds-baselineRebuilds), "topology-rebuilds/op")
		})
	}
}

// BenchmarkProjectionRecoveryGuardOverhead compares a wide scene with and
// without the runtime's facet recovery surface. It is a focused microbenchmark,
// not a whole-runtime performance acceptance result.
func BenchmarkProjectionRecoveryGuardOverhead(b *testing.B) {
	build := func(b *testing.B, rt facet.RuntimeServices) {
		b.Helper()
		root := buildBenchmarkProjectionWide(3, 3)
		attachTree(root)
		sys := NewSystem()
		sys.SetRuntime(rt)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sys.MarkDirty(root.Base().ID())
			sys.Run(root, FrameInfo{Number: uint64(i + 1)})
		}
	}
	b.Run("unguarded", func(b *testing.B) { build(b, projectionStateRuntimeStub{}) })
	b.Run("guarded", func(b *testing.B) { build(b, &recoveryStub{}) })
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
