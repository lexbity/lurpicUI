// Command runtime-gate executes the runtime acceptance manifest through the
// repository's shared test-evidence engine. Unsupported backend and device
// modes are errors until their phase installs a real consumer.
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
	"regexp"
	"strings"
	"time"

	"codeburg.org/lexbit/lurpicui/internal/testevidence"
)

func main() {
	root := flag.String("root", ".", "repository root")
	phase := flag.String("phase", "", "phase to run (empty selects every manifest phase)")
	all := flag.Bool("all", false, "run every implemented manifest phase in dependency order")
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
		p9VulkanMeasure := *phase == "P9" && *backend == "vulkan" && *requireBackend
		softwareMeasure := (*phase == "P3" || *phase == "P4" || *phase == "P5") && *backend == "software" && !*requireBackend
		if !p9VulkanMeasure && !softwareMeasure || *mode != "" && *mode != testevidence.ModeHost || *androidNDK || *device || *requirePlatform {
			fail("measurement supports host software phases P3, P4, P5 and required host Vulkan phase P9")
		}
		benchmark, pkg, fixture := "BenchmarkIdentityLookupSweep", "./runtime", "runtime/live_index_test.go"
		if *phase == "P4" {
			benchmark, pkg, fixture = "BenchmarkTextServiceLabel", "./text", "text/service_test.go"
		} else if *phase == "P5" {
			benchmark, pkg, fixture = "BenchmarkProjectionQuietRetainedTopology", "./projection", "projection/projection_bench_test.go"
		} else if *phase == "P9" {
			benchmark, pkg, fixture = "BenchmarkVulkanSubmit_AcceptanceCorpus", "./render/vulkan", "render/vulkan/bench_test.go"
		}
		if err := runBenchmarkMeasure(*root, *phase, benchmark, pkg, fixture, *count); err != nil {
			fail("%s measurement: %v", *phase, err)
		}
		return
	}
	p9Vulkan := *phase == "P9" && *backend == "vulkan" && *requireBackend && !*androidNDK && !*device && !*requirePlatform
	if *androidNDK || *device || *requirePlatform || (*requireBackend && !p9Vulkan) || (*backend != "software" && !p9Vulkan) {
		fail("requested Android, backend, or platform-evidence path is not implemented for this phase")
	}
	if *contracts != "" {
		contractPhase := *contracts
		if contractPhase == "all" {
			contractPhase = "P1,P2,P3,P4,P5,P6,P9"
		}
		if contractPhase != "P1" && contractPhase != "P2" && contractPhase != "P3" && contractPhase != "P4" && contractPhase != "P5" && contractPhase != "P6" && contractPhase != "P9" && contractPhase != "P1,P2" && contractPhase != "P1,P2,P3" && contractPhase != "P1,P2,P3,P4" && contractPhase != "P1,P2,P3,P4,P5" && contractPhase != "P1,P2,P3,P4,P5,P6" && contractPhase != "P1,P2,P3,P4,P5,P6,P9" {
			fail("source contracts for %s are not implemented", contractPhase)
		}
		if err := runContracts(*root, contractPhase); err != nil {
			fail("contracts: %v", err)
		}
		fmt.Printf("runtime-gate: contracts %s passed\n", contractPhase)
		return
	}
	if *mode != "" && *mode != testevidence.ModeHost && *mode != testevidence.ModeRace {
		fail("mode must be host or race, got %q", *mode)
	}
	if *all && *phase != "" {
		fail("--all cannot be combined with --phase")
	}
	phases := []string{*phase}
	if *all || *phase == "" {
		phases = []string{"P1", "P2", "P3", "P4", "P5", "P6"}
	}
	manifest := filepath.Join(*root, "runtime", "testdata", "performance", "gates.json")
	runner := &testevidence.Runner{
		Root:         *root,
		EvidenceRoot: filepath.Join(*root, "build", "runtime-evidence"),
	}
	for _, phase := range phases {
		if phase != "P1" && phase != "P2" && phase != "P3" && phase != "P4" && phase != "P5" && phase != "P6" && phase != "P9" {
			fail("phase %s is not implemented", phase)
		}
		if phase == "P9" && !p9Vulkan {
			fail("P9 requires --backend vulkan --require-backend; the Vulkan gate must never pass as a software run")
		}
		report, err := runner.Run(context.Background(), manifest, testevidence.Options{
			Phase: phase, Mode: *mode, Count: *count, Backend: *backend,
		})
		if err != nil {
			fail("phase %s: %v", phase, err)
		}
		fmt.Printf("runtime-gate: phase %s passed (%d gate(s), evidence in %s)\n", phase, len(report.Gates), report.EvidenceDir)
	}
}

func runBenchmarkMeasure(root, phase, benchmark, pkg, fixturePath string, repetitions int) error {
	started := time.Now().UTC()
	if repetitions == 0 {
		repetitions = 5
	}
	if repetitions < 1 || repetitions > 20 {
		return fmt.Errorf("measurement repetitions must be in [1,20], got %d", repetitions)
	}
	command := []string{"go", "test", "-run=^$", "-bench=^" + benchmark + "$", "-benchtime=1s", fmt.Sprintf("-count=%d", repetitions), "-benchmem", pkg}
	cmd := exec.Command(command[0], command[1:]...) //nolint:gosec // fixed repository-owned benchmark
	cmd.Dir = root
	var output bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &output)
	cmd.Stderr = io.MultiWriter(os.Stderr, &output)
	if err := cmd.Run(); err != nil {
		return err
	}
	if err := validateBenchmarkOutput(output.Bytes(), benchmark); err != nil {
		return err
	}
	revisionCmd := exec.Command("git", "rev-parse", "HEAD")
	revisionCmd.Dir = root
	revision, err := revisionCmd.Output()
	if err != nil {
		return fmt.Errorf("read source revision: %w", err)
	}
	fixture, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(fixturePath))) //nolint:gosec // root and fixture path are repository-owned command inputs
	if err != nil {
		return fmt.Errorf("read benchmark fixture %s: %w", fixturePath, err)
	}
	fixtureDigest := fmt.Sprintf("%x", sha256.Sum256(fixture))
	worktreeDigest, err := benchmarkWorktreeDigest(root)
	if err != nil {
		return err
	}
	goVersion, err := exec.Command("go", "version").Output()
	if err != nil {
		return fmt.Errorf("read Go toolchain version: %w", err)
	}
	stamp := started.Format("20060102T150405.000000000Z")
	evidenceDir := filepath.Join(root, "build", "runtime-evidence", strings.TrimSpace(string(revision)), phase, "measure", stamp)
	if err := os.MkdirAll(evidenceDir, 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, strings.ToLower(phase)+"-benchmark.txt"), output.Bytes(), 0o600); err != nil {
		return err
	}
	backend := "software"
	if phase == "P9" {
		backend = "vulkan"
	}
	metadata := fmt.Sprintf("{\n  \"revision\": %q,\n  \"worktree_digest\": %q,\n  \"fixture\": %q,\n  \"fixture_digest\": %q,\n  \"toolchain\": %q,\n  \"package\": %q,\n  \"backend\": %q,\n  \"benchmark\": %q,\n  \"repetitions\": %q,\n  \"command\": %q,\n  \"started_utc\": %q,\n  \"sample_count\": %q,\n  \"warmup_count\": %q,\n  \"result_digest\": %q\n}\n",
		strings.TrimSpace(string(revision)), worktreeDigest, fixturePath, fixtureDigest, strings.TrimSpace(string(goVersion)), pkg, backend, benchmark, fmt.Sprint(repetitions), strings.Join(command, " "), started.Format(time.RFC3339Nano), fmt.Sprintf("%d per benchmark subcase", repetitions), "benchmark harness warmup", fmt.Sprintf("%x", sha256.Sum256(output.Bytes())))
	if err := os.WriteFile(filepath.Join(evidenceDir, "metadata.json"), []byte(metadata), 0o600); err != nil { //nolint:gosec // evidenceDir is derived from the caller-selected repository root and fixed path segments
		return err
	}
	fmt.Printf("runtime-gate: %s measurements recorded in %s\n", phase, evidenceDir)
	return nil
}

func validateBenchmarkOutput(output []byte, benchmark string) error {
	if bytes.Contains(output, []byte("--- SKIP: Benchmark"+benchmark)) {
		return fmt.Errorf("required benchmark %s skipped", benchmark)
	}
	benchmarkLine := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(benchmark) + `-[0-9]+[[:space:]]`)
	if !benchmarkLine.Match(output) {
		return fmt.Errorf("required benchmark %s did not produce a measurement", benchmark)
	}
	return nil
}

func benchmarkWorktreeDigest(root string) (string, error) {
	diff := exec.Command("git", "diff", "--binary", "HEAD", "--")
	diff.Dir = root
	tracked, err := diff.Output()
	if err != nil {
		return "", fmt.Errorf("read tracked worktree diff: %w", err)
	}
	untrackedCmd := exec.Command("git", "ls-files", "--others", "--exclude-standard", "-z")
	untrackedCmd.Dir = root
	names, err := untrackedCmd.Output()
	if err != nil {
		return "", fmt.Errorf("read untracked worktree files: %w", err)
	}
	h := sha256.New()
	_, _ = h.Write(tracked)
	for _, name := range bytes.Split(names, []byte{0}) {
		if len(name) == 0 {
			continue
		}
		_, _ = h.Write(name)
		content, err := os.ReadFile(filepath.Join(root, string(name))) //nolint:gosec // name comes from git's repository-local untracked-file inventory
		if err != nil {
			return "", fmt.Errorf("read untracked worktree file %q: %w", name, err)
		}
		_, _ = h.Write(content)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
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
