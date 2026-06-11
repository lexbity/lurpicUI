// lurpic is the build tool for lurpicUI applications.
//
// It orchestrates Go cross-compile, Rust cross-compile, Java compilation,
// dex generation, asset bundling, manifest assembly, signing, and APK
// packaging into one developer-facing workflow.
//
// Usage:
//
//	lurpic build android              # produces an APK in ./build/
//	lurpic build android --release    # produces a release-signed APK
//	lurpic run android                # builds, installs, launches on connected device
//	lurpic run android --emulator     # launches an emulator if not running, then run
//	lurpic validate demos             # runs demo validation suites
//	lurpic clean                      # removes build artifacts
//	lurpic version                    # prints version information
package main

import (
	"fmt"
	"os"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case cmdBuildName:
		os.Exit(cmdBuild(os.Args[2:]))
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "validate":
		os.Exit(cmdValidate(os.Args[2:]))
	case "clean":
		os.Exit(cmdClean(os.Args[2:]))
	case "doctor":
		os.Exit(cmdDoctor(os.Args[2:]))
	case "logcat":
		os.Exit(cmdLogcat(os.Args[2:]))
	case "crash":
		os.Exit(cmdCrash(os.Args[2:]))
	case "screenshot":
		os.Exit(cmdScreenshot(os.Args[2:]))
	case "version":
		os.Exit(cmdVersion())
	case "help", "-h", "--help":
		printUsage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`lurpic - build tool for lurpicUI applications

Usage:
  lurpic build android [flags]    Build an Android APK
  lurpic run android [flags]      Build, install, and run on Android device
  lurpic doctor [platform]        Diagnose toolchain setup
  lurpic validate demos           Run shared marks and demo module validation suites
  lurpic logcat [flags]           Stream or clear device logcat
  lurpic crash [flags]            Pull tombstones and symbolicate crash dumps
  lurpic screenshot [flags]       Capture device screenshot for golden testing
  lurpic clean                    Remove build artifacts
  lurpic version                  Print version information
  lurpic help                     Show this help message

Build flags:
  --release                        Build release APK/AAB (requires keystore config)
  --aab                            Build an Android App Bundle (.aab) instead of APK
  -o, --output PATH                Output path for the APK/AAB
  --sdk-path PATH                  Android SDK path (highest priority)
  --ndk-path PATH                  Android NDK path (highest priority)
  --jdk-path PATH                  JDK path (highest priority)

Run flags:
  --emulator                       Launch on emulator (starts if not running)
  --release                        Build release APK before running

Doctor:
  lurpic doctor android            Check Android toolchain
  lurpic doctor --verbose          Show detailed information

Configuration hierarchy (highest to lowest):
  1. Command-line flags (--sdk-path, etc.)
  2. Project config (lurpic.toml [android.sdk] section)
  3. User config (~/.config/lurpic/config.toml)
  4. Environment variables (ANDROID_HOME, ANDROID_NDK_HOME, JAVA_HOME)
  5. Auto-detection (common install paths)

Environment:
  ANDROID_HOME                     Path to Android SDK
  ANDROID_NDK_HOME                 Path to Android NDK
  JAVA_HOME                        Path to JDK
  LURPIC_KEYSTORE_PASSWORD         Release keystore password
`)
}
