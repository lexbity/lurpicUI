//go:build android_cross

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAndroidCrossCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cross-compile test in short mode")
	}
	dir := filepath.Join("..", "..")
	cmd := exec.Command("go", "build", "./demos/lurpic_studio/...")
	cmd.Env = append(os.Environ(), "GOOS=android", "GOARCH=arm64")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("GOOS=android GOARCH=arm64 build failed: %v\n%s", err, out)
	}
}

func TestAndroidCrossCompileAmd64(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cross-compile test in short mode")
	}
	dir := filepath.Join("..", "..")
	cmd := exec.Command("go", "build", "./demos/lurpic_studio/...")
	cmd.Env = append(os.Environ(), "GOOS=android", "GOARCH=amd64")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("GOOS=android GOARCH=amd64 build failed: %v\n%s", err, out)
	}
}

func TestLinuxAmd64Build(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build test in short mode")
	}
	dir := filepath.Join("..", "..")
	cmd := exec.Command("go", "build", "./demos/lurpic_studio/...")
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("GOOS=linux GOARCH=amd64 build failed: %v\n%s", err, out)
	}
}
