package gesture

import (
	"sort"
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

// GesturePipeline manages active recognizers and deferred signal delivery.
type GesturePipeline struct {
	mu                 sync.RWMutex
	roles              map[facet.FacetID]GestureRole
	queue              []func()
	pointer            pointerState
	PlatformAdaptation PlatformAdaptation
}

type pointerState struct {
	active   bool
	startPos gfx.Point
	lastPos  gfx.Point
}

type recognizerDispatch struct {
	recognizer Recognizer
	touches    []Touch
	event      InputEvent
	order      int
	state      RecognizerState
}

// NewGesturePipeline constructs an empty gesture pipeline.
func NewGesturePipeline() *GesturePipeline {
	return &GesturePipeline{roles: make(map[facet.FacetID]GestureRole)}
}

// SetPlatformAdaptation configures desktop-specific gesture mappings.
func (p *GesturePipeline) SetPlatformAdaptation(mode PlatformAdaptation) {
	if p == nil {
		return
	}
	p.PlatformAdaptation = mode
}

// Register attaches a recognizer role to a facet ID.
func (p *GesturePipeline) Register(id facet.FacetID, role GestureRole) {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.roles == nil {
		p.roles = make(map[facet.FacetID]GestureRole)
	}
	clone := GestureRole{Recognizers: append([]Recognizer(nil), role.Recognizers...)}
	p.roles[id] = clone
	p.mu.Unlock()
}

// Unregister removes any recognizers registered for the facet.
func (p *GesturePipeline) Unregister(id facet.FacetID) {
	if p == nil {
		return
	}
	p.mu.Lock()
	delete(p.roles, id)
	p.mu.Unlock()
}

// Enqueue appends a deferred signal delivery callback.
func (p *GesturePipeline) Enqueue(fn func()) {
	if p == nil || fn == nil {
		return
	}
	p.mu.Lock()
	p.queue = append(p.queue, fn)
	p.mu.Unlock()
}

// DrainQueuedSignals executes and clears queued signal callbacks.
func (p *GesturePipeline) DrainQueuedSignals() {
	if p == nil {
		return
	}
	p.mu.Lock()
	queue := append([]func(){}, p.queue...)
	p.queue = p.queue[:0]
	p.mu.Unlock()
	for _, fn := range queue {
		if fn != nil {
			fn()
		}
	}
}

// Process routes one raw input event to the hit facet and its ancestors.
func (p *GesturePipeline) Process(event platform.Event, hitFacet facet.FacetID, tree facet.Tree) {
	if p == nil || tree == nil || hitFacet == 0 {
		return
	}
	switch raw := event.(type) {
	case platform.EventPointer:
		p.processPointer(raw, hitFacet, tree)
	case platform.EventScroll:
		p.processScroll(raw, hitFacet, tree)
	}
}

func (p *GesturePipeline) processPointer(raw platform.EventPointer, hitFacet facet.FacetID, tree facet.Tree) {
	touches, inputEvent := p.normalizePointer(raw)
	path := buildFacetPath(tree, hitFacet)
	if len(path) == 0 {
		return
	}

	dispatches := make([]recognizerDispatch, 0, 8)
	order := 0
	for _, node := range path {
		if node == nil || node.Base() == nil {
			continue
		}
		role := p.roleForFacet(node.Base().ID())
		if len(role.Recognizers) == 0 {
			continue
		}
		recogs := append([]Recognizer(nil), role.Recognizers...)
		sort.SliceStable(recogs, func(i, j int) bool {
			di := recogs[i].Delegate()
			dj := recogs[j].Delegate()
			if di.Priority != dj.Priority {
				return di.Priority > dj.Priority
			}
			return i < j
		})
		for _, r := range recogs {
			if r == nil {
				continue
			}
			switch raw.Kind {
			case platform.PointerPress:
				r.TouchesBegan(touches, inputEvent)
			case platform.PointerMove:
				r.TouchesMoved(touches, inputEvent)
			case platform.PointerRelease:
				r.TouchesEnded(touches, inputEvent)
			case platform.PointerEnter, platform.PointerLeave:
				r.TouchesCancelled(touches, inputEvent)
			default:
				continue
			}
			dispatches = append(dispatches, recognizerDispatch{
				recognizer: r,
				touches:    append([]Touch(nil), touches...),
				event:      inputEvent,
				order:      order,
				state:      r.State(),
			})
			order++
		}
	}

	p.resolveCompetition(dispatches)
	p.queueRecognizerSignals(dispatches)
}

func (p *GesturePipeline) processScroll(raw platform.EventScroll, hitFacet facet.FacetID, tree facet.Tree) {
	if raw.Modifiers&platform.ModControl == 0 {
		return
	}
	path := buildFacetPath(tree, hitFacet)
	if len(path) == 0 {
		return
	}
	dispatches := make([]recognizerDispatch, 0, 4)
	inputEvent := InputEvent{Modifiers: raw.Modifiers, Timestamp: 0}
	for _, node := range path {
		if node == nil || node.Base() == nil {
			continue
		}
		role := p.roleForFacet(node.Base().ID())
		if len(role.Recognizers) == 0 {
			continue
		}
		for _, r := range role.Recognizers {
			if r == nil {
				continue
			}
			if handler, ok := r.(scrollGestureHandler); ok {
				handler.ScrollGesture(raw, inputEvent)
				dispatches = append(dispatches, recognizerDispatch{
					recognizer: r,
					event:      inputEvent,
				})
			}
		}
	}
	p.resolveCompetition(dispatches)
	p.queueRecognizerSignals(dispatches)
}

func (p *GesturePipeline) queueRecognizerSignals(dispatches []recognizerDispatch) {
	for _, d := range dispatches {
		if d.recognizer == nil {
			continue
		}
		if emitter, ok := d.recognizer.(SignalEmitter); ok {
			emitter.QueueSignals(p, d.touches, d.event)
		}
	}
}

func (p *GesturePipeline) roleForFacet(id facet.FacetID) GestureRole {
	p.mu.RLock()
	role := p.roles[id]
	p.mu.RUnlock()
	if len(role.Recognizers) == 0 {
		return GestureRole{}
	}
	return GestureRole{Recognizers: append([]Recognizer(nil), role.Recognizers...)}
}

func (p *GesturePipeline) normalizePointer(e platform.EventPointer) ([]Touch, InputEvent) {
	touch := Touch{
		ID:        0,
		Position:  e.Position,
		PrevPos:   p.pointer.lastPos,
		StartPos:  p.pointer.startPos,
		Force:     0,
		Timestamp: 0,
	}
	switch e.Kind {
	case platform.PointerPress:
		p.pointer.active = true
		p.pointer.startPos = e.Position
		p.pointer.lastPos = e.Position
		touch.PrevPos = e.Position
		touch.StartPos = e.Position
	case platform.PointerMove:
		touch.StartPos = p.pointer.startPos
		p.pointer.lastPos = e.Position
	case platform.PointerRelease:
		touch.StartPos = p.pointer.startPos
		p.pointer.lastPos = e.Position
		p.pointer.active = false
	case platform.PointerEnter, platform.PointerLeave:
		touch.StartPos = p.pointer.startPos
		p.pointer.lastPos = e.Position
	}
	return []Touch{touch}, InputEvent{
		Modifiers: e.Modifiers,
		Timestamp: 0,
	}
}

func (p *GesturePipeline) resolveCompetition(dispatches []recognizerDispatch) {
	if len(dispatches) == 0 {
		return
	}
	losers := make(map[Recognizer]struct{})
	for i := 0; i < len(dispatches); i++ {
		a := dispatches[i]
		if a.recognizer == nil {
			continue
		}
		for j := i + 1; j < len(dispatches); j++ {
			b := dispatches[j]
			if b.recognizer == nil {
				continue
			}
			if shouldRequireFailure(a.recognizer, b.recognizer) && !isFailedOrPossible(b.recognizer.State()) {
				losers[a.recognizer] = struct{}{}
				continue
			}
			if shouldRequireFailure(b.recognizer, a.recognizer) && !isFailedOrPossible(a.recognizer.State()) {
				losers[b.recognizer] = struct{}{}
				continue
			}
			if simultaneousAllowed(a.recognizer, b.recognizer) {
				continue
			}
			if a.recognizer.State() == RecognizerEnded && b.recognizer.State() == RecognizerEnded {
				da := a.recognizer.Delegate()
				db := b.recognizer.Delegate()
				switch {
				case da.Priority > db.Priority:
					losers[b.recognizer] = struct{}{}
				case db.Priority > da.Priority:
					losers[a.recognizer] = struct{}{}
				case a.order < b.order:
					losers[b.recognizer] = struct{}{}
				case b.order < a.order:
					losers[a.recognizer] = struct{}{}
				}
			}
		}
	}
	for r := range losers {
		if sm, ok := r.(interface{ Transition(RecognizerState) }); ok {
			sm.Transition(RecognizerFailed)
			continue
		}
		r.Reset()
	}
}

func shouldRequireFailure(a, b Recognizer) bool {
	if a == nil || b == nil {
		return false
	}
	id := b.ID()
	for _, required := range a.Delegate().ShouldRequireFailureOf {
		if required == id {
			return true
		}
	}
	return false
}

func simultaneousAllowed(a, b Recognizer) bool {
	if a == nil || b == nil {
		return false
	}
	ad := a.Delegate()
	bd := b.Delegate()
	if ad.ShouldRecognizeSimultaneouslyWith != nil && ad.ShouldRecognizeSimultaneouslyWith(b) {
		return true
	}
	if bd.ShouldRecognizeSimultaneouslyWith != nil && bd.ShouldRecognizeSimultaneouslyWith(a) {
		return true
	}
	return false
}

func isFailedOrPossible(state RecognizerState) bool {
	return state == RecognizerPossible || state == RecognizerFailed
}

func buildFacetPath(tree facet.Tree, id facet.FacetID) []facet.FacetImpl {
	if tree == nil || id == 0 {
		return nil
	}
	node := tree.FacetByID(id)
	if node == nil {
		return nil
	}
	var path []facet.FacetImpl
	for node != nil {
		path = append(path, node)
		base := node.Base()
		if base == nil {
			break
		}
		parent := base.Parent()
		if parent == nil {
			break
		}
		next := parent.Impl()
		if next == nil {
			next = parent
		}
		node = next
	}
	return path
}
