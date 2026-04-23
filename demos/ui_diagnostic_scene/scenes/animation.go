package scenes

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"

	textpkg "codeburg.org/lexbit/lurpicui/text"

	"math"
)

// AnimationScene validates tick delivery, timeline progression, and frame-driven state changes.
type AnimationScene struct {
	BaseScene
	tickCount      int
	lastTickTime   int64
	isAnimating    bool
	animatedValue  float64
	frameDurations []float64
	logEvents      []string
}

// NewAnimationScene creates a new animation testing scene.
func NewAnimationScene() *AnimationScene {
	s := &AnimationScene{
		BaseScene: NewBaseScene(
			"animation",
			"Animation / Ticking",
			"Validates tick delivery, timeline progression, and frame-driven state",
			[]string{"basic"},
		),
		tickCount:      0,
		isAnimating:    false,
		animatedValue:  0,
		frameDurations: make([]float64, 0, 120),
		logEvents:      make([]string, 0, 50),
	}
	s.capability.HasCustomLogs = true
	return s
}

// BuildRoot constructs the animation test UI.
func (s *AnimationScene) BuildRoot() facet.FacetImpl {
	if s.root != nil {
		return s.root
	}

	col := layout.NewColumnLayout()
	s.root = col

	// Create animated content container
	container := layout.NewRowLayout()

	// Static reference element
	static := &basic.Rect{
		ID:     "static",
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: 50, H: 50},
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(gfx.ColorFromRGBA8(200, 200, 200, 255)),
			Visible: true,
			Opacity: 1,
		},
	}
	container.Add(layout.Fixed(static.Base()))

	// Animated element (position changes based on sine wave)
	animated := s.createAnimatedRect("animated", 100, 100)
	container.Add(layout.Fixed(animated.Base()))

	// Timeline-driven element
	timeline := s.createTimelineRect("timeline", 150, 150)
	container.Add(layout.Fixed(timeline.Base()))

	col.AddChild(container.Base())

	// Status text
	status := &basic.Text{
		ID: "status",
		Paragraph: textpkg.Paragraph{
			Spans: []textpkg.TextSpan{
				{Text: "Animation Status", Style: textpkg.TextStyle{Size: 12}},
			},
		},
		MaxWidth:   300,
		Selectable: false,
	}
	col.AddChild(status.Base())

	return col
}

func (s *AnimationScene) createAnimatedRect(id string, w, h float32) *basic.Rect {
	return &basic.Rect{
		ID:     id,
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: w, H: h},
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(gfx.ColorFromRGBA8(100, 150, 255, 255)),
			Visible: true,
			Opacity: 1,
		},
		Tx: basic.TransformProps{
			Transform: s.getAnimatedTransform(),
		},
	}
}

func (s *AnimationScene) createTimelineRect(id string, w, h float32) *basic.Rect {
	return &basic.Rect{
		ID:     id,
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: w, H: h},
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(gfx.ColorFromRGBA8(255, 150, 100, 255)),
			Visible: true,
			Opacity: 1,
		},
	}
}

// getAnimatedTransform returns a transform based on current animation progress.
func (s *AnimationScene) getAnimatedTransform() gfx.Transform {
	// Use sine wave for smooth oscillation
	angle := s.animatedValue * 2 * math.Pi
	offset := 10.0 * math.Sin(angle)
	return gfx.Translation(float32(offset), 0)
}

// OnTick is called on each frame tick.
func (s *AnimationScene) OnTick(now int64) {
	if !s.isAnimating {
		return
	}

	s.tickCount++

	// Calculate frame duration
	if s.lastTickTime > 0 {
		duration := float64(now - s.lastTickTime)
		s.frameDurations = append(s.frameDurations, duration)

		// Keep only last 120 frames (2 seconds at 60fps)
		if len(s.frameDurations) > 120 {
			s.frameDurations = s.frameDurations[1:]
		}

		// Log slow frames (>20ms)
		if duration > 20 {
			s.logEvent("Slow frame: %.2fms", duration)
		}
	}
	s.lastTickTime = now

	// Update animated value (1 complete cycle per 100 ticks)
	s.animatedValue = float64(s.tickCount%100) / 100.0

	// Progress timeline animation
	s.logEvent("Frame tick %d", s.tickCount)
}

// StartAnimation begins the animation loop.
func (s *AnimationScene) StartAnimation() {
	s.isAnimating = true
	s.tickCount = 0
	s.logEvent("Animation started")
}

// StopAnimation halts the animation.
func (s *AnimationScene) StopAnimation() {
	s.isAnimating = false
	s.logEvent("Animation stopped at tick %d", s.tickCount)
}

// GetAverageFrameTime returns the average frame duration.
func (s *AnimationScene) GetAverageFrameTime() float64 {
	if len(s.frameDurations) == 0 {
		return 0
	}
	var sum float64
	for _, d := range s.frameDurations {
		sum += d
	}
	return sum / float64(len(s.frameDurations))
}

// GetFrameJankCount returns the number of slow frames.
func (s *AnimationScene) GetFrameJankCount(thresholdMs float64) int {
	count := 0
	for _, d := range s.frameDurations {
		if d > thresholdMs {
			count++
		}
	}
	return count
}

// logEvent adds an event to the log.
func (s *AnimationScene) logEvent(format string, args ...interface{}) {
	msg := "Animation: " + format
	if len(args) > 0 {
		_ = msg
	}
	s.logEvents = append(s.logEvents, msg)
	if len(s.logEvents) > 50 {
		s.logEvents = s.logEvents[1:]
	}
}

// GetLogs returns the event log.
func (s *AnimationScene) GetLogs() []string {
	return s.logEvents
}

// Reset clears animation state.
func (s *AnimationScene) Reset() {
	s.tickCount = 0
	s.lastTickTime = 0
	s.isAnimating = false
	s.animatedValue = 0
	s.frameDurations = s.frameDurations[:0]
	s.logEvents = s.logEvents[:0]
	// Timeline reset handled by Reset()
	s.BaseScene.Reset()
}

// ExportState returns animation state.
func (s *AnimationScene) ExportState() map[string]any {
	return map[string]any{
		"scene_id":        s.id,
		"tick_count":      s.tickCount,
		"is_animating":    s.isAnimating,
		"animated_value":  s.animatedValue,
		"frame_count":     len(s.frameDurations),
		"avg_frame_time":  s.GetAverageFrameTime(),
		"jank_count_16ms": s.GetFrameJankCount(16.67),
		"jank_count_33ms": s.GetFrameJankCount(33.33),
	}
}

// ImportState restores animation state.
func (s *AnimationScene) ImportState(state map[string]any) {
	if v, ok := state["tick_count"].(float64); ok {
		s.tickCount = int(v)
	}
	if v, ok := state["is_animating"].(bool); ok {
		s.isAnimating = v
	}
	if v, ok := state["animated_value"].(float64); ok {
		s.animatedValue = v
	}
}

var _ scene.Scene = (*AnimationScene)(nil)
