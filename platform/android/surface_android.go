//go:build android

package android

/*
#cgo LDFLAGS: -landroid
#include <android/native_window.h>
#include <stdint.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
)

// androidSurface implements render.SoftwareSurface by locking the
// ANativeWindow's back buffer, exposing it for the software renderer to blit
// into, and posting it on Unlock. The window is configured for RGBA_8888, which
// matches the renderer's image.RGBA byte order.
var _ render.SoftwareSurface = (*androidSurface)(nil)

// windowFormatRGBA8888 is ANativeWindow's RGBA_8888 format constant.
const windowFormatRGBA8888 = 1

func (s *androidSurface) nativeWindow() *C.ANativeWindow {
	return (*C.ANativeWindow)(unsafe.Pointer(s.window))
}

// Lock acquires the ANativeWindow back buffer for CPU rendering.
func (s *androidSurface) Lock() error {
	if s == nil || s.window == 0 {
		return errors.New("android surface: no native window")
	}
	win := s.nativeWindow()

	if !s.geomSet {
		// Pass 0,0 to inherit the window's native size while forcing the pixel
		// format to RGBA_8888.
		C.ANativeWindow_setBuffersGeometry(win, 0, 0, C.int32_t(windowFormatRGBA8888))
		s.geomSet = true
	}

	var buf C.ANativeWindow_Buffer
	if rc := C.ANativeWindow_lock(win, &buf, nil); rc != 0 {
		return fmt.Errorf("android surface: ANativeWindow_lock failed (%d)", int(rc))
	}

	s.bits = unsafe.Pointer(buf.bits)
	s.strideBytes = int(buf.stride) * 4
	s.width = int(buf.width)
	s.height = int(buf.height)
	s.locked = true
	return nil
}

// Buffer returns the locked back-buffer memory as a byte slice. It is only valid
// between Lock and Unlock.
func (s *androidSurface) Buffer() []byte {
	if s == nil || !s.locked || s.bits == nil {
		return nil
	}
	n := s.strideBytes * s.height
	if n <= 0 {
		return nil
	}
	return unsafe.Slice((*byte)(s.bits), n)
}

// Stride returns the back-buffer row stride in bytes.
func (s *androidSurface) Stride() int {
	return s.strideBytes
}

// Unlock posts the rendered buffer to the display.
func (s *androidSurface) Unlock(_ []gfx.Rect) error {
	if s == nil || !s.locked {
		return nil
	}
	s.locked = false
	s.bits = nil
	if rc := C.ANativeWindow_unlockAndPost(s.nativeWindow()); rc != 0 {
		return fmt.Errorf("android surface: ANativeWindow_unlockAndPost failed (%d)", int(rc))
	}
	return nil
}
