package main

import (
	"flag"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/capindex"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/config"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/rules"
)

const version = "0.1.0-dev"

// checkFlags holds parsed flags for the `check` subcommand.
type checkFlags struct {
	format       string
	severity     string
	failOn       string
	config       string
	baseline     string
	rules        string
	noSuggest    bool
	includeTests bool
	root         string
}

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	if len(args) < 2 {
		printUsage()
		return 2
	}

	switch args[1] {
	case "check":
		return runCheck(args[2:])
	case "capabilities":
		return runCapabilities(args[2:])
	case "explain":
		return runExplain(args[2:])
	case "version":
		return runVersion()
	case "help", "-h", "--help":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", args[1])
		printUsage()
		return 2
	}
}

func runCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	cliflags := checkFlags{}
	fs.StringVar(&cliflags.format, "format", "text", "output format (text, json, github)")
	fs.StringVar(&cliflags.severity, "severity", "warn", "minimum severity to report (info, warn, error)")
	fs.StringVar(&cliflags.failOn, "fail-on", "error", "minimum severity that forces non-zero exit (info, warn, error)")
	fs.StringVar(&cliflags.config, "config", "", "path to .lurpiclint.toml (auto-discovered from cwd)")
	fs.StringVar(&cliflags.baseline, "baseline", "", "suppress findings recorded in a baseline file")
	fs.StringVar(&cliflags.rules, "rules", "", "comma-separated list of rules to enable")
	fs.BoolVar(&cliflags.noSuggest, "no-suggest", false, "disable info-level shape-match suggestions")
	fs.BoolVar(&cliflags.includeTests, "include-tests", false, "include _test.go files in analysis")
	fs.StringVar(&cliflags.root, "root", "", "module root for capability introspection")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Validate flag values.
	switch cliflags.format {
	case "text", "json", "github":
	default:
		fmt.Fprintf(os.Stderr, "error: invalid --format value %q (valid: text, json, github)\n", cliflags.format)
		return 2
	}
	if _, ok := diag.SeverityFromString(cliflags.severity); !ok {
		fmt.Fprintf(os.Stderr, "error: invalid --severity value %q (valid: info, warn, error)\n", cliflags.severity)
		return 2
	}
	if _, ok := diag.SeverityFromString(cliflags.failOn); !ok {
		fmt.Fprintf(os.Stderr, "error: invalid --fail-on value %q (valid: info, warn, error)\n", cliflags.failOn)
		return 2
	}

	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"."}
	}

	loadCfg := loader.Config{
		IncludeTests: cliflags.includeTests,
		Root:         cliflags.root,
	}

	result, err := loader.Load(patterns, loadCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 3
	}

	// Build the reporter.
	reporter, err := diag.NewReporter(cliflags.format, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 3
	}

	// Parse --rules flag into enabled-IDs list.
	var enabledIDs []string
	if cliflags.rules != "" {
		for _, id := range strings.Split(cliflags.rules, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				enabledIDs = append(enabledIDs, id)
			}
		}
	}

	// Build capindex for shape-match suggestions (LL004).
	var capabilities []capindex.Capability
	root := cliflags.root
	if root == "" {
		root = findModuleRoot()
	}
	if root != "" {
		capResult, capErr := loader.Load([]string{
			root + "/marks/...",
			root + "/layout/...",
			root + "/facet",
		}, loader.Config{})
		if capErr == nil {
			capabilities = capindex.Scan(capResult, capindex.ScanConfig{
				ModulePath: "codeburg.org/lexbit/lurpicui",
				ModuleRoot: root,
			})
		}
	}

	// ---- Config file ----
	var cfgFile *config.Config
	configPath := cliflags.config
	if configPath == "" {
		var found string
		found, err = config.Discover(".")
		if err == nil && found != "" {
			var loaded config.Config
			loaded, err = config.LoadFile(found)
			if err == nil {
				cfgFile = &loaded
			}
		}
	} else if configPath != "" {
		if _, statErr := os.Stat(configPath); statErr == nil {
			var loaded config.Config
			loaded, loadErr := config.LoadFile(configPath)
			if loadErr != nil {
				fmt.Fprintf(os.Stderr, "error: loading config: %v\n", loadErr)
				return 2
			}
			cfgFile = &loaded
		}
	}

	// ---- Path exclusion ----
	if cfgFile != nil && len(cfgFile.Paths.Exclude) > 0 {
		var filteredFiles []*loader.ParsedFile
		for _, f := range result.Files {
			if !cfgFile.PathExcluded(f.Path) {
				filteredFiles = append(filteredFiles, f)
			}
		}
		result.Files = filteredFiles
	}

	// ---- Severity overrides (config >> defaults, flag >> config) ----
	severityOverrides := make(map[string]diag.Severity)
	if cfgFile != nil {
		for ruleID, rc := range cfgFile.Rules {
			if sev, ok := diag.SeverityFromString(rc.Severity); ok && sev != diag.SeverityInfo {
				severityOverrides[ruleID] = sev
			}
		}
	}
	if cliflags.noSuggest {
		severityOverrides["LL004"] = diag.SeverityOff
	}

	// ---- Collect inline ignore directives ----
	var ignores []config.IgnoreDirective
	for _, f := range result.Files {
		ignores = append(ignores, config.ParseIgnoreDirectives(f.Fset, f.AST)...)
	}

	// ---- Build rule-engine context and run rules ----
	ctx := &rules.Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
		Index: capabilities,
	}

	diagnostics := rules.Run(ctx, rules.DefaultRegistry, rules.RunConfig{
		EnabledIDs:        enabledIDs,
		SeverityOverrides: severityOverrides,
	})

	// ---- Inline ignore suppression ----
	diagnostics = config.SuppressByIgnore(diagnostics, ignores)

	// ---- Baseline suppression ----
	var staleBaseline []config.BaselineEntry
	if cliflags.baseline != "" {
		if _, statErr := os.Stat(cliflags.baseline); statErr == nil {
			baseline, berr := config.LoadBaseline(cliflags.baseline)
			if berr != nil {
				fmt.Fprintf(os.Stderr, "error: loading baseline: %v\n", berr)
				return 3
			}
			diagnostics, staleBaseline = config.SuppressByBaseline(diagnostics, baseline)
		}
	}

	// Filter by minimum severity.
	minSeverity, _ := diag.SeverityFromString(cliflags.severity)
	failOnSeverity, _ := diag.SeverityFromString(cliflags.failOn)

	filtered := filterDiagnostics(diagnostics, minSeverity)

	// Append stale baseline entries as info diagnostics.
	for _, se := range staleBaseline {
		filtered = append(filtered, &diag.Diagnostic{
			RuleID:   "lurpiclint-stale-baseline",
			Severity: diag.SeverityInfo,
			Pos:      token.Position{Filename: se.File, Line: se.Line},
			Message:  "stale baseline entry: " + se.RuleID + " no longer produces findings",
		})
	}

	// Sort for deterministic output.
	diag.SortDiagnostics(filtered)

	// Report.
	if err := reporter.Report(filtered); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 3
	}

	// Determine exit code: 1 if any diagnostic meets or exceeds the
	// fail-on threshold.  An empty set never fails.
	for _, d := range filtered {
		if d.Severity >= failOnSeverity {
			return 1
		}
	}
	return 0
}

// findModuleRoot walks up from the working directory to find the module root
// (directory containing go.mod).  Returns empty string when not found.
func findModuleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// filterDiagnostics returns a new slice containing only diagnostics whose
// severity is at least min.
func filterDiagnostics(d []*diag.Diagnostic, min diag.Severity) []*diag.Diagnostic {
	if min == diag.SeverityInfo {
		return d
	}
	out := make([]*diag.Diagnostic, 0, len(d))
	for _, di := range d {
		if di.Severity >= min {
			out = append(out, di)
		}
	}
	return out
}

func runCapabilities(args []string) int {
	fs := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	format := fs.String("format", "text", "output format (text, json)")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Validate format.
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(os.Stderr, "error: invalid --format value %q (valid: text, json)\n", *format)
		return 2
	}

	root := findModuleRoot()
	if root == "" {
		fmt.Fprintln(os.Stderr, "error: cannot find module root (no go.mod found)")
		return 3
	}

	// Load framework packages for introspection.
	patterns := []string{
		root + "/marks/...",
		root + "/layout/...",
		root + "/facet",
	}

	result, err := loader.Load(patterns, loader.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 3
	}

	caps := capindex.Scan(result, capindex.ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: root,
	})

	switch *format {
	case "json":
		emitter := capindex.NewJSONEmitter(os.Stdout)
		if err := emitter.Emit(caps); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 3
		}
	default:
		emitter := capindex.NewTextEmitter(os.Stdout)
		if err := emitter.Emit(caps); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 3
		}
	}

	return 0
}

func runExplain(args []string) int {
	if len(args) < 1 || args[0] == "" {
		fmt.Fprintln(os.Stderr, "usage: lurpiclint explain <rule-id>")
		return 2
	}

	ruleID := args[0]
	rule := rules.DefaultRegistry.Lookup(ruleID)
	if rule == nil {
		fmt.Fprintf(os.Stderr, "unknown rule: %s\n", ruleID)
		return 2
	}

	fmt.Printf("Rule %s (%s)\n", rule.ID(), rule.DefaultSeverity().String())
	fmt.Println()
	fmt.Printf("  %s\n", rule.Description())

	// Print extended explanation if the rule implements Explain().
	if expl, ok := rule.(rules.Explainer); ok {
		fmt.Println()
		fmt.Printf("  %s\n", expl.Explain())
	}

	return 0
}

func runVersion() int {
	fmt.Printf("lurpiclint version %s\n", version)
	return 0
}

func printUsage() {
	fmt.Print(`lurpiclint - static analyzer for lurpicUI applications

Usage:
  lurpiclint check [flags] [packages...]   run rules, the build gate
  lurpiclint capabilities [flags]          emit the uxauthoring index
  lurpiclint explain <rule-id>             print a rule's rationale and fix
  lurpiclint version                       print version information

Check flags:
  --format string         output format (text, json, github) (default "text")
  --severity string       minimum severity to report (info, warn, error) (default "warn")
  --fail-on string        minimum severity that forces non-zero exit (default "error")
  --config string         path to .lurpiclint.toml (auto-discovered from cwd)
  --baseline string       suppress findings recorded in a baseline file
  --rules string          comma-separated list of rules to enable (default all)
  --no-suggest            disable info-level shape-match suggestions
  --include-tests         include _test.go files in analysis
  --root string           module root for capability introspection

Capabilities flags:
  --format string         output format (text, json) (default "text")

Exit codes:
  0   no findings at or above --fail-on
  1   findings at or above --fail-on
  2   usage error (bad flags/paths)
  3   internal error (parse failure, panic recovered)
`)
}
