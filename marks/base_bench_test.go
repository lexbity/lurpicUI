package marks

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/store"
)

// BenchmarkCore_attach_50_mark_tree measures the attach cost of a 50-mark
// tree with one dynamic declared binding per mark (NFR-10). The first
// iteration pays the per-type reflection walk; every later instance of the
// type reads the cached declaration — the steady-state attach shape. The
// 2x-pre-RX-2 budget is recorded in the RX-2 P3 slice output against this
// bench; steady-state frames never touch the walk.
func BenchmarkCore_attach_50_mark_tree(b *testing.B) {
	s := store.NewValueStore("bench")

	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		marks := make([]*declaredMark, 0, 50)
		for i := 0; i < 50; i++ {
			marks = append(marks, newDeclaredMark(s))
		}
		b.StartTimer()

		for _, m := range marks {
			facet.Attach(m, facet.AttachContext{})
		}

		b.StopTimer()
		for _, m := range marks {
			facet.Dispose(m)
		}
		b.StartTimer()
	}
}
