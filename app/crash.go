package app

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

// CrashReport contains information about a captured crash.
type CrashReport struct {
	Time    time.Time `json:"time"`
	Signal  string    `json:"signal,omitempty"`
	Panic   string    `json:"panic,omitempty"`
	Stack   string    `json:"stack"`
	Version string    `json:"version"`
	GoArch  string    `json:"goarch"`
	GoOS    string    `json:"goos"`
}

// crashDir is the directory where crash reports are written.
// On Android this is the app's files directory; on desktop it's a temp dir.
var crashDir = func() string {
	if dir := os.Getenv("LURPIC_CRASH_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err == nil {
		dir := filepath.Join(home, ".lurpic", "crashes")
		//nolint:gosec // crash dump dir
		_ = os.MkdirAll(dir, 0755)
		return dir
	}
	return os.TempDir()
}()

// InstallCrashHandler sets up signal handlers and a Go panic handler that
// writes crash reports to the crash directory.
func InstallCrashHandler() {
	// Capture SIGABRT (native crash on Android) and SIGSEGV.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGABRT, syscall.SIGSEGV)
	go func() {
		for sig := range sigCh {
			writeCrashReport(CrashReport{
				Time:    time.Now(),
				Signal:  sig.String(),
				Stack:   stackTrace(3),
				Version: version,
				GoArch:  runtime.GOARCH,
				GoOS:    runtime.GOOS,
			})
		}
	}()

	// Capture Go panics via a deferred recover in the main loop.
	// This is installed by calling WrapMain with the app's run function.
}

// WrapMain wraps an app run function with panic recovery and crash reporting.
func WrapMain(fn func() error) func() error {
	return func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				panicMsg := fmt.Sprintf("%v", r)
				report := CrashReport{
					Time:    time.Now(),
					Panic:   panicMsg,
					Stack:   stackTrace(3),
					Version: version,
					GoArch:  runtime.GOARCH,
					GoOS:    runtime.GOOS,
				}
				writeCrashReport(report)
				err = fmt.Errorf("app panic: %v", r)
			}
		}()
		return fn()
	}
}

// writeCrashReport writes a crash report to the crash directory.
func writeCrashReport(r CrashReport) {
	name := fmt.Sprintf("crash_%s.txt", r.Time.Format("20060102_150405.000"))
	path := filepath.Join(crashDir, name)
	f, err := os.Create(path) //nolint:gosec // path from user config
	if err != nil {
		fmt.Fprintf(os.Stderr, "crash: cannot write report to %s: %v\n", path, err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "lurpicUI Crash Report\n")
	fmt.Fprintf(f, "Time:    %s\n", r.Time.Format(time.RFC3339))
	fmt.Fprintf(f, "Version: %s\n", r.Version)
	fmt.Fprintf(f, "Arch:    %s/%s\n", r.GoOS, r.GoArch)
	if r.Signal != "" {
		fmt.Fprintf(f, "Signal:  %s\n", r.Signal)
	}
	if r.Panic != "" {
		fmt.Fprintf(f, "Panic:   %s\n", r.Panic)
	}
	fmt.Fprintf(f, "\nStack trace:\n%s\n", r.Stack)
	fmt.Fprintf(f, "--- end crash report ---\n")
}

// stackTrace returns a formatted stack trace, skipping the given number of frames.
func stackTrace(skip int) string {
	buf := make([]byte, 64*1024)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// version is set at build time via -ldflags.
var version = "0.1.0-dev"
