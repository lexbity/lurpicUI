package testevidence

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixtureModule writes a self-contained Go module so the evidence runner
// spawns real `go test` processes. The tests are tiny but actual binaries: a
// fake success sink would prove nothing about skip, completion or watchdog
// handling.
func fixtureModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	writeFixtureFile(t, dir, "go.mod", "module fixture\n\ngo 1.21\n")
	for name, body := range files {
		writeFixtureFile(t, dir, name, body)
	}
	return dir
}

func writeFixtureFile(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

func fixtureManifest(t *testing.T, gates []Gate) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gates.json")
	raw, err := json.Marshal(Manifest{Version: 1, Gates: gates})
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func fixtureGate(pkg string, tests ...string) Gate {
	return Gate{
		ID:               "fixture-gate",
		Phase:            "fixture",
		Package:          pkg,
		Tests:            tests,
		Mode:             ModeHost,
		Required:         true,
		Repetitions:      1,
		TestBudgetMS:     10000,
		PackageTimeoutMS: 30000,
	}
}

func runFixture(t *testing.T, root, manifest string, opts Options) (*Report, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	runner := &Runner{Root: root, EvidenceRoot: t.TempDir()}
	return runner.Run(ctx, manifest, opts)
}

func TestRequiredSkipFails(t *testing.T) {
	root := fixtureModule(t, map[string]string{
		"skip_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestSkips(t *testing.T) { t.Skip(\"deliberate\") }\n",
	})
	report, err := runFixture(t, root, fixtureManifest(t, []Gate{fixtureGate(".", "TestSkips")}), Options{})
	if err == nil {
		t.Fatal("a required skip must fail the gate")
	}
	if report == nil || report.Passed {
		t.Fatalf("report = %+v", report)
	}
	if len(report.Gates) != 1 || report.Gates[0].Passed {
		t.Fatalf("gate result = %+v", report.Gates)
	}
}

func TestRequiredSubtestSkipFails(t *testing.T) {
	root := fixtureModule(t, map[string]string{
		"subtest_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestParent(t *testing.T) {\n\tt.Run(\"child\", func(t *testing.T) { t.Skip(\"deliberate\") })\n}\n",
	})
	report, err := runFixture(t, root, fixtureManifest(t, []Gate{fixtureGate(".", "TestParent/child")}), Options{})
	if err == nil {
		t.Fatal("a required subtest skip must fail the gate")
	}
	if report == nil || report.Passed {
		t.Fatalf("report = %+v", report)
	}
}

func TestMissingSelectionFails(t *testing.T) {
	root := fixtureModule(t, map[string]string{
		"ok_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestPresent(t *testing.T) {}\n",
	})
	report, err := runFixture(t, root, fixtureManifest(t, []Gate{fixtureGate(".", "TestAbsent")}), Options{})
	if err == nil {
		t.Fatal("a required test that does not exist must fail the gate")
	}
	if report == nil || len(report.Gates) != 1 || report.Gates[0].Passed {
		t.Fatalf("report = %+v", report)
	}
	if len(report.Gates[0].Failures) == 0 || !strings.Contains(strings.Join(report.Gates[0].Failures, " "), "TestAbsent") {
		t.Fatalf("failure does not name the missing test: %+v", report.Gates[0].Failures)
	}
}

func TestWrongBuildModeFails(t *testing.T) {
	root := fixtureModule(t, map[string]string{
		"always_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestAlways(t *testing.T) {}\n",
		"race_test.go":   "//go:build race\n\npackage fixture\n\nimport \"testing\"\n\nfunc TestOnlyRace(t *testing.T) {}\n",
	})
	gate := fixtureGate(".", "TestOnlyRace")
	gate.Mode = ModeHost
	report, err := runFixture(t, root, fixtureManifest(t, []Gate{gate}), Options{})
	if err == nil {
		t.Fatal("a race-only test selected in host mode must fail, not silently pass")
	}
	if report == nil || report.Gates[0].Passed {
		t.Fatalf("report = %+v", report)
	}
}

func TestWatchdogTerminatesStalledBinary(t *testing.T) {
	root := fixtureModule(t, map[string]string{
		"stall_test.go": "package fixture\n\nimport (\n\t\"testing\"\n\t\"time\"\n)\n\nfunc TestStalls(t *testing.T) { time.Sleep(30 * time.Second) }\n",
	})
	gate := fixtureGate(".", "TestStalls")
	gate.TestBudgetMS = 200
	gate.PackageTimeoutMS = 20000
	start := time.Now()
	report, err := runFixture(t, root, fixtureManifest(t, []Gate{gate}), Options{})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("a stalled test must fail the gate")
	}
	if elapsed > 10*time.Second {
		t.Fatalf("watchdog took %s; it did not interrupt the stalled binary", elapsed)
	}
	if report == nil || len(report.Gates) != 1 || len(report.Gates[0].Runs) != 1 {
		t.Fatalf("report = %+v", report)
	}
	run := report.Gates[0].Runs[0]
	if !run.TimedOut || run.BudgetFailed == "" {
		t.Fatalf("stalled run = %+v, want timed out with a budget failure", run)
	}
}

func TestTruncatedJSONFails(t *testing.T) {
	root := fixtureModule(t, map[string]string{
		"ok_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestOk(t *testing.T) {}\n",
	})
	fakeGo := filepath.Join(root, "fake-go")
	script := `#!/bin/sh
case "$2" in
-list)
  echo TestOk
  exit 0
  ;;
-json)
  printf '%s\n' '{"Action":"run","Package":"fixture","Test":"TestOk"}'
  printf '%s\n' 'this is not a test event'
  exit 0
  ;;
esac
exit 0
`
	if err := os.WriteFile(fakeGo, []byte(script), 0o755); err != nil {
		t.Fatalf("fake go: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	runner := &Runner{Root: root, GoCmd: fakeGo, EvidenceRoot: t.TempDir()}
	report, err := runner.Run(ctx, fixtureManifest(t, []Gate{fixtureGate(".", "TestOk")}), Options{})
	if err == nil {
		t.Fatal("a truncated JSON stream must fail the gate even when the process exits zero")
	}
	if report == nil || len(report.Gates) != 1 || report.Gates[0].Passed {
		t.Fatalf("report = %+v", report)
	}
	if tail := report.Gates[0].Runs[0].OutputTail; !strings.Contains(tail, "malformed") {
		t.Fatalf("output tail %q does not report the malformed stream", tail)
	}
}

func TestMissingArtifactFails(t *testing.T) {
	root := fixtureModule(t, map[string]string{
		"pass_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {}\n",
	})
	gate := fixtureGate(".", "TestPass")
	gate.Artifacts = []string{"proof.json"}
	report, err := runFixture(t, root, fixtureManifest(t, []Gate{gate}), Options{})
	if err == nil {
		t.Fatal("a missing declared artifact must fail the gate")
	}
	if report == nil || report.Gates[0].Passed {
		t.Fatalf("report = %+v", report)
	}
	if !strings.Contains(strings.Join(report.Gates[0].Failures, " "), "proof.json") {
		t.Fatalf("failures = %+v", report.Gates[0].Failures)
	}
}

// TestPassingGateWritesEvidence is the positive control: the same path that
// rejects missing artifacts must accept a gate that actually wrote its
// declared evidence, and must persist the summary and per-gate record.
func TestPassingGateWritesEvidence(t *testing.T) {
	root := fixtureModule(t, map[string]string{
		"artifact_test.go": "package fixture\n\nimport (\n\t\"os\"\n\t\"path/filepath\"\n\t\"testing\"\n)\n\nfunc TestWritesArtifact(t *testing.T) {\n\tdir := os.Getenv(\"TESTEVIDENCE_ARTIFACT_DIR\")\n\tif dir == \"\" { t.Fatal(\"artifact dir missing\") }\n\tif err := os.WriteFile(filepath.Join(dir, \"proof.json\"), []byte(\"{}\"), 0o644); err != nil { t.Fatal(err) }\n}\n",
	})
	gate := fixtureGate(".", "TestWritesArtifact")
	gate.Artifacts = []string{"proof.json"}
	report, err := runFixture(t, root, fixtureManifest(t, []Gate{gate}), Options{})
	if err != nil {
		t.Fatalf("passing gate: %v (%+v)", err, report)
	}
	if report == nil || !report.Passed {
		t.Fatalf("report = %+v", report)
	}
	if _, err := os.Stat(filepath.Join(report.EvidenceDir, "summary.json")); err != nil {
		t.Fatalf("summary: %v", err)
	}
	if _, err := os.Stat(filepath.Join(report.EvidenceDir, "fixture-gate", "gate.json")); err != nil {
		t.Fatalf("per-gate evidence: %v", err)
	}
	if _, err := os.Stat(filepath.Join(report.EvidenceDir, "fixture-gate", "proof.json")); err != nil {
		t.Fatalf("declared artifact: %v", err)
	}
}
