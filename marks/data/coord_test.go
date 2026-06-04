package data

import (
	"math"
	"testing"
)

func TestPt_narrows_float64_to_float32(t *testing.T) {
	p := Pt(3.141592653589793, 2.718281828459045)
	if p.X != float32(3.141592653589793) {
		t.Fatalf("X = %v, want %v", p.X, float32(3.141592653589793))
	}
	if p.Y != float32(2.718281828459045) {
		t.Fatalf("Y = %v, want %v", p.Y, float32(2.718281828459045))
	}
}

func TestPt_zero(t *testing.T) {
	p := Pt(0, 0)
	if p.X != 0 || p.Y != 0 {
		t.Fatalf("zero = %#v, want {0,0}", p)
	}
}

func TestPt_negative(t *testing.T) {
	p := Pt(-100.5, -200.25)
	if p.X != -100.5 {
		t.Fatalf("X = %v, want -100.5", p.X)
	}
	if p.Y != -200.25 {
		t.Fatalf("Y = %v, want -200.25", p.Y)
	}
}

func TestPt_large_values(t *testing.T) {
	p := Pt(1e38, -1e38)
	if p.X == 0 || p.Y == 0 {
		t.Fatal("large values should not truncate to zero")
	}
}

func TestPt_nan(t *testing.T) {
	p := Pt(math.NaN(), math.NaN())
	if !math.IsNaN(float64(p.X)) || !math.IsNaN(float64(p.Y)) {
		t.Fatal("expected NaN to propagate")
	}
}

func TestPt_infinity(t *testing.T) {
	p := Pt(math.Inf(1), math.Inf(-1))
	if !math.IsInf(float64(p.X), 1) {
		t.Fatal("expected +Inf for X")
	}
	if !math.IsInf(float64(p.Y), -1) {
		t.Fatal("expected -Inf for Y")
	}
}

func TestPt_integer_values(t *testing.T) {
	p := Pt(42, 99)
	if p.X != 42 || p.Y != 99 {
		t.Fatalf("got %#v, want {42,99}", p)
	}
}
