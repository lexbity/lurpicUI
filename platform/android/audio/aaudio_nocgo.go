//go:build android && !cgo

package audio

import (
	"errors"
	"unsafe"
)

// No-op stubs for android without CGO. These satisfy the linker when the
// platform/android/audio package is imported during CGO-disabled builds
// (e.g., go vet on host with GOOS=android). Real audio requires CGO.

var errCgoRequired = errors.New("android audio: requires CGO")

func cAAudioAvailable() bool                        { return false }
func cAAudioStreamOpen(int, int, int, bool) (unsafe.Pointer, error) {
	return nil, errCgoRequired
}
func cAAudioStreamWrite(unsafe.Pointer, []int16) (int, error) {
	return 0, errCgoRequired
}
func cAAudioStreamPause(unsafe.Pointer) error        { return errCgoRequired }
func cAAudioStreamResume(unsafe.Pointer) error       { return errCgoRequired }
func cAAudioStreamClose(unsafe.Pointer) error        { return errCgoRequired }
func cAAudioStreamLatency(unsafe.Pointer) int        { return 0 }
func cOpenSLStreamOpen(int, int, int) (unsafe.Pointer, error) {
	return nil, errCgoRequired
}
func cOpenSLStreamWrite(unsafe.Pointer, []int16) (int, error) {
	return 0, errCgoRequired
}
func cOpenSLStreamPause(unsafe.Pointer) error         { return errCgoRequired }
func cOpenSLStreamResume(unsafe.Pointer) error        { return errCgoRequired }
func cOpenSLStreamClose(unsafe.Pointer) error         { return errCgoRequired }
