package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/rules"
)

// run is the dispatch entry point that returns an exit code.
// It is invoked by main() which maps the exit code to os.Exit.

func TestRun_ExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		// dispatch
		{"no args", []string{"lurpiclint"}, 2},
		{"unknown subcommand", []string{"lurpiclint", "unknown"}, 2},

		// version
		{"version", []string{"lurpiclint", "version"}, 0},

		// help
		{"help", []string{"lurpiclint", "help"}, 0},
		{"help -h", []string{"lurpiclint", "-h"}, 0},
		{"help --help", []string{"lurpiclint", "--help"}, 0},

		// check – happy paths
		{"check no args", []string{"lurpiclint", "check"}, 0},
		{"check with paths", []string{"lurpiclint", "check", "./..."}, 0},
		{"check with flags", []string{"lurpiclint", "check", "--format", "json", "--severity", "error"}, 0},
		{"check with --no-suggest", []string{"lurpiclint", "check", "--no-suggest"}, 0},
		{"check with --include-tests", []string{"lurpiclint", "check", "--include-tests"}, 0},
		{"check with --root", []string{"lurpiclint", "check", "--root", "."}, 0},
		{"check with --config", []string{"lurpiclint", "check", "--config", ".lurpiclint.toml"}, 0},
		{"check with --baseline", []string{"lurpiclint", "check", "--baseline", "baseline.json"}, 0},
		{"check with --rules", []string{"lurpiclint", "check", "--rules", "LL001,LL003"}, 0},
		{"check with all flags", []string{"lurpiclint", "check", "--format", "github", "--severity", "warn", "--fail-on", "error", "--no-suggest", "--root", "."}, 0},

		// check – bad flags
		{"check bad flag", []string{"lurpiclint", "check", "--bad-flag"}, 2},
		{"check bad format value", []string{"lurpiclint", "check", "--format", "bad"}, 2},
		{"check missing flag value", []string{"lurpiclint", "check", "--format"}, 2},

		// capabilities – happy
		{"capabilities", []string{"lurpiclint", "capabilities"}, 0},
		{"capabilities --format json", []string{"lurpiclint", "capabilities", "--format", "json"}, 0},

		// capabilities – bad flags
		{"capabilities bad flag", []string{"lurpiclint", "capabilities", "--bad"}, 2},
		{"capabilities missing value", []string{"lurpiclint", "capabilities", "--format"}, 2},

		// explain
		{"explain no rule", []string{"lurpiclint", "explain"}, 2},
		{"explain empty rule", []string{"lurpiclint", "explain", ""}, 2},
		{"explain valid rule", []string{"lurpiclint", "explain", "LL001"}, 0},
		{"explain unknown rule", []string{"lurpiclint", "explain", "LL999"}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := run(tt.args)
			if got != tt.want {
				t.Errorf("run(%v) = %d, want %d", tt.args[1:], got, tt.want)
			}
		})
	}
}

// testData returns the absolute path to testdata within this package.
var testData = func() string {
	// During test execution the working directory is the package directory.
	// filepath.Abs returns the canonical path regardless.
	p, _ := filepath.Abs("testdata")
	return p
}()

func TestRunCheck_OnGoodPackage(t *testing.T) {
	path := filepath.Join(testData, "check/good")
	got := runCheck([]string{path})
	if got != 0 {
		t.Errorf("runCheck(%s) = %d, want 0", path, got)
	}
}

func TestRunCheck_OnBadPackage_Returns3(t *testing.T) {
	path := filepath.Join(testData, "check/bad")
	got := runCheck([]string{path})
	if got != 3 {
		t.Errorf("runCheck(%s) = %d, want 3 (parse error)", path, got)
	}
}

func TestRunCheck_FlagParsing(t *testing.T) {
	// Verify that various flag value combinations parse without error.
	// These tests use a valid fixture path so the loader succeeds.
	goodPath := filepath.Join(testData, "check/good")

	tests := []struct {
		name string
		args []string
		want int
	}{
		{"empty", []string{goodPath}, 0},
		{"format text", []string{"--format", "text", goodPath}, 0},
		{"format json", []string{"--format", "json", goodPath}, 0},
		{"format github", []string{"--format", "github", goodPath}, 0},
		{"severity info", []string{"--severity", "info", goodPath}, 0},
		{"severity warn", []string{"--severity", "warn", goodPath}, 0},
		{"severity error", []string{"--severity", "error", goodPath}, 0},
		{"fail-on info", []string{"--fail-on", "info", goodPath}, 0},
		{"fail-on warn", []string{"--fail-on", "warn", goodPath}, 0},
		{"fail-on error", []string{"--fail-on", "error", goodPath}, 0},
		{"no-suggest", []string{"--no-suggest", goodPath}, 0},
		{"include-tests", []string{"--include-tests", goodPath}, 0},
		{"config path", []string{"--config", "/some/path/.lurpiclint.toml", goodPath}, 0},
		{"baseline path", []string{"--baseline", "/some/path/baseline.json", goodPath}, 0},
		{"rules single", []string{"--rules", "LL003", goodPath}, 0},
		{"rules multiple", []string{"--rules", "LL001,LL003,LL010", goodPath}, 0},
		{"root relative", []string{"--root", testData, goodPath}, 0},
		{"paths after flags", []string{"--format", "json", "--severity", "error", goodPath}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runCheck(tt.args)
			if got != tt.want {
				t.Errorf("runCheck(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestRunCheck_BadFlagValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"unknown flag", []string{"--bogus"}, 2},
		{"truncated format", []string{"--format"}, 2},
		{"truncated severity", []string{"--severity"}, 2},
		{"truncated fail-on", []string{"--fail-on"}, 2},
		{"truncated config", []string{"--config"}, 2},
		{"truncated baseline", []string{"--baseline"}, 2},
		{"truncated rules", []string{"--rules"}, 2},
		{"truncated root", []string{"--root"}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runCheck(tt.args)
			if got != tt.want {
				t.Errorf("runCheck(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestRunCheck_NonexistentPath_Returns3(t *testing.T) {
	got := runCheck([]string{"/nonexistent/path/xyz"})
	if got != 3 {
		t.Errorf("runCheck with nonexistent path = %d, want 3", got)
	}
}

func TestRunCheck_IncludeTestsFlag(t *testing.T) {
	// withtests dir has both code.go and code_test.go
	path := filepath.Join(testData, "../internal/loader/testdata/withtests")

	// Without --include-tests: only code.go loaded → exit 0
	got := runCheck([]string{path})
	if got != 0 {
		t.Errorf("without --include-tests: runCheck(%s) = %d, want 0", path, got)
	}

	// With --include-tests: both files loaded → exit 0
	got = runCheck([]string{"--include-tests", path})
	if got != 0 {
		t.Errorf("with --include-tests: runCheck(%s) = %d, want 0", path, got)
	}
}

func TestRunCheck_InvalidFlagValues_Returns2(t *testing.T) {
	goodPath := filepath.Join(testData, "check/good")

	tests := []struct {
		name string
		args []string
	}{
		{"bad severity", []string{"--severity", "bogus", goodPath}},
		{"bad fail-on", []string{"--fail-on", "bogus", goodPath}},
		{"bad format", []string{"--format", "bogus", goodPath}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runCheck(tt.args)
			if got != 2 {
				t.Errorf("runCheck(%v) = %d, want 2", tt.args, got)
			}
		})
	}
}

func TestRunCheck_JSONOutputIsValid(t *testing.T) {
	goodPath := filepath.Join(testData, "check/good")

	// Capture stdout by running check with JSON format.
	var buf bytes.Buffer
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	format := fs.String("format", "text", "")
	_ = fs.Parse([]string{"--format", "json", goodPath})
	_ = format

	// Verify the reporter produces valid JSON through the check pipeline.
	got := runCheck([]string{"--format", "json", goodPath})
	if got != 0 {
		t.Fatalf("runCheck with --format json = %d, want 0", got)
	}

	// We can't easily capture stdout from runCheck, so instead verify the
	// diag.NewReporter + Report path is wired correctly by checking that
	// the JSON reporter emits a valid envelope for zero diagnostics.
	r, err := diag.NewReporter("json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Report(nil); err != nil {
		t.Fatal(err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("json reporter output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if sv, ok := raw["schema_version"].(float64); !ok || sv != 1 {
		t.Errorf("schema_version = %v, want 1", raw["schema_version"])
	}
}

func TestRunCheck_GitHubFormatOutput(t *testing.T) {
	goodPath := filepath.Join(testData, "check/good")
	got := runCheck([]string{"--format", "github", goodPath})
	if got != 0 {
		t.Errorf("runCheck with --format github = %d, want 0", got)
	}
}

func TestRunExplain_ExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"no args", []string{}, 2},
		{"empty string", []string{""}, 2},
		{"one rule", []string{"LL001"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runExplain(tt.args)
			if got != tt.want {
				t.Errorf("runExplain(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestRunCapabilities_ExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"no args", []string{}, 0},
		{"format text", []string{"--format", "text"}, 0},
		{"format json", []string{"--format", "json"}, 0},
		{"bad flag", []string{"--bogus"}, 2},
		{"truncated format", []string{"--format"}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runCapabilities(tt.args)
			if got != tt.want {
				t.Errorf("runCapabilities(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestExplain_EveryRegisteredRule(t *testing.T) {
	// Verify that explain produces non-empty output for every registered rule.
	for _, rule := range rules.DefaultRegistry.Rules() {
		t.Run(rule.ID(), func(t *testing.T) {
			got := runExplain([]string{rule.ID()})
			if got != 0 {
				t.Errorf("runExplain(%s) = %d, want 0", rule.ID(), got)
			}
		})
	}
}

func TestExplain_UnknownRule(t *testing.T) {
	got := runExplain([]string{"LL999"})
	if got != 2 {
		t.Errorf("runExplain(LL999) = %d, want 2", got)
	}
}

func TestRunVersion_Output(t *testing.T) {
	got := runVersion()
	if got != 0 {
		t.Errorf("runVersion() = %d, want 0", got)
	}
}

// --- Gate tests ---

func BenchmarkFullRepoCheck(b *testing.B) {
	root := findModuleRoot()
	if root == "" {
		b.Fatal("module root not found")
	}
	for i := 0; i < b.N; i++ {
		// Disable LL004 (needs capindex scan, dominates time).
		got := runCheck([]string{"--no-suggest", root})
		if got == 3 {
			b.Fatal("check returned exit code 3 (internal error)")
		}
	}
}

func TestGate_BadFixtureFails(t *testing.T) {
	// A deliberately-bad fixture must produce a non-zero exit.
	path := filepath.Join(testData, "../internal/rules/testdata/reinvent/ll003_bad")
	got := runCheck([]string{path})
	if got == 0 {
		t.Error("expected non-zero exit for bad LL003 fixture, got 0")
	}
}

func TestGate_CleanFixturePasses(t *testing.T) {
	// A clean fixture must exit 0.
	path := filepath.Join(testData, "../internal/rules/testdata/reinvent/ll003_good")
	got := runCheck([]string{path})
	// ll003_good has LL001 (warn) which is below default --fail-on of error,
	// so it should pass with exit 0.
	if got != 0 {
		t.Errorf("expected exit 0 for clean LL003 fixture, got %d", got)
	}
}

func TestGate_AgentDirectivesExist(t *testing.T) {
	root := findModuleRoot()
	files := []string{
		root + "/.agents/lurpiclint.md",
		root + "/.codex/lurpiclint.md",
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("agent directive file missing: %s", f)
			continue
		}
		content := string(data)
		// Must reference the lurpiclint command.
		if !strings.Contains(content, "lurpiclint") {
			t.Errorf("%s: must reference lurpiclint", f)
		}
		// Must reference at least one rule ID.
		if !strings.Contains(content, "LL001") && !strings.Contains(content, "LL003") {
			t.Errorf("%s: must reference at least one rule ID (e.g. LL001)", f)
		}
	}
}
