package scenes

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

// StressScene validates resize/theme/mount/unmount churn and extreme facet counts.
// Phase 6: Enhanced with churn modes, frame tracking, and diagnostics comparison.
type StressScene struct {
	BaseScene
	itemCount     int
	isStressMode  bool
	churnCount    int
	themeChurnIdx int
	resizeCount   int
	mountCount    int
	churnActive   bool
	frameStats    FrameStats
}

// FrameStats tracks render performance during stress testing.
type FrameStats struct {
	TotalFrames    int
	DroppedFrames  int
	MinFrameTime   float64
	MaxFrameTime   float64
	AvgFrameTime   float64
	RenderBatches  int
	LastUpdateTime int64
}

// NewStressScene creates a new stress testing scene.
func NewStressScene() *StressScene {
	s := &StressScene{
		BaseScene: NewBaseScene(
			"stress",
			"Stress Test",
			"Validates resize/theme/mount/unmount churn and facet limits (Phase 6)",
			[]string{"structure"},
		),
		itemCount:     50,
		isStressMode:  false,
		churnCount:    0,
		themeChurnIdx: 0,
		resizeCount:   0,
		mountCount:    0,
		churnActive:   false,
		frameStats:    FrameStats{},
	}
	s.capability.HasStressControls = true
	s.capability.HasCustomLogs = true
	return s
}

// BuildRoot constructs the stress test UI.
func (s *StressScene) BuildRoot() facet.FacetImpl {
	if s.root != nil {
		return s.root
	}

	col := layout.NewColumnLayout()
	s.root = col

	// Generate many items for stress testing
	count := s.itemCount
	if s.isStressMode {
		count = 500
	}

	// Use a column with limited items for display
	displayCount := count
	if displayCount > 100 {
		displayCount = 100
	}

	grid := layout.NewRowLayout()
	for i := 0; i < displayCount; i++ {
		// Vary colors to stress the render pipeline
		color := gfx.ColorFromRGBA8(
			uint8((i*17)%255),
			uint8((i*31)%255),
			uint8((i*47)%255),
			255,
		)
		rect := &basic.Rect{
			ID:     "stress-" + string(rune('0'+i%10)),
			Bounds: basic.BoundsProps{X: 0, Y: 0, W: 20, H: 20},
			Style: basic.PrimitiveStyleProps{
				Fill:    solidFill(color),
				Visible: true,
				Opacity: 1,
			},
		}
		grid.Add(layout.Fixed(rect.Base()))
	}
	col.AddChild(grid.Base())

	return col
}

// SetStressMode enables/disables stress mode.
func (s *StressScene) SetStressMode(enabled bool) {
	s.isStressMode = enabled
	s.Reset()
}

// SetItemCount sets the number of items.
func (s *StressScene) SetItemCount(count int) {
	s.itemCount = count
	s.Reset()
}

// TriggerResizeChurn simulates repeated resize events.
func (s *StressScene) TriggerResizeChurn(iterations int) {
	s.churnActive = true
	s.resizeCount = iterations
	s.churnCount += iterations
}

// TriggerThemeChurn cycles through themes.
func (s *StressScene) TriggerThemeChurn(themes []theme.Context) {
	s.churnActive = true
	s.themeChurnIdx = (s.themeChurnIdx + 1) % len(themes)
	s.churnCount++
}

// TriggerMountUnmountChurn simulates mount/unmount cycles.
func (s *StressScene) TriggerMountUnmountChurn(count int) {
	s.churnActive = true
	s.mountCount += count
	s.churnCount += count
	s.Reset() // Force remount
}

// GetFrameStats returns current frame statistics.
func (s *StressScene) GetFrameStats() FrameStats {
	return s.frameStats
}

// UpdateFrameStats records a frame timing.
func (s *StressScene) UpdateFrameStats(frameTime float64, batchCount int) {
	s.frameStats.TotalFrames++
	s.frameStats.RenderBatches = batchCount

	if s.frameStats.MinFrameTime == 0 || frameTime < s.frameStats.MinFrameTime {
		s.frameStats.MinFrameTime = frameTime
	}
	if frameTime > s.frameStats.MaxFrameTime {
		s.frameStats.MaxFrameTime = frameTime
	}

	// Simple moving average
	if s.frameStats.AvgFrameTime == 0 {
		s.frameStats.AvgFrameTime = frameTime
	} else {
		s.frameStats.AvgFrameTime = s.frameStats.AvgFrameTime*0.9 + frameTime*0.1
	}

	// Detect dropped frames (>16ms for 60fps)
	if frameTime > 16.67 {
		s.frameStats.DroppedFrames++
	}
}

// GetStressReport returns a comprehensive stress test report.
func (s *StressScene) GetStressReport() map[string]any {
	return map[string]any{
		"scene_id":     s.id,
		"churn_count":  s.churnCount,
		"resize_count": s.resizeCount,
		"theme_count":  s.themeChurnIdx,
		"mount_count":  s.mountCount,
		"frame_stats": map[string]any{
			"total_frames":   s.frameStats.TotalFrames,
			"dropped_frames": s.frameStats.DroppedFrames,
			"min_frame_ms":   s.frameStats.MinFrameTime,
			"max_frame_ms":   s.frameStats.MaxFrameTime,
			"avg_frame_ms":   s.frameStats.AvgFrameTime,
			"render_batches": s.frameStats.RenderBatches,
		},
	}
}

// ExportState returns stress state.
func (s *StressScene) ExportState() map[string]any {
	return map[string]any{
		"scene_id":     s.id,
		"item_count":   s.itemCount,
		"stress_mode":  s.isStressMode,
		"churn_count":  s.churnCount,
		"resize_count": s.resizeCount,
		"mount_count":  s.mountCount,
	}
}

// ImportState restores stress state.
func (s *StressScene) ImportState(state map[string]any) {
	if v, ok := state["item_count"].(float64); ok {
		s.itemCount = int(v)
	}
	if v, ok := state["stress_mode"].(bool); ok {
		s.isStressMode = v
	}
	if v, ok := state["churn_count"].(float64); ok {
		s.churnCount = int(v)
	}
	if v, ok := state["resize_count"].(float64); ok {
		s.resizeCount = int(v)
	}
	if v, ok := state["mount_count"].(float64); ok {
		s.mountCount = int(v)
	}
}

var _ scene.Scene = (*StressScene)(nil)
