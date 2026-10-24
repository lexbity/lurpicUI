package schema_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/driveschema"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/drive/schema/gen"
)

func schemaDir(t testing.TB) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "schema"))
	if err != nil {
		t.Fatalf("schema dir: %v", err)
	}
	return dir
}

// TestSchemaDrift is the G8.2 gate (the FFI-inventory regime): regenerate
// into a temp dir and byte-diff against the committed artifacts. Any skew
// between the registry and the committed schema/client fails — the fix is to
// regenerate, never to hand-edit.
func TestSchemaDrift(t *testing.T) {
	tmp := t.TempDir()
	if err := driveschema.Generate(tmp); err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, rel := range []string{"schema.json", filepath.Join("gen", "client.go")} {
		committed, err := os.ReadFile(filepath.Join(schemaDir(t), rel))
		if err != nil {
			t.Fatalf("read committed %s: %v", rel, err)
		}
		fresh, err := os.ReadFile(filepath.Join(tmp, rel))
		if err != nil {
			t.Fatalf("read fresh %s: %v", rel, err)
		}
		if !bytes.Equal(committed, fresh) {
			t.Errorf("%s drifted from the registry — run: lurpiclint drive-schema --out drive/schema", rel)
		}
	}
}

// TestClientCoversRegistry is the G8.3 compile-time pairing: the generated
// client's Methods() covers every registry method (and vice versa).
func TestClientCoversRegistry(t *testing.T) {
	client := map[string]bool{}
	for _, m := range gen.Methods() {
		client[m] = true
	}
	registry := map[string]bool{}
	for _, spec := range proto.Registry {
		registry[spec.Name] = true
		if !client[spec.Name] {
			t.Errorf("registry method %q has no generated client method", spec.Name)
		}
	}
	for m := range client {
		if !registry[m] {
			t.Errorf("client method %q is not in the registry (stale generation)", m)
		}
	}
}

// TestSchemaJSONValid: the committed schema.json parses and carries the
// protocol major.
func TestSchemaJSONValid(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(schemaDir(t), "schema.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var doc struct {
		Protocol int `json:"protocol"`
		Methods  []struct {
			Name string `json:"name"`
		} `json:"methods"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("schema.json: %v", err)
	}
	if doc.Protocol != proto.ProtocolMajor {
		t.Fatalf("protocol = %d, want %d", doc.Protocol, proto.ProtocolMajor)
	}
	if len(doc.Methods) != len(proto.Registry) {
		t.Fatalf("methods = %d, want %d", len(doc.Methods), len(proto.Registry))
	}
}

// TestGenerateIdempotent is the G8.1 gate: two generations byte-match.
func TestGenerateIdempotent(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	if err := driveschema.Generate(a); err != nil {
		t.Fatalf("generate a: %v", err)
	}
	if err := driveschema.Generate(b); err != nil {
		t.Fatalf("generate b: %v", err)
	}
	for _, rel := range []string{"schema.json", filepath.Join("gen", "client.go")} {
		ca, _ := os.ReadFile(filepath.Join(a, rel))
		cb, _ := os.ReadFile(filepath.Join(b, rel))
		if !bytes.Equal(ca, cb) {
			t.Errorf("generation is not byte-deterministic for %s", rel)
		}
	}
}
