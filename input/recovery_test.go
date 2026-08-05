package input

import (
	"sync"
	"sync/atomic"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

// inputRecoveryStub mirrors the runtime's facet-recovery contract so input's
// use of the RecoveryHook surface can be tested in isolation (input cannot
// import runtime without an import cycle).
type inputRecoveryStub struct {
	mu       sync.RWMutex
	poisoned map[facet.FacetID]struct{}
}

func (r *inputRecoveryStub) hook() RecoveryHook {
	return func(id facet.FacetID, role string, cb func()) bool {
		if r.isPoisoned(id) {
			return false
		}
		defer func() {
			if p := recover(); p != nil {
				r.poison(id)
			}
		}()
		cb()
		return !r.isPoisoned(id)
	}
}

func (r *inputRecoveryStub) poison(id facet.FacetID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.poisoned == nil {
		r.poisoned = make(map[facet.FacetID]struct{})
	}
	r.poisoned[id] = struct{}{}
}

func (r *inputRecoveryStub) isPoisoned(id facet.FacetID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.poisoned == nil {
		return false
	}
	_, ok := r.poisoned[id]
	return ok
}

// panicInputFacet wires every input/focus handler to record a call and, when
// panicOn is true, panic.
type panicInputFacet struct {
	facet.Facet
	input facet.InputRole
	focus facet.FocusRole
	calls atomic.Int32
}

func (f *panicInputFacet) Base() *facet.Facet               { f.BindImpl(f); return &f.Facet }
func (f *panicInputFacet) OnAttach(ctx facet.AttachContext) {}
func (f *panicInputFacet) OnDetach()                        {}
func (f *panicInputFacet) OnActivate()                      {}
func (f *panicInputFacet) OnDeactivate()                    {}

func (f *panicInputFacet) init(panicOn bool) {
	run := func() {
		f.calls.Add(1)
		if panicOn {
			panic("boom")
		}
	}
	f.input.OnPointer = func(e facet.PointerEvent) bool { run(); return true }
	f.input.OnScroll = func(e facet.ScrollEvent) bool { run(); return true }
	f.input.OnKey = func(e facet.KeyEvent) bool { run(); return true }
	f.input.OnText = func(e facet.TextEvent) bool { run(); return true }
	f.input.OnDismiss = func(e facet.DismissEvent) bool { run(); return true }
	f.input.OnTouch = func(e facet.TouchEvent) bool { run(); return true }
	f.focus.OnFocusGained = func() { run() }
	f.focus.OnFocusLost = func() { run() }
	f.AddRole(&f.input)
	f.AddRole(&f.focus)
}

func TestDeliver_PanicInOnPointer_Quarantined(t *testing.T) {
	rec := &inputRecoveryStub{}
	SetRecoveryHook(rec.hook())
	defer ClearRecoveryHook()

	f := &panicInputFacet{Facet: facet.NewFacet()}
	f.init(true)
	facetID := f.Base().ID()
	event := RoutedEvent{Target: facetID, Event: PointerPressEvent{Button: platform.PointerLeft}}

	if got := Deliver(event, f); got {
		t.Fatal("Deliver reported handled for a panicking facet")
	}
	if !rec.isPoisoned(facetID) {
		t.Fatal("expected facet to be quarantined after OnPointer panic")
	}
	if got := f.calls.Load(); got != 1 {
		t.Fatalf("OnPointer invoked %d times, want 1", got)
	}

	if got := Deliver(event, f); got {
		t.Fatal("Deliver reported handled for a quarantined facet")
	}
	if got := f.calls.Load(); got != 1 {
		t.Fatalf("OnPointer invoked %d times after quarantine, want 1 (poisoned facet skipped)", got)
	}
}

func TestDeliver_HandlerPanics_Quarantined(t *testing.T) {
	rec := &inputRecoveryStub{}
	SetRecoveryHook(rec.hook())
	defer ClearRecoveryHook()

	cases := []struct {
		name  string
		event func() DeliveredEvent
	}{
		{"pointer", func() DeliveredEvent { return PointerPressEvent{Button: platform.PointerLeft} }},
		{"scroll", func() DeliveredEvent { return ScrollEvent{DeltaY: 1} }},
		{"key", func() DeliveredEvent { return KeyInputEvent{Key: platform.KeyA} }},
		{"text", func() DeliveredEvent { return TextInputEvent{Text: "a"} }},
		{"dismiss", func() DeliveredEvent { return DismissEvent{} }},
		{"touch", func() DeliveredEvent { return TouchInputEvent{Event: facet.TouchEvent{}} }},
		{"focus_gained", func() DeliveredEvent { return FocusGainedEvent{} }},
		{"focus_lost", func() DeliveredEvent { return FocusLostEvent{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &panicInputFacet{Facet: facet.NewFacet()}
			f.init(true)
			facetID := f.Base().ID()

			_ = Deliver(RoutedEvent{Target: facetID, Event: tc.event()}, f)
			if !rec.isPoisoned(facetID) {
				t.Fatal("expected facet to be quarantined after handler panic")
			}
			if got := f.calls.Load(); got != 1 {
				t.Fatalf("handler invoked %d times, want 1", got)
			}

			_ = Deliver(RoutedEvent{Target: facetID, Event: tc.event()}, f)
			if got := f.calls.Load(); got != 1 {
				t.Fatalf("handler invoked %d times after quarantine, want 1", got)
			}
		})
	}
}

func TestDeliver_NoHook_PanicPropagates(t *testing.T) {
	ClearRecoveryHook()

	f := &panicInputFacet{Facet: facet.NewFacet()}
	f.init(true)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected the panic to propagate with no recovery hook")
		}
	}()
	_ = Deliver(RoutedEvent{Target: f.Base().ID(), Event: PointerPressEvent{Button: platform.PointerLeft}}, f)
}

// panicHitFacet panics in OnHitTest and counts calls.
type panicHitFacet struct {
	facet.Facet
	hit   facet.HitRole
	calls atomic.Int32
}

func (f *panicHitFacet) Base() *facet.Facet               { f.BindImpl(f); return &f.Facet }
func (f *panicHitFacet) OnAttach(ctx facet.AttachContext) {}
func (f *panicHitFacet) OnDetach()                        {}
func (f *panicHitFacet) OnActivate()                      {}
func (f *panicHitFacet) OnDeactivate()                    {}

func (f *panicHitFacet) init() {
	f.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		f.calls.Add(1)
		panic("boom")
	}
	f.AddRole(&f.hit)
}

func TestHitResolve_PanicInOnHitTest_Quarantined(t *testing.T) {
	rec := &inputRecoveryStub{}
	SetRecoveryHook(rec.hook())
	defer ClearRecoveryHook()

	f := &panicHitFacet{Facet: facet.NewFacet()}
	f.init()
	facetID := f.Base().ID()

	if got := refineHitTest(f, facetID, gfx.Point{}); got != nil {
		t.Fatal("refineHitTest returned a hit for a panicking facet")
	}
	if !rec.isPoisoned(facetID) {
		t.Fatal("expected facet to be quarantined after OnHitTest panic")
	}
	if got := f.calls.Load(); got != 1 {
		t.Fatalf("OnHitTest invoked %d times, want 1", got)
	}

	if got := refineHitTest(f, facetID, gfx.Point{}); got != nil {
		t.Fatal("refineHitTest returned a hit for a quarantined facet")
	}
	if got := f.calls.Load(); got != 1 {
		t.Fatalf("OnHitTest invoked %d times after quarantine, want 1", got)
	}
}

func TestRecoveryHook_InstalledAndCleared(t *testing.T) {
	ClearRecoveryHook()
	if h := currentRecoveryHook(); h != nil {
		t.Fatal("expected nil hook initially")
	}
	sentinel := RecoveryHook(func(facet.FacetID, string, func()) bool { return true })
	SetRecoveryHook(sentinel)
	if h := currentRecoveryHook(); h == nil {
		t.Fatal("expected hook after SetRecoveryHook")
	}
	ClearRecoveryHook()
	if h := currentRecoveryHook(); h != nil {
		t.Fatal("expected nil hook after ClearRecoveryHook")
	}
}
