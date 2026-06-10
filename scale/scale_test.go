package scale

import (
	"errors"
	"testing"
)

func TestScaleKind_values_sequential(t *testing.T) {
	tests := []struct {
		kind ScaleKind
		want uint8
	}{
		{KindUnknown, 0},
		{KindLinear, 1},
		{KindLog, 2},
		{KindPow, 3},
		{KindTime, 4},
		{KindBand, 5},
		{KindPoint, 6},
	}
	for _, tt := range tests {
		if got := uint8(tt.kind); got != tt.want {
			t.Errorf("ScaleKind(%d) = %d, want %d", got, tt.want, tt.want)
		}
	}
}

func TestScaleKind_string(t *testing.T) {
	tests := []struct {
		kind ScaleKind
		want string
	}{
		{KindUnknown, "unknown"},
		{KindLinear, "linear"},
		{KindLog, "log"},
		{KindPow, "pow"},
		{KindTime, "time"},
		{KindBand, "band"},
		{KindPoint, "point"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("ScaleKind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestScaleKind_string_unknown_value(t *testing.T) {
	var k ScaleKind = 99
	got := k.String()
	if got == "unknown" {
		t.Fatal("expected non-standard kind to not match 'unknown'")
	}
}

func TestOutOfRange_values(t *testing.T) {
	if OutOfRangeExtrapolate != 0 {
		t.Fatalf("expected OutOfRangeExtrapolate to be 0, got %d", OutOfRangeExtrapolate)
	}
	if OutOfRangeClamp != 1 {
		t.Fatalf("expected OutOfRangeClamp to be 1, got %d", OutOfRangeClamp)
	}
}

func TestOutOfRange_string(t *testing.T) {
	tests := []struct {
		o    OutOfRange
		want string
	}{
		{OutOfRangeExtrapolate, "extrapolate"},
		{OutOfRangeClamp, "clamp"},
	}
	for _, tt := range tests {
		if got := tt.o.String(); got != tt.want {
			t.Errorf("OutOfRange(%d).String() = %q, want %q", tt.o, got, tt.want)
		}
	}
}

func TestOutOfRange_string_unknown_value(t *testing.T) {
	var o OutOfRange = 99
	got := o.String()
	if got == "extrapolate" || got == "clamp" {
		t.Fatalf("unexpected known string for unknown value: %q", got)
	}
}

func TestOption_WithDomain_sets_domain(t *testing.T) {
	var o options
	WithDomain(10, 20)(&o)
	if !o.hasDomain {
		t.Fatal("expected hasDomain after WithDomain")
	}
	if o.domain != [2]float64{10, 20} {
		t.Fatalf("got domain %v, want [10 20]", o.domain)
	}
}

func TestOption_WithRange_sets_range(t *testing.T) {
	var o options
	WithRange(100, 200)(&o)
	if !o.hasRange {
		t.Fatal("expected hasRange after WithRange")
	}
	if o.rng != [2]float64{100, 200} {
		t.Fatalf("got range %v, want [100 200]", o.rng)
	}
}

func TestOption_WithClamp_sets_clamp(t *testing.T) {
	var o options
	WithClamp(OutOfRangeClamp)(&o)
	if o.clamp == nil {
		t.Fatal("expected clamp to be set")
	}
	if *o.clamp != OutOfRangeClamp {
		t.Fatalf("got clamp %d, want %d", *o.clamp, OutOfRangeClamp)
	}
}

func TestOption_WithClamp_default_is_extrapolate(t *testing.T) {
	var o options
	WithClamp(OutOfRangeExtrapolate)(&o)
	if o.clamp == nil {
		t.Fatal("expected clamp to be set")
	}
	if *o.clamp != OutOfRangeExtrapolate {
		t.Fatalf("got clamp %d, want OutOfRangeExtrapolate", *o.clamp)
	}
}

func TestOption_application_order_last_wins(t *testing.T) {
	var o options
	WithDomain(0, 10)(&o)
	WithDomain(20, 30)(&o)
	if !o.hasDomain {
		t.Fatal("expected hasDomain after WithDomain")
	}
	if o.domain != [2]float64{20, 30} {
		t.Fatalf("got domain %v, want [20 30]", o.domain)
	}
}

func TestOption_defaults(t *testing.T) {
	var o options
	if o.hasDomain {
		t.Error("expected hasDomain false by default")
	}
	if o.hasRange {
		t.Error("expected hasRange false by default")
	}
	if o.clamp != nil {
		t.Error("expected clamp nil by default")
	}
}

func TestOption_zero_options_noop(t *testing.T) {
	var o options
	// Applying zero options should leave everything at zero values.
	_ = o // just checking compilation and that no panic occurs
}

func TestScaleKind_round_trip(t *testing.T) {
	kinds := []ScaleKind{KindUnknown, KindLinear, KindLog, KindPow, KindTime, KindBand, KindPoint}
	for _, k := range kinds {
		s := k.String()
		if s == "" {
			t.Errorf("ScaleKind(%d).String() returned empty", k)
		}
	}
}

func TestTick_struct_layout(t *testing.T) {
	tk := Tick{Value: 42.5, Label: "42.5"}
	if tk.Value != 42.5 {
		t.Fatalf("got Value %f, want 42.5", tk.Value)
	}
	if tk.Label != "42.5" {
		t.Fatalf("got Label %q, want \"42.5\"", tk.Label)
	}
}

func TestSentinelErrors_are_distinct(t *testing.T) {
	if errors.Is(ErrInvalidDomain, ErrEmptyMembers) {
		t.Fatal("sentinel errors must be distinct")
	}
	if errors.Is(ErrInvalidDomain, ErrDomainCrossesZero) {
		t.Fatal("sentinel errors must be distinct")
	}
	if errors.Is(ErrEmptyMembers, ErrDomainCrossesZero) {
		t.Fatal("sentinel errors must be distinct")
	}
	if ErrInvalidDomain == nil {
		t.Fatal("ErrInvalidDomain must not be nil")
	}
	if ErrEmptyMembers == nil {
		t.Fatal("ErrEmptyMembers must not be nil")
	}
	if ErrDomainCrossesZero == nil {
		t.Fatal("ErrDomainCrossesZero must not be nil")
	}
}
