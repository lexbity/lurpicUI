package testkit

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
)

func TestDeterministicTime_is_UTC(t *testing.T) {
	dt := DeterministicTime(2026, time.June, 4, 12, 0, 0, 0)
	if dt.Location() != time.UTC {
		t.Fatalf("location = %v, want UTC", dt.Location())
	}
	if dt.Year() != 2026 || dt.Month() != time.June || dt.Day() != 4 {
		t.Fatalf("date = %s", dt)
	}
	if dt.Hour() != 12 || dt.Minute() != 0 {
		t.Fatalf("time = %s", dt)
	}
}

func TestDeterministicTime_nsec_precision(t *testing.T) {
	dt := DeterministicTime(2026, time.January, 1, 0, 0, 0, 123456789)
	if dt.Nanosecond() != 123456789 {
		t.Fatalf("nsec = %d", dt.Nanosecond())
	}
}

func TestDeterministicTime_zero_is_epoch_in_utc(t *testing.T) {
	dt := DeterministicTime(1, time.January, 1, 0, 0, 0, 0)
	if !dt.IsZero() {
		t.Fatal("expected IsZero")
	}
}

func TestAssertDiffers_passes_on_different_surfaces(t *testing.T) {
	a := NewMemorySurface(2, 2)
	if err := a.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	a.Buffer()[0] = 255
	a.Buffer()[3] = 255
	if err := a.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	b := NewMemorySurface(2, 2)
	if err := b.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	b.Buffer()[0] = 128
	b.Buffer()[3] = 255
	if err := b.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	r := &recordingTB{}
	AssertDiffers(r, a, b, "diff_test")
	if len(r.errors) != 0 {
		t.Fatalf("expected pass, got errors: %v", r.errors)
	}
}

func TestAssertDiffers_fails_on_identical_surfaces(t *testing.T) {
	s := NewMemorySurface(1, 1)
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	s.Buffer()[0] = 255
	s.Buffer()[3] = 255
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	r := &recordingTB{}
	AssertDiffers(r, s, s, "identical_test")
	if len(r.errors) == 0 {
		t.Fatal("expected error for identical surfaces")
	}
}

func TestAssertGoldenPair_with_matching_and_different_goldens(t *testing.T) {
	oldDir := goldenBaseDir
	goldenBaseDir = t.TempDir()
	t.Cleanup(func() { goldenBaseDir = oldDir })

	a := NewMemorySurface(1, 1)
	if err := a.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	a.Buffer()[0] = 255
	a.Buffer()[3] = 255
	if err := a.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	b := NewMemorySurface(1, 1)
	if err := b.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	b.Buffer()[0] = 0
	b.Buffer()[3] = 255
	if err := b.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	oldUpdate := *updateGolden
	*updateGolden = true
	t.Cleanup(func() { *updateGolden = oldUpdate })

	AssertGolden(t, a, "AssertGoldenPair_test_default")
	AssertGolden(t, b, "AssertGoldenPair_test_rtl")
	*updateGolden = false

	AssertGoldenPair(t, a, b, "AssertGoldenPair_test")
}

func TestRenderRTLPair_calls_fn_with_both_directions(t *testing.T) {
	var dirs []facet.WritingDirection
	ltr, rtl := RenderRTLPair(t, func(t testing.TB, dir facet.WritingDirection) *MemorySurface {
		dirs = append(dirs, dir)
		return NewMemorySurface(1, 1)
	})
	if ltr == nil || rtl == nil {
		t.Fatal("expected non-nil surfaces")
	}
	if len(dirs) != 2 {
		t.Fatalf("fn called %d times, want 2", len(dirs))
	}
	if dirs[0] != facet.WritingDirectionLTR {
		t.Fatalf("first call dir = %v, want LTR", dirs[0])
	}
	if dirs[1] != facet.WritingDirectionRTL {
		t.Fatalf("second call dir = %v, want RTL", dirs[1])
	}
}
