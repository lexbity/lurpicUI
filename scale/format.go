package scale

import (
	"math"
	"strconv"
)

// autoPrecision returns the number of decimal places needed to distinguish
// values that are step apart. For step = 20, precision = 0; for step = 0.2,
// precision = 1; for step = 0.002, precision = 3. Degenerate and non-positive
// steps return 0.
func autoPrecision(step float64) int {
	if step <= 0 || math.IsNaN(step) || math.IsInf(step, 0) {
		return 0
	}
	log := math.Log10(step)
	p := -int(math.Floor(log))
	if p < 0 {
		p = 0
	}
	return p
}

// FormatFixed formats v with exactly precision decimal places, no trailing
// zeros stripped. Negative precision is treated as 0.
func FormatFixed(v float64, precision int) string {
	if precision < 0 {
		precision = 0
	}
	return strconv.FormatFloat(v, 'f', precision, 64)
}

// FormatSignificant formats v with digits significant digits using the
// shortest representation that preserves the given precision. Uses 'e'
// notation when the exponent is >= digits or < -4, matching Go's %g rule.
// digits must be >= 1.
func FormatSignificant(v float64, digits int) string {
	if digits < 1 {
		digits = 1
	}
	if v == 0 {
		return "0"
	}
	abs := math.Abs(v)
	exp := int(math.Floor(math.Log10(abs)))
	if exp < -4 || exp >= digits {
		return strconv.FormatFloat(v, 'e', digits-1, 64)
	}
	precision := digits - exp - 1
	return strconv.FormatFloat(v, 'f', precision, 64)
}

// FormatSI formats v with an SI prefix (k, M, G, m, µ, n) so that the
// numeric part has at most 3 digits before the decimal point. Zero returns
// "0". Very small values (< 1e-9) fall back to scientific notation.
func FormatSI(v float64) string {
	if v == 0 {
		return "0"
	}
	abs := math.Abs(v)
	var prefix string
	var divisor float64
	switch {
	case abs >= 1e9:
		prefix = "G"; divisor = 1e9
	case abs >= 1e6:
		prefix = "M"; divisor = 1e6
	case abs >= 1e3:
		prefix = "k"; divisor = 1e3
	case abs >= 1:
		prefix = ""; divisor = 1
	case abs >= 1e-3:
		prefix = "m"; divisor = 1e-3
	case abs >= 1e-6:
		prefix = "µ"; divisor = 1e-6
	case abs >= 1e-9:
		prefix = "n"; divisor = 1e-9
	default:
		return strconv.FormatFloat(v, 'e', 2, 64)
	}
	val := v / divisor
	// Use 'g' with 3 significant digits; for values in [1,1000) this
	// uses fixed-point format and strips trailing zeros automatically.
	return strconv.FormatFloat(val, 'g', 3, 64) + prefix
}
