package runtime

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	drive "codeburg.org/lexbit/lurpicui/drive/server"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

// inputProbe is a facet with a real pointer-handler hit surface — the click
// target and the preemption observer.
type inputProbe struct {
	facet.Facet
	layout facet.LayoutRole
	input  facet.InputRole
	hit    facet.HitRole
	focus  facet.FocusRole

	clicks int32
}

func (p *inputProbe) Base() *facet.Facet           { return &p.Facet }
func (p *inputProbe) OnAttach(facet.AttachContext) {}
func (p *inputProbe) OnDetach()                    {}
func (p *inputProbe) OnActivate()                  {}
func (p *inputProbe) OnDeactivate()                {}

func newInputRuntime(t *testing.T, queue platform.EventQueue) (*Runtime, *inputProbe) {
	t.Helper()
	probe := &inputProbe{}
	probe.Facet = facet.NewFacet()
	probe.BindImpl(probe)
	probe.SetTestID("probe")
	probe.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		return facet.HitResult{Hit: true}
	}
	probe.input.OnPointer = func(e facet.PointerEvent) bool {
		if e.Kind == platform.PointerRelease {
			probe.clicks++
		}
		return true
	}
	probe.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 50}}
	}
	probe.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		probe.layout.ArrangedBounds = bounds
	}
	probe.layout.Child.SupportedPlacement = facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree | facet.SupportsLinear
	probe.AddRole(&probe.layout)
	probe.AddRole(&probe.input)
	probe.AddRole(&probe.hit)
	probe.focus.Focusable = func() bool { return true }
	probe.AddRole(&probe.focus)

	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Drive.Enabled = true
	rt, err := New(cfg, &inputQueueApp{q: queue}, &testWindow{width: 200, height: 100}, &backendFixture{}, probe)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(func() { rt.Shutdown() })
	rt.RunOneFrameAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	return rt, probe
}

// inputDriveCall runs one drive input verb to completion.
func inputDriveCall(t *testing.T, rt *Runtime, method string, params any, grants drive.Grants) drive.Result {
	t.Helper()
	var raw json.RawMessage
	if params != nil {
		enc, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		raw = enc
	}
	cmd := drive.Command{ID: 1, Method: method, Params: raw, Grants: grants, Reply: make(chan drive.Result, 1)}
	rt.DriveSink().Enqueue(cmd)
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 256; i++ {
		rt.RunOneFrameAt(epoch.Add(time.Duration(i) * time.Second))
		select {
		case res := <-cmd.Reply:
			return res
		default:
		}
	}
	t.Fatalf("drive %q did not reply", method)
	return drive.Result{}
}

// recordQueue records enqueued events so tests observe the injection path.
type recordQueue struct {
	mu     sync.Mutex
	events []platform.Event
	// seen is the enqueue history — survives Poll consumption, so tests
	// assert provenance after the pipeline ran.
	seen []platform.Event
}

func (q *recordQueue) Push(e platform.Event) { q.EnqueueDrive(e) }
func (q *recordQueue) EnqueueDrive(evs ...platform.Event) {
	q.mu.Lock()
	q.events = append(q.events, evs...)
	q.seen = append(q.seen, evs...)
	q.mu.Unlock()
}

// seenDriveEvents snapshots the enqueue history.
func (q *recordQueue) seenDriveEvents() []platform.Event {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]platform.Event(nil), q.seen...)
}
func (q *recordQueue) Poll() []platform.Event {
	q.mu.Lock()
	defer q.mu.Unlock()
	e := q.events
	q.events = nil
	return e
}
func (q *recordQueue) Wait(timeout time.Duration) []platform.Event { return q.Poll() }

func TestDriveInput_ClickSynthesizesPlatformShapedEvents(t *testing.T) {
	q := &recordQueue{}
	rt, probe := newInputRuntime(t, q)

	res := inputDriveCall(t, rt, "input.click", map[string]any{"x": 50, "y": 25}, allGrants())
	if res.Err != nil {
		t.Fatalf("input.click: %v", res.Err)
	}
	// The click flowed the real pipeline: the probe's release handler fired.
	if probe.clicks != 1 {
		t.Fatalf("clicks = %d, want 1", probe.clicks)
	}
	// Every enqueued event carries SourceDrive (G5.1 provenance).
	for _, ev := range q.seenDriveEvents() {
		if pe, ok := ev.(platform.EventPointer); !ok || pe.Source != platform.SourceDrive {
			t.Fatalf("injected event = %+v, want SourceDrive pointers", ev)
		}
	}
	// Ownership returned to human after the step.
	if own := rt.inputOwnership.Get(); own.Owner != OwnerHuman {
		t.Fatalf("ownership after step = %v", own.Owner)
	}
}

func TestDriveInput_OwnershipLifecycle(t *testing.T) {
	q := &recordQueue{}
	rt, _ := newInputRuntime(t, q)

	if own := rt.inputOwnership.Get(); own.Owner != OwnerHuman {
		t.Fatalf("initial ownership = %v", own.Owner)
	}
	// Mid-step ownership is observable: a long drag holds OwnerClient until
	// the events settle.
	res := inputDriveCall(t, rt, "input.drag", map[string]any{
		"from_x": 0, "from_y": 0, "to_x": 100, "to_y": 50, "steps": 20,
	}, allGrants())
	if res.Err != nil {
		t.Fatalf("input.drag: %v", res.Err)
	}
	if own := rt.inputOwnership.Get(); own.Owner != OwnerHuman {
		t.Fatalf("ownership after drag = %v, want human", own.Owner)
	}
}

// gatedQueue doses events one per Poll and injects the human event after a
// fixed number of drive events — deterministic preemption (no sleeps: the
// repo's test policy bans them; the injection point is a queue position).
type gatedQueue struct {
	mu         sync.Mutex
	events     []platform.Event
	humanAfter int
	human      platform.Event
	polls      int
}

func (q *gatedQueue) EnqueueDrive(evs ...platform.Event) {
	q.mu.Lock()
	q.events = append(q.events, evs...)
	q.mu.Unlock()
}
func (q *gatedQueue) Push(e platform.Event) { q.EnqueueDrive(e) }
func (q *gatedQueue) Poll() []platform.Event {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.events) == 0 {
		return nil
	}
	q.polls++
	println("POLL polls=", q.polls, "after=", q.humanAfter, "queued=", len(q.events))
	if q.polls == q.humanAfter {
		// The human event lands at the FRONT: it is observed by the next
		// poll, mid-step — appending would defer it until after the step
		// completed, which is not a preemption.
		q.events = append([]platform.Event{q.human}, q.events...)
	}
	e := q.events[0]
	q.events = q.events[1:]
	return []platform.Event{e}
}
func (q *gatedQueue) Wait(timeout time.Duration) []platform.Event { return q.Poll() }

// TestDriveInput_PreemptionByPlatformEvent (FR-15): a human (SourcePlatform)
// event observed mid-step severs the step — the reply reports
// input.preempted, ownership flips to Human, remaining drive events are
// voided, and the interim own.preempted buffer holds the notification.
func TestDriveInput_PreemptionByPlatformEvent(t *testing.T) {
	q := &gatedQueue{
		humanAfter: 4,
		human: platform.EventPointer{
			Kind: platform.PointerMove, Position: gfx.Point{X: 7, Y: 7}, Source: platform.SourcePlatform,
		},
	}
	rt, _ := newInputRuntime(t, q)

	cmd := drive.Command{ID: 9, Method: "input.drag", Grants: allGrants(), Reply: make(chan drive.Result, 1)}
	enc, err := json.Marshal(map[string]any{"from_x": 0, "from_y": 0, "to_x": 100, "to_y": 50, "steps": 40})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cmd.Params = enc
	rt.DriveSink().Enqueue(cmd)
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var res drive.Result
	for i := 0; i < 256; i++ {
		rt.RunOneFrameAt(epoch.Add(time.Duration(i) * time.Second))
		select {
		case res = <-cmd.Reply:
		default:
		}
		if res.Payload != nil || res.Err != nil {
			break
		}
	}
	if res.Err == nil || res.Err.Code != "input.preempted" {
		t.Fatalf("result = %+v, want input.preempted", res.Err)
	}
	// Ownership returned to human.
	if own := rt.inputOwnership.Get(); own.Owner != OwnerHuman {
		t.Fatalf("ownership after preemption = %v", own.Owner)
	}
	// The interim own.preempted buffer holds the notification (deleted in P7).
	sink := rt.DriveSink().(*driveRuntimeSink)
	if len(sink.preempted) == 0 || sink.preempted[0].CommandID != 9 {
		t.Fatalf("preempted buffer = %+v", sink.preempted)
	}
	// No drive pointer events survive in the pending set.
	for _, ev := range rt.pendingEvents {
		if pe, ok := ev.(platform.EventPointer); ok && pe.Source == platform.SourceDrive {
			t.Fatalf("preempted step's events survived: %+v", pe)
		}
	}
}

func TestDriveInput_FocusWithoutPointer(t *testing.T) {
	q := &recordQueue{}
	rt, probe := newInputRuntime(t, q)
	res := inputDriveCall(t, rt, "input.focus", map[string]any{
		"target": map[string]string{"testid": "probe"},
	}, allGrants())
	if res.Err != nil {
		t.Fatalf("input.focus: %v", res.Err)
	}
	if focused := rt.focusManager.FocusedImpl(); focused == nil || focused.Base().ID() != probe.Base().ID() {
		t.Fatal("input.focus must drive the FocusRole without a pointer")
	}
}

func TestDriveInput_GrantMatrix(t *testing.T) {
	q := &recordQueue{}
	rt, _ := newInputRuntime(t, q)

	// Every verb × the grant sets that matter: input verbs require the
	// input grant; cmd.execute requires write; reads require read.
	verbs := []struct {
		method string
		params any
	}{
		{"input.click", map[string]any{"x": 1, "y": 1}},
		{"input.press", map[string]any{"x": 1, "y": 1}},
		{"input.release", map[string]any{"x": 1, "y": 1}},
		{"input.move", map[string]any{"x": 1, "y": 1}},
		{"input.drag", map[string]any{"from_x": 0, "from_y": 0, "to_x": 1, "to_y": 1}},
		{"input.type", map[string]any{"text": "x"}},
		{"input.scroll", map[string]any{"x": 1, "y": 1, "delta_y": 1}},
		{"input.focus", map[string]any{"target": map[string]string{"testid": "probe"}}},
	}
	for _, v := range verbs {
		res := inputDriveCall(t, rt, v.method, v.params, drive.Grants{Read: true, Write: true, Time: true})
		if res.Err == nil || res.Err.Code != "grant.denied" {
			t.Errorf("%s without input grant = %+v, want grant.denied", v.method, res.Err)
		}
	}
	// The read-only set works with just the input grant missing.
	res := inputDriveCall(t, rt, "cmd.execute", map[string]any{"id": "nope"}, drive.Grants{Input: true})
	if res.Err == nil || res.Err.Code != "grant.denied" {
		t.Fatalf("cmd.execute without write grant = %+v, want grant.denied", res.Err)
	}
}

func TestDriveInput_TypeRequiresIMEFocus(t *testing.T) {
	q := &recordQueue{}
	rt, _ := newInputRuntime(t, q)
	// No focused mark at all: typed refusal.
	if res := inputDriveCall(t, rt, "input.type", map[string]any{"text": "hi"}, allGrants()); res.Err == nil ||
		!strings.Contains(res.Err.Detail, "focused text mark") {
		t.Fatalf("err = %+v", res.Err)
	}
}

// inputQueueApp is a minimal platform.App around an injected event queue
// (the drive injection path needs a real queue).
type inputQueueApp struct {
	q platform.EventQueue
}

func (a *inputQueueApp) Events() platform.EventQueue { return a.q }
func (a *inputQueueApp) Destroy()                    {}
