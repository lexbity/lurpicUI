package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDoctor_checkGo_present(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("go", "version")).Then("go version go1.24.0 linux/amd64\n", "", nil)

	// Just verify it doesn't panic and uses the runner
	checkGo(f, false)
}

func TestDoctor_checkGo_absent(t *testing.T) {
	f := newFakeRunner()
	// No match registered → runner returns error

	checkGo(f, false)
}

func TestDoctor_checkRust_present(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("rustc", "--version")).Then("rustc 1.80.0\n", "", nil)

	checkRust(f, false)
}

func TestDoctor_checkRust_absent(t *testing.T) {
	f := newFakeRunner()
	checkRust(f, false)
}

func TestDoctor_checkCargoNdk_present(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("cargo", "ndk", "--version")).Then("cargo-ndk 3.0.0\n", "", nil)

	checkCargoNdk(f, false)
}

func TestDoctor_checkCargoNdk_absent(t *testing.T) {
	f := newFakeRunner()
	checkCargoNdk(f, false)
}

func TestDoctor_checkSystemImage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses unix paths")
	}

	sdkDir := t.TempDir()
	// Create system image directory
	imgDir := filepath.Join(sdkDir, "system-images", "android-33", "google_apis", "x86_64")
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if !checkSystemImage(sdkDir, 33, "x86_64") {
		t.Fatal("expected system image to be found")
	}
}

func TestDoctor_checkSystemImage_notFound(t *testing.T) {
	sdkDir := t.TempDir()
	if checkSystemImage(sdkDir, 33, "x86_64") {
		t.Fatal("expected system image to not be found")
	}
}

func TestDoctor_checkManagedAVD(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	avdName := "lurpic_api33_google_apis_x86_64"
	avdDir := filepath.Join(homeDir, ".android", "avd", avdName+".avd")
	if err := os.MkdirAll(avdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if !checkManagedAVD(avdName) {
		t.Fatal("expected managed AVD to be found")
	}
}

func TestDoctor_checkManagedAVD_notFound(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	if checkManagedAVD("lurpic_api33_google_apis_x86_64") {
		t.Fatal("expected managed AVD to not be found")
	}
}

func TestDoctor_findEmulatorTool(t *testing.T) {
	sdkDir := t.TempDir()
	emuDir := filepath.Join(sdkDir, "emulator")
	if err := os.MkdirAll(emuDir, 0o755); err != nil {
		t.Fatal(err)
	}
	emuPath := filepath.Join(emuDir, "emulator")
	if err := os.WriteFile(emuPath, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	tool, err := findEmulatorTool(sdkDir)
	if err != nil {
		t.Fatalf("findEmulatorTool: %v", err)
	}
	if tool != emuPath {
		t.Fatalf("expected %q, got %q", emuPath, tool)
	}
}

func TestDoctor_findCmdlineTool(t *testing.T) {
	sdkDir := t.TempDir()
	toolDir := filepath.Join(sdkDir, "cmdline-tools", "latest", "bin")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	toolPath := filepath.Join(toolDir, "sdkmanager")
	if err := os.WriteFile(toolPath, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	tool, err := findCmdlineTool(sdkDir, "sdkmanager")
	if err != nil {
		t.Fatalf("findCmdlineTool: %v", err)
	}
	if tool != toolPath {
		t.Fatalf("expected %q, got %q", toolPath, tool)
	}
}
