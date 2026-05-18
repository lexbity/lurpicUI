//go:build (!linux || !cgo) && !android

package vulkan

import "errors"

func Version() (string, error) {
	return "", errors.New("vulkan: Rust bridge requires linux with cgo")
}

func resetRustLibraryLoaderForTest() {}
