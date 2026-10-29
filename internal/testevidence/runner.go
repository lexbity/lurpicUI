package testevidence

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const unknownDigest = "unknown"

// Runner executes a manifest. Root is the repository root; GoCmd defaults to
// "go". EvidenceRoot defaults to <root>/build/runtime-evidence.
type Runner struct {
	Root         string
	GoCmd        string
	EvidenceRoot string
	Env          []string
	Stdout       io.Writer
	Stderr       io.Writer
	Now          func() time.Time
}

// Options select the manifest slice to execute.
type Options struct {
	// Phase limits execution to one phase. Empty means all phases (--all).
	Phase string
	// Mode limits execution to one gate mode ("host" or "race"). Empty runs
	// both.
	Mode string
	// Count overrides each gate's repetition count when > 0.
	Count int
	// Backend is the declared backend of this invocation. A gate that names a
	// different backend is a mismatch, never silently relabelled.
	Backend string
}

// GateResult is the immutable per-gate record.
type GateResult struct {
	ID             string      `json:"id"`
	Phase          string      `json:"phase"`
	Package        string      `json:"package"`
	Tests          []string    `json:"tests"`
	Mode           string      `json:"mode"`
	Backend        string      `json:"backend,omitempty"`
	Repetitions    int         `json:"repetitions"`
	Runs           []RunRecord `json:"runs"`
	Required       bool        `json:"required"`
	Passed         bool        `json:"passed"`
	Failures       []string    `json:"failures,omitempty"`
	RunDurationMS  int64       `json:"run_duration_ms"`
	RequiredCaseID []string    `json:"required_case_ids,omitempty"`
	Artifacts      []string    `json:"artifacts,omitempty"`
	EvidenceDir    string      `json:"evidence_dir"`
}

// RunRecord is one controlled repetition of a gate.
type RunRecord struct {
	Repetition    int      `json:"repetition"`
	Passed        bool     `json:"passed"`
	DurationMS    int64    `json:"duration_ms"`
	ExitCode      int      `json:"exit_code"`
	TimedOut      bool     `json:"timed_out"`
	BudgetFailed  string   `json:"budget_failed,omitempty"`
	Completed     []string `json:"completed"`
	Skipped       []string `json:"skipped,omitempty"`
	OutputTail    string   `json:"output_tail,omitempty"`
	StackCaptured bool     `json:"stack_captured,omitempty"`
}

// Report is the committed, machine-readable evidence summary.
type Report struct {
	Revision       string       `json:"revision"`
	WorktreeDigest string       `json:"worktree_digest"`
	Phase          string       `json:"phase"`
	Mode           string       `json:"mode"`
	Backend        string       `json:"backend,omitempty"`
	Started        string       `json:"started"`
	Finished       string       `json:"finished"`
	Gates          []GateResult `json:"gates"`
	Passed         bool         `json:"passed"`
	EvidenceDir    string       `json:"evidence_dir"`
}

type testEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
	Output  string  `json:"Output"`
}

type runState struct {
	mu               sync.Mutex
	running          map[string]time.Time
	completed        map[string]bool
	skipped          map[string]bool
	seenRun          map[string]bool
	lastLine         time.Time
	packagesSeen     map[string]bool
	packagesTerminal map[string]bool
	packageMismatch  []string
	gatePackage      string
}

// Run executes the selected manifest slice and writes evidence. It returns a
// non-nil error whenever any required evidence is missing or failed; a nil
// report never accompanies a passing run.
func (r *Runner) Run(ctx context.Context, manifestPath string, opts Options) (*Report, error) {
	if r == nil {
		return nil, errors.New("testevidence: nil runner")
	}
	if r.Root == "" {
		r.Root = "."
	}
	if r.GoCmd == "" {
		r.GoCmd = "go"
	}
	if r.Now == nil {
		r.Now = time.Now
	}
	if r.Stdout == nil {
		r.Stdout = os.Stdout
	}
	if r.Stderr == nil {
		r.Stderr = os.Stderr
	}
	m, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	if opts.Phase != "" && !contains(m.Phases(), opts.Phase) {
		return nil, fmt.Errorf("testevidence: phase %q is not in the manifest (phases: %s)", opts.Phase, strings.Join(m.Phases(), ", "))
	}
	gates := m.Select(opts.Phase, opts.Mode)
	if len(gates) == 0 {
		return nil, fmt.Errorf("testevidence: selection phase=%q mode=%q is empty — an empty selection cannot pass", opts.Phase, opts.Mode)
	}
	for _, g := range gates {
		if opts.Backend != "" && g.Backend != "" && g.Backend != opts.Backend {
			return nil, fmt.Errorf("testevidence: gate %s declares backend %q, invocation is %q", g.ID, g.Backend, opts.Backend)
		}
	}

	revisionCommand := exec.Command("git", "rev-parse", "HEAD")
	revisionCommand.Dir = r.Root
	revisionBytes, revisionErr := revisionCommand.Output()
	revision := strings.TrimSpace(string(revisionBytes))
	if revisionErr != nil || revision == "" {
		revision = unknownDigest
	}
	digest := r.worktreeDigest()
	started := r.Now().UTC()
	if r.EvidenceRoot == "" {
		r.EvidenceRoot = filepath.Join(r.Root, "build", "runtime-evidence")
	}
	modeLabel := opts.Mode
	if modeLabel == "" {
		modeLabel = "all"
	}
	phaseLabel := opts.Phase
	if phaseLabel == "" {
		phaseLabel = "all"
	}
	runDir := filepath.Join(r.EvidenceRoot, sanitize(revision), phaseLabel, modeLabel, started.Format("20060102T150405.000000000"))
	report := &Report{
		Revision:       revision,
		WorktreeDigest: digest,
		Phase:          phaseLabel,
		Mode:           modeLabel,
		Backend:        opts.Backend,
		Started:        started.Format(time.RFC3339Nano),
		EvidenceDir:    runDir,
	}
	if err := os.MkdirAll(runDir, 0o750); err != nil {
		return nil, fmt.Errorf("testevidence: evidence dir: %w", err)
	}
	if opts.Count < 0 {
		return nil, errors.New("testevidence: count must not be negative")
	}

	passed := true
	for i := range gates {
		g := gates[i]
		if opts.Count > 0 {
			g.Repetitions = opts.Count
		}
		res := r.runGate(ctx, g, runDir)
		report.Gates = append(report.Gates, res)
		if res.Required && !res.Passed {
			passed = false
		}
	}
	report.Passed = passed
	report.Finished = r.Now().UTC().Format(time.RFC3339Nano)
	if err := report.write(runDir); err != nil {
		return report, err
	}
	if !passed {
		return report, fmt.Errorf("testevidence: %d gate(s) failed", failedGates(report.Gates))
	}
	return report, nil
}

func failedGates(gates []GateResult) int {
	n := 0
	for _, g := range gates {
		if !g.Passed {
			n++
		}
	}
	return n
}

func (r *Runner) runGate(ctx context.Context, g Gate, rootDir string) GateResult {
	start := r.Now()
	res := GateResult{
		ID:             g.ID,
		Phase:          g.Phase,
		Package:        g.Package,
		Tests:          append([]string(nil), g.Tests...),
		Mode:           g.Mode,
		Backend:        g.Backend,
		Repetitions:    g.Repetitions,
		Required:       g.Required,
		RequiredCaseID: append([]string(nil), g.RequiredCaseIDs...),
		Artifacts:      append([]string(nil), g.Artifacts...),
		EvidenceDir:    filepath.Join(rootDir, sanitize(g.ID)),
	}
	if err := os.MkdirAll(res.EvidenceDir, 0o750); err != nil {
		res.Failures = append(res.Failures, "evidence dir: "+err.Error())
		return res
	}
	listed, listErr := r.listTests(ctx, g.Package, g.Mode, g.Tags)
	if listErr != nil {
		res.Failures = append(res.Failures, "list: "+listErr.Error())
		return res
	}
	// The provenance check compares the stream's Package field (the import
	// path) against the gate's declared package; resolve the manifest's
	// relative reference to its import path. A resolution failure falls back
	// to the manifest spelling — the gate then fails provenance honestly if
	// the stream disagrees.
	importPath, pathErr := r.resolvePackagePath(ctx, g.Package)
	if pathErr != nil {
		importPath = g.Package
	}
	for _, name := range g.Tests {
		top := name
		if idx := strings.IndexByte(top, '/'); idx >= 0 {
			top = top[:idx]
		}
		if !listed[top] {
			res.Failures = append(res.Failures, fmt.Sprintf("required test %q does not exist in %s (a -run matching zero tests is a failed gate)", top, g.Package))
		}
	}
	if len(res.Failures) > 0 {
		return res
	}

	allPassed := true
	for rep := 1; rep <= g.Repetitions; rep++ {
		record := r.runOnce(ctx, g, res.EvidenceDir, rep, importPath)
		res.Runs = append(res.Runs, record)
		if !record.Passed {
			allPassed = false
		} else {
			// Check artifacts per repetition (D10.1): fail fast if artifacts are missing
			for _, artifact := range g.Artifacts {
				path := filepath.Join(res.EvidenceDir, artifact)
				if st, err := os.Stat(path); err != nil || st.IsDir() {
					res.Failures = append(res.Failures, "missing artifact "+artifact+" (repetition "+fmt.Sprint(rep)+")")
					allPassed = false
				}
			}
			for _, caseID := range g.RequiredCaseIDs {
				path := filepath.Join(res.EvidenceDir, sanitize(caseID)+".json")
				if st, err := os.Stat(path); err != nil || st.IsDir() {
					res.Failures = append(res.Failures, "missing case evidence "+caseID+" (repetition "+fmt.Sprint(rep)+")")
					allPassed = false
				}
			}
		}
	}
	res.Passed = allPassed && len(res.Failures) == 0
	res.RunDurationMS = r.Now().Sub(start).Milliseconds()
	if !res.Passed {
		fmt.Fprintf(r.Stderr, "testevidence: FAIL %s (%s %s) mode=%s\n", g.ID, g.Package, strings.Join(g.Tests, ","), g.Mode)
		for _, f := range res.Failures {
			fmt.Fprintf(r.Stderr, "  %s\n", f)
		}
		if len(res.Runs) > 0 {
			last := res.Runs[len(res.Runs)-1]
			if last.OutputTail != "" {
				fmt.Fprintf(r.Stderr, "%s\n", last.OutputTail)
			}
		}
		return res
	}
	fmt.Fprintf(r.Stdout, "PASS %s %s %s mode=%s reps=%d in %dms\n",
		g.ID, g.Package, strings.Join(g.Tests, ","), g.Mode, g.Repetitions, res.RunDurationMS)
	return res
}

// resolvePackagePath resolves a manifest package reference ("./drive") to
// its import path — the identity the go test JSON stream reports in the
// Package field, which the provenance check compares against.
func (r *Runner) resolvePackagePath(ctx context.Context, pkg string) (string, error) {
	cmd := exec.CommandContext(ctx, r.GoCmd, "list", "-f", "{{.ImportPath}}", pkg) //nolint:gosec // committed manifest package
	cmd.Dir = r.Root
	cmd.Env = r.env()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %w: %s", pkg, err, strings.TrimSpace(stderr.String()))
	}
	out := strings.TrimSpace(stdout.String())
	if out == "" {
		return "", fmt.Errorf("%s: empty import path", pkg)
	}
	return out, nil
}

func (r *Runner) listTests(ctx context.Context, pkg, mode string, tags []string) (map[string]bool, error) {
	args := []string{"test", "-list", `.*`}
	args = append(args, modeFlags(mode, tags)...)
	args = append(args, pkg)
	cmd := exec.CommandContext(ctx, r.GoCmd, args...) //nolint:gosec // committed manifest package
	cmd.Dir = r.Root
	cmd.Env = r.env()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %w: %s", pkg, err, strings.TrimSpace(stderr.String()))
	}
	names := make(map[string]bool)
	sc := bufio.NewScanner(&stdout)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "? ") ||
			strings.HasPrefix(line, "---") || strings.HasPrefix(line, "no test files") {
			continue
		}
		names[line] = true
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%s: test listing is empty", pkg)
	}
	return names, nil
}

// runOnce executes one controlled repetition. go test -json is the only
// execution path; the runner inspects every action and never treats a process
// exit status alone as success.
func (r *Runner) runOnce(ctx context.Context, g Gate, dir string, rep int, importPath string) RunRecord {
	rec := RunRecord{Repetition: rep}
	start := r.Now()
	timeout := time.Duration(g.PackageTimeoutMS) * time.Millisecond
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pattern := rootPattern(g.Tests)
	args := []string{"test", "-json", "-count=1", "-run", pattern, "-timeout", timeout.String()}
	args = append(args, modeFlags(g.Mode, g.Tags)...)
	args = append(args, g.Package)
	cmd := exec.CommandContext(runCtx, r.GoCmd, args...) //nolint:gosec // committed manifest package
	cmd.Dir = r.Root
	cmd.Env = append(r.env(),
		"TESTEVIDENCE_ARTIFACT_DIR="+dir,
		"TESTEVIDENCE_GATE_ID="+g.ID,
		"TESTEVIDENCE_REPETITION="+fmt.Sprint(rep),
	)
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 2 * time.Second
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		rec.OutputTail = "stdout pipe: " + err.Error()
		return rec
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		rec.OutputTail = "start: " + err.Error()
		return rec
	}

	state := &runState{
		running:          make(map[string]time.Time),
		completed:        make(map[string]bool),
		skipped:          make(map[string]bool),
		seenRun:          make(map[string]bool),
		lastLine:         r.Now(),
		packagesSeen:     make(map[string]bool),
		packagesTerminal: make(map[string]bool),
		packageMismatch:  []string{},
		gatePackage:      importPath,
	}
	var output bytes.Buffer
	budgetHit := make(chan string, 1)
	var budgetOnce sync.Once
	watchdog := startTestWatchdog(runCtx, state, time.Duration(g.TestBudgetMS)*time.Millisecond, r.Now, func(name string) {
		budgetOnce.Do(func() {
			budgetHit <- name
			cancel()
		})
	}, cmd)

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		var ev testEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			// A truncated/non-JSON stream is an execution fault even if the
			// process later exits zero.
			rec.OutputTail = boundedTail("malformed go test -json line: "+string(line), 2048)
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			break
		}
		state.mu.Lock()
		state.lastLine = r.Now()
		// Handle package-level events (Test == "")
		if ev.Test == "" {
			// Track that we've seen this package
			if ev.Package != "" && ev.Action == "start" {
				state.packagesSeen[ev.Package] = true
			}
			// Mark package as terminal on pass/fail/skip
			if ev.Package != "" && (ev.Action == "pass" || ev.Action == "fail" || ev.Action == "skip") {
				state.packagesTerminal[ev.Package] = true
			}
			// Check package provenance: ev.Package must match gate.Package
			if ev.Package != "" && ev.Package != state.gatePackage {
				state.packageMismatch = append(state.packageMismatch, ev.Package)
			}
		}
		switch ev.Action {
		case "run":
			if ev.Test != "" {
				state.running[ev.Test] = r.Now()
				state.seenRun[ev.Test] = true
			}
		case "pass":
			if ev.Test != "" {
				delete(state.running, ev.Test)
				state.completed[ev.Test] = true
			}
		case "fail":
			if ev.Test != "" {
				delete(state.running, ev.Test)
				state.completed[ev.Test] = true
			}
		case "skip":
			if ev.Test != "" {
				delete(state.running, ev.Test)
				state.skipped[ev.Test] = true
			}
		}
		state.mu.Unlock()
		if ev.Output != "" && output.Len() < 1<<20 {
			output.WriteString(ev.Output)
		}
	}
	scanErr := sc.Err()
	waitErr := cmd.Wait()
	watchdog.Stop()
	rec.DurationMS = r.Now().Sub(start).Milliseconds()
	// Record whether stack was captured by the watchdog
	rec.StackCaptured = watchdog.StackCaptured()

	// Check if budget was hit - drain the channel if present
	if len(budgetHit) > 0 {
		rec.TimedOut = true
		rec.BudgetFailed = <-budgetHit
	}
	if runCtx.Err() == context.DeadlineExceeded && !rec.TimedOut {
		rec.TimedOut = true
		rec.BudgetFailed = "package timeout " + timeout.String()
	}
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			rec.ExitCode = exitErr.ExitCode()
		} else {
			rec.ExitCode = -1
		}
	}
	if scanErr != nil && rec.OutputTail == "" {
		rec.OutputTail = "stream: " + scanErr.Error()
	}

	state.mu.Lock()
	for name := range state.completed {
		rec.Completed = append(rec.Completed, name)
	}
	for name := range state.skipped {
		rec.Skipped = append(rec.Skipped, name)
	}
	for name := range state.running {
		rec.Completed = append(rec.Completed, "INCOMPLETE:"+name)
	}
	state.mu.Unlock()
	sort.Strings(rec.Completed)
	sort.Strings(rec.Skipped)
	if stderr.Len() > 0 {
		output.WriteString(stderr.String())
	}
	if rec.OutputTail == "" {
		rec.OutputTail = boundedTail(output.String(), 4096)
	}
	rec.Passed = r.verify(g, rec, state)
	if !rec.Passed && rec.OutputTail == "" {
		rec.OutputTail = "gate failed with no captured output"
	}
	return rec
}

// verify applies the strict completion rule: every required root and every
// required subtest must have a pass action; any selected skip fails; a parent
// pass cannot mask a skipped child.
func (r *Runner) verify(g Gate, rec RunRecord, state *runState) bool {
	if rec.ExitCode != 0 || rec.TimedOut {
		return false
	}
	completed := make(map[string]bool, len(rec.Completed))
	for _, name := range rec.Completed {
		completed[name] = true
	}
	skipped := make(map[string]bool, len(rec.Skipped))
	for _, name := range rec.Skipped {
		skipped[name] = true
	}
	for _, required := range g.Tests {
		if !completed[required] {
			return false
		}
		if skipped[required] {
			return false
		}
		prefix := required + "/"
		for name := range skipped {
			if name == required || strings.HasPrefix(name, prefix) {
				return false
			}
		}
	}
	// Any selected test that never emitted a terminal action is missing
	// evidence, even if the package exited zero.
	state.mu.Lock()
	defer state.mu.Unlock()
	// Package-terminal event verification (D10.1):
	// If the process exited successfully (ExitCode==0 and !TimedOut), we require:
	// 1. Every package that appeared in the stream must have a terminal action
	// 2. At least one package must have appeared
	// 3. Every package must match the gate's declared package (provenance)
	// If the process was killed (timeout, non-zero exit), we cannot require
	// package-terminal events as the process may have been terminated mid-stream.
	if rec.ExitCode == 0 && !rec.TimedOut {
		if len(state.packagesSeen) == 0 {
			return false
		}
		for pkg := range state.packagesSeen {
			if !state.packagesTerminal[pkg] {
				return false
			}
		}
		if len(state.packageMismatch) > 0 {
			return false
		}
	}
	for name := range state.seenRun {
		if !state.completed[name] && !state.skipped[name] {
			return false
		}
	}
	return true
}

// rootPattern selects the top-level roots of every required test. Subtest
// paths are verified from the JSON action stream instead of being embedded in
// -run: Go splits -run on '/', so a joined subtest pattern would change the
// regexp segments and silently match nothing (a gap the engine must never
// accept).
func rootPattern(tests []string) string {
	seen := make(map[string]bool, len(tests))
	roots := make([]string, 0, len(tests))
	for _, name := range tests {
		root := name
		if idx := strings.IndexByte(root, '/'); idx >= 0 {
			root = root[:idx]
		}
		if !seen[root] {
			seen[root] = true
			roots = append(roots, regexp.QuoteMeta(root))
		}
	}
	sort.Strings(roots)
	return "^(" + strings.Join(roots, "|") + ")$"
}

func modeFlags(mode string, tags []string) []string {
	var flags []string
	if mode == ModeRace {
		flags = append(flags, "-race")
	}
	if len(tags) > 0 {
		sorted := append([]string(nil), tags...)
		sort.Strings(sorted)
		flags = append(flags, "-tags", strings.Join(sorted, ","))
	}
	return flags
}

func (r *Runner) env() []string {
	env := os.Environ()
	env = append(env, r.Env...)
	return env
}

func (r *Runner) worktreeDigest() string {
	tracked := exec.Command("git", "diff", "--binary", "HEAD", "--")
	tracked.Dir = r.Root
	diff, err := tracked.Output()
	if err != nil {
		return unknownDigest
	}
	others := exec.Command("git", "ls-files", "--others", "--exclude-standard", "-z")
	others.Dir = r.Root
	names, err := others.Output()
	if err != nil {
		return unknownDigest
	}
	h := sha256.New()
	_, _ = h.Write(diff)
	for _, name := range bytes.Split(names, []byte{0}) {
		if len(name) == 0 {
			continue
		}
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(name)))
		_, _ = h.Write(length[:])
		_, _ = h.Write(name)
		path := filepath.Join(r.Root, string(name))
		info, statErr := os.Lstat(path)
		if statErr != nil {
			return unknownDigest
		}
		_, _ = h.Write([]byte(info.Mode().String()))
		if info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(path)
			if readErr != nil {
				return unknownDigest
			}
			binary.BigEndian.PutUint64(length[:], uint64(len(target)))
			_, _ = h.Write(length[:])
			_, _ = h.Write([]byte(target))
			continue
		}
		if !info.Mode().IsRegular() {
			return unknownDigest
		}
		if info.Size() < 0 {
			return unknownDigest
		}
		binary.BigEndian.PutUint64(length[:], uint64(info.Size())) //nolint:gosec // negative sizes were rejected above
		_, _ = h.Write(length[:])
		file, openErr := os.Open(path) //nolint:gosec // path came from git's untracked-file list and is joined to the repository root
		if openErr != nil {
			return unknownDigest
		}
		_, copyErr := io.Copy(h, file)
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil {
			return unknownDigest
		}
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:])
}

func boundedTail(s string, limit int) string {
	s = strings.TrimSpace(s)
	if len(s) <= limit {
		return s
	}
	return s[len(s)-limit:]
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func sanitize(s string) string {
	if s == "" {
		return "unnamed"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if len(out) > 128 {
		out = out[:128]
	}
	return out
}
