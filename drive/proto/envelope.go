// Package proto defines the LDP wire envelope (QP-2): newline-delimited
// JSON over a Unix socket, one object per line, 4 MiB hard line bound.
package proto

import "encoding/json"

// MaxLineBytes is the hard input-validation bound on one wire line (NFR-9).
const MaxLineBytes = 4 << 20

// ProtocolMajor is the current protocol major. Majors are hard breaks
// (NFR-11): the handshake refuses any other value with ErrVersion.
const ProtocolMajor = 1

// Request is a client→server message.
type Request struct {
	V      int             `json:"v"`                // protocol major; MUST be 1
	ID     uint64          `json:"id"`               // client-unique
	Method string          `json:"method"`           // dot-namespaced, e.g. "sys.ping"
	Params json.RawMessage `json:"params,omitempty"` // object or omitted
}

// Response is a server→client reply to a Request.
type Response struct {
	ID     uint64          `json:"id"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
	Frame  uint64          `json:"frame"`            // epoch the response observed (QP-6)
	Cached bool            `json:"cached,omitempty"` // idempotent retry hit (QP-3)
}

// Notification is a server→client push (no subscriptions before Phase 7).
type Notification struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Error is a typed protocol error; Code is a closed enum (§3.2).
type Error struct {
	Code   ErrorCode `json:"code"`
	Detail string    `json:"detail,omitempty"`
}

// Error implements the error interface so proto errors flow through Go code.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Detail == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Detail
}

// ErrorCode is the closed error enum of §3.2.
type ErrorCode string

const (
	ErrVersion    ErrorCode = "version.mismatch"
	ErrAuth       ErrorCode = "auth.denied"
	ErrGrant      ErrorCode = "grant.denied"
	ErrAmbiguous  ErrorCode = "locator.ambiguous"
	ErrUnresolved ErrorCode = "locator.unresolved"
	ErrPreempted  ErrorCode = "input.preempted"
	ErrTimeMode   ErrorCode = "time.mode"
	ErrMutating   ErrorCode = "state.mutating"
	ErrDetached   ErrorCode = "backpressure.detached"
	ErrInternal   ErrorCode = "internal.panic"
)
