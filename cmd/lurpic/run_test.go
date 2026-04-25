package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAndroidRunner_run_onRunningEmulator(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a shell script adb stub")
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
		t.Fatalf("write adb stub: %v", err)
	}

	apkPath := filepath.Join(t.TempDir(), "app.apk")
	if err := os.WriteFile(apkPath, []byte("fake apk"), 0o644); err != nil {
		t.Fatalf("write apk stub: %v", err)
	}

	runner := &androidRunner{
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
		t.Skip("test uses shell script tool stubs")
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

	avd, err := selectAndroidAVD(filepath.Join(emulatorDir, "emulator"), sdk)
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
