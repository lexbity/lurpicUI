package structure

import (
	"fmt"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

func benchmarkTableMeasureArrangeProject(b *testing.B, n int) {
	columns := []TableColumn{
		{Key: "id", Label: "ID"},
		{Key: "name", Label: "Name"},
		{Key: "status", Label: "Status"},
		{Key: "owner", Label: "Owner"},
	}
	rows := make([]TableRow, n)
	for i := range rows {
		rows[i] = TableRow{
			Key:   fmt.Sprintf("row-%d", i),
			Cells: []string{fmt.Sprintf("%03d", i), fmt.Sprintf("Row %d", i), "Active", "Benchmark"},
		}
	}
	data := TableData{
		Columns:       columns,
		Rows:          rows,
		SortColumnKey: "id",
	}

	fonts := testkit.TestFontRegistry(b)
	rt := cardRuntimeStub{fonts: fonts}
	ctx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(0, 0, 1280, 800)
	selection := store.NewValueStore("")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		t := NewTable("Benchmark table", data, selection)
		facet.Attach(t, facet.AttachContext{Runtime: rt, Theme: ctx})
		t.Layout.Measure(facet.MeasureContext{
			Runtime:          rt,
			Theme:            ctx,
			ContentScale:     1,
			Density:          facet.DensityID(theme.DensityIDComfortable),
			WritingDirection: facet.WritingDirectionLTR,
		}, facet.Constraints{MaxSize: gfx.Size{W: bounds.Width(), H: bounds.Height()}})
		t.Layout.Arrange(facet.ArrangeContext{
			Runtime:     rt,
			Theme:       ctx,
			ParentGroup: t.Layout.Parent,
			ChildGroup:  t.Layout.Child,
		}, bounds)
		_ = t.Projection.Project(facet.ProjectionContext{
			Runtime:      rt,
			Bounds:       bounds,
			ContentScale: 1,
		})
		b.StopTimer()
		facet.Dispose(t)
		b.StartTimer()
	}
}

func BenchmarkTableMeasureArrangeProject_N100(b *testing.B) {
	benchmarkTableMeasureArrangeProject(b, 100)
}

func BenchmarkTableMeasureArrangeProject_N1000(b *testing.B) {
	benchmarkTableMeasureArrangeProject(b, 1000)
}
