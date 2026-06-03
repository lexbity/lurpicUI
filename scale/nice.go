package scale

import (
	"math"
)

const maxTicks = 1000

// tickStep computes a human-friendly step size for generating approximately
// count ticks spanning [lo, hi]. The step mantissa is snapped to {1, 2, 5}
// multiplied by a power of 10, matching d3-scale behavior.
// Returns 0 for degenerate spans or non-positive counts.
func tickStep(lo, hi float64, count int) float64 {
	if count <= 0 {
		return 0
	}
	span := hi - lo
	if math.IsNaN(span) {
		return math.NaN()
	}
	span = math.Abs(span)
	if span <= 0 {
		return 0
	}
	step0 := span / float64(count)
	mag := math.Pow10(int(math.Floor(math.Log10(step0))))
	error := step0 / mag

	var step float64
	switch {
	case error >= math.Sqrt(50):
		step = 10 * mag
	case error >= math.Sqrt(10):
		step = 5 * mag
	case error >= math.Sqrt(2):
		step = 2 * mag
	default:
		step = 1 * mag
	}
	return step
}

// ticks generates approximately count tick values spanning [lo, hi] using
// the 1/2/5 mantissa step algorithm. The returned slice contains no NaN or
// Inf values and no duplicates. Degenerate inputs and unreasonably large
// tick counts return nil.
func ticks(lo, hi float64, count int) []float64 {
	if lo > hi {
		lo, hi = hi, lo
	}
	step := tickStep(lo, hi, count)
	if step == 0 {
		return nil
	}
	start := math.Ceil(lo/step) * step
	end := math.Floor(hi/step) * step
	if start > end {
		return nil
	}
	n := int(math.Round((end-start)/step)) + 1
	if n <= 0 || n > maxTicks {
		return nil
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = start + float64(i)*step
	}
	return out
}

// Nice rounds lo and hi outward to human-friendly multiples of the computed
// tick step. This is used to extend a data domain to cover "nice" round
// numbers for axis display.
func Nice(lo, hi float64, count int) (float64, float64) {
	if lo == hi || count <= 0 {
		return lo, hi
	}
	step := tickStep(lo, hi, count)
	if step <= 0 || math.IsNaN(step) {
		return lo, hi
	}
	return math.Floor(lo/step) * step, math.Ceil(hi/step) * step
}

// tickLabels formats a slice of tick values as Tick structs with
// auto-selected precision so adjacent labels are distinct but not noisy.
func tickLabels(vals []float64) []Tick {
	if len(vals) == 0 {
		return nil
	}
	precision := 0
	if len(vals) >= 2 {
		precision = autoPrecision(math.Abs(vals[1] - vals[0]))
	}
	out := make([]Tick, len(vals))
	for i, v := range vals {
		out[i] = Tick{
			Value: v,
			Label: FormatFixed(v, precision),
		}
	}
	return out
}
