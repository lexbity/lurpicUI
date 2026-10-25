package main

import (
	"bufio"
	"encoding/json"
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
	// hello result retained for diagnostics and future resume flows.
	instanceID   string
	sessionID    string
	credential   string
	generation   string
	schemaDigest string
	lastCut      string
	lastFrame    string
}

func driveConnect(socket, tokenFile string) (*driveJSON, error) {
	token, err := os.ReadFile(tokenFile) //nolint:gosec // operator-supplied path by design
	if err != nil {
		return nil, fmt.Errorf("read token: %w", err)
	}
	nc, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	rw := bufio.NewReadWriter(bufio.NewReader(nc), bufio.NewWriter(nc))
	c := &driveJSON{rw: rw, conn: nc, token: strings.TrimSpace(string(token))}
	res, err := c.call("sys.hello", json.RawMessage(fmt.Sprintf(
		`{"token":%s,"client_name":"lurpic-cli","create_session":true}`,
		mustQuote(c.token))))
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
	}
	if err := json.Unmarshal(res.Result, &hr); err != nil {
		_ = nc.Close()
		return nil, fmt.Errorf("hello payload: %w", err)
	}
	c.instanceID, c.sessionID, c.credential = hr.InstanceID, hr.SessionID, hr.Credential
	c.generation, c.schemaDigest = hr.Generation, hr.SchemaDigest
	return c, nil
}

// mustQuote JSON-encodes one string; marshal of a plain string cannot fail.
func mustQuote(s string) string {
	enc, err := json.Marshal(s)
	if err != nil {
		// Unreachable for plain strings; keep the token out of the message.
		return `""`
	}
	return string(enc)
}

// Call implements gen.Transport: the typed client routes through the same
// wire round trip, recording the last reply's cut/frame attribution.
func (c *driveJSON) Call(method string, params any) (json.RawMessage, error) {
	res, err := c.call(method, params)
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
	return res.Result, nil
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
	req, err := json.Marshal(proto.Request{
		V:      proto.ProtocolMajor,
		ID:     strconv.FormatUint(c.next, 10),
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
	line, err := c.rw.ReadString('\n')
	if err != nil {
		return proto.Response{}, err
	}
	line = strings.TrimRight(line, "\n")
	res, derr := proto.DecodeResponse([]byte(line))
	if derr != nil {
		return proto.Response{}, fmt.Errorf("decode %q: %w", line, derr)
	}
	return res, nil
}

func (c *driveJSON) close() { _ = c.conn.Close() }

// driveFlagSet builds the shared drive verb flags. The major-2 CLI speaks
// hello + ping only; observation verbs return with their native v2 slices.
func driveFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("drive-flags", flag.ContinueOnError)
	fs.String("socket", "", "drive socket path")
	fs.String("token-file", "", "drive token file path")
	return fs
}
