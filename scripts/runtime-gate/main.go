// Command runtime-gate executes the runtime acceptance manifest through the
// repository's shared test-evidence engine. Unsupported backend, device, and
// measurement modes are errors until their phase installs a real consumer.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"codeburg.org/lexbit/lurpicui/internal/testevidence"
)

func main() {
	root := flag.String("root", ".", "repository root")
	phase := flag.String("phase", "", "phase to run (empty selects every manifest phase)")
	mode := flag.String("mode", "", "build mode: host or race")
	count := flag.Int("count", 0, "repetition override (0 uses manifest values)")
	contracts := flag.String("contracts", "", "run source contracts for a phase or all")
	backend := flag.String("backend", "software", "render backend")
	requireBackend := flag.Bool("require-backend", false, "require the selected backend to execute")
	androidNDK := flag.Bool("android-ndk", false, "build and inspect Android artifacts")
	device := flag.Bool("device", false, "run physical-device evidence")
	measure := flag.Bool("measure", false, "run committed performance measurements")
	requirePlatform := flag.Bool("require-platform-evidence", false, "require provisioned platform evidence")
	flag.Parse()

	if *measure {
		if *phase != "P3" || *mode != "" && *mode != testevidence.ModeHost || *backend != "software" || *androidNDK || *device || *requirePlatform || *requireBackend {
			fail("P3 measurement supports only --phase P3 --backend software")
		}
		if err := runP3Measure(*root, *count); err != nil {
			fail("P3 measurement: %v", err)
		}
		return
	}
	if *androidNDK || *device || *requirePlatform || *requireBackend || *backend != "software" {
		fail("requested Android, backend, or platform-evidence path is not implemented in P3")
	}
	if *contracts != "" {
		contractPhase := *contracts
		if contractPhase == "all" {
			contractPhase = "P1,P2,P3"
		}
		if contractPhase != "P1" && contractPhase != "P2" && contractPhase != "P3" && contractPhase != "P1,P2" && contractPhase != "P1,P2,P3" {
			fail("source contracts for %s are not implemented", contractPhase)
		}
		if err := runContracts(*root, contractPhase); err != nil {
			fail("contracts: %v", err)
		}
		fmt.Printf("runtime-gate: contracts %s passed\n", contractPhase)
		return
	}
	if *phase == "" {
		fail("--phase P1 is required until later phase manifests exist")
	}
	if *phase != "P1" && *phase != "P2" && *phase != "P3" {
		fail("phase %s is not implemented", *phase)
	}
	if *mode != "" && *mode != testevidence.ModeHost && *mode != testevidence.ModeRace {
		fail("mode must be host or race, got %q", *mode)
	}
	manifest := filepath.Join(*root, "runtime", "testdata", "performance", "gates.json")
	runner := &testevidence.Runner{
		Root:         *root,
		EvidenceRoot: filepath.Join(*root, "build", "runtime-evidence"),
	}
	report, err := runner.Run(context.Background(), manifest, testevidence.Options{
		Phase:   *phase,
		Mode:    *mode,
		Count:   *count,
		Backend: *backend,
	})
	if err != nil {
		fail("%v", err)
	}
	fmt.Printf("runtime-gate: phase %s passed (%d gate(s), evidence in %s)\n", *phase, len(report.Gates), report.EvidenceDir)
}

func runP3Measure(root string, repetitions int) error {
	started := time.Now().UTC()
	if repetitions == 0 {
		repetitions = 5
	}
	if repetitions < 1 || repetitions > 20 {
		return fmt.Errorf("measurement repetitions must be in [1,20], got %d", repetitions)
	}
	command := []string{"go", "test", "-run=^$", "-bench=^BenchmarkIdentityLookupSweep$", "-benchtime=1s", fmt.Sprintf("-count=%d", repetitions), "-benchmem", "./runtime"}
	cmd := exec.Command(command[0], command[1:]...) //nolint:gosec // fixed repository-owned benchmark
	cmd.Dir = root
	var output bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &output)
	cmd.Stderr = io.MultiWriter(os.Stderr, &output)
	if err := cmd.Run(); err != nil {
		return err
	}
	revisionCmd := exec.Command("git", "rev-parse", "HEAD")
	revisionCmd.Dir = root
	revision, err := revisionCmd.Output()
	if err != nil {
		return fmt.Errorf("read source revision: %w", err)
	}
	fixture, err := os.ReadFile(filepath.Join(root, "runtime", "live_index_test.go")) //nolint:gosec // root is the explicit repository selected by the local caller
	if err != nil {
		return fmt.Errorf("read benchmark fixture: %w", err)
	}
	fixtureDigest := fmt.Sprintf("%x", sha256.Sum256(fixture))
	stamp := started.Format("20060102T150405.000000000Z")
	evidenceDir := filepath.Join(root, "build", "runtime-evidence", strings.TrimSpace(string(revision)), "P3", "measure", stamp)
	if err := os.MkdirAll(evidenceDir, 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "lookup-benchmark.txt"), output.Bytes(), 0o600); err != nil {
		return err
	}
	metadata := fmt.Sprintf("{\n  \"revision\": %q,\n  \"fixture_digest\": %q,\n  \"command\": %q,\n  \"started_utc\": %q,\n  \"sample_count\": \"10 per size\",\n  \"warmup_count\": \"benchmark harness\",\n  \"result_digest\": %q\n}\n",
		strings.TrimSpace(string(revision)), fixtureDigest, strings.Join(command, " "), started.Format(time.RFC3339Nano), fmt.Sprintf("%x", sha256.Sum256(output.Bytes())))
	if err := os.WriteFile(filepath.Join(evidenceDir, "metadata.json"), []byte(metadata), 0o600); err != nil { //nolint:gosec // evidenceDir is derived from the caller-selected repository root and fixed path segments
		return err
	}
	fmt.Printf("runtime-gate: P3 lookup measurements recorded in %s\n", evidenceDir)
	return nil
}

func runContracts(root, phase string) error {
	path := filepath.Join(root, "scripts", "runtime-gate", "check-source.sh")
	cmd := exec.Command("bash", path, root, phase) //nolint:gosec // repository-owned checker
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "runtime-gate: "+format+"\n", args...)
	os.Exit(2)
}
