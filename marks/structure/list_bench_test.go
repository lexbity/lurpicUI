package structure

import (
	"fmt"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/theme"
)

func benchmarkListMeasureArrangeProject(b *testing.B, n int) {
	entries := make([]ListEntry, n)
	for i := range entries {
		entries[i] = ListEntry{
			Key:            fmt.Sprintf("k%d", i),
			Label:          fmt.Sprintf("List item %d", i),
			SupportingText: "Supporting text for the benchmark row",
		}
	}

	fonts := testkit.TestFontRegistry(b)
	rt := listRuntimeStub{
		fonts: fonts,
		icons: map[string]runtimepkg.IconAsset{},
	}
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 960, 4800)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		l := NewList("Benchmark list", entries)
		facet.Attach(l, facet.AttachContext{Runtime: rt, Theme: ctx})
		l.Layout.Measure(facet.MeasureContext{
			Runtime:          rt,
			Theme:            ctx,
			ContentScale:     1,
			Density:          facet.DensityID(theme.DensityIDComfortable),
			WritingDirection: facet.WritingDirectionLTR,
		}, facet.Constraints{MaxSize: gfx.Size{W: bounds.Width(), H: bounds.Height()}})
		l.Layout.Arrange(facet.ArrangeContext{
			Runtime:     rt,
			Theme:       ctx,
			ParentGroup: l.Layout.Parent,
			ChildGroup:  l.Layout.Child,
		}, bounds)
		_ = l.Projection.Project(facet.ProjectionContext{
			Runtime:      rt,
			Bounds:       bounds,
			ContentScale: 1,
		})
		b.StopTimer()
		facet.Dispose(l)
		b.StartTimer()
	}
}

func BenchmarkListMeasureArrangeProject_N100(b *testing.B) {
	benchmarkListMeasureArrangeProject(b, 100)
}

func BenchmarkListMeasureArrangeProject_N1000(b *testing.B) {
	benchmarkListMeasureArrangeProject(b, 1000)
}
