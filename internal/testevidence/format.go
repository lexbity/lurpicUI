package testevidence

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// FormatCheck runs the repository-wide gofmt gate over packages discovered
// through `go list` metadata. It moves the former drive-gate implementation
// into the shared engine so both consumers use one source enumeration and one
// honest failure rule: a package that cannot be listed is a formatting-gate
// failure unless the committed manifest explicitly excludes it as a malformed
// parser fixture.
func FormatCheck(root string, excludes []string) error {
	if root == "" {
		root = "."
	}
	excluded := make(map[string]bool, len(excludes))
	for _, path := range excludes {
		excluded[path] = true
	}
	list := exec.Command("go", "list", "-f", "{{.Dir}}\t{{.ImportPath}}", "./...")
	list.Dir = root
	var stdout, stderr bytes.Buffer
	list.Stdout = &stdout
	list.Stderr = &stderr
	if err := list.Run(); err != nil {
		return fmt.Errorf("go list: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var failures []string
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		dir, importPath := parts[0], parts[1]
		if excluded[importPath] {
			continue
		}
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			return fmt.Errorf("gofmt list %s: %w", importPath, err)
		}
		if len(files) == 0 {
			continue
		}
		gofmt := exec.Command("gofmt", append([]string{"-l"}, files...)...) //nolint:gosec // files come from go list metadata
		gofmt.Dir = root
		out, err := gofmt.Output()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: gofmt: %v", importPath, err))
			continue
		}
		if len(out) > 0 {
			failures = append(failures, fmt.Sprintf("%s: unformatted files:\n%s", importPath, out))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("format gate failed (%d package(s)):\n%s", len(failures), strings.Join(failures, "\n"))
	}
	return nil
}
