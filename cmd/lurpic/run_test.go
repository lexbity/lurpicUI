package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAndroidRunner_run_onRunningEmulator(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a shell script adb helper")
	}

	sdk := t.TempDir()
	platformTools := filepath.Join(sdk, "platform-tools")
	if err := os.MkdirAll(platformTools, 0o755); err != nil {
		t.Fatalf("mkdir platform-tools: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "adb.log")
	adbPath := filepath.Join(platformTools, "adb")
	adbScript := "#!/bin/sh\n" +
		"log_file=\"" + logPath + "\"\n" +
		"case \"$1\" in\n" +
		"  devices)\n" +
		"    printf 'List of devices attached\\n'\n" +
		"    printf 'emulator-5554\\tdevice\\n'\n" +
		"    ;;\n" +
		"  install)\n" +
		"    printf 'install %s\\n' \"$*\" >> \"$log_file\"\n" +
		"    printf 'Success\\n'\n" +
		"    ;;\n" +
		"  shell)\n" +
		"    printf 'shell %s\\n' \"$*\" >> \"$log_file\"\n" +
		"    printf 'Starting: Intent { }\\n'\n" +
		"    ;;\n" +
		"  wait-for-device)\n" +
		"    printf 'wait-for-device\\n' >> \"$log_file\"\n" +
		"    ;;\n" +
		"  *)\n" +
		"    printf 'unexpected %s\\n' \"$*\" >> \"$log_file\"\n" +
		"    ;;\n" +
		"esac\n"
	if err := os.WriteFile(adbPath, []byte(adbScript), 0o755); err != nil {
		t.Fatalf("write adb helper: %v", err)
	}

	apkPath := filepath.Join(t.TempDir(), "app.apk")
	if err := os.WriteFile(apkPath, []byte("fake apk"), 0o644); err != nil {
		t.Fatalf("write apk helper: %v", err)
	}

	runner := &androidRunner{
		runner:      newExecRunner(),
		emulator:    true,
		sdk:         sdk,
		apkPath:     apkPath,
		packageName: "org.example.app",
	}
	if err := runner.run(); err != nil {
		t.Fatalf("runner.run() error = %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read adb log: %v", err)
	}
	logs := string(data)
	if !strings.Contains(logs, "install install -r "+apkPath) {
		t.Fatalf("expected install call in log, got:\n%s", logs)
	}
	if !strings.Contains(logs, "shell shell am start -n org.example.app/org.lurpicui.bridge.LurpicNativeActivity") {
		t.Fatalf("expected launch call in log, got:\n%s", logs)
	}
}

func TestSelectAndroidAVD_createsDefaultWhenNoneExist(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses shell script tool helpers")
	}

	sdk := t.TempDir()
	platformTools := filepath.Join(sdk, "platform-tools")
	cmdlineTools := filepath.Join(sdk, "cmdline-tools", "latest", "bin")
	emulatorDir := filepath.Join(sdk, "emulator")
	for _, dir := range []string{platformTools, cmdlineTools, emulatorDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	logPath := filepath.Join(t.TempDir(), "tool.log")
	writeTool := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	writeTool(filepath.Join(emulatorDir, "emulator"), "#!/bin/sh\n"+
		"log_file=\""+logPath+"\"\n"+
		"if [ \"$1\" = \"-list-avds\" ]; then\n"+
		"  printf '\\n'\n"+
		"  exit 0\n"+
		"fi\n"+
		"printf 'emulator %s\\n' \"$*\" >> \"$log_file\"\n")

	writeTool(filepath.Join(cmdlineTools, "sdkmanager"), "#!/bin/sh\n"+
		"log_file=\""+logPath+"\"\n"+
		"printf 'sdkmanager %s\\n' \"$*\" >> \"$log_file\"\n")

	writeTool(filepath.Join(cmdlineTools, "avdmanager"), "#!/bin/sh\n"+
		"log_file=\""+logPath+"\"\n"+
		"printf 'avdmanager %s\\n' \"$*\" >> \"$log_file\"\n")

	r := &androidRunner{
		runner: newExecRunner(),
		sdk:    sdk,
	}
	avd, err := r.selectAndroidAVD(filepath.Join(emulatorDir, "emulator"))
	if err != nil {
		t.Fatalf("selectAndroidAVD() error = %v", err)
	}

	want := defaultAndroidAVDName()
	if avd != want {
		t.Fatalf("selectAndroidAVD() = %q, want %q", avd, want)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tool log: %v", err)
	}
	logs := string(data)
	if !strings.Contains(logs, "sdkmanager "+defaultAndroidSystemImage()) {
		t.Fatalf("expected sdkmanager install call, got:\n%s", logs)
	}
	if !strings.Contains(logs, "avdmanager create avd -n "+want) {
		t.Fatalf("expected avdmanager create call, got:\n%s", logs)
	}
}

func TestHasRunningEmulator_parsesDeviceOutput(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "devices")).Then(
		"List of devices attached\nemulator-5554\tdevice\n",
		"",
		nil,
	)

	r := &androidRunner{runner: f}
	running, err := r.hasRunningEmulator("adb")
	if err != nil {
		t.Fatalf("hasRunningEmulator: %v", err)
	}
	if !running {
		t.Fatal("expected running emulator")
	}
}

func TestHasRunningEmulator_noDevice(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "devices")).Then(
		"List of devices attached\n\n",
		"",
		nil,
	)

	r := &androidRunner{runner: f}
	running, err := r.hasRunningEmulator("adb")
	if err != nil {
		t.Fatalf("hasRunningEmulator: %v", err)
	}
	if running {
		t.Fatal("expected no running emulator")
	}
}

func TestHasRunningEmulator_offlineDevice(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "devices")).Then(
		"List of devices attached\nemulator-5554\toffline\n",
		"",
		nil,
	)

	r := &androidRunner{runner: f}
	running, err := r.hasRunningEmulator("adb")
	if err != nil {
		t.Fatalf("hasRunningEmulator: %v", err)
	}
	if running {
		t.Fatal("expected offline device not to count as running")
	}
}

func TestInstallAPK_argv(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "install", "-r", "/path/to/app.apk")).Then(
		"Success\n",
		"",
		nil,
	)

	r := &androidRunner{runner: f}
	if err := r.installAPK("adb", "/path/to/app.apk"); err != nil {
		t.Fatalf("installAPK: %v", err)
	}

	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Path != "adb" {
		t.Fatalf("expected adb, got %q", calls[0].Path)
	}
}

func TestInstallAPK_failure(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "install", "-r", "bad.apk")).Then(
		"adb: failed to install bad.apk",
		"",
		errors.New("exit status 1"),
	)

	r := &androidRunner{runner: f}
	err := r.installAPK("adb", "bad.apk")
	if err == nil {
		t.Fatal("expected error for failed install")
	}
}

func TestLaunchAPK_argv(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "shell", "am", "start", "-n",
		"org.test.app/org.lurpicui.bridge.LurpicNativeActivity")).Then(
		"Starting: Intent { }\n",
		"",
		nil,
	)

	r := &androidRunner{runner: f}
	if err := r.launchAPK("adb", "org.test.app"); err != nil {
		t.Fatalf("launchAPK: %v", err)
	}

	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Path != "adb" {
		t.Fatalf("expected adb, got %q", calls[0].Path)
	}
}

func TestLaunchAPK_failure(t *testing.T) {
	f := newFakeRunner()
	f.When(MatchCommand("adb", "shell", "am", "start", "-n",
		"org.test.app/org.lurpicui.bridge.LurpicNativeActivity")).Then(
		"Error: Activity not started",
		"",
		errors.New("exit status 1"),
	)

	r := &androidRunner{runner: f}
	err := r.launchAPK("adb", "org.test.app")
	if err == nil {
		t.Fatal("expected error for failed launch")
	}
}

func TestSelectAndroidAVD_usesEnvOverride(t *testing.T) {
	r := &androidRunner{runner: newFakeRunner()}
	t.Setenv("ANDROID_AVD_NAME", "my_custom_avd")

	avd, err := r.selectAndroidAVD("emulator")
	if err != nil {
		t.Fatalf("selectAndroidAVD: %v", err)
	}
	if avd != "my_custom_avd" {
		t.Fatalf("expected 'my_custom_avd', got %q", avd)
	}
}

func TestSelectAndroidAVD_usesLurpicEnvOverride(t *testing.T) {
	r := &androidRunner{runner: newFakeRunner()}
	t.Setenv("LURPIC_ANDROID_AVD", "lurpic_custom")

	avd, err := r.selectAndroidAVD("emulator")
	if err != nil {
		t.Fatalf("selectAndroidAVD: %v", err)
	}
	if avd != "lurpic_custom" {
		t.Fatalf("expected 'lurpic_custom', got %q", avd)
	}
}

func TestFindAndroidEmulator_found(t *testing.T) {
	sdk := t.TempDir()
	emulatorDir := filepath.Join(sdk, "emulator")
	if err := os.MkdirAll(emulatorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	emulatorPath := filepath.Join(emulatorDir, "emulator")
	if err := os.WriteFile(emulatorPath, []byte("#!/bin/sh\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}

	path, err := findAndroidEmulator(sdk)
	if err != nil {
		t.Fatalf("findAndroidEmulator: %v", err)
	}
	if path != emulatorPath {
		t.Fatalf("expected %q, got %q", emulatorPath, path)
	}
}

func TestFindAndroidEmulator_notFound(t *testing.T) {
	_, err := findAndroidEmulator(t.TempDir())
	if err == nil {
		t.Fatal("expected error when emulator not found")
	}
}
