package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	format              string
	severity            string
	failOn              string
	failOnStaleBaseline bool
	config              string
	baseline            string
	rules               string
	noSuggest           bool
	includeTests        bool
	root                string
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
	case "baseline":
		return runBaseline(args[2:])
	case "verify-layout":
		return runVerifyLayout(args[2:])
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
	fs.BoolVar(&cliflags.failOnStaleBaseline, "fail-on-stale-baseline", false, "non-zero exit when baseline entries are stale (CI)")
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

	// Merge _test.go files from framework packages so contract rules
	// LL029–LL033 can detect contracttest helper invocations (FR-2).
	mergeFrameworkTestFiles(result, root)

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

	// Append stale baseline entries as diagnostics.  Severity is info
	// by default; when --fail-on-stale-baseline is set, stale entries
	// are emitted at the fail-on severity so CI can force hygiene.
	staleSeverity := diag.SeverityInfo
	if cliflags.failOnStaleBaseline {
		staleSeverity = failOnSeverity
	}
	for _, se := range staleBaseline {
		filtered = append(filtered, &diag.Diagnostic{
			RuleID:   "lurpiclint-stale-baseline",
			Severity: staleSeverity,
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

// runVerifyLayout handles the `verify-layout` subcommand.
// It generates a transient _verifylayout_test.go in the target package,
// runs `go test -json`, and reports findings via a results-file channel.
func runVerifyLayout(args []string) int {
	fs := flag.NewFlagSet("verify-layout", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	builderFlag := fs.String("builder", "BuildRoot", "in-package builder function name")
	sizeFlag := fs.String("size", "1280x800", "window size as WxH")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Parse WxH.
	var screenW, screenH int
	if _, err := fmt.Sscanf(*sizeFlag, "%dx%d", &screenW, &screenH); err != nil || screenW <= 0 || screenH <= 0 {
		fmt.Fprintf(os.Stderr, "error: invalid --size %q (expected WxH, e.g. 1280x800)\n", *sizeFlag)
		return 2
	}

	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"."}
	}

	// Resolve the target directory from the first pattern.  If the
	// pattern is a relative path like ".", resolve it to a real dir.
	targetDir := patterns[0]
	if !filepath.IsAbs(targetDir) && !strings.Contains(targetDir, "/") && !strings.HasSuffix(targetDir, "...") {
		// Single package name like "demos/quick_square_app".
		// Keep as-is; go test will resolve it.
	} else if strings.HasSuffix(targetDir, "/...") {
		// Glob pattern — use the prefix directory.
		targetDir = strings.TrimSuffix(targetDir, "/...")
	} else {
		// Try to resolve relative to cwd.
		abs, err := filepath.Abs(targetDir)
		if err == nil {
			targetDir = abs
		}
	}

	// If targetDir is not an absolute path, try to resolve it as a
	// package path using go list.
	pkgDir := targetDir
	if !filepath.IsAbs(pkgDir) {
		// Use go list to resolve the package directory.
		//nolint:gosec // G204: targetDir is CLI-controlled, not user-input-injection
		out, err := exec.Command("go", "list", "-f", "{{.Dir}}", targetDir).Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot resolve package %q: %v\n", targetDir, err)
			return 2
		}
		pkgDir = strings.TrimSpace(string(out))
	}

	// Read the package clause from the first non-test .go file.
	pkgName := readPackageName(pkgDir)
	if pkgName == "" {
		fmt.Fprintf(os.Stderr, "error: cannot determine package name in %s\n", pkgDir)
		return 2
	}

	// Strip any pkg. prefix from -builder (the generated test is in-package).
	builderName := *builderFlag
	if idx := strings.LastIndex(builderName, "."); idx >= 0 {
		builderName = builderName[idx+1:]
	}

	// Remove any pre-existing generated test (both old and current name).
	// NOTE: filename must NOT start with '_' — the go tool ignores such files.
	_ = os.Remove(filepath.Join(pkgDir, "_verifylayout_test.go")) // pre-rename cleanup
	genFilePath := filepath.Join(pkgDir, "lurpiclint_verifylayout_test.go")
	_ = os.Remove(genFilePath) // current name cleanup

	// Build the generated test template.
	genCode := fmt.Sprintf(`package %s

import (
	"encoding/json"
	"os"
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	vl "codeburg.org/lexbit/lurpicui/cmd/lurpiclint/verifylayout"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestLurpiclintVerifyLayout(t *testing.T) {
	root := %s(app.BuildContext{
		WindowSize:   gfx.Size{W: %d, H: %d},
		ContentScale: 1,
	})
	findings := vl.Check(root, vl.Options{Size: gfx.Size{W: %d, H: %d}})
	if out := os.Getenv("LURPIC_VERIFYLAYOUT_OUT"); out != "" {
		b, _ := json.Marshal(findings)
		_ = os.WriteFile(out, b, 0600)
	}
	if len(findings) > 0 {
		t.Fatalf("verify-layout: %%d finding(s)", len(findings))
	}
}
`, pkgName, builderName, screenW, screenH, screenW, screenH)

	// Write the generated test file; defer cleanup.
	if err := os.WriteFile(genFilePath, []byte(genCode), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "error: writing %s: %v\n", genFilePath, err)
		return 3
	}
	defer func() { _ = os.Remove(genFilePath) }()

	// Create a temp file for the results.
	resultsFile, err := os.CreateTemp("", "lurpiclint-verify-*.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: creating results file: %v\n", err)
		return 3
	}
	resultsPath := resultsFile.Name()
	resultsFile.Close()
	defer func() { _ = os.Remove(resultsPath) }()

	// Run go test -json.
	//nolint:gosec // G204: pkgDir is CLI-controlled, not user-input-injection
	cmd := exec.Command("go", "test", "-json", "-run", "^TestLurpiclintVerifyLayout$", "-count=1", pkgDir)
	cmd.Env = append(os.Environ(), "LURPIC_VERIFYLAYOUT_OUT="+resultsPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Check for compile errors in the JSON output.
		compileErr := extractCompileError(stdout.Bytes(), stderr.String())
		if compileErr != "" {
			fmt.Fprintf(os.Stderr, "verify-layout: target does not compile:\n%s\n", compileErr)
			return 3
		}
		// If the test ran but failed (findings exist), that's not a
		// go-command error — the exit code is 1 and we handle it below.
		// But if go test itself failed (no test binary), we surface it.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			// Test failure — proceed to read results.
		} else {
			fmt.Fprintf(os.Stderr, "error: go test failed: %v\n%s\n", err, stderr.String())
			return 3
		}
	}

	// Parse the JSON stream for test events.
	dec := json.NewDecoder(&stdout)
	testFailed := false
	for dec.More() {
		var evt struct {
			Action string `json:"action"`
			Test   string `json:"test,omitempty"`
			Output string `json:"output,omitempty"`
		}
		if err := dec.Decode(&evt); err != nil {
			break
		}
		if evt.Action == "fail" && evt.Test == "TestLurpiclintVerifyLayout" {
			testFailed = true
		}
	}

	// Read the results file.
	//nolint:gosec // G304: resultsPath is a temp file we just created
	data, err := os.ReadFile(resultsPath)
	if err != nil || len(data) == 0 {
		if testFailed {
			fmt.Fprintln(os.Stderr, "verify-layout: test failed but no results file found (builder may have panicked)")
			return 3
		}
		fmt.Fprintln(os.Stdout, "verify-layout: OK (no results file)")
		return 0
	}

	var findings []struct {
		Kind   string `json:"kind"`
		Type   string `json:"type"`
		Field  string `json:"field"`
		Source string `json:"source"`
		Detail string `json:"detail"`
		Hint   string `json:"hint"`
	}
	if err := json.Unmarshal(data, &findings); err != nil {
		fmt.Fprintf(os.Stderr, "error: reading findings: %v\n", err)
		return 3
	}

	if len(findings) == 0 {
		fmt.Fprintf(os.Stdout, "verify-layout OK: 0 finding(s) at %dx%d\n", screenW, screenH)
		return 0
	}

	fmt.Fprintf(os.Stdout, "verify-layout FAILED: %d finding(s) at %dx%d\n", len(findings), screenW, screenH)
	for _, f := range findings {
		src := f.Source
		if src != "" {
			src += "  "
		}
		hint := ""
		if f.Hint != "" {
			hint = "\n                fix: " + f.Hint
		}
		fmt.Fprintf(os.Stdout, "\n  %-18s %s%s%s%s\n", f.Kind, src, f.Type, fieldSuffix(f.Field), hint)
		fmt.Fprintf(os.Stdout, "                %s\n", f.Detail)
	}
	fmt.Fprintln(os.Stdout, "\n  hint: these are structural soundness checks, not correctness.")
	return 1
}

// readPackageName reads the package clause from the first non-_test.go file
// in the given directory.  Returns "" on failure.
func readPackageName(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
		if err != nil {
			continue
		}
		return f.Name.Name
	}
	return ""
}

// fieldSuffix returns " <field>" if field is non-empty, else "".
func fieldSuffix(field string) string {
	if field != "" {
		return " " + field
	}
	return ""
}

// extractCompileError attempts to find a compile error in go test -json output.
func extractCompileError(jsonOutput []byte, stderr string) string {
	// The JSON stream may contain events with Action:"fail" at the
	// package level that carry compiler output in the "output" field.
	dec := json.NewDecoder(bytes.NewReader(jsonOutput))
	for dec.More() {
		var evt struct {
			Action string `json:"action"`
			Output string `json:"output,omitempty"`
		}
		if err := dec.Decode(&evt); err != nil {
			break
		}
		if evt.Action == "fail" && strings.Contains(evt.Output, "compile") {
			return evt.Output
		}
	}
	// Fall back to stderr.
	if stderr != "" {
		return stderr
	}
	return ""
}

// mergeFrameworkTestFiles appends _test.go files from framework packages
// (marks/, layout/) to the loaded result so contract rules LL029–LL033 can
// detect contracttest helper invocations (FR-2).  Test files from
// non-framework packages are deliberately excluded: the rest of the analysis
// is unchanged, and the loader's IncludeTests flag stays opt-in for users who
// want it globally.
func mergeFrameworkTestFiles(result *loader.LoadResult, root string) {
	if root == "" {
		return
	}
	testResult, testErr := loader.Load([]string{
		root + "/marks/...",
		root + "/layout/...",
	}, loader.Config{IncludeTests: true})
	if testErr != nil {
		return
	}
	for _, pkg := range testResult.Packages {
		// The framework package's Path is its directory, which is the same key
		// under which the package under test lives in the loaded result
		// (whether the test files are internal (package <name>) or external
		// (<name>_test)).
		mainPkg := result.Packages[pkg.Path]
		if mainPkg == nil {
			continue
		}
		for _, pf := range pkg.Files {
			if !strings.HasSuffix(pf.Path, "_test.go") {
				continue
			}
			mainPkg.Files = append(mainPkg.Files, pf)
			result.Files = append(result.Files, pf)
		}
	}
	sort.Slice(result.Files, func(i, j int) bool {
		return result.Files[i].Path < result.Files[j].Path
	})
	for _, pkg := range result.Packages {
		sort.Slice(pkg.Files, func(i, j int) bool {
			return pkg.Files[i].Path < pkg.Files[j].Path
		})
	}
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

// runBaseline handles the `baseline generate` subcommand.
func runBaseline(args []string) int {
	if len(args) == 0 || args[0] != "generate" {
		fmt.Fprintf(os.Stderr, "usage: lurpiclint baseline generate [-o path] [patterns...]\n")
		return 2
	}
	args = args[1:] // consume "generate"

	fs := flag.NewFlagSet("baseline generate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	outputPath := fs.String("o", "lurpiclint-baseline.json", "output path for the baseline JSON")
	sevName := fs.String("severity", "warn", "minimum severity to include (info, warn, error)")
	root := fs.String("root", "", "module root for capability introspection")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	minSeverity, ok := diag.SeverityFromString(*sevName)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: invalid --severity value %q\n", *sevName)
		return 2
	}

	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"."}
	}

	// Load files.
	result, err := loader.Load(patterns, loader.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 3
	}

	// Build capindex for LL004.
	var capabilities []capindex.Capability
	modRoot := *root
	if modRoot == "" {
		modRoot = findModuleRoot()
	}
	if modRoot != "" {
		capResult, capErr := loader.Load([]string{
			modRoot + "/marks/...",
			modRoot + "/layout/...",
			modRoot + "/facet",
		}, loader.Config{})
		if capErr == nil {
			capabilities = capindex.Scan(capResult, capindex.ScanConfig{
				ModulePath: "codeburg.org/lexbit/lurpicui",
				ModuleRoot: modRoot,
			})
		}
	}

	// Merge framework _test.go files so contract rules LL029–LL033 see the
	// contracttest helper invocations — the baseline must reflect the same
	// reality the check gate sees.
	mergeFrameworkTestFiles(result, modRoot)

	// Run all rules.
	ctx := &rules.Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
		Index: capabilities,
	}

	diagnostics := rules.Run(ctx, rules.DefaultRegistry, rules.RunConfig{})

	// Suppress //lurpiclint:ignore directives, matching the check pipeline so
	// the baseline records exactly what the gate would report.
	var ignores []config.IgnoreDirective
	for _, f := range result.Files {
		ignores = append(ignores, config.ParseIgnoreDirectives(f.Fset, f.AST)...)
	}
	diagnostics = config.SuppressByIgnore(diagnostics, ignores)

	// Filter by severity.
	var filtered []*diag.Diagnostic
	for _, d := range diagnostics {
		if d.Severity >= minSeverity {
			filtered = append(filtered, d)
		}
	}

	// Convert to baseline entries.
	entries := make([]config.BaselineEntry, 0, len(filtered))
	for _, d := range filtered {
		entries = append(entries, config.BaselineEntry{
			RuleID:  d.RuleID,
			File:    d.Pos.Filename,
			Line:    d.Pos.Line,
			Message: d.Message,
		})
	}

	// Sort deterministically: file, line, rule_id, message.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].File != entries[j].File {
			return entries[i].File < entries[j].File
		}
		if entries[i].Line != entries[j].Line {
			return entries[i].Line < entries[j].Line
		}
		if entries[i].RuleID != entries[j].RuleID {
			return entries[i].RuleID < entries[j].RuleID
		}
		return entries[i].Message < entries[j].Message
	})

	baseline := config.Baseline{Entries: entries}
	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshaling baseline: %v\n", err)
		return 3
	}

	if err := os.WriteFile(*outputPath, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "error: writing %s: %v\n", *outputPath, err)
		return 3
	}

	fmt.Fprintf(os.Stderr, "wrote %d baseline entries to %s\n", len(entries), *outputPath)
	return 0
}

func printUsage() {
	fmt.Print(`lurpiclint - static analyzer for lurpicUI applications

Usage:
  lurpiclint check [flags] [packages...]         run rules, the build gate
  lurpiclint capabilities [flags]                emit the uxauthoring index
  lurpiclint explain <rule-id>                   print a rule's rationale and fix
  lurpiclint baseline generate [flags] [pkgs]    generate baseline JSON from current findings
  lurpiclint verify-layout [flags]               run layout-tree assertion (library mode preferred)
  lurpiclint version                             print version information

Check flags:
  --format string             output format (text, json, github) (default "text")
  --severity string           minimum severity to report (info, warn, error) (default "warn")
  --fail-on string            minimum severity that forces non-zero exit (default "error")
  --fail-on-stale-baseline    non-zero exit when baseline entries are stale (CI)
  --config string             path to .lurpiclint.toml (auto-discovered from cwd)
  --baseline string           suppress findings recorded in a baseline file
  --rules string              comma-separated list of rules to enable (default all)
  --no-suggest                disable info-level shape-match suggestions
  --include-tests             include _test.go files in analysis
  --root string               module root for capability introspection

Baseline flags:
  -o string                   output path (default "lurpiclint-baseline.json")
  --severity string           minimum severity to include (default "warn")

Capabilities flags:
  --format string             output format (text, json) (default "text")

Exit codes:
  0   no findings at or above --fail-on
  1   findings at or above --fail-on
  2   usage error (bad flags/paths)
  3   internal error (parse failure, panic recovered)
`)
}
