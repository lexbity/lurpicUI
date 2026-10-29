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
		"derived.read",
		"facet.query",
		"frame.run",
		"input.click",
		"input.drag",
		"input.focus",
		"input.move",
		"input.press",
		"input.release",
		"input.scroll",
		"input.type",
		"locate.resolve",
		"op.cancel",
		"op.status",
		"own.state",
		"session.list",
		"session.revoke",
		"session.state",
		"store.list",
		"store.read",
		"sub.cancel",
		"sub.state",
		"sub.watch",
		"sys.hello",
		"sys.ping",
		"time.advance",
		"time.mode",
		"tree.dump",
	}
}

// SchemaDigest returns the canonical schema digest of Methods().
func SchemaDigest() string {
	return "sha256:ca321f0176b012f25fc1490de3c84a837014a5a9b02559e69cc7e40b96548979"
}

// CmdExecute sends cmd.execute with optional params.
func (c *Client) CmdExecute(params any) (json.RawMessage, error) {
	return c.call("cmd.execute", params)
}

// CmdList sends cmd.list with optional params.
func (c *Client) CmdList(params any) (json.RawMessage, error) {
	return c.call("cmd.list", params)
}

// DerivedRead sends derived.read with optional params.
func (c *Client) DerivedRead(params any) (json.RawMessage, error) {
	return c.call("derived.read", params)
}

// FacetQuery sends facet.query with optional params.
func (c *Client) FacetQuery(params any) (json.RawMessage, error) {
	return c.call("facet.query", params)
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

// LocateResolve sends locate.resolve with optional params.
func (c *Client) LocateResolve(params any) (json.RawMessage, error) {
	return c.call("locate.resolve", params)
}

// OpCancel sends op.cancel with optional params.
func (c *Client) OpCancel(params any) (json.RawMessage, error) {
	return c.call("op.cancel", params)
}

// OpStatus sends op.status with optional params.
func (c *Client) OpStatus(params any) (json.RawMessage, error) {
	return c.call("op.status", params)
}

// OwnState sends own.state with optional params.
func (c *Client) OwnState(params any) (json.RawMessage, error) {
	return c.call("own.state", params)
}

// SessionList sends session.list with optional params.
func (c *Client) SessionList(params any) (json.RawMessage, error) {
	return c.call("session.list", params)
}

// SessionRevoke sends session.revoke with optional params.
func (c *Client) SessionRevoke(params any) (json.RawMessage, error) {
	return c.call("session.revoke", params)
}

// SessionState sends session.state with optional params.
func (c *Client) SessionState(params any) (json.RawMessage, error) {
	return c.call("session.state", params)
}

// StoreList sends store.list with optional params.
func (c *Client) StoreList(params any) (json.RawMessage, error) {
	return c.call("store.list", params)
}

// StoreRead sends store.read with optional params.
func (c *Client) StoreRead(params any) (json.RawMessage, error) {
	return c.call("store.read", params)
}

// SubCancel sends sub.cancel with optional params.
func (c *Client) SubCancel(params any) (json.RawMessage, error) {
	return c.call("sub.cancel", params)
}

// SubState sends sub.state with optional params.
func (c *Client) SubState(params any) (json.RawMessage, error) {
	return c.call("sub.state", params)
}

// SubWatch sends sub.watch with optional params.
func (c *Client) SubWatch(params any) (json.RawMessage, error) {
	return c.call("sub.watch", params)
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

// TreeDump sends tree.dump with optional params.
func (c *Client) TreeDump(params any) (json.RawMessage, error) {
	return c.call("tree.dump", params)
}
