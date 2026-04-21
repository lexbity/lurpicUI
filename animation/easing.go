package animation

import (
	"fmt"
	"math"
	"strings"
	"sync"
)

// EasingFunc maps normalized time to eased progress.
type EasingFunc func(t float32) float32

// Logger captures the warning path used for easing fallbacks.
type Logger interface {
	Warn(msg string, args ...any)
}

var packageLogger Logger

// SetLogger installs a package-level logger for easing warnings.
func SetLogger(l Logger) {
	packageLogger = l
}

// EasingRegistry stores named easing functions.
type EasingRegistry struct {
	mu    sync.RWMutex
	funcs map[string]EasingFunc
}

// NewEasingRegistry constructs an empty registry.
func NewEasingRegistry() *EasingRegistry {
	return &EasingRegistry{funcs: make(map[string]EasingFunc)}
}

// DefaultEasingRegistry returns a registry preloaded with all standard curves.
func DefaultEasingRegistry() *EasingRegistry {
	r := NewEasingRegistry()
	r.Register("linear", Linear())
	r.Register("standard", CubicBezier(0.2, 0, 0, 1))
	r.Register("decelerate", CubicBezier(0, 0, 0, 1))
	r.Register("accelerate", CubicBezier(0.55, 0, 1, 1))
	r.Register("ease-in", CubicBezier(0.4, 0, 1, 1))
	r.Register("ease-out", CubicBezier(0, 0, 0.05, 1))
	r.Register("ease-in-out", CubicBezier(0.4, 0, 0.6, 1))
	r.Register("spring", Spring(0.7, 7))
	r.Register("bounce-out", BounceOut())
	r.Register("elastic-out", ElasticOut())
	return r
}

// Register stores or replaces an easing function.
func (r *EasingRegistry) Register(name string, fn EasingFunc) {
	if r == nil || fn == nil {
		return
	}
	r.mu.Lock()
	if r.funcs == nil {
		r.funcs = make(map[string]EasingFunc)
	}
	r.funcs[normalizeEasingName(name)] = fn
	r.mu.Unlock()
}

// Get returns the easing function for the supplied name.
func (r *EasingRegistry) Get(name string) (EasingFunc, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	fn, ok := r.funcs[normalizeEasingName(name)]
	r.mu.RUnlock()
	return fn, ok
}

// MustGet returns the easing function or panics if missing.
func (r *EasingRegistry) MustGet(name string) EasingFunc {
	if fn, ok := r.Get(name); ok {
		return fn
	}
	panic(fmt.Sprintf("animation: easing %q not found", name))
}

// Linear returns a no-op easing function.
func Linear() EasingFunc {
	return func(t float32) float32 {
		return clamp01(t)
	}
}

// CubicBezier evaluates a cubic-bezier easing curve using a precomputed
// sample table plus Newton-Raphson refinement.
func CubicBezier(x1, y1, x2, y2 float32) EasingFunc {
	curve := cubicBezierCurve{
		ax: float64(3*x1 - 3*x2 + 1),
		bx: float64(3*x2 - 6*x1),
		cx: float64(3 * x1),
		ay: float64(3*y1 - 3*y2 + 1),
		by: float64(3*y2 - 6*y1),
		cy: float64(3 * y1),
	}
	curve.init()
	return func(t float32) float32 {
		return curve.solve(clamp01(t))
	}
}

// Spring evaluates the analytical unit-step response of a second-order system.
func Spring(damping, frequency float32) EasingFunc {
	if frequency <= 0 {
		frequency = 1
	}
	return func(t float32) float32 {
		t = clamp01(t)
		if t == 0 {
			return 0
		}
		if t == 1 {
			return 1
		}
		w := float64(frequency)
		z := float64(damping)
		x := float64(t)
		switch {
		case z < 1:
			wd := w * math.Sqrt(1-z*z)
			if wd == 0 {
				return float32(1 - math.Exp(-z*w*x)*(1+w*x))
			}
			c := z / math.Sqrt(1-z*z)
			return float32(1 - math.Exp(-z*w*x)*(math.Cos(wd*x)+c*math.Sin(wd*x)))
		case z == 1:
			return float32(1 - math.Exp(-w*x)*(1+w*x))
		default:
			s := math.Sqrt(z*z - 1)
			r1 := -w * (z - s)
			r2 := -w * (z + s)
			denom := r2 - r1
			if denom == 0 {
				return 1
			}
			a := r2 / denom
			b := -r1 / denom
			return float32(1 - a*math.Exp(r1*x) - b*math.Exp(r2*x))
		}
	}
}

// BounceOut returns a classic bouncing ease-out curve.
func BounceOut() EasingFunc {
	return func(t float32) float32 {
		t = clamp01(t)
		switch {
		case t < 1/2.75:
			return 7.5625 * t * t
		case t < 2/2.75:
			t -= 1.5 / 2.75
			return 7.5625*t*t + 0.75
		case t < 2.5/2.75:
			t -= 2.25 / 2.75
			return 7.5625*t*t + 0.9375
		default:
			t -= 2.625 / 2.75
			return 7.5625*t*t + 0.984375
		}
	}
}

// ElasticOut returns an elastic overshooting ease-out curve.
func ElasticOut() EasingFunc {
	return func(t float32) float32 {
		t = clamp01(t)
		if t == 0 || t == 1 {
			return t
		}
		return float32(math.Pow(2, -10*float64(t))*math.Sin((float64(t)-0.075)*(2*math.Pi)/0.3) + 1)
	}
}

func normalizeEasingName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func warnUnknownEasing(name string) {
	if packageLogger != nil {
		packageLogger.Warn("animation: unknown easing name; falling back to linear", "name", name)
	}
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 || math.IsNaN(float64(v)) {
		return 1
	}
	return v
}

type cubicBezierCurve struct {
	ax, bx, cx float64
	ay, by, cy float64
	samples    [11]float64
}

func (c *cubicBezierCurve) init() {
	for i := 0; i < len(c.samples); i++ {
		c.samples[i] = c.sampleCurveX(float64(i) / 10)
	}
}

func (c *cubicBezierCurve) solve(x float32) float32 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	guess := c.guessT(float64(x))
	for i := 0; i < 8; i++ {
		est := c.sampleCurveX(guess) - float64(x)
		if math.Abs(est) < 1e-7 {
			break
		}
		deriv := c.sampleCurveDerivativeX(guess)
		if deriv == 0 {
			break
		}
		guess -= est / deriv
		if guess <= 0 {
			guess = 0
			break
		}
		if guess >= 1 {
			guess = 1
			break
		}
	}
	return float32(c.sampleCurveY(guess))
}

func (c *cubicBezierCurve) guessT(x float64) float64 {
	last := len(c.samples) - 1
	i := 1
	for ; i < last && c.samples[i] <= x; i++ {
	}
	idx := i - 1
	span := c.samples[i] - c.samples[idx]
	if span == 0 {
		return float64(idx) / float64(last)
	}
	frac := (x - c.samples[idx]) / span
	return (float64(idx) + frac) / float64(last)
}

func (c *cubicBezierCurve) sampleCurveX(t float64) float64 {
	return ((c.ax*t+c.bx)*t + c.cx) * t
}

func (c *cubicBezierCurve) sampleCurveY(t float64) float64 {
	return ((c.ay*t+c.by)*t + c.cy) * t
}

func (c *cubicBezierCurve) sampleCurveDerivativeX(t float64) float64 {
	return (3*c.ax*t+2*c.bx)*t + c.cx
}
