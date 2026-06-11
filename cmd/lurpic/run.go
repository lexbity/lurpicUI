package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

type runFlags struct {
	emulator      bool
	release       bool
	bootTimeout   time.Duration
	deviceSerial  string
	avdName       string
	abi           string
	gpuMode       string
	forceSoftware bool
	project       string
	noLogcat      bool
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	var flags runFlags
	fs.BoolVar(&flags.emulator, "emulator", false, "Launch on emulator (starts if not running)")
	fs.BoolVar(&flags.release, "release", false, "Build release APK before running")
	fs.DurationVar(&flags.bootTimeout, "boot-timeout", 5*time.Minute, "Emulator boot timeout")
	fs.StringVar(&flags.deviceSerial, "device", "", "Target device serial (e.g. emulator-5554)")
	fs.StringVar(&flags.avdName, "avd", "", "AVD name (emulator only)")
	fs.StringVar(&flags.abi, "abi", "", "Target ABI (e.g. x86_64, arm64-v8a; emulator defaults to x86_64)")
	fs.StringVar(&flags.gpuMode, "gpu", "auto", "Emulator GPU mode")
	fs.BoolVar(&flags.forceSoftware, "force-software", false, "Force software renderer")
	fs.StringVar(&flags.project, "project", "", "Project directory containing lurpic.toml (default: search upward from cwd)")
	fs.BoolVar(&flags.noLogcat, "no-logcat", false, "Do not stream logcat after launch")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: platform required (e.g., 'android')")
		fmt.Fprintln(os.Stderr, "Usage: lurpic run android [flags]")
		return 1
	}

	platform := fs.Arg(0)
	if platform != platformAndroid {
		fmt.Fprintf(os.Stderr, "Error: unsupported platform '%s' (only 'android' supported)\n", platform)
		return 1
	}

	// Parse any flags that appeared after the platform token (the standard flag
	// parser stops at the first non-flag argument), so flag order is free.
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	return runAndroid(flags)
}

func runAndroid(flags runFlags) int {
	buildAbi := flags.abi
	if flags.emulator && buildAbi == "" {
		buildAbi = archX8664
	}

	builder, err := prepareAndroidBuild(buildFlags{release: flags.release, abi: buildAbi, project: flags.project})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if err := builder.build(); err != nil {
		fmt.Fprintf(os.Stderr, "Build failed: %v\n", err)
		return 1
	}

	execRunner := newExecRunner()
	adb, err := findSDKTool(builder.sdk, "adb")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	serial := flags.deviceSerial

	if serial == "" && flags.emulator {
		sess, err := resolveEmulator(execRunner, builder, flags)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		serial = sess.Serial
		// The spawned emulator is intentionally left running so the developer
		// can watch the app; it is reused on the next run.
		if sess.spawned {
			fmt.Printf("Emulator %s left running (reused on the next run).\n", serial)
		}
	}

	if serial == "" && !flags.emulator {
		serial, err = resolveDeviceSerial(execRunner, adb)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	}

	if serial == "" {
		fmt.Fprintln(os.Stderr, "Error: no target device found; use --device <serial> or --emulator")
		return 1
	}

	if err := installAPK(execRunner, adb, serial, builder.outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Clear the log buffer so the stream starts from this launch.
	clearLogcat(execRunner, adb, serial)

	if err := launchAPK(execRunner, adb, serial, builder.config.App.ID, flags.forceSoftware); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if flags.noLogcat {
		return 0
	}

	// Stream logcat so the developer sees app output (and any crash). This
	// blocks until interrupted (Ctrl-C); the emulator keeps running afterward.
	fmt.Printf("\nStreaming logcat for %s (Ctrl-C to stop)...\n", builder.config.App.ID)
	streamLogcat(execRunner, adb, serial)
	return 0
}

// resolveEmulator provisions, boots an emulator and returns an EmulatorSession.
func resolveEmulator(runner Runner, builder *androidBuilder, flags runFlags) (*EmulatorSession, error) {
	arch := DefaultEmulatorArchitecture()
	if flags.abi != "" {
		a, ok := ArchitectureByABI(flags.abi)
		if !ok {
			return nil, fmt.Errorf("unsupported ABI: %s", flags.abi)
		}
		arch = a
	}

	// Pass --avd flag as env var so EmulatorManager.resolveAVD picks it up
	if flags.avdName != "" {
		if err := os.Setenv("LURPIC_ANDROID_AVD", flags.avdName); err != nil {
			return nil, err
		}
	}

	mgr := NewEmulatorManager(runner, builder.sdk, builder.apiLevel, arch, flags.gpuMode, false)
	ctx, cancel := context.WithTimeout(context.Background(), flags.bootTimeout)
	defer cancel()

	sess, err := mgr.EnsureRunning(ctx)
	if err != nil {
		return nil, fmt.Errorf("emulator: %w", err)
	}
	return sess, nil
}

// resolveDeviceSerial finds a single connected physical device.
func resolveDeviceSerial(runner Runner, adb string) (string, error) {
	output, err := runner.Output(CommandSpec{
		Path: adb,
		Args: []string{"devices"},
	})
	if err != nil {
		return "", fmt.Errorf("adb devices failed: %w\n%s", err, output)
	}

	var candidates []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of") || strings.HasPrefix(line, "* ") {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 && parts[1] == "device" && !strings.HasPrefix(parts[0], "emulator-") {
			candidates = append(candidates, parts[0])
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no connected devices found; connect a device via USB or use --emulator")
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("multiple devices connected (%s); use --device <serial> to select one", strings.Join(candidates, ", "))
	}
	return candidates[0], nil
}

// adbArgs prepends -s <serial> to adb arguments when serial is known.
func adbArgs(serial string, args ...string) []string {
	if serial != "" {
		return append([]string{"-s", serial}, args...)
	}
	return args
}

func installAPK(runner Runner, adb, serial, apkPath string) error {
	fmt.Printf("Installing APK: %s\n", apkPath)
	output, err := runner.Output(CommandSpec{
		Path: adb,
		Args: adbArgs(serial, "install", "-r", apkPath),
	})
	if err != nil {
		return fmt.Errorf("adb install failed: %w\n%s", err, output)
	}
	return nil
}

// clearLogcat empties the device log buffer so the subsequent stream starts
// fresh. Failures are non-fatal.
func clearLogcat(runner Runner, adb, serial string) {
	_, _ = runner.Output(CommandSpec{
		Path: adb,
		Args: adbArgs(serial, "logcat", "-c"),
	})
}

// streamLogcat streams the device log to stdout until interrupted (Ctrl-C). The
// filter shows the framework's own tags verbosely plus AndroidRuntime crashes,
// and everything else at warning level (which still surfaces native crashes and
// linker/activity-manager errors).
func streamLogcat(runner Runner, adb, serial string) {
	_ = runner.Run(CommandSpec{
		Path: adb,
		Args: adbArgs(serial, "logcat", "-v", "time",
			"LurpicAsset:V", "LurpicBridge:V", "LurpicNativeActivity:V", "AndroidRuntime:V", "*:W"),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
}

func launchAPK(runner Runner, adb, serial, packageName string, forceSoftware bool) error {
	component := fmt.Sprintf("%s/org.lurpicui.bridge.LurpicNativeActivity", packageName)
	fmt.Printf("Launching app: %s\n", component)

	env := os.Environ()
	if forceSoftware {
		env = append(env, "LURPIC_RENDER_BACKEND=software")
	}

	output, err := runner.Output(CommandSpec{
		Path: adb,
		Args: adbArgs(serial, "shell", "am", "start", "-n", component),
		Env:  env,
	})
	if err != nil {
		return fmt.Errorf("adb shell am start failed: %w\n%s", err, output)
	}
	return nil
}
