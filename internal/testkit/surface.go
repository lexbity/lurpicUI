package testkit

import (
	"errors"
	"image"
	"image/color"
	"sync"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

// MemorySurface is an in-memory platform.Surface implementation.
type MemorySurface struct {
	mu        sync.Mutex
	buffer    []byte
	presented []byte
	stride    int
	width     int
	height    int
	locked    bool
}

// NewMemorySurface constructs a memory-backed surface.
func NewMemorySurface(width, height int) *MemorySurface {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	s := &MemorySurface{}
	s.Resize(width, height)
	return s
}

// Resize reallocates the surface backing buffers.
func (s *MemorySurface) Resize(width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	s.width = width
	s.height = height
	s.stride = width * 4
	s.buffer = make([]byte, width*height*4)
	s.presented = make([]byte, width*height*4)
	s.locked = false
}

// Buffer returns the writable surface buffer.
func (s *MemorySurface) Buffer() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.locked {
		panic("testkit: Buffer called without Lock")
	}
	return s.buffer
}

// Stride returns the row stride in bytes.
func (s *MemorySurface) Stride() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stride
}

// Size returns the surface dimensions.
func (s *MemorySurface) Size() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.width, s.height
}

// Scale reports the platform content scale for the surface.
func (s *MemorySurface) Scale() float32 {
	return 1
}

// Lock marks the surface as writable and copies the last presented frame into the buffer.
func (s *MemorySurface) Lock() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked {
		return errors.New("testkit: surface already locked")
	}
	copy(s.buffer, s.presented)
	s.locked = true
	return nil
}

// Unlock presents the current buffer contents.
func (s *MemorySurface) Unlock(dirtyRects []gfx.Rect) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.locked {
		return errors.New("testkit: surface not locked")
	}
	copy(s.presented, s.buffer)
	s.locked = false
	_ = dirtyRects
	return nil
}

// Capture returns a copy of the last presented frame.
func (s *MemorySurface) Capture() *image.RGBA {
	s.mu.Lock()
	defer s.mu.Unlock()
	img := image.NewRGBA(image.Rect(0, 0, s.width, s.height))
	copy(img.Pix, s.presented)
	return img
}

// PixelAt returns the last presented pixel at x,y.
func (s *MemorySurface) PixelAt(x, y int) color.RGBA {
	s.mu.Lock()
	defer s.mu.Unlock()
	if x < 0 || y < 0 || x >= s.width || y >= s.height {
		return color.RGBA{}
	}
	off := y*s.stride + x*4
	return color.RGBA{
		R: s.presented[off],
		G: s.presented[off+1],
		B: s.presented[off+2],
		A: s.presented[off+3],
	}
}

var _ platform.Surface = (*MemorySurface)(nil)
