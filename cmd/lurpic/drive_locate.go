package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"codeburg.org/lexbit/lurpicui/drive/proto"
)

// driveJSON is the shared client plumbing for the drive verbs: authenticated
// hello (session create), strict v2 request/response over the socket, one
// command per invocation. The wire envelope is the generated proto package;
// the CLI never hand-rolls message shapes.
type driveJSON struct {
	rw    *bufio.ReadWriter
	conn  net.Conn
	next  uint64
	token string
	// Hello result retained for diagnostics and the same-instance recovery path.
	instanceID          string
	sessionID           string
	credential          string
	generation          string
	schemaDigest        string
	lastCut             string
	lastFrame           string
	queuedNotifications []proto.Notification
	reconnect           func() (*driveJSON, error)
}

type driveSessionFile struct {
	InstanceID string `json:"instance_id"`
	SessionID  string `json:"session_id"`
	Credential string `json:"resume_credential"`
}

func driveConnect(socket, tokenFile, sessionFile string) (*driveJSON, error) {
	token, err := os.ReadFile(tokenFile) //nolint:gosec // operator-supplied path by design
	if err != nil {
		return nil, fmt.Errorf("read token: %w", err)
	}
	var saved *driveSessionFile
	if sessionFile != "" {
		info, statErr := os.Lstat(sessionFile)
		if statErr == nil {
			if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
				return nil, fmt.Errorf("session file must be a regular 0600 file")
			}
			contents, readErr := os.ReadFile(sessionFile) //nolint:gosec // operator-supplied credential path
			var parsed driveSessionFile
			if readErr != nil || json.Unmarshal(contents, &parsed) != nil || parsed.InstanceID == "" || parsed.SessionID == "" || parsed.Credential == "" {
				return nil, fmt.Errorf("session file is invalid")
			}
			saved = &parsed
		} else if !os.IsNotExist(statErr) {
			return nil, fmt.Errorf("inspect session file: %w", statErr)
		}
	}
	return driveConnectWithToken(socket, strings.TrimSpace(string(token)), sessionFile, saved)
}

func driveConnectWithToken(socket, token, sessionFile string, saved *driveSessionFile) (*driveJSON, error) {
	nc, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	rw := bufio.NewReadWriter(bufio.NewReader(nc), bufio.NewWriter(nc))
	c := &driveJSON{rw: rw, conn: nc, token: token}
	helloParams := map[string]any{"token": c.token, "client_name": "lurpic-cli", "create_session": true}
	if saved != nil {
		helloParams = map[string]any{"token": c.token, "client_name": "lurpic-cli", "resume": map[string]string{
			"instance_id": saved.InstanceID, "session_id": saved.SessionID, "resume_credential": saved.Credential,
		}}
	}
	params, err := json.Marshal(helloParams)
	if err != nil {
		_ = nc.Close()
		return nil, fmt.Errorf("encode hello: %w", err)
	}
	res, err := c.call("sys.hello", json.RawMessage(params))
	if err != nil {
		_ = nc.Close()
		return nil, fmt.Errorf("hello: %w", err)
	}
	if !res.OK {
		_ = nc.Close()
		return nil, fmt.Errorf("hello rejected: %s", res.Error.Error())
	}
	var hr struct {
		InstanceID   string   `json:"instance_id"`
		SessionID    string   `json:"session_id"`
		Credential   string   `json:"resume_credential"`
		Generation   string   `json:"connection_generation"`
		SchemaDigest string   `json:"schema_digest"`
		Methods      []string `json:"methods"`
		NextID       string   `json:"next_id"`
	}
	if err := json.Unmarshal(res.Result, &hr); err != nil {
		_ = nc.Close()
		return nil, fmt.Errorf("hello payload: %w", err)
	}
	resume := driveSessionFile{InstanceID: hr.InstanceID, SessionID: hr.SessionID, Credential: hr.Credential}
	if saved != nil {
		resume.Credential = saved.Credential
	}
	c.instanceID, c.sessionID, c.credential = hr.InstanceID, hr.SessionID, resume.Credential
	c.generation, c.schemaDigest = hr.Generation, hr.SchemaDigest
	if sessionFile != "" && saved == nil {
		contents, marshalErr := json.Marshal(driveSessionFile{InstanceID: hr.InstanceID, SessionID: hr.SessionID, Credential: hr.Credential})
		if marshalErr != nil {
			_ = nc.Close()
			return nil, fmt.Errorf("encode session file: %w", marshalErr)
		}
		f, createErr := os.OpenFile(sessionFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // operator-selected local credential file
		if createErr != nil {
			_ = nc.Close()
			return nil, fmt.Errorf("create session file: %w", createErr)
		}
		_, writeErr := f.Write(contents)
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			_ = os.Remove(sessionFile)
			_ = nc.Close()
			return nil, fmt.Errorf("write session file: %w", errors.Join(writeErr, closeErr))
		}
	}
	if nextID, parseErr := strconv.ParseUint(hr.NextID, 10, 64); parseErr == nil && nextID > 0 {
		c.next = nextID - 1
	}
	c.reconnect = func() (*driveJSON, error) {
		return driveConnectWithToken(socket, token, sessionFile, &resume)
	}
	return c, nil
}

// Call implements gen.Transport: the typed client routes through the same
// wire round trip, recording the last reply's cut/frame attribution.
func (c *driveJSON) Call(method string, params any) (json.RawMessage, error) {
	id := c.next + 1
	res, err := c.call(method, params)
	if err != nil && method == "cmd.execute" && c.reconnect != nil {
		if reconnectErr := c.reconnectSession(); reconnectErr != nil {
			return nil, errors.Join(err, fmt.Errorf("resume session: %w", reconnectErr))
		}
		res, err = c.callID(id, method, params)
	}
	if err != nil {
		return nil, err
	}
	if !res.OK {
		if res.Error != nil {
			return nil, res.Error
		}
		return nil, fmt.Errorf("drive: request failed")
	}
	c.lastCut, c.lastFrame = res.Cut, res.Frame
	if method == "cmd.execute" {
		var receipt struct {
			OperationID string `json:"operation_id"`
			State       string `json:"state"`
		}
		if json.Unmarshal(res.Result, &receipt) == nil && receipt.State == "accepted" {
			outcome, err := c.Await(receipt.OperationID)
			if err != nil {
				return nil, fmt.Errorf("operation %s: %w", receipt.OperationID, err)
			}
			return outcome, nil
		}
	}
	return res.Result, nil
}

// Await consumes the operation's terminal notification. This synchronous
// CLI transport has one reader; the generated client exposes this boundary
// so callers do not mistake an acceptance receipt for completion.
func (c *driveJSON) Await(operationID string) (json.RawMessage, error) {
	for {
		notification, err := c.nextNotification()
		if err != nil {
			if c.reconnect == nil {
				return nil, err
			}
			if reconnectErr := c.reconnectSession(); reconnectErr != nil {
				return nil, errors.Join(err, fmt.Errorf("resume session: %w", reconnectErr))
			}
			status, statusErr := c.call("op.status", map[string]string{"operation_id": operationID})
			if statusErr != nil {
				return nil, statusErr
			}
			if !status.OK {
				if status.Error != nil {
					return nil, status.Error
				}
				return nil, fmt.Errorf("op.status failed after reconnect")
			}
			var recovered struct {
				State   string          `json:"state"`
				Result  json.RawMessage `json:"result"`
				Failure *proto.Error    `json:"error"`
			}
			if err := json.Unmarshal(status.Result, &recovered); err != nil {
				return nil, fmt.Errorf("decode recovered operation status: %w", err)
			}
			switch recovered.State {
			case "completed":
				return recovered.Result, nil
			case "cancelled", "failed":
				if recovered.Failure != nil {
					return nil, recovered.Failure
				}
				return nil, fmt.Errorf("operation %s ended in state %q", operationID, recovered.State)
			case "running":
				// The runtime may still be finishing the disconnect fence. Keep
				// waiting for its terminal notification on the resumed session.
				continue
			default:
				return nil, fmt.Errorf("op.status returned unknown state %q", recovered.State)
			}
		}
		if notification.Method != "op.completed" {
			continue
		}
		var params struct {
			OperationID string          `json:"operation_id"`
			Outcome     json.RawMessage `json:"outcome"`
		}
		if err := json.Unmarshal(notification.Params, &params); err != nil {
			return nil, err
		}
		if params.OperationID != operationID {
			continue
		}
		var outcome struct {
			State string       `json:"state"`
			Error *proto.Error `json:"error"`
		}
		if err := json.Unmarshal(params.Outcome, &outcome); err != nil {
			return nil, err
		}
		if outcome.State != "completed" {
			if outcome.Error != nil {
				return nil, outcome.Error
			}
			return nil, fmt.Errorf("operation %s ended in state %q", operationID, outcome.State)
		}
		return params.Outcome, nil
	}
}

func (c *driveJSON) nextNotification() (proto.Notification, error) {
	for len(c.queuedNotifications) > 0 {
		notification := c.queuedNotifications[0]
		c.queuedNotifications = c.queuedNotifications[1:]
		return notification, nil
	}
	line, err := c.rw.ReadString('\n')
	if err != nil {
		return proto.Notification{}, err
	}
	var notification proto.Notification
	if err := json.Unmarshal([]byte(line), &notification); err != nil || notification.V != proto.ProtocolMajor || notification.Method == "" {
		return proto.Notification{}, fmt.Errorf("invalid operation notification")
	}
	return notification, nil
}

func (c *driveJSON) call(method string, params any) (proto.Response, error) {
	var raw json.RawMessage
	if params != nil {
		if already, ok := params.(json.RawMessage); ok {
			// Pre-encoded: keep as-is (json.Marshal would return plain
			// []byte, and the next pass would base64-encode it).
			raw = already
		} else {
			enc, err := json.Marshal(params)
			if err != nil {
				return proto.Response{}, err
			}
			raw = json.RawMessage(enc)
		}
	}
	c.next++
	return c.callID(c.next, method, raw)
}

func (c *driveJSON) callID(id uint64, method string, params any) (proto.Response, error) {
	var raw json.RawMessage
	if params != nil {
		if already, ok := params.(json.RawMessage); ok {
			raw = already
		} else {
			enc, err := json.Marshal(params)
			if err != nil {
				return proto.Response{}, err
			}
			raw = json.RawMessage(enc)
		}
	}
	if id > c.next {
		c.next = id
	}
	req, err := json.Marshal(proto.Request{
		V:      proto.ProtocolMajor,
		ID:     strconv.FormatUint(id, 10),
		Method: method,
		Params: raw,
	})
	if err != nil {
		return proto.Response{}, err
	}
	if _, err := c.rw.Write(append(req, '\n')); err != nil {
		return proto.Response{}, err
	}
	if err := c.rw.Flush(); err != nil {
		return proto.Response{}, err
	}
	for {
		line, readErr := c.rw.ReadString('\n')
		if readErr != nil {
			return proto.Response{}, readErr
		}
		line = strings.TrimRight(line, "\n")
		var header struct {
			Method string `json:"method"`
		}
		if json.Unmarshal([]byte(line), &header) == nil && header.Method != "" {
			var notification proto.Notification
			if err := json.Unmarshal([]byte(line), &notification); err != nil || notification.V != proto.ProtocolMajor {
				return proto.Response{}, fmt.Errorf("decode notification: malformed v2 message")
			}
			if len(c.queuedNotifications) >= 64 {
				return proto.Response{}, fmt.Errorf("drive: notification buffer is full")
			}
			c.queuedNotifications = append(c.queuedNotifications, notification)
			continue
		}
		res, derr := proto.DecodeResponse([]byte(line))
		if derr != nil {
			return proto.Response{}, fmt.Errorf("decode %q: %w", line, derr)
		}
		return res, nil
	}
}

func (c *driveJSON) reconnectSession() error {
	if c.reconnect == nil {
		return fmt.Errorf("session resume is unavailable")
	}
	fresh, err := c.reconnect()
	if err != nil {
		return err
	}
	old := c.conn
	queued := append([]proto.Notification(nil), c.queuedNotifications...)
	*c = *fresh
	c.queuedNotifications = append(queued, c.queuedNotifications...)
	if old != nil {
		_ = old.Close()
	}
	return nil
}

func (c *driveJSON) close() { _ = c.conn.Close() }

// driveFlagSet builds the shared drive verb flags. The major-2 CLI speaks
// hello + ping only; observation verbs return with their native v2 slices.
func driveFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("drive-flags", flag.ContinueOnError)
	fs.String("socket", "", "drive socket path")
	fs.String("token-file", "", "drive token file path")
	fs.String("session-file", "", "optional 0600 file for resuming this session")
	fs.String("name", "", "registered command name")
	fs.Uint64("version", 0, "registered command contract version")
	fs.String("operation-id", "", "operation identity for status recovery")
	return fs
}
