// Port contracts (RFC §Architecture): the seam between the transport and
// the runtime. Declared in the consumer-side drive root package; neither
// production drive package imports runtime — the runtime implements the
// RuntimePort and the app installs both sides explicitly before Listen.
package drive

import (
	"encoding/json"
)

// ConnectionID identifies one transport connection within the server.
type ConnectionID uint64

// TransportReason is why a transport closed or detached.
type TransportReason string

const (
	ReasonEOF        TransportReason = "eof"
	ReasonWriteFault TransportReason = "write.fault"
	ReasonIdle       TransportReason = "idle.deadline"
	ReasonAuth       TransportReason = "auth.denied"
	ReasonServerStop TransportReason = "server.close"
)

// Command is one driver request crossing the seam toward the runtime
// thread. All values are immutable after Submit; the runtime claims the
// ledger entry before scheduling work.
type Command struct {
	// Connection is the admitting transport connection.
	Connection ConnectionID
	// Trusted marks in-process testkit commands. Socket transports never set
	// it; this keeps unported methods out of the major-2 dispatch surface.
	Trusted bool
	// Session and Generation bind the command to its admitting session and
	// connection generation (fencing).
	Session    string
	Generation uint64
	// ID is the session-scoped request sequence (positive uint64).
	ID uint64
	// OpID is the stable operation identity "instance/session/seq".
	OpID string
	// Method and Params are the validated request.
	Method string
	Params json.RawMessage
	// Grants is the immutable session grant set.
	Grants Grants
	// Accepted publishes an operation receipt before application work
	// completes. The buffered channel never blocks the runtime thread.
	Accepted chan Receipt
	// Reply is buffered 1; the runtime thread sends exactly once.
	Reply chan Result
}

// Receipt identifies an admitted operation at the cut that claimed its
// request ID.
type Receipt struct {
	OperationID   string
	AcceptedCut   string
	AcceptedFrame string
	NextID        uint64
}

// Result is the runtime thread's reply to a Command.
type Result struct {
	Payload json.RawMessage
	Err     error
	Cut     string
	Frame   string
}

// ControlSignal is an out-of-band wake for the runtime: it is never hidden
// behind data traffic and is observed before the next operation step.
type ControlSignal struct {
	Kind       ControlSignalKind
	Reason     TransportReason
	Session    string
	Generation uint64
}

// ControlSignalKind enumerates the signal classes.
type ControlSignalKind string

const (
	SignalConnectionLost ControlSignalKind = "connection.lost"
	SignalLeaseExpired   ControlSignalKind = "lease.expired"
	SignalServerStop     ControlSignalKind = "server.close"
	SignalFatal          ControlSignalKind = "fatal"
)

// PublishedSnapshot is the immutable protocol metadata the runtime
// publishes atomically; transport workers read it without touching live
// engine state. The owner sessions back the transport's keepalive lease
// enforcement (a suspended controller loses its lease; a silent observer
// does not).
type PublishedSnapshot struct {
	Cut                string // sealed cut identifier (decimal string)
	Frame              string // last submitted frame (decimal string)
	InstanceID         string
	ExportSchemaDigest string
	TimeOwnerSession   string // session owning the virtual-time lease, "" when none
	InputOwnerSession  string // session owning an in-flight input gesture, "" when none
}

// RuntimePort is the runtime side of the seam. Implementations drain at a
// cut on the runtime thread, never on socket goroutines.
type RuntimePort interface {
	// Submit enqueues one command; bounded MPSC enqueue, overload is
	// explicit (an error, never a silent drop).
	Submit(cmd Command) error
	// SignalControl delivers a loss/lease/fatal wake.
	SignalControl(sig ControlSignal)
	// Observe returns the current published protocol snapshot.
	Observe() PublishedSnapshot
}

// TransportEvents is the transport side of the seam, implemented by
// drive/server and consumed by the runtime (installed at app construction,
// before Listen).
type TransportEvents interface {
	// CloseTransport reports a transport loss so the runtime can fence the
	// affected session's accepted work at the next safe boundary.
	CloseTransport(conn ConnectionID, reason TransportReason)
}
