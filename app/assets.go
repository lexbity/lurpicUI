//go:build !android

package app

import (
	"fmt"
	"os"
	"path/filepath"
)

const maxBootstrapAssetSize = 1 << 20 // 1 MB

// Asset reads a bundled asset file as a single byte slice.
//
// **Bootstrap-only API.** Use this only for small files (configs, fonts)
// needed before the runtime's asset pipeline (Manager + PakFS/DevFS) is
// available. All large media (images, textures, audio) must be loaded
// through the Manager via facet asset handles so they benefit from
// streaming, progressive LODs, cache budgeting, and eviction.
//
// The function reads the entire file into memory at once — it bypasses
// the asset system entirely. Files larger than MaxBootstrapAssetSize
// (1 MiB) produce a diagnostic warning at Info level.
//
// File resolution order:
//  1. ./assets/<path> (relative to working directory)
//  2. <exe-dir>/assets/<path>
//  3. ../../assets/<path> (project-root relative during development)
func Asset(path string) ([]byte, error) {
	possiblePaths := []string{
		filepath.Join("assets", path),
		filepath.Join(getExeDir(), "assets", path),
		filepath.Join("..", "..", "assets", path),
	}

	var lastErr error
	for _, p := range possiblePaths {
		data, err := os.ReadFile(p) //nolint:gosec // path from user config
		if err == nil {
			if len(data) > maxBootstrapAssetSize {
				fmt.Fprintf(os.Stderr, "app: Asset(%q) read %d bytes — large media should use the asset manager (Manager.LoadImage etc.)\n", path, len(data))
			}
			return data, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("asset not found: %s (last error: %w)", path, lastErr)
}

func getExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
