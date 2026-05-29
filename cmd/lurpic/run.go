package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultAndroidAPIVersion    = "33"
	defaultAndroidAVDDevice     = "pixel_6"
	defaultAndroidAVDTarget     = "google_apis"
	defaultAndroidAVDArchX86_64 = "x86_64"
	defaultAndroidAVDArchArm64  = "arm64-v8a"
)

type runFlags struct {
	emulator     bool
	release      bool
	bootTimeout  time.Duration
	deviceSerial string
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	var flags runFlags
	fs.BoolVar(&flags.emulator, "emulator", false, "Launch on emulator")
	fs.BoolVar(&flags.release, "release", false, "Build release APK")
	fs.DurationVar(&flags.bootTimeout, "boot-timeout", 5*time.Minute, "Emulator boot timeout (e.g. 10m, 300s)")
	fs.StringVar(&flags.deviceSerial, "device", "", "Target device serial (e.g. emulator-5554)")

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
	if platform != "android" {
		fmt.Fprintf(os.Stderr, "Error: unsupported platform '%s' (only 'android' supported)\n", platform)
		return 1
	}

	return runAndroid(flags)
}

func runAndroid(flags runFlags) int {
	builder, err := prepareAndroidBuild(buildFlags{release: flags.release})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if err := builder.build(); err != nil {
		fmt.Fprintf(os.Stderr, "Build failed: %v\n", err)
		return 1
	}

	runner := &androidRunner{
		runner:       newExecRunner(),
		emulator:     flags.emulator,
		sdk:          builder.sdk,
		apkPath:      builder.outputPath,
		packageName:  builder.config.App.ID,
		bootTimeout:  flags.bootTimeout,
		deviceSerial: flags.deviceSerial,
	}
	if err := runner.run(); err != nil {
		fmt.Fprintf(os.Stderr, "Run failed: %v\n", err)
		return 1
	}

	return 0
}

type androidRunner struct {
	runner       Runner
	emulator     bool
	sdk          string
	apkPath      string
	packageName  string
	bootTimeout  time.Duration
	deviceSerial string
}

// adbArgs prepends -s <serial> to adb arguments when a serial is known.
func adbArgs(serial string, args ...string) []string {
	if serial != "" {
		return append([]string{"-s", serial}, args...)
	}
	return args
}

func (r *androidRunner) run() error {
	adb, err := findSDKTool(r.sdk, "adb")
	if err != nil {
		return fmt.Errorf("adb not found: %w", err)
	}

	serial := r.deviceSerial

	if serial == "" && r.emulator {
		serial, err = r.resolveEmulatorSerial(adb)
		if err != nil {
			return err
		}
	}

	if serial == "" && !r.emulator {
		serial, err = r.resolveDeviceSerial(adb)
		if err != nil {
			return err
		}
	}

	if serial == "" {
		return fmt.Errorf("no target device or emulator found; use --device <serial> or --emulator")
	}

	if err := r.installAPK(adb, serial, r.apkPath); err != nil {
		return err
	}
	return r.launchAPK(adb, serial, r.packageName)
}

// resolveEmulatorSerial finds or spawns an emulator and returns its serial.
func (r *androidRunner) resolveEmulatorSerial(adb string) (string, error) {
	serial, count, err := r.listRunningEmulators(adb)
	if err != nil {
		return "", err
	}
	if count > 1 {
		return "", fmt.Errorf("multiple emulators running; use --device <serial> to select one")
	}
	if count == 1 {
		return serial, nil
	}

	// No emulator running — spawn one
	if err := r.launchEmulator(); err != nil {
		return "", err
	}
	serial = "emulator-5554"
	mgr := &EmulatorManager{runner: r.runner}
	ctx, cancel := context.WithTimeout(context.Background(), r.bootTimeout)
	defer cancel()
	if err := mgr.waitForBoot(ctx, adb, serial); err != nil {
		return "", err
	}
	return serial, nil
}

// resolveDeviceSerial finds a single connected physical device (non-emulator).
func (r *androidRunner) resolveDeviceSerial(adb string) (string, error) {
	output, err := r.runner.Output(CommandSpec{
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

func (r *androidRunner) launchEmulator() error {
	emulator, err := findAndroidEmulator(r.sdk)
	if err != nil {
		return err
	}

	avd, err := r.selectAndroidAVD(emulator)
	if err != nil {
		return err
	}

	fmt.Printf("Launching emulator %q...\n", avd)
	_, err = r.runner.Start(CommandSpec{
		Path:   emulator,
		Args:   []string{"-avd", avd, "-no-snapshot-save", "-no-boot-anim"},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		return fmt.Errorf("start emulator: %w", err)
	}
	return nil
}

func (r *androidRunner) listRunningEmulators(adb string) (serial string, count int, err error) {
	output, err := r.runner.Output(CommandSpec{
		Path: adb,
		Args: []string{"devices"},
	})
	if err != nil {
		return "", 0, fmt.Errorf("adb devices failed: %w\n%s", err, output)
	}

	var serials []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "emulator-") && strings.Contains(line, "\tdevice") {
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) > 0 {
				serials = append(serials, strings.TrimSpace(parts[0]))
			}
		}
	}
	if len(serials) == 0 {
		return "", 0, nil
	}
	return serials[0], len(serials), nil
}

func (r *androidRunner) installAPK(adb, serial, apkPath string) error {
	fmt.Printf("Installing APK: %s\n", apkPath)
	output, err := r.runner.Output(CommandSpec{
		Path: adb,
		Args: adbArgs(serial, "install", "-r", apkPath),
	})
	if err != nil {
		return fmt.Errorf("adb install failed: %w\n%s", err, output)
	}
	return nil
}

func (r *androidRunner) launchAPK(adb, serial, packageName string) error {
	component := fmt.Sprintf("%s/org.lurpicui.bridge.LurpicNativeActivity", packageName)
	fmt.Printf("Launching app: %s\n", component)
	output, err := r.runner.Output(CommandSpec{
		Path: adb,
		Args: adbArgs(serial, "shell", "am", "start", "-n", component),
	})
	if err != nil {
		return fmt.Errorf("adb shell am start failed: %w\n%s", err, output)
	}
	return nil
}

func findAndroidEmulator(sdk string) (string, error) {
	candidates := []string{
		filepath.Join(sdk, "emulator", "emulator"),
	}
	if runtime.GOOS == "windows" {
		candidates[0] += ".exe"
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("emulator binary not found in Android SDK")
}

func (r *androidRunner) selectAndroidAVD(emulator string) (string, error) {
	if avd := os.Getenv("ANDROID_AVD_NAME"); avd != "" {
		return avd, nil
	}
	if avd := os.Getenv("LURPIC_ANDROID_AVD"); avd != "" {
		return avd, nil
	}

	return r.createDefaultAndroidAVD()
}

func (r *androidRunner) createDefaultAndroidAVD() (string, error) {
	avdmanager, err := findSDKTool(r.sdk, "avdmanager")
	if err != nil {
		return "", fmt.Errorf("avdmanager not found: %w", err)
	}

	sdkmanager, err := findSDKTool(r.sdk, "sdkmanager")
	if err != nil {
		return "", fmt.Errorf("sdkmanager not found: %w", err)
	}

	systemImage := defaultAndroidSystemImage()
	if err := r.ensureAndroidPackage(sdkmanager, systemImage); err != nil {
		return "", err
	}

	avdName := defaultAndroidAVDName()
	fmt.Printf("Creating default Android Virtual Device %q...\n", avdName)
	if err := r.runner.Run(CommandSpec{
		Path:   avdmanager,
		Args:   []string{"create", "avd", "-n", avdName, "-k", systemImage, "-d", defaultAndroidAVDDevice, "--force"},
		Stdin:  strings.NewReader("no\n"),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return "", fmt.Errorf("create avd failed: %w", err)
	}
	return avdName, nil
}

func (r *androidRunner) ensureAndroidPackage(sdkmanager string, pkg string) error {
	if err := r.runner.Run(CommandSpec{
		Path:   sdkmanager,
		Args:   []string{pkg},
		Stdin:  strings.NewReader("y\n"),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("sdkmanager install %s failed: %w", pkg, err)
	}
	return nil
}

func defaultAndroidSystemImage() string {
	arch := defaultAndroidAVDArchX86_64
	if runtime.GOARCH == "arm64" {
		arch = defaultAndroidAVDArchArm64
	}
	return fmt.Sprintf("system-images;android-%s;%s;%s", defaultAndroidAPIVersion, defaultAndroidAVDTarget, arch)
}

func defaultAndroidAVDName() string {
	return fmt.Sprintf("lurpic_api%s_%s_%s", defaultAndroidAPIVersion, defaultAndroidAVDTarget, defaultAndroidAVDArch())
}

func defaultAndroidAVDArch() string {
	if runtime.GOARCH == "arm64" {
		return defaultAndroidAVDArchArm64
	}
	return defaultAndroidAVDArchX86_64
}
