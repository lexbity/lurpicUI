package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func cmdDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	verbose := fs.Bool("verbose", false, "Show detailed information")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	platform := ""
	if fs.NArg() > 0 {
		platform = fs.Arg(0)
	}

	switch platform {
	case "", "android":
		return doctorAndroid(*verbose)
	default:
		fmt.Fprintf(os.Stderr, "Unknown platform: %s (supported: android)\n", platform)
		return 1
	}
}

func doctorAndroid(verbose bool) int {
	fmt.Println("Checking Android toolchain...")
	fmt.Println()

	runner := newExecRunner()

	checkGo(runner, verbose)
	checkRust(runner, verbose)
	checkCargoNdk(runner, verbose)
	checkEmulatorToolchain(runner, verbose)

	fmt.Println()

	userConfig, _ := loadUserConfig()
	detector := &ToolchainDetector{
		Runner:     runner,
		UserConfig: userConfig,
	}

	report, _ := detector.GetToolchainReport()

	if report.SDK.OK && verbose {
		checkSDKComponents(report.SDK.Path)
	}

	// Check toolchain version pins if a project config exists.
	projectRoot, err := findProjectRoot()
	if err == nil {
		if cfg, loadErr := loadConfig(projectRoot); loadErr == nil && cfg != nil {
			if warnings := checkToolchainPins(cfg, report.SDK.Path, report.NDK.Path); len(warnings) > 0 {
				fmt.Println()
				fmt.Println("Toolchain version pin warnings:")
				for _, w := range warnings {
					fmt.Printf("  ! %s\n", w)
				}
			}
		}
	}

	fmt.Print(report.String())

	if report.CanBuild() {
		return 0
	}
	return 1
}

func checkGo(runner Runner, verbose bool) {
	out, err := runner.Output(CommandSpec{Path: "go", Args: []string{"version"}})
	if err != nil {
		fmt.Println("✗ Go not found")
		fmt.Println("  Install: https://go.dev/dl/ or use your package manager")
		return
	}

	version := strings.TrimSpace(string(out))
	if verbose {
		fmt.Printf("✓ %s\n", version)
	} else {
		fmt.Println("✓ Go")
	}
}

func checkRust(runner Runner, verbose bool) {
	out, err := runner.Output(CommandSpec{Path: "rustc", Args: []string{"--version"}})
	if err != nil {
		fmt.Println("✗ Rust not found")
		fmt.Println("  Install: https://rustup.rs/")
		return
	}

	version := strings.TrimSpace(string(out))
	if verbose {
		fmt.Printf("✓ %s\n", version)
	} else {
		fmt.Println("✓ Rust")
	}
}

func checkCargoNdk(runner Runner, verbose bool) {
	out, err := runner.Output(CommandSpec{Path: "cargo", Args: []string{"ndk", "--version"}})
	if err != nil {
		fmt.Println("✗ cargo-ndk not found")
		fmt.Println("  Install: cargo install cargo-ndk")
		return
	}

	version := strings.TrimSpace(string(out))
	if verbose {
		fmt.Printf("✓ %s\n", version)
	} else {
		fmt.Println("✓ cargo-ndk")
	}
}

func checkEmulatorToolchain(runner Runner, verbose bool) {
	// Use full auto-detection (env vars + common install paths), matching how
	// the build/run commands resolve the SDK, so the emulator checks run even
	// when ANDROID_HOME is unset.
	sdk, err := detectAndroidSDK()
	if err != nil {
		if verbose {
			fmt.Printf("✗ Android SDK not found; skipping emulator checks: %v\n", err)
		}
		return
	}

	fmt.Println()
	fmt.Println("Emulator toolchain:")

	// Check emulator binary
	emuTool, err := findEmulatorTool(sdk)
	if err == nil {
		fmt.Println("  ✓ emulator binary")
		if verbose {
			fmt.Printf("    Path: %s\n", emuTool)
		}
	} else {
		fmt.Println("  ✗ emulator binary not found")
		fmt.Println("    Install via SDK Manager: sdkmanager \"emulator\"")
	}

	// Check sdkmanager
	_, err = findCmdlineTool(sdk, "sdkmanager")
	if err == nil {
		fmt.Println("  ✓ sdkmanager")
	} else {
		fmt.Println("  ✗ sdkmanager not found")
	}

	// Check avdmanager
	_, err = findCmdlineTool(sdk, "avdmanager")
	if err == nil {
		fmt.Println("  ✓ avdmanager")
	} else {
		fmt.Println("  ✗ avdmanager not found")
	}

	// Check for x86_64 google_apis system image
	sysImage := fmt.Sprintf("system-images;android-33;google_apis;x86_64")
	_ = sysImage
	if verbose {
		imageFound := checkSystemImage(sdk, 33, "x86_64")
		if imageFound {
			fmt.Printf("  ✓ system image: android-33 google_apis x86_64\n")
		} else {
			fmt.Printf("  ✗ system image not found: system-images;android-33;google_apis;x86_64\n")
		}
	}

	// Check managed AVD
	avdDir := fmt.Sprintf("lurpic_api33_google_apis_x86_64")
	_ = avdDir
	if verbose {
		avdFound := checkManagedAVD("lurpic_api33_google_apis_x86_64")
		if avdFound {
			fmt.Println("  ✓ managed AVD: lurpic_api33_google_apis_x86_64")
		} else {
			fmt.Println("  ✗ managed AVD not found (will be created on first run)")
		}
	}
}

func checkSystemImage(sdk string, api int, abi string) bool {
	imageDir := filepath.Join(sdk, "system-images", fmt.Sprintf("android-%d", api), "google_apis", abi)
	if _, err := os.Stat(imageDir); err == nil {
		return true
	}
	return false
}

func checkManagedAVD(name string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	avdDir := filepath.Join(home, ".android", "avd", name+".avd")
	if _, err := os.Stat(avdDir); err == nil {
		return true
	}
	return false
}

func checkSDKComponents(sdkPath string) {
	fmt.Println("  SDK Components:")

	platformsDir := sdkPath + "/platforms"
	if entries, err := os.ReadDir(platformsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "android-") {
				fmt.Printf("    ✓ platform/%s\n", entry.Name())
			}
		}
	}

	buildToolsDir := sdkPath + "/build-tools"
	if entries, err := os.ReadDir(buildToolsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				fmt.Printf("    ✓ build-tools/%s\n", entry.Name())
			}
		}
	}

	adbPath := sdkPath + "/platform-tools/adb"
	if runtime.GOOS == "windows" {
		adbPath += ".exe"
	}
	if _, err := os.Stat(adbPath); err == nil {
		fmt.Println("    ✓ platform-tools (adb)")
	}
}
