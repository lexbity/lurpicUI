// Command drive-gate is the LDP drive suite runner. It is one of the two
// executable consumers of internal/testevidence: the committed drive manifest
// selects required tests, and the shared engine lists, executes, verifies and
// records them. A missing, skipped, unexecuted, timed-out or wrong-mode test
// fails; an empty selection is never a success.
//
// Modes:
//
//	drive-gate --suite S1 [--count N]   run one suite's gates
//	drive-gate --format                 gofmt/parse gate over go list metadata,
//	                                    excluding the manifest's malformed
//	                                    fixture packages
//
// Stdlib only. Exit 0 means every required test ran and passed.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"codeburg.org/lexbit/lurpicui/internal/testevidence"
)

func main() {
	suite := flag.String("suite", "", "suite to run (e.g. S1)")
	count := flag.Int("count", 0, "repetition count override for the suite (0 keeps the manifest)")
	format := flag.Bool("format", false, "run the gofmt/parse gate over go list metadata")
	root := flag.String("root", ".", "repository root")
	out := flag.String("evidence", "", "evidence output dir (default build/drive-evidence)")
	flag.Parse()

	manifestPath := filepath.Join(*root, "drive", "testdata", "gates.json")
	if *format {
		manifest, err := testevidence.LoadManifest(manifestPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "drive-gate: manifest: %v\n", err)
			os.Exit(2)
		}
		if err := testevidence.FormatCheck(*root, manifest.FormatExcludes); err != nil {
			fmt.Fprintf(os.Stderr, "drive-gate: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("drive-gate: format gate passed")
		return
	}
	if *suite == "" {
		fmt.Fprintln(os.Stderr, "drive-gate: --suite or --format is required")
		os.Exit(2)
	}
	evidence := *out
	if evidence == "" {
		evidence = filepath.Join(*root, "build", "drive-evidence")
	}
	runner := &testevidence.Runner{
		Root:         *root,
		EvidenceRoot: evidence,
	}
	report, err := runner.Run(context.Background(), manifestPath, testevidence.Options{
		Phase:   *suite,
		Count:   *count,
		Backend: "software",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "drive-gate: %v\n", err)
		if report != nil {
			fmt.Fprintf(os.Stderr, "drive-gate: suite %s evidence: %s\n", *suite, report.EvidenceDir)
		}
		os.Exit(1)
	}
	fmt.Printf("drive-gate: suite %s passed (%d gate(s), evidence in %s)\n", *suite, len(report.Gates), report.EvidenceDir)
}
