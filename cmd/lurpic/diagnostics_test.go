package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── symbols.go tests ────────────────────────────────────────────────────

func TestCollectSOFiles_findsSO(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "libgo.so"), []byte("so"), 0644)
	os.WriteFile(filepath.Join(dir, "libother.so"), []byte("so2"), 0644)
	os.WriteFile(filepath.Join(dir, "not_a_lib.txt"), []byte("txt"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "subdir", "libnested.so"), []byte("nested"), 0644)

	files, err := collectSOFiles(dir)
	if err != nil {
		t.Fatalf("collectSOFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 .so files, got %d: %v", len(files), files)
	}
}

func TestCollectSOFiles_emptyDir(t *testing.T) {
	dir := t.TempDir()
	files, err := collectSOFiles(dir)
	if err != nil {
		t.Fatalf("collectSOFiles on empty dir: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestCollectSOFiles_nonexistentDir(t *testing.T) {
	files, err := collectSOFiles("/nonexistent/path")
	if err != nil {
		t.Fatalf("collectSOFiles on nonexistent: %v", err)
	}
	if files != nil {
		t.Fatalf("expected nil, got %v", files)
	}
}

func TestFindSymbolSet_fromLibDir(t *testing.T) {
	buildDir := t.TempDir()
	libRoot := filepath.Join(buildDir, "android", "lib", "arm64-v8a")
	os.MkdirAll(libRoot, 0755)
	os.WriteFile(filepath.Join(libRoot, "libgo.so"), []byte("go"), 0644)
	os.WriteFile(filepath.Join(libRoot, "liblurpic_render.so"), []byte("render"), 0644)

	syms, err := findSymbolSet(buildDir)
	if err != nil {
		t.Fatalf("findSymbolSet: %v", err)
	}
	abis := syms.ABIs()
	if len(abis) != 1 || abis[0] != "arm64-v8a" {
		t.Fatalf("expected [arm64-v8a], got %v", abis)
	}
	if len(syms.ABIMap["arm64-v8a"]) != 2 {
		t.Fatalf("expected 2 .so files for arm64-v8a, got %d", len(syms.ABIMap["arm64-v8a"]))
	}
}

func TestFindSymbolSet_debugSymbolsOverrideLib(t *testing.T) {
	buildDir := t.TempDir()

	libRoot := filepath.Join(buildDir, "android", "lib", "arm64-v8a")
	os.MkdirAll(libRoot, 0755)
	os.WriteFile(filepath.Join(libRoot, "libgo.so"), []byte("stripped"), 0644)

	symRoot := filepath.Join(buildDir, "android", "native-debug-symbols", "arm64-v8a")
	os.MkdirAll(symRoot, 0755)
	os.WriteFile(filepath.Join(symRoot, "libgo.so"), []byte("unstripped"), 0644)

	syms, err := findSymbolSet(buildDir)
	if err != nil {
		t.Fatalf("findSymbolSet: %v", err)
	}
	// debug symbols take precedence; ABIMap should point to the debug-symbols path
	symDir := syms.symbolDirForNDKStack("arm64-v8a")
	if !strings.Contains(symDir, "native-debug-symbols") {
		t.Fatalf("expected native-debug-symbols dir, got %s", symDir)
	}
}

func TestFindSymbolSet_multipleABIs(t *testing.T) {
	buildDir := t.TempDir()
	for _, abi := range []string{"arm64-v8a", "x86_64", "armeabi-v7a"} {
		dir := filepath.Join(buildDir, "android", "lib", abi)
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "libgo.so"), []byte(abi), 0644)
	}

	syms, err := findSymbolSet(buildDir)
	if err != nil {
		t.Fatalf("findSymbolSet: %v", err)
	}
	abis := syms.ABIs()
	if len(abis) != 3 {
		t.Fatalf("expected 3 ABIs, got %d: %v", len(abis), abis)
	}
}

func TestSymbolSet_prefersDebugSymbolsOverLib(t *testing.T) {
	buildDir := t.TempDir()

	// Create lib/<abi> with a stripped-looking file.
	libRoot := filepath.Join(buildDir, "android", "lib", "arm64-v8a")
	os.MkdirAll(libRoot, 0755)
	os.WriteFile(filepath.Join(libRoot, "libgo.so"), []byte("stripped"), 0644)

	// Create native-debug-symbols/<abi> with an unstripped copy.
	symRoot := filepath.Join(buildDir, "android", "native-debug-symbols", "arm64-v8a")
	os.MkdirAll(symRoot, 0755)
	os.WriteFile(filepath.Join(symRoot, "libgo.so"), []byte("unstripped"), 0644)

	syms, err := findSymbolSet(buildDir)
	if err != nil {
		t.Fatalf("findSymbolSet: %v", err)
	}
	symDir := syms.symbolDirForNDKStack("arm64-v8a")
	if !strings.HasSuffix(symDir, "native-debug-symbols/arm64-v8a") {
		t.Fatalf("expected native-debug-symbols dir, got %s", symDir)
	}
}

func TestFindSymbolSet_noBuildDir(t *testing.T) {
	_, err := findSymbolSet("/does/not/exist")
	if err == nil {
		t.Fatal("expected error for nonexistent build dir")
	}
}

func TestFindSymbolSet_emptyBuildDir(t *testing.T) {
	buildDir := t.TempDir()
	os.MkdirAll(filepath.Join(buildDir, "android", "lib"), 0755)
	_, err := findSymbolSet(buildDir)
	if err == nil {
		t.Fatal("expected error for empty lib dir")
	}
}

// ─── diagnostics.go tests ────────────────────────────────────────────────

func TestDetectABIFromTombstone_arm64(t *testing.T) {
	content := `*** *** *** *** *** *** *** *** *** *** *** *** *** *** *** ***
Build fingerprint: 'google/emu64a/emu64a:14/...
ABI: 'arm64-v8a'
`
	abi := detectABIFromTombstone(content)
	if abi != "arm64-v8a" {
		t.Fatalf("expected arm64-v8a, got %q", abi)
	}
}

func TestDetectABIFromTombstone_x86_64(t *testing.T) {
	content := `ABI: 'x86_64'
`
	abi := detectABIFromTombstone(content)
	if abi != "x86_64" {
		t.Fatalf("expected x86_64, got %q", abi)
	}
}

func TestDetectABIFromTombstone_heuristic(t *testing.T) {
	content := `signal 11 (SIGSEGV), code 1, fault addr 0x0
    x0  0000000000000000  x1  0000007f8b440000  x2  0000000000000020
`
	abi := detectABIFromTombstone(content)
	if abi != "arm64-v8a" {
		t.Fatalf("expected arm64-v8a (heuristic), got %q", abi)
	}
}

func TestDetectABIFromTombstone_unknown(t *testing.T) {
	content := `some random log line`
	abi := detectABIFromTombstone(content)
	if abi != "" {
		t.Fatalf("expected empty, got %q", abi)
	}
}

func TestFindTombstones_findsFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "tombstone_00"), []byte("crash1"), 0644)
	os.WriteFile(filepath.Join(dir, "tombstone_01"), []byte("crash2"), 0644)
	os.WriteFile(filepath.Join(dir, "not_a_tombstone"), []byte("nope"), 0644)
	os.MkdirAll(filepath.Join(dir, "tombstone_02_dir"), 0755)

	tombstones := findTombstones(dir)
	if len(tombstones) != 2 {
		t.Fatalf("expected 2 tombstones, got %d: %v", len(tombstones), tombstones)
	}
}

func TestFindTombstones_empty(t *testing.T) {
	dir := t.TempDir()
	tombstones := findTombstones(dir)
	if len(tombstones) != 0 {
		t.Fatalf("expected 0 tombstones, got %d", len(tombstones))
	}
}

func TestFindTombstones_nonexistentDir(t *testing.T) {
	tombstones := findTombstones("/nonexistent")
	if tombstones != nil {
		t.Fatalf("expected nil, got %v", tombstones)
	}
}

func TestFindNDKStack_findsFromEnv(t *testing.T) {
	ndkDir := t.TempDir()
	ndkStack := filepath.Join(ndkDir, "ndk-stack")
	os.WriteFile(ndkStack, []byte("#!/bin/sh"), 0755)

	t.Setenv("ANDROID_NDK_HOME", ndkDir)
	path := findNDKStack("")
	if path == "" {
		t.Fatal("expected non-empty ndk-stack path")
	}
	if path != ndkStack {
		t.Fatalf("expected %s, got %s", ndkStack, path)
	}
}

func TestFindNDKStack_fromSDK(t *testing.T) {
	// Clear env vars so the test exercises the SDK-inference branch, not the
	// host environment's ANDROID_NDK_HOME.
	t.Setenv("ANDROID_NDK_HOME", "")
	t.Setenv("NDK_HOME", "")
	sdkDir := t.TempDir()
	ndkDir := filepath.Join(sdkDir, "ndk", "27.0.12077973")
	os.MkdirAll(ndkDir, 0755)
	ndkStack := filepath.Join(ndkDir, "ndk-stack")
	os.WriteFile(ndkStack, []byte("#!/bin/sh"), 0755)

	path := findNDKStack(sdkDir)
	if path == "" {
		t.Fatal("expected non-empty ndk-stack path from SDK")
	}
}

func TestFindNDKStack_notFound(t *testing.T) {
	path := findNDKStack("/nonexistent")
	if path != "" {
		t.Fatalf("expected empty for nonexistent, got %q", path)
	}
}

// ─── Integration: crash subcommand arg construction ──────────────────────

func TestCrashSubcommand_ndkStackInvocation(t *testing.T) {
	f := newFakeRunner()
	ndkStack := "/fake/ndk-stack"
	buildDir := t.TempDir()
	symDir := filepath.Join(buildDir, "android", "lib", "arm64-v8a")
	os.MkdirAll(symDir, 0755)
	os.WriteFile(filepath.Join(symDir, "libgo.so"), []byte("unstripped"), 0644)

	tombstoneDir := t.TempDir()
	tsPath := filepath.Join(tombstoneDir, "tombstone_00")
	os.WriteFile(tsPath, []byte("ABI: 'arm64-v8a'\nsignal 11"), 0644)

	// Register ndk-stack invocation mock
	f.When(MatchCommand(ndkStack, "-sym", symDir, "-dump", tsPath)).Then(
		"#00 pc 00010234  libgo.so!MyFunction+0x1234\n",
		"",
		nil,
	)

	// Verify the command construction
	out, err := f.Output(CommandSpec{
		Path: ndkStack,
		Args: []string{"-sym", symDir, "-dump", tsPath},
	})
	if err != nil {
		t.Fatalf("ndk-stack mock failed: %v", err)
	}
	if !strings.Contains(string(out), "libgo.so") {
		t.Fatalf("expected ndk-stack output containing libgo.so, got %q", string(out))
	}
}

func TestLogcatSubcommand_adbLogcatInvocation(t *testing.T) {
	f := newFakeRunner()
	adb := "/fake/adb"
	serial := "emulator-5554"

	// Verify adbArgs produces correct logcat command
	args := adbArgs(serial, "logcat", "-v", "time", "LurpicBridge:V", "*:W")
	expected := []string{"-s", "emulator-5554", "logcat", "-v", "time", "LurpicBridge:V", "*:W"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("arg[%d]: expected %q, got %q", i, expected[i], args[i])
		}
	}

	// Verify the command runs
	f.When(MatchCommand(adb)).Then("log output", "", nil)

	err := f.Run(CommandSpec{
		Path: adb,
		Args: args,
	})
	if err != nil {
		t.Fatalf("logcat mock failed: %v", err)
	}
}

func TestLogcatSubcommand_clearInvocation(t *testing.T) {
	f := newFakeRunner()
	adb := "/fake/adb"
	serial := "emulator-5554"

	args := adbArgs(serial, "logcat", "-c")
	f.When(MatchCommand(adb, args...)).Then("", "", nil)

	err := f.Run(CommandSpec{
		Path: adb,
		Args: args,
	})
	if err != nil {
		t.Fatalf("clear mock failed: %v", err)
	}
}

// ─── Smoke: arg construction for logcat ─────────────────────────────────

func TestLogcatAdbArgs_noSerial(t *testing.T) {
	args := adbArgs("", "logcat", "-v", "time")
	expected := []string{"logcat", "-v", "time"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("arg[%d]: expected %q, got %q", i, expected[i], args[i])
		}
	}
}

func TestLogcatAdbArgs_withSerial(t *testing.T) {
	args := adbArgs("emulator-5554", "logcat", "-c")
	expected := []string{"-s", "emulator-5554", "logcat", "-c"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("arg[%d]: expected %q, got %q", i, expected[i], args[i])
		}
	}
}

// ─── Crash: pull tombstone arg construction ─────────────────────────────

func TestCrashPullTombstones_adbArgs(t *testing.T) {
	f := newFakeRunner()
	adb := "/fake/adb"
	serial := "emulator-5554"
	localDir := "/tmp/tombstones"

	args := adbArgs(serial, "pull", "/data/tombstones", localDir)
	f.When(MatchCommand(adb, args...)).Then("", "", errors.New("no tombstones"))

	_, err := f.Output(CommandSpec{
		Path: adb,
		Args: args,
	})
	if err == nil {
		t.Fatal("expected error for no tombstones")
	}
}
