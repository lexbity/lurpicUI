package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"codeburg.org/lexbit/lurpicui/drive/proto"
)

// driveJSON is the shared client plumbing for the drive verbs: handshake,
// request/response over the socket, one command per invocation. Phase 8
// replaces this with the generated typed client.
type driveJSON struct {
	rw        *bufio.ReadWriter
	conn      net.Conn
	next      uint64
	token     string
	lastFrame uint64
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
	helloParams, err := json.Marshal(map[string]string{"token": strings.TrimSpace(string(token))})
	if err != nil {
		_ = nc.Close()
		return nil, err
	}
	res, err := c.call("sys.hello", json.RawMessage(helloParams))
	if err != nil {
		_ = nc.Close()
		return nil, fmt.Errorf("hello: %w", err)
	}
	if !res.OK {
		_ = nc.Close()
		return nil, fmt.Errorf("hello rejected: %s", res.Error.Error())
	}
	return c, nil
}

// Call implements gen.Transport: the typed client routes through the same
// wire round trip, recording the last reply's frame epoch.
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
	c.lastFrame = res.Frame
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
	req, err := json.Marshal(proto.Request{V: proto.ProtocolMajor, ID: c.next, Method: method, Params: raw})
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
	var res proto.Response
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &res); err != nil {
		return proto.Response{}, fmt.Errorf("decode %q: %w", line, err)
	}
	return res, nil
}

func (c *driveJSON) close() { _ = c.conn.Close() }

// driveFlagSet builds the shared drive verb flags.
func driveFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("drive-flags", flag.ContinueOnError)
	fs.String("socket", "", "drive socket path")
	fs.String("token-file", "", "drive token file path")
	fs.String("testid", "", "author TestID locator")
	fs.String("role", "", "accessibility role locator")
	fs.String("name", "", "accessible name (with --role)")
	fs.String("key", "", "collection key locator")
	fs.String("text", "", "accessible-name text locator")
	fs.String("kind", "", "descriptor kind locator (family/typeName)")
	fs.String("depth", "", "tree.dump depth")
	fs.String("class", "", "watch class (state.changed, tree.changed, frame.completed, own.preempted)")
	fs.String("store", "", "watch named store")
	return fs
}

// cmdDriveWatch implements `lurpic drive watch [--class C] [--store S]` —
// subscribes and streams notifications to stdout until interrupted
// (LDP QP-13).
func cmdDriveWatch(fs *flag.FlagSet, conn *driveJSON) int {
	filter := map[string]string{"class": "frame.completed"}
	if v := fs.Lookup("class"); v != nil && v.Value.String() != "" {
		filter["class"] = v.Value.String()
	}
	if v := fs.Lookup("store"); v != nil && v.Value.String() != "" {
		filter["store"] = v.Value.String()
	}
	res, err := conn.call("sub.watch", map[string]any{"filter": filter})
	if err != nil {
		fmt.Fprintf(os.Stderr, "drive: %v\n", err)
		return 1
	}
	if !res.OK {
		fmt.Fprintf(os.Stderr, "drive: %v\n", res.Error)
		return 1
	}
	fmt.Printf("subscribed id=%s\n", strings.TrimSpace(string(res.Result)))
	for {
		line, err := conn.rw.ReadString('\n')
		if err != nil {
			return 0
		}
		fmt.Print(line)
	}
}
