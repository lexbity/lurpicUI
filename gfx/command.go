package gfx

import (
	"image"

	"codeburg.org/lexbit/lurpicui/text"
)

type Command interface {
	isCommand()
}

type PushTransform struct {
	Matrix Transform
}

type PopTransform struct{}

type PushClipRect struct {
	Rect Rect
}

type PopClip struct{}

type PushOpacity struct {
	Alpha float32
}

type PopOpacity struct{}

type FillRect struct {
	Rect  Rect
	Brush Brush
}

type StrokeRect struct {
	Rect   Rect
	Stroke StrokeStyle
	Brush  Brush
}

type FillPath struct {
	Path  Path
	Brush Brush
}

type StrokePath struct {
	Path   Path
	Stroke StrokeStyle
	Brush  Brush
}

type DrawPolyline struct {
	Points []Point
	Stroke StrokeStyle
	Brush  Brush
	Closed bool
}

type DrawPoints struct {
	Points []Point
	Radius float32
	Brush  Brush
}

type DrawGlyphRun struct {
	Run    text.GlyphRun
	Origin Point
	Brush  Brush
}

type DrawSelectionRects struct {
	Rects []Rect
	Brush Brush
}

type SamplingMode uint8

const (
	SamplingNearest SamplingMode = iota
	SamplingBilinear
)

type DrawImage struct {
	Image    *image.RGBA
	DestRect Rect
	SrcRect  Rect
	Sampling SamplingMode
	Opacity  float32
}

type RenderBatchCacheID uint64

type BeginRenderBatch struct {
	Bounds  Rect
	CacheID RenderBatchCacheID
}

type EndRenderBatch struct{}

type CommandList struct {
	Commands []Command
}

func (cmd PushTransform) isCommand()      {}
func (cmd PopTransform) isCommand()       {}
func (cmd PushClipRect) isCommand()       {}
func (cmd PopClip) isCommand()            {}
func (cmd PushOpacity) isCommand()        {}
func (cmd PopOpacity) isCommand()         {}
func (cmd FillRect) isCommand()           {}
func (cmd StrokeRect) isCommand()         {}
func (cmd FillPath) isCommand()           {}
func (cmd StrokePath) isCommand()         {}
func (cmd DrawPolyline) isCommand()       {}
func (cmd DrawPoints) isCommand()         {}
func (cmd DrawGlyphRun) isCommand()       {}
func (cmd DrawSelectionRects) isCommand() {}
func (cmd DrawImage) isCommand()          {}
func (cmd BeginRenderBatch) isCommand()         {}
func (cmd EndRenderBatch) isCommand()           {}

func (cl *CommandList) Add(cmd Command) {
	if cl == nil {
		return
	}
	cl.Commands = append(cl.Commands, cmd)
}

func (cl *CommandList) Reset() {
	if cl == nil {
		return
	}
	cl.Commands = cl.Commands[:0]
}

func (cl *CommandList) Len() int {
	if cl == nil {
		return 0
	}
	return len(cl.Commands)
}
