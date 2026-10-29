// Streaming (LDP v2 RFC D7, §Streaming): runtime cuts publish immutable
// records into ONE shared bounded log; subscribers own cursors and bounded
// credit. There are no per-client FIFOs: a stream is a (filter, position,
// credit) view over the log. A subscriber that falls below the retention
// floor expires explicitly; it never silently resumes after a gap. Terminal
// stream state stays queryable after its last notification is lost.
package drive

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// EventClass enumerates the supported stream event classes.
type EventClass string

const (
	EventStateChanged      EventClass = "state.changed"
	EventTreeChanged       EventClass = "tree.changed"
	EventFrameCompleted    EventClass = "frame.completed"
	EventFrameReady        EventClass = "frame.ready"
	EventOwnPreempted      EventClass = "own.preempted"
	EventOperationTerminal EventClass = "operation.terminal"
)

// EventClasses is the closed class set, for filter validation.
var EventClasses = []EventClass{
	EventStateChanged, EventTreeChanged, EventFrameCompleted,
	EventFrameReady, EventOwnPreempted, EventOperationTerminal,
}

// ValidEventClass reports whether c is a supported class.
func ValidEventClass(c EventClass) bool {
	for _, known := range EventClasses {
		if known == c {
			return true
		}
	}
	return false
}

// Event is one immutable observation inside a CutRecord. Its identity is
// (instance, cursor, ordinal); values are obtained through the read APIs,
// never carried here (state.changed is a version delta, not a per-write
// replay).
type Event struct {
	Ordinal uint64          `json:"ordinal"`
	Class   EventClass      `json:"class"`
	Name    string          `json:"name,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// CutRecord is one sealed immutable log entry: the events observed at one
// cut. The hub assigns Cursor; the runtime assigns Cut/Frame/Events.
type CutRecord struct {
	Cursor uint64  `json:"cursor"`
	Cut    string  `json:"cut"`
	Frame  string  `json:"frame"`
	Events []Event `json:"events"`
}

// Size bounds one record's encoded size for the retention accounting.
func (r CutRecord) Size() int {
	n := 64 // cursor/cut/frame framing overhead
	for _, ev := range r.Events {
		n += 48 + len(ev.Name) + len(ev.Payload)
	}
	return n
}

// StreamFilter selects which events a stream observes. Empty selects all.
type StreamFilter struct {
	Classes []EventClass `json:"classes,omitempty"`
	Names   []string     `json:"names,omitempty"`
}

// Clone returns an independent copy.
func (f StreamFilter) Clone() StreamFilter {
	out := StreamFilter{}
	if len(f.Classes) > 0 {
		out.Classes = append([]EventClass(nil), f.Classes...)
	}
	if len(f.Names) > 0 {
		out.Names = append([]string(nil), f.Names...)
	}
	return out
}

func (f StreamFilter) matches(ev Event) bool {
	if len(f.Classes) > 0 {
		found := false
		for _, c := range f.Classes {
			if c == ev.Class {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(f.Names) > 0 && ev.Class == EventStateChanged {
		found := false
		for _, n := range f.Names {
			if n == ev.Name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (f StreamFilter) validate() error {
	seen := map[EventClass]bool{}
	for _, c := range f.Classes {
		if !ValidEventClass(c) {
			return fmt.Errorf("drive: unknown event class %q", c)
		}
		if seen[c] {
			return fmt.Errorf("drive: duplicate event class %q", c)
		}
		seen[c] = true
	}
	for _, n := range f.Names {
		if n == "" {
			return fmt.Errorf("drive: empty filter name")
		}
	}
	return nil
}

// StreamRetention bounds the shared log. A busy app can exhaust bytes or
// count before age; expiry is explicit either way.
type StreamRetention struct {
	MaxRecords     int           // sealed records retained
	MaxBytes       int           // summed record size retained
	MaxRecordBytes int           // one record's hard bound (refusal above it)
	RetentionAge   time.Duration // age beyond which a record is evicted
}

// DefaultStreamRetention is the documented default retention contract.
var DefaultStreamRetention = StreamRetention{
	MaxRecords:     4096,
	MaxBytes:       4 << 20,
	MaxRecordBytes: 256 << 10,
	RetentionAge:   10 * time.Minute,
}

// StreamState is the lifecycle state of one subscription.
type StreamState string

const (
	StreamLive      StreamState = "live"
	StreamExpired   StreamState = "expired"
	StreamCancelled StreamState = "cancelled"
)

// Stream is one subscriber's bounded-credit view over the shared log.
// Cursors and credit belong to the subscriber; the log is shared.
type Stream struct {
	ID         string
	Session    string
	Connection ConnectionID
	Filter     StreamFilter
	Credit     int // credit window 1..8

	mu        sync.Mutex
	state     StreamState
	reason    string
	sent      uint64 // last cursor handed to the subscriber
	acked     uint64 // last cursor the subscriber acknowledged
	expiredAt time.Time
	bound     bool // transport delivery sink attached
}

// Position reports the delivery/acknowledgment cursors.
func (s *Stream) Position() (sent, acked uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sent, s.acked
}

// State reports the lifecycle state and terminal reason.
func (s *Stream) State() (StreamState, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, s.reason
}

// maxStreams bounds the live stream objects of one hub (a finite resource
// like sessions and connections; Watch refuses beyond it).
const maxStreams = 64

// StreamHub is the shared bounded log plus its live streams. The runtime
// appends at cut boundaries; the transport drains on credit and binds
// delivery. All operations are safe from any thread; the runtime thread is
// the only production Append caller.
type StreamHub struct {
	mu        sync.Mutex
	retention StreamRetention
	records   []logRecord // ascending cursor
	bytes     int
	head      uint64 // last assigned cursor
	nextID    uint64
	streams   map[string]*Stream
	now       func() time.Time
	// notify is the transport's nonblocking delivery signal: fired after an
	// append or ack when some live stream has sendable records. It must not
	// call back into the hub while the hub lock is held.
	notify func(streamID string)
}

// logRecord pairs a sealed record with its append instant (retention age).
type logRecord struct {
	CutRecord
	at time.Time
}

// NewStreamHub builds the hub with the default retention contract.
func NewStreamHub() *StreamHub {
	return NewStreamHubWithRetention(DefaultStreamRetention)
}

// NewStreamHubWithRetention builds the hub with explicit retention bounds
// (all must be positive; MaxRecordBytes ≤ MaxBytes).
func NewStreamHubWithRetention(r StreamRetention) *StreamHub {
	if r.MaxRecords <= 0 || r.MaxBytes <= 0 || r.MaxRecordBytes <= 0 || r.RetentionAge <= 0 || r.MaxRecordBytes > r.MaxBytes {
		panic("drive: invalid stream retention bounds")
	}
	return &StreamHub{
		retention: r,
		streams:   make(map[string]*Stream),
		now:       time.Now,
	}
}

// SetNow injects the wall clock (tests).
func (h *StreamHub) SetNow(now func() time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.now = now
}

// SetNotify installs the delivery signal.
func (h *StreamHub) SetNotify(fn func(streamID string)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.notify = fn
}

// Bounds reports the current retention window: floor is the oldest retained
// cursor (0 when the log is empty), head the newest assigned.
func (h *StreamHub) Bounds() (floor, head uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.boundsLocked()
}

func (h *StreamHub) boundsLocked() (uint64, uint64) {
	if len(h.records) == 0 {
		return 0, h.head
	}
	return h.records[0].Cursor, h.head
}

// Append publishes one sealed record. The hub assigns the cursor and
// enforces retention: overflow evicts the oldest records; a record beyond
// MaxRecordBytes is refused (the log never truncates a retained event's
// contents).
func (h *StreamHub) Append(rec CutRecord) error {
	h.mu.Lock()
	if rec.Size() > h.retention.MaxRecordBytes {
		h.mu.Unlock()
		return fmt.Errorf("drive: cut record of %d bytes exceeds the %d byte record bound", rec.Size(), h.retention.MaxRecordBytes)
	}
	now := h.now()
	h.head++
	rec.Cursor = h.head
	if len(rec.Events) > 0 {
		for i := range rec.Events {
			rec.Events[i].Ordinal = uint64(i)
		}
		// Records are immutable after publish; own the caller's slice.
		rec.Events = append([]Event(nil), rec.Events...)
	} else {
		rec.Events = nil
	}
	h.records = append(h.records, logRecord{CutRecord: rec, at: now})
	h.bytes += rec.Size()
	h.evictLocked(now)
	// Expiry is eager and credit-independent: a stream whose next
	// undelivered record was just evicted can never resume contiguously, so
	// it expires now whether or not it has credit to receive anything.
	expired := h.expireBelowFloorLockedAll()
	ready := h.collectSendableLocked()
	h.mu.Unlock()
	h.fireNotify(ready)
	h.fireNotify(expired)
	return nil
}

// expireBelowFloorLockedAll expires every live stream positioned below the
// retention floor (h.mu held).
func (h *StreamHub) expireBelowFloorLockedAll() []string {
	floor, _ := h.boundsLocked()
	if floor == 0 {
		return nil
	}
	now := h.now()
	var expired []string
	for id, s := range h.streams {
		s.mu.Lock()
		if s.state == StreamLive && s.sent+1 < floor {
			s.state = StreamExpired
			s.reason = "retention.exhausted"
			s.expiredAt = now
			expired = append(expired, id)
		}
		s.mu.Unlock()
	}
	return expired
}

// evictLocked drops records beyond the count, byte, or age bound.
func (h *StreamHub) evictLocked(now time.Time) {
	drop := 0
	for drop < len(h.records) {
		rec := h.records[drop]
		if len(h.records)-drop > h.retention.MaxRecords ||
			h.bytes > h.retention.MaxBytes ||
			now.Sub(rec.at) >= h.retention.RetentionAge {
			h.bytes -= rec.Size()
			drop++
			continue
		}
		break
	}
	if drop > 0 {
		h.records = append(h.records[:0], h.records[drop:]...)
	}
}

// StreamErrorKind classifies stream-contract rejections.
type StreamErrorKind string

const (
	StreamErrParams StreamErrorKind = "invalid_params"
	StreamErrCursor StreamErrorKind = "invalid_cursor"
)

// StreamError is a typed stream-contract rejection (invalid watch
// parameters vs. an unrecoverable cursor position).
type StreamError struct {
	Kind   StreamErrorKind
	Detail string
}

func (e *StreamError) Error() string { return e.Detail }

// WatchSpec is the validated sub.watch payload the hub accepts.
type WatchSpec struct {
	Session    string
	Connection ConnectionID
	Filter     StreamFilter
	After      uint64 // start position; 0 means "from the next record"
	Credit     int
}

// Watch creates a stream observing the log from After. After must name a
// recoverable position: at-or-before head, and not below the retention floor
// (a position whose following record was evicted can never be delivered
// contiguously — that is a typed refusal, not a silent gap).
func (h *StreamHub) Watch(spec WatchSpec) (*Stream, error) {
	if spec.Credit < 1 || spec.Credit > 8 {
		return nil, &StreamError{Kind: StreamErrParams, Detail: fmt.Sprintf("drive: credit window %d outside 1..8", spec.Credit)}
	}
	if err := spec.Filter.validate(); err != nil {
		return nil, &StreamError{Kind: StreamErrParams, Detail: err.Error()}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.streams) >= maxStreams {
		return nil, &StreamError{Kind: StreamErrParams, Detail: fmt.Sprintf("drive: live stream limit (%d) reached", maxStreams)}
	}
	floor, head := h.boundsLocked()
	if spec.After > head {
		return nil, &StreamError{Kind: StreamErrCursor, Detail: fmt.Sprintf("drive: after position %d is beyond the log head %d", spec.After, head)}
	}
	if spec.After > 0 && spec.After+1 < floor {
		return nil, &StreamError{Kind: StreamErrCursor, Detail: fmt.Sprintf("drive: after position %d is below the retention floor %d; the gap cannot be redelivered", spec.After, floor-1)}
	}
	h.nextID++
	s := &Stream{
		ID:         "s" + strconv.FormatUint(h.nextID, 10),
		Session:    spec.Session,
		Connection: spec.Connection,
		Filter:     spec.Filter.Clone(),
		Credit:     spec.Credit,
		state:      StreamLive,
		sent:       spec.After,
		acked:      spec.After,
	}
	h.streams[s.ID] = s
	return s, nil
}

// Get returns the stream by ID (owning-session authority checks).
func (h *StreamHub) Get(streamID string) (*Stream, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.streams[streamID]
	return s, ok
}

// BindDelivery marks the transport delivery sink attached for a stream
// owned by the given session/connection, and reports the stream's current
// state (it may expire between the watch reply and the bind).
func (h *StreamHub) BindDelivery(streamID, session string, conn ConnectionID) (StreamState, string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.streams[streamID]
	if !ok || s.Session != session || s.Connection != conn {
		return "", "unknown stream"
	}
	s.mu.Lock()
	s.bound = true
	state, reason := s.state, s.reason
	s.mu.Unlock()
	return state, reason
}

// Ack records the subscriber's acknowledged position and reports whether
// new credit freed up (the caller should drain).
func (h *StreamHub) Ack(streamID, session string, conn ConnectionID, position uint64) (StreamState, string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.streams[streamID]
	if !ok || s.Session != session || s.Connection != conn {
		return "", "unknown stream", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StreamLive {
		return s.state, s.reason, false
	}
	if position > s.sent || position < s.acked {
		return s.state, "ack position out of range", false
	}
	s.acked = position
	credit := uint64(s.Credit) - (s.sent - s.acked) //nolint:gosec // credit validated 1..8 at Watch
	sendable := false
	if credit > 0 {
		floor, _ := h.boundsLocked()
		if s.sent+1 < floor {
			sendable = true
		} else {
			for i := range h.records {
				rec := &h.records[i]
				if rec.Cursor <= s.sent {
					continue
				}
				matched := false
				for _, ev := range rec.Events {
					if s.Filter.matches(ev) {
						matched = true
						break
					}
				}
				if matched {
					sendable = true
					break
				}
				// Unmatched records don't consume credit; keep scanning.
			}
		}
	}
	return s.state, s.reason, sendable
}

// Cancel terminally cancels a stream (idempotent; repeated cancel returns
// the terminal state).
func (h *StreamHub) Cancel(streamID, session string) (StreamState, string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.streams[streamID]
	if !ok {
		return "", "unknown stream"
	}
	s.mu.Lock()
	if s.state == StreamLive {
		s.state = StreamCancelled
		s.reason = "cancelled"
		s.expiredAt = h.now()
	}
	state, reason := s.state, s.reason
	s.mu.Unlock()
	return state, reason
}

// StreamStatus is the sub.state document.
type StreamStatus struct {
	ID      string
	State   StreamState
	Reason  string
	Sent    uint64
	Acked   uint64
	Credit  int
	Floor   uint64
	Head    uint64
	Filter  StreamFilter
	Session string
}

// Status returns the full sub.state document for the owning session.
func (h *StreamHub) Status(streamID, session string) (StreamStatus, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.streams[streamID]
	if !ok || s.Session != session {
		return StreamStatus{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	floor, head := h.boundsLocked()
	return StreamStatus{
		ID: s.ID, State: s.state, Reason: s.reason, Sent: s.sent, Acked: s.acked,
		Credit: s.Credit, Floor: floor, Head: head, Filter: s.Filter.Clone(), Session: s.Session,
	}, true
}

// CloseConnection terminally cancels every stream bound to one transport
// connection (reason connection.lost): the stream object is per-connection;
// recovery is a new sub.watch at the acknowledged position.
func (h *StreamHub) CloseConnection(conn ConnectionID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	for _, s := range h.streams {
		if s.Connection != conn {
			continue
		}
		s.mu.Lock()
		if s.state == StreamLive {
			s.state = StreamCancelled
			s.reason = "connection.lost"
			s.expiredAt = now
		}
		s.mu.Unlock()
	}
}

// CloseSession cancels every stream owned by one session (revocation path).
func (h *StreamHub) CloseSession(session string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	for _, s := range h.streams {
		if s.Session != session {
			continue
		}
		s.mu.Lock()
		if s.state == StreamLive {
			s.state = StreamCancelled
			s.reason = "session.revoked"
			s.expiredAt = now
		}
		s.mu.Unlock()
	}
}

// Drain delivers the stream's sendable records: matched, contiguous from
// sent+1, within credit. If the log no longer holds the next undelivered
// record, the stream expires explicitly (retention.exhausted) and the
// drained result reports the expiry instead of a gap. Records matching
// nothing advance the position without an empty delivery, keeping the tail
// contiguous.
func (h *StreamHub) Drain(streamID, session string, conn ConnectionID) (recs []CutRecord, st StreamState, reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.streams[streamID]
	if !ok || s.Session != session || s.Connection != conn {
		return nil, "", "unknown stream"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StreamLive {
		return nil, s.state, s.reason
	}
	h.expireBelowFloorLocked(s, h.now())
	if s.state != StreamLive {
		return nil, s.state, s.reason
	}
	credit := uint64(s.Credit) - (s.sent - s.acked) //nolint:gosec // credit validated 1..8 at Watch
	for i := range h.records {
		rec := &h.records[i]
		if rec.Cursor <= s.sent {
			continue
		}
		if credit == 0 {
			break
		}
		matched := make([]Event, 0, len(rec.Events))
		for _, ev := range rec.Events {
			if s.Filter.matches(ev) {
				matched = append(matched, ev)
			}
		}
		s.sent = rec.Cursor
		if len(matched) == 0 {
			continue
		}
		recs = append(recs, CutRecord{Cursor: rec.Cursor, Cut: rec.Cut, Frame: rec.Frame, Events: matched})
		credit--
	}
	return recs, s.state, s.reason
}

// expireBelowFloorLocked expires a live stream whose next undelivered
// record was evicted (below the retention floor).
func (h *StreamHub) expireBelowFloorLocked(s *Stream, now time.Time) {
	floor, _ := h.boundsLocked()
	if floor == 0 {
		return
	}
	if s.sent+1 < floor {
		s.state = StreamExpired
		s.reason = "retention.exhausted"
		s.expiredAt = now
	}
}

// collectSendableLocked returns the IDs of live streams with sendable
// records; h.mu must be held (each stream's own lock is taken briefly).
func (h *StreamHub) collectSendableLocked() []string {
	if h.notify == nil {
		return nil
	}
	var ready []string
	for id, s := range h.streams {
		s.mu.Lock()
		live := s.state == StreamLive
		sent, acked, credit := s.sent, s.acked, s.Credit
		filter := s.Filter
		s.mu.Unlock()
		if !live {
			continue
		}
		if credit <= 0 || uint64(credit) <= sent-acked {
			continue
		}
		floor, _ := h.boundsLocked()
		if sent+1 < floor {
			ready = append(ready, id)
			continue
		}
		for i := range h.records {
			rec := &h.records[i]
			if rec.Cursor <= sent {
				continue
			}
			matched := false
			for _, ev := range rec.Events {
				if filter.matches(ev) {
					matched = true
					break
				}
			}
			if matched {
				ready = append(ready, id)
				break
			}
		}
	}
	return ready
}

// fireNotify signals the transport outside every hub lock.
func (h *StreamHub) fireNotify(ready []string) {
	if h == nil || len(ready) == 0 {
		return
	}
	h.mu.Lock()
	notify := h.notify
	h.mu.Unlock()
	if notify == nil {
		return
	}
	for _, id := range ready {
		notify(id)
	}
}

// Sweep reclaims terminal stream objects past their queryable lifetime
// (bounded residency for stream metadata; the terminal state was already
// reported to whoever asked while it existed).
func (h *StreamHub) Sweep(now time.Time) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for id, s := range h.streams {
		s.mu.Lock()
		terminal := s.state != StreamLive && !s.expiredAt.IsZero() && now.Sub(s.expiredAt) > 10*time.Minute
		s.mu.Unlock()
		if terminal {
			delete(h.streams, id)
			n++
		}
	}
	return n
}
