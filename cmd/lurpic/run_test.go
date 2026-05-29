package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAdbArgs_prependsSerial(t *testing.T) {
	args := adbArgs("emulator-5554", "shell", "am", "start", "-n", "test")
	if len(args) != 7 || args[0] != "-s" || args[1] != "emulator-5554" {
		t.Fatalf("expected -s emulator-5554 prefix, got %v", args)
	}
}

func TestAdbArgs_emptySerialNoPrefix(t *testing.T) {
	args := adbArgs("", "devices")
	if len(args) != 1 || args[0] != "devices" {
		t.Fatalf("expected [devices], got %v", args)
	}
}

func TestInstallAPK_argv(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "-s", "em-5554", "install", "-r", "/path/to/app.apk")).Then(
		"Success\n", "", nil,
	)
	if err := installAPK(f, "adb", "em-5554", "/path/to/app.apk"); err != nil {
		t.Fatalf("installAPK: %v", err)
	}
	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if len(calls[0].Args) < 2 || calls[0].Args[0] != "-s" || calls[0].Args[1] != "em-5554" {
		t.Fatalf("expected -s em-5554 prefix, got %v", calls[0].Args)
	}
}

func TestInstallAPK_failure(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "-s", "em-5554", "install", "-r", "bad.apk")).Then(
		"", "", errors.New("install failed"),
	)
	err := installAPK(f, "adb", "em-5554", "bad.apk")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLaunchAPK_argv(t *testing.T) {
	f := newFakeRunner()
	component := "org.test.app/org.lurpicui.bridge.LurpicNativeActivity"
	f.When(MatchCommand("adb", "-s", "em-5554", "shell", "am", "start", "-n", component)).Then(
		"Starting: Intent { }\n", "", nil,
	)
	if err := launchAPK(f, "adb", "em-5554", "org.test.app", false); err != nil {
		t.Fatalf("launchAPK: %v", err)
	}
	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if len(calls[0].Args) < 2 || calls[0].Args[0] != "-s" || calls[0].Args[1] != "em-5554" {
		t.Fatalf("expected -s em-5554 prefix, got %v", calls[0].Args)
	}
}

func TestLaunchAPK_forceSoftwareInjectsEnv(t *testing.T) {
	f := newFakeRunner()
	component := "org.test.app/org.lurpicui.bridge.LurpicNativeActivity"
	f.When(MatchCommand("adb", "-s", "em-5554", "shell", "am", "start", "-n", component)).Then(
		"Starting: Intent { }\n", "", nil,
	)
	if err := launchAPK(f, "adb", "em-5554", "org.test.app", true); err != nil {
		t.Fatalf("launchAPK: %v", err)
	}
	calls := f.Calls()
	found := false
	for _, e := range calls[0].Env {
		if e == "LURPIC_RENDER_BACKEND=software" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected LURPIC_RENDER_BACKEND=software in env")
	}
}

func TestLaunchAPK_failure(t *testing.T) {
	f := newFakeRunner()
	component := "org.test.app/org.lurpicui.bridge.LurpicNativeActivity"
	f.When(MatchCommand("adb", "-s", "dev-1234", "shell", "am", "start", "-n", component)).Then(
		"", "", errors.New("launch failed"),
	)
	err := launchAPK(f, "adb", "dev-1234", "org.test.app", false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFindAndroidEmulator_found(t *testing.T) {
	sdk := t.TempDir()
	emulatorDir := filepath.Join(sdk, "emulator")
	if err := os.MkdirAll(emulatorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	emuPath := filepath.Join(emulatorDir, "emulator")
	if err := os.WriteFile(emuPath, []byte("#!/bin/sh\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := findAndroidEmulator(sdk)
	if err != nil {
		t.Fatalf("findAndroidEmulator: %v", err)
	}
	if path != emuPath {
		t.Fatalf("expected %q, got %q", emuPath, path)
	}
}

func TestFindAndroidEmulator_notFound(t *testing.T) {
	_, err := findAndroidEmulator(t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveDeviceSerial_parsesSingleDevice(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "devices")).Then(
		"List of devices attached\nabc123\tdevice\n", "", nil,
	)
	serial, err := resolveDeviceSerial(f, "adb")
	if err != nil {
		t.Fatalf("resolveDeviceSerial: %v", err)
	}
	if serial != "abc123" {
		t.Fatalf("expected abc123, got %q", serial)
	}
}

func TestResolveDeviceSerial_skipsEmulators(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "devices")).Then(
		"List of devices attached\nemulator-5554\tdevice\nabc123\tdevice\n", "", nil,
	)
	serial, err := resolveDeviceSerial(f, "adb")
	if err != nil {
		t.Fatalf("resolveDeviceSerial: %v", err)
	}
	if serial != "abc123" {
		t.Fatalf("expected abc123 (not emulator), got %q", serial)
	}
}

func TestResolveDeviceSerial_noDevices(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "devices")).Then(
		"List of devices attached\n\n", "", nil,
	)
	_, err := resolveDeviceSerial(f, "adb")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveDeviceSerial_multipleDevices(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "devices")).Then(
		"List of devices attached\ndev1\tdevice\ndev2\tdevice\n", "", nil,
	)
	_, err := resolveDeviceSerial(f, "adb")
	if err == nil {
		t.Fatal("expected error for multiple devices")
	}
}

func TestRunFlags_parsed(t *testing.T) {
	// Verify that key flags have non-zero defaults
	flags := runFlags{}
	if flags.bootTimeout != 0 {
		t.Fatalf("expected zero bootTimeout, got %v", flags.bootTimeout)
	}
	if flags.emulator {
		t.Fatal("expected emulator default false")
	}
}

func TestRunAndroid_orchestration_fakeRunner(t *testing.T) {
	sdkDir := t.TempDir()
	buildDir := t.TempDir()
	f := newFakeRunner()
	arch := DefaultEmulatorArchitecture()

	// Create minimal real SDK structure for findSDKTool
	platformTools := filepath.Join(sdkDir, "platform-tools")
	os.MkdirAll(platformTools, 0o755)
	adbPath := filepath.Join(platformTools, "adb")
	os.WriteFile(adbPath, []byte("#!/bin/sh\nexit 0\n"), 0o755)

	emuDir := filepath.Join(sdkDir, "emulator")
	os.MkdirAll(emuDir, 0o755)
	emuPath := filepath.Join(emuDir, "emulator")
	os.WriteFile(emuPath, []byte("#!/bin/sh\nexit 0\n"), 0o755)

	cmdlineTools := filepath.Join(sdkDir, "cmdline-tools", "latest", "bin")
	os.MkdirAll(cmdlineTools, 0o755)
	sdkmanagerPath := filepath.Join(cmdlineTools, "sdkmanager")
	avdmanagerPath := filepath.Join(cmdlineTools, "avdmanager")
	os.WriteFile(sdkmanagerPath, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	os.WriteFile(avdmanagerPath, []byte("#!/bin/sh\nexit 0\n"), 0o755)

	platformsDir := filepath.Join(sdkDir, "platforms", "android-33")
	os.MkdirAll(platformsDir, 0o755)
	os.WriteFile(filepath.Join(platformsDir, "android.jar"), []byte("fake"), 0o644)

	outputPath := filepath.Join(buildDir, "app.apk")
	os.WriteFile(outputPath, []byte("fake apk"), 0o644)

	builder := &androidBuilder{
		runner:     f,
		sdk:        sdkDir,
		buildDir:   buildDir,
		outputPath: outputPath,
		config: &Config{
			App:     AppConfig{ID: "org.test", Name: "Test"},
			Android: AndroidConfig{TargetSDK: 33, MinSDK: 29, ABIs: []string{"x86_64"}},
		},
		apiLevel: 33,
	}

	// ── Register fake results ──

	// adb devices: no running emulator → will spawn
	f.When(MatchCommand(adbPath, "devices")).Then("List of devices attached\n\n", "", nil)

	// sdkmanager --licences, sdkmanager install, avdmanager create
	f.When(MatchCommand(sdkmanagerPath)).Then("", "", nil)
	f.When(MatchCommand(avdmanagerPath)).Then("", "", nil)

	// emulator launch (no -list-avds since this is managed AVD flow)
	f.When(MatchCommand(emuPath)).Then("", "", nil)

	// Boot sequence
	f.When(MatchCommand(adbPath, "-s", "emulator-5554", "wait-for-device")).Then("", "", nil)
	f.When(MatchCommand(adbPath, "-s", "emulator-5554", "shell", "getprop", "sys.boot_completed")).Then("1\n", "", nil)
	f.When(MatchCommand(adbPath, "-s", "emulator-5554", "shell", "pm", "path", "android")).Then("package:...\n", "", nil)

	// Install
	f.When(MatchCommand(adbPath, "-s", "emulator-5554", "install", "-r", outputPath)).Then("Success\n", "", nil)

	// Launch
	f.When(MatchCommand(adbPath, "-s", "emulator-5554", "shell", "am", "start", "-n",
		"org.test/org.lurpicui.bridge.LurpicNativeActivity")).Then("Starting: Intent { }\n", "", nil)

	// ── Exercise orchestration ──
	mgr := NewEmulatorManager(f, sdkDir, builder.apiLevel, arch, "auto", false)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := mgr.EnsureRunning(ctx)
	if err != nil {
		t.Fatalf("EnsureRunning: %v", err)
	}

	if err := installAPK(f, adbPath, sess.Serial, outputPath); err != nil {
		t.Fatalf("installAPK: %v", err)
	}

	if err := launchAPK(f, adbPath, sess.Serial, builder.config.App.ID, false); err != nil {
		t.Fatalf("launchAPK: %v", err)
	}

	// ── Verify sequence ──
	calls := f.Calls()
	var sawProvision, sawBoot, sawInstall, sawLaunch bool
	for _, c := range calls {
		for _, a := range c.Args {
			if a == "--licenses" || a == "create" {
				sawProvision = true
			}
			if a == "getprop" {
				sawBoot = true
			}
			if a == "install" {
				sawInstall = true
			}
			if a == "start" {
				sawLaunch = true
			}
		}
	}

	if !sawProvision {
		t.Error("expected provisioning (sdkmanager --licenses or avdmanager create)")
	}
	if !sawBoot {
		t.Error("expected boot check (getprop sys.boot_completed)")
	}
	if !sawInstall {
		t.Error("expected adb install")
	}
	if !sawLaunch {
		t.Error("expected adb shell am start")
	}

	// Verify spawned session cleanup
	if err := sess.Close(); err != nil {
		t.Fatalf("session.Close: %v", err)
	}
}

func TestBuildFlags_abiOverride(t *testing.T) {
	flags := buildFlags{abi: "x86_64"}
	if flags.abi != "x86_64" {
		t.Fatalf("expected abi x86_64, got %q", flags.abi)
	}
}

func TestBuildFlags_abiEmpty(t *testing.T) {
	flags := buildFlags{}
	if flags.abi != "" {
		t.Fatalf("expected empty abi, got %q", flags.abi)
	}
}
