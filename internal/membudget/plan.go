// Package membudget owns checked, fixed-capacity reservations for framework
// resources. Wallets are independent: an unused wallet cannot be borrowed.
package membudget

import (
	"errors"
	"fmt"
	"math"
	"sync"
)

type Profile uint8

const (
	Compact Profile = iota
	Standard
)

func (p Profile) String() string {
	if p == Compact {
		return "compact"
	}
	if p == Standard {
		return "standard"
	}
	return "unknown"
}

type Kind uint8

const (
	Text Kind = iota
	Projection
	SoftwareRaster
	AssetsCPU
	GPU
	Jobs
	Frame
	Diagnostics
	kindCount
)

type Plan struct {
	Profile Profile
	Caps    [kindCount]uint64
}

func DefaultPlan(profile Profile) Plan {
	miB := uint64(1 << 20)
	perOwner := [kindCount]uint64{48, 32, 48, 96, 128, 64, 64, 32}
	if profile == Compact {
		perOwner = [kindCount]uint64{12, 8, 12, 24, 32, 16, 16, 8}
	}
	p := Plan{Profile: profile}
	for i, n := range perOwner {
		p.Caps[i] = n * miB
	}
	return p
}

func (p Plan) Validate() error {
	if p.Profile != Compact && p.Profile != Standard {
		return errors.New("membudget: invalid resource profile")
	}
	var total uint64
	for i, cap := range p.Caps {
		if cap == 0 {
			return fmt.Errorf("membudget: wallet %d has zero capacity", i)
		}
		if math.MaxUint64-total < cap {
			return errors.New("membudget: aggregate capacity overflows uint64")
		}
		total += cap
	}
	return nil
}

type Wallet struct {
	mu                            sync.Mutex
	cap, reserved, live, retiring uint64
	nextID                        uint64
	allocations                   map[uint64]allocation
}

type allocationState uint8

const (
	allocationReserved allocationState = iota + 1
	allocationLive
	allocationRetiring
)

type allocation struct {
	bytes uint64
	state allocationState
}

// Reservation is the unique ownership token for one wallet allocation.
// State transitions are validated against the wallet, so a duplicate or stale
// release cannot free another allocation with the same byte size.
type Reservation struct {
	wallet *Wallet
	id     uint64
}

func NewWallet(capacity uint64) (*Wallet, error) {
	if capacity == 0 {
		return nil, errors.New("membudget: wallet capacity must be positive")
	}
	return &Wallet{cap: capacity, allocations: make(map[uint64]allocation)}, nil
}

func (w *Wallet) Reserve(bytes uint64) (*Reservation, error) {
	if w == nil {
		return nil, errors.New("membudget: nil wallet")
	}
	if bytes == 0 {
		return nil, errors.New("membudget: zero-byte reservation is invalid")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	used := w.reserved + w.live + w.retiring
	if used < w.reserved || bytes > w.cap-used {
		return nil, fmt.Errorf("membudget: resource limit: requested %d bytes with %d of %d charged", bytes, used, w.cap)
	}
	if w.nextID == math.MaxUint64 {
		return nil, errors.New("membudget: allocation ID space exhausted")
	}
	w.nextID++
	w.allocations[w.nextID] = allocation{bytes: bytes, state: allocationReserved}
	w.reserved += bytes
	return &Reservation{wallet: w, id: w.nextID}, nil
}

func (r *Reservation) ID() uint64 {
	if r == nil {
		return 0
	}
	return r.id
}

// Bytes reports the exact reserved backing capacity assigned to this token.
func (r *Reservation) Bytes() uint64 {
	if r == nil || r.wallet == nil {
		return 0
	}
	r.wallet.mu.Lock()
	defer r.wallet.mu.Unlock()
	entry, ok := r.wallet.allocations[r.id]
	if !ok {
		return 0
	}
	return entry.bytes
}

func (r *Reservation) Publish() error {
	return r.transition(allocationReserved, allocationLive)
}

func (r *Reservation) Cancel() error {
	if r == nil || r.wallet == nil {
		return errors.New("membudget: nil reservation")
	}
	w := r.wallet
	w.mu.Lock()
	defer w.mu.Unlock()
	entry, ok := w.allocations[r.id]
	if !ok || entry.state != allocationReserved {
		return errors.New("membudget: cancel requires a live reserved allocation")
	}
	w.reserved -= entry.bytes
	delete(w.allocations, r.id)
	return nil
}

func (r *Reservation) Release() error {
	if r == nil || r.wallet == nil {
		return errors.New("membudget: nil reservation")
	}
	w := r.wallet
	w.mu.Lock()
	defer w.mu.Unlock()
	entry, ok := w.allocations[r.id]
	if !ok || entry.state != allocationLive {
		return errors.New("membudget: release requires a live allocation")
	}
	w.live -= entry.bytes
	delete(w.allocations, r.id)
	return nil
}

func (r *Reservation) Retire() error {
	return r.transition(allocationLive, allocationRetiring)
}

func (r *Reservation) Acknowledge() error {
	if r == nil || r.wallet == nil {
		return errors.New("membudget: nil reservation")
	}
	w := r.wallet
	w.mu.Lock()
	defer w.mu.Unlock()
	entry, ok := w.allocations[r.id]
	if !ok || entry.state != allocationRetiring {
		return errors.New("membudget: acknowledgment requires a retiring allocation")
	}
	w.retiring -= entry.bytes
	delete(w.allocations, r.id)
	return nil
}

func (r *Reservation) transition(from, to allocationState) error {
	if r == nil || r.wallet == nil {
		return errors.New("membudget: nil reservation")
	}
	w := r.wallet
	w.mu.Lock()
	defer w.mu.Unlock()
	entry, ok := w.allocations[r.id]
	if !ok || entry.state != from {
		return errors.New("membudget: invalid allocation state transition")
	}
	switch from {
	case allocationReserved:
		w.reserved -= entry.bytes
	case allocationLive:
		w.live -= entry.bytes
	}
	switch to {
	case allocationLive:
		w.live += entry.bytes
	case allocationRetiring:
		w.retiring += entry.bytes
	}
	entry.state = to
	w.allocations[r.id] = entry
	return nil
}

func (w *Wallet) Snapshot() (capacity, reserved, live, retiring uint64) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.cap, w.reserved, w.live, w.retiring
}
