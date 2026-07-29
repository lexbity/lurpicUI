package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyLayout_CLI_QuickSquareApp(t *testing.T) {
	// Requires the demo to exist and be buildable.
	root := findModuleRoot()
	if root == "" {
		t.Skip("module root not found")
	}
	demoDir := filepath.Join(root, "demos", "quick_square_app")
	if _, err := os.Stat(demoDir); err != nil {
		t.Skip("quick_square_app not available:", err)
	}

	got := runVerifyLayout([]string{
		"-builder", "buildRoot",
		"-size", "320x320",
		filepath.Join(root, "demos/quick_square_app"),
	})
	if got != 0 {
		t.Errorf("runVerifyLayout(quick_square_app) = %d, want 0", got)
	}

	// Assert the transient test file was cleaned up.
	leakPath := filepath.Join(demoDir, "lurpiclint_verifylayout_test.go")
	if _, err := os.Stat(leakPath); err == nil {
		t.Error("transient test file was not cleaned up:", leakPath)
	}
}

func TestVerifyLayout_CLI_BadSize(t *testing.T) {
	got := runVerifyLayout([]string{
		"-builder", "buildRoot",
		"-size", "not-a-size",
		"demos/quick_square_app",
	})
	if got != 2 {
		t.Errorf("runVerifyLayout(bad size) = %d, want 2", got)
	}
}

func TestVerifyLayout_CLI_BadFlags(t *testing.T) {
	got := runVerifyLayout([]string{
		"--nonexistent-flag",
	})
	if got != 2 {
		t.Errorf("runVerifyLayout(bad flag) = %d, want 2", got)
	}
}

func TestVerifyLayout_CLI_NonExistentPkg(t *testing.T) {
	got := runVerifyLayout([]string{
		"-builder", "BuildRoot",
		"-size", "320x320",
		"nonexistent/package/path",
	})
	if got != 2 {
		t.Errorf("runVerifyLayout(nonexistent pkg) = %d, want 2", got)
	}
}
