package render

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
)

var _ = text.GlyphRun{}

type Surface interface {
	Size() (width, height int)
	Resize(width, height int)
}

type LayerID uint64

type Layer struct {
	ID          LayerID
	Bounds      gfx.Rect
	Opacity     float32
	Commands    gfx.CommandList
	CommandHash uint64
}

type Frame struct {
	Layers       []Layer
	DirtyRegions []gfx.Rect
}

type Backend interface {
	Initialize(surface Surface) error
	Submit(frame *Frame) error
	Resize(width, height int) error
	Destroy()
}
