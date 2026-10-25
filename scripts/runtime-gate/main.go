// Command runtime-gate executes the runtime acceptance manifest through the
// repository's shared test-evidence engine. Unsupported backend, device, and
// measurement modes are errors until their phase installs a real consumer.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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

	if *androidNDK || *device || *measure || *requirePlatform || *requireBackend || *backend != "software" {
		fail("requested Android, backend, platform-evidence, or measurement path is not implemented in P1")
	}
	if *contracts != "" {
		contractPhase := *contracts
		if contractPhase == "all" {
			contractPhase = "P1"
		}
		if contractPhase != "P1" {
			fail("source contracts for %s are not implemented", contractPhase)
		}
		if err := runContracts(*root); err != nil {
			fail("contracts: %v", err)
		}
		fmt.Println("runtime-gate: contracts P1 passed")
		return
	}
	if *phase == "" {
		fail("--phase P1 is required until later phase manifests exist")
	}
	if *phase != "P1" {
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

func runContracts(root string) error {
	path := filepath.Join(root, "scripts", "runtime-gate", "check-source.sh")
	cmd := exec.Command("bash", path, root) //nolint:gosec // repository-owned checker
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "runtime-gate: "+format+"\n", args...)
	os.Exit(2)
}
