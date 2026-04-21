package gesture

import (
	"reflect"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

type mockNode struct {
	*facet.Facet
}

func newMockNode() *mockNode {
	base := facet.NewFacet()
	n := &mockNode{Facet: &base}
	n.Facet.BindImpl(n)
	return n
}

func (n *mockNode) Base() *facet.Facet           { return n.Facet }
func (n *mockNode) ID() facet.FacetID            { return n.Facet.ID() }
func (n *mockNode) OnAttach(facet.AttachContext) {}
func (n *mockNode) OnDetach()                    {}
func (n *mockNode) OnActivate()                  {}
func (n *mockNode) OnDeactivate()                {}

type mockTree struct {
	nodes map[facet.FacetID]facet.FacetImpl
}

func (t *mockTree) FacetByID(id facet.FacetID) facet.FacetImpl {
	if t == nil {
		return nil
	}
	return t.nodes[id]
}

func (t *mockTree) RootStyleContext() any { return nil }

type mockRecognizer struct {
	name        string
	id          RecognizerID
	sm          *StateMachine
	delegate    RecognizerDelegate
	calls       []string
	transitions []RecognizerState
	beginState  RecognizerState
	moveState   RecognizerState
	endState    RecognizerState
	cancelState RecognizerState
	queued      int
}

func newMockRecognizer(name string) *mockRecognizer {
	return &mockRecognizer{name: name, id: NewRecognizerID(), sm: NewStateMachine(name)}
}

func (r *mockRecognizer) ID() RecognizerID { return r.id }
func (r *mockRecognizer) State() RecognizerState {
	return r.sm.State()
}
func (r *mockRecognizer) Delegate() RecognizerDelegate { return r.delegate }
func (r *mockRecognizer) Reset()                       { r.sm.Reset() }
func (r *mockRecognizer) Transition(next RecognizerState) {
	r.transitions = append(r.transitions, next)
	r.sm.Transition(next)
}
func (r *mockRecognizer) TouchesBegan(touches []Touch, event InputEvent) {
	r.calls = append(r.calls, r.name+":began")
	if r.beginState != RecognizerPossible {
		r.Transition(r.beginState)
	}
}
func (r *mockRecognizer) TouchesMoved(touches []Touch, event InputEvent) {
	r.calls = append(r.calls, r.name+":moved")
	if r.moveState != RecognizerPossible {
		r.Transition(r.moveState)
	}
}
func (r *mockRecognizer) TouchesEnded(touches []Touch, event InputEvent) {
	r.calls = append(r.calls, r.name+":ended")
	if r.endState != RecognizerPossible {
		r.Transition(r.endState)
	}
}
func (r *mockRecognizer) TouchesCancelled(touches []Touch, event InputEvent) {
	r.calls = append(r.calls, r.name+":cancelled")
	if r.cancelState != RecognizerPossible {
		r.Transition(r.cancelState)
	}
}
func (r *mockRecognizer) QueueSignals(q SignalQueue, touches []Touch, event InputEvent) {
	q.Enqueue(func() { r.queued++ })
}

func TestStateMachine_transitions(t *testing.T) {
	sm := NewStateMachine("mock")
	sm.Transition(RecognizerBegan)
	if sm.State() != RecognizerBegan {
		t.Fatalf("state = %v", sm.State())
	}
	sm.Transition(RecognizerChanged)
	if sm.State() != RecognizerChanged {
		t.Fatalf("state = %v", sm.State())
	}
	sm.Transition(RecognizerEnded)
	if sm.State() != RecognizerEnded {
		t.Fatalf("state = %v", sm.State())
	}
	sm.Reset()
	if sm.State() != RecognizerPossible {
		t.Fatalf("state = %v", sm.State())
	}
	sm.Transition(RecognizerFailed)
	if sm.State() != RecognizerPossible {
		t.Fatalf("failed should auto-reset to possible, got %v", sm.State())
	}
	sm.Transition(RecognizerCancelled)
	if sm.State() != RecognizerPossible {
		t.Fatalf("cancelled should auto-reset to possible, got %v", sm.State())
	}
}

func TestStateMachine_invalidTransitions_panic(t *testing.T) {
	sm := NewStateMachine("mock")
	sm.Transition(RecognizerEnded)
	mustPanicContains(t, func() { sm.Transition(RecognizerBegan) }, "mock", "Ended", "Began")
}

func TestGesturePipeline_routingOrder_childBeforeParent(t *testing.T) {
	tree, root, child := buildGestureTree()
	rootRec := newMockRecognizer("root")
	childRec := newMockRecognizer("child")
	rootRec.beginState = RecognizerEnded
	childRec.beginState = RecognizerEnded

	p := NewGesturePipeline()
	p.Register(root.ID(), GestureRole{Recognizers: []Recognizer{rootRec}})
	p.Register(child.ID(), GestureRole{Recognizers: []Recognizer{childRec}})

	p.Process(platform.EventPointer{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 2}}, child.ID(), tree)

	want := []string{"child:began", "root:began"}
	if !reflect.DeepEqual(append([]string(nil), childRec.calls...), []string{"child:began"}) {
		t.Fatalf("child calls = %#v", childRec.calls)
	}
	if !reflect.DeepEqual(append([]string(nil), rootRec.calls...), []string{"root:began"}) {
		t.Fatalf("root calls = %#v", rootRec.calls)
	}
	if got := append(append([]string(nil), childRec.calls...), rootRec.calls...); !reflect.DeepEqual(got, want) {
		t.Fatalf("call order = %#v want %#v", got, want)
	}
}

func TestGesturePipeline_registerReplaceAndUnregister(t *testing.T) {
	tree, root, _ := buildGestureTree()
	first := newMockRecognizer("first")
	second := newMockRecognizer("second")
	first.beginState = RecognizerEnded
	second.beginState = RecognizerEnded

	p := NewGesturePipeline()
	p.Register(root.ID(), GestureRole{Recognizers: []Recognizer{first}})
	p.Register(root.ID(), GestureRole{Recognizers: []Recognizer{second}})
	p.Process(platform.EventPointer{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 2}}, root.ID(), tree)
	if len(first.calls) != 0 {
		t.Fatalf("first should have been replaced, got %#v", first.calls)
	}
	if len(second.calls) != 1 {
		t.Fatalf("second should have been called once, got %#v", second.calls)
	}

	p.Unregister(root.ID())
	p.Process(platform.EventPointer{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 2}}, root.ID(), tree)
	if len(second.calls) != 1 {
		t.Fatalf("unregister should stop calls, got %#v", second.calls)
	}
}

func TestGesturePipeline_competition_priority_wins(t *testing.T) {
	tree, root, _ := buildGestureTree()
	high := newMockRecognizer("high")
	low := newMockRecognizer("low")
	high.beginState = RecognizerEnded
	low.beginState = RecognizerEnded
	high.delegate.Priority = 1
	low.delegate.Priority = 0

	p := NewGesturePipeline()
	p.Register(root.ID(), GestureRole{Recognizers: []Recognizer{low, high}})
	p.Process(platform.EventPointer{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 2}}, root.ID(), tree)
	if !containsState(high.transitions, RecognizerEnded) {
		t.Fatalf("high should have ended, transitions=%#v", high.transitions)
	}
	if !containsState(low.transitions, RecognizerFailed) {
		t.Fatalf("low should have been failed, transitions=%#v", low.transitions)
	}
}

func TestGesturePipeline_competition_simultaneous_allows_both(t *testing.T) {
	tree, root, _ := buildGestureTree()
	a := newMockRecognizer("a")
	b := newMockRecognizer("b")
	a.beginState = RecognizerEnded
	b.beginState = RecognizerEnded
	a.delegate.ShouldRecognizeSimultaneouslyWith = func(other Recognizer) bool { return true }

	p := NewGesturePipeline()
	p.Register(root.ID(), GestureRole{Recognizers: []Recognizer{a, b}})
	p.Process(platform.EventPointer{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 2}}, root.ID(), tree)
	p.DrainQueuedSignals()
	if a.queued != 1 || b.queued != 1 {
		t.Fatalf("both recognizers should have queued signals: a=%d b=%d", a.queued, b.queued)
	}
}

func TestGesturePipeline_competition_requireFailure(t *testing.T) {
	tree, root, _ := buildGestureTree()
	a := newMockRecognizer("a")
	b := newMockRecognizer("b")
	a.beginState = RecognizerEnded
	b.beginState = RecognizerFailed
	a.delegate.ShouldRequireFailureOf = []RecognizerID{b.ID()}

	p := NewGesturePipeline()
	p.Register(root.ID(), GestureRole{Recognizers: []Recognizer{b, a}})
	p.Process(platform.EventPointer{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 2}}, root.ID(), tree)
	p.DrainQueuedSignals()
	if !containsState(a.transitions, RecognizerEnded) {
		t.Fatalf("a should have ended, transitions=%#v", a.transitions)
	}
	if a.queued != 1 {
		t.Fatalf("a should have queued signal, got %d", a.queued)
	}
}

func TestGesturePipeline_signalQueueing(t *testing.T) {
	tree, root, _ := buildGestureTree()
	r := newMockRecognizer("signal")
	r.beginState = RecognizerEnded

	p := NewGesturePipeline()
	p.Register(root.ID(), GestureRole{Recognizers: []Recognizer{r}})
	p.Process(platform.EventPointer{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 2}}, root.ID(), tree)
	if r.queued != 0 {
		t.Fatalf("signal should be queued, not drained, got %d", r.queued)
	}
	p.DrainQueuedSignals()
	if r.queued != 1 {
		t.Fatalf("signal should drain exactly once, got %d", r.queued)
	}
}

func buildGestureTree() (*mockTree, *mockNode, *mockNode) {
	root := newMockNode()
	child := newMockNode()
	root.Facet.AddChild(child.Facet)
	nodes := map[facet.FacetID]facet.FacetImpl{
		root.ID():  root,
		child.ID(): child,
	}
	return &mockTree{nodes: nodes}, root, child
}

func containsState(states []RecognizerState, want RecognizerState) bool {
	for _, got := range states {
		if got == want {
			return true
		}
	}
	return false
}

func mustPanicContains(t *testing.T, fn func(), parts ...string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic")
		}
		msg := ""
		if s, ok := r.(string); ok {
			msg = s
		} else {
			msg = ""
		}
		for _, part := range parts {
			if !strings.Contains(msg, part) {
				t.Fatalf("panic %q does not contain %q", msg, part)
			}
		}
	}()
	fn()
}
