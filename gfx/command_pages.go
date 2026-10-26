package gfx

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/text"
	"github.com/go-text/typesetting/language"
)

// CommandPage is an immutable, contiguous command sequence owned by one
// sealed render frame. Its fields and backing storage are private.
type CommandPage struct {
	commands   []CommandView
	fontLeases []*text.FontRenderLease
	bytes      uint64
}

// CommandRange is a bounded read-only view into one command page.
type CommandRange struct {
	page  *CommandPage
	start int
	count int
}

// SealCommandPageFromSource seals a known number of commands directly into one
// page allocation. It validates the source before allocation and visits it a
// second time to copy; no flattened intermediate list is created. A live frame
// reservation is mandatory and the source must remain stable for the call.
func SealCommandPageFromSource(commandCount int, source func(func(Command) bool), reservation *membudget.Reservation) (*CommandPage, error) {
	if commandCount < 0 || source == nil || reservation == nil {
		return nil, errors.New("gfx: invalid command page source")
	}
	var estimated uint64
	commandSlots, err := checkedSizeProduct(uint64(commandCount), uint64(unsafe.Sizeof(CommandView{})))
	if err != nil {
		return nil, err
	}
	if err := addBytes(&estimated, commandSlots); err != nil {
		return nil, err
	}
	var estimateErr error
	estimateCount := 0
	source(func(command Command) bool {
		if estimateErr != nil {
			return false
		}
		if estimateCount >= commandCount {
			estimateErr = errors.New("gfx: command source exceeded declared count")
			return false
		}
		payloadBytes, err := commandPayloadBytes(command)
		if err != nil {
			estimateErr = err
			return false
		}
		if err := addBytes(&estimated, payloadBytes); err != nil {
			estimateErr = err
			return false
		}
		if err := addBytes(&estimated, uint64(unsafe.Sizeof((*text.FontRenderLease)(nil)))); err != nil {
			estimateErr = err
			return false
		}
		estimateCount++
		return true
	})
	if estimateErr != nil {
		return nil, estimateErr
	}
	if estimateCount != commandCount {
		return nil, fmt.Errorf("gfx: command source produced %d commands, declared %d", estimateCount, commandCount)
	}
	if estimated > reservation.Bytes() {
		return nil, fmt.Errorf("gfx: command page needs %d bytes, reservation has %d", estimated, reservation.Bytes())
	}
	owned := make([]CommandView, commandCount)
	fontLeases := make([]*text.FontRenderLease, 0, commandCount)
	bytes := commandSlots
	index := 0
	var buildErr error
	cleanup := func() {
		for i := 0; i < index; i++ {
			clearCommandReferences(owned[i].payload)
			owned[i] = CommandView{}
		}
		clear(owned)
		for _, lease := range fontLeases {
			_ = lease.Release()
		}
		clear(fontLeases)
	}
	source(func(command Command) bool {
		if buildErr != nil {
			return false
		}
		if index >= commandCount {
			buildErr = errors.New("gfx: command source exceeded declared count")
			return false
		}
		payloadBytes, err := commandPayloadBytes(command)
		if err != nil {
			buildErr = err
			return false
		}
		if err := addBytes(&bytes, payloadBytes); err != nil {
			buildErr = err
			return false
		}
		if err := addBytes(&bytes, uint64(unsafe.Sizeof((*text.FontRenderLease)(nil)))); err != nil {
			buildErr = err
			return false
		}
		if bytes > estimated {
			buildErr = errors.New("gfx: command source grew during sealing")
			return false
		}
		cloned, err := cloneCommand(command)
		if err != nil {
			buildErr = err
			return false
		}
		owned[index] = ReadCommand(cloned)
		// Include the current command in unwind even if font pinning fails.
		index++
		if glyphRun, ok := cloned.(DrawGlyphRun); ok {
			if !glyphRun.Run.Face.IsZero() {
				lease, pinErr := glyphRun.Run.Face.PinForRender()
				if pinErr != nil {
					buildErr = fmt.Errorf("gfx: pin glyph font at command %d: %w", index, pinErr)
					return false
				}
				fontLeases = append(fontLeases, lease)
			}
		}
		return true
	})
	if buildErr == nil && index != commandCount {
		buildErr = fmt.Errorf("gfx: command source produced %d commands, declared %d", index, commandCount)
	}
	if buildErr != nil {
		cleanup()
		return nil, buildErr
	}
	if bytes > reservation.Bytes() {
		cleanup()
		return nil, fmt.Errorf("gfx: command source grew during sealing: needs %d bytes, reservation has %d", bytes, reservation.Bytes())
	}
	return &CommandPage{commands: owned, fontLeases: fontLeases, bytes: bytes}, nil
}

// EstimateCommandPageBytes returns the complete backing capacity required by
// a sealed page, including each command value and all nested copied payloads.
func EstimateCommandPageBytes(commands []Command) (uint64, error) {
	var total uint64
	commandSlots, err := checkedSizeProduct(uint64(len(commands)), uint64(unsafe.Sizeof(CommandView{})))
	if err != nil {
		return 0, err
	}
	if err := addBytes(&total, commandSlots); err != nil {
		return 0, err
	}
	for i, command := range commands {
		bytes, err := commandPayloadBytes(command)
		if err != nil {
			return 0, fmt.Errorf("gfx: command page estimate at index %d: %w", i, err)
		}
		if err := addBytes(&total, bytes); err != nil {
			return 0, err
		}
		if err := addBytes(&total, uint64(unsafe.Sizeof((*text.FontRenderLease)(nil)))); err != nil {
			return 0, err
		}
	}
	return total, nil
}

func (p *CommandPage) Len() int {
	if p == nil {
		return 0
	}
	return len(p.commands)
}

func (p *CommandPage) BackingBytes() uint64 {
	if p == nil {
		return 0
	}
	return p.bytes
}

func (p *CommandPage) Range(start, count int) (CommandRange, error) {
	if p == nil || start < 0 || count < 0 || start > len(p.commands) || count > len(p.commands)-start {
		return CommandRange{}, errors.New("gfx: command range is outside its page")
	}
	return CommandRange{page: p, start: start, count: count}, nil
}

func (r CommandRange) Len() int { return r.count }

func (r CommandRange) At(index int) CommandView {
	if index < 0 || index >= r.count {
		panic("gfx: command range index out of bounds")
	}
	return r.page.commands[r.start+index]
}

func (r CommandRange) Slice(start, end int) CommandRange {
	if start < 0 || end < start || end > r.count {
		panic("gfx: command subrange out of bounds")
	}
	return CommandRange{page: r.page, start: r.start + start, count: end - start}
}

func (r CommandRange) Visit(fn func(CommandView) bool) {
	if r.page == nil || fn == nil {
		return
	}
	end := r.start + r.count
	for i := r.start; i < end; i++ {
		if !fn(r.page.commands[i]) {
			return
		}
	}
}

func (p *CommandPage) release() error {
	if p == nil {
		return nil
	}
	var firstErr error
	for _, lease := range p.fontLeases {
		if err := lease.Release(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	clear(p.fontLeases)
	p.fontLeases = nil
	for i := range p.commands {
		clearCommandReferences(p.commands[i].payload)
		p.commands[i] = CommandView{}
	}
	clear(p.commands)
	p.commands = nil
	p.bytes = 0
	return firstErr
}

// Release clears the page's pointer-bearing command storage. The frame lease
// owner calls it only after the final consumer acknowledges the generation.
func (p *CommandPage) Release() error { return p.release() }

func commandPayloadBytes(command Command) (uint64, error) {
	if command == nil {
		return 0, errors.New("gfx: nil command cannot be sealed")
	}
	switch c := command.(type) {
	case PushTransform:
		return uint64(unsafe.Sizeof(c)), nil
	case PopTransform:
		return uint64(unsafe.Sizeof(c)), nil
	case PushClipRect:
		return uint64(unsafe.Sizeof(c)), nil
	case PopClip:
		return uint64(unsafe.Sizeof(c)), nil
	case PushOpacity:
		return uint64(unsafe.Sizeof(c)), nil
	case PopOpacity:
		return uint64(unsafe.Sizeof(c)), nil
	case FillRect:
		return sumSliceSizes(2*unsafe.Sizeof(c), sliceSize{len(c.Brush.GradientStops), unsafe.Sizeof(GradientStop{})})
	case StrokeRect:
		return sumSliceSizes(2*unsafe.Sizeof(c), sliceSize{len(c.Stroke.Dash), unsafe.Sizeof(float32(0))}, sliceSize{len(c.Brush.GradientStops), unsafe.Sizeof(GradientStop{})})
	case FillPath:
		return sumSliceSizes(2*unsafe.Sizeof(c), sliceSize{len(c.Path.Segments), unsafe.Sizeof(PathSegment{})}, sliceSize{len(c.Brush.GradientStops), unsafe.Sizeof(GradientStop{})})
	case StrokePath:
		return sumSliceSizes(2*unsafe.Sizeof(c), sliceSize{len(c.Path.Segments), unsafe.Sizeof(PathSegment{})}, sliceSize{len(c.Stroke.Dash), unsafe.Sizeof(float32(0))}, sliceSize{len(c.Brush.GradientStops), unsafe.Sizeof(GradientStop{})})
	case DrawPolyline:
		return sumSliceSizes(2*unsafe.Sizeof(c), sliceSize{len(c.Points), unsafe.Sizeof(Point{})}, sliceSize{len(c.Stroke.Dash), unsafe.Sizeof(float32(0))}, sliceSize{len(c.Brush.GradientStops), unsafe.Sizeof(GradientStop{})})
	case DrawPoints:
		return sumSliceSizes(2*unsafe.Sizeof(c), sliceSize{len(c.Points), unsafe.Sizeof(Point{})}, sliceSize{len(c.Brush.GradientStops), unsafe.Sizeof(GradientStop{})})
	case DrawGlyphRun:
		return sumSliceSizes(2*unsafe.Sizeof(c), sliceSize{1, unsafe.Sizeof(text.FontRenderLease{})}, sliceSize{len(c.Run.Glyphs), unsafe.Sizeof(text.PositionedGlyph{})}, sliceSize{len(c.Run.GraphemeAdvances), unsafe.Sizeof(float32(0))}, sliceSize{len(c.Run.Text), 1}, sliceSize{len(c.Run.Style.Family), 1}, sliceSize{len(c.Run.Language), 1}, sliceSize{len(c.Brush.GradientStops), unsafe.Sizeof(GradientStop{})})
	case DrawSelectionRects:
		return sumSliceSizes(2*unsafe.Sizeof(c), sliceSize{len(c.Rects), unsafe.Sizeof(Rect{})}, sliceSize{len(c.Brush.GradientStops), unsafe.Sizeof(GradientStop{})})
	case DrawImage:
		return uint64(unsafe.Sizeof(c)), nil
	case DrawTexture:
		return uint64(unsafe.Sizeof(c)), nil
	case DrawBlurredShadow:
		return sumSliceSizes(2*unsafe.Sizeof(c), sliceSize{len(c.Path.Segments), unsafe.Sizeof(PathSegment{})})
	case BeginRenderBatch:
		return uint64(unsafe.Sizeof(c)), nil
	case EndRenderBatch:
		return uint64(unsafe.Sizeof(c)), nil
	default:
		return 0, errors.New("gfx: unsupported command type " + commandType(command))
	}
}

func cloneCommand(command Command) (Command, error) {
	switch c := command.(type) {
	case PushTransform, PopTransform, PushClipRect, PopClip, PushOpacity, PopOpacity,
		DrawTexture, BeginRenderBatch, EndRenderBatch:
		return c, nil
	case FillRect:
		c.Brush = cloneBrush(c.Brush)
		return c, nil
	case StrokeRect:
		c.Stroke = cloneStroke(c.Stroke)
		c.Brush = cloneBrush(c.Brush)
		return c, nil
	case FillPath:
		c.Path = clonePath(c.Path)
		c.Brush = cloneBrush(c.Brush)
		return c, nil
	case StrokePath:
		c.Path = clonePath(c.Path)
		c.Stroke = cloneStroke(c.Stroke)
		c.Brush = cloneBrush(c.Brush)
		return c, nil
	case DrawPolyline:
		c.Points = copyValues(c.Points)
		c.Stroke = cloneStroke(c.Stroke)
		c.Brush = cloneBrush(c.Brush)
		return c, nil
	case DrawPoints:
		c.Points = copyValues(c.Points)
		c.Brush = cloneBrush(c.Brush)
		return c, nil
	case DrawGlyphRun:
		c.Run.Glyphs = copyValues(c.Run.Glyphs)
		c.Run.GraphemeAdvances = copyValues(c.Run.GraphemeAdvances)
		c.Run.Text = cloneString(c.Run.Text)
		c.Run.Style.Family = cloneString(c.Run.Style.Family)
		c.Run.Language = language.Language(cloneString(string(c.Run.Language)))
		c.Brush = cloneBrush(c.Brush)
		return c, nil
	case DrawSelectionRects:
		c.Rects = copyValues(c.Rects)
		c.Brush = cloneBrush(c.Brush)
		return c, nil
	case DrawImage:
		return c, nil
	case DrawBlurredShadow:
		c.Path = clonePath(c.Path)
		return c, nil
	default:
		return nil, errors.New("gfx: unsupported command type " + commandType(command))
	}
}

func clonePath(p Path) Path { return Path{Segments: copyValues(p.Segments)} }

func cloneStroke(s StrokeStyle) StrokeStyle { s.Dash = copyValues(s.Dash); return s }

func cloneBrush(b Brush) Brush {
	b.GradientStops = copyValues(b.GradientStops)
	return b
}

func cloneString(s string) string { return strings.Clone(s) }

func clearCommandReferences(command Command) {
	switch c := command.(type) {
	case FillRect:
		clear(c.Brush.GradientStops)
	case StrokeRect:
		clear(c.Stroke.Dash)
		clear(c.Brush.GradientStops)
	case FillPath:
		clear(c.Path.Segments)
		clear(c.Brush.GradientStops)
	case StrokePath:
		clear(c.Path.Segments)
		clear(c.Stroke.Dash)
		clear(c.Brush.GradientStops)
	case DrawPolyline:
		clear(c.Points)
		clear(c.Stroke.Dash)
		clear(c.Brush.GradientStops)
	case DrawPoints:
		clear(c.Points)
		clear(c.Brush.GradientStops)
	case DrawGlyphRun:
		clear(c.Run.Glyphs)
		clear(c.Run.GraphemeAdvances)
		clear(c.Brush.GradientStops)
	case DrawSelectionRects:
		clear(c.Rects)
		clear(c.Brush.GradientStops)
	case DrawBlurredShadow:
		clear(c.Path.Segments)
	}
}

func addBytes(total *uint64, n uint64) error {
	if ^uint64(0)-*total < n {
		return errors.New("gfx: command page byte estimate overflows")
	}
	*total += n
	return nil
}

func checkedSizeProduct(count, size uint64) (uint64, error) {
	if count != 0 && size > ^uint64(0)/count {
		return 0, errors.New("gfx: command page size multiplication overflows")
	}
	return count * size, nil
}

type sliceSize struct {
	count int
	size  uintptr
}

func sumSliceSizes(base uintptr, slices ...sliceSize) (uint64, error) {
	total := uint64(base)
	for _, slice := range slices {
		if slice.count < 0 || uint64(slice.count) > ^uint64(0)/uint64(slice.size) {
			return 0, errors.New("gfx: command payload size multiplication overflows")
		}
		if err := addBytes(&total, uint64(slice.count)*uint64(slice.size)); err != nil {
			return 0, err
		}
	}
	return total, nil
}

func commandType(command Command) string {
	return fmt.Sprintf("%T", command)
}

func copyValues[T any](source []T) []T {
	if len(source) == 0 {
		return nil
	}
	owned := make([]T, len(source))
	copy(owned, source)
	return owned
}
