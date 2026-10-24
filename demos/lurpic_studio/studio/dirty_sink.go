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

	// drive counters from the last frame (LDP §3.9; zero until the drive
	// seam is enabled).
	driveCmds   uint64
	driveSent   uint64
	driveDenied uint64
	drivePanics uint64
	hasDrive    bool
}

// NewDirtySink builds a sink retaining up to retention snapshots.
func NewDirtySink(retention int) *DirtySink {
	if retention < 1 {
		retention = 1
	}
	return &DirtySink{retention: retention}
}

// OnFrame records the frame's drive counters (LDP P10): the E5 diagnostics
// overlay renders them as one row. Zero until the drive seam is enabled and
// a client has connected (FR-4/QP-13 gating).
func (s *DirtySink) OnFrame(stats diagnostics.FrameStats) {
	s.mu.Lock()
	s.driveCmds = uint64(max(0, stats.DriveCommandsDrained))
	s.driveSent = stats.DriveResponsesSent
	s.driveDenied = stats.DriveGrantDenied
	s.drivePanics = stats.DriveSocketGoroutinePanics
	s.hasDrive = true
	s.mu.Unlock()
}

// DriveCounters returns the last frame's drive counters (cmd drained, sent,
// denied, panics) and whether the drive seam reported at all.
func (s *DirtySink) DriveCounters() (uint64, uint64, uint64, uint64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.driveCmds, s.driveSent, s.driveDenied, s.drivePanics, s.hasDrive
}

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
