// Package gen is the typed LDP client generated from
// drive/proto/registry.go by lurpiclint drive-schema. DO NOT EDIT —
// regenerate with: lurpiclint drive-schema --out drive/schema
package gen

import (
	"encoding/json"
	"fmt"
)

// Transport sends one request and returns one response payload or a
// protocol error (the socket round trip; see cmd/lurpic driveJSON).
type Transport interface {
	Call(method string, params any) (json.RawMessage, error)
}

// Client is the typed LDP client (protocol major 2).
type Client struct {
	Transport Transport
}

// call routes one typed call through the transport.
func (c *Client) call(method string, params any) (json.RawMessage, error) {
	if c == nil || c.Transport == nil {
		return nil, fmt.Errorf("gen: nil transport")
	}
	return c.Transport.Call(method, params)
}

// Methods returns the client's method names, sorted (the advertised
// registry list the client covers; the drift gate pairs this against the
// registry).
func Methods() []string {
	return []string{
		"cmd.execute",
		"cmd.list",
		"frame.run",
		"input.click",
		"input.drag",
		"input.focus",
		"input.move",
		"input.press",
		"input.release",
		"input.scroll",
		"input.type",
		"op.status",
		"sys.hello",
		"sys.ping",
		"time.advance",
		"time.mode",
	}
}

// SchemaDigest returns the canonical schema digest of Methods().
func SchemaDigest() string {
	return "sha256:063b8ae3ce41279196244c72963603fafd0fd54294f457257449c34982953cff"
}

// CmdExecute sends cmd.execute with optional params.
func (c *Client) CmdExecute(params any) (json.RawMessage, error) {
	return c.call("cmd.execute", params)
}

// CmdList sends cmd.list with optional params.
func (c *Client) CmdList(params any) (json.RawMessage, error) {
	return c.call("cmd.list", params)
}

// FrameRun sends frame.run with optional params.
func (c *Client) FrameRun(params any) (json.RawMessage, error) {
	return c.call("frame.run", params)
}

// InputClick sends input.click with optional params.
func (c *Client) InputClick(params any) (json.RawMessage, error) {
	return c.call("input.click", params)
}

// InputDrag sends input.drag with optional params.
func (c *Client) InputDrag(params any) (json.RawMessage, error) {
	return c.call("input.drag", params)
}

// InputFocus sends input.focus with optional params.
func (c *Client) InputFocus(params any) (json.RawMessage, error) {
	return c.call("input.focus", params)
}

// InputMove sends input.move with optional params.
func (c *Client) InputMove(params any) (json.RawMessage, error) {
	return c.call("input.move", params)
}

// InputPress sends input.press with optional params.
func (c *Client) InputPress(params any) (json.RawMessage, error) {
	return c.call("input.press", params)
}

// InputRelease sends input.release with optional params.
func (c *Client) InputRelease(params any) (json.RawMessage, error) {
	return c.call("input.release", params)
}

// InputScroll sends input.scroll with optional params.
func (c *Client) InputScroll(params any) (json.RawMessage, error) {
	return c.call("input.scroll", params)
}

// InputType sends input.type with optional params.
func (c *Client) InputType(params any) (json.RawMessage, error) {
	return c.call("input.type", params)
}

// OpStatus sends op.status with optional params.
func (c *Client) OpStatus(params any) (json.RawMessage, error) {
	return c.call("op.status", params)
}

// SysHello performs the handshake: token + create_session.
func (c *Client) SysHello(token string) (json.RawMessage, error) {
	params, err := json.Marshal(map[string]any{"token": token, "client_name": "gen-client", "create_session": true})
	if err != nil {
		return nil, err
	}
	return c.call("sys.hello", params)
}

// SysPing sends sys.ping and returns the payload (pong, cut, frame).
func (c *Client) SysPing() (json.RawMessage, error) {
	return c.call("sys.ping", nil)
}

// TimeAdvance sends time.advance with optional params.
func (c *Client) TimeAdvance(params any) (json.RawMessage, error) {
	return c.call("time.advance", params)
}

// TimeMode sends time.mode with optional params.
func (c *Client) TimeMode(params any) (json.RawMessage, error) {
	return c.call("time.mode", params)
}
