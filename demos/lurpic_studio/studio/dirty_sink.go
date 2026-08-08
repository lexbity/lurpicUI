package studio

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/runtime"
)

// DirtySink is the E5 exhibit's observer (F-diag-access + F-dirtysources): it
// implements runtime.DiagnosticsHook and opts into the runtime's
// DirtySnapshotSink capability, staging each frame's dirty snapshot into a
// bounded ring buffer that the E5 facet reads on its own projection pass. It
// never renders synchronously — the frame's critical path is unchanged
// (NFR-introspect-neutral).
//
// Concurrency: the runtime invokes OnDirtySnapshot on the runtime thread, while
// the E5 facet's projection may run in a forked projection goroutine, so the
// buffer is mutex-guarded. In practice the access is single-producer /
// single-consumer, but the lock keeps -race honest under forked projection.
type DirtySink struct {
	mu        sync.Mutex
	buf       []runtime.DirtySnapshot // last retention snapshots, oldest first
	retention int
	paused    bool
	snapshots int
	live      bool
}

// NewDirtySink builds a sink retaining up to retention snapshots.
func NewDirtySink(retention int) *DirtySink {
	if retention < 1 {
		retention = 1
	}
	return &DirtySink{retention: retention}
}

// OnFrame satisfies runtime.DiagnosticsHook. The sink's value is the dirty
// snapshots, not per-frame stats, so this is a no-op.
func (s *DirtySink) OnFrame(diagnostics.FrameStats) {}

// OnDirtySnapshot stages a frame's snapshot into the ring buffer.
func (s *DirtySink) OnDirtySnapshot(snap runtime.DirtySnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.paused {
		return
	}
	if len(s.buf) >= s.retention {
		copy(s.buf, s.buf[1:])
		s.buf[len(s.buf)-1] = snap
	} else {
		s.buf = append(s.buf, snap)
	}
	s.snapshots++
	s.live = true
}

// Snapshots returns the retained snapshots in frame order (oldest first).
func (s *DirtySink) Snapshots() []runtime.DirtySnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]runtime.DirtySnapshot, len(s.buf))
	copy(out, s.buf)
	return out
}

// Latest returns the most recent snapshot, or zero when none has been staged.
func (s *DirtySink) Latest() (runtime.DirtySnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.buf) == 0 {
		return runtime.DirtySnapshot{}, false
	}
	return s.buf[len(s.buf)-1], true
}

// Live reports whether the sink has received at least one snapshot.
func (s *DirtySink) Live() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.live
}

// Count returns the total number of snapshots staged since construction.
func (s *DirtySink) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshots
}

// Paused reports whether capture is paused.
func (s *DirtySink) Paused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused
}

// SetPaused pauses (true) or resumes (false) snapshot capture.
func (s *DirtySink) SetPaused(paused bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = paused
}

// SetRetention resizes the ring buffer (the E5 retention slider).
func (s *DirtySink) SetRetention(retention int) {
	if retention < 1 {
		retention = 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retention = retention
	if len(s.buf) > retention {
		drop := len(s.buf) - retention
		s.buf = append([]runtime.DirtySnapshot(nil), s.buf[drop:]...)
	}
}
