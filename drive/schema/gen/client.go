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
		"sys.hello",
		"sys.ping",
	}
}

// SchemaDigest returns the canonical schema digest of Methods().
func SchemaDigest() string {
	return "sha256:4d9ae899dad43c573b7df036d31dc4592f7e0ae6ac44b68b0360d974393fc66c"
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
