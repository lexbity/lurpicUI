package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFullPackageBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build test in short mode")
	}
	root := filepath.Join("..", "..")
	cmd := exec.Command("go", "build", "./demos/lurpic_studio/...")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("full build failed: %v\n%s", err, out)
	}
}
