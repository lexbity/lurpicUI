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

// Client is the typed LDP client (protocol major 1).
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

// Methods returns the client's method names, sorted (the registry list the
// client covers; the drift gate pairs this against the registry).
func Methods() []string {
	return []string{
		"cmd.execute",
		"cmd.list",
		"derived.read",
		"facet.query",
		"frame.packet",
		"frame.pixels",
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
		"store.list",
		"store.read",
		"store.write",
		"sub.cancel",
		"sub.watch",
		"sys.hello",
		"sys.info",
		"sys.ping",
		"time.advance",
		"time.mode",
		"tree.dump",
	}
}

// CmdExecute dispatches a registered command.
func (c *Client) CmdExecute(id string) (json.RawMessage, error) {
	return c.call("cmd.execute", map[string]string{"id": id})
}

// CmdList sends cmd.list with optional params.
func (c *Client) CmdList(params any) (json.RawMessage, error) {
	return c.call("cmd.list", params)
}

// DerivedRead peeks a named derived (value + dirty flag).
func (c *Client) DerivedRead(name string) (json.RawMessage, error) {
	return c.call("derived.read", map[string]string{"name": name})
}

// FacetQuery queries a facet (target locator map, details flag).
func (c *Client) FacetQuery(target map[string]string, details bool) (json.RawMessage, error) {
	return c.call("facet.query", map[string]any{"target": target, "details": details})
}

// FramePacket sends frame.packet with optional params.
func (c *Client) FramePacket(params any) (json.RawMessage, error) {
	return c.call("frame.packet", params)
}

// FramePixels sends frame.pixels with optional params.
func (c *Client) FramePixels(params any) (json.RawMessage, error) {
	return c.call("frame.pixels", params)
}

// FrameRun applies steps and an advance policy (§3.5).
func (c *Client) FrameRun(steps []map[string]any, advance map[string]any) (json.RawMessage, error) {
	return c.call("frame.run", map[string]any{"steps": steps, "advance": advance})
}

// InputClick sends input.click at (x,y).
func (c *Client) InputClick(x, y float32) (json.RawMessage, error) {
	return c.call("input.click", map[string]float32{"x": x, "y": y})
}

// InputDrag drags from (fromX,fromY) to (toX,toY).
func (c *Client) InputDrag(fromX, fromY, toX, toY float32) (json.RawMessage, error) {
	return c.call("input.drag", map[string]float32{"from_x": fromX, "from_y": fromY, "to_x": toX, "to_y": toY})
}

// InputFocus focuses the target (a locator map, e.g. {"testid":"..."}).
func (c *Client) InputFocus(target map[string]string) (json.RawMessage, error) {
	return c.call("input.focus", map[string]any{"target": target})
}

// InputMove sends input.move at (x,y).
func (c *Client) InputMove(x, y float32) (json.RawMessage, error) {
	return c.call("input.move", map[string]float32{"x": x, "y": y})
}

// InputPress sends input.press at (x,y).
func (c *Client) InputPress(x, y float32) (json.RawMessage, error) {
	return c.call("input.press", map[string]float32{"x": x, "y": y})
}

// InputRelease sends input.release at (x,y).
func (c *Client) InputRelease(x, y float32) (json.RawMessage, error) {
	return c.call("input.release", map[string]float32{"x": x, "y": y})
}

// InputScroll scrolls at (x,y) by (deltaX,deltaY).
func (c *Client) InputScroll(x, y, deltaX, deltaY float32) (json.RawMessage, error) {
	return c.call("input.scroll", map[string]any{
		"x": x, "y": y, "delta_x": deltaX, "delta_y": deltaY,
	})
}

// InputType types text into the focused IME mark.
func (c *Client) InputType(text string) (json.RawMessage, error) {
	return c.call("input.type", map[string]string{"text": text})
}

// LocateResolve resolves a locator (a map, e.g. {"testid":"..."}).
func (c *Client) LocateResolve(locator map[string]string) (json.RawMessage, error) {
	return c.call("locate.resolve", map[string]any{"locator": locator})
}

// StoreList sends store.list with optional params.
func (c *Client) StoreList(params any) (json.RawMessage, error) {
	return c.call("store.list", params)
}

// StoreRead reads a named store.
func (c *Client) StoreRead(name string) (json.RawMessage, error) {
	return c.call("store.read", map[string]string{"name": name})
}

// StoreWrite writes a value or collection mutation to the addressed store.
// The response carries bypassed_app_invariants (QP-12).
func (c *Client) StoreWrite(name string, value any) (json.RawMessage, error) {
	return c.call("store.write", map[string]any{"name": name, "value": value})
}

// SubCancel removes a subscription.
func (c *Client) SubCancel(id uint64) (json.RawMessage, error) {
	return c.call("sub.cancel", map[string]uint64{"id": id})
}

// SubWatch registers a subscription filter.
func (c *Client) SubWatch(filter map[string]string) (json.RawMessage, error) {
	return c.call("sub.watch", map[string]any{"filter": filter})
}

// SysHello performs the handshake with the token.
func (c *Client) SysHello(token string) (json.RawMessage, error) {
	params, err := json.Marshal(map[string]string{"token": token})
	if err != nil {
		return nil, err
	}
	return c.call("sys.hello", params)
}

// SysInfo sends sys.info with optional params.
func (c *Client) SysInfo(params any) (json.RawMessage, error) {
	return c.call("sys.info", params)
}

// SysPing sends sys.ping and returns the payload ({"pong":true}).
func (c *Client) SysPing() (json.RawMessage, error) {
	return c.call("sys.ping", nil)
}

// TimeAdvance advances virtual time by d seconds; the reply lands after
// exactly one complete frame (QP-5).
func (c *Client) TimeAdvance(d float64) (json.RawMessage, error) {
	return c.call("time.advance", map[string]float64{"d": d})
}

// TimeMode switches the clock ("wall" or "virtual").
func (c *Client) TimeMode(mode string) (json.RawMessage, error) {
	return c.call("time.mode", map[string]string{"mode": mode})
}

// TreeDump sends tree.dump with optional params.
func (c *Client) TreeDump(params any) (json.RawMessage, error) {
	return c.call("tree.dump", params)
}
