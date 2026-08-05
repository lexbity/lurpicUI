package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
)

// BenchmarkRunOneFrame measures the full per-frame cost, including the
// frame-vs-shutdown lock pair (frameMu RLock/RUnlock). NFR-2 caps the added
// lock cost at 200 ns/frame; the uncontended read lock is a single atomic op.
func BenchmarkRunOneFrame(b *testing.B) {
	root := facet.NewFacet()
	cfg := DefaultConfig()
	reg, err := layout.StandardLayerRegistry()
	if err != nil {
		b.Fatal(err)
	}
	cfg.LayerRegistry = reg
	rt, err := New(cfg, nil, nil, &backendFixture{}, &root)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.RunOneFrame()
	}
}
