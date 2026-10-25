// Package proto defines the LDP v2 wire envelope (LDP v2 RFC §Wire):
// newline-delimited JSON over a Unix socket, one object per line, 4 MiB
// hard line bound. Majors are hard breaks: no v1 adapters, aliases, or
// compatibility dispatch. Identifiers that carry uint64 precision (request
// IDs, cuts, frames) are decimal strings on the wire.
package proto

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"unicode/utf8"
)

// MaxLineBytes is the hard input-validation bound on one wire line.
const MaxLineBytes = 4 << 20

// MaxNestingDepth is the deepest allowed JSON nesting; deeper values are
// rejected before admission.
const MaxNestingDepth = 32

// ProtocolMajor is the current protocol major. The handshake refuses any
// other value with a protocol.version_mismatch error.
const ProtocolMajor = 2

// Request is a client→server message.
type Request struct {
	V      int             `json:"v"`
	ID     string          `json:"id"`     // positive decimal uint64, no leading zeros
	Method string          `json:"method"` // dot-namespaced, e.g. "sys.ping"
	Params json.RawMessage `json:"params,omitempty"`
}

// Response is a server→client reply to a Request.
type Response struct {
	V           int             `json:"v"`
	ID          string          `json:"id"`
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       *Error          `json:"error,omitempty"`
	Cut         string          `json:"cut,omitempty"`   // sealed cut the reply observes
	Frame       string          `json:"frame,omitempty"` // last submitted frame
	NonAdmitted bool            `json:"non_admitted,omitempty"`
}

// Notification is a server→client push. Every notification carries v;
// engine observations carry cut and frame.
type Notification struct {
	V      int             `json:"v"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
	Cut    string          `json:"cut,omitempty"`
	Frame  string          `json:"frame,omitempty"`
}

// Error is a typed protocol error; Code is a closed enum. Retryable means
// retry the same admission or recover per the error contract — never
// "execute the same mutation under a new ID".
type Error struct {
	Code        ErrorCode       `json:"code"`
	Detail      string          `json:"detail,omitempty"`
	Details     json.RawMessage `json:"details,omitempty"`
	Retryable   bool            `json:"retryable,omitempty"`
	NonAdmitted bool            `json:"non_admitted,omitempty"`
}

// Error implements the error interface so proto errors flow through Go code.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := string(e.Code)
	if e.Detail != "" {
		s += ": " + e.Detail
	}
	if e.NonAdmitted {
		s += " (non_admitted)"
	}
	return s
}

// ErrorCode is the closed error enum of the v2 registry.
type ErrorCode string

const (
	ErrVersionMismatch     ErrorCode = "protocol.version_mismatch"
	ErrSchemaMismatch      ErrorCode = "protocol.schema_mismatch"
	ErrMalformed           ErrorCode = "protocol.malformed"
	ErrLineLimit           ErrorCode = "protocol.line_limit"
	ErrUnknownMethod       ErrorCode = "protocol.unknown_method"
	ErrInvalidParams       ErrorCode = "protocol.invalid_params"
	ErrAuth                ErrorCode = "auth.denied"
	ErrGrant               ErrorCode = "grant.denied"
	ErrInstanceChanged     ErrorCode = "instance.changed"
	ErrSessionExpired      ErrorCode = "session.expired"
	ErrSessionFenced       ErrorCode = "session.fenced"
	ErrRequestGap          ErrorCode = "request.gap"
	ErrRequestConflict     ErrorCode = "request.conflict"
	ErrRequestExpired      ErrorCode = "request.expired"
	ErrCapacityBusy        ErrorCode = "capacity.busy"
	ErrResourceLimit       ErrorCode = "resource.limit"
	ErrOperationExpired    ErrorCode = "operation.expired"
	ErrOperationCancelled  ErrorCode = "operation.cancelled"
	ErrOperationDeadline   ErrorCode = "operation.deadline"
	ErrJobDeadline         ErrorCode = "job.deadline"
	ErrUnresolved          ErrorCode = "locator.unresolved"
	ErrAmbiguous           ErrorCode = "locator.ambiguous"
	ErrTargetUnavailable   ErrorCode = "target.unavailable"
	ErrTargetNotFocusable  ErrorCode = "target.not_focusable"
	ErrInputBusy           ErrorCode = "input.busy"
	ErrInputPreempted      ErrorCode = "input.preempted"
	ErrTimeBusy            ErrorCode = "time.busy"
	ErrTimeMode            ErrorCode = "time.mode"
	ErrGestureActive       ErrorCode = "time.gesture_active"
	ErrCommandUnavailable  ErrorCode = "command.unavailable"
	ErrCommandFailed       ErrorCode = "command.failed"
	ErrStateUnexported     ErrorCode = "state.unexported"
	ErrStateWrongKind      ErrorCode = "state.wrong_kind"
	ErrStreamExpired       ErrorCode = "stream.expired"
	ErrStreamInvalidCursor ErrorCode = "stream.invalid_cursor"
	ErrArtifactExpired     ErrorCode = "artifact.expired"
	ErrRenderUnavailable   ErrorCode = "render.unavailable"
	ErrRenderInvalidated   ErrorCode = "render.invalidated"
	ErrRenderFailed        ErrorCode = "render.failed"
	ErrInternalPanic       ErrorCode = "internal.panic"
)

// VersionError reports an envelope whose v does not match the negotiated
// major. It is a hard handshake break: the connection closes.
type VersionError struct{ Got int }

func (e *VersionError) Error() string {
	return fmt.Sprintf("client protocol major %d, server major %d", e.Got, ProtocolMajor)
}

// LineLimitError reports a wire line beyond MaxLineBytes.
type LineLimitError struct{ Size int }

func (e *LineLimitError) Error() string {
	return fmt.Sprintf("line is %d bytes, bound is %d", e.Size, MaxLineBytes)
}

// ValidateRequestID enforces the wire ID grammar: a positive uint64 in
// canonical decimal form (no sign, no leading zeros, no whitespace).
func ValidateRequestID(id string) (uint64, error) {
	if id == "" {
		return 0, errors.New("empty request id")
	}
	if len(id) > 20 {
		return 0, fmt.Errorf("request id %q exceeds uint64 decimal width", id)
	}
	if id[0] == '0' && len(id) > 1 {
		return 0, fmt.Errorf("request id %q has a leading zero", id)
	}
	for i := 0; i < len(id); i++ {
		if id[i] < '0' || id[i] > '9' {
			return 0, fmt.Errorf("request id %q is not decimal", id)
		}
	}
	v, err := strconv.ParseUint(id, 10, 64)
	if err != nil || v == 0 {
		return 0, fmt.Errorf("request id %q must be a positive uint64", id)
	}
	return v, nil
}

// ValidateNumber rejects JSON number literals that are out of float64
// range or otherwise non-finite (encoding/json accepts "1e999" as +Inf).
// The literal arrives as a json.Number token.
func ValidateNumber(n json.Number) error {
	lit := n.String()
	if _, err := strconv.ParseInt(lit, 10, 64); err == nil {
		return nil
	}
	if _, err := strconv.ParseUint(lit, 10, 64); err == nil {
		return nil
	}
	f, err := strconv.ParseFloat(lit, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return fmt.Errorf("number %q is non-finite or overflows", lit)
	}
	return nil
}

// DecodeRequest strictly decodes one wire line into a Request. It rejects:
// invalid UTF-8, non-object lines, duplicate JSON keys at any depth,
// unknown envelope fields, wrong v, non-canonical IDs, non-object params,
// nesting deeper than MaxNestingDepth, non-finite/overflowing numbers, and
// trailing data. Decoding is bounded by the line bytes; there is no
// panic/recover validation path.
func DecodeRequest(line []byte) (Request, error) {
	if len(line) > MaxLineBytes {
		return Request{}, &LineLimitError{Size: len(line)}
	}
	if !utf8.Valid(line) {
		return Request{}, errors.New("line is not valid UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()

	tok, err := dec.Token()
	if err != nil {
		return Request{}, fmt.Errorf("malformed JSON: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return Request{}, errors.New("envelope must be a JSON object")
	}

	var req Request
	seen := map[string]bool{}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return Request{}, fmt.Errorf("malformed JSON: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return Request{}, errors.New("envelope key must be a string")
		}
		if seen[key] {
			return Request{}, fmt.Errorf("duplicate envelope key %q", key)
		}
		seen[key] = true
		switch key {
		case "v":
			req.V, err = decodeInt(dec)
			if err != nil {
				return Request{}, fmt.Errorf("field v: %w", err)
			}
		case "id":
			req.ID, err = decodeString(dec)
			if err != nil {
				return Request{}, fmt.Errorf("field id: %w", err)
			}
			if _, err := ValidateRequestID(req.ID); err != nil {
				return Request{}, err
			}
		case "method":
			req.Method, err = decodeString(dec)
			if err != nil {
				return Request{}, fmt.Errorf("field method: %w", err)
			}
		case "params":
			req.Params, err = decodeParams(dec)
			if err != nil {
				return Request{}, fmt.Errorf("field params: %w", err)
			}
		default:
			return Request{}, fmt.Errorf("unknown envelope field %q", key)
		}
	}
	// Consume the closing brace.
	if _, err := dec.Token(); err != nil {
		return Request{}, fmt.Errorf("malformed JSON: %w", err)
	}
	// Only trailing whitespace may follow the object.
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		return Request{}, errors.New("trailing data after envelope object")
	}
	if req.V != ProtocolMajor {
		return Request{}, &VersionError{Got: req.V}
	}
	if req.ID == "" {
		return Request{}, errors.New("envelope is missing id")
	}
	if req.Method == "" {
		return Request{}, errors.New("envelope is missing method")
	}
	return req, nil
}

// decodeParams strictly decodes the params member: it must be present and a
// JSON object, and its entire subtree obeys the depth/duplicate/number
// rules. The raw bytes are re-emitted verbatim for the method layer.
func decodeParams(dec *json.Decoder) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("malformed JSON: %w", err)
	}
	if !utf8.Valid(raw) || !bytes.HasPrefix(bytes.TrimLeft(raw, " \t\r\n"), []byte("{")) {
		return nil, errors.New("params must be a JSON object")
	}
	sub := json.NewDecoder(bytes.NewReader(raw))
	sub.UseNumber()
	if err := walkValue(sub, 0); err != nil {
		return nil, err
	}
	// The sub-decode must consume exactly the whole raw value.
	var extra json.RawMessage
	if err := sub.Decode(&extra); err != io.EOF {
		return nil, errors.New("trailing data inside params")
	}
	return raw, nil
}

// walkValue strictly walks one JSON value: duplicate object keys, depth
// beyond MaxNestingDepth, and non-finite numbers are protocol errors.
func walkValue(dec *json.Decoder, depth int) error {
	if depth > MaxNestingDepth {
		return fmt.Errorf("nesting exceeds %d levels", MaxNestingDepth)
	}
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("malformed JSON: %w", err)
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return fmt.Errorf("malformed JSON: %w", err)
				}
				key, ok := keyTok.(string)
				if !ok {
					return errors.New("object key must be a string")
				}
				if seen[key] {
					return fmt.Errorf("duplicate key %q", key)
				}
				seen[key] = true
				if err := walkValue(dec, depth+1); err != nil {
					return err
				}
			}
			_, err := dec.Token() // closing brace
			return err
		case '[':
			for dec.More() {
				if err := walkValue(dec, depth+1); err != nil {
					return err
				}
			}
			_, err := dec.Token() // closing bracket
			return err
		default:
			return errors.New("unexpected delimiter")
		}
	case json.Number:
		return ValidateNumber(t)
	}
	return nil
}

func decodeInt(dec *json.Decoder) (int, error) {
	tok, err := dec.Token()
	if err != nil {
		return 0, fmt.Errorf("malformed JSON: %w", err)
	}
	n, ok := tok.(json.Number)
	if !ok {
		return 0, errors.New("must be a number")
	}
	v, err := strconv.ParseInt(n.String(), 10, 64)
	if err != nil {
		return 0, errors.New("must be an integer")
	}
	return int(v), nil
}

func decodeString(dec *json.Decoder) (string, error) {
	tok, err := dec.Token()
	if err != nil {
		return "", fmt.Errorf("malformed JSON: %w", err)
	}
	s, ok := tok.(string)
	if !ok {
		return "", errors.New("must be a string")
	}
	return s, nil
}

// DecodeResponse strictly decodes one server→client reply line with the
// same grammar rules as DecodeRequest.
func DecodeResponse(line []byte) (Response, error) {
	if len(line) > MaxLineBytes {
		return Response{}, &LineLimitError{Size: len(line)}
	}
	if !utf8.Valid(line) {
		return Response{}, errors.New("line is not valid UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return Response{}, fmt.Errorf("malformed JSON: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return Response{}, errors.New("envelope must be a JSON object")
	}
	var res Response
	seen := map[string]bool{}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return Response{}, fmt.Errorf("malformed JSON: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return Response{}, errors.New("envelope key must be a string")
		}
		if seen[key] {
			return Response{}, fmt.Errorf("duplicate envelope key %q", key)
		}
		seen[key] = true
		switch key {
		case "v":
			res.V, err = decodeInt(dec)
		case "id":
			res.ID, err = decodeString(dec)
		case "ok":
			var b bool
			t, terr := dec.Token()
			b, ok = t.(bool)
			if terr != nil || !ok {
				err = errors.New("ok must be a boolean")
			} else {
				res.OK = b
			}
		case "result":
			var raw json.RawMessage
			raw, err = decodeRaw(dec)
			res.Result = raw
		case "error":
			res.Error, err = decodeError(dec)
		case "cut", "frame":
			var s string
			s, err = decodeString(dec)
			if key == "cut" {
				res.Cut = s
			} else {
				res.Frame = s
			}
		case "non_admitted":
			t, terr := dec.Token()
			b, ok := t.(bool)
			if terr != nil || !ok {
				err = errors.New("non_admitted must be a boolean")
			} else {
				res.NonAdmitted = b
			}
		default:
			return Response{}, fmt.Errorf("unknown envelope field %q", key)
		}
		if err != nil {
			return Response{}, fmt.Errorf("field %s: %w", key, err)
		}
	}
	if _, err := dec.Token(); err != nil {
		return Response{}, fmt.Errorf("malformed JSON: %w", err)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		return Response{}, errors.New("trailing data after envelope object")
	}
	if res.V != ProtocolMajor {
		return Response{}, &VersionError{Got: res.V}
	}
	if res.Error != nil && res.OK {
		return Response{}, errors.New("response carries both ok and error")
	}
	if res.Error == nil && !res.OK {
		return Response{}, errors.New("failed response requires an error object")
	}
	if res.Error != nil && res.Result != nil {
		return Response{}, errors.New("error response must not carry a result")
	}
	return res, nil
}

// decodeRaw consumes one arbitrary (strictly valid) JSON value and returns
// its verbatim bytes.
func decodeRaw(dec *json.Decoder) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("malformed JSON: %w", err)
	}
	sub := json.NewDecoder(bytes.NewReader(raw))
	sub.UseNumber()
	if err := walkValue(sub, 0); err != nil {
		return nil, err
	}
	return raw, nil
}

// decodeError consumes an error object: a closed code enum plus optional
// detail/retryable/non_admitted flags.
func decodeError(dec *json.Decoder) (*Error, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("malformed JSON: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, errors.New("error must be an object")
	}
	e := &Error{}
	seen := map[string]bool{}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := keyTok.(string)
		if seen[key] {
			return nil, fmt.Errorf("duplicate error key %q", key)
		}
		seen[key] = true
		switch key {
		case "code":
			s, serr := decodeString(dec)
			if serr != nil {
				return nil, serr
			}
			if !isValidErrorCode(ErrorCode(s)) {
				return nil, fmt.Errorf("unknown error code %q", s)
			}
			e.Code = ErrorCode(s)
		case "detail":
			e.Detail, err = decodeString(dec)
		case "retryable":
			var b bool
			b, err = decodeBool(dec)
			e.Retryable = b
		case "non_admitted":
			var b bool
			b, err = decodeBool(dec)
			e.NonAdmitted = b
		default:
			return nil, fmt.Errorf("unknown error field %q", key)
		}
		if err != nil {
			return nil, err
		}
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if e.Code == "" {
		return nil, errors.New("error object requires code")
	}
	return e, nil
}

func decodeBool(dec *json.Decoder) (bool, error) {
	t, err := dec.Token()
	if err != nil {
		return false, err
	}
	b, ok := t.(bool)
	if !ok {
		return false, errors.New("must be a boolean")
	}
	return b, nil
}
