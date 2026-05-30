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

func cAAudioAvailable() bool                        { return false }
func cAAudioStreamOpen(int, int, int, bool) (unsafe.Pointer, error) {
	return nil, ErrNotAndroid
}
func cAAudioStreamWrite(unsafe.Pointer, []int16) (int, error) {
	return 0, ErrNotAndroid
}
func cAAudioStreamPause(unsafe.Pointer) error        { return ErrNotAndroid }
func cAAudioStreamResume(unsafe.Pointer) error       { return ErrNotAndroid }
func cAAudioStreamClose(unsafe.Pointer) error        { return ErrNotAndroid }
func cAAudioStreamLatency(unsafe.Pointer) int        { return 0 }
func cOpenSLStreamOpen(int, int, int) (unsafe.Pointer, error) {
	return nil, ErrNotAndroid
}
func cOpenSLStreamWrite(unsafe.Pointer, []int16) (int, error) {
	return 0, ErrNotAndroid
}
func cOpenSLStreamPause(unsafe.Pointer) error         { return ErrNotAndroid }
func cOpenSLStreamResume(unsafe.Pointer) error        { return ErrNotAndroid }
func cOpenSLStreamClose(unsafe.Pointer) error         { return ErrNotAndroid }
