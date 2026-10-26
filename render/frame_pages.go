package render

import (
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/membudget"
)

// FrameInput is mutable assembly data. SealFrame copies it into a single
// immutable command page and transfers the supplied reservation to that page.
type FrameInput struct {
	Batches      []BatchInput
	Layers       []LayerInput
	DirtyRegions []gfx.Rect
}

type BatchInput struct {
	ID       RenderBatchID
	Bounds   gfx.Rect
	Opacity  float32
	Commands gfx.CommandList
}

type LayerInput struct {
	RenderOrder int
	ClipRect    gfx.Rect
	FirstBatch  int
	BatchCount  int
}

// FrameBatch contains only immutable metadata. Its commands stay in the
// frame-owned gfx command page and are visited through Frame.VisitBatchCommands.
type FrameBatch struct {
	ID      RenderBatchID
	Bounds  gfx.Rect
	Opacity float32
}

// FrameLayer refers to a contiguous range in the frame's one batch directory.
type FrameLayer struct {
	RenderOrder int
	ClipRect    gfx.Rect
	FirstBatch  int
	BatchCount  int
}

type frameStorage struct {
	generation    uint64
	page          *gfx.CommandPage
	batches       []FrameBatch
	commandRanges []gfx.CommandRange
	layers        []FrameLayer
	dirty         []gfx.Rect
	reservation   *membudget.Reservation
	refs          atomic.Int64
}

type frameLease struct {
	storage  *frameStorage
	released atomic.Bool
}

// Frame is a lease on immutable frame pages. A caller that retains the frame
// beyond its current consumer must call Retain and later Release that lease.
type Frame struct{ lease *frameLease }

const maxFrameLeases = 8

func EstimateFrameBytes(input FrameInput) (uint64, error) {
	commandCount := 0
	var total uint64
	if err := addFrameBytes(&total, uint64(unsafe.Sizeof(frameStorage{}))); err != nil {
		return 0, err
	}
	if err := addFrameBytes(&total, uint64(unsafe.Sizeof(Frame{}))); err != nil {
		return 0, err
	}
	leaseBytes, err := checkedFrameProduct(maxFrameLeases, unsafe.Sizeof(frameLease{}))
	if err != nil {
		return 0, err
	}
	if err := addFrameBytes(&total, leaseBytes); err != nil {
		return 0, err
	}
	for i := range input.Batches {
		if len(input.Batches[i].Commands.Commands) > math.MaxInt-commandCount {
			return 0, errors.New("render: command count overflow")
		}
		commandCount += len(input.Batches[i].Commands.Commands)
		bytes, err := gfx.EstimateCommandPageBytes(input.Batches[i].Commands.Commands)
		if err != nil {
			return 0, fmt.Errorf("render: batch %d: %w", i, err)
		}
		if err := addFrameBytes(&total, bytes); err != nil {
			return 0, err
		}
	}
	if err := addFrameSlice(&total, len(input.Batches), unsafe.Sizeof(FrameBatch{})); err != nil {
		return 0, err
	}
	if err := addFrameSlice(&total, len(input.Batches), unsafe.Sizeof(gfx.CommandRange{})); err != nil {
		return 0, err
	}
	if err := addFrameSlice(&total, len(input.Layers), unsafe.Sizeof(FrameLayer{})); err != nil {
		return 0, err
	}
	if err := addFrameSlice(&total, len(input.DirtyRegions), unsafe.Sizeof(gfx.Rect{})); err != nil {
		return 0, err
	}
	return total, nil
}

func addFrameSlice(total *uint64, count int, elementSize uintptr) error {
	if count < 0 || uint64(count) > ^uint64(0)/uint64(elementSize) {
		return errors.New("render: frame metadata size overflows")
	}
	return addFrameBytes(total, uint64(count)*uint64(elementSize))
}

func checkedFrameProduct(count int, elementSize uintptr) (uint64, error) {
	if count < 0 || elementSize != 0 && uint64(count) > ^uint64(0)/uint64(elementSize) {
		return 0, errors.New("render: frame backing size multiplication overflows")
	}
	return uint64(count) * uint64(elementSize), nil
}

// SealFrame deep-copies all command payloads and metadata into one charged
// immutable frame generation. The caller must reserve EstimateFrameBytes
// before invoking this function. Ownership of reservation transfers on success.
func SealFrame(input FrameInput, reservation *membudget.Reservation) (*Frame, error) {
	if reservation == nil {
		return nil, errors.New("render: frame sealing requires a resource reservation")
	}
	estimate, err := EstimateFrameBytes(input)
	if err != nil {
		return nil, err
	}
	if estimate > reservation.Bytes() {
		return nil, fmt.Errorf("render: frame needs %d bytes, reservation has %d", estimate, reservation.Bytes())
	}
	commandCount := 0
	for i := range input.Batches {
		if len(input.Batches[i].Commands.Commands) > math.MaxInt-commandCount {
			return nil, errors.New("render: command count overflow")
		}
		commandCount += len(input.Batches[i].Commands.Commands)
	}
	batches := make([]FrameBatch, len(input.Batches))
	ranges := make([]gfx.CommandRange, len(input.Batches))
	for i := range input.Batches {
		batch := input.Batches[i]
		batches[i] = FrameBatch{ID: batch.ID, Bounds: batch.Bounds, Opacity: batch.Opacity}
	}
	page, err := gfx.SealCommandPageFromSource(commandCount, func(yield func(gfx.Command) bool) {
		for i := range input.Batches {
			for _, command := range input.Batches[i].Commands.Commands {
				if !yield(command) {
					return
				}
			}
		}
	}, reservation)
	if err != nil {
		return nil, err
	}
	start := 0
	for i := range input.Batches {
		ranges[i], err = page.Range(start, len(input.Batches[i].Commands.Commands))
		if err != nil {
			_ = page.Release()
			return nil, err
		}
		start += len(input.Batches[i].Commands.Commands)
	}
	layers := make([]FrameLayer, len(input.Layers))
	for i, layer := range input.Layers {
		if layer.FirstBatch < 0 || layer.BatchCount < 0 || layer.FirstBatch > len(batches) || layer.BatchCount > len(batches)-layer.FirstBatch {
			_ = page.Release()
			return nil, fmt.Errorf("render: layer %d batch range is invalid", i)
		}
		layers[i] = FrameLayer(layer)
	}
	if err := reservation.Publish(); err != nil {
		_ = page.Release()
		return nil, fmt.Errorf("render: publish frame reservation: %w", err)
	}
	storage := &frameStorage{generation: reservation.ID(), page: page, batches: batches, commandRanges: ranges, layers: layers, dirty: append([]gfx.Rect(nil), input.DirtyRegions...), reservation: reservation}
	storage.refs.Store(1)
	return &Frame{lease: &frameLease{storage: storage}}, nil
}

func addFrameBytes(total *uint64, n uint64) error {
	if ^uint64(0)-*total < n {
		return errors.New("render: frame backing size overflows")
	}
	*total += n
	return nil
}

func (f *Frame) storage() *frameStorage {
	if f == nil || f.lease == nil || f.lease.released.Load() {
		return nil
	}
	return f.lease.storage
}

func (f *Frame) BatchCount() int {
	if s := f.storage(); s != nil {
		return len(s.batches)
	}
	return 0
}

func (f *Frame) BatchAt(index int) (FrameBatch, bool) {
	s := f.storage()
	if s == nil || index < 0 || index >= len(s.batches) {
		return FrameBatch{}, false
	}
	return s.batches[index], true
}

func (f *Frame) LayerCount() int {
	if s := f.storage(); s != nil {
		return len(s.layers)
	}
	return 0
}

func (f *Frame) LayerAt(index int) (FrameLayer, bool) {
	s := f.storage()
	if s == nil || index < 0 || index >= len(s.layers) {
		return FrameLayer{}, false
	}
	return s.layers[index], true
}

func (f *Frame) DirtyRegionCount() int {
	if s := f.storage(); s != nil {
		return len(s.dirty)
	}
	return 0
}

func (f *Frame) DirtyRegionAt(index int) (gfx.Rect, bool) {
	s := f.storage()
	if s == nil || index < 0 || index >= len(s.dirty) {
		return gfx.Rect{}, false
	}
	return s.dirty[index], true
}

func (f *Frame) VisitBatchCommands(index int, visit func(gfx.Command) bool) bool {
	s := f.storage()
	if s == nil || index < 0 || index >= len(s.commandRanges) {
		return false
	}
	s.commandRanges[index].Visit(visit)
	return true
}

func (f *Frame) BatchCommandCount(index int) (int, bool) {
	s := f.storage()
	if s == nil || index < 0 || index >= len(s.commandRanges) {
		return 0, false
	}
	return s.commandRanges[index].Len(), true
}

// AppendBatchCommands copies command interface values into caller-owned
// scratch; nested payloads remain immutable frame-page values.
func (f *Frame) AppendBatchCommands(index int, dst []gfx.Command) ([]gfx.Command, bool) {
	s := f.storage()
	if s == nil || index < 0 || index >= len(s.commandRanges) {
		return dst, false
	}
	s.commandRanges[index].Visit(func(command gfx.Command) bool { dst = append(dst, command); return true })
	return dst, true
}

func (f *Frame) BackingBytes() uint64 {
	if s := f.storage(); s != nil {
		return s.reservation.Bytes()
	}
	return 0
}

// Generation is the unique resource-allocation generation backing this frame.
// It is stable across retained leases and cannot be reused by the frame wallet.
func (f *Frame) Generation() uint64 {
	if s := f.storage(); s != nil {
		return s.generation
	}
	return 0
}

// Retain creates a distinct lease on the same immutable generation.
func (f *Frame) Retain() (*Frame, error) {
	s := f.storage()
	if s == nil {
		return nil, errors.New("render: cannot retain released frame")
	}
	for {
		refs := s.refs.Load()
		if refs <= 0 {
			return nil, errors.New("render: frame generation is retired")
		}
		if refs >= maxFrameLeases {
			return nil, errors.New("render: frame generation lease limit reached")
		}
		if s.refs.CompareAndSwap(refs, refs+1) {
			return &Frame{lease: &frameLease{storage: s}}, nil
		}
	}
}

// Release relinquishes this lease. The final release clears pointer-bearing
// page storage and acknowledges its wallet reservation exactly once.
func (f *Frame) Release() error {
	if f == nil || f.lease == nil {
		return errors.New("render: invalid frame lease")
	}
	if !f.lease.released.CompareAndSwap(false, true) {
		return errors.New("render: duplicate frame lease release")
	}
	s := f.lease.storage
	if s.refs.Add(-1) != 0 {
		return nil
	}
	clear(s.batches)
	clear(s.commandRanges)
	clear(s.layers)
	clear(s.dirty)
	s.batches, s.commandRanges, s.layers, s.dirty = nil, nil, nil, nil
	pageErr := s.page.Release()
	s.page = nil
	if err := s.reservation.Retire(); err != nil {
		return err
	}
	if err := s.reservation.Acknowledge(); err != nil {
		return err
	}
	return pageErr
}
