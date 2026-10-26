package store

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

type scopeState uint8

const (
	scopeConstructing scopeState = iota
	scopeBound
	scopeClosing
	scopeClosed
)

type queuedSignal struct {
	generation uint64
	deliver    func()
}

type scopeResource interface {
	closeScopeResource()
}

type releaseResource struct{ release func() }

func (r *releaseResource) closeScopeResource() {
	if r.release != nil {
		r.release()
		r.release = nil
	}
}

type derivedFlushable interface {
	flushIfDirty() bool
	scopeRank() uint64
	scopeSequence() uint64
}

type scopeReadGuard struct {
	mu    sync.Mutex
	stack []any
}

type scopePhaseState struct {
	mu       sync.Mutex
	flushing bool
}

// FlushResult reports the work performed by one scope flush.
type FlushResult struct {
	Evaluated  int
	Recomputed int
}

// Scope owns reactive stores, derived computations, source subscriptions, and
// deferred signal deliveries. A scope is created before the objects it owns.
type Scope struct {
	mu         sync.Mutex
	state      scopeState
	generation uint64
	identity   *RuntimeIdentity
	parent     *Scope
	children   []*Scope
	closedCh   chan struct{}
	resources  []scopeResource
	sequence   uint64
	dirty      []derivedFlushable
	dirtySet   map[derivedFlushable]struct{}
	signals    []queuedSignal
	flushing   bool
	readGuard  *scopeReadGuard
	phaseState *scopePhaseState

	frameEvals      atomic.Uint64
	frameRecomputes atomic.Uint64
}

// NewScope creates an unbound application or standalone-test ownership scope.
func NewScope() *Scope {
	return &Scope{generation: 1, dirtySet: make(map[derivedFlushable]struct{}), closedCh: make(chan struct{}), readGuard: &scopeReadGuard{}, phaseState: &scopePhaseState{}}
}

// Child creates a scope whose resources end when either it or its parent closes.
func (s *Scope) Child() *Scope {
	if s == nil {
		panic("store: child scope requires an owner")
	}
	child := NewScope()
	child.parent = s
	s.mu.Lock()
	if s.state >= scopeClosing {
		s.mu.Unlock()
		panic("store: cannot add a child to a closing scope")
	}
	if s.state == scopeBound {
		child.state = scopeBound
		child.identity = s.identity
	}
	child.readGuard = s.readGuard
	child.phaseState = s.phaseState
	s.children = append(s.children, child)
	s.mu.Unlock()
	return child
}

// Bind associates the scope with one runtime identity. Rebinding to that same
// identity is idempotent; binding it to another runtime is an author error.
func (s *Scope) Bind(identity *RuntimeIdentity) {
	if s == nil || identity == nil {
		panic("store: scope binding requires a scope and runtime identity")
	}
	s.bindTree(identity)
}

func (s *Scope) bindTree(identity *RuntimeIdentity) {
	s.mu.Lock()
	if s.state >= scopeClosing {
		s.mu.Unlock()
		panic("store: cannot bind a closing scope")
	}
	if s.identity != nil && s.identity != identity {
		s.mu.Unlock()
		panic("store: scope is already bound to another runtime")
	}
	s.identity = identity
	s.state = scopeBound
	children := append([]*Scope(nil), s.children...)
	s.mu.Unlock()
	for _, child := range children {
		child.bindTree(identity)
	}
}

// RuntimeIdentity is an opaque, unique runtime ownership token.
type RuntimeIdentity struct{ marker byte }

// NewRuntimeIdentity creates a token used to bind one scope to one runtime.
func NewRuntimeIdentity() *RuntimeIdentity {
	return &RuntimeIdentity{marker: 1}
}

// Owns reports whether candidate is this scope or one of its descendants.
func (s *Scope) Owns(candidate *Scope) bool {
	return s != nil && candidate != nil && s.ancestorOf(candidate)
}

func (s *Scope) addResource(resource scopeResource) {
	if resource == nil {
		panic("store: nil scope resource")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state >= scopeClosing {
		panic("store: cannot register a resource in a closing scope")
	}
	s.resources = append(s.resources, resource)
}

// OwnRelease makes an external signal or source subscription part of this
// scope's lifetime. The release runs once during Close in reverse order.
func (s *Scope) OwnRelease(release func()) {
	if release == nil {
		return
	}
	s.addResource(&releaseResource{release: release})
}

func (s *Scope) ancestorOf(other *Scope) bool {
	for current := other; current != nil; current = current.parent {
		if current == s {
			return true
		}
	}
	return false
}

func requireScope(scope *Scope) {
	if scope == nil {
		panic("store: explicit owner scope is required")
	}
	scope.mu.Lock()
	closing := scope.state >= scopeClosing
	scope.mu.Unlock()
	if closing {
		panic("store: cannot construct a reactive value in a closing scope")
	}
}

func (s *Scope) enqueueSignal(deliver func()) {
	if deliver == nil {
		return
	}
	s.mu.Lock()
	if s.state >= scopeClosing {
		s.mu.Unlock()
		return
	}
	s.signals = append(s.signals, queuedSignal{generation: s.generation, deliver: deliver})
	s.mu.Unlock()
}

// QueueSignal schedules a callback for this scope's signal phase.
func (s *Scope) QueueSignal(deliver func()) { s.enqueueSignal(deliver) }

// FlushSignals delivers queued notifications in bounded same-phase batches.
// Notifications queued by a later frame phase remain until the next signal cut.
func (s *Scope) FlushSignals() int {
	if s == nil {
		return 0
	}
	const maxIterations = 16
	total := 0
	for iteration := 0; ; iteration++ {
		if iteration >= maxIterations {
			panic("store: signal delivery exceeded 16 batches; likely a signal cycle")
		}
		work := false
		s.visitTree(func(owner *Scope) {
			batch, generation := owner.takeSignalBatch()
			if len(batch) == 0 {
				return
			}
			work = true
			total += len(batch)
			owner.deliverSignalBatch(batch, generation)
		})
		if !work {
			return total
		}
	}
}

func (s *Scope) takeSignalBatch() ([]queuedSignal, uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state >= scopeClosing || len(s.signals) == 0 {
		return nil, s.generation
	}
	batch := s.signals
	s.signals = nil
	return batch, s.generation
}

func (s *Scope) deliverSignalBatch(batch []queuedSignal, generation uint64) {
	for i := range batch {
		s.mu.Lock()
		valid := s.state < scopeClosing && s.generation == generation && batch[i].generation == generation
		s.mu.Unlock()
		if valid && batch[i].deliver != nil {
			batch[i].deliver()
		}
		batch[i].deliver = nil
	}
}

func (s *Scope) registerDerived(derived derivedFlushable) uint64 {
	if derived == nil {
		panic("store: nil derived registration")
	}
	s.phaseState.mu.Lock()
	flushingTree := s.phaseState.flushing
	s.phaseState.mu.Unlock()
	if flushingTree {
		panic("store: derived registration during scope-tree flush")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state >= scopeClosing {
		panic("store: cannot register a derived value in a closing scope")
	}
	if s.flushing {
		panic("store: derived registration during flush")
	}
	s.sequence++
	if s.sequence == 0 {
		panic("store: scope sequence exhausted")
	}
	if s.dirtySet == nil {
		s.dirtySet = make(map[derivedFlushable]struct{})
	}
	s.dirty = append(s.dirty, derived)
	s.dirtySet[derived] = struct{}{}
	return s.sequence
}

func (s *Scope) enqueueDerived(derived derivedFlushable) {
	if s == nil || derived == nil {
		return
	}
	s.mu.Lock()
	if s.state < scopeClosing {
		if _, exists := s.dirtySet[derived]; !exists {
			s.dirty = append(s.dirty, derived)
			s.dirtySet[derived] = struct{}{}
		}
	}
	s.mu.Unlock()
}

// FlushDerived recomputes only dirty computations in declared dependency order.
func (s *Scope) FlushDerived() FlushResult {
	if s == nil {
		return FlushResult{}
	}
	s.phaseState.mu.Lock()
	if s.phaseState.flushing {
		s.phaseState.mu.Unlock()
		panic("store: recursive derived flush")
	}
	s.phaseState.flushing = true
	s.phaseState.mu.Unlock()
	defer func() {
		s.phaseState.mu.Lock()
		s.phaseState.flushing = false
		s.phaseState.mu.Unlock()
	}()
	var result FlushResult
	s.visitTree(func(owner *Scope) {
		local := owner.flushDerivedLocal()
		result.Evaluated += local.Evaluated
		result.Recomputed += local.Recomputed
	})
	return result
}

func (s *Scope) flushDerivedLocal() (result FlushResult) {
	s.mu.Lock()
	if s.state >= scopeClosing {
		s.mu.Unlock()
		return result
	}
	if s.flushing {
		s.mu.Unlock()
		panic("store: recursive derived flush")
	}
	s.flushing = true
	work := s.dirty
	s.dirty = nil
	s.dirtySet = make(map[derivedFlushable]struct{})
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.flushing = false
		s.mu.Unlock()
	}()
	sort.SliceStable(work, func(i, j int) bool {
		if work[i].scopeRank() != work[j].scopeRank() {
			return work[i].scopeRank() < work[j].scopeRank()
		}
		return work[i].scopeSequence() < work[j].scopeSequence()
	})
	for _, derived := range work {
		if derived.flushIfDirty() {
			result.Evaluated++
			result.Recomputed++
		}
	}
	return result
}

func (s *Scope) visitTree(visit func(*Scope)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	live := s.state < scopeClosing
	s.mu.Unlock()
	if !live {
		return
	}
	visit(s)
	for index := 0; ; {
		s.mu.Lock()
		if s.state >= scopeClosing || index >= len(s.children) {
			s.mu.Unlock()
			return
		}
		child := s.children[index]
		s.mu.Unlock()
		child.visitTree(visit)
		s.mu.Lock()
		if index < len(s.children) && s.children[index] == child {
			index++
		}
		s.mu.Unlock()
	}
}

// ResetFrameDerivedCounters starts this scope's frame-local activity sample.
func (s *Scope) ResetFrameDerivedCounters() {
	if s == nil {
		return
	}
	s.frameEvals.Store(0)
	s.frameRecomputes.Store(0)
}

// FrameDerivedCounters returns this scope's derived activity since reset.
func (s *Scope) FrameDerivedCounters() (uint64, uint64) {
	if s == nil {
		return 0, 0
	}
	return s.frameEvals.Load(), s.frameRecomputes.Load()
}

// ResetFrameDerivedCountersTree resets counters for this scope and every live
// child scope. Runtime owners use this at their frame boundary.
func (s *Scope) ResetFrameDerivedCountersTree() {
	s.visitTree(func(owner *Scope) {
		owner.ResetFrameDerivedCounters()
	})
}

// FrameDerivedCountersTree returns the runtime-wide sum of per-scope counters.
func (s *Scope) FrameDerivedCountersTree() (evaluations, recomputations uint64) {
	s.visitTree(func(owner *Scope) {
		e, r := owner.FrameDerivedCounters()
		evaluations += e
		recomputations += r
	})
	return evaluations, recomputations
}

func (s *Scope) recordDerivedGet(recomputed bool) {
	s.frameEvals.Add(1)
	if recomputed {
		s.frameRecomputes.Add(1)
	}
}

func (s *Scope) recordDerivedRecompute() { s.frameRecomputes.Add(1) }

func (s *Scope) pushComputing(value any) {
	guard := s.readGuard
	guard.mu.Lock()
	for _, current := range guard.stack {
		if current == value {
			guard.mu.Unlock()
			panic("store: derived dependency cycle while computing")
		}
	}
	guard.stack = append(guard.stack, value)
	guard.mu.Unlock()
}

func (s *Scope) popComputing(value any) {
	guard := s.readGuard
	guard.mu.Lock()
	if len(guard.stack) == 0 || guard.stack[len(guard.stack)-1] != value {
		guard.mu.Unlock()
		panic("store: derived compute stack corruption")
	}
	guard.stack[len(guard.stack)-1] = nil
	guard.stack = guard.stack[:len(guard.stack)-1]
	guard.mu.Unlock()
}

func (s *Scope) assertDerivedReadAllowed(source any) {
	s.assertUsable()
	guard := s.readGuard
	guard.mu.Lock()
	if len(guard.stack) == 0 {
		guard.mu.Unlock()
		return
	}
	current := guard.stack[len(guard.stack)-1]
	guard.mu.Unlock()
	reader, ok := current.(interface{ declares(any) bool })
	if !ok || !reader.declares(source) {
		panic(fmt.Sprintf("store: derived computation read undeclared dependency %T", source))
	}
}

func (s *Scope) assertUsable() {
	if s == nil {
		panic("store: reactive value requires an explicit owner scope")
	}
	s.mu.Lock()
	closed := s.state >= scopeClosing
	s.mu.Unlock()
	if closed {
		panic("store: reactive value used after its owner scope closed")
	}
}

// Close fences queued deliveries, closes child scopes in reverse order, and
// releases every owned reactive graph. It is safe to call more than once.
func (s *Scope) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.state == scopeClosed {
		s.mu.Unlock()
		return
	}
	if s.state == scopeClosing {
		closed := s.closedCh
		s.mu.Unlock()
		<-closed
		return
	}
	s.state = scopeClosing
	s.generation++
	if s.generation == 0 {
		s.mu.Unlock()
		panic("store: scope generation exhausted")
	}
	for i := range s.signals {
		s.signals[i].deliver = nil
	}
	s.signals = nil
	for i := range s.dirty {
		s.dirty[i] = nil
	}
	s.dirty = nil
	s.dirtySet = nil
	children := s.children
	s.children = nil
	resources := s.resources
	s.resources = nil
	s.mu.Unlock()

	for i := len(children) - 1; i >= 0; i-- {
		children[i].Close()
	}
	for i := len(resources) - 1; i >= 0; i-- {
		resources[i].closeScopeResource()
		resources[i] = nil
	}
	s.mu.Lock()
	s.state = scopeClosed
	parent := s.parent
	s.parent = nil
	close(s.closedCh)
	s.mu.Unlock()
	if parent != nil {
		parent.removeChild(s)
	}
}

func (s *Scope) removeChild(child *Scope) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, current := range s.children {
		if current != child {
			continue
		}
		copy(s.children[i:], s.children[i+1:])
		s.children[len(s.children)-1] = nil
		s.children = s.children[:len(s.children)-1]
		return
	}
}
