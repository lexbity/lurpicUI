// Package testtmp provides filesystem helpers for tests whose artifacts have
// hard path-length limits — Unix domain sockets, whose sun_path field is 108
// bytes including the terminating NUL on Linux.
package testtmp

import (
	"os"
	"path/filepath"
	"testing"
)

// maxSocketPath caps len(dir + "/drive.sock") so the resulting sun_path stays
// within the 108-byte Linux limit with margin.
const maxSocketPath = 100

// ShortSocketDir returns a freshly created private (0700) directory whose
// path is short enough to hold a Unix socket file, regardless of how deep
// TMPDIR/GOTMPDIR point. t.TempDir() honors GOTMPDIR, which the CMake go
// wrapper sets to <build-dir>/go-tmp — deep enough that bind(2) fails with
// EINVAL on the socket path. The directory is removed when the test finishes.
// The test is skipped when no available base yields a short-enough path.
func ShortSocketDir(t *testing.T) string {
	t.Helper()
	for _, base := range []string{os.TempDir(), "/tmp", "."} {
		//nolint:usetesting // deliberate: t.TempDir() honors GOTMPDIR, which can exceed the sun_path limit
		dir, err := os.MkdirTemp(base, "lurpicdrv")
		if err != nil {
			continue
		}
		if len(filepath.Join(dir, "drive.sock")) <= maxSocketPath {
			//nolint:gosec // G302: 0700 is correct for a private directory (traversable, not a file)
			if err := os.Chmod(dir, 0o700); err != nil {
				_ = os.RemoveAll(dir)
				t.Fatalf("chmod socket dir: %v", err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			return dir
		}
		_ = os.RemoveAll(dir)
	}
	t.Skip("no temp base with a short enough path for a unix socket")
	return ""
}
