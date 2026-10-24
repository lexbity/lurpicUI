// Package driveschema generates the LDP schema artifacts from the protocol
// registry (QP-15 / FR-20): a JSON Schema describing every method, and a
// typed Go client (drive/schema/gen) used by the `lurpic drive` CLI. The
// registry (drive/proto/registry.go) is the single source of truth; the
// drift gate fails on any diff between the committed artifacts and a fresh
// generation.
package driveschema

import (
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codeburg.org/lexbit/lurpicui/drive/proto"
)

// Generate writes schema.json and gen/client.go into outDir. Both artifacts
// are byte-deterministic (sorted, fixed templates) so the drift gate is a
// plain byte diff.
func Generate(outDir string) error {
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return fmt.Errorf("schema dir: %w", err)
	}
	genDir := filepath.Join(outDir, "gen")
	if err := os.MkdirAll(genDir, 0o750); err != nil {
		return fmt.Errorf("gen dir: %w", err)
	}
	schema, err := schemaJSON()
	if err != nil {
		return err
	}
	// 0644 artifacts are committed sources (the drift gate diffs them);
	// the gosec 0600 preference targets secret files, not published schema.
	if err := os.WriteFile(filepath.Join(outDir, "schema.json"), schema, 0o644); err != nil { //nolint:gosec // committed schema source, not a secret
		return fmt.Errorf("write schema.json: %w", err)
	}
	formatted, err := gofmtSource(clientGo())
	if err != nil {
		return fmt.Errorf("gofmt gen/client.go: %w", err)
	}
	if err := os.WriteFile(filepath.Join(genDir, "client.go"), formatted, 0o644); err != nil { //nolint:gosec // committed client source, not a secret
		return fmt.Errorf("write gen/client.go: %w", err)
	}
	return nil
}

// gofmtSource formats generated Go source.
func gofmtSource(src []byte) ([]byte, error) {
	return format.Source(src)
}

// schemaJSON builds the JSON Schema document: every registry method with its
// grant, mutating flag, tier, twin, and result shape.
func schemaJSON() ([]byte, error) {
	methods := make([]map[string]any, 0, len(proto.Registry))
	for _, spec := range proto.Registry {
		m := map[string]any{
			"name":     spec.Name,
			"grant":    spec.Grant,
			"mutating": spec.Mutating,
		}
		if spec.Tier != "" {
			m["tier"] = spec.Tier
		}
		if spec.Twin != "" {
			m["twin"] = spec.Twin
		}
		methods = append(methods, m)
	}
	doc := map[string]any{
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"title":       "LDP — Lurpic Drive Protocol methods",
		"protocol":    proto.ProtocolMajor,
		"description": "Generated from drive/proto/registry.go by lurpiclint drive-schema. DO NOT EDIT.",
		"methods":     methods,
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// clientName converts a dot-namespaced method to a Go identifier:
// "input.click" → "InputClick".
func clientName(method string) string {
	parts := strings.Split(method, ".")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

// clientGo renders the typed client: a transport-agnostic Client whose
// per-method calls cover every registry entry (compile-time pairing, G8.3).
func clientGo() []byte {
	var sb strings.Builder
	sb.WriteString(`// Package gen is the typed LDP client generated from
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

// Client is the typed LDP client (protocol major ` + fmt.Sprint(proto.ProtocolMajor) + `).
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
`)
	names := proto.MethodNames()
	for _, name := range names {
		sb.WriteString("\t\t\"" + name + "\",\n")
	}
	sb.WriteString("\t}\n}\n\n")

	// Deterministic method order for emission.
	sorted := append([]proto.MethodSpec(nil), proto.Registry...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	for _, spec := range sorted {
		name := clientName(spec.Name)
		switch spec.Name {
		case "sys.ping":
			sb.WriteString(`// ` + name + ` sends sys.ping and returns the payload ({"pong":true}).
func (c *Client) ` + name + `() (json.RawMessage, error) {
	return c.call("sys.ping", nil)
}

`)
		case "sys.hello":
			sb.WriteString(`// ` + name + ` performs the handshake with the token.
func (c *Client) ` + name + `(token string) (json.RawMessage, error) {
	params, err := json.Marshal(map[string]string{"token": token})
	if err != nil {
		return nil, err
	}
	return c.call("sys.hello", params)
}

`)
		case "sub.watch":
			sb.WriteString(`// ` + name + ` registers a subscription filter.
func (c *Client) ` + name + `(filter map[string]string) (json.RawMessage, error) {
	return c.call("sub.watch", map[string]any{"filter": filter})
}

`)
		case "sub.cancel":
			sb.WriteString(`// ` + name + ` removes a subscription.
func (c *Client) ` + name + `(id uint64) (json.RawMessage, error) {
	return c.call("sub.cancel", map[string]uint64{"id": id})
}

`)
		case "cmd.execute":
			sb.WriteString(`// ` + name + ` dispatches a registered command.
func (c *Client) ` + name + `(id string) (json.RawMessage, error) {
	return c.call("cmd.execute", map[string]string{"id": id})
}

`)
		case "input.type":
			sb.WriteString(`// ` + name + ` types text into the focused IME mark.
func (c *Client) ` + name + `(text string) (json.RawMessage, error) {
	return c.call("input.type", map[string]string{"text": text})
}

`)
		case "input.drag":
			sb.WriteString(`// ` + name + ` drags from (fromX,fromY) to (toX,toY).
func (c *Client) ` + name + `(fromX, fromY, toX, toY float32) (json.RawMessage, error) {
	return c.call("input.drag", map[string]float32{"from_x": fromX, "from_y": fromY, "to_x": toX, "to_y": toY})
}

`)
		case "input.scroll":
			sb.WriteString(`// ` + name + ` scrolls at (x,y) by (deltaX,deltaY).
func (c *Client) ` + name + `(x, y, deltaX, deltaY float32) (json.RawMessage, error) {
	return c.call("input.scroll", map[string]any{
		"x": x, "y": y, "delta_x": deltaX, "delta_y": deltaY,
	})
}

`)
		case "input.focus":
			sb.WriteString(`// ` + name + ` focuses the target (a locator map, e.g. {"testid":"..."}).
func (c *Client) ` + name + `(target map[string]string) (json.RawMessage, error) {
	return c.call("input.focus", map[string]any{"target": target})
}

`)
		case "store.write":
			sb.WriteString(`// ` + name + ` writes a value or collection mutation to the addressed store.
// The response carries bypassed_app_invariants (QP-12).
func (c *Client) ` + name + `(name string, value any) (json.RawMessage, error) {
	return c.call("store.write", map[string]any{"name": name, "value": value})
}

`)
		case "store.read":
			sb.WriteString(`// ` + name + ` reads a named store.
func (c *Client) ` + name + `(name string) (json.RawMessage, error) {
	return c.call("store.read", map[string]string{"name": name})
}

`)
		case "derived.read":
			sb.WriteString(`// ` + name + ` peeks a named derived (value + dirty flag).
func (c *Client) ` + name + `(name string) (json.RawMessage, error) {
	return c.call("derived.read", map[string]string{"name": name})
}

`)
		case "locate.resolve":
			sb.WriteString(`// ` + name + ` resolves a locator (a map, e.g. {"testid":"..."}).
func (c *Client) ` + name + `(locator map[string]string) (json.RawMessage, error) {
	return c.call("locate.resolve", map[string]any{"locator": locator})
}

`)
		case "facet.query":
			sb.WriteString(`// ` + name + ` queries a facet (target locator map, details flag).
func (c *Client) ` + name + `(target map[string]string, details bool) (json.RawMessage, error) {
	return c.call("facet.query", map[string]any{"target": target, "details": details})
}

`)
		case "frame.run":
			sb.WriteString(`// ` + name + ` applies steps and an advance policy (§3.5).
func (c *Client) ` + name + `(steps []map[string]any, advance map[string]any) (json.RawMessage, error) {
	return c.call("frame.run", map[string]any{"steps": steps, "advance": advance})
}

`)
		case "time.mode":
			sb.WriteString(`// ` + name + ` switches the clock ("wall" or "virtual").
func (c *Client) ` + name + `(mode string) (json.RawMessage, error) {
	return c.call("time.mode", map[string]string{"mode": mode})
}

`)
		case "time.advance":
			sb.WriteString(`// ` + name + ` advances virtual time by d seconds; the reply lands after
// exactly one complete frame (QP-5).
func (c *Client) ` + name + `(d float64) (json.RawMessage, error) {
	return c.call("time.advance", map[string]float64{"d": d})
}

`)
		default:
			// Generic shape for parameterless or x/y verbs.
			if spec.Name == "sys.info" || spec.Name == "store.list" || spec.Name == "cmd.list" ||
				spec.Name == "frame.packet" || spec.Name == "frame.pixels" || spec.Name == "tree.dump" {
				sb.WriteString(`// ` + name + ` sends ` + spec.Name + ` with optional params.
func (c *Client) ` + name + `(params any) (json.RawMessage, error) {
	return c.call("` + spec.Name + `", params)
}

`)
				continue
			}
			// input.click/press/release/move: x/y (+button).
			sb.WriteString(`// ` + name + ` sends ` + spec.Name + ` at (x,y).
func (c *Client) ` + name + `(x, y float32) (json.RawMessage, error) {
	return c.call("` + spec.Name + `", map[string]float32{"x": x, "y": y})
}

`)
		}
	}
	return []byte(sb.String())
}
