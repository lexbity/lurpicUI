package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteCrashReport_createsFile(t *testing.T) {
	tmpDir := t.TempDir()
	crashDir = tmpDir

	report := CrashReport{
		Signal:  "SIGABRT",
		Stack:   "goroutine 1 [running]:\nmain.foo()\n\tsrc/main.go:10",
		Version: "0.1.0-test",
		GoArch:  "arm64",
		GoOS:    "android",
	}
	writeCrashReport(report)

	// Find the created crash file
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one crash file")
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !strings.Contains(string(content), "SIGABRT") {
		t.Errorf("expected crash report to contain SIGABRT, got: %s", string(content))
	}
	if !strings.Contains(string(content), "arm64") {
		t.Errorf("expected crash report to contain arch, got: %s", string(content))
	}
	if !strings.Contains(string(content), "main.foo") {
		t.Errorf("expected crash report to contain stack trace, got: %s", string(content))
	}
}

func TestWriteCrashReport_panicReport(t *testing.T) {
	tmpDir := t.TempDir()
	crashDir = tmpDir

	report := CrashReport{
		Panic:   "runtime error: index out of range [5] with length 3",
		Stack:   "goroutine 1 [running]:\nmain.bar()\n\tsrc/main.go:42",
		Version: "0.1.0-test",
		GoArch:  "amd64",
		GoOS:    "linux",
	}
	writeCrashReport(report)

	entries, _ := os.ReadDir(tmpDir)
	if len(entries) == 0 {
		t.Fatal("expected crash file")
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !strings.Contains(string(content), "index out of range") {
		t.Errorf("expected panic message in report, got: %s", string(content))
	}
	if !strings.Contains(string(content), "main.bar") {
		t.Errorf("expected stack trace in report, got: %s", string(content))
	}
}

func TestStackTrace_nonEmpty(t *testing.T) {
	trace := stackTrace(0)
	if len(trace) == 0 {
		t.Fatal("expected non-empty stack trace")
	}
	if !strings.Contains(trace, "stackTrace") {
		t.Errorf("expected stack trace to contain 'stackTrace', got: %s", trace)
	}
}

func TestWrapMain_panicRecovery(t *testing.T) {
	ran := false
	fn := WrapMain(func() error {
		ran = true
		return nil
	})
	if err := fn(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !ran {
		t.Fatal("expected function to run")
	}
}

func TestWrapMain_panicCaptured(t *testing.T) {
	tmpDir := t.TempDir()
	crashDir = tmpDir

	fn := WrapMain(func() error {
		panic("test panic for crash handler")
	})
	err := fn()
	if err == nil {
		t.Fatal("expected error from panicking function")
	}
	if !strings.Contains(err.Error(), "test panic") {
		t.Errorf("expected error to contain 'test panic', got: %v", err)
	}

	// Verify crash report was written
	entries, _ := os.ReadDir(tmpDir)
	if len(entries) == 0 {
		t.Fatal("expected crash report file")
	}
}
