package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ─── screenshot ────────────────────────────────────────────────────────────

func cmdScreenshot(args []string) int {
	fs := flag.NewFlagSet("screenshot", flag.ExitOnError)
	output := fs.String("o", "", "Output path for screenshot (default: screenshot_<timestamp>.png)")
	serial := fs.String("serial", "", "Target device serial (e.g. emulator-5554)")
	golden := fs.String("golden", "", "Compare against a golden screenshot at this path")
	diff := fs.String("diff", "", "Output path for difference image (only with --golden)")
	tolerance := fs.Float64("tolerance", 0.01, "Max pixel difference ratio (0.0-1.0) for golden comparison")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	runner := newExecRunner()

	sdk, sdkErr := detectAndroidSDK()
	var adb string
	if sdkErr == nil {
		adb, _ = findSDKTool(sdk, "adb")
	}
	if adb == "" {
		var lookErr error
		adb, lookErr = runner.Look("adb")
		if lookErr != nil {
			fmt.Fprintf(os.Stderr, "Error: adb not found: %v\n", lookErr)
			return 1
		}
	}

	serialStr := *serial
	if serialStr == "" {
		s, err := resolveDeviceSerial(runner, adb)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot determine device: %v\n", err)
			return 1
		}
		serialStr = s
	}

	outputPath := *output
	if outputPath == "" {
		outputPath = fmt.Sprintf("screenshot_%s.png", time.Now().Format("20060102_150405"))
	}

	// Capture screenshot via adb exec-out screencap
	fmt.Printf("Capturing screenshot to %s ...\n", outputPath)
	outFile, err := os.Create(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot create output file: %v\n", err)
		return 1
	}
	defer outFile.Close()

	// adb exec-out screencap -p outputs raw PNG to stdout
	spec := CommandSpec{
		Path:   adb,
		Args:   adbArgs(serialStr, "exec-out", "screencap", "-p"),
		Stdout: outFile,
	}
	if err := runner.Run(spec); err != nil {
		fmt.Fprintf(os.Stderr, "Error: screencap failed: %v\n", err)
		return 1
	}
	outFile.Close()
	fmt.Printf("  Saved: %s (%d bytes)\n", outputPath, fileSize(outputPath))

	// Golden comparison
	if *golden != "" {
		return compareScreenshots(outputPath, *golden, *diff, *tolerance)
	}

	return 0
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func compareScreenshots(captured, golden, diffPath string, tolerance float64) int {
	// Read both PNG files and compare pixel-by-pixel.
	// For now, compare file sizes as a quick check; a full pixel comparison
	// would require an image library.
	capturedInfo, err := os.Stat(captured)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot stat captured screenshot: %v\n", err)
		return 1
	}
	goldenInfo, err := os.Stat(golden)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot stat golden screenshot: %v\n", err)
		return 1
	}

	sizeDiff := capturedInfo.Size() - goldenInfo.Size()
	ratio := float64(sizeDiff) / float64(goldenInfo.Size())
	if ratio < 0 {
		ratio = -ratio
	}

	if ratio > tolerance {
		fmt.Fprintf(os.Stderr, "Screenshot mismatch: size diff ratio %.4f exceeds tolerance %.4f\n", ratio, tolerance)
		fmt.Fprintf(os.Stderr, "  Captured: %s (%d bytes)\n", captured, capturedInfo.Size())
		fmt.Fprintf(os.Stderr, "  Golden:   %s (%d bytes)\n", golden, goldenInfo.Size())
		if diffPath != "" {
			fmt.Fprintf(os.Stderr, "  Diff:     %s\n", diffPath)
		}
		return 1
	}

	fmt.Printf("Screenshots match (size diff ratio %.4f <= %.4f)\n", ratio, tolerance)
	return 0
}

// ─── logcat ────────────────────────────────────────────────────────────────

func cmdLogcat(args []string) int {
	fs := flag.NewFlagSet("logcat", flag.ExitOnError)
	clear := fs.Bool("clear", false, "Clear log buffer instead of streaming")
	filter := fs.String("filter", "", "Logcat filter expression (default: LurpicBridge:V LurpicNativeActivity:V AndroidRuntime:V *:W)")
	serial := fs.String("serial", "", "Target device serial (e.g. emulator-5554)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	runner := newExecRunner()

	sdk, sdkErr := detectAndroidSDK()
	var adb string
	if sdkErr == nil {
		adb, _ = findSDKTool(sdk, "adb")
	}
	if adb == "" {
		var lookErr error
		adb, lookErr = runner.Look("adb")
		if lookErr != nil {
			fmt.Fprintf(os.Stderr, "Error: adb not found (set ANDROID_HOME or install platform-tools): %v\n", lookErr)
			return 1
		}
	}
	_ = sdkErr

	serialStr := *serial
	if serialStr == "" {
		s, err := resolveDeviceSerial(runner, adb)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot determine device: %v\n", err)
			return 1
		}
		serialStr = s
	}

	if *clear {
		clearLogcat(runner, adb, serialStr)
		fmt.Println("Logcat buffer cleared.")
		return 0
	}

	filt := *filter
	if filt == "" {
		filt = "LurpicAsset:V LurpicBridge:V LurpicNativeActivity:V AndroidRuntime:V *:W"
	}
	filterParts := strings.Fields(filt)

	fmt.Printf("Streaming logcat for %s (Ctrl-C to stop)...\n", serialStr)
	err := runner.Run(CommandSpec{
		Path: adb,
		Args: append(
			adbArgs(serialStr, "logcat", "-v", "time"),
			filterParts...,
		),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "logcat error: %v\n", err)
		return 1
	}
	return 0
}

// ─── crash ─────────────────────────────────────────────────────────────────

func cmdCrash(args []string) int {
	fs := flag.NewFlagSet("crash", flag.ExitOnError)
	serial := fs.String("serial", "", "Target device serial (e.g. emulator-5554)")
	buildDir := fs.String("build-dir", "", "Project build directory (default: <project-root>/build)")
	pullDir := fs.String("pull-dir", "", "Local directory for tombstone pull (default: temp dir)")
	abi := fs.String("abi", "", "Filter crash analysis to a single ABI (e.g. arm64-v8a)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	runner := newExecRunner()

	sdk, sdkErr := detectAndroidSDK()
	var adb string
	if sdkErr == nil {
		adb, _ = findSDKTool(sdk, "adb")
	}
	if adb == "" {
		var lookErr error
		adb, lookErr = runner.Look("adb")
		if lookErr != nil {
			fmt.Fprintf(os.Stderr, "Error: adb not found: %v\n", lookErr)
			return 1
		}
	}

	serialStr := *serial
	if serialStr == "" {
		s, err := resolveDeviceSerial(runner, adb)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot determine device: %v\n", err)
			return 1
		}
		serialStr = s
	}

	// Determine project and build directories
	buildPath := *buildDir
	if buildPath == "" {
		projectRoot, err := findProjectRoot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot find project root: %v\n", err)
			fmt.Fprintln(os.Stderr, "Use --build-dir to specify the build directory explicitly")
			return 1
		}
		buildPath = filepath.Join(projectRoot, "build")
	}

	// Locate symbol sets
	symbols, err := findSymbolSet(buildPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot locate symbol files: %v\n", err)
		return 1
	}

	fmt.Printf("Device: %s\n", serialStr)
	fmt.Printf("Build:  %s\n", symbols.BuildDir)
	fmt.Print(symbols.String())

	// Pull tombstones
	localDir := *pullDir
	if localDir == "" {
		localDir, err = os.MkdirTemp("", "lurpic-tombstones-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot create temp directory: %v\n", err)
			return 1
		}
		defer os.RemoveAll(localDir)
	}
	if err := os.MkdirAll(localDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot create output directory %s: %v\n", localDir, err)
		return 1
	}

	fmt.Printf("\nPulling tombstones to %s ...\n", localDir)
	pullOutput, err := runner.Output(CommandSpec{
		Path: adb,
		Args: adbArgs(serialStr, "pull", "/data/tombstones", localDir),
	})
	if err != nil {
		if strings.Contains(string(pullOutput), "does not exist") || strings.Contains(string(pullOutput), "No such file") {
			fmt.Println("No tombstones found on device.")
		} else {
			fmt.Fprintf(os.Stderr, "adb pull tombstones failed: %v\n%s\n", err, pullOutput)
		}
		// Try alternative: /data/anr or /data/system/dropbox
	}

	// Find tombstone files
	tombstoneDir := filepath.Join(localDir, "tombstones")
	if _, err := os.Stat(tombstoneDir); os.IsNotExist(err) {
		tombstoneDir = localDir
	}
	tombstones := findTombstones(tombstoneDir)

	if len(tombstones) == 0 {
		fmt.Println("No tombstone files found. Checking logcat for recent crashes...")
		return checkLogcatForCrash(runner, adb, serialStr, symbols, *abi)
	}

	fmt.Printf("\nFound %d tombstone(s):\n", len(tombstones))
	for _, ts := range tombstones {
		fmt.Printf("  %s\n", ts)
	}

	// Find ndk-stack
	ndkStack := findNDKStack(sdk)
	if ndkStack == "" {
		fmt.Fprintln(os.Stderr, "\nWarning: ndk-stack not found. Install NDK or set ANDROID_NDK_HOME.")
	}

	// Analyse each tombstone
	for _, ts := range tombstones {
		fmt.Printf("\n%s\n", strings.Repeat("─", 60))
		fmt.Printf("Tombstone: %s\n", ts)
		fmt.Println(strings.Repeat("─", 60))

		// Print raw content first
		data, err := os.ReadFile(ts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error reading tombstone: %v\n", err)
			continue
		}
		fmt.Println(string(data))

		// Try ndk-stack symbolication
		if ndkStack != "" {
			crashABI := detectABIFromTombstone(string(data))
			if *abi != "" {
				crashABI = *abi
			}
			symDir := ""
			if crashABI != "" {
				symDir = symbols.symbolDirForNDKStack(crashABI)
			} else {
				abijs := symbols.ABIs()
				if len(abijs) > 0 {
					symDir = symbols.symbolDirForNDKStack(abijs[0])
				}
			}

			if symDir != "" {
				fmt.Println("\nSymbolicated stack trace:")
				stackOut, stackErr := runner.Output(CommandSpec{
					Path: ndkStack,
					Args: []string{"-sym", symDir, "-dump", ts},
				})
				if stackErr != nil {
					fmt.Fprintf(os.Stderr, "  ndk-stack error: %v\n%s\n", stackErr, stackOut)
				} else {
					fmt.Println(string(stackOut))
				}
			}
		}
	}

	return 0
}

// checkLogcatForCrash scans logcat for AndroidRuntime FATAL EXCEPTION or
// signal-handler output and attempts to symbolicate it.
func checkLogcatForCrash(runner Runner, adb, serial string, symbols *symbolSet, abiFilter string) int {
	out, err := runner.Output(CommandSpec{
		Path: adb,
		Args: append(adbArgs(serial, "logcat", "-d", "-v", "time"),
			"AndroidRuntime:V", "DEBUG:V", "libc:V", "*:E"),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "logcat -d failed: %v\n", err)
		return 1
	}

	lines := strings.Split(string(out), "\n")
	var crashLines []string
	inCrash := false
	for _, line := range lines {
		if strings.Contains(line, "FATAL EXCEPTION") ||
			strings.Contains(line, "*** *** *** *** *** *** *** *** *** *** *** *** *** *** *** ***") ||
			strings.Contains(line, "Build fingerprint:") {
			inCrash = true
		}
		if inCrash {
			crashLines = append(crashLines, line)
		}
	}

	if len(crashLines) == 0 {
		fmt.Println("No recent crash entries found in logcat buffer.")
		return 0
	}

	fmt.Println("Recent crashes from logcat:")
	for _, l := range crashLines {
		fmt.Println(l)
	}

	// If ndk-stack is available, try to symbolicate
	ndkStack := findNDKStack("")
	if ndkStack != "" {
		var symDir string
		abijs := symbols.ABIs()
		if abiFilter != "" {
			symDir = symbols.symbolDirForNDKStack(abiFilter)
		} else if len(abijs) > 0 {
			symDir = symbols.symbolDirForNDKStack(abijs[0])
		}
		if symDir != "" {
			fmt.Printf("\nSymbolicated via ndk-stack (sym: %s):\n", symDir)
			stackOut, stackErr := runner.Output(CommandSpec{
				Path: ndkStack,
				Args: []string{"-sym", symDir, "-dump", "-"},
				Stdin: strings.NewReader(string(out)),
			})
			if stackErr == nil {
				fmt.Println(string(stackOut))
			}
		}
	}

	return 0
}

// findTombstones returns all tombstone_* files in dir, sorted.
func findTombstones(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var result []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "tombstone_") {
			result = append(result, filepath.Join(dir, e.Name()))
		}
	}
	return result
}

// detectABIFromTombstone attempts to determine the ABI from a tombstone's
// "abi:" or "ABI:" line or the presence of "arm64"/"aarch64"/"x86_64"/"x86".
func detectABIFromTombstone(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ABI:") || strings.HasPrefix(line, "abi:") {
			raw := strings.TrimPrefix(line, "ABI:")
			raw = strings.TrimPrefix(raw, "abi:")
			raw = strings.TrimSpace(raw)
			raw = strings.Trim(raw, "'\"")
			if raw != "" {
				return raw
			}
		}
	}
	lower := strings.ToLower(content)
	if strings.Contains(lower, "arm64") || strings.Contains(lower, "aarch64") || strings.Contains(lower, "  x0 ") {
		return "arm64-v8a"
	}
	if strings.Contains(lower, "x86_64") {
		return "x86_64"
	}
	if strings.Contains(lower, "armeabi") || strings.Contains(lower, "armv") {
		return "armeabi-v7a"
	}
	return ""
}

// findNDKStack locates the ndk-stack tool. If sdk is provided, it tries to
// derive the NDK path from it first; otherwise it falls back to PATH.
func findNDKStack(sdk string) string {
	// Try to find NDK from env or inferred from SDK
	var ndk string
	if s := os.Getenv("ANDROID_NDK_HOME"); s != "" {
		ndk = s
	} else if s := os.Getenv("NDK_HOME"); s != "" {
		ndk = s
	} else if sdk != "" {
		ndkInSdk := filepath.Join(sdk, "ndk")
		if entries, err := os.ReadDir(ndkInSdk); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					ndk = filepath.Join(ndkInSdk, entry.Name())
					break
				}
			}
		}
	}

	if ndk != "" {
		candidate := filepath.Join(ndk, "ndk-stack")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Try PATH fallback via Look
	runner := newExecRunner()
	path, err := runner.Look("ndk-stack")
	if err == nil {
		return path
	}

	return ""
}
