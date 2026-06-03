package scale

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by scale constructors and methods.
var (
	ErrInvalidDomain   = errors.New("scale: invalid domain")
	ErrDomainCrossesZero = errors.New("scale: log domain must not cross or include zero")
	ErrEmptyMembers    = errors.New("scale: empty member set")
)

// Scale maps a data domain onto a numeric range expressed in local layer
// pixels. All arithmetic is float64; callers narrow to float32 at the gfx
// boundary.
type Scale interface {
	// Map converts a domain value to a range position. Total: never panics.
	Map(value float64) float64
	// Domain returns the configured input interval [lo, hi]. For ordinal
	// scales, lo/hi describe the index span; use the ordinal API for members.
	Domain() (lo, hi float64)
	// Range returns the configured output interval [lo, hi] in local pixels.
	Range() (lo, hi float64)
	// Kind identifies the concrete scale for tooling/diagnostics.
	Kind() ScaleKind
}

// InvertibleScale is implemented by continuous scales (Linear, Log, Pow, Time).
// Band/Point provide InvertRange instead because their inverse is a discrete
// membership query, not a continuous value.
type InvertibleScale interface {
	Scale
	// Invert converts a range position back to a domain value.
	Invert(position float64) float64
}

// Ticker produces reference values and labels for axes and gridlines.
type Ticker interface {
	// Ticks returns approximately count ticks (implementations round to a
	// human-friendly step; the exact count is advisory, not guaranteed).
	Ticks(count int) []Tick
}

// Tick is a reference value from a scale's domain with its formatted label.
type Tick struct {
	Value float64
	Label string
}

// ScaleKind identifies the concrete scale type for tooling and diagnostics.
type ScaleKind uint8

const (
	KindUnknown ScaleKind = iota
	KindLinear
	KindLog
	KindPow
	KindTime
	KindBand
	KindPoint
)

func (k ScaleKind) String() string {
	switch k {
	case KindUnknown:
		return "unknown"
	case KindLinear:
		return "linear"
	case KindLog:
		return "log"
	case KindPow:
		return "pow"
	case KindTime:
		return "time"
	case KindBand:
		return "band"
	case KindPoint:
		return "point"
	default:
		return fmt.Sprintf("ScaleKind(%d)", uint8(k))
	}
}

// OutOfRange selects the clamping behavior for Map/Invert on out-of-domain
// input.
type OutOfRange uint8

const (
	// OutOfRangeExtrapolate extrapolates linearly outside the domain.
	// This is the default behavior.
	OutOfRangeExtrapolate OutOfRange = iota
	// OutOfRangeClamp clamps the output to the nearest range endpoint.
	OutOfRangeClamp
)

func (o OutOfRange) String() string {
	switch o {
	case OutOfRangeExtrapolate:
		return "extrapolate"
	case OutOfRangeClamp:
		return "clamp"
	default:
		return fmt.Sprintf("OutOfRange(%d)", uint8(o))
	}
}

// options accumulates optional configuration shared across scale types.
// Unexported so that Option values can only be constructed via the public
// With* functions.
type options struct {
	domain        [2]float64
	hasDomain     bool
	strDomain     []string
	rng           [2]float64
	hasRange      bool
	clamp         *OutOfRange
	base          *float64
	exponent      *float64
	paddingInner  *float64
	paddingOuter  *float64
	align         *float64
}

// Option configures a scale during construction.
// Each concrete scale constructor applies the provided options in order;
// when the same option appears multiple times the last value wins.
type Option func(*options)

// WithDomain sets the scale's input domain [lo, hi].
func WithDomain(lo, hi float64) Option {
	return func(o *options) {
		o.domain = [2]float64{lo, hi}
		o.hasDomain = true
	}
}

// WithRange sets the scale's output range [lo, hi] in local layer pixels.
func WithRange(lo, hi float64) Option {
	return func(o *options) {
		o.rng = [2]float64{lo, hi}
		o.hasRange = true
	}
}

// WithClamp sets the out-of-range behavior for Map/Invert.
func WithClamp(c OutOfRange) Option {
	return func(o *options) {
		o.clamp = &c
	}
}

// WithBase sets the logarithmic base for LogScale. Must be positive and not 1.
func WithBase(base float64) Option {
	return func(o *options) {
		o.base = &base
	}
}

// WithExponent sets the power exponent for PowScale. Must be positive.
func WithExponent(exp float64) Option {
	return func(o *options) {
		o.exponent = &exp
	}
}

// WithPaddingInner sets the inner padding ratio for Band/Point scales.
// 0 = no gap between bands, 1 = no band width (only gaps).
func WithPaddingInner(p float64) Option {
	return func(o *options) {
		o.paddingInner = &p
	}
}

// WithPaddingOuter sets the outer padding ratio for Band/Point scales.
// 0 = no padding on range ends, higher values add more padding.
func WithPaddingOuter(p float64) Option {
	return func(o *options) {
		o.paddingOuter = &p
	}
}

// WithAlign sets the alignment for Band/Point scales: 0 = left/right-justified,
// 0.5 = centered (default), 1 = right/left-justified.
func WithAlign(a float64) Option {
	return func(o *options) {
		o.align = &a
	}
}
