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
	ledger                        *walletLedger
	parent                        *Wallet
	partitioned                   bool
	cap, reserved, live, retiring uint64
}

type walletLedger struct {
	mu          sync.Mutex
	nextID      uint64
	allocations map[uint64]allocation
}

type allocationState uint8

const (
	allocationReserved allocationState = iota + 1
	allocationLive
	allocationRetiring
)

type allocation struct {
	owner *Wallet
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
	return &Wallet{cap: capacity, ledger: &walletLedger{allocations: make(map[uint64]allocation)}}, nil
}

// Partition fixes child capacities before the first reservation. Children
// cannot borrow from one another, and the parent becomes aggregate-only.
// All descendants share one allocation identity sequence and one ledger, so
// a delayed acknowledgment cannot confuse equal-size allocations in siblings.
func (w *Wallet) Partition(capacities ...uint64) ([]*Wallet, error) {
	if w == nil || len(capacities) == 0 {
		return nil, errors.New("membudget: invalid wallet partition")
	}
	w.ledger.mu.Lock()
	defer w.ledger.mu.Unlock()
	if w.partitioned || w.reserved != 0 || w.live != 0 || w.retiring != 0 || w.ledger.nextID != 0 {
		return nil, errors.New("membudget: wallet partition must precede reservations")
	}
	var total uint64
	for _, capacity := range capacities {
		if capacity == 0 || capacity > w.cap-total {
			return nil, errors.New("membudget: partition capacities exceed wallet")
		}
		total += capacity
	}
	if total != w.cap {
		return nil, errors.New("membudget: partition must assign the complete wallet")
	}
	children := make([]*Wallet, len(capacities))
	for i, capacity := range capacities {
		children[i] = &Wallet{cap: capacity, ledger: w.ledger, parent: w}
	}
	w.partitioned = true
	return children, nil
}

func (w *Wallet) Reserve(bytes uint64) (*Reservation, error) {
	if w == nil {
		return nil, errors.New("membudget: nil wallet")
	}
	if bytes == 0 {
		return nil, errors.New("membudget: zero-byte reservation is invalid")
	}
	w.ledger.mu.Lock()
	defer w.ledger.mu.Unlock()
	if err := w.checkCapacityLocked(bytes, 1); err != nil {
		return nil, err
	}
	return w.claimLocked(bytes), nil
}

// ReserveBatch admits every allocation atomically. It is used when one frame
// targets several windows, so a later refusal cannot leave a partial set.
func (w *Wallet) ReserveBatch(sizes []uint64) ([]*Reservation, error) {
	if w == nil || len(sizes) == 0 {
		return nil, errors.New("membudget: invalid batch reservation")
	}
	w.ledger.mu.Lock()
	defer w.ledger.mu.Unlock()
	var total uint64
	for _, size := range sizes {
		if size == 0 || size > math.MaxUint64-total {
			return nil, errors.New("membudget: invalid batch reservation size")
		}
		total += size
	}
	if err := w.checkCapacityLocked(total, uint64(len(sizes))); err != nil {
		return nil, err
	}
	result := make([]*Reservation, len(sizes))
	for i, size := range sizes {
		result[i] = w.claimLocked(size)
	}
	return result, nil
}

// ResourceLimit is an ordinary recoverable refusal made before allocation.
type ResourceLimit struct{ Requested, Used, Capacity uint64 }

func (e *ResourceLimit) Error() string {
	return fmt.Sprintf("membudget: resource limit: requested %d bytes with %d of %d charged", e.Requested, e.Used, e.Capacity)
}

func (w *Wallet) checkCapacityLocked(bytes, count uint64) error {
	if w.partitioned {
		return errors.New("membudget: partitioned wallet only reports aggregate charges")
	}
	used := w.reserved + w.live + w.retiring
	if used < w.reserved || used > w.cap || bytes > w.cap-used {
		return &ResourceLimit{Requested: bytes, Used: used, Capacity: w.cap}
	}
	if count > math.MaxUint64-w.ledger.nextID {
		return errors.New("membudget: allocation ID space exhausted")
	}
	return nil
}
func (w *Wallet) claimLocked(bytes uint64) *Reservation {
	w.ledger.nextID++
	w.ledger.allocations[w.ledger.nextID] = allocation{owner: w, bytes: bytes, state: allocationReserved}
	w.addChargeLocked(allocationReserved, bytes)
	return &Reservation{wallet: w, id: w.ledger.nextID}
}

func (w *Wallet) addChargeLocked(state allocationState, bytes uint64) {
	for owner := w; owner != nil; owner = owner.parent {
		switch state {
		case allocationReserved:
			owner.reserved += bytes
		case allocationLive:
			owner.live += bytes
		case allocationRetiring:
			owner.retiring += bytes
		}
	}
}

func (w *Wallet) removeChargeLocked(state allocationState, bytes uint64) {
	for owner := w; owner != nil; owner = owner.parent {
		switch state {
		case allocationReserved:
			owner.reserved -= bytes
		case allocationLive:
			owner.live -= bytes
		case allocationRetiring:
			owner.retiring -= bytes
		}
	}
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
	r.wallet.ledger.mu.Lock()
	defer r.wallet.ledger.mu.Unlock()
	entry, ok := r.wallet.ledger.allocations[r.id]
	if !ok || entry.owner != r.wallet {
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
	w.ledger.mu.Lock()
	defer w.ledger.mu.Unlock()
	entry, ok := w.ledger.allocations[r.id]
	if !ok || entry.owner != w || entry.state != allocationReserved {
		return errors.New("membudget: cancel requires a live reserved allocation")
	}
	w.removeChargeLocked(allocationReserved, entry.bytes)
	delete(w.ledger.allocations, r.id)
	return nil
}

func (r *Reservation) Release() error {
	if r == nil || r.wallet == nil {
		return errors.New("membudget: nil reservation")
	}
	w := r.wallet
	w.ledger.mu.Lock()
	defer w.ledger.mu.Unlock()
	entry, ok := w.ledger.allocations[r.id]
	if !ok || entry.owner != w || entry.state != allocationLive {
		return errors.New("membudget: release requires a live allocation")
	}
	w.removeChargeLocked(allocationLive, entry.bytes)
	delete(w.ledger.allocations, r.id)
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
	w.ledger.mu.Lock()
	defer w.ledger.mu.Unlock()
	entry, ok := w.ledger.allocations[r.id]
	if !ok || entry.owner != w || entry.state != allocationRetiring {
		return errors.New("membudget: acknowledgment requires a retiring allocation")
	}
	w.removeChargeLocked(allocationRetiring, entry.bytes)
	delete(w.ledger.allocations, r.id)
	return nil
}

func (r *Reservation) transition(from, to allocationState) error {
	if r == nil || r.wallet == nil {
		return errors.New("membudget: nil reservation")
	}
	w := r.wallet
	w.ledger.mu.Lock()
	defer w.ledger.mu.Unlock()
	entry, ok := w.ledger.allocations[r.id]
	if !ok || entry.owner != w || entry.state != from {
		return errors.New("membudget: invalid allocation state transition")
	}
	w.removeChargeLocked(from, entry.bytes)
	w.addChargeLocked(to, entry.bytes)
	entry.state = to
	w.ledger.allocations[r.id] = entry
	return nil
}

func (w *Wallet) Snapshot() (capacity, reserved, live, retiring uint64) {
	if w == nil {
		return
	}
	w.ledger.mu.Lock()
	defer w.ledger.mu.Unlock()
	return w.cap, w.reserved, w.live, w.retiring
}
