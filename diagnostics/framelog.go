package diagnostics

import (
	"sync"
	"time"
)

// FrameLogEntry stores one frame's stats and timestamp.
type FrameLogEntry struct {
	Stats     FrameStats
	Timestamp time.Time
}

// FrameLogSummary summarizes a rolling frame log.
type FrameLogSummary struct {
	FrameCount       int
	AvgFPS           float32
	AvgProjected     float32
	AvgJobsCommitted float32
	MaxLayoutMs      float32
	MaxProjectMs     float32
	MaxRenderMs      float32
	CacheHitRate     float32
}

// FrameLog stores a bounded rolling window of frame statistics.
type FrameLog struct {
	mu      sync.RWMutex
	entries []FrameLogEntry
	maxSize int
}

// NewFrameLog constructs a log with a bounded size.
func NewFrameLog(windowSize int) *FrameLog {
	if windowSize <= 0 {
		windowSize = 120
	}
	return &FrameLog{maxSize: windowSize}
}

// Record appends a new frame entry to the rolling window.
func (l *FrameLog) Record(stats FrameStats) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, FrameLogEntry{Stats: stats, Timestamp: time.Now()})
	if len(l.entries) > l.maxSize {
		excess := len(l.entries) - l.maxSize
		copy(l.entries, l.entries[excess:])
		l.entries = l.entries[:l.maxSize]
	}
}

// Recent returns the newest n entries in chronological order.
func (l *FrameLog) Recent(n int) []FrameLogEntry {
	if l == nil || n <= 0 {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if len(l.entries) == 0 {
		return nil
	}
	if n > len(l.entries) {
		n = len(l.entries)
	}
	start := len(l.entries) - n
	out := make([]FrameLogEntry, n)
	copy(out, l.entries[start:])
	return out
}

// Summary computes aggregate values over the current window.
func (l *FrameLog) Summary() FrameLogSummary {
	if l == nil {
		return FrameLogSummary{}
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	count := len(l.entries)
	if count == 0 {
		return FrameLogSummary{}
	}
	var (
		totalProjected     float32
		totalJobsCommitted float32
		totalCacheHits     float32
		totalCacheMisses   float32
		maxLayout          float32
		maxProject         float32
		maxRender          float32
	)
	for i, entry := range l.entries {
		stats := entry.Stats
		totalProjected += float32(stats.ProjectedFacets)
		totalJobsCommitted += float32(stats.JobsCommitted)
		totalCacheHits += float32(stats.CacheHits)
		totalCacheMisses += float32(stats.ProjectedFacets)
		layoutMs := float32(stats.LayoutDuration.Milliseconds())
		projectMs := float32(stats.ProjectDuration.Milliseconds())
		renderMs := float32(stats.RenderDuration.Milliseconds())
		if i == 0 || layoutMs > maxLayout {
			maxLayout = layoutMs
		}
		if i == 0 || projectMs > maxProject {
			maxProject = projectMs
		}
		if i == 0 || renderMs > maxRender {
			maxRender = renderMs
		}
	}
	avgFPS := float32(0)
	if count >= 2 {
		first := l.entries[0].Timestamp
		last := l.entries[count-1].Timestamp
		if dur := last.Sub(first); dur > 0 {
			avgFPS = float32(count-1) / float32(dur.Seconds())
		}
	}
	hitRate := float32(0)
	if totalCacheHits+totalCacheMisses > 0 {
		hitRate = totalCacheHits / (totalCacheHits + totalCacheMisses)
	}
	return FrameLogSummary{
		FrameCount:       count,
		AvgFPS:           avgFPS,
		AvgProjected:     totalProjected / float32(count),
		AvgJobsCommitted: totalJobsCommitted / float32(count),
		MaxLayoutMs:      maxLayout,
		MaxProjectMs:     maxProject,
		MaxRenderMs:      maxRender,
		CacheHitRate:     hitRate,
	}
}

// Hook adapts diagnostics components to the runtime diagnostics hook interface.
type Hook struct {
	FrameLog  *FrameLog
	Overlay   *Overlay
	Inspector *Inspector
	HitProbe  *HitProbe
}

// OnFrame records the supplied frame stats.
func (h *Hook) OnFrame(stats FrameStats) {
	if h == nil || h.FrameLog == nil {
		return
	}
	h.FrameLog.Record(stats)
}
