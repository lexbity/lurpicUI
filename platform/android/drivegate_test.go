package android_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDriveGate_tagGateCompilesBothWays is the G10.1 gate: the drivegate
// package compiles under GOOS=android with and without the lurpicdrive tag.
// It exercises the real gate files via a synthetic package (the tag pair
// lives in platform/android, which needs the NDK toolchain to build — the
// gate files themselves are self-contained).
func TestDriveGate_tagGateCompilesBothWays(t *testing.T) {
	srcA, err := os.ReadFile("drivegate_android.go")
	if err != nil {
		t.Fatalf("read tagged gate: %v", err)
	}
	srcS, err := os.ReadFile("drivegate_stub.go")
	if err != nil {
		t.Fatalf("read stub gate: %v", err)
	}
	for _, tc := range []struct {
		name string
		tag  bool
		src  string
	}{
		{"tagged", true, string(srcA)},
		{"untagged", false, string(srcS)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "gate.go"), []byte(tc.src), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module gate\n\ngo 1.27.2\n"), 0o644); err != nil {
				t.Fatalf("write go.mod: %v", err)
			}
			// Package clause lives under platform/android (package android);
			// the synthetic copy keeps the clause and builds standalone.
			args := []string{
				"build",
				"-o", filepath.Join(dir, "out.a"),
			}
			if tc.tag {
				args = append(args, "-tags", "lurpicdrive")
			}
			args = append(args, "./.")
			cmd := exec.Command("go", args...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GOOS=android", "GOARCH=arm64", "CGO_ENABLED=0")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("GOOS=android build (%s): %v\n%s", tc.name, err, out)
			}
		})
	}
}

// TestDriveGate_releaseAbsence is the G10.2 gate: a GOOS=android build
// WITHOUT the lurpicdrive tag carries the stub's unavailable marker and NOT
// the tagged gate's enabled marker — verified via go tool nm on the archive.
func TestDriveGate_releaseAbsence(t *testing.T) {
	srcA, _ := os.ReadFile("drivegate_android.go")
	srcS, _ := os.ReadFile("drivegate_stub.go")

	build := func(t *testing.T, src string, tag bool) string {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "gate.go"), []byte(src), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module gate\n\ngo 1.27.2\n"), 0o644); err != nil {
			t.Fatalf("write go.mod: %v", err)
		}
		out := filepath.Join(dir, "out.a")
		args := []string{"build", "-o", out}
		if tag {
			args = append(args, "-tags", "lurpicdrive")
		}
		args = append(args, "./.")
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOOS=android", "GOARCH=arm64", "CGO_ENABLED=0")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build: %v\n%s", err, output)
		}
		return out
	}

	// Stub (release posture): the unavailable refusal symbol is present.
	stubArchive := build(t, string(srcS), false)
	nmStub, err := exec.Command("go", "tool", "nm", stubArchive).CombinedOutput()
	if err != nil {
		t.Fatalf("nm stub: %v", err)
	}
	if !strings.Contains(string(nmStub), "errDriveUnavailable") {
		t.Fatal("release posture must carry the unavailable refusal symbol")
	}

	// Tagged (debug posture): the refusal symbol is absent.
	tagArchive := build(t, string(srcA), true)
	nmTagged, err := exec.Command("go", "tool", "nm", tagArchive).CombinedOutput()
	if err != nil {
		t.Fatalf("nm tagged: %v", err)
	}
	if strings.Contains(string(nmTagged), "errDriveUnavailable") {
		t.Fatal("tagged build must not carry the refusal symbol")
	}
}

// TestDriveGate_builderWiring: the android builder passes -tags lurpicdrive
// on debug builds and never on release builds (source-level wiring check —
// the full build needs the NDK toolchain).
func TestDriveGate_builderWiring(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "cmd", "lurpic", "android_builder.go"))
	if err != nil {
		t.Fatalf("read builder: %v", err)
	}
	src := string(raw)
	if !strings.Contains(src, "lurpicdrive") {
		t.Fatal("android builder does not wire the lurpicdrive tag")
	}
	// The tag must be conditional on the debug (non-release) build.
	idx := strings.Index(src, "lurpicdrive")
	context := src[max(0, idx-600) : idx+200]
	if strings.Contains(context, "if b.release {") && !strings.Contains(context, "!b.release") {
		t.Fatal("lurpicdrive tag must be wired to the non-release build")
	}
}
