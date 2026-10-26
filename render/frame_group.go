package render

import (
	"errors"
	"sync/atomic"
)

const MaxFrameWindows = 8
const PrimaryWindow = "__primary__"

// WindowDescriptor identifies one immutable window packet and its surface.
type WindowDescriptor struct {
	ID                 string
	SurfaceGeneration  uint64
	ResourceGeneration uint64
}

type frameGroupStorage struct {
	frames [MaxFrameWindows]*Frame
	count  int
	refs   atomic.Int64
}

type frameGroupLease struct {
	storage  *frameGroupStorage
	released atomic.Bool
}

// FrameGroup owns the complete window set for one logical frame. A failed
// window prevents readiness of the group; each consumer owns a distinct lease.
type FrameGroup struct{ lease *frameGroupLease }

// TakeFrameGroup transfers the supplied frame leases after validating the
// complete window directory. On error ownership stays with the caller.
func TakeFrameGroup(frames []*Frame) (*FrameGroup, error) {
	if len(frames) == 0 || len(frames) > MaxFrameWindows {
		return nil, errors.New("render: invalid frame window count")
	}
	for i, frame := range frames {
		if frame.storage() == nil {
			return nil, errors.New("render: frame group contains an unavailable frame")
		}
		for j := 0; j < i; j++ {
			if frames[j].WindowID() == frame.WindowID() {
				return nil, errors.New("render: duplicate frame window")
			}
		}
	}
	s := &frameGroupStorage{count: len(frames)}
	copy(s.frames[:], frames)
	s.refs.Store(1)
	return &FrameGroup{lease: &frameGroupLease{storage: s}}, nil
}

func (g *FrameGroup) storage() *frameGroupStorage {
	if g == nil || g.lease == nil || g.lease.released.Load() {
		return nil
	}
	return g.lease.storage
}
func (g *FrameGroup) WindowCount() int {
	if s := g.storage(); s != nil {
		return s.count
	}
	return 0
}
func (g *FrameGroup) Generation() uint64 {
	if s := g.storage(); s != nil {
		return s.frames[0].Generation()
	}
	return 0
}
func (g *FrameGroup) WindowAt(index int) (WindowDescriptor, bool) {
	s := g.storage()
	if s == nil || index < 0 || index >= s.count {
		return WindowDescriptor{}, false
	}
	f := s.frames[index]
	return WindowDescriptor{ID: f.WindowID(), SurfaceGeneration: f.SurfaceGeneration(), ResourceGeneration: f.Generation()}, true
}
func (g *FrameGroup) RetainWindow(index int) (*Frame, error) {
	s := g.storage()
	if s == nil || index < 0 || index >= s.count {
		return nil, errors.New("render: unavailable group window")
	}
	return s.frames[index].Retain()
}
func (g *FrameGroup) Retain() (*FrameGroup, error) {
	s := g.storage()
	if s == nil {
		return nil, errors.New("render: unavailable frame group")
	}
	for {
		refs := s.refs.Load()
		if refs <= 0 || refs >= maxFrameLeases {
			return nil, errors.New("render: frame group lease limit")
		}
		if s.refs.CompareAndSwap(refs, refs+1) {
			return &FrameGroup{lease: &frameGroupLease{storage: s}}, nil
		}
	}
}
func (g *FrameGroup) Release() error {
	if g == nil || g.lease == nil || !g.lease.released.CompareAndSwap(false, true) {
		return errors.New("render: duplicate or invalid frame group release")
	}
	s := g.lease.storage
	if s.refs.Add(-1) != 0 {
		return nil
	}
	var result error
	for i := 0; i < s.count; i++ {
		if err := s.frames[i].Release(); err != nil && result == nil {
			result = err
		}
		s.frames[i] = nil
	}
	s.count = 0
	return result
}
