package scale

import (
	"math"
	"testing"
)

// --- tickStep ---

func TestTickStep_basic(t *testing.T) {
	tests := []struct {
		name  string
		lo, hi float64
		count int
		want  float64
	}{
		{"[0,97] count=5 → step=20", 0, 97, 5, 20},
		{"[0,97] count=10 → step=10", 0, 97, 10, 10},
		{"[0,1] count=5 → step=0.2", 0, 1, 5, 0.2},
		{"[0,1] count=10 → step=0.1", 0, 1, 10, 0.1},
		{"[-100,100] count=5 → step=50", -100, 100, 5, 50},
		{"[-100,100] count=10 → step=20", -100, 100, 10, 20},
		{"[0,1000] count=5 → step=200", 0, 1000, 5, 200},
		{"[0,1000] count=10 → step=100", 0, 1000, 10, 100},
		{"[0,0.01] count=5 → step=0.002", 0, 0.01, 5, 0.002},
		{"[0,1e6] count=10 → step=100000", 0, 1e6, 10, 100000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tickStep(tt.lo, tt.hi, tt.count)
			if got != tt.want {
				t.Errorf("tickStep(%g,%g,%d) = %g, want %g", tt.lo, tt.hi, tt.count, got, tt.want)
			}
		})
	}
}

func TestTickStep_reversed(t *testing.T) {
	// Reversed domain should produce same step as forward
	fwd := tickStep(0, 100, 5)
	rev := tickStep(100, 0, 5)
	if fwd != rev {
		t.Fatalf("tickStep(100,0,5) = %g, want %g (same as forward)", rev, fwd)
	}
}

func TestTickStep_degenerate(t *testing.T) {
	if got := tickStep(5, 5, 10); got != 0 {
		t.Fatalf("tickStep(5,5,10) = %g, want 0", got)
	}
}

func TestTickStep_zero_count(t *testing.T) {
	if got := tickStep(0, 100, 0); got != 0 {
		t.Fatalf("tickStep(0,100,0) = %g, want 0", got)
	}
	if got := tickStep(0, 100, -1); got != 0 {
		t.Fatalf("tickStep(0,100,-1) = %g, want 0", got)
	}
}

func TestTickStep_nan(t *testing.T) {
	got := tickStep(math.NaN(), 100, 10)
	if !math.IsNaN(got) {
		t.Fatalf("tickStep(NaN,100,10) = %g, want NaN", got)
	}
}

// --- ticks ---

func TestTicks_golden(t *testing.T) {
	tests := []struct {
		name  string
		lo, hi float64
		count int
		want  []float64
	}{
		{"[0,97] c=5", 0, 97, 5, []float64{0, 20, 40, 60, 80}},
		{"[0,97] c=10", 0, 97, 10, []float64{0, 10, 20, 30, 40, 50, 60, 70, 80, 90}},
		{"[0,100] c=5", 0, 100, 5, []float64{0, 20, 40, 60, 80, 100}},
		{"[0,100] c=10", 0, 100, 10, []float64{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100}},
		{"[0,1] c=5", 0, 1, 5, []float64{0, 0.2, 0.4, 0.6, 0.8, 1.0}},
		{"[0,1] c=10", 0, 1, 10, []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}},
		{"[-100,100] c=5", -100, 100, 5, []float64{-100, -50, 0, 50, 100}},
		{"[-100,100] c=10", -100, 100, 10, []float64{-100, -80, -60, -40, -20, 0, 20, 40, 60, 80, 100}},
		{"[-97,-3] c=5", -97, -3, 5, []float64{-80, -60, -40, -20}},
		{"[-97,-3] c=10", -97, -3, 10, []float64{-90, -80, -70, -60, -50, -40, -30, -20, -10}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ticks(tt.lo, tt.hi, tt.count)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d\n  got:  %v\n  want: %v", len(got), len(tt.want), got, tt.want)
			}
			const eps = 1e-12
			for i := range got {
				if math.Abs(got[i]-tt.want[i]) > eps {
					t.Fatalf("[%d] = %g, want %g\n  got:  %v\n  want: %v", i, got[i], tt.want[i], got, tt.want)
				}
			}
		})
	}
}

func TestTicks_reversed_domain(t *testing.T) {
	// Reversed domain should still produce increasing ticks
	got := ticks(100, 0, 5)
	want := []float64{0, 20, 40, 60, 80, 100}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d\n  got: %v\n  want: %v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("[%d] = %g, want %g", i, got[i], want[i])
		}
	}
}

func TestTicks_degenerate(t *testing.T) {
	if got := ticks(5, 5, 10); got != nil {
		t.Fatalf("ticks(5,5,10) = %v, want nil", got)
	}
}

func TestTicks_zero_count(t *testing.T) {
	if got := ticks(0, 100, 0); got != nil {
		t.Fatalf("ticks(0,100,0) = %v, want nil", got)
	}
}

func TestTicks_no_duplicates(t *testing.T) {
	vals := ticks(-1e6, 1e6, 20)
	seen := make(map[float64]bool)
	for _, v := range vals {
		if seen[v] {
			t.Fatalf("duplicate tick: %g", v)
		}
		seen[v] = true
	}
}

func TestTicks_no_nan(t *testing.T) {
	vals := ticks(-1e6, 1e6, 20)
	for _, v := range vals {
		if math.IsNaN(v) {
			t.Fatal("found NaN tick")
		}
	}
}

func TestTicks_monotonic(t *testing.T) {
	vals := ticks(-1e6, 1e6, 10)
	for i := 1; i < len(vals); i++ {
		if vals[i] <= vals[i-1] {
			t.Fatalf("non-monotonic at [%d]: %g <= %g", i, vals[i], vals[i-1])
		}
	}
}

func TestTicks_beyond_max_count(t *testing.T) {
	// Large-enough domain with fine-grained step exceeds maxTicks.
	got := ticks(0, 2000, 2000)
	if got != nil {
		t.Fatalf("expected nil for tick count > maxTicks, got len=%d", len(got))
	}
}

func TestTicks_step_overshoots_span(t *testing.T) {
	// When tickStep produces a step larger than the domain span,
	// no tick falls within the interval → nil.
	// tickStep(-97,-3,1) → span=94, step=100 (>94)
	got := ticks(-97, -3, 1)
	if got != nil {
		t.Fatalf("expected nil when step exceeds span, got %v", got)
	}
}

func TestTicks_reasonable_count(t *testing.T) {
	// Should not explode for edge-case inputs
	vals := ticks(0, 100, 1)
	if len(vals) == 0 || len(vals) > 100 {
		t.Fatalf("unexpected tick count: %d", len(vals))
	}
}

// --- Nice ---

func TestNice_basic(t *testing.T) {
	tests := []struct {
		name        string
		lo, hi      float64
		count       int
		wantLo, wantHi float64
	}{
		{"[0,97] c=5", 0, 97, 5, 0, 100},
		{"[3,97] c=5", 3, 97, 5, 0, 100},
		{"[0,1] c=5", 0, 1, 5, 0, 1},
		{"[0.12,0.87] c=5", 0.12, 0.87, 5, 0, 1},
		{"[-97,-3] c=5", -97, -3, 5, -100, 0},
		{"[-100,100] c=5", -100, 100, 5, -100, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLo, gotHi := Nice(tt.lo, tt.hi, tt.count)
			if gotLo != tt.wantLo || gotHi != tt.wantHi {
				t.Errorf("Nice(%g,%g,%d) = (%g,%g), want (%g,%g)",
					tt.lo, tt.hi, tt.count, gotLo, gotHi, tt.wantLo, tt.wantHi)
			}
		})
	}
}

func TestNice_degenerate(t *testing.T) {
	lo, hi := Nice(5, 5, 10)
	if lo != 5 || hi != 5 {
		t.Fatalf("Nice(5,5,10) = (%g,%g), want (5,5)", lo, hi)
	}
}

func TestNice_zero_count(t *testing.T) {
	lo, hi := Nice(0, 100, 0)
	if lo != 0 || hi != 100 {
		t.Fatalf("Nice(0,100,0) = (%g,%g), want (0,100)", lo, hi)
	}
}

func TestNice_step_is_zero(t *testing.T) {
	// An extremely small domain can cause tickStep to return 0.
	// Nice should fall back to the original domain.
	small := math.SmallestNonzeroFloat64
	lo, hi := Nice(0, small, 10)
	if lo != 0 || hi != small {
		t.Fatalf("Nice(0,smallest,10) = (%g,%g), want (0,%g)", lo, hi, small)
	}
}

// --- tickLabels ---

func TestTickLabels_format(t *testing.T) {
	vals := []float64{0, 0.1, 0.5, 1, 10, 100, 1000}
	labels := tickLabels(vals)
	if len(labels) != len(vals) {
		t.Fatalf("len = %d, want %d", len(labels), len(vals))
	}
	wantLabels := []string{"0", "0.1", "0.5", "1", "10", "100", "1000"}
	for i, tl := range labels {
		if tl.Value != vals[i] {
			t.Errorf("labels[%d].Value = %g, want %g", i, tl.Value, vals[i])
		}
		if tl.Label != wantLabels[i] {
			t.Errorf("labels[%d].Label = %q, want %q", i, tl.Label, wantLabels[i])
		}
	}
}

// --- LinearScale.Ticks ---

func TestLinear_Ticks_basic(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500))
	ticks := s.Ticks(5)
	if len(ticks) == 0 {
		t.Fatal("expected non-empty ticks")
	}
	// Values should be increasing
	for i := 1; i < len(ticks); i++ {
		if ticks[i].Value <= ticks[i-1].Value {
			t.Fatalf("non-monotonic tick at [%d]: %g <= %g", i, ticks[i].Value, ticks[i-1].Value)
		}
	}
	// All ticks should have non-empty labels
	for _, tk := range ticks {
		if tk.Label == "" {
			t.Fatal("empty label")
		}
	}
}

func TestLinear_Ticks_degenerate_domain(t *testing.T) {
	s := NewLinear(WithDomain(5, 5), WithRange(0, 100))
	if got := s.Ticks(5); got != nil {
		t.Fatalf("expected nil ticks for degenerate domain, got %v", got)
	}
}

func TestLinear_Ticks_zero_count(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500))
	if got := s.Ticks(0); got != nil {
		t.Fatalf("expected nil ticks for count=0, got %v", got)
	}
}

// --- interface satisfaction ---

func TestLinear_implements_Ticker(t *testing.T) {
	var tk Ticker = NewLinear()
	_ = tk
}

// --- benchmarks ---

func BenchmarkLinearScale_Ticks(b *testing.B) {
	s := NewLinear(WithDomain(0, 1000), WithRange(0, 500))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Ticks(10)
	}
}
