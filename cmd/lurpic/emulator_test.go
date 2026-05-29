package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func createFakeADB(t *testing.T, sdkDir, output string) {
	t.Helper()
	platformTools := filepath.Join(sdkDir, "platform-tools")
	if err := os.MkdirAll(platformTools, 0o755); err != nil {
		t.Fatal(err)
	}
	adbPath := filepath.Join(platformTools, "adb")
	script := "#!/bin/sh\nprintf '" + output + "'\n"
	if err := os.WriteFile(adbPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func createFakeEmulator(t *testing.T, sdkDir string) string {
	t.Helper()
	emulatorDir := filepath.Join(sdkDir, "emulator")
	if err := os.MkdirAll(emulatorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	emuPath := filepath.Join(emulatorDir, "emulator")
	// Write a script that echoes args then sleeps briefly
	script := "#!/bin/sh\necho \"$@\" > /dev/null\n"
	if err := os.WriteFile(emuPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return emuPath
}

func createFakeSDK(t *testing.T, sdkDir string) {
	t.Helper()
	createFakeADB(t, sdkDir, "List of devices attached\n\n")
	createFakeEmulator(t, sdkDir)
	// Create platforms dir for validity
	if err := os.MkdirAll(filepath.Join(sdkDir, "platforms", "android-33"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Create cmdline-tools for sdkmanager/avdmanager
	cmdlineTools := filepath.Join(sdkDir, "cmdline-tools", "latest", "bin")
	if err := os.MkdirAll(cmdlineTools, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sdkmanager", "avdmanager"} {
		p := filepath.Join(cmdlineTools, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEmulatorManager_launchArgv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses shell script helpers")
	}

	sdkDir := t.TempDir()
	createFakeSDK(t, sdkDir)

	f := newFakeRunner()
	arch := DefaultEmulatorArchitecture()

	adbPath := filepath.Join(sdkDir, "platform-tools", "adb")
	emuTool := filepath.Join(sdkDir, "emulator", "emulator")

	// Fake adb output: no devices
	f.When(MatchCommand(adbPath, "devices")).Then("List of devices attached\n\n", "", nil)

	// Fake emulator -list-avds: no existing AVD, will create one
	f.When(MatchCommand(emuTool, "-list-avds")).Then("\n", "", nil)

	// Register sdkmanager and avdmanager command matchers (resolved by findCmdlineTool)
	sdkmanagerPath := filepath.Join(sdkDir, "cmdline-tools", "latest", "bin", "sdkmanager")
	avdmanagerPath := filepath.Join(sdkDir, "cmdline-tools", "latest", "bin", "avdmanager")
	f.When(MatchCommand(sdkmanagerPath)).Then("", "", nil)
	f.When(MatchCommand(avdmanagerPath)).Then("", "", nil)

	// Emulator launch should succeed
	emuArgs := []string{
		"-avd", "lurpic_api33_google_apis_x86_64",
		"-no-snapshot",
		"-no-boot-anim",
		"-gpu", "auto",
		"-port", "5554",
	}
	f.When(MatchCommand(emuTool)).Then("", "", nil)

	// Wait-for-device and boot checks
	f.When(MatchCommand(adbPath, "-s", "emulator-5554", "wait-for-device")).Then("", "", nil)
	f.When(MatchCommand(adbPath, "-s", "emulator-5554", "shell", "getprop", "sys.boot_completed")).Then("1\n", "", nil)
	f.When(MatchCommand(adbPath, "-s", "emulator-5554", "shell", "pm", "path", "android")).Then("package:/system/framework/framework.jar\n", "", nil)

	mgr := NewEmulatorManager(f, sdkDir, 33, arch, "auto", false)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := mgr.EnsureRunning(ctx)
	if err != nil {
		t.Fatalf("EnsureRunning: %v", err)
	}
	defer sess.Close()

	calls := f.Calls()
	// Find the emulator launch call
	var launchCall *CommandSpec
	for i := range calls {
		if calls[i].Path == emuTool && len(calls[i].Args) > 0 && calls[i].Args[0] == "-avd" {
			launchCall = &calls[i]
			break
		}
	}
	if launchCall == nil {
		t.Fatal("expected an emulator launch call")
	}
	if len(launchCall.Args) != len(emuArgs) {
		t.Fatalf("expected %d args, got %d: %v", len(emuArgs), len(launchCall.Args), launchCall.Args)
	}
	for i, expected := range emuArgs {
		if launchCall.Args[i] != expected {
			t.Fatalf("arg %d: expected %q, got %q", i, expected, launchCall.Args[i])
		}
	}
	if sess.Serial != "emulator-5554" {
		t.Fatalf("expected serial emulator-5554, got %q", sess.Serial)
	}
	if !sess.spawned {
		t.Fatal("expected session to be marked as spawned")
	}
}

func TestEmulatorManager_reusesRunning(t *testing.T) {
	sdkDir := t.TempDir()
	createFakeSDK(t, sdkDir)

	adbPath := filepath.Join(sdkDir, "platform-tools", "adb")

	f := newFakeRunner()
	arch := DefaultEmulatorArchitecture()

	// Fake adb output: one running emulator
	f.When(MatchCommand(adbPath, "devices")).Then(
		"List of devices attached\nemulator-5554\tdevice\n",
		"", nil,
	)

	mgr := NewEmulatorManager(f, sdkDir, 33, arch, "auto", false)
	ctx := context.Background()

	sess, err := mgr.EnsureRunning(ctx)
	if err != nil {
		t.Fatalf("EnsureRunning: %v", err)
	}
	defer sess.Close()

	if sess.Serial != "emulator-5554" {
		t.Fatalf("expected serial emulator-5554, got %q", sess.Serial)
	}
	if sess.spawned {
		t.Fatal("expected session to NOT be marked as spawned (reused)")
	}

	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected only 1 call (adb devices), got %d", len(calls))
	}
}

func TestEmulatorSession_CloseKillsSpawned(t *testing.T) {
	f := newFakeRunner()
	handle, err := f.Start(CommandSpec{Path: "sleep", Args: []string{"60"}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	sess := &EmulatorSession{
		Serial:  "emulator-5554",
		proc:    handle,
		spawned: true,
	}

	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestEmulatorSession_CloseNoopForReused(t *testing.T) {
	sess := &EmulatorSession{
		Serial:  "emulator-5554",
		proc:    nil,
		spawned: false,
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("Close should be a no-op: %v", err)
	}
}

func TestEmulatorSession_CloseNoopForNil(t *testing.T) {
	var sess *EmulatorSession
	if err := sess.Close(); err != nil {
		t.Fatalf("Close on nil should be a no-op: %v", err)
	}
}

func TestEmulatorManager_findRunningEmulator_parses(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "devices")).Then(
		"List of devices attached\nemulator-5554\tdevice\nemulator-5556\toffline\n",
		"", nil,
	)

	mgr := &EmulatorManager{runner: f}
	serial, err := mgr.findRunningEmulator("adb")
	if err != nil {
		t.Fatalf("findRunningEmulator: %v", err)
	}
	if serial != "emulator-5554" {
		t.Fatalf("expected emulator-5554, got %q", serial)
	}
}

func TestEmulatorManager_findRunningEmulator_none(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "devices")).Then(
		"List of devices attached\n\n", "", nil,
	)

	mgr := &EmulatorManager{runner: f}
	serial, err := mgr.findRunningEmulator("adb")
	if err != nil {
		t.Fatalf("findRunningEmulator: %v", err)
	}
	if serial != "" {
		t.Fatalf("expected empty serial, got %q", serial)
	}
}

func TestEmulatorManager_gpuModeDefault(t *testing.T) {
	mgr := NewEmulatorManager(newFakeRunner(), "/sdk", 33, DefaultEmulatorArchitecture(), "", false)
	if mgr.gpuMode != "auto" {
		t.Fatalf("default gpuMode should be 'auto', got %q", mgr.gpuMode)
	}
}

func TestEmulatorManager_gpuModeExplicit(t *testing.T) {
	mgr := NewEmulatorManager(newFakeRunner(), "/sdk", 33, DefaultEmulatorArchitecture(), "host", true)
	if mgr.gpuMode != "host" {
		t.Fatalf("expected gpuMode 'host', got %q", mgr.gpuMode)
	}
	if !mgr.headless {
		t.Fatal("expected headless to be true")
	}
}
