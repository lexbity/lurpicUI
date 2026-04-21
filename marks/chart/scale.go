package chart

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

// ScaleKind identifies the chart scale family.
type ScaleKind uint8

const (
	ScaleLinear ScaleKind = iota
	ScaleLog
	ScaleBand
	ScaleTime
)

func (k ScaleKind) String() string {
	switch k {
	case ScaleLinear:
		return "linear"
	case ScaleLog:
		return "log"
	case ScaleBand:
		return "band"
	case ScaleTime:
		return "time"
	default:
		return fmt.Sprintf("ScaleKind(%d)", uint8(k))
	}
}

// ScaleModel maps domain values to chart positions.
type ScaleModel interface {
	Kind() ScaleKind
	Map(value any) float32
	Ticks(desired int) []any
	FormatTick(value any) string
}

// LinearScale maps numeric values over a linear range.
type LinearScale struct {
	DomainMin float64
	DomainMax float64
	RangeMin  float32
	RangeMax  float32
	Precision int
}

func (s LinearScale) Kind() ScaleKind { return ScaleLinear }

func (s LinearScale) Map(value any) float32 {
	v, ok := asFloat64(value)
	if !ok {
		return s.RangeMin
	}
	if s.DomainMax == s.DomainMin {
		return s.RangeMin
	}
	p := (v - s.DomainMin) / (s.DomainMax - s.DomainMin)
	return s.RangeMin + float32(p)*(s.RangeMax-s.RangeMin)
}

func (s LinearScale) Ticks(desired int) []any {
	if desired <= 0 {
		desired = 5
	}
	if s.DomainMax == s.DomainMin {
		return []any{s.DomainMin}
	}
	if desired == 1 {
		return []any{s.DomainMin}
	}
	out := make([]any, 0, desired)
	step := (s.DomainMax - s.DomainMin) / float64(desired-1)
	for i := 0; i < desired; i++ {
		out = append(out, s.DomainMin+step*float64(i))
	}
	return out
}

func (s LinearScale) FormatTick(value any) string {
	v, ok := asFloat64(value)
	if !ok {
		return fmt.Sprint(value)
	}
	precision := s.Precision
	if precision < 0 {
		precision = 0
	}
	return strconv.FormatFloat(v, 'f', precision, 64)
}

// LogScale maps numeric values logarithmically.
type LogScale struct {
	DomainMin float64
	DomainMax float64
	RangeMin  float32
	RangeMax  float32
	Base      float64
}

func (s LogScale) Kind() ScaleKind { return ScaleLog }

func (s LogScale) Map(value any) float32 {
	v, ok := asFloat64(value)
	if !ok || v <= 0 || s.DomainMin <= 0 || s.DomainMax <= 0 || s.DomainMax == s.DomainMin {
		return s.RangeMin
	}
	base := s.Base
	if base <= 1 {
		base = 10
	}
	min := math.Log(s.DomainMin) / math.Log(base)
	max := math.Log(s.DomainMax) / math.Log(base)
	cur := math.Log(v) / math.Log(base)
	p := (cur - min) / (max - min)
	return s.RangeMin + float32(p)*(s.RangeMax-s.RangeMin)
}

func (s LogScale) Ticks(desired int) []any {
	if desired <= 0 {
		desired = 5
	}
	if desired == 1 {
		return []any{s.DomainMin}
	}
	if s.DomainMin <= 0 || s.DomainMax <= 0 {
		return nil
	}
	base := s.Base
	if base <= 1 {
		base = 10
	}
	minExp := math.Floor(math.Log(s.DomainMin) / math.Log(base))
	maxExp := math.Ceil(math.Log(s.DomainMax) / math.Log(base))
	var out []any
	for exp := minExp; exp <= maxExp; exp++ {
		v := math.Pow(base, exp)
		if v >= s.DomainMin && v <= s.DomainMax {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		out = append(out, s.DomainMin, s.DomainMax)
	}
	return out
}

func (s LogScale) FormatTick(value any) string {
	v, ok := asFloat64(value)
	if !ok {
		return fmt.Sprint(value)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// BandScale maps categorical values to evenly spaced slots.
type BandScale struct {
	Categories []string
	RangeMin   float32
	RangeMax   float32
}

func (s BandScale) Kind() ScaleKind { return ScaleBand }

func (s BandScale) Map(value any) float32 {
	key := fmt.Sprint(value)
	if len(s.Categories) == 0 {
		return s.RangeMin
	}
	step := (s.RangeMax - s.RangeMin) / float32(len(s.Categories))
	for i, category := range s.Categories {
		if category == key {
			return s.RangeMin + step*float32(i) + step/2
		}
	}
	return s.RangeMin
}

func (s BandScale) Ticks(desired int) []any {
	out := make([]any, 0, len(s.Categories))
	for _, category := range s.Categories {
		out = append(out, category)
	}
	return out
}

func (s BandScale) FormatTick(value any) string {
	return fmt.Sprint(value)
}

// TimeScale maps time values to a linear range.
type TimeScale struct {
	DomainStart time.Time
	DomainEnd   time.Time
	RangeMin    float32
	RangeMax    float32
}

func (s TimeScale) Kind() ScaleKind { return ScaleTime }

func (s TimeScale) Map(value any) float32 {
	var t time.Time
	switch v := value.(type) {
	case time.Time:
		t = v
	case string:
		if parsed, err := time.Parse(time.RFC3339, v); err == nil {
			t = parsed
		} else {
			return s.RangeMin
		}
	default:
		return s.RangeMin
	}
	if s.DomainEnd.Equal(s.DomainStart) {
		return s.RangeMin
	}
	p := t.Sub(s.DomainStart).Seconds() / s.DomainEnd.Sub(s.DomainStart).Seconds()
	return s.RangeMin + float32(p)*(s.RangeMax-s.RangeMin)
}

func (s TimeScale) Ticks(desired int) []any {
	if desired <= 0 {
		desired = 5
	}
	if s.DomainEnd.Equal(s.DomainStart) {
		return []any{s.DomainStart}
	}
	if desired == 1 {
		return []any{s.DomainStart}
	}
	out := make([]any, 0, desired)
	step := s.DomainEnd.Sub(s.DomainStart) / time.Duration(desired-1)
	for i := 0; i < desired; i++ {
		out = append(out, s.DomainStart.Add(step*time.Duration(i)))
	}
	return out
}

func (s TimeScale) FormatTick(value any) string {
	switch v := value.(type) {
	case time.Time:
		return v.Format("2006-01-02")
	default:
		return fmt.Sprint(value)
	}
}

func asFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint64:
		return float64(v), true
	default:
		return 0, false
	}
}
