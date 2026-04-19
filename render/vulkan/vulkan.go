package vulkan

import "codeburg.org/lexbit/lurpicui/render"

type Backend struct{}

func (b *Backend) Initialize(s render.Surface) error { panic("vulkan: not implemented") }
func (b *Backend) Submit(f *render.Frame) error      { panic("vulkan: not implemented") }
func (b *Backend) Resize(w, h int) error             { panic("vulkan: not implemented") }
func (b *Backend) Destroy()                          {}

var _ render.Backend = (*Backend)(nil)
