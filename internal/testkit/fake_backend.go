package testkit

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
)

// FakeBackend is a controllable render.Backend for testing.
type FakeBackend struct {
	InitializeErr   error
	SubmitErr       error
	SubmitErrAfter  int // return SubmitErr after this many Submit calls; 0 means never
	InitializeCount int
	SubmitCount     int
	DestroyCount    int
	LastFrame       *render.Frame
	LastSurface     render.Surface
}

func (b *FakeBackend) Initialize(surface render.Surface) error {
	b.InitializeCount++
	b.LastSurface = surface
	return b.InitializeErr
}

func (b *FakeBackend) Submit(frame *render.Frame) error {
	b.SubmitCount++
	b.LastFrame = frame
	if b.SubmitErrAfter > 0 && b.SubmitCount >= b.SubmitErrAfter && b.SubmitErr != nil {
		return b.SubmitErr
	}
	return nil
}

func (b *FakeBackend) Resize(width, height int) error { return nil }

func (b *FakeBackend) Destroy() {
	b.DestroyCount++
}

func (b *FakeBackend) Buffer() ([]byte, error) { return make([]byte, 4), nil }
func (b *FakeBackend) Unlock([]gfx.Rect) error { return nil }

var _ render.Backend = (*FakeBackend)(nil)
