package testtmp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShortSocketDir(t *testing.T) {
	dir := ShortSocketDir(t)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat returned dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", dir)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("dir perms = %o, want 700", info.Mode().Perm())
	}
	sock := filepath.Join(dir, "drive.sock")
	if len(sock) > maxSocketPath {
		t.Errorf("socket path %q is %d bytes, want <= %d", sock, len(sock), maxSocketPath)
	}
	if filepath.IsAbs(dir) && strings.Contains(dir, "go-tmp") {
		t.Errorf("dir %q came from a deep GOTMPDIR; want a short base", dir)
	}
}

// TestShortSocketDirSurvivesDeepTempDir simulates the CMake wrapper setting
// GOTMPDIR to a deep build path: t.TempDir() would land there, but
// ShortSocketDir must still return a short path (os.MkdirTemp ignores
// GOTMPDIR and uses TMPDIR).
func TestShortSocketDirSurvivesDeepTempDir(t *testing.T) {
	deep := t.TempDir() + "/build/ci-software-headless/go-tmp"
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOTMPDIR", deep)
	dir := ShortSocketDir(t)
	if len(filepath.Join(dir, "drive.sock")) > maxSocketPath {
		t.Errorf("socket path too long under deep GOTMPDIR: %d bytes", len(filepath.Join(dir, "drive.sock")))
	}
}
