package scale

import (
	"math"
	"testing"
)

// --- autoPrecision ---

func TestAutoPrecision(t *testing.T) {
	tests := []struct {
		step float64
		want int
	}{
		{20, 0},
		{10, 0},
		{5, 0},
		{2, 0},
		{1, 0},
		{0.5, 1},
		{0.2, 1},
		{0.1, 1},
		{0.05, 2},
		{0.02, 2},
		{0.01, 2},
		{0.005, 3},
		{0.002, 3},
		{0.001, 3},
		{0.0005, 4},
		{0.0002, 4},
		{100, 0},
		{1000, 0},
	}
	for _, tt := range tests {
		got := autoPrecision(tt.step)
		if got != tt.want {
			t.Errorf("autoPrecision(%g) = %d, want %d", tt.step, got, tt.want)
		}
	}
}

func TestAutoPrecision_degenerate(t *testing.T) {
	if got := autoPrecision(0); got != 0 {
		t.Fatalf("autoPrecision(0) = %d, want 0", got)
	}
	if got := autoPrecision(-5); got != 0 {
		t.Fatalf("autoPrecision(-5) = %d, want 0", got)
	}
}

func TestAutoPrecision_nan_inf(t *testing.T) {
	if got := autoPrecision(math.NaN()); got != 0 {
		t.Fatalf("autoPrecision(NaN) = %d, want 0", got)
	}
	if got := autoPrecision(math.Inf(1)); got != 0 {
		t.Fatalf("autoPrecision(+Inf) = %d, want 0", got)
	}
}

// --- FormatFixed ---

func TestFormatFixed(t *testing.T) {
	tests := []struct {
		v         float64
		precision int
		want      string
	}{
		{0, 0, "0"},
		{0.5, 1, "0.5"},
		{0, 1, "0.0"},
		{1, 1, "1.0"},
		{1.5, 0, "2"}, // round-half-to-even → 2
		{-1.5, 0, "-2"},
		{3.14159, 2, "3.14"},
		{3.14159, 3, "3.142"},
		{100, 0, "100"},
		{100.5, 0, "100"}, // round-half-to-even → 100 (even)
		{0.1, 1, "0.1"},
		{0.1, 2, "0.10"},
		{-0.5, 1, "-0.5"},
		{0, 4, "0.0000"},
	}
	for _, tt := range tests {
		got := FormatFixed(tt.v, tt.precision)
		if got != tt.want {
			t.Errorf("FormatFixed(%g,%d) = %q, want %q", tt.v, tt.precision, got, tt.want)
		}
	}
}

func TestFormatFixed_negative_precision(t *testing.T) {
	got := FormatFixed(3.14159, -1)
	if got != "3" {
		t.Fatalf("FormatFixed(3.14159,-1) = %q, want %q", got, "3")
	}
}

func TestFormatFixed_nan(t *testing.T) {
	got := FormatFixed(math.NaN(), 2)
	if got != "NaN" {
		t.Fatalf("FormatFixed(NaN,2) = %q, want %q", got, "NaN")
	}
}

// --- FormatSignificant ---

func TestFormatSignificant(t *testing.T) {
	tests := []struct {
		v      float64
		digits int
		want   string
	}{
		{3.14159, 3, "3.14"},
		{3.14159, 1, "3"},
		{3.14159, 4, "3.142"},
		{0, 3, "0"},
		{1000, 3, "1.00e+03"},
		{1000000, 3, "1.00e+06"},
		{0.001234, 2, "0.0012"},
		{0.001234, 3, "0.00123"},
		{-3.14159, 3, "-3.14"},
	}
	for _, tt := range tests {
		got := FormatSignificant(tt.v, tt.digits)
		if got != tt.want {
			t.Errorf("FormatSignificant(%g,%d) = %q, want %q", tt.v, tt.digits, got, tt.want)
		}
	}
}

func TestFormatSignificant_zero_digits(t *testing.T) {
	got := FormatSignificant(3.14159, 0)
	if got != "3" {
		t.Fatalf("FormatSignificant(3.14159,0) = %q, want %q", got, "3")
	}
}

// --- FormatSI ---

func TestFormatSI(t *testing.T) {
	tests := []struct {
		v    float64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{500, "500"},
		{999, "999"},
		{1000, "1k"},
		{1500, "1.5k"},
		{1000000, "1M"},
		{2500000, "2.5M"},
		{1000000000, "1G"},
		{0.5, "500m"},
		{0.1, "100m"},
		{0.001, "1m"},
		{0.0005, "500µ"},
		{0.000001, "1µ"},
		{0.0000005, "500n"},
		{0.000000001, "1n"},
		{-1000, "-1k"},
		{-0.5, "-500m"},
	}
	for _, tt := range tests {
		got := FormatSI(tt.v)
		if got != tt.want {
			t.Errorf("FormatSI(%g) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestFormatSI_tiny(t *testing.T) {
	got := FormatSI(1e-10)
	// Should fall back to scientific notation
	if got == "" {
		t.Fatal("expected non-empty result for 1e-10")
	}
}

// --- Integration: tickLabels with autoPrecision ---

func TestTickLabels_auto_precision(t *testing.T) {
	tests := []struct {
		name string
		vals []float64
		want []string
	}{
		{"integer step", []float64{0, 20, 40, 60, 80, 100},
			[]string{"0", "20", "40", "60", "80", "100"}},
		{"tenths step", []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
			[]string{"0.0", "0.1", "0.2", "0.3", "0.4", "0.5", "0.6", "0.7", "0.8", "0.9", "1.0"}},
		{"fifths step", []float64{0, 0.2, 0.4, 0.6, 0.8, 1.0},
			[]string{"0.0", "0.2", "0.4", "0.6", "0.8", "1.0"}},
		{"negative domain", []float64{-100, -50, 0, 50, 100},
			[]string{"-100", "-50", "0", "50", "100"}},
		{"hundredths step", []float64{0, 0.02, 0.04, 0.06, 0.08},
			[]string{"0.00", "0.02", "0.04", "0.06", "0.08"}},
		{"single tick", []float64{42}, []string{"42"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := tickLabels(tt.vals)
			if len(labels) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(labels), len(tt.want))
			}
			for i := range labels {
				if labels[i].Value != tt.vals[i] {
					t.Errorf("[%d].Value = %g, want %g", i, labels[i].Value, tt.vals[i])
				}
				if labels[i].Label != tt.want[i] {
					t.Errorf("[%d].Label = %q, want %q", i, labels[i].Label, tt.want[i])
				}
			}
		})
	}
}

func TestTickLabels_nil_input(t *testing.T) {
	if got := tickLabels(nil); got != nil {
		t.Fatal("expected nil for nil input")
	}
}

func TestTickLabels_empty_input(t *testing.T) {
	if got := tickLabels([]float64{}); got != nil {
		t.Fatal("expected nil for empty input")
	}
}

// --- Integration: LinearScale.Ticks produces consistent labels ---

func TestLinear_Ticks_labels_consistent(t *testing.T) {
	// All ticks within a single scale should have the same precision
	s := NewLinear(WithDomain(0, 1), WithRange(0, 500))
	ticks := s.Ticks(10)
	if len(ticks) == 0 {
		t.Fatal("expected ticks")
	}
	// All labels should be non-empty and have the same number of decimal places
	var dp *int
	for _, tk := range ticks {
		if tk.Label == "" {
			t.Fatal("empty label")
		}
		n := 0
		for i := 0; i < len(tk.Label); i++ {
			if tk.Label[i] == '.' {
				n = len(tk.Label) - i - 1
				break
			}
		}
		if dp == nil {
			dp = &n
		} else if n != *dp {
			t.Errorf("inconsistent decimal places: got %d, want %d for label %q", n, *dp, tk.Label)
		}
	}
}

// --- benchmarks ---

func BenchmarkFormatFixed(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		FormatFixed(3.14159, 2)
	}
}

func BenchmarkFormatSI(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		FormatSI(1234567)
	}
}
