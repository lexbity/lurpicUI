package scenes

import (
	"fmt"
	"sort"

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
	failureSource string
	failureCounts map[string]int
	recentNotes   []string
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
		failureCounts: make(map[string]int),
		recentNotes:   make([]string, 0, 32),
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
	col.Gap = 8
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

	if s.churnActive || s.failureSource != "" {
		status := &basic.Text{
			ID: "stress-status",
			Paragraph: textParagraph(fmt.Sprintf(
				"churn=%d resize=%d density=%d mount=%d failure=%s",
				s.churnCount, s.resizeCount, s.themeChurnIdx, s.mountCount, s.failureSource,
			)),
			MaxWidth: 480,
		}
		col.AddChild(status.Base())
	}

	return col
}

// SetStressMode enables/disables stress mode.
func (s *StressScene) SetStressMode(enabled bool) {
	s.isStressMode = enabled
	s.note("state mutation", fmt.Sprintf("stress mode=%t", enabled))
	s.Reset()
}

// SetItemCount sets the number of items.
func (s *StressScene) SetItemCount(count int) {
	s.itemCount = count
	s.note("state mutation", fmt.Sprintf("item count=%d", count))
	s.Reset()
}

// TriggerResizeChurn simulates repeated resize events.
func (s *StressScene) TriggerResizeChurn(iterations int) {
	s.churnActive = true
	if iterations < 0 {
		iterations = 0
	}
	s.resizeCount += iterations
	s.churnCount += iterations
	s.note("layout", fmt.Sprintf("resize churn=%d", iterations))
}

// TriggerThemeChurn cycles through themes.
func (s *StressScene) TriggerThemeChurn(themes []theme.Context) {
	s.churnActive = true
	if len(themes) == 0 {
		s.recordFailure("theme", "theme churn requested with no theme inputs")
		return
	}
	s.themeChurnIdx = (s.themeChurnIdx + 1) % len(themes)
	s.churnCount++
	s.note("theme", fmt.Sprintf("theme churn=%d", s.themeChurnIdx))
}

// TriggerDensityChurn simulates density changes during stress.
func (s *StressScene) TriggerDensityChurn(scales []float32) {
	s.churnActive = true
	if len(scales) == 0 {
		s.recordFailure("layout", "density churn requested with no scales")
		return
	}
	s.note("layout", fmt.Sprintf("density churn samples=%d", len(scales)))
	s.churnCount += len(scales)
}

// TriggerInputSpam simulates repeated input spikes during churn.
func (s *StressScene) TriggerInputSpam(count int) {
	s.churnActive = true
	if count < 0 {
		count = 0
	}
	s.churnCount += count
	s.note("input", fmt.Sprintf("input spam=%d", count))
}

// TriggerSceneResetChurn simulates repeated scene resets.
func (s *StressScene) TriggerSceneResetChurn(count int) {
	s.churnActive = true
	if count < 0 {
		count = 0
	}
	s.churnCount += count
	s.note("runtime", fmt.Sprintf("scene reset churn=%d", count))
}

// TriggerMountUnmountChurn simulates mount/unmount cycles.
func (s *StressScene) TriggerMountUnmountChurn(count int) {
	s.churnActive = true
	if count < 0 {
		count = 0
	}
	s.mountCount += count
	s.churnCount += count
	s.note("runtime", fmt.Sprintf("mount/unmount churn=%d", count))
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
	failures := make(map[string]int, len(s.failureCounts))
	keys := make([]string, 0, len(s.failureCounts))
	for k := range s.failureCounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		failures[k] = s.failureCounts[k]
	}
	return map[string]any{
		"scene_id":       s.id,
		"churn_count":    s.churnCount,
		"resize_count":   s.resizeCount,
		"theme_count":    s.themeChurnIdx,
		"mount_count":    s.mountCount,
		"failure_source": s.failureSource,
		"failure_counts": failures,
		"recent_notes":   append([]string(nil), s.recentNotes...),
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
		"scene_id":       s.id,
		"item_count":     s.itemCount,
		"stress_mode":    s.isStressMode,
		"churn_count":    s.churnCount,
		"resize_count":   s.resizeCount,
		"mount_count":    s.mountCount,
		"failure_source": s.failureSource,
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
	if v, ok := state["failure_source"].(string); ok {
		s.failureSource = v
	}
}

func (s *StressScene) RecordFailure(source, message string) {
	s.recordFailure(source, message)
}

func (s *StressScene) note(source, message string) {
	entry := fmt.Sprintf("%s: %s", source, message)
	s.recentNotes = append(s.recentNotes, entry)
	if len(s.recentNotes) > 32 {
		s.recentNotes = s.recentNotes[len(s.recentNotes)-32:]
	}
}

func (s *StressScene) recordFailure(source, message string) {
	if source == "" {
		source = "runtime"
	}
	if s.failureCounts == nil {
		s.failureCounts = make(map[string]int)
	}
	s.failureSource = source
	s.failureCounts[source]++
	s.note(source, message)
}

var _ scene.Scene = (*StressScene)(nil)
