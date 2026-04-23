package diagnostics

import (
	"time"
)

// FrameStatsView is an app-facing view of frame statistics.
// It provides stable access to timing and pipeline metrics without
// exposing engine internals.
type FrameStatsView struct {
	// FrameNumber is the sequential frame identifier.
	FrameNumber uint64

	// Timing information
	TotalDuration     time.Duration
	LayoutDuration    time.Duration
	ProjectDuration   time.Duration
	RenderDuration    time.Duration

	// Pipeline statistics
	DirtyFacetCount      int
	ProjectedFacetCount  int
	CacheHitCount        int
	RenderBatchCount     int

	// Job statistics
	JobsCommitted int
	JobsDiscarded int

	// Timestamp when these stats were captured
	CapturedAt time.Time
}

// FPS calculates the instantaneous frames per second.
func (f FrameStatsView) FPS() float64 {
	if f.TotalDuration <= 0 {
		return 0
	}
	return float64(time.Second) / float64(f.TotalDuration)
}

// IsBudgetHealthy reports whether the frame stayed within 16.67ms (60fps).
func (f FrameStatsView) IsBudgetHealthy() bool {
	return f.TotalDuration < 16_670_000 // 16.67ms in nanoseconds
}

// LayoutRatio returns the proportion of time spent in layout.
func (f FrameStatsView) LayoutRatio() float64 {
	return f.ratio(f.LayoutDuration)
}

// ProjectRatio returns the proportion of time spent in projection.
func (f FrameStatsView) ProjectRatio() float64 {
	return f.ratio(f.ProjectDuration)
}

// RenderRatio returns the proportion of time spent in rendering.
func (f FrameStatsView) RenderRatio() float64 {
	return f.ratio(f.RenderDuration)
}

func (f FrameStatsView) ratio(d time.Duration) float64 {
	if f.TotalDuration <= 0 {
		return 0
	}
	return float64(d) / float64(f.TotalDuration)
}

// FrameStatsHistory maintains a rolling window of frame statistics.
type FrameStatsHistory struct {
	entries  []FrameStatsView
	capacity int
	pos      int
	filled   bool
}

// NewFrameStatsHistory creates a new history with the specified capacity.
func NewFrameStatsHistory(capacity int) *FrameStatsHistory {
	return &FrameStatsHistory{
		entries:  make([]FrameStatsView, capacity),
		capacity: capacity,
	}
}

// Add appends a new frame stats entry to the history.
func (h *FrameStatsHistory) Add(stats FrameStatsView) {
	if h == nil || h.capacity <= 0 {
		return
	}
	h.entries[h.pos] = stats
	h.pos++
	if h.pos >= h.capacity {
		h.pos = 0
		h.filled = true
	}
}

// GetAll returns all entries in chronological order.
func (h *FrameStatsHistory) GetAll() []FrameStatsView {
	if h == nil || !h.filled && h.pos == 0 {
		return nil
	}

	if !h.filled {
		return append([]FrameStatsView(nil), h.entries[:h.pos]...)
	}

	// Return in chronological order (oldest first)
	result := make([]FrameStatsView, h.capacity)
	copy(result, h.entries[h.pos:])
	copy(result[h.capacity-h.pos:], h.entries[:h.pos])
	return result
}

// Latest returns the most recent frame stats, or zero values if empty.
func (h *FrameStatsHistory) Latest() FrameStatsView {
	if h == nil || (!h.filled && h.pos == 0) {
		return FrameStatsView{}
	}

	if h.pos == 0 {
		return h.entries[h.capacity-1]
	}
	return h.entries[h.pos-1]
}

// AverageFPS returns the average FPS over the stored history.
func (h *FrameStatsHistory) AverageFPS() float64 {
	entries := h.GetAll()
	if len(entries) == 0 {
		return 0
	}

	var totalDuration time.Duration
	for _, e := range entries {
		totalDuration += e.TotalDuration
	}

	if totalDuration <= 0 {
		return 0
	}

	return float64(len(entries)) / (float64(totalDuration) / float64(time.Second))
}

// MinMaxFPS returns the minimum and maximum FPS in the history.
func (h *FrameStatsHistory) MinMaxFPS() (min, max float64) {
	entries := h.GetAll()
	if len(entries) == 0 {
		return 0, 0
	}

	min = entries[0].FPS()
	max = min

	for _, e := range entries[1:] {
		fps := e.FPS()
		if fps < min {
			min = fps
		}
		if fps > max {
			max = fps
		}
	}

	return min, max
}
