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
	rw   *bufio.ReadWriter
	conn net.Conn
	next uint64
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
	c := &driveJSON{rw: rw, conn: nc}
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

// cmdDriveResolve implements `lurpic drive resolve --testid ID [--role R
// --name N] [--key K] [--text T] [--kind K] [--coords X,Y]` — prints the
// resolved match or the typed locator error.
func cmdDriveResolve(fs *flag.FlagSet, conn *driveJSON) int {
	locator := map[string]string{}
	if v := fs.Lookup("testid"); v != nil && v.Value.String() != "" {
		locator["testid"] = v.Value.String()
	}
	if v := fs.Lookup("role"); v != nil && v.Value.String() != "" {
		locator["role"] = v.Value.String()
	}
	if v := fs.Lookup("name"); v != nil && v.Value.String() != "" {
		locator["name"] = v.Value.String()
	}
	if v := fs.Lookup("key"); v != nil && v.Value.String() != "" {
		locator["key"] = v.Value.String()
	}
	if v := fs.Lookup("text"); v != nil && v.Value.String() != "" {
		locator["text"] = v.Value.String()
	}
	if v := fs.Lookup("kind"); v != nil && v.Value.String() != "" {
		locator["kind"] = v.Value.String()
	}
	res, err := conn.call("locate.resolve", map[string]any{"locator": locator})
	if err != nil {
		fmt.Fprintf(os.Stderr, "drive: %v\n", err)
		return 1
	}
	if !res.OK {
		fmt.Fprintf(os.Stderr, "drive: %v\n", res.Error)
		return 1
	}
	fmt.Println(string(res.Result))
	return 0
}

// cmdDriveTree implements `lurpic drive tree [--from locator-flag...] [--depth N]`
// — prints the subtree dump as JSON.
func cmdDriveTree(fs *flag.FlagSet, conn *driveJSON) int {
	params := map[string]any{}
	if v := fs.Lookup("testid"); v != nil && v.Value.String() != "" {
		params["from"] = map[string]string{"testid": v.Value.String()}
	}
	if v := fs.Lookup("depth"); v != nil {
		var depth int
		if _, err := fmt.Sscanf(v.Value.String(), "%d", &depth); err == nil && depth > 0 {
			params["depth"] = depth
		}
	}
	res, err := conn.call("tree.dump", params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "drive: %v\n", err)
		return 1
	}
	if !res.OK {
		fmt.Fprintf(os.Stderr, "drive: %v\n", res.Error)
		return 1
	}
	fmt.Println(string(res.Result))
	return 0
}

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
	return fs
}
