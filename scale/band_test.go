package scale

import (
	"math"
	"testing"
)

func TestBand_basic_geometry(t *testing.T) {
	members := []string{"A", "B", "C"}
	s := NewBand(members, WithRange(0, 300))
	// n=3, pi=0, po=0, span=300
	// step = 300 / (3 - 0 + 0) = 100
	if got := s.Step(); got != 100 {
		t.Fatalf("Step = %f, want 100", got)
	}
	// bandwidth = 100 * (1-0) = 100
	if got := s.Bandwidth(); got != 100 {
		t.Fatalf("Bandwidth = %f, want 100", got)
	}
	// start = 0 + (300 - 100*3) * 0.5 = 0
	// Band A: [0, 100)
	start, width, ok := s.Band("A")
	if !ok || start != 0 || width != 100 {
		t.Fatalf("Band(A) = (%f,%f,%v), want (0,100,true)", start, width, ok)
	}
	// Band B: [100, 200)
	start, width, ok = s.Band("B")
	if !ok || start != 100 || width != 100 {
		t.Fatalf("Band(B) = (%f,%f,%v), want (100,100,true)", start, width, ok)
	}
	// Band C: [200, 300)
	start, width, ok = s.Band("C")
	if !ok || start != 200 || width != 100 {
		t.Fatalf("Band(C) = (%f,%f,%v), want (200,100,true)", start, width, ok)
	}
}

func TestBand_center(t *testing.T) {
	s := NewBand([]string{"A", "B"}, WithRange(0, 200))
	// step=100, bandwidth=100, start=0
	c, ok := s.Center("A")
	if !ok || c != 50 {
		t.Fatalf("Center(A) = (%f,%v), want (50,true)", c, ok)
	}
	c, ok = s.Center("B")
	if !ok || c != 150 {
		t.Fatalf("Center(B) = (%f,%v), want (150,true)", c, ok)
	}
	_, ok = s.Center("missing")
	if ok {
		t.Fatal("Center(missing) should return ok=false")
	}
}

func TestBand_inner_padding(t *testing.T) {
	members := []string{"A", "B", "C"}
	s := NewBand(members, WithRange(0, 300), WithPaddingInner(0.2))
	// n=3, pi=0.2, po=0, span=300, align=0.5
	// denom = 3 - 0.2 + 0 = 2.8
	// step = 300 / 2.8 = 107.142857...
	// bandwidth = step * (1 - 0.2) = 85.714285...
	// start = (300 - step*(3-0.2)) * 0.5 = (300 - 107.142857*2.8) * 0.5 = 0
	// A: [0, 85.714], B: [107.143, 192.857], C: [214.286, 300.000]
	const eps = 1e-6
	step := s.Step()
	bw := s.Bandwidth()
	wantStep := 300.0 / 2.8

	if math.Abs(step-wantStep) > eps {
		t.Fatalf("step = %f, want %f", step, wantStep)
}
	if bw >= step {
		t.Fatalf("expected bandwidth < step with inner padding: step=%f bw=%f", step, bw)
	}

	aStart, aBw, ok := s.Band("A")
	if !ok || aBw != bw {
		t.Fatalf("Band(A) bandwidth = %f, want %f", aBw, bw)
	}
	if math.Abs(aStart) > eps {
		t.Fatalf("Band(A) start = %f, want 0 (align=0.5 fills both sides equally)", aStart)
	}

	bStart, _, ok := s.Band("B")
	if !ok {
		t.Fatal("Band(B) not found")
	}
	if math.Abs(bStart-aStart-step) > eps {
		t.Fatalf("Band(B) start = %f, want %f (A.start + step)", bStart, aStart+step)
	}

	cStart, _, ok := s.Band("C")
	if !ok {
		t.Fatal("Band(C) not found")
	}
	if cStart+bw > 300+eps {
		t.Fatalf("Band(C) extends past range: %f + %f = %f > 300", cStart, bw, cStart+bw)
	}
	if math.Abs(cStart+bw-300) > eps {
		t.Fatalf("Band(C) end = %f, want 300 (bands fill range exactly)", cStart+bw)
	}
	if cs, _ := s.Center("B"); math.Abs(cs-(aStart+step+bw/2)) > eps {
		t.Fatalf("Center(B) = %f, want %f", cs, aStart+step+bw/2)
	}
}

func TestBand_outer_padding(t *testing.T) {
	members := []string{"A", "B"}
	s := NewBand(members, WithRange(0, 200), WithPaddingOuter(0.1))
	// n=2, pi=0, po=0.1, span=200
	// step = 200 / (2 - 0 + 0.2) = 200 / 2.2 = 90.909...
	// bandwidth = 90.909...
	// start = 0 + (200 - 90.909*2) * 0.5 = 0 + (200-181.81)*0.5 = 9.09
	aStart, _, ok := s.Band("A")
	if !ok || aStart <= 0 {
		t.Fatalf("Band(A) start = %f, want > 0 (outer padding)", aStart)
	}
	bStart, bBw, ok := s.Band("B")
	if !ok {
		t.Fatal("Band(B) not found")
	}
	if bStart+bBw > 200+1e-9 {
		t.Fatalf("Band(B) extends past range: %f + %f = %f > 200", bStart, bBw, bStart+bBw)
	}
}

func TestBand_align_left(t *testing.T) {
	s := NewBand([]string{"A", "B"}, WithRange(0, 200), WithPaddingOuter(0.1), WithAlign(0))
	aStart, _, _ := s.Band("A")
	if aStart != 0 {
		t.Fatalf("Band(A) with align=0 should start at 0, got %f", aStart)
	}
}

func TestBand_align_right(t *testing.T) {
	s := NewBand([]string{"A", "B"}, WithRange(0, 200), WithPaddingOuter(0.1), WithAlign(1))
	bStart, bBw, _ := s.Band("B")
	if math.Abs(bStart+bBw-200) > 1e-9 {
		t.Fatalf("Band(B) end with align=1 = %f, want 200", bStart+bBw)
	}
}

// --- InvertRange ---

func TestBand_invertRange_basic(t *testing.T) {
	s := NewBand([]string{"A", "B", "C"}, WithRange(0, 300))
	tests := []struct {
		pos    float64
		want   string
		ok     bool
	}{
		{0, "A", true},
		{50, "A", true},
		{99.9, "A", true},
		{100, "B", true},
		{150, "B", true},
		{200, "C", true},
		{250, "C", true},
		{299.9, "C", true},
		{-1, "", false},
		{300, "", false},
		{500, "", false},
	}
	for _, tt := range tests {
		got, ok := s.InvertRange(tt.pos)
		if ok != tt.ok || got != tt.want {
			t.Errorf("InvertRange(%g) = (%q,%v), want (%q,%v)", tt.pos, got, ok, tt.want, tt.ok)
		}
	}
}

func TestBand_invertRange_gap(t *testing.T) {
	s := NewBand([]string{"A", "B"}, WithRange(0, 200), WithPaddingInner(0.5))
	// step = 200 / (2 - 0.5 + 0) = 200/1.5 = 133.33
	// bandwidth = 133.33 * 0.5 = 66.67
	// Band A: start=0, end=66.67, gap=66.67-133.33, Band B: start=133.33, end=200
	_, ok := s.InvertRange(90) // in the gap
	if ok {
		t.Fatal("InvertRange(90) should be in gap (ok=false)")
	}
}

// --- empty member set ---

func TestBand_empty_members(t *testing.T) {
	s := NewBand(nil, WithRange(0, 100))
	if s.Step() != 0 {
		t.Fatalf("Step = %f, want 0 for empty", s.Step())
	}
	if s.Bandwidth() != 0 {
		t.Fatalf("Bandwidth = %f, want 0 for empty", s.Bandwidth())
	}
	_, _, ok := s.Band("anything")
	if ok {
		t.Fatal("expected ok=false for empty members")
	}
	_, ok = s.Center("anything")
	if ok {
		t.Fatal("expected ok=false for empty members")
	}
	_, ok = s.InvertRange(50)
	if ok {
		t.Fatal("expected ok=false for empty members on InvertRange")
	}
}

// --- Map / Domain / Range (Scale interface) ---

func TestBand_Map_by_index(t *testing.T) {
	s := NewBand([]string{"A", "B", "C"}, WithRange(0, 300))
	if got := s.Map(0); got != 0 {
		t.Fatalf("Map(0) = %f, want 0 (band A start)", got)
	}
	if got := s.Map(1); got != 100 {
		t.Fatalf("Map(1) = %f, want 100 (band B start)", got)
	}
	if got := s.Map(2); got != 200 {
		t.Fatalf("Map(2) = %f, want 200 (band C start)", got)
	}
}

func TestBand_Map_out_of_range(t *testing.T) {
	s := NewBand([]string{"A", "B"}, WithRange(0, 200))
	if !math.IsNaN(s.Map(-1)) {
		t.Fatal("Map(-1) should return NaN")
	}
	if !math.IsNaN(s.Map(2)) {
		t.Fatal("Map(2) should return NaN")
	}
}

func TestBand_Domain(t *testing.T) {
	s := NewBand([]string{"A", "B", "C"}, WithRange(0, 300))
	lo, hi := s.Domain()
	if lo != 0 || hi != 2 {
		t.Fatalf("Domain = (%f,%f), want (0,2)", lo, hi)
	}
}

func TestBand_Domain_single(t *testing.T) {
	s := NewBand([]string{"only"}, WithRange(0, 100))
	lo, hi := s.Domain()
	if lo != 0 || hi != 0 {
		t.Fatalf("Domain(single) = (%f,%f), want (0,0)", lo, hi)
	}
}

func TestBand_Range(t *testing.T) {
	s := NewBand([]string{"A", "B"}, WithRange(50, 200))
	lo, hi := s.Range()
	if lo != 50 || hi != 200 {
		t.Fatalf("Range = (%f,%f), want (50,200)", lo, hi)
	}
}

// --- interface satisfaction ---

func TestBand_implements_Scale(t *testing.T) {
	var sc Scale = NewBand([]string{"A", "B"}, WithRange(0, 100))
	_ = sc
}

func TestBand_kind(t *testing.T) {
	s := NewBand([]string{"A"}, WithRange(0, 100))
	if s.Kind() != KindBand {
		t.Fatalf("Kind = %s, want KindBand", s.Kind())
	}
}

func TestBand_zero_span_range(t *testing.T) {
	s := NewBand([]string{"A", "B"}, WithRange(100, 100))
	// span = 0 → step = 0, bandwidth = 0
	if s.Step() != 0 {
		t.Fatalf("Step = %f, want 0 for zero-span range", s.Step())
	}
	if s.Bandwidth() != 0 {
		t.Fatalf("Bandwidth = %f, want 0 for zero-span range", s.Bandwidth())
	}
	start, width, ok := s.Band("A")
	if !ok || width != 0 {
		t.Fatalf("Band(A) = (%f,%f,%v)", start, width, ok)
	}
}

func TestBand_full_inner_padding(t *testing.T) {
	// pi=1, po=0 → denom = 1-1+0 = 0 → step=0
	s := NewBand([]string{"single"}, WithRange(0, 100), WithPaddingInner(1))
	if s.Step() != 0 {
		t.Fatalf("Step = %f, want 0 for denom=0", s.Step())
	}
}

// --- benchmarks ---

func BenchmarkBandScale_Band(b *testing.B) {
	members := make([]string, 10)
	for i := range members {
		members[i] = string(rune('A' + i))
	}
	s := NewBand(members, WithRange(0, 1000))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Band("E")
	}
}

func BenchmarkBandScale_InvertRange(b *testing.B) {
	members := make([]string, 10)
	for i := range members {
		members[i] = string(rune('A' + i))
	}
	s := NewBand(members, WithRange(0, 1000))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.InvertRange(550)
	}
}
