package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecRunner_Run_goVersion(t *testing.T) {
	r := newExecRunner()
	var stdout, stderr bytes.Buffer
	err := r.Run(CommandSpec{
		Path:   "go",
		Args:   []string{"version"},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("go version failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "go") {
		t.Fatalf("expected go version output, got %q", stdout.String())
	}
}

func TestExecRunner_Run_nonZeroExit(t *testing.T) {
	r := newExecRunner()
	var stdout, stderr bytes.Buffer
	err := r.Run(CommandSpec{
		Path:   "go",
		Args:   []string{"tool", "nonexistent"},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err == nil {
		t.Fatal("expected error for nonexistent tool")
	}
	if stderr.Len() == 0 {
		t.Log("non-zero exit should produce stderr output")
	}
}

func TestExecRunner_Run_nilWritersDiscard(t *testing.T) {
	r := newExecRunner()
	err := r.Run(CommandSpec{
		Path: "go",
		Args: []string{"version"},
	})
	if err != nil {
		t.Fatalf("go version failed: %v", err)
	}
}

func TestExecRunner_Output_goEnv(t *testing.T) {
	r := newExecRunner()
	out, err := r.Output(CommandSpec{
		Path: "go",
		Args: []string{"env", "GOROOT"},
	})
	if err != nil {
		t.Fatalf("go env GOROOT failed: %v", err)
	}
	if len(bytes.TrimSpace(out)) == 0 {
		t.Fatal("expected non-empty GOROOT output")
	}
}

func TestExecRunner_Output_combinedCapture(t *testing.T) {
	r := newExecRunner()
	out, err := r.Output(CommandSpec{
		Path: "go",
		Args: []string{"tool", "nonexistent"},
	})
	if err == nil {
		t.Fatal("expected error for nonexistent tool")
	}
	if len(out) == 0 {
		t.Log("expected combined output from failed command")
	}
}

func TestExecRunner_Look_found(t *testing.T) {
	r := newExecRunner()
	path, err := r.Look("go")
	if err != nil {
		t.Fatalf("Look(go) failed: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("expected absolute path, got %q", path)
	}
}

func TestExecRunner_Look_notFound(t *testing.T) {
	r := newExecRunner()
	_, err := r.Look("this-executable-does-not-exist-12345")
	if err == nil {
		t.Fatal("expected error for nonexistent executable")
	}
}

func TestExecRunner_Dir_propagation(t *testing.T) {
	r := newExecRunner()
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := r.Output(CommandSpec{
		Path: "ls",
		Args: []string{"test.txt"},
		Dir:  tmpDir,
	})
	if err != nil {
		t.Fatalf("ls test.txt in %s failed: %v", tmpDir, err)
	}
	if !strings.Contains(string(out), "test.txt") {
		t.Fatalf("expected ls to find test.txt in %s, got %q", tmpDir, string(out))
	}
}

func TestExecRunner_Env_propagation(t *testing.T) {
	r := newExecRunner()
	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal("sh not found:", err)
	}
	out, err := r.Output(CommandSpec{
		Path: shPath,
		Args: []string{"-c", "echo -n LURPIC_TEST_VAR=${LURPIC_TEST_VAR}"},
		Env:  append(os.Environ(), "LURPIC_TEST_VAR=runner_test_value"),
	})
	if err != nil {
		t.Fatalf("env check failed: %v", err)
	}
	got := string(out)
	if got != "LURPIC_TEST_VAR=runner_test_value" {
		t.Fatalf("expected LURPIC_TEST_VAR=runner_test_value, got %q", got)
	}
}

func TestFakeRunner_RecordsCalls(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "devices")).Then(
		"List of devices attached\nemulator-5554\tdevice\n",
		"",
		nil,
	)

	err := f.Run(CommandSpec{
		Path: "adb",
		Args: []string{"devices"},
	})
	if err != nil {
		t.Fatalf("fake adb devices failed: %v", err)
	}

	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Path != "adb" {
		t.Fatalf("expected path adb, got %q", calls[0].Path)
	}
	if len(calls[0].Args) != 1 || calls[0].Args[0] != "devices" {
		t.Fatalf("expected args [devices], got %v", calls[0].Args)
	}
}

func TestFakeRunner_OutputReturnsCannedStdout(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("emulator", "-list-avds")).Then(
		"pixel_6\npixel_7\n",
		"",
		nil,
	)

	out, err := f.Output(CommandSpec{
		Path: "emulator",
		Args: []string{"-list-avds"},
	})
	if err != nil {
		t.Fatalf("fake emulator -list-avds failed: %v", err)
	}
	expected := "pixel_6\npixel_7\n"
	if string(out) != expected {
		t.Fatalf("expected %q, got %q", expected, string(out))
	}
}

func TestFakeRunner_OutputReturnsCannedStderrWithError(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "install", "-r", "app.apk")).Then(
		"",
		"adb: error: failed to install",
		errors.New("install failed"),
	)

	_, err := f.Output(CommandSpec{
		Path: "adb",
		Args: []string{"install", "-r", "app.apk"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "install failed" {
		t.Fatalf("expected 'install failed', got %v", err)
	}
}

func TestFakeRunner_RunWritesCannedOutput(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("go", "version")).Then(
		"go version go1.24.0 linux/amd64\n",
		"",
		nil,
	)

	var stdout bytes.Buffer
	err := f.Run(CommandSpec{
		Path:   "go",
		Args:   []string{"version"},
		Stdout: &stdout,
	})
	if err != nil {
		t.Fatalf("fake go version failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "go version") {
		t.Fatalf("expected go version output, got %q", stdout.String())
	}
}

func TestFakeRunner_MatchByPathOnly(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb")).Then(
		"output",
		"",
		nil,
	)

	out, err := f.Output(CommandSpec{
		Path: "adb",
		Args: []string{"devices"},
	})
	if err != nil {
		t.Fatalf("got unexpected error: %v", err)
	}
	if string(out) != "output" {
		t.Fatalf("expected 'output', got %q", string(out))
	}
}

func TestFakeRunner_LastRegisteredMatchWins(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb")).Then(
		"first",
		"",
		nil,
	)
	f.When(MatchCommand("adb")).Then(
		"second",
		"",
		nil,
	)

	out, err := f.Output(CommandSpec{
		Path: "adb",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "second" {
		t.Fatalf("expected 'second' (last registered), got %q", string(out))
	}
}

func TestFakeRunner_NoMatchReturnsError(t *testing.T) {
	f := newFakeRunner()
	err := f.Run(CommandSpec{
		Path: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for unmatched command")
	}
}

func TestFakeRunner_RecordsMultipleCallsInOrder(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "devices")).Then(
		"emulator-5554\tdevice\n",
		"",
		nil,
	)
	f.When(MatchCommand("adb", "install", "-r", "test.apk")).Then(
		"Success\n",
		"",
		nil,
	)

	_, _ = f.Output(CommandSpec{Path: "adb", Args: []string{"devices"}})
	_ = f.Run(CommandSpec{Path: "adb", Args: []string{"install", "-r", "test.apk"}})

	calls := f.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].Args[0] != "devices" {
		t.Fatalf("call 0: expected devices, got %v", calls[0].Args)
	}
	if calls[1].Args[0] != "install" {
		t.Fatalf("call 1: expected install, got %v", calls[1].Args)
	}
}

func TestFakeRunner_CallCount(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("echo")).Then("", "", nil)

	if f.CallCount() != 0 {
		t.Fatalf("expected 0 calls before execution, got %d", f.CallCount())
	}

	_ = f.Run(CommandSpec{Path: "echo"})
	if f.CallCount() != 1 {
		t.Fatalf("expected 1 call, got %d", f.CallCount())
	}

	_ = f.Run(CommandSpec{Path: "echo"})
	if f.CallCount() != 2 {
		t.Fatalf("expected 2 calls, got %d", f.CallCount())
	}
}

func TestFakeRunner_OutputIncludesBothStdoutAndStderr(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "shell", "input", "keyevent", "82")).Then(
		"stdout line\n",
		"stderr warning\n",
		nil,
	)

	out, err := f.Output(CommandSpec{
		Path: "adb",
		Args: []string{"shell", "input", "keyevent", "82"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	combined := string(out)
	if !strings.Contains(combined, "stdout line") || !strings.Contains(combined, "stderr warning") {
		t.Fatalf("expected both stdout and stderr in combined output, got %q", combined)
	}
}

func TestFakeRunner_Look_found(t *testing.T) {
	f := newFakeRunner()
	path, err := f.Look("go")
	if err != nil {
		t.Fatalf("Look(go) failed: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
}

func TestFakeRunner_Look_notFound(t *testing.T) {
	f := newFakeRunner()
	_, err := f.Look("this-executable-does-not-exist-12345")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMatchCommand_exactArgs(t *testing.T) {
	m := MatchCommand("adb", "devices")
	if !m(CommandSpec{Path: "adb", Args: []string{"devices"}}) {
		t.Fatal("should match adb devices")
	}
	if m(CommandSpec{Path: "adb", Args: []string{"install", "app.apk"}}) {
		t.Fatal("should not match adb install")
	}
	if m(CommandSpec{Path: "go"}) {
		t.Fatal("should not match go")
	}
}

func TestMatchCommand_pathOnly(t *testing.T) {
	m := MatchCommand("adb")
	if !m(CommandSpec{Path: "adb", Args: []string{"devices"}}) {
		t.Fatal("should match adb with any args")
	}
	if !m(CommandSpec{Path: "adb"}) {
		t.Fatal("should match adb with no args")
	}
	if m(CommandSpec{Path: "go"}) {
		t.Fatal("should not match go")
	}
}

func TestExecRunner_Output_stdinIsNotSentToStderr(t *testing.T) {
	r := newExecRunner()
	out, err := r.Output(CommandSpec{
		Path:   "go",
		Args:   []string{"env", "GOARCH"},
		Stdin:  strings.NewReader("should not appear in output"),
	})
	if err != nil {
		t.Fatalf("go env GOARCH failed: %v", err)
	}
	got := strings.TrimSpace(string(out))
	if got == "" {
		t.Fatal("expected non-empty GOARCH output")
	}
	if strings.Contains(got, "should not appear") {
		t.Fatal("stdin content leaked into output")
	}
}

func TestExecRunner_Start_thenWait(t *testing.T) {
	r := newExecRunner()
	handle, err := r.Start(CommandSpec{
		Path: "go",
		Args: []string{"version"},
	})
	if err != nil {
		t.Fatalf("Start go version: %v", err)
	}
	if err := handle.Wait(); err != nil {
		t.Fatalf("Wait go version: %v", err)
	}
}

func TestExecRunner_Start_kill(t *testing.T) {
	r := newExecRunner()
	handle, err := r.Start(CommandSpec{
		Path: "sleep",
		Args: []string{"60"},
	})
	if err != nil {
		t.Fatalf("Start sleep: %v", err)
	}
	if err := handle.Kill(); err != nil {
		t.Fatalf("Kill sleep: %v", err)
	}
	if err := handle.Wait(); err == nil {
		t.Log("killed process may return nil or non-nil error on Wait")
	}
}

func TestFakeRunner_Start_recordsCall(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("emulator")).Then("", "", nil)

	handle, err := f.Start(CommandSpec{
		Path: "emulator",
		Args: []string{"-avd", "test"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Path != "emulator" {
		t.Fatalf("expected emulator, got %q", calls[0].Path)
	}

	// ProcessHandle should be a no-op fake
	if err := handle.Kill(); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	if err := handle.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}

func TestFakeRunner_Start_returnsErrorOnNoMatch(t *testing.T) {
	f := newFakeRunner()
	// Don't register any match - Start should still succeed (returns fake handle)
	// but the fake handle's Wait will return an error
	handle, err := f.Start(CommandSpec{
		Path: "nonexistent",
	})
	if err != nil {
		t.Fatalf("Start should not fail even without match: %v", err)
	}
	if err := handle.Wait(); err == nil {
		t.Fatal("expected Wait to return error for unmatched command")
	}
}
