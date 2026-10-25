package drive_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive/proto"
)

// TestDocsMatchRegistry is the doc-registry parity gate (LDP Phase 9):
// every method in the registry appears in Documentation/drive.md's method
// table, and every method named in the doc exists in the registry. Doc drift
// is a reviewable diff, same regime as the schema drift gate.
func TestDocsMatchRegistry(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "Documentation", "drive.md"))
	if err != nil {
		t.Fatalf("read drive.md: %v", err)
	}
	doc := string(raw)

	// Forward: every ADVERTISED method (the major-2 wire surface) must be
	// documented. Unported trusted-seam entries stay out of the doc until
	// their native slice lands (D12: the advertised surface is the doc).
	advertised := proto.AdvertisedMethodNames()
	for _, m := range advertised {
		if !strings.Contains(doc, m) {
			t.Errorf("advertised method %q is missing from Documentation/drive.md — update the doc with the method", m)
		}
	}
	// The v1 direct-write surface must be gone from docs and registry.
	if strings.Contains(doc, "store.write") {
		t.Errorf("Documentation/drive.md still names store.write — the direct-write surface is deleted")
	}
	// Reverse direction: backtick-quoted dotted tokens must exist in the
	// registry (catches stale doc entries). Prose punctuation never survives
	// the backtick extraction, so no heuristics are needed.
	registrySet := map[string]bool{}
	for _, m := range proto.MethodNames() {
		registrySet[m] = true
	}
	for _, line := range strings.Split(doc, "\n") {
		if !strings.Contains(line, "`") {
			continue
		}
		for _, tok := range backtickTokens(line) {
			if !strings.Contains(tok, ".") {
				continue
			}
			if !registrySet[tok] && isLikelyMethodName(tok) {
				t.Errorf("Documentation/drive.md names method %q which is not in the registry", tok)
			}
		}
	}
}

// backtickTokens extracts backtick-delimited tokens from one line.
func backtickTokens(line string) []string {
	var out []string
	for {
		i := strings.Index(line, "`")
		if i < 0 {
			return out
		}
		rest := line[i+1:]
		j := strings.Index(rest, "`")
		if j < 0 {
			return out
		}
		out = append(out, rest[:j])
		line = rest[j+1:]
	}
}

// nonDispatchTokens are documented protocol tokens that are not dispatched
// methods: server→client events, typed error codes, and wildcards.
var nonDispatchTokens = map[string]bool{
	"sub.dead":        true,
	"own.preempted":   true,
	"input.preempted": true,
	"input.*":         true,
	"state.changed":   true,
	"tree.changed":    true,
	"frame.completed": true,
	// v2 typed error codes named in the doc.
	"request.conflict":          true,
	"request.expired":           true,
	"request.gap":               true,
	"instance.changed":          true,
	"protocol.unknown_method":   true,
	"protocol.malformed":        true,
	"protocol.line_limit":       true,
	"protocol.version_mismatch": true,
}

// isLikelyMethodName filters module paths, slash-paths, events, error codes,
// and wildcards from the reverse check.
func isLikelyMethodName(token string) bool {
	return !strings.Contains(token, "/") && !nonDispatchTokens[token]
}
