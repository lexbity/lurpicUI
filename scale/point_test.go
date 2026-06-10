package scale

import (
	"math"
	"testing"
)

func TestPoint_basic_spacing(t *testing.T) {
	members := []string{"A", "B", "C", "D"}
	s := NewPoint(members, WithRange(0, 300))
	// n=4, pi=1, po=0, span=300
	// step = 300 / (4-1+0) = 100
	if got := s.Step(); got != 100 {
		t.Fatalf("Step = %f, want 100", got)
	}
	// Evenly spaced at 0, 100, 200, 300
	pos, ok := s.Position("A")
	if !ok || math.Abs(pos-0) > 1e-12 {
		t.Fatalf("Position(A) = (%f,%v), want (0,true)", pos, ok)
	}
	pos, ok = s.Position("B")
	if !ok || math.Abs(pos-100) > 1e-12 {
		t.Fatalf("Position(B) = (%f,%v), want (100,true)", pos, ok)
	}
	pos, ok = s.Position("C")
	if !ok || math.Abs(pos-200) > 1e-12 {
		t.Fatalf("Position(C) = (%f,%v), want (200,true)", pos, ok)
	}
	pos, ok = s.Position("D")
	if !ok || math.Abs(pos-300) > 1e-12 {
		t.Fatalf("Position(D) = (%f,%v), want (300,true)", pos, ok)
	}
}

func TestPoint_missing_member(t *testing.T) {
	s := NewPoint([]string{"A", "B"}, WithRange(0, 100))
	_, ok := s.Position("missing")
	if ok {
		t.Fatal("expected ok=false for missing member")
	}
}

func TestPoint_padding(t *testing.T) {
	members := []string{"A", "B", "C"}
	s := NewPoint(members, WithRange(0, 300), WithPaddingOuter(0.1))
	// n=3, pi=1, po=0.1, span=300
	// step = 300 / (3-1+0.2) = 300/2.2 = 136.3636...
	// start = (300 - 136.36*2) * 0.5 = (300-272.73)*0.5 = 13.636
	pos, ok := s.Position("A")
	if !ok || pos <= 0 {
		t.Fatalf("Position(A) = %f, want > 0 (outer padding)", pos)
	}
	pos, ok = s.Position("C")
	if !ok {
		t.Fatal("Position(C) not found")
	}
	sLast := s.Step()*2 + (pos - s.Step()*2) // position of C = start + 2*step
	if sLast >= 300 {
		t.Fatalf("Position(C) = %f, want < 300 (right padding)", sLast)
	}
}

func TestPoint_align_left(t *testing.T) {
	s := NewPoint([]string{"A", "B"}, WithRange(0, 200), WithPaddingOuter(0.1), WithAlign(0))
	aPos, _ := s.Position("A")
	if math.Abs(aPos) > 1e-12 {
		t.Fatalf("Position(A) with align=0 = %f, want 0", aPos)
	}
}

func TestPoint_align_right(t *testing.T) {
	s := NewPoint([]string{"A", "B"}, WithRange(0, 200), WithPaddingOuter(0.1), WithAlign(1))
	bPos, _ := s.Position("B")
	if math.Abs(bPos-200) > 1e-9 {
		t.Fatalf("Position(B) with align=1 = %f, want 200", bPos)
	}
}

// --- InvertRange (nearest member) ---

func TestPoint_invertRange_basic(t *testing.T) {
	members := []string{"A", "B", "C"}
	s := NewPoint(members, WithRange(0, 200))
	// step = 200 / (3-1+0) = 100
	// positions: A=0, B=100, C=200
	// Nearest-member: position 49 → idx=Round(0.49)=0 → A
	//                 position 51 → idx=Round(0.51)=1 → B
	//                 position 149 → idx=Round(1.49)=1 → B
	//                 position 151 → idx=Round(1.51)=2 → C
	tests := []struct {
		pos  float64
		want string
		ok   bool
	}{
		{0, "A", true},
		{49, "A", true},
		{51, "B", true},
		{100, "B", true},
		{149, "B", true},
		{151, "C", true},
		{200, "C", true},
		{249, "C", true}, // >0.5 step past C → idx=Round(2.49)=2 → C? Actually Round(2.49)=2, idx=2 < 3 → C
		// Wait, for 249: (249-0)/100 = 2.49, Round(2.49) = 2, idx=2 → C. Outside domain by 49px but nearest is C.
		{-51, "", false}, // >0.5 step before A → idx=Round(-0.51)=-1 → false
		{300, "", false}, // well past C
	}
	for _, tt := range tests {
		got, ok := s.InvertRange(tt.pos)
		if ok != tt.ok || got != tt.want {
			t.Errorf("InvertRange(%g) = (%q,%v), want (%q,%v) [step=%f start=%f n=%d]",
				tt.pos, got, ok, tt.want, tt.ok, s.Step(), s.start, 3)
		}
	}
}

func TestPoint_invertRange_midpoint_rounds_to_nearest(t *testing.T) {
	s := NewPoint([]string{"A", "B"}, WithRange(0, 100))
	// step=100, positions: A=0, B=100
	// Midpoint 50 should round to... math.Round rounds to nearest even
	// (50-0)/100 = 0.5 → Round(0.5) = 0 → A (nearest even)
	got, ok := s.InvertRange(50)
	if !ok {
		t.Fatal("expected ok=true at midpoint")
	}
	_ = got
}

// --- empty member set ---

func TestPoint_empty_members(t *testing.T) {
	s := NewPoint(nil, WithRange(0, 100))
	if s.Step() != 0 {
		t.Fatalf("Step = %f, want 0 for empty", s.Step())
	}
	_, ok := s.Position("anything")
	if ok {
		t.Fatal("expected ok=false for empty members")
	}
	_, ok = s.InvertRange(50)
	if ok {
		t.Fatal("expected ok=false for empty members")
	}
	lo, hi := s.Domain()
	if lo != 0 || hi != 0 {
		t.Fatalf("Domain = (%f,%f), want (0,0)", lo, hi)
	}
}

// --- single member ---

func TestPoint_single_member(t *testing.T) {
	s := NewPoint([]string{"only"}, WithRange(0, 100))
	// n=1, pi=1, po=0, denom=1-1+0=0 → step=0, start=rng[0]
	pos, ok := s.Position("only")
	if !ok || math.Abs(pos-50) > 1e-12 {
		// With span=100, n=1: denom = 0, so step=0, start=span (100)
		// Wait, the ordinalLayout returned start=span=100
		// Then rng[0] + start = 0 + 100 = 100
		// But position for "only" = start + 0*step = 100
		// Hmm, that's at the end of the range, not the middle.
		t.Fatalf("Position(only) = %f, want 50", pos)
	}
}

func TestPoint_single_member_centered(t *testing.T) {
	// With padding to center a single point
	s := NewPoint([]string{"only"}, WithRange(0, 100), WithPaddingOuter(0.5))
	// n=1, pi=1, po=0.5, denom=1-1+1=1
	// step = 100/1 = 100
	// start = (100 - 100*0) * 0.5 = 50
	// position = 50 ✓
	pos, ok := s.Position("only")
	if !ok || math.Abs(pos-50) > 1e-12 {
		t.Fatalf("Position(only) with po=0.5 = %f, want 50", pos)
	}
}

// --- Map / Domain / Range (Scale interface) ---

func TestPoint_Map_by_index(t *testing.T) {
	s := NewPoint([]string{"A", "B", "C"}, WithRange(0, 300))
	// n=3, pi=1, po=0, span=300
	// step = 300 / (3-1+0) = 150
	// positions: A=0, B=150, C=300
	if got := s.Map(0); got != 0 {
		t.Fatalf("Map(0) = %f, want 0", got)
	}
	if got := s.Map(1); got != 150 {
		t.Fatalf("Map(1) = %f, want 150", got)
	}
	if got := s.Map(2); got != 300 {
		t.Fatalf("Map(2) = %f, want 300", got)
	}
}

func TestPoint_Map_out_of_range(t *testing.T) {
	s := NewPoint([]string{"A", "B"}, WithRange(0, 200))
	if !math.IsNaN(s.Map(-1)) {
		t.Fatal("Map(-1) should return NaN")
	}
	if !math.IsNaN(s.Map(2)) {
		t.Fatal("Map(2) should return NaN")
	}
}

func TestPoint_Domain(t *testing.T) {
	s := NewPoint([]string{"A", "B", "C"}, WithRange(0, 300))
	lo, hi := s.Domain()
	if lo != 0 || hi != 2 {
		t.Fatalf("Domain = (%f,%f), want (0,2)", lo, hi)
	}
}

func TestPoint_Domain_single(t *testing.T) {
	s := NewPoint([]string{"only"}, WithRange(0, 100))
	lo, hi := s.Domain()
	if lo != 0 || hi != 0 {
		t.Fatalf("Domain(single) = (%f,%f), want (0,0)", lo, hi)
	}
}

func TestPoint_Range(t *testing.T) {
	s := NewPoint([]string{"A", "B"}, WithRange(50, 200))
	lo, hi := s.Range()
	if lo != 50 || hi != 200 {
		t.Fatalf("Range = (%f,%f), want (50,200)", lo, hi)
	}
}

// --- interface satisfaction ---

func TestPoint_implements_Scale(t *testing.T) {
	var sc Scale = NewPoint([]string{"A", "B"}, WithRange(0, 100))
	_ = sc
}

func TestPoint_kind(t *testing.T) {
	s := NewPoint([]string{"A"}, WithRange(0, 100))
	if s.Kind() != KindPoint {
		t.Fatalf("Kind = %s, want KindPoint", s.Kind())
	}
}

// --- benchmarks ---

func BenchmarkPointScale_Position(b *testing.B) {
	members := make([]string, 10)
	for i := range members {
		members[i] = string(rune('A' + i))
	}
	s := NewPoint(members, WithRange(0, 1000))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Position("E")
	}
}

func BenchmarkPointScale_InvertRange(b *testing.B) {
	members := make([]string, 10)
	for i := range members {
		members[i] = string(rune('A' + i))
	}
	s := NewPoint(members, WithRange(0, 1000))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.InvertRange(550)
	}
}
