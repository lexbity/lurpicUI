package diagnostics

import (
	"testing"
)

// TestFrameStatsShape pins the RX-1 P5 FrameStats shape: the hot-path counters
// exist and default to zero on a fresh instance (compile-time field presence +
// zero-value reset semantics).
func TestFrameStatsShape(t *testing.T) {
	var s FrameStats
	if s.GateCount != 0 || s.PruneCount != 0 {
		t.Errorf("gate/prune counters not zero-valued: %+v", s)
	}
	if s.CollectCount != 0 || s.MaterializeCount != 0 || s.HitTestCount != 0 {
		t.Errorf("collect/materialize/hit counters not zero-valued: %+v", s)
	}
	if s.LayerResolveCount != 0 || s.ArrangeCount != 0 {
		t.Errorf("layer-resolve/arrange counters not zero-valued: %+v", s)
	}
	if s.DerivedEvaluated != 0 || s.DerivedRecomputed != 0 {
		t.Errorf("derived counters not zero-valued: %+v", s)
	}

	// Assign each field so the shape is pinned at compile time (a removed or
	// renamed field breaks this test).
	s.DerivedEvaluated = 1
	s.DerivedRecomputed = 2
	s.DerivedFlushDuration = 0
	s.ArrangeCount = 3
	s.CollectCount = 4
	s.MaterializeCount = 5
	s.HitTestCount = 6
	s.LayerResolveCount = 7
	s.PruneCount = 8
	s.GateCount = 9
	if s.GateCount != 9 || s.ArrangeCount != 3 {
		t.Fatalf("assigned counters not retained: %+v", s)
	}
}
