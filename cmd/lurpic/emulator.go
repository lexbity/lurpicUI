package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultEmulatorPort     = 5554
	defaultBootTimeout      = 5 * time.Minute
	defaultBootPollInterval = 3 * time.Second

	// documentedAVDStdin is the stdin sent to avdmanager create avd to decline
	// the custom hardware profile prompt.
	documentedAVDStdin = "no\n"

	// documentedLicenseStdin is the stdin sent to sdkmanager --licenses to accept
	// all license agreements.
	documentedLicenseStdin = "y\n"

	// documentedPackageStdin is the stdin sent to sdkmanager install to confirm
	// package download and installation.
	documentedPackageStdin = "y\n"
)

type EmulatorManager struct {
	runner           Runner
	sdk              string
	apiLevel         int
	arch             Architecture
	gpuMode          string
	headless         bool
	bootPollInterval time.Duration
}

type EmulatorSession struct {
	Serial  string
	proc    ProcessHandle
	spawned bool
}

func (s *EmulatorSession) Close() error {
	if s == nil || !s.spawned || s.proc == nil {
		return nil
	}
	if err := s.proc.Kill(); err != nil {
		return fmt.Errorf("kill emulator: %w", err)
	}
	_ = s.proc.Wait()
	return nil
}

func NewEmulatorManager(runner Runner, sdk string, apiLevel int, arch Architecture, gpuMode string, headless bool) *EmulatorManager {
	gpu := gpuMode
	if gpu == "" {
		gpu = "auto"
	}
	return &EmulatorManager{
		runner:           runner,
		sdk:              sdk,
		apiLevel:         apiLevel,
		arch:             arch,
		gpuMode:          gpu,
		headless:         headless,
		bootPollInterval: defaultBootPollInterval,
	}
}

func (m *EmulatorManager) EnsureRunning(ctx context.Context) (*EmulatorSession, error) {
	adb, err := findSDKTool(m.sdk, "adb")
	if err != nil {
		return nil, fmt.Errorf("adb not found: %w", err)
	}

	serial, err := m.findRunningEmulator(adb)
	if err != nil {
		return nil, err
	}
	if serial != "" {
		fmt.Printf("Using already-running emulator: %s\n", serial)
		return &EmulatorSession{Serial: serial, spawned: false}, nil
	}

	emulatorPath, err := findEmulatorTool(m.sdk)
	if err != nil {
		return nil, err
	}

	avdName, err := m.resolveAVD(emulatorPath)
	if err != nil {
		return nil, err
	}

	port := defaultEmulatorPort
	serial = fmt.Sprintf("emulator-%d", port)

	fmt.Printf("Launching emulator %q (serial %s)...\n", avdName, serial)
	args := []string{
		"-avd", avdName,
		"-no-snapshot",
		"-no-boot-anim",
		"-gpu", m.gpuMode,
		"-port", fmt.Sprintf("%d", port),
	}
	if m.headless {
		args = append(args, "-no-window")
	}

	handle, err := m.runner.Start(CommandSpec{
		Path:   emulatorPath,
		Args:   args,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("start emulator: %w", err)
	}

	if err := m.waitForBoot(ctx, adb, serial); err != nil {
		_ = handle.Kill()
		_ = handle.Wait()
		return nil, err
	}

	return &EmulatorSession{Serial: serial, proc: handle, spawned: true}, nil
}

func (m *EmulatorManager) findRunningEmulator(adb string) (string, error) {
	output, err := m.runner.Output(CommandSpec{
		Path: adb,
		Args: []string{"devices"},
	})
	if err != nil {
		return "", fmt.Errorf("adb devices failed: %w\n%s", err, output)
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "emulator-") && strings.Contains(line, "\tdevice") {
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) > 0 {
				serial := strings.TrimSpace(parts[0])
				if serial != "" {
					return serial, nil
				}
			}
		}
	}
	return "", nil
}

func (m *EmulatorManager) resolveAVD(emulatorPath string) (string, error) {
	if avd := os.Getenv("LURPIC_ANDROID_AVD"); avd != "" {
		return avd, nil
	}
	if avd := os.Getenv("ANDROID_AVD_NAME"); avd != "" {
		return avd, nil
	}
	return m.managedAVD()
}

func (m *EmulatorManager) managedAVDName() string {
	return fmt.Sprintf("lurpic_api%d_google_apis_%s", m.apiLevel, m.arch.EmulatorABI)
}

func (m *EmulatorManager) managedAVD() (string, error) {
	avdName := m.managedAVDName()
	avdDir := filepath.Join(os.Getenv("HOME"), ".android", "avd", avdName+".avd")
	if _, err := os.Stat(avdDir); err == nil { //nolint:gosec // path from user config
		return avdName, nil
	}
	return m.createDefaultAVD()
}

func (m *EmulatorManager) createDefaultAVD() (string, error) {
	avdmanager, err := findCmdlineTool(m.sdk, "avdmanager")
	if err != nil {
		return "", err
	}
	sdkmanager, err := findCmdlineTool(m.sdk, "sdkmanager")
	if err != nil {
		return "", err
	}

	// Accept SDK licenses non-interactively
	if err := m.runner.Run(CommandSpec{
		Path:   sdkmanager,
		Args:   []string{"--licenses"},
		Stdin:  strings.NewReader(documentedLicenseStdin),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return "", fmt.Errorf("sdkmanager --licenses failed: %w", err)
	}

	systemImage := fmt.Sprintf("system-images;android-%d;google_apis;%s", m.apiLevel, m.arch.EmulatorABI)
	if err := m.ensurePackage(sdkmanager, systemImage); err != nil {
		return "", err
	}

	avdName := m.managedAVDName()
	fmt.Printf("Creating default Android Virtual Device %q...\n", avdName)
	if err := m.runner.Run(CommandSpec{
		Path:   avdmanager,
		Args:   []string{"create", "avd", "-n", avdName, "-k", systemImage, "-d", "pixel_6", "--force"},
		Stdin:  strings.NewReader(documentedAVDStdin),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return "", fmt.Errorf("create avd failed: %w", err)
	}
	return avdName, nil
}

func (m *EmulatorManager) ensurePackage(sdkmanager, pkg string) error {
	if err := m.runner.Run(CommandSpec{
		Path:   sdkmanager,
		Args:   []string{pkg},
		Stdin:  strings.NewReader(documentedPackageStdin),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("sdkmanager install %s failed: %w", pkg, err)
	}
	return nil
}

func (m *EmulatorManager) waitForBoot(ctx context.Context, adb, serial string) error {
	deadline, hasDeadline := ctx.Deadline()
	timeout := defaultBootTimeout
	if hasDeadline {
		timeout = time.Until(deadline)
	}
	deadline = time.Now().Add(timeout)
	fmt.Printf("Waiting for emulator %s to boot (timeout %v)...\n", serial, timeout)

	if err := m.runner.Run(CommandSpec{
		Path: adb,
		Args: []string{"-s", serial, "wait-for-device"},
	}); err != nil {
		return fmt.Errorf("adb wait-for-device failed: %w", err)
	}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return fmt.Errorf("boot wait cancelled: %w", ctx.Err())
		default:
		}
		out, err := m.runner.Output(CommandSpec{
			Path: adb,
			Args: []string{"-s", serial, "shell", "getprop", "sys.boot_completed"},
		})
		if err == nil && strings.TrimSpace(string(out)) == "1" {
			// Boot completed, now wait for package manager
			pmOut, pmErr := m.runner.Output(CommandSpec{
				Path: adb,
				Args: []string{"-s", serial, "shell", "pm", "path", platformAndroid},
			})
			if pmErr == nil && strings.Contains(string(pmOut), "package:") {
				fmt.Printf("Emulator %s boot complete\n", serial)
				return nil
			}
		}
		time.Sleep(m.bootPollInterval)
	}
	return fmt.Errorf("timed out waiting for emulator %s to boot", serial)
}
