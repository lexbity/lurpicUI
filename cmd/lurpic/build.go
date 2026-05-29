package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type buildFlags struct {
	release    bool
	output     string
	keystore   string
	ksAlias    string
	ksPassword string
	sdkPath    string
	ndkPath    string
	jdkPath    string
	abi        string
}

func cmdBuild(args []string) int {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	var flags buildFlags
	fs.BoolVar(&flags.release, "release", false, "Build release APK")
	fs.StringVar(&flags.output, "o", "", "Output path for APK")
	fs.StringVar(&flags.output, "output", "", "Output path for APK")
	fs.StringVar(&flags.keystore, "keystore", "", "Release keystore path (overrides config)")
	fs.StringVar(&flags.ksAlias, "ks-alias", "", "Release keystore alias (overrides config)")
	fs.StringVar(&flags.ksPassword, "ks-pass", "", "Release keystore password (overrides config)")
	fs.StringVar(&flags.sdkPath, "sdk-path", "", "Android SDK path (overrides config/env)")
	fs.StringVar(&flags.ndkPath, "ndk-path", "", "Android NDK path (overrides config/env)")
	fs.StringVar(&flags.jdkPath, "jdk-path", "", "JDK path (overrides config/env)")
	fs.StringVar(&flags.abi, "abi", "", "Target ABI (default: all configured in lurpic.toml)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: platform required (e.g., 'android')")
		fmt.Fprintln(os.Stderr, "Usage: lurpic build android [flags]")
		return 1
	}

	platform := fs.Arg(0)
	if platform != "android" {
		fmt.Fprintf(os.Stderr, "Error: unsupported platform '%s' (only 'android' supported)\n", platform)
		return 1
	}

	return buildAndroid(flags)
}

func buildAndroid(flags buildFlags) int {
	builder, err := prepareAndroidBuild(flags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if err := builder.build(); err != nil {
		fmt.Fprintf(os.Stderr, "Build failed: %v\n", err)
		return 1
	}

	fmt.Printf("APK built: %s\n", builder.outputPath)
	return 0
}

func prepareAndroidBuild(flags buildFlags) (*androidBuilder, error) {
	// Detect project root and load configuration
	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("cannot find project root: %w", err)
	}

	config, err := loadConfig(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("error loading lurpic.toml: %w", err)
	}

	// Load user config for toolchain paths
	userConfig, _ := loadUserConfig()

	// Create toolchain detector with all sources
	detector := &ToolchainDetector{
		FlagSDK:    flags.sdkPath,
		FlagNDK:    flags.ndkPath,
		FlagJDK:    flags.jdkPath,
		Config:     config,
		UserConfig: userConfig,
	}

	// Detect Android SDK
	sdk, sdkSource, err := detector.DetectSDK()
	if err != nil {
		return nil, fmt.Errorf("%w\n\nRun 'lurpic doctor' for detailed diagnostics.", err)
	}

	// Detect Android NDK
	ndk, ndkSource, err := detector.DetectNDK(sdk)
	if err != nil {
		return nil, fmt.Errorf("%w\n\nRun 'lurpic doctor' for detailed diagnostics.", err)
	}

	// Detect JDK
	jdk, jdkSource, err := detector.DetectJDK()
	if err != nil {
		return nil, fmt.Errorf("%w\n\nRun 'lurpic doctor' for detailed diagnostics.", err)
	}

	fmt.Printf("Android SDK: %s (found via %s)\n", sdk, sdkSource)
	fmt.Printf("Android NDK: %s (found via %s)\n", ndk, ndkSource)
	fmt.Printf("JDK: %s (found via %s)\n", jdk, jdkSource)
	fmt.Printf("App ID: %s\n", config.App.ID)
	fmt.Printf("App Name: %s\n", config.App.Name)

	// Create build directory
	buildDir := filepath.Join(projectRoot, "build", "android")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return nil, fmt.Errorf("error creating build directory: %w", err)
	}

	// Determine output path
	outputPath := flags.output
	if outputPath == "" {
		suffix := "debug"
		if flags.release {
			suffix = "release"
		}
		outputPath = filepath.Join(buildDir, fmt.Sprintf("%s-%s.apk", config.App.ID, suffix))
	}

	// If --abi flag is set, restrict build to that single ABI
	if flags.abi != "" {
		if _, ok := ArchitectureByABI(flags.abi); !ok {
			return nil, fmt.Errorf("unsupported ABI: %s", flags.abi)
		}
		config.Android.ABIs = []string{flags.abi}
	}

	// Apply command-line overrides to config for keystore path and alias
	// (password is NOT stored in config — passed via the ksPassword field instead)
	if flags.keystore != "" {
		config.Android.Keystore.Path = flags.keystore
	}
	if flags.ksAlias != "" {
		config.Android.Keystore.Alias = flags.ksAlias
	}

	builder := &androidBuilder{
		runner:      newExecRunner(),
		sdk:         sdk,
		ndk:         ndk,
		projectRoot: projectRoot,
		buildDir:    buildDir,
		config:      config,
		release:     flags.release,
		outputPath:  outputPath,
		ksPassword:  flags.ksPassword,
	}
	return builder, nil
}

func findProjectRoot() (string, error) {
	// Start from current directory and walk up looking for lurpic.toml
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot get working directory: %w", err)
	}

	for {
		configPath := filepath.Join(dir, "lurpic.toml")
		if _, err := os.Stat(configPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no lurpic.toml found (are you in a lurpicUI project?)")
}
