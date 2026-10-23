package marks

import (
	"os/exec"
	"strings"
	"testing"
)

// TestDescribe_no_unsafe_import: the marks package must not reach around the
// type system. The blanket reflect ban was lifted by RX-2 P3 (Q1): declared
// bindings walk the mark's exported struct fields with reflect at
// RegisterRoles — reflection is the binding-declaration mechanism, scoped to
// marks/base.go and fail-closed (panics on unreadable binding fields).
// Dynamic code execution remains banned.
func TestDescribe_no_unsafe_import(t *testing.T) {
	out, err := exec.Command("go", "list", "-f", "{{.Imports}}", "codeburg.org/lexbit/lurpicui/marks").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, banned := range []string{"unsafe", "os", "net", "net/http"} {
		if strings.Contains(string(out), "\""+banned+"\"") {
			t.Fatalf("marks package imports banned package %q: %s", banned, out)
		}
	}
}
