// Drive diagnostics counters (RFC §Observability): gauges and cumulative
// counters over the shared FrameStats surface. Counters are added only on
// paths their owning slice introduces; names not yet counted are not
// declared.
package drive

import "sync/atomic"

// Stats is the shared, thread-safe drive counter set. One instance is
// owned by the runtime port and shared with the server so both sides count
// into the same surface.
type Stats struct {
	commandsAdmitted      atomic.Uint64
	commandsDrained       atomic.Uint64
	responsesSent         atomic.Uint64
	grantDenied           atomic.Uint64
	socketGoroutinePanics atomic.Uint64
	handlerPanics         atomic.Uint64
	duplicateHits         atomic.Uint64
	requestConflicts      atomic.Uint64
	requestExpired        atomic.Uint64
	admissionRejected     atomic.Uint64
	preempted             atomic.Uint64
	streamRecordRefused   atomic.Uint64
}

// AddCommandsAdmitted counts ledger claims.
func (s *Stats) AddCommandsAdmitted(n int) { s.commandsAdmitted.Add(uint64(n)) } //nolint:gosec // bounded by admission

// AddCommandsDrained counts commands executed on the runtime thread.
func (s *Stats) AddCommandsDrained(n int) { s.commandsDrained.Add(uint64(n)) } //nolint:gosec // bounded by drained commands

func (s *Stats) AddResponseSent()         { s.responsesSent.Add(1) }
func (s *Stats) AddGrantDenied()          { s.grantDenied.Add(1) }
func (s *Stats) AddSocketGoroutinePanic() { s.socketGoroutinePanics.Add(1) }
func (s *Stats) AddHandlerPanic()         { s.handlerPanics.Add(1) }
func (s *Stats) AddDuplicateHit()         { s.duplicateHits.Add(1) }
func (s *Stats) AddRequestConflict()      { s.requestConflicts.Add(1) }
func (s *Stats) AddRequestExpired()       { s.requestExpired.Add(1) }
func (s *Stats) AddAdmissionRejected()    { s.admissionRejected.Add(1) }
func (s *Stats) AddPreempted()            { s.preempted.Add(1) }

// AddStreamRecordRefused counts cut records the shared log refused (an
// oversize record) — the log never truncates or silently drops contents.
func (s *Stats) AddStreamRecordRefused() { s.streamRecordRefused.Add(1) }

// Snapshot returns the cumulative counter values.
type StatsSnapshot struct {
	CommandsAdmitted      uint64
	CommandsDrained       uint64
	ResponsesSent         uint64
	GrantDenied           uint64
	SocketGoroutinePanics uint64
	HandlerPanics         uint64
	DuplicateHits         uint64
	RequestConflicts      uint64
	RequestExpired        uint64
	AdmissionRejected     uint64
	Preempted             uint64
	StreamRecordRefused   uint64
}

// Load reads the full counter set.
func (s *Stats) Load() StatsSnapshot {
	return StatsSnapshot{
		CommandsAdmitted:      s.commandsAdmitted.Load(),
		CommandsDrained:       s.commandsDrained.Load(),
		ResponsesSent:         s.responsesSent.Load(),
		GrantDenied:           s.grantDenied.Load(),
		SocketGoroutinePanics: s.socketGoroutinePanics.Load(),
		HandlerPanics:         s.handlerPanics.Load(),
		DuplicateHits:         s.duplicateHits.Load(),
		RequestConflicts:      s.requestConflicts.Load(),
		RequestExpired:        s.requestExpired.Load(),
		AdmissionRejected:     s.admissionRejected.Load(),
		Preempted:             s.preempted.Load(),
		StreamRecordRefused:   s.streamRecordRefused.Load(),
	}
}
