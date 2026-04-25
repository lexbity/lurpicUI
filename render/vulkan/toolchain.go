package vulkan

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
)

var lookPath = exec.LookPath
var command = exec.Command

func CheckRustToolchain() error {
	cargoPath, err := lookPath("cargo")
	if err != nil {
		return errors.New("vulkan: cargo not found; install Rust with rustup and ensure cargo is on PATH")
	}

	cmd := command(cargoPath, "--version")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("vulkan: cargo --version failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func RustCrateRoot() (string, error) {
	_, file, _, ok := goruntime.Caller(0)
	if !ok {
		return "", errors.New("vulkan: unable to determine repository root")
	}
	return filepath.Clean(filepath.Dir(file)), nil
}

func RustCrateManifestPath() (string, error) {
	root, err := RustCrateRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "crates", "lurpic_render", "Cargo.toml"), nil
}

func BuildRustLibrary() error {
	if err := CheckRustToolchain(); err != nil {
		return err
	}

	manifest, err := RustCrateManifestPath()
	if err != nil {
		return err
	}
	root, err := RustCrateRoot()
	if err != nil {
		return err
	}

	cmd := command("cargo", "build", "--manifest-path", manifest)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("vulkan: cargo build failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	resetRustLibraryLoaderForTest()
	return nil
}
