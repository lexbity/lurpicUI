package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

// lurpicNativeActivityJava is the canonical Java NativeActivity source. It is
// compiled and dexed into every APK; its package/class name
// (org.lurpicui.bridge.LurpicNativeActivity) is a contract with the JNI symbols
// in platform/android/internal/bridge/lurpic_android.c.
//
//go:embed androidtmpl/LurpicNativeActivity.java
var lurpicNativeActivityJava string

// javaPackagePath mirrors the Java package of the embedded activity, used to lay
// the source out for javac.
var javaPackagePath = filepath.Join("org", "lurpicui", "bridge")

// buildJavaDex compiles the embedded Java activity against android.jar and dexes
// it into build/android/classes.dex, which assembleAPK injects at the APK root.
func (b *androidBuilder) buildJavaDex() error {
	fmt.Println("Compiling Java + dex...")

	javac := filepath.Join(b.jdk, "bin", "javac")
	jarTool := filepath.Join(b.jdk, "bin", "jar")
	if runtime.GOOS == platformWindows {
		javac += ".exe"
		jarTool += ".exe"
	}
	if _, err := os.Stat(javac); err != nil {
		return fmt.Errorf("javac not found at %s: %w", javac, err)
	}
	if _, err := os.Stat(jarTool); err != nil {
		return fmt.Errorf("jar not found at %s: %w", jarTool, err)
	}

	d8, err := findSDKTool(b.sdk, "d8")
	if err != nil {
		return fmt.Errorf("d8 not found: %w", err)
	}

	androidJar, err := b.androidJarPath()
	if err != nil {
		return err
	}

	// Write the embedded source into a package-shaped tree for javac.
	srcDir := filepath.Join(b.buildDir, "java")
	pkgDir := filepath.Join(srcDir, javaPackagePath)
	//nolint:gosec // build output dir
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		return err
	}
	srcFile := filepath.Join(pkgDir, "LurpicNativeActivity.java")
	//nolint:gosec // shared build artifact
	if err := os.WriteFile(srcFile, []byte(lurpicNativeActivityJava), 0644); err != nil {
		return fmt.Errorf("write java source: %w", err)
	}

	// Compile to class files. --release 11 keeps the bytecode within d8's
	// supported range regardless of the host JDK version.
	classesDir := filepath.Join(b.buildDir, "classes")
	if err := os.RemoveAll(classesDir); err != nil {
		return err
	}
	//nolint:gosec // build output dir
	if err := os.MkdirAll(classesDir, 0755); err != nil {
		return err
	}
	if err := b.runner.Run(CommandSpec{
		Path:   javac,
		Args:   []string{"--release", "11", "-classpath", androidJar, "-d", classesDir, srcFile},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("javac failed: %w", err)
	}

	// Pack the class files (including inner classes) into a jar for d8.
	classesJar := filepath.Join(b.buildDir, "classes.jar")
	_ = os.Remove(classesJar)
	if err := b.runner.Run(CommandSpec{
		Path:   jarTool,
		Args:   []string{"cf", classesJar, "-C", classesDir, "."},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("jar failed: %w", err)
	}

	// Dex the jar. d8 writes classes.dex into the output directory.
	dexDir := filepath.Join(b.buildDir, "dex")
	if err := os.RemoveAll(dexDir); err != nil {
		return err
	}
	//nolint:gosec // build output dir
	if err := os.MkdirAll(dexDir, 0755); err != nil {
		return err
	}
	if err := b.runner.Run(CommandSpec{
		Path: d8,
		Args: []string{
			"--release",
			"--min-api", strconv.Itoa(b.config.Android.MinSDK),
			"--lib", androidJar,
			"--output", dexDir,
			classesJar,
		},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("d8 failed: %w", err)
	}

	produced := filepath.Join(dexDir, "classes.dex")
	if _, err := os.Stat(produced); err != nil {
		return fmt.Errorf("d8 did not produce classes.dex: %w", err)
	}
	if err := copyFile(produced, filepath.Join(b.buildDir, "classes.dex")); err != nil {
		return fmt.Errorf("stage classes.dex: %w", err)
	}

	fmt.Printf("  Dexed: %s\n", filepath.Join(b.buildDir, "classes.dex"))
	return nil
}

// androidJarPath resolves android.jar for the target SDK, falling back to the
// highest installed platform.
func (b *androidBuilder) androidJarPath() (string, error) {
	target := filepath.Join(b.sdk, "platforms", fmt.Sprintf("android-%d", b.config.Android.TargetSDK), "android.jar")
	if _, err := os.Stat(target); err == nil {
		return target, nil
	}

	platformsDir := filepath.Join(b.sdk, "platforms")
	entries, err := os.ReadDir(platformsDir)
	if err != nil {
		return "", fmt.Errorf("no android platforms found in %s: %w", platformsDir, err)
	}
	best := ""
	for _, e := range entries {
		candidate := filepath.Join(platformsDir, e.Name(), "android.jar")
		if _, err := os.Stat(candidate); err == nil {
			best = candidate
		}
	}
	if best == "" {
		return "", fmt.Errorf("android.jar not found; install a platform (e.g. sdkmanager \"platforms;android-%d\")", b.config.Android.TargetSDK)
	}
	return best, nil
}
