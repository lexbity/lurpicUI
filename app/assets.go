//go:build !android
// +build !android

package app

import (
	"fmt"
	"os"
	"path/filepath"
)

// Asset reads a bundled asset file.
//
// On desktop platforms (Linux, macOS, Windows), this reads from the
// application's assets/ directory relative to the executable or working directory.
//
// On Android, this would use AssetManager via JNI (see assets_android.go).
//
// Example:
//
//	data, err := app.Asset("fonts/regular.ttf")
//	if err != nil {
//	    log.Fatal(err)
//	}
func Asset(path string) ([]byte, error) {
	// Try multiple locations for the assets directory
	possiblePaths := []string{
		// Relative to working directory
		filepath.Join("assets", path),
		// Relative to executable (if we can determine it)
		filepath.Join(getExeDir(), "assets", path),
		// In a standard location relative to project root
		filepath.Join("..", "..", "assets", path),
	}

	var lastErr error
	for _, p := range possiblePaths {
		data, err := os.ReadFile(p)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("asset not found: %s (last error: %w)", path, lastErr)
}

// getExeDir returns the directory of the current executable
func getExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
