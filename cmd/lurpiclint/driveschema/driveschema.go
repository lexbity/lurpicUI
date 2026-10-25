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

// schemaJSON builds the JSON Schema document: every advertised registry
// method with its grant, mutating flag, tier, twin, and result shape. The
// major-2 schema never contains an unported method (D12) or a write grant.
func schemaJSON() ([]byte, error) {
	methods := make([]map[string]any, 0, len(proto.Registry))
	for _, spec := range proto.AdvertisedRegistry() {
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
		"$schema":       "https://json-schema.org/draft/2020-12/schema",
		"title":         "LDP — Lurpic Drive Protocol methods",
		"protocol":      proto.ProtocolMajor,
		"schema_digest": proto.SchemaDigest(),
		"description":   "Generated from drive/proto/registry.go by lurpiclint drive-schema. DO NOT EDIT.",
		"methods":       methods,
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
// per-method calls cover every advertised registry entry (compile-time
// pairing). The generated method list is the major-2 wire surface only —
// unported methods are absent, and store.write cannot be generated at all.
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

// Methods returns the client's method names, sorted (the advertised
// registry list the client covers; the drift gate pairs this against the
// registry).
func Methods() []string {
	return []string{
`)
	names := proto.AdvertisedMethodNames()
	for _, name := range names {
		sb.WriteString("\t\t\"" + name + "\",\n")
	}
	sb.WriteString("\t}\n}\n\n")

	// SchemaDigest is the server-side digest this client is compatible
	// with (lurpic drive --check compares hello's schema_digest against a
	// digest of Methods() in the same canonical form).
	sb.WriteString("// SchemaDigest returns the canonical schema digest of Methods().\n")
	sb.WriteString("func SchemaDigest() string {\n")
	sb.WriteString("\treturn \"" + proto.SchemaDigest() + "\"\n")
	sb.WriteString("}\n\n")

	// Deterministic method order for emission.
	sorted := append([]proto.MethodSpec(nil), proto.AdvertisedRegistry()...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	for _, spec := range sorted {
		name := clientName(spec.Name)
		switch spec.Name {
		case "sys.ping":
			sb.WriteString("// " + name + " sends sys.ping and returns the payload (pong, cut, frame).\n" +
				"func (c *Client) " + name + "() (json.RawMessage, error) {\n" +
				"\treturn c.call(\"sys.ping\", nil)\n}\n\n")
		case "sys.hello":
			sb.WriteString("// " + name + " performs the handshake: token + create_session.\n" +
				"func (c *Client) " + name + "(token string) (json.RawMessage, error) {\n" +
				"\tparams, err := json.Marshal(map[string]any{\"token\": token, \"client_name\": \"gen-client\", \"create_session\": true})\n" +
				"\tif err != nil {\n\t\treturn nil, err\n\t}\n" +
				"\treturn c.call(\"sys.hello\", params)\n}\n\n")
		default:
			// Generic shape for future advertised methods; a native v2
			// slice replaces this with its concrete typed call.
			sb.WriteString("// " + name + " sends " + spec.Name + " with optional params.\n" +
				"func (c *Client) " + name + "(params any) (json.RawMessage, error) {\n" +
				"\treturn c.call(\"" + spec.Name + "\", params)\n}\n\n")
		}
	}
	return []byte(sb.String())
}
