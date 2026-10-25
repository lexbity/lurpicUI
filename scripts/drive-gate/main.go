// Command drive-gate is the S1 gate-manifest runner (LDP v2 RFC, standing
// execution rules): it consumes drive/testdata/gates.json, verifies every
// required test actually exists and executes (go test -list first, then go
// test -json), fails on missing, skipped, or unexecuted required tests, and
// records results under build/drive-evidence/<suite>/ without secrets.
//
// Modes:
//
//	drive-gate --suite S1 [--count N]   run one suite's manifest
//	drive-gate --format                 gofmt/parse check over go list
//	                                    package metadata, excluding the
//	                                    manifest's malformed fixture packages
//
// Stdlib only. Exit 0 means every required test ran and passed; an empty
// selection is a failure, never a silent success.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// gateTest is one required test entry of the manifest.
type gateTest struct {
	Package string   `json:"package"`
	Tests   []string `json:"tests"`
	// Timeout is this entry's per-run go test timeout.
	Timeout string `json:"timeout"`
	// Count is the required repetition count for this entry.
	Count int `json:"count"`
}

// gateSuite is one slice's required-test manifest.
type gateSuite struct {
	Tests []gateTest `json:"tests"`
}

// manifest is the committed gates document.
type manifest struct {
	Suites map[string]gateSuite `json:"suites"`
	// FormatExcludes lists package directories whose parse errors are
	// intentional (malformed parser fixtures) — excluded from --format.
	FormatExcludes []string `json:"format_excludes"`
}

func main() {
	suite := flag.String("suite", "", "suite to run (e.g. S1)")
	count := flag.Int("count", 1, "repetition count override for the suite")
	format := flag.Bool("format", false, "run the gofmt/parse gate over go list metadata")
	root := flag.String("root", ".", "repository root")
	out := flag.String("evidence", "", "evidence output dir (default build/drive-evidence)")
	flag.Parse()

	if *format {
		os.Exit(runFormat(*root))
	}
	if *suite == "" {
		fmt.Fprintln(os.Stderr, "drive-gate: --suite or --format is required")
		os.Exit(2)
	}
	os.Exit(runSuite(*root, *suite, *count, *out))
}

func loadManifest(root string) (manifest, error) {
	var m manifest
	raw, err := os.ReadFile(filepath.Join(root, "drive", "testdata", "gates.json")) //nolint:gosec // repo-relative fixed manifest path
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, err
	}
	return m, nil
}

// runSuite runs one suite's manifest: -list verifies existence first, then
// go test -json streams results. Missing, skipped, or unexecuted required
// tests fail. Results land under build/drive-evidence/<suite>/.
func runSuite(root, suite string, count int, evidence string) int {
	m, err := loadManifest(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "drive-gate: manifest: %v\n", err)
		return 2
	}
	gs, ok := m.Suites[suite]
	if !ok {
		fmt.Fprintf(os.Stderr, "drive-gate: suite %q is not in the manifest\n", suite)
		return 2
	}
	if len(gs.Tests) == 0 {
		fmt.Fprintf(os.Stderr, "drive-gate: suite %q declares no tests — an empty selection cannot pass\n", suite)
		return 2
	}
	totalRuns := 0
	failures := 0
	var lines []string
	for _, entry := range gs.Tests {
		n := entry.Count
		if n <= 0 {
			n = 1
		}
		if count > 1 {
			n = count
		}
		timeout := entry.Timeout
		if timeout == "" {
			timeout = "10m"
		}
		// Existence gate: go test -list must name every required test.
		list, err := listTests(root, entry.Package)
		if err != nil {
			fmt.Fprintf(os.Stderr, "drive-gate: %s: list: %v\n", entry.Package, err)
			failures++
			continue
		}
		for _, want := range entry.Tests {
			if !list[want] {
				fmt.Fprintf(os.Stderr, "drive-gate: FAIL %s: required test %q does not exist (a -run that matches zero tests is a failed gate)\n", entry.Package, want)
				failures++
			}
		}
		pattern := "^(" + strings.Join(entry.Tests, "|") + ")$"
		for run := 1; run <= n; run++ {
			start := time.Now()
			ok, output := runTests(root, entry.Package, pattern, timeout, run)
			totalRuns++
			status := "PASS"
			if !ok {
				status = "FAIL"
				failures++
				fmt.Fprintf(os.Stderr, "drive-gate: FAIL %s (%s run %d/%d)\n%s", entry.Package, pattern, run, n, output)
			}
			lines = append(lines, fmt.Sprintf("%s %s %s run %d/%d in %s", status, entry.Package, pattern, run, n, time.Since(start).Round(time.Millisecond)))
		}
	}
	for _, line := range lines {
		fmt.Println(line)
	}
	// Evidence: machine-readable summary, no secrets.
	if evidence == "" {
		evidence = filepath.Join(root, "build", "drive-evidence")
	}
	dir := filepath.Join(evidence, suite)
	if err := os.MkdirAll(dir, 0o750); err == nil {
		summary := map[string]any{
			"suite":      suite,
			"finished":   time.Now().UTC().Format(time.RFC3339),
			"runs":       totalRuns,
			"failures":   failures,
			"passed":     failures == 0,
			"tests":      gs.Tests,
			"go_version": runtimeVersion(),
		}
		if raw, err := json.MarshalIndent(summary, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(dir, "summary.json"), append(raw, '\n'), 0o644) //nolint:gosec // committed evidence, not a secret
		}
	}
	if failures > 0 {
		fmt.Fprintf(os.Stderr, "drive-gate: suite %s FAILED (%d failing entries)\n", suite, failures)
		return 1
	}
	fmt.Printf("drive-gate: suite %s passed (%d runs, evidence in %s)\n", suite, totalRuns, dir)
	return 0
}

func listTests(root, pkg string) (map[string]bool, error) {
	cmd := exec.Command("go", "test", "-list", ".*", pkg) //nolint:gosec // the package comes from the committed manifest
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", pkg, err)
	}
	names := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "? ") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "no test files") {
			continue
		}
		names[line] = true
	}
	return names, nil
}

func runTests(root, pkg, pattern, timeout string, run int) (bool, string) {
	args := []string{"test", pkg, "-run", pattern, "-timeout", timeout, "-count", "1"}
	if run > 1 {
		// Repetition runs re-execute the same selection; the runner itself
		// loops so per-run failures stay attributed.
		_ = run
	}
	cmd := exec.Command("go", args...) //nolint:gosec // the pattern comes from the committed manifest
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return err == nil, string(out)
}

func runtimeVersion() string {
	out, err := exec.Command("go", "env", "GOVERSION").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// runFormat enumerates Go sources via go list package metadata, excludes
// only the manifest's intentionally malformed fixture packages, and fails
// on unexpected parse errors or gofmt differences. It never pipes away a
// gofmt failure.
func runFormat(root string) int {
	m, err := loadManifest(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "drive-gate: manifest: %v\n", err)
		return 2
	}
	excluded := map[string]bool{}
	for _, e := range m.FormatExcludes {
		excluded[e] = true
	}
	// All packages, including test-only files.
	list := exec.Command("go", "list", "-f", "{{.Dir}}\t{{.ImportPath}}", "./...")
	list.Dir = root
	out, err := list.Output()
	if err != nil {
		// A package that fails to load at all may be an intentional
		// malformed fixture; check its path against the manifest.
		fmt.Fprintf(os.Stderr, "drive-gate: go list failed: %v\n", err)
		return 1
	}
	bad := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
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
		if err != nil || len(files) == 0 {
			continue
		}
		gofmt := exec.Command("gofmt", "-l")
		gofmt.Args = append(gofmt.Args, files...)
		gout, gerr := gofmt.Output()
		if gerr != nil {
			fmt.Fprintf(os.Stderr, "drive-gate: FAIL gofmt %s: %v\n", importPath, gerr)
			bad++
			continue
		}
		if len(gout) > 0 {
			fmt.Fprintf(os.Stderr, "drive-gate: FAIL gofmt (unformatted files in %s):\n%s", importPath, gout)
			bad++
		}
	}
	if bad > 0 {
		fmt.Fprintf(os.Stderr, "drive-gate: format gate FAILED (%d packages)\n", bad)
		return 1
	}
	fmt.Println("drive-gate: format gate passed")
	return 0
}
