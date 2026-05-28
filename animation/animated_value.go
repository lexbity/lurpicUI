package animation

import (
	"reflect"
	"time"
)

// TransitionSpec describes one animated transition.
type TransitionSpec struct {
	Duration time.Duration
	Easing   string
	Delay    time.Duration
}

// AnimatedValue presents a smoothly interpolated view of a source value.
type AnimatedValue[T Interpolatable[T]] struct {
	source func() T

	current T
	target  T
	start   T

	spec       TransitionSpec
	activeSpec TransitionSpec

	registry    *EasingRegistry
	activeEase  EasingFunc
	animating   bool
	started     bool
	progress    float32
	elapsed     time.Duration
	delayRemain time.Duration
}

// NewAnimatedValue constructs an animated value and snaps it to the source.
func NewAnimatedValue[T Interpolatable[T]](source func() T, spec TransitionSpec, easing *EasingRegistry) *AnimatedValue[T] {
	av := &AnimatedValue[T]{
		source:   source,
		spec:     normalizeSpec(spec),
		registry: easing,
	}
	if av.registry == nil {
		av.registry = DefaultEasingRegistry()
	}
	av.SnapToTarget()
	return av
}

// Current returns the rendered value.
func (av *AnimatedValue[T]) Current() T {
	return av.current
}

// Target returns the current authoritative target value.
func (av *AnimatedValue[T]) Target() T {
	return av.target
}

// Progress reports the current transition progress.
func (av *AnimatedValue[T]) Progress() float32 {
	if !av.started || !av.animating {
		return 1
	}
	return av.progress
}

// IsAnimating reports whether a transition is in flight.
func (av *AnimatedValue[T]) IsAnimating() bool {
	return av.animating
}

// Tick advances the animation and reports whether the rendered value changed.
func (av *AnimatedValue[T]) Tick(dt time.Duration) bool {
	observed := av.observeSource()
	if !av.started {
		av.seed(observed)
		return false
	}
	if !reflect.DeepEqual(observed, av.target) {
		av.beginTransition(observed)
	}
	if !av.animating {
		return false
	}
	return av.advance(dt)
}

// SnapToTarget synchronizes the current rendered value with the source.
func (av *AnimatedValue[T]) SnapToTarget() {
	observed := av.observeSource()
	av.current = observed
	av.target = observed
	av.start = observed
	av.activeSpec = av.spec
	av.activeEase = av.resolveEasing(av.activeSpec.Easing)
	av.animating = false
	av.started = true
	av.progress = 1
	av.elapsed = 0
	av.delayRemain = 0
}

// SetSpec updates the default transition spec for future retargets.
func (av *AnimatedValue[T]) SetSpec(spec TransitionSpec) {
	av.spec = normalizeSpec(spec)
}

func (av *AnimatedValue[T]) observeSource() T {
	if av.source == nil {
		var zero T
		return zero
	}
	return av.source()
}

func (av *AnimatedValue[T]) seed(observed T) {
	av.current = observed
	av.target = observed
	av.start = observed
	av.activeSpec = av.spec
	av.activeEase = av.resolveEasing(av.activeSpec.Easing)
	av.animating = false
	av.started = true
	av.progress = 1
	av.elapsed = 0
	av.delayRemain = 0
}

func (av *AnimatedValue[T]) beginTransition(target T) {
	av.start = av.current
	av.target = target
	av.activeSpec = av.spec
	av.activeEase = av.resolveEasing(av.activeSpec.Easing)
	av.elapsed = 0
	av.delayRemain = av.activeSpec.Delay
	av.progress = 0
	av.animating = !reflect.DeepEqual(av.start, av.target)
	if !av.animating {
		av.current = av.target
		av.progress = 1
	}
}

func (av *AnimatedValue[T]) advance(dt time.Duration) bool {
	if dt < 0 {
		dt = 0
	}
	prev := av.current
	remaining := dt
	if av.delayRemain > 0 {
		if remaining <= av.delayRemain {
			av.delayRemain -= remaining
			return false
		}
		remaining -= av.delayRemain
		av.delayRemain = 0
	}
	if av.activeSpec.Duration <= 0 {
		av.current = av.target
		av.progress = 1
		av.animating = false
		return !reflect.DeepEqual(prev, av.current)
	}
	av.elapsed += remaining
	if av.elapsed >= av.activeSpec.Duration {
		av.current = av.target
		av.progress = 1
		av.animating = false
		return !reflect.DeepEqual(prev, av.current)
	}
	av.progress = float32(av.elapsed) / float32(av.activeSpec.Duration)
	if av.progress < 0 {
		av.progress = 0
	}
	if av.progress > 1 {
		av.progress = 1
	}
	eased := av.activeEase
	if eased == nil {
		eased = Linear()
	}
	av.progress = eased(av.progress)
	av.current = av.start.Lerp(av.target, av.progress)
	return !reflect.DeepEqual(prev, av.current)
}

func (av *AnimatedValue[T]) resolveEasing(name string) EasingFunc {
	reg := av.registry
	if reg == nil {
		return Linear()
	}
	if fn, ok := reg.Get(name); ok {
		return fn
	}
	if name != "" && normalizeEasingName(name) != "linear" {
		warnUnknownEasing(name)
	}
	if fn, ok := reg.Get("linear"); ok {
		return fn
	}
	return Linear()
}

func normalizeSpec(spec TransitionSpec) TransitionSpec {
	if spec.Duration < 0 {
		spec.Duration = 0
	}
	if spec.Delay < 0 {
		spec.Delay = 0
	}
	if spec.Easing == "" {
		spec.Easing = "linear"
	}
	return spec
}

// AnimatedFloat32 is the Float32 specialization of AnimatedValue.
type AnimatedFloat32 = AnimatedValue[Float32]
