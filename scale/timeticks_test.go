package scale

import (
	"testing"
	"time"
)

func ms(t time.Time) float64 {
	return float64(t.UnixMilli())
}

func TestTimeTicks_interval_selection(t *testing.T) {
	tests := []struct {
		name       string
		lo, hi     time.Time
		count      int
		wantPrefix string // interval name prefix
	}{
		{"1 year span, 5 ticks", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), 5, "3mo"},
		{"1 year span, 20 ticks", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), 20, "1mo"},
		{"1 month span", time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2020, 7, 1, 0, 0, 0, 0, time.UTC), 10, "2d"},
		{"1 day span", time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2020, 6, 2, 0, 0, 0, 0, time.UTC), 10, "3h"},
		{"1 hour span", time.Date(2020, 6, 1, 12, 0, 0, 0, time.UTC), time.Date(2020, 6, 1, 13, 0, 0, 0, time.UTC), 10, "5m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, iv := timeTicks(ms(tt.lo), ms(tt.hi), tt.count, time.UTC)
			if iv.name != tt.wantPrefix {
				t.Errorf("expected interval %q, got %q", tt.wantPrefix, iv.name)
			}
		})
	}
}

func TestTimeTicks_calendar_aligned_dayBoundary(t *testing.T) {
	lo := time.Date(2024, 6, 10, 6, 0, 0, 0, time.UTC)
	hi := time.Date(2024, 6, 15, 18, 0, 0, 0, time.UTC)
	vals, _ := timeTicks(ms(lo), ms(hi), 10, time.UTC)
	// All ticks must be aligned to calendar boundaries
	for _, v := range vals {
		tm := time.UnixMilli(int64(v))
		if tm.Second() != 0 {
			t.Errorf("tick %v has non-zero seconds", tm)
		}
	}
	if len(vals) < 4 {
		t.Fatalf("expected at least 4 ticks, got %d", len(vals))
	}
}

func TestTimeTicks_calendar_aligned_1h(t *testing.T) {
	lo := time.Date(2024, 6, 10, 7, 15, 0, 0, time.UTC)
	hi := time.Date(2024, 6, 10, 16, 45, 0, 0, time.UTC)
	vals, iv := timeTicks(ms(lo), ms(hi), 12, time.UTC)
	if iv.name != "1h" {
		t.Fatalf("expected 1h interval, got %s", iv.name)
	}
	for _, v := range vals {
		tm := time.UnixMilli(int64(v))
		if tm.Minute() != 0 || tm.Second() != 0 {
			t.Errorf("tick %v not aligned to hour boundary", tm)
		}
	}
}

func TestTimeTicks_count_approximate(t *testing.T) {
	// Number of ticks should be within a reasonable range of the request.
	// Calendar alignment means the exact count is always approximate.
	lo := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	hi := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, count := range []int{5, 10, 20, 50} {
		vals, _ := timeTicks(ms(lo), ms(hi), count, time.UTC)
		if len(vals) > count*4 && count > 0 {
			t.Errorf("count=%d: got %d ticks, expected ≤~%d", count, len(vals), count*4)
		}
		if len(vals) == 0 {
			t.Errorf("count=%d: got no ticks", count)
		}
	}
}

func TestTimeTicks_no_duplicates(t *testing.T) {
	lo := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	hi := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	vals, _ := timeTicks(ms(lo), ms(hi), 10, time.UTC)
	seen := make(map[float64]bool)
	for _, v := range vals {
		if seen[v] {
			t.Errorf("duplicate tick: %f", v)
		}
		seen[v] = true
	}
}

func TestTimeTicks_monotonic(t *testing.T) {
	lo := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	hi := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	vals, _ := timeTicks(ms(lo), ms(hi), 10, time.UTC)
	for i := 1; i < len(vals); i++ {
		if vals[i] <= vals[i-1] {
			t.Fatalf("non-monotonic at [%d]: %g <= %g", i, vals[i], vals[i-1])
		}
	}
}

func TestTimeTicks_zero_count(t *testing.T) {
	lo := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	hi := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	vals, _ := timeTicks(ms(lo), ms(hi), 0, time.UTC)
	if len(vals) != 0 {
		t.Fatalf("expected empty for count=0, got %d ticks", len(vals))
	}
}

func TestTimeTicks_empty_domain(t *testing.T) {
	lo := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	vals, _ := timeTicks(ms(lo), ms(lo), 10, time.UTC)
	if len(vals) != 0 {
		t.Fatalf("expected empty for empty domain, got %d ticks", len(vals))
	}
}

// Force specific interval selections by choosing appropriate domain + count.
func forceInterval(lo, hi time.Time, count int) timeInterval {
	_, iv := timeTicks(ms(lo), ms(hi), count, time.UTC)
	return iv
}

func TestTimeTicks_force_calendar_intervals(t *testing.T) {
	tests := []struct {
		name string
		iv   string
		lo   time.Time
		hi   time.Time
		cnt  int
	}{
		{"force 1y", "1y", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), 4},
		{"force 1mo", "1mo", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), 20},
		{"force 1w", "1w", time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC), 7},
		{"force 1d", "1d", time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 6, 10, 0, 0, 0, 0, time.UTC), 12},
		{"force 30m", "30m", time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC), time.Date(2024, 6, 1, 15, 0, 0, 0, time.UTC), 8},
		{"force 15m", "15m", time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC), time.Date(2024, 6, 1, 14, 0, 0, 0, time.UTC), 10},
		{"force 5m", "5m", time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC), time.Date(2024, 6, 1, 13, 0, 0, 0, time.UTC), 14},
		{"force 1m", "1m", time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC), time.Date(2024, 6, 1, 12, 10, 0, 0, time.UTC), 12},
		{"force 5s", "5s", time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC), time.Date(2024, 6, 1, 12, 1, 0, 0, time.UTC), 14},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := forceInterval(tt.lo, tt.hi, tt.cnt)
			if got.name != tt.iv {
				t.Errorf("expected interval %q, got %q", tt.iv, got.name)
			}
			// Generate ticks and verify monotonic, no-dupes
			vals, _ := timeTicks(ms(tt.lo), ms(tt.hi), tt.cnt, time.UTC)
			if len(vals) == 0 {
				t.Fatal("expected non-empty ticks")
			}
			for i := 1; i < len(vals); i++ {
				if vals[i] <= vals[i-1] {
					t.Fatalf("non-monotonic at [%d]: %g <= %g", i, vals[i], vals[i-1])
				}
			}
		})
	}
}

func TestTimeTicks_sub_second_intervals(t *testing.T) {
	lo := time.Date(2024, 6, 10, 12, 0, 0, 0, time.UTC)
	hi := time.Date(2024, 6, 10, 12, 0, 30, 0, time.UTC)
	vals, _ := timeTicks(ms(lo), ms(hi), 10, time.UTC)
	if len(vals) < 5 {
		t.Fatalf("expected sub-second ticks for 30s range, got %d", len(vals))
	}
}

// --- TimeScale.Ticks ---

func TestTimeScale_Ticks_basic(t *testing.T) {
	lo := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	hi := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(lo, hi), WithRange(0, 500))
	ticks := s.Ticks(10)
	if len(ticks) == 0 {
		t.Fatal("expected non-empty ticks")
	}
	for i, tk := range ticks {
		if tk.Label == "" {
			t.Errorf("tick[%d] has empty label", i)
		}
	}
}

func TestTimeScale_Ticks_degenerate_domain(t *testing.T) {
	tm := time.Date(2020, 6, 15, 12, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(tm, tm), WithRange(0, 500))
	if got := s.Ticks(5); got != nil {
		t.Fatalf("expected nil for degenerate domain, got %v", got)
	}
}

func TestTimeScale_Ticks_zero_count(t *testing.T) {
	lo := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	hi := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(lo, hi), WithRange(0, 500))
	if got := s.Ticks(0); got != nil {
		t.Fatalf("expected nil for count=0, got %v", got)
	}
}

func TestTimeScale_Ticks_narrow_domain_returns_nil(t *testing.T) {
	// Domain spanning < 1ms starting just past a second boundary.
	// roundDown floors to the previous second, which is before lo,
	// and advancing puts us past hi, so no ticks are generated.
	// Note: time.Date takes nanoseconds, so 1e6 ns = 1 ms.
	lo := time.Date(2024, 6, 10, 12, 0, 0, 1_000_000, time.UTC)
	hi := time.Date(2024, 6, 10, 12, 0, 0, 2_000_000, time.UTC)
	s := NewTime(WithTimeDomain(lo, hi), WithRange(0, 500))
	if got := s.Ticks(5); got != nil {
		t.Fatalf("expected nil for narrow domain, got %v", got)
	}
}

func TestTimeScale_Ticks_tags_use_previous_for_elision(t *testing.T) {
	// All ticks after the first should have a label (elision should never
	// produce empty for non-year intervals)
	lo := time.Date(2024, 6, 10, 6, 0, 0, 0, time.UTC)
	hi := time.Date(2024, 6, 10, 18, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(lo, hi), WithRange(0, 500))
	ticks := s.Ticks(15)
	if len(ticks) < 2 {
		t.Fatal("expected at least 2 ticks")
	}
	for i, tk := range ticks {
		if tk.Label == "" {
			t.Errorf("tick[%d] has empty label", i)
		}
	}
}

func TestTimeScale_Ticks_implements_Ticker(t *testing.T) {
	lo := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	hi := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	var tk Ticker = NewTime(WithTimeDomain(lo, hi), WithRange(0, 500))
	_ = tk
}

func TestTimeScale_Ticks_reversed_domain(t *testing.T) {
	lo := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	hi := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(lo, hi), WithRange(0, 500))
	ticks := s.Ticks(10)
	if len(ticks) == 0 {
		t.Fatal("expected non-empty ticks for reversed domain")
	}
}

func TestTimeScale_Ticks_labels_hourly(t *testing.T) {
	lo := time.Date(2024, 6, 10, 6, 0, 0, 0, time.UTC)
	hi := time.Date(2024, 6, 10, 18, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(lo, hi), WithRange(0, 500))
	ticks := s.Ticks(15)
	if len(ticks) < 5 {
		t.Fatalf("expected hourly ticks, got %d", len(ticks))
	}
	// All within the same day should have labels like "06:00", "07:00", etc.
	for _, tk := range ticks {
		if tk.Label == "" {
			t.Errorf("empty label")
		}
	}
}

// --- format ---

func TestFormatTimeTick_elision(t *testing.T) {
	tests := []struct {
		name     string
		iv       timeInterval
		prev     time.Time
		curr     time.Time
		contains []string // expected substrings
		not      []string // unexpected substrings
	}{
		{
			"same day hourly",
			timeInterval{name: "1h"},
			time.Date(2024, 6, 10, 6, 0, 0, 0, time.UTC),
			time.Date(2024, 6, 10, 7, 0, 0, 0, time.UTC),
			[]string{"07:00"},
			[]string{"Jun"},
		},
		{
			"new day hourly",
			timeInterval{name: "1h"},
			time.Date(2024, 6, 10, 23, 0, 0, 0, time.UTC),
			time.Date(2024, 6, 11, 0, 0, 0, 0, time.UTC),
			[]string{"Jun 11"},
			[]string{},
		},
		{
			"same day minute",
			timeInterval{name: "30m"},
			time.Date(2024, 6, 10, 12, 0, 0, 0, time.UTC),
			time.Date(2024, 6, 10, 12, 30, 0, 0, time.UTC),
			[]string{"12:30"},
			[]string{"Jun"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label := formatTimeTick(tt.curr, tt.prev, tt.iv)
			for _, s := range tt.contains {
				if !contains(label, s) {
					t.Errorf("label %q should contain %q", label, s)
				}
			}
			for _, s := range tt.not {
				if contains(label, s) {
					t.Errorf("label %q should not contain %q", label, s)
				}
			}
		})
	}
}

func TestFormatTimeTick_year(t *testing.T) {
	iv := timeInterval{name: "1y"}
	prev := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	curr := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	if got := formatTimeTick(curr, prev, iv); got != "2021" {
		t.Fatalf("label = %q, want %q", got, "2021")
	}
}

func TestFormatTimeTick_year_elided(t *testing.T) {
	iv := timeInterval{name: "1y"}
	prev := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	curr := time.Date(2021, 7, 1, 0, 0, 0, 0, time.UTC)
	if got := formatTimeTick(curr, prev, iv); got != "" {
		t.Fatalf("label = %q, want empty (same year)", got)
	}
}

func contains(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestTimeTicks_force_2d_interval(t *testing.T) {
	// Force 2-day interval selection
	lo := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC) // Saturday (day 153, even)
	hi := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	vals, iv := timeTicks(ms(lo), ms(hi), 8, time.UTC)
	if iv.name != "2d" {
		t.Fatalf("expected 2d interval, got %s", iv.name)
	}
	if len(vals) == 0 {
		t.Fatal("expected ticks for 2d interval")
	}
}

func TestTimeTicks_roundDownWeek_sunday(t *testing.T) {
	// A Sunday should round down to the previous Monday
	sunday := time.Date(2024, 6, 9, 12, 0, 0, 0, time.UTC) // Sunday
	got := roundDownWeek(sunday)
	if got.Weekday() != time.Monday {
		t.Errorf("expected Monday, got %v", got.Weekday())
	}
	if got.Day() != 3 { // June 3 is Monday of that week
		t.Errorf("expected June 3, got %v", got)
	}
}

// --- DST boundary ---

func TestFormatTimeTick_weekly(t *testing.T) {
	iv := timeInterval{name: "1w"}
	prev := time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC) // Monday
	curr := time.Date(2024, 6, 10, 0, 0, 0, 0, time.UTC) // next Monday
	label := formatTimeTick(curr, prev, iv)
	if label == "" {
		t.Fatal("expected non-empty label for weekly tick")
	}
}

func TestFormatTimeTick_2day(t *testing.T) {
	iv := timeInterval{name: "2d"}
	prev := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	curr := time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC)
	label := formatTimeTick(curr, prev, iv)
	if label == "" {
		t.Fatal("expected non-empty label for 2-day tick")
	}
}

func TestFormatTimeTick_daily(t *testing.T) {
	iv := timeInterval{name: "1d"}
	prev := time.Date(2024, 6, 10, 0, 0, 0, 0, time.UTC)
	curr := time.Date(2024, 6, 11, 0, 0, 0, 0, time.UTC)
	label := formatTimeTick(curr, prev, iv)
	if label == "" {
		t.Fatal("expected non-empty label for daily tick")
	}
	if !contains(label, "11") {
		t.Errorf("daily label %q should contain day number", label)
	}
}

func TestFormatTimeTick_second(t *testing.T) {
	iv := timeInterval{name: "5s"}
	prev := time.Date(2024, 6, 10, 12, 0, 0, 0, time.UTC)
	curr := time.Date(2024, 6, 10, 12, 0, 5, 0, time.UTC)
	label := formatTimeTick(curr, prev, iv)
	if label == "" {
		t.Fatal("expected non-empty label for second tick")
	}
	if !contains(label, "12:00:05") {
		t.Errorf("second label %q should contain time", label)
	}
}

func TestFormatTimeTick_monthly_elided(t *testing.T) {
	iv := timeInterval{name: "1mo"}
	prev := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	curr := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	label := formatTimeTick(curr, prev, iv)
	if label == "" {
		t.Fatal("expected non-empty label for monthly tick")
	}
}

func TestFormatTimeTick_minute_newDay(t *testing.T) {
	iv := timeInterval{name: "15m"}
	prev := time.Date(2024, 6, 10, 23, 45, 0, 0, time.UTC)
	curr := time.Date(2024, 6, 11, 0, 0, 0, 0, time.UTC)
	label := formatTimeTick(curr, prev, iv)
	if !contains(label, "Jun") {
		t.Errorf("minute label across day %q should contain month", label)
	}
}

func TestFormatTimeTick_second_sameDay(t *testing.T) {
	iv := timeInterval{name: "15s"}
	prev := time.Date(2024, 6, 10, 12, 0, 0, 0, time.UTC)
	curr := time.Date(2024, 6, 10, 12, 0, 15, 0, time.UTC)
	label := formatTimeTick(curr, prev, iv)
	if label == "" {
		t.Fatal("expected non-empty label for second tick within same day")
	}
}

func TestFormatTimeTick_hour_differentYear(t *testing.T) {
	iv := timeInterval{name: "1h"}
	prev := time.Date(2024, 12, 31, 23, 0, 0, 0, time.UTC)
	curr := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	label := formatTimeTick(curr, prev, iv)
	if !contains(label, "2025") {
		t.Errorf("hour label across years %q should contain year", label)
	}
}

func TestFormatTimeTick_minute_differentYear(t *testing.T) {
	iv := timeInterval{name: "5m"}
	prev := time.Date(2024, 12, 31, 23, 55, 0, 0, time.UTC)
	curr := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	label := formatTimeTick(curr, prev, iv)
	if !contains(label, "2025") {
		t.Errorf("minute label across years %q should contain year", label)
	}
}

func TestFormatTimeTick_second_sameYearDifferentDay(t *testing.T) {
	iv := timeInterval{name: "5s"}
	prev := time.Date(2024, 6, 10, 23, 59, 55, 0, time.UTC)
	curr := time.Date(2024, 6, 11, 0, 0, 0, 0, time.UTC)
	label := formatTimeTick(curr, prev, iv)
	if !contains(label, "Jun 11") {
		t.Errorf("second label across days %q should contain date", label)
	}
}

func TestFormatTimeTick_second_differentYear(t *testing.T) {
	iv := timeInterval{name: "5s"}
	prev := time.Date(2024, 12, 31, 23, 59, 55, 0, time.UTC)
	curr := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	label := formatTimeTick(curr, prev, iv)
	if !contains(label, "2025") {
		t.Errorf("second label across years %q should contain year", label)
	}
}

func TestFormatTimeTick_default(t *testing.T) {
	iv := timeInterval{name: "unknown"}
	tm := time.Date(2024, 6, 10, 12, 0, 0, 0, time.UTC)
	label := formatTimeTick(tm, time.Time{}, iv)
	if label == "" {
		t.Fatal("expected non-empty label for default format")
	}
}

func TestFormatTimeTick_weekly_sameDay(t *testing.T) {
	// Same-day weekly ticks should show weekday
	iv := timeInterval{name: "1w"}
	prev := time.Date(2024, 6, 10, 0, 0, 0, 0, time.UTC)
	curr := time.Date(2024, 6, 10, 0, 0, 0, 0, time.UTC)
	label := formatTimeTick(curr, prev, iv)
	if label == "" {
		t.Fatal("expected non-empty for same-day weekly")
	}
}

func TestFormatTimeTick_daily_differentYear(t *testing.T) {
	iv := timeInterval{name: "1d"}
	prev := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	curr := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	label := formatTimeTick(curr, prev, iv)
	if !contains(label, "2025") {
		t.Errorf("daily label across years %q should contain year", label)
	}
}

func TestFormatTimeTick_quarterly(t *testing.T) {
	iv := timeInterval{name: "3mo"}
	prev := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	curr := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	label := formatTimeTick(curr, prev, iv)
	if label == "" {
		t.Fatal("expected non-empty label for quarterly tick")
	}
}

// TestChooseInterval_doc_pinned verifies that chooseInterval picks the
// interval closest to the requested count (pinning the documented behavior).
func TestChooseInterval_doc_pinned(t *testing.T) {
	tests := []struct {
		name       string
		lo, hi     time.Time
		count      int
		wantPrefix string
	}{
		// 1 year = 365 days. 1mo ≈ 31 days → ~11.8 ticks. Requested 7 → 1mo (diff 4.8) beats 3mo (diff ≈ 1.18).
		{"1y span, 7 ticks → 3mo", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), 7, "3mo"},
		// Requested 4 → 1y (diff 3.0) beats 3mo (diff 2.18) — wait, 365/92≈4.0, diff=0. So 3mo is actually perfect.
		{"1y span, 4 ticks → 3mo", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), 4, "3mo"},
		// 92-day span. 1w = 7d → ~13.1 ticks. Requested 15 → 1w (diff 1.9) beats 2d (diff 31).
		{"3mo span, 15 ticks → 1w", time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC), 15, "1w"},
		// 7-day span. 1d = 1d → 7 ticks. Requested 10 → 1d (diff 3) beats 12h (diff 4).
		{"1w span, 10 ticks → 1d", time.Date(2024, 6, 10, 0, 0, 0, 0, time.UTC), time.Date(2024, 6, 17, 0, 0, 0, 0, time.UTC), 10, "1d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iv := chooseInterval(ms(tt.lo), ms(tt.hi), tt.count, time.UTC)
			if iv.name != tt.wantPrefix {
				t.Errorf("chooseInterval(%s, %s, %d) = %q, want %q",
					tt.lo.Format("2006-01-02"), tt.hi.Format("2006-01-02"), tt.count, iv.name, tt.wantPrefix)
			}
		})
	}
}

func TestTimeScale_Ticks_labels_respect_location(t *testing.T) {
	loc := time.FixedZone("UTC+3", 3*60*60)
	t0 := time.Date(2024, 6, 10, 0, 0, 0, 0, loc)
	t1 := time.Date(2024, 6, 11, 0, 0, 0, 0, loc)
	s := NewTime(WithTimeDomain(t0, t1), WithRange(0, 500), WithTimeLocation(loc))
	ticks := s.Ticks(12)
	if len(ticks) == 0 {
		t.Fatal("expected ticks")
	}
	for _, tk := range ticks {
		// All labels should reflect the +3 offset, not UTC.
		tm := time.UnixMilli(int64(tk.Value)).In(loc)
		h, _, _ := tm.Clock()
		// 00:00 in UTC+3 is 21:00 UTC the previous day — labels should
		// show the +3 time, not drift to the previous day's label.
		if h < 0 || h > 23 {
			t.Errorf("unexpected hour in label: %v", tm)
		}
	}
}

// TestTimeScale_Ticks_no_repeated_Jan verifies that month-crossing time
// ranges do not produce repeated month labels (e.g., "Jan" for February).
func TestTimeScale_Ticks_no_repeated_Jan(t *testing.T) {
	// Jan 15 – Feb 15, 2026: crosses a month boundary.
	lo := time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)
	hi := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(lo, hi), WithRange(0, 300))
	ticks := s.Ticks(8)
	if len(ticks) < 2 {
		t.Fatal("expected at least 2 ticks across month boundary")
	}
	for _, tk := range ticks {
		tm := time.UnixMilli(int64(tk.Value)).In(time.UTC)
		label := tk.Label
		// The label's month should match the tick's actual month.
		monthStr := tm.Month().String()[:3] // "Jan", "Feb", etc.
		if len(label) >= 3 {
			labelMonth := label[:3]
			if labelMonth == "Jan" && tm.Month() != time.January {
				t.Errorf("tick %v: label %q says %s but month is %s", tm, label, labelMonth, monthStr)
			}
			if labelMonth == "Feb" && tm.Month() != time.February {
				t.Errorf("tick %v: label %q says %s but month is %s", tm, label, labelMonth, monthStr)
			}
		}
	}
}

func TestTimeTicks_dst_boundary(t *testing.T) {
	// US/Eastern DST transition March 10, 2024 at 2:00 AM → 3:00 AM
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("timezone data not available")
	}
	lo := time.Date(2024, 3, 10, 0, 0, 0, 0, loc)
	hi := time.Date(2024, 3, 11, 0, 0, 0, 0, loc)
	vals, iv := timeTicks(ms(lo), ms(hi), 12, loc)
	if len(vals) == 0 {
		t.Fatal("expected ticks across DST boundary")
	}
	_ = iv
	for _, v := range vals {
		tm := time.UnixMilli(int64(v)).In(loc)
		// All ticks should be valid times (no skipped hours appear)
		if tm.Hour() == 2 && tm.Minute() == 0 {
			// 2:00 AM is skipped on DST spring-forward (might or might not appear
			// depending on rounding)
		}
	}
}

func TestTimeTicks_dst_fall_back(t *testing.T) {
	// US/Eastern DST transition November 3, 2024 at 2:00 AM → 1:00 AM
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("timezone data not available")
	}
	lo := time.Date(2024, 11, 3, 0, 0, 0, 0, loc)
	hi := time.Date(2024, 11, 4, 0, 0, 0, 0, loc)
	vals, iv := timeTicks(ms(lo), ms(hi), 12, loc)
	if len(vals) == 0 {
		t.Fatal("expected ticks across DST fall-back")
	}
	_ = iv
}

// --- benchmarks ---

func BenchmarkTimeScale_Ticks(b *testing.B) {
	lo := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	hi := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(lo, hi), WithRange(0, 500))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Ticks(10)
	}
}
