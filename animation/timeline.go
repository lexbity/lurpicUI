package animation

import (
	"math"
	"sort"
	"time"

	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/store"
)

// PlaybackState tracks timeline playback status.
type PlaybackState uint8

const (
	PlaybackStopped PlaybackState = iota
	PlaybackPlaying
	PlaybackPaused
)

// LoopMode controls what happens when a timeline reaches its bounds.
type LoopMode uint8

const (
	LoopNone LoopMode = iota
	LoopRepeat
	LoopPingPong
)

// TimelineConfig configures playback behavior.
type TimelineConfig struct {
	Duration float64
	Loop     LoopMode
	Speed    float32
}

// Timeline is a store-backed playback controller.
type Timeline struct {
	T     *store.ValueStore[float64]
	State *store.ValueStore[PlaybackState]

	cfg     TimelineConfig
	speed   float32
	forward bool

	runtime            *runtime.Runtime
	unregister         func()
	unregisterShutdown func()
	disposed           bool
}

// Keyframe maps a point in time to a value.
type Keyframe[T any] struct {
	T      float64
	Value  T
	Easing string
}

// KeyframeSequence evaluates values between keyframes.
type KeyframeSequence[T Interpolatable[T]] struct {
	keyframes []Keyframe[T]
	easing    *EasingRegistry
}

// NewTimeline constructs a new timeline and registers it for phase-1 ticks.
func NewTimeline(rt *runtime.Runtime, cfg TimelineConfig) *Timeline {
	if cfg.Speed == 0 {
		cfg.Speed = 1
	}
	tl := &Timeline{
		T:       store.NewValueStore(float64(0)),
		State:   store.NewValueStore(PlaybackStopped),
		cfg:     cfg,
		speed:   cfg.Speed,
		forward: true,
		runtime: rt,
	}
	tl.bind()
	return tl
}

// Play starts or resumes playback.
func (tl *Timeline) Play() {
	if tl.State == nil {
		return
	}
	tl.State.Set(PlaybackPlaying)
}

// Pause pauses playback without changing T.
func (tl *Timeline) Pause() {
	if tl.State == nil {
		return
	}
	tl.State.Set(PlaybackPaused)
}

// Stop stops playback and rewinds to zero.
func (tl *Timeline) Stop() {
	if tl.T != nil {
		tl.T.Set(0)
	}
	if tl.State != nil {
		tl.State.Set(PlaybackStopped)
	}
	tl.forward = true
}

// Seek moves the playhead to an absolute position.
func (tl *Timeline) Seek(t float64) {
	if tl.T == nil {
		return
	}
	tl.T.Set(tl.clampT(t))
}

// SetSpeed changes playback speed.
func (tl *Timeline) SetSpeed(speed float32) {
	tl.speed = speed
}

// tick advances playback on the runtime thread.
func (tl *Timeline) tick(dt time.Duration) {
	if tl.T == nil || tl.State == nil {
		return
	}
	if tl.State.Get() != PlaybackPlaying {
		return
	}
	if dt <= 0 || tl.speed == 0 {
		return
	}
	delta := float64(dt) * float64(tl.speed) / float64(time.Second)
	if delta == 0 {
		return
	}
	if tl.cfg.Duration <= 0 {
		tl.advanceInfinite(delta)
		return
	}
	switch tl.cfg.Loop {
	case LoopRepeat:
		tl.advanceRepeat(delta)
	case LoopPingPong:
		tl.advancePingPong(delta)
	default:
		tl.advanceNone(delta)
	}
}

// Evaluate returns the value at time t.
func (ks *KeyframeSequence[T]) Evaluate(t float64) T {
	if ks == nil || len(ks.keyframes) == 0 {
		panic("animation: empty keyframe sequence")
	}
	if len(ks.keyframes) == 1 {
		return ks.keyframes[0].Value
	}
	if t <= ks.keyframes[0].T {
		return ks.keyframes[0].Value
	}
	last := ks.keyframes[len(ks.keyframes)-1]
	if t >= last.T {
		return last.Value
	}
	i := sort.Search(len(ks.keyframes)-1, func(i int) bool {
		return t < ks.keyframes[i+1].T
	})
	if i < 0 {
		i = 0
	}
	a := ks.keyframes[i]
	b := ks.keyframes[i+1]
	if a.T == t {
		return a.Value
	}
	if b.T == t {
		return b.Value
	}
	span := b.T - a.T
	if span <= 0 {
		return b.Value
	}
	local := float32((t - a.T) / span)
	if local < 0 {
		local = 0
	}
	if local > 1 {
		local = 1
	}
	return a.Value.Lerp(b.Value, ks.segmentEasing(a.Easing)(local))
}

// AsSource returns a source function that evaluates the sequence at tl.T.
func (ks *KeyframeSequence[T]) AsSource(tl *Timeline) func() T {
	return func() T {
		if tl == nil || tl.T == nil {
			var zero T
			return zero
		}
		return ks.Evaluate(tl.T.Get())
	}
}

func (tl *Timeline) clampT(t float64) float64 {
	if tl.cfg.Duration <= 0 {
		if t < 0 {
			return 0
		}
		return t
	}
	if t < 0 {
		return 0
	}
	if t > tl.cfg.Duration {
		return tl.cfg.Duration
	}
	return t
}

func (tl *Timeline) advanceInfinite(delta float64) {
	if tl.T == nil {
		return
	}
	current := tl.T.Get() + delta
	if current < 0 {
		current = 0
	}
	tl.T.Set(current)
}

func (tl *Timeline) advanceNone(delta float64) {
	current := tl.T.Get()
	next := current + delta
	if next >= tl.cfg.Duration {
		tl.T.Set(tl.cfg.Duration)
		tl.State.Set(PlaybackStopped)
		tl.forward = true
		return
	}
	if next <= 0 {
		tl.T.Set(0)
		tl.State.Set(PlaybackStopped)
		tl.forward = true
		return
	}
	tl.T.Set(next)
}

func (tl *Timeline) advanceRepeat(delta float64) {
	duration := tl.cfg.Duration
	current := tl.T.Get()
	next := math.Mod(current+delta, duration)
	if next < 0 {
		next += duration
	}
	tl.T.Set(next)
}

func (tl *Timeline) advancePingPong(delta float64) {
	if delta < 0 {
		tl.forward = !tl.forward
		delta = -delta
	}
	current := tl.T.Get()
	remaining := delta
	for remaining > 0 {
		if tl.forward {
			toEdge := tl.cfg.Duration - current
			if remaining <= toEdge {
				current += remaining
				break
			}
			current = tl.cfg.Duration
			remaining -= toEdge
			tl.forward = false
			continue
		}
		toEdge := current
		if remaining <= toEdge {
			current -= remaining
			break
		}
		current = 0
		remaining -= toEdge
		tl.forward = true
	}
	tl.T.Set(tl.clampT(current))
}

func (ks *KeyframeSequence[T]) segmentEasing(name string) EasingFunc {
	reg := ks.easing
	if reg == nil {
		reg = DefaultEasingRegistry()
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

// NewKeyframeSequence constructs a sorted keyframe sequence.
func NewKeyframeSequence[T Interpolatable[T]](keyframes []Keyframe[T], easing *EasingRegistry) *KeyframeSequence[T] {
	out := append([]Keyframe[T](nil), keyframes...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].T < out[j].T
	})
	if easing == nil {
		easing = DefaultEasingRegistry()
	}
	return &KeyframeSequence[T]{keyframes: out, easing: easing}
}
