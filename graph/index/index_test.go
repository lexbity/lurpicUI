package index

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// helper: build an index from (id, x, y, w, h) tuples.
func buildIndex(t *testing.T, entries ...float32) *rstarTree {
	t.Helper()
	if len(entries)%5 != 0 {
		t.Fatal("entries must be multiples of 5: id,x,y,w,h")
	}
	b := NewRStarIndexBuilder(len(entries) / 5)
	for i := 0; i < len(entries); i += 5 {
		b.Add(EntityID(entries[i]), gfx.RectFromXYWH(entries[i+1], entries[i+2], entries[i+3], entries[i+4]))
	}
	return b.buildWithCancel(nil)
}

func TestRStarIndex_query_empty(t *testing.T) {
	b := NewRStarIndexBuilder(0)
	idx := b.Build()
	got := idx.Query(gfx.RectFromXYWH(0, 0, 100, 100))
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestRStarIndex_query_single_entity(t *testing.T) {
	idx := buildIndex(t, 1, 10, 10, 20, 20)
	got := idx.Query(gfx.RectFromXYWH(0, 0, 50, 50))
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("expected [1], got %v", got)
	}
}

func TestRStarIndex_query_non_overlapping(t *testing.T) {
	idx := buildIndex(t, 1, 10, 10, 20, 20)
	got := idx.Query(gfx.RectFromXYWH(100, 100, 50, 50))
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestRStarIndex_querypoint_hit(t *testing.T) {
	idx := buildIndex(t, 7, 10, 10, 20, 20)
	id, ok := idx.QueryPoint(gfx.Point{X: 20, Y: 20}, 1)
	if !ok || id != 7 {
		t.Fatalf("expected hit id=7, got id=%v ok=%v", id, ok)
	}
}

func TestRStarIndex_querypoint_miss(t *testing.T) {
	idx := buildIndex(t, 7, 10, 10, 20, 20)
	_, ok := idx.QueryPoint(gfx.Point{X: 200, Y: 200}, 1)
	if ok {
		t.Fatal("expected miss")
	}
}

func TestRStarIndex_querynearest(t *testing.T) {
	// Three entities: close at (5,5), medium at (50,50), far at (200,200)
	idx := buildIndex(t,
		1, 4, 4, 2, 2,
		2, 49, 49, 2, 2,
		3, 199, 199, 2, 2,
	)
	id, ok := idx.QueryNearest(gfx.Point{X: 0, Y: 0}, 300)
	if !ok || id != 1 {
		t.Fatalf("expected id=1 (nearest), got id=%v ok=%v", id, ok)
	}
}

func TestRStarIndex_bounds_covers_all(t *testing.T) {
	idx := buildIndex(t,
		1, 10, 10, 5, 5,
		2, 50, 50, 5, 5,
		3, 100, 100, 5, 5,
	)
	b := idx.Bounds()
	if b.Min.X > 10 || b.Min.Y > 10 || b.Max.X < 105 || b.Max.Y < 105 {
		t.Fatalf("bounds too tight: %+v", b)
	}
}

func TestRStarIndex_len(t *testing.T) {
	idx := buildIndex(t, 1, 0, 0, 10, 10, 2, 20, 20, 10, 10, 3, 40, 40, 10, 10)
	if idx.Len() != 3 {
		t.Fatalf("expected Len=3, got %d", idx.Len())
	}
}

func TestRStarIndex_large_dataset(t *testing.T) {
	const N = 100_000
	b := NewRStarIndexBuilder(N)
	for i := 0; i < N; i++ {
		x := float32(i%1000) * 10
		y := float32(i/1000) * 10
		b.Add(EntityID(i+1), gfx.RectFromXYWH(x, y, 8, 8))
	}
	tree := b.buildWithCancel(nil)

	start := time.Now()
	got := tree.Query(gfx.RectFromXYWH(0, 0, 100, 100))
	elapsed := time.Since(start)

	if elapsed > time.Millisecond {
		t.Fatalf("query took %v, expected < 1ms", elapsed)
	}
	// Region [0,0,100,100] covers x in [0,100), y in [0,100).
	// Entities with x in 0..90 (step 10) and y in [0,90] (step 10) qualify.
	// Each entity is 8x8 so x=90,y=90 has bounds [90,90,98,98] which intersects.
	if len(got) == 0 {
		t.Fatal("expected non-empty result for large dataset query")
	}
}

func TestLODIndex_all_individual_at_high_zoom(t *testing.T) {
	// 3 large nodes (100x100) at high zoom: 100*2 = 200 > MinIndividualPixels(8)
	idx := buildIndex(t,
		1, 0, 0, 100, 100,
		2, 200, 0, 100, 100,
		3, 400, 0, 100, 100,
	)
	result := idx.QueryLOD(gfx.RectFromXYWH(-1000, -1000, 5000, 5000), 2.0)
	if len(result.Individuals) != 3 {
		t.Fatalf("expected 3 individuals at high zoom, got %d individuals, %d clusters",
			len(result.Individuals), len(result.Clusters))
	}
	if len(result.Clusters) != 0 {
		t.Fatalf("expected 0 clusters at high zoom, got %d", len(result.Clusters))
	}
}

func TestLODIndex_all_clusters_at_low_zoom(t *testing.T) {
	// 3 nodes (1x1) at low zoom: 1*0.01 = 0.01 < MinIndividualPixels(8)
	idx := buildIndex(t,
		1, 0, 0, 1, 1,
		2, 2, 0, 1, 1,
		3, 4, 0, 1, 1,
	)
	result := idx.QueryLOD(gfx.RectFromXYWH(-100, -100, 1000, 1000), 0.01)
	if len(result.Individuals) != 0 {
		t.Fatalf("expected 0 individuals at low zoom, got %d", len(result.Individuals))
	}
	if len(result.Clusters) == 0 {
		t.Fatal("expected clusters at low zoom, got none")
	}
}

func TestLODIndex_partial_lod_at_medium_zoom(t *testing.T) {
	// Large nodes (50x50) at medium zoom: some individual, some clustered
	// pixelsPerUnit=0.2: 50*0.2=10 ≥ 8 → individual
	// Small nodes (1x1): 1*0.2=0.2 < 8 → cluster
	idx := buildIndex(t,
		1, 0, 0, 50, 50, // individual
		2, 100, 0, 50, 50, // individual
		3, 200, 0, 1, 1, // cluster
	)
	result := idx.QueryLOD(gfx.RectFromXYWH(-100, -100, 2000, 2000), 0.2)
	if len(result.Individuals) == 0 {
		t.Fatal("expected some individuals at medium zoom")
	}
	if len(result.Clusters) == 0 {
		t.Fatal("expected some clusters at medium zoom")
	}
}

func TestLODIndex_viewport_culls_offscreen(t *testing.T) {
	idx := buildIndex(t,
		1, 0, 0, 10, 10,
		2, 1000, 1000, 10, 10,
	)
	result := idx.QueryLOD(gfx.RectFromXYWH(0, 0, 50, 50), 2.0)
	total := len(result.Individuals) + len(result.Clusters)
	if total != 1 {
		t.Fatalf("expected 1 entity in viewport, got %d", total)
	}
}

// alreadyCancelled is a mock Canceller that always reports cancelled.
type alreadyCancelled struct{}

func (alreadyCancelled) Cancelled() bool { return true }

func TestIndexBuilder_cancellation_exits_early(t *testing.T) {
	const N = 100_000
	b := NewRStarIndexBuilder(N)
	for i := 0; i < N; i++ {
		b.Add(EntityID(i+1), gfx.RectFromXYWH(float32(i), 0, 1, 1))
	}

	result := b.BuildWithCancel(alreadyCancelled{})
	if result == nil {
		t.Fatal("expected non-nil result even when cancelled")
	}
	// Pre-cancelled build returns empty tree, not a full N-entity tree.
	if result.Len() == N {
		t.Fatalf("expected early exit (Len < N=%d), got Len=%d", N, result.Len())
	}
}
