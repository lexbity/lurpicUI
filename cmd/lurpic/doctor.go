package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
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

	// Check Go version
	checkGo(verbose)

	// Check Rust version
	checkRust(verbose)

	// Check cargo-ndk
	checkCargoNdk(verbose)

	// Check Android toolchains
	fmt.Println()

	// Load user config if available
	userConfig, _ := loadUserConfig()

	// Create detector without flags (for doctor we want to see actual detection sources)
	detector := &ToolchainDetector{
		UserConfig: userConfig,
	}

	// Get full toolchain report
	report, _ := detector.GetToolchainReport()

	// Check SDK components in detail if SDK is found
	if report.SDK.OK && verbose {
		checkSDKComponents(report.SDK.Path)
	}

	// Print summary
	fmt.Print(report.String())

	if report.CanBuild() {
		return 0
	}
	return 1
}

func checkGo(verbose bool) {
	cmd := exec.Command("go", "version")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("✗ Go not found")
		fmt.Println("  Install: https://go.dev/dl/ or use your package manager")
		return
	}

	version := strings.TrimSpace(string(output))
	if verbose {
		fmt.Printf("✓ %s\n", version)
	} else {
		fmt.Println("✓ Go")
	}
}

func checkRust(verbose bool) {
	cmd := exec.Command("rustc", "--version")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("✗ Rust not found")
		fmt.Println("  Install: https://rustup.rs/")
		return
	}

	version := strings.TrimSpace(string(output))
	if verbose {
		fmt.Printf("✓ %s\n", version)
	} else {
		fmt.Println("✓ Rust")
	}
}

func checkCargoNdk(verbose bool) {
	cmd := exec.Command("cargo", "ndk", "--version")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("✗ cargo-ndk not found")
		fmt.Println("  Install: cargo install cargo-ndk")
		return
	}

	version := strings.TrimSpace(string(output))
	if verbose {
		fmt.Printf("✓ %s\n", version)
	} else {
		fmt.Println("✓ cargo-ndk")
	}
}

func checkSDKComponents(sdkPath string) {
	fmt.Println("  SDK Components:")

	// Check platforms
	platformsDir := sdkPath + "/platforms"
	if entries, err := os.ReadDir(platformsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "android-") {
				fmt.Printf("    ✓ platform/%s\n", entry.Name())
			}
		}
	}

	// Check build-tools
	buildToolsDir := sdkPath + "/build-tools"
	if entries, err := os.ReadDir(buildToolsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				fmt.Printf("    ✓ build-tools/%s\n", entry.Name())
			}
		}
	}

	// Check platform-tools
	adbPath := sdkPath + "/platform-tools/adb"
	if runtime.GOOS == "windows" {
		adbPath += ".exe"
	}
	if _, err := os.Stat(adbPath); err == nil {
		fmt.Println("    ✓ platform-tools (adb)")
	}
}
