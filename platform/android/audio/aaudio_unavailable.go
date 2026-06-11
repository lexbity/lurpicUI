//go:build !android

package audio

import (
	"errors"
	"unsafe"
)

// Backend is a stub on non-Android platforms.
type Backend struct{}

// NewBackend returns a stub on non-Android platforms.
func NewBackend() *Backend {
	return nil
}

// ErrNotAndroid is returned when Android audio functions are called on
// non-Android platforms.
var ErrNotAndroid = errors.New("android audio: not available on this platform")

//nolint:unused // build-tag stub
func cAAudioAvailable() bool { return false }

//nolint:unused // build-tag stub
func cAAudioStreamOpen(int, int, int, bool) (unsafe.Pointer, error) {
	return nil, ErrNotAndroid
}

//nolint:unused // build-tag stub
func cAAudioStreamWrite(unsafe.Pointer, []int16) (int, error) {
	return 0, ErrNotAndroid
}

//nolint:unused // build-tag stub
func cAAudioStreamPause(unsafe.Pointer) error { return ErrNotAndroid }

//nolint:unused // build-tag stub
func cAAudioStreamResume(unsafe.Pointer) error { return ErrNotAndroid }

//nolint:unused // build-tag stub
func cAAudioStreamClose(unsafe.Pointer) error { return ErrNotAndroid }

//nolint:unused // build-tag stub
func cAAudioStreamLatency(unsafe.Pointer) int { return 0 }

//nolint:unused // build-tag stub
func cOpenSLStreamOpen(int, int, int) (unsafe.Pointer, error) {
	return nil, ErrNotAndroid
}

//nolint:unused // build-tag stub
func cOpenSLStreamWrite(unsafe.Pointer, []int16) (int, error) {
	return 0, ErrNotAndroid
}

//nolint:unused // build-tag stub
func cOpenSLStreamPause(unsafe.Pointer) error { return ErrNotAndroid }

//nolint:unused // build-tag stub
func cOpenSLStreamResume(unsafe.Pointer) error { return ErrNotAndroid }

//nolint:unused // build-tag stub
func cOpenSLStreamClose(unsafe.Pointer) error { return ErrNotAndroid }
