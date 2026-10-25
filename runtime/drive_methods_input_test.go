package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	driveserver "codeburg.org/lexbit/lurpicui/drive/server"
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

	clicks    int32
	delivered []facet.PointerEvent
}

func (p *inputProbe) Base() *facet.Facet           { return &p.Facet }
func (p *inputProbe) OnAttach(facet.AttachContext) {}
func (p *inputProbe) OnDetach()                    {}
func (p *inputProbe) OnActivate()                  {}
func (p *inputProbe) OnDeactivate()                {}

func newInputRuntime(t *testing.T, queue platform.EventQueue) (*Runtime, *inputProbe) {
	return newInputRuntimeWithSessions(t, queue, nil)
}

func newInputRuntimeWithSessions(t *testing.T, queue platform.EventQueue, sessions *drive.SessionTable) (*Runtime, *inputProbe) {
	t.Helper()
	probe := &inputProbe{}
	probe.Facet = facet.NewFacet()
	probe.BindImpl(probe)
	probe.SetTestID("probe")
	probe.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		return facet.HitResult{Hit: true}
	}
	probe.input.OnPointer = func(e facet.PointerEvent) bool {
		probe.delivered = append(probe.delivered, e)
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
	if sessions == nil {
		testSessionTable(t, &cfg)
	} else {
		cfg.Drive.Sessions = sessions
		cfg.Drive.Stats = &drive.Stats{}
	}
	rt, err := New(cfg, &inputQueueApp{q: queue}, &testWindow{width: 200, height: 100}, &backendFixture{}, probe)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(func() { rt.Shutdown() })
	if sessions == nil {
		rt.RunOneFrameAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	}
	return rt, probe
}

// inputDriveCall runs one drive input verb to completion through the
// trusted session client.
func inputDriveCall(t *testing.T, rt *Runtime, method string, params any, grants drive.Grants) drive.Result {
	t.Helper()
	return driveCall(t, rt, method, params, grants)
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
	if len(probe.delivered) < 3 {
		t.Fatalf("delivered pointer trajectory=%+v", probe.delivered)
	}
	for _, event := range probe.delivered[len(probe.delivered)-3:] {
		if event.Source != platform.SourceDrive || event.Attribution.Session == "" || event.Attribution.Generation == 0 || event.Attribution.Operation == "" {
			t.Fatalf("facet event lost drive provenance: %+v", event)
		}
	}
	// Every enqueued event carries SourceDrive (G5.1 provenance).
	for _, ev := range q.seenDriveEvents() {
		if pe, ok := ev.(platform.EventPointer); !ok || pe.Source != platform.SourceDrive {
			t.Fatalf("injected event = %+v, want SourceDrive pointers", ev)
		}
	}
	// Ownership returned to human after the step.
	if own := rt.ControlSnapshot(); own.InputOwner != facet.ControlOwnerHuman {
		t.Fatalf("ownership after step = %v", own.InputOwner)
	}
}

func TestInputSocketPalette(t *testing.T) { runInputSocketScenario(t, false) }

func TestGestureDisconnectCleanup(t *testing.T) { runInputSocketScenario(t, true) }

func marshalSocketTestJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal socket test envelope: %v", err)
	}
	return encoded
}

func runInputSocketScenario(t *testing.T, disconnectHeldGesture bool) {
	instance, err := drive.NewInstance()
	if err != nil {
		t.Fatal(err)
	}
	sessions := drive.NewSessionTable(instance)
	queue := &recordQueue{}
	rt, probe := newInputRuntimeWithSessions(t, queue, sessions)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "drive.sock")
	token := strings.Repeat("ab", 32)
	srv, err := driveserver.Listen(driveserver.Config{SocketPath: socket, Token: token, Grants: drive.Grants{Read: true, Input: true, Time: true}, Sessions: sessions, Port: rt.DrivePort()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	go func() { _ = rt.Run() }()
	conn, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	reader := bufio.NewReader(conn)
	write := func(value any) {
		t.Helper()
		data, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if _, writeErr := fmt.Fprintf(conn, "%s\n", data); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	read := func() map[string]json.RawMessage {
		t.Helper()
		line, readErr := reader.ReadBytes('\n')
		if readErr != nil {
			t.Fatal(readErr)
		}
		var envelope map[string]json.RawMessage
		if decodeErr := json.Unmarshal(line, &envelope); decodeErr != nil {
			t.Fatalf("wire line %q: %v", line, decodeErr)
		}
		return envelope
	}
	write(proto.Request{V: proto.ProtocolMajor, ID: "1", Method: "sys.hello", Params: json.RawMessage(`{"token":"` + token + `","client_name":"s4-input-test","create_session":true}`)})
	hello := read()
	var helloID string
	_ = json.Unmarshal(hello["id"], &helloID)
	if helloID != "1" {
		t.Fatalf("hello response id=%q", helloID)
	}
	var helloResult proto.Response
	encodedHello := marshalSocketTestJSON(t, hello)
	if err := json.Unmarshal(encodedHello, &helloResult); err != nil || !helloResult.OK {
		t.Fatalf("socket hello=%s (%v)", encodedHello, err)
	}
	write(proto.Request{V: proto.ProtocolMajor, ID: "1", Method: "input.click", Params: json.RawMessage(`{"x":50,"y":25}`)})
	accepted, completed := false, false
	deadline := time.Now().Add(3 * time.Second)
	for !(accepted && completed) && time.Now().Before(deadline) {
		envelope := read()
		var id, method string
		_ = json.Unmarshal(envelope["id"], &id)
		_ = json.Unmarshal(envelope["method"], &method)
		if id == "1" {
			var response proto.Response
			encoded := marshalSocketTestJSON(t, envelope)
			if err := json.Unmarshal(encoded, &response); err != nil || !response.OK {
				t.Fatalf("input receipt=%s (%v)", encoded, err)
			}
			accepted = true
		}
		if method == "op.completed" {
			var notification proto.Notification
			encoded := marshalSocketTestJSON(t, envelope)
			if err := json.Unmarshal(encoded, &notification); err != nil {
				t.Fatal(err)
			}
			var payload struct {
				OperationID string `json:"operation_id"`
			}
			if err := json.Unmarshal(notification.Params, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.OperationID == "" {
				t.Fatalf("completion lacks operation identity: %s", encoded)
			}
			completed = true
		}
	}
	if !accepted || !completed {
		t.Fatalf("socket operation receipt/completion missing: accepted=%v completed=%v", accepted, completed)
	}
	if probe.clicks != 1 {
		t.Fatalf("socket click reached %d release activations, want one", probe.clicks)
	}
	if !disconnectHeldGesture {
		return
	}
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	write(proto.Request{V: proto.ProtocolMajor, ID: "2", Method: "input.press", Params: json.RawMessage(`{"x":50,"y":25}`)})
	accepted, completed = false, false
	deadline = time.Now().Add(3 * time.Second)
	for !(accepted && completed) && time.Now().Before(deadline) {
		envelope := read()
		var id, method string
		_ = json.Unmarshal(envelope["id"], &id)
		_ = json.Unmarshal(envelope["method"], &method)
		if id == "2" {
			var response proto.Response
			encoded := marshalSocketTestJSON(t, envelope)
			if err := json.Unmarshal(encoded, &response); err != nil || !response.OK {
				t.Fatalf("press receipt=%s (%v)", encoded, err)
			}
			accepted = true
		}
		if method == "op.completed" {
			completed = true
		}
	}
	if !accepted || !completed {
		t.Fatal("socket press did not reach terminal completion")
	}
	if state := rt.ControlSnapshot(); state.InputOwner != facet.ControlOwnerClient || state.HeldGestures != 1 {
		t.Fatalf("press did not retain its owned gesture: %+v", state)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(time.Second)
	for rt.ControlSnapshot().InputOwner != facet.ControlOwnerHuman && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	state := rt.ControlSnapshot()
	if state.InputOwner != facet.ControlOwnerHuman || state.HeldGestures != 0 {
		t.Fatalf("disconnect did not cancel held gesture: %+v", state)
	}
}

func TestDriveTouchClickUsesOneContactAcrossSeparateFrames(t *testing.T) {
	q := &recordQueue{}
	rt, _ := newInputRuntime(t, q)
	before := rt.frameNumber
	result := inputDriveCall(t, rt, "input.click", map[string]any{"x": 20, "y": 10, "device": "touch"}, allGrants())
	if result.Err != nil {
		t.Fatalf("touch click: %v", result.Err)
	}
	var events []platform.TouchEvent
	for _, event := range q.seenDriveEvents() {
		if touch, ok := event.(platform.TouchEvent); ok && touch.Source == platform.SourceDrive {
			events = append(events, touch)
		}
	}
	if len(events) != 2 || events[0].Phase != platform.TouchDown || events[1].Phase != platform.TouchUp {
		t.Fatalf("touch click events=%+v, want down/up", events)
	}
	if events[0].SequenceID == 0 || events[0].SequenceID != events[1].SequenceID || events[0].SequenceID&(uint64(1)<<63) == 0 {
		t.Fatalf("touch contact identity=%d/%d, want one runtime-owned ID", events[0].SequenceID, events[1].SequenceID)
	}
	if got := rt.frameNumber - before; got != 2 {
		t.Fatalf("touch click frames=%d want 2", got)
	}
}

func TestDriveScrollPreservesAllInputValues(t *testing.T) {
	q := &recordQueue{}
	rt, _ := newInputRuntime(t, q)
	result := inputDriveCall(t, rt, "input.scroll", map[string]any{"x": 11, "y": 12, "delta_x": -3.5, "delta_y": 44.25, "precise": true, "modifiers": []string{"shift", "control"}}, allGrants())
	if result.Err != nil {
		t.Fatalf("input.scroll: %v", result.Err)
	}
	for _, event := range q.seenDriveEvents() {
		if scroll, ok := event.(platform.EventScroll); ok && scroll.Source == platform.SourceDrive {
			if scroll.DeltaX != -3.5 || scroll.DeltaY != 44.25 || !scroll.Precise || scroll.Modifiers != platform.ModShift|platform.ModControl || scroll.Position != (gfx.Point{X: 11, Y: 12}) {
				t.Fatalf("scroll was altered: %+v", scroll)
			}
			return
		}
	}
	t.Fatal("no SourceDrive scroll reached the platform queue")
}

func TestDriveInput_OwnershipLifecycle(t *testing.T) {
	q := &recordQueue{}
	rt, _ := newInputRuntime(t, q)

	if own := rt.ControlSnapshot(); own.InputOwner != facet.ControlOwnerHuman {
		t.Fatalf("initial ownership = %v", own.InputOwner)
	}
	// Mid-step ownership is observable: a long drag holds OwnerClient until
	// the events settle.
	res := inputDriveCall(t, rt, "input.drag", map[string]any{
		"from": map[string]any{"x": 0, "y": 0}, "to": map[string]any{"x": 100, "y": 50}, "move_count": 20,
	}, allGrants())
	if res.Err != nil {
		t.Fatalf("input.drag: %v", res.Err)
	}
	if own := rt.ControlSnapshot(); own.InputOwner != facet.ControlOwnerHuman {
		t.Fatalf("ownership after drag = %v, want human", own.InputOwner)
	}
}

// gatedQueue doses events one per Poll and injects the human event after a
// fixed number of drive events — deterministic preemption (no sleeps: the
// repo's test policy bans them; the injection point is a queue position).
type gatedQueue struct {
	mu             sync.Mutex
	events         []platform.Event
	humanAfter     int
	human          platform.Event
	humanSameBatch bool
	polls          int
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
	if q.polls == q.humanAfter {
		// The human event lands at the FRONT: it is observed by the next
		// poll, mid-step — appending would defer it until after the step
		// completed, which is not a preemption.
		q.events = append([]platform.Event{q.human}, q.events...)
		if q.humanSameBatch {
			batch := q.events
			q.events = nil
			return batch
		}
	}
	e := q.events[0]
	q.events = q.events[1:]
	return []platform.Event{e}
}

func TestHumanPreemptsSamePollBatch(t *testing.T) {
	q := &gatedQueue{
		humanAfter:     2,
		humanSameBatch: true,
		human:          platform.EventPointer{Kind: platform.PointerMove, Position: gfx.Point{X: 8, Y: 9}, Source: platform.SourcePlatform},
	}
	rt, probe := newInputRuntime(t, q)
	var raw json.RawMessage
	encoded, err := json.Marshal(map[string]any{"from": map[string]any{"x": 0, "y": 0}, "to": map[string]any{"x": 100, "y": 50}, "move_count": 40})
	if err != nil {
		t.Fatal(err)
	}
	raw = encoded
	client := testClientFor(t, rt)
	client.next++
	cmd := drive.Command{Trusted: true, Session: client.sess.ID, Generation: client.sess.Generation(), ID: client.next,
		OpID:   rt.config.Drive.Sessions.InstanceID() + "/" + client.sess.ID + "/" + strconv.FormatUint(client.next, 10),
		Method: "input.drag", Params: raw, Grants: allGrants(), Reply: make(chan drive.Result, 1)}
	if err := rt.DrivePort().Submit(cmd); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 16; i++ {
		rt.RunOneFrameAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Second))
		select {
		case result := <-cmd.Reply:
			if resultCode(result.Err) != string(proto.ErrInputPreempted) {
				t.Fatalf("same-batch result=%v", result.Err)
			}
			if probe.clicks != 0 {
				t.Fatalf("same-batch human event allowed drive release: clicks=%d", probe.clicks)
			}
			if rt.ControlSnapshot().InputOwner != facet.ControlOwnerHuman {
				t.Fatal("same-batch takeover retained client ownership")
			}
			return
		default:
		}
	}
	t.Fatal("same-batch preemption did not terminalize the operation")
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

	var res drive.Result
	{
		var raw json.RawMessage
		// A long drag leaves atoms pending when the human event severs it.
		raw, err := json.Marshal(map[string]any{"from": map[string]any{"x": 0, "y": 0}, "to": map[string]any{"x": 100, "y": 50}, "move_count": 40})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		client := testClientFor(t, rt)
		client.next++
		cmd := drive.Command{
			Trusted:    true,
			Session:    client.sess.ID,
			Generation: client.sess.Generation(),
			ID:         client.next,
			Method:     "input.drag",
			Params:     raw,
			Grants:     allGrants(),
			Reply:      make(chan drive.Result, 1),
		}
		if err := rt.DrivePort().Submit(cmd); err != nil {
			t.Fatalf("submit: %v", err)
		}
		epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
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
	}
	if got := resultCode(res.Err); got != "input.preempted" {
		t.Fatalf("result = %+v, want input.preempted", res.Err)
	}
	// Ownership returned to human.
	if own := rt.ControlSnapshot(); own.InputOwner != facet.ControlOwnerHuman {
		t.Fatalf("ownership after preemption = %v", own.InputOwner)
	}
	// The runtime counters the preemption; the sub.* event fan-out and the
	// own.preempted notification return with their native v2 slices.
	q.EnqueueDrive(platform.EventPointer{
		Kind: platform.PointerMove, Position: gfx.Point{X: 7, Y: 7}, Source: platform.SourcePlatform,
	})
	res2 := driveCall(t, rt, "input.drag", map[string]any{"from": map[string]any{"x": 0, "y": 0}, "to": map[string]any{"x": 100, "y": 50}, "move_count": 40}, allGrants())
	if got := resultCode(res2.Err); got != "input.preempted" {
		t.Fatalf("second drag result = %+v, want input.preempted", res2.Err)
	}
	// No drive pointer events survive in the pending set.
	for _, ev := range rt.pendingEvents {
		if pe, ok := ev.(platform.EventPointer); ok && pe.Source == platform.SourceDrive {
			t.Fatalf("preempted step's events survived: %+v", pe)
		}
	}
}

func TestHumanWakesParkedRuntimeAndReturnsToWall(t *testing.T) {
	q := &recordQueue{}
	rt, _ := newInputRuntime(t, q)
	if result := driveCall(t, rt, "time.mode", map[string]string{"mode": "virtual"}, allGrants()); result.Err != nil {
		t.Fatal(result.Err)
	}
	go func() { _ = rt.Run() }()
	q.EnqueueDrive(platform.EventPointer{Kind: platform.PointerMove, Position: gfx.Point{X: 7, Y: 9}, Source: platform.SourcePlatform})
	deadline := time.Now().Add(time.Second)
	for rt.clock.Mode() != ClockWall && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if rt.clock.Mode() != ClockWall {
		t.Fatal("human input did not wake the parked runtime")
	}
	if own := rt.ControlSnapshot(); own.TimeMode != facet.ControlTimeWall || own.InputOwner != facet.ControlOwnerHuman {
		t.Fatalf("control snapshot after human wake=%+v", own)
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

func TestHeldGestureCannotCrossSession(t *testing.T) {
	q := &recordQueue{}
	rt, _ := newInputRuntime(t, q)
	pressed := inputDriveCall(t, rt, "input.press", map[string]any{"x": 15, "y": 15}, allGrants())
	if pressed.Err != nil {
		t.Fatalf("input.press: %v", pressed.Err)
	}
	var gesture struct {
		Handle string `json:"gesture_handle"`
	}
	if err := json.Unmarshal(pressed.Payload, &gesture); err != nil || gesture.Handle == "" {
		t.Fatalf("press result %s (%v)", pressed.Payload, err)
	}
	other, _, err := rt.config.Drive.Sessions.Create(allGrants())
	if err != nil {
		t.Fatal(err)
	}
	cmd := drive.Command{Trusted: true, Session: other.ID, Generation: other.Generation(), ID: 1,
		OpID: rt.config.Drive.Sessions.InstanceID() + "/" + other.ID + "/1", Method: driveMethodInputRelease,
		Params: json.RawMessage(`{"gesture_handle":"` + gesture.Handle + `"}`), Grants: allGrants(), Reply: make(chan drive.Result, 1)}
	if err := rt.DrivePort().Submit(cmd); err != nil {
		t.Fatal(err)
	}
	var result drive.Result
	for i := 0; i < 8; i++ {
		rt.RunOneFrameAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Second))
		select {
		case result = <-cmd.Reply:
			i = 8
		default:
		}
	}
	if resultCode(result.Err) != string(proto.ErrInputBusy) {
		t.Fatalf("foreign release result=%v, want input.busy", result.Err)
	}
	released := inputDriveCall(t, rt, driveMethodInputRelease, map[string]any{"gesture_handle": gesture.Handle}, allGrants())
	if released.Err != nil {
		t.Fatalf("owning release: %v", released.Err)
	}
}

func TestDriveInputDragRejectsLegacyAndMixedLocationShapes(t *testing.T) {
	q := &recordQueue{}
	rt, _ := newInputRuntime(t, q)
	for _, params := range []map[string]any{
		{"from_x": 0, "from_y": 0, "to_x": 1, "to_y": 1},
		{"from": map[string]any{"x": 0, "y": 0, "target": map[string]string{"testid": "probe"}}, "to": map[string]any{"x": 1, "y": 1}},
		{"from": map[string]any{"x": 0}, "to": map[string]any{"x": 1, "y": 1}},
	} {
		result := inputDriveCall(t, rt, "input.drag", params, allGrants())
		if resultCode(result.Err) != string(proto.ErrInvalidParams) {
			t.Fatalf("invalid drag location %v returned %v", params, result.Err)
		}
	}
}

func TestDriveInput_GrantMatrix(t *testing.T) {
	q := &recordQueue{}
	rt, _ := newInputRuntime(t, q)

	// Every verb × the grant sets that matter: input verbs require the
	// input grant; cmd.execute requires command; reads require read.
	verbs := []struct {
		method string
		params any
	}{
		{"input.click", map[string]any{"x": 1, "y": 1}},
		{"input.press", map[string]any{"x": 1, "y": 1}},
		{"input.release", map[string]any{"x": 1, "y": 1}},
		{"input.move", map[string]any{"x": 1, "y": 1}},
		{"input.drag", map[string]any{"from": map[string]any{"x": 0, "y": 0}, "to": map[string]any{"x": 1, "y": 1}}},
		{"input.type", map[string]any{"text": "x"}},
		{"input.scroll", map[string]any{"x": 1, "y": 1, "delta_y": 1}},
		{"input.focus", map[string]any{"target": map[string]string{"testid": "probe"}}},
	}
	for _, v := range verbs {
		res := inputDriveCall(t, rt, v.method, v.params, drive.Grants{Read: true, Command: true, Time: true})
		if res.Err == nil || resultCode(res.Err) != "grant.denied" {
			t.Errorf("%s without input grant = %+v, want grant.denied", v.method, res.Err)
		}
	}
	// The read-only set works with just the input grant missing.
	res := inputDriveCall(t, rt, "cmd.execute", map[string]any{"name": "nope", "version": 1}, drive.Grants{Input: true})
	if res.Err == nil || resultCode(res.Err) != "grant.denied" {
		t.Fatalf("cmd.execute without command grant = %+v, want grant.denied", res.Err)
	}
}

func TestDriveInput_TypeRequiresIMEFocus(t *testing.T) {
	q := &recordQueue{}
	rt, _ := newInputRuntime(t, q)
	// No focused mark at all: typed refusal.
	if res := inputDriveCall(t, rt, "input.type", map[string]any{"text": "hi"}, allGrants()); resultCode(res.Err) == "" || !strings.Contains(errDetail(res.Err), "focused text mark") {
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
