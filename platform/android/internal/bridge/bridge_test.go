//go:build !android
// +build !android

package bridge

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/platform"
)

func TestEventQueue_PushAndPoll(t *testing.T) {
	q := NewEventQueue()

	// Initially empty
	events := q.Poll()
	if len(events) != 0 {
		t.Errorf("expected empty queue, got %d events", len(events))
	}

	// Push an event
	event := Event{Type: EventTypeStart}
	q.Push(event)

	// Poll should return the event
	events = q.Poll()
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventTypeStart {
		t.Errorf("expected EventTypeStart, got %v", events[0].Type)
	}

	// Queue should be empty again
	events = q.Poll()
	if len(events) != 0 {
		t.Errorf("expected empty queue after poll, got %d events", len(events))
	}
}

func TestEventQueue_Wait(t *testing.T) {
	q := NewEventQueue()

	// Push an event in background
	go func() {
		time.Sleep(10 * time.Millisecond)
		q.Push(Event{Type: EventTypeResume})
	}()

	// Wait should return when event is available
	events := q.Wait()
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventTypeResume {
		t.Errorf("expected EventTypeResume, got %v", events[0].Type)
	}
}

func TestEventQueue_Close(t *testing.T) {
	q := NewEventQueue()

	if q.IsClosed() {
		t.Error("new queue should not be closed")
	}

	q.Close()

	if !q.IsClosed() {
		t.Error("queue should be closed after Close()")
	}

	// Push should be no-op on closed queue
	q.Push(Event{Type: EventTypeStart})
	events := q.Poll()
	if len(events) != 0 {
		t.Error("push on closed queue should not add events")
	}
}

func TestEventQueue_MultipleEvents(t *testing.T) {
	q := NewEventQueue()

	// Push multiple events
	q.Push(Event{Type: EventTypeStart})
	q.Push(Event{Type: EventTypeResume})
	q.Push(Event{Type: EventTypePause})

	events := q.Poll()
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	// Check order is preserved
	if events[0].Type != EventTypeStart {
		t.Errorf("event 0: expected EventTypeStart, got %v", events[0].Type)
	}
	if events[1].Type != EventTypeResume {
		t.Errorf("event 1: expected EventTypeResume, got %v", events[1].Type)
	}
	if events[2].Type != EventTypePause {
		t.Errorf("event 2: expected EventTypePause, got %v", events[2].Type)
	}
}

func TestEventQueue_TouchEvent(t *testing.T) {
	q := NewEventQueue()

	event := Event{
		Type:      EventTypeTouch,
		PointerID: 1,
		Phase:     TouchDown,
		X:         100.5,
		Y:         200.5,
		Pressure:  0.8,
		Major:     10.0,
		Minor:     8.0,
	}
	q.Push(event)

	events := q.Poll()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Type != EventTypeTouch {
		t.Errorf("expected EventTypeTouch, got %v", e.Type)
	}
	if e.PointerID != 1 {
		t.Errorf("expected PointerID 1, got %d", e.PointerID)
	}
	if e.Phase != TouchDown {
		t.Errorf("expected TouchDown, got %v", e.Phase)
	}
	if e.X != 100.5 || e.Y != 200.5 {
		t.Errorf("expected position (100.5, 200.5), got (%f, %f)", e.X, e.Y)
	}
}

func TestGetEventQueue_Singleton(t *testing.T) {
	// Get the global queue twice
	q1 := GetEventQueue()
	q2 := GetEventQueue()

	// Should be the same instance
	if q1 != q2 {
		t.Error("GetEventQueue should return the same instance")
	}
}

func TestInit(t *testing.T) {
	// Init should not panic
	Init()
}

func TestTouchEventExtendedFields(t *testing.T) {
	q := NewEventQueue()

	event := Event{
		Type:        EventTypeTouch,
		PointerID:   42,
		Phase:       TouchDown,
		X:           100.0,
		Y:           200.0,
		Pressure:    0.75,
		Major:       12.0,
		Minor:       10.0,
		Source:      0x1002,   // AINPUT_SOURCE_TOUCHSCREEN
		DeviceID:    1,
		ToolType:    1,        // AMOTION_EVENT_TOOL_TYPE_FINGER
		ButtonState: 0,
		EventTime:   1234567890,
	}
	q.Push(event)

	events := q.Poll()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Type != EventTypeTouch {
		t.Errorf("expected EventTypeTouch, got %v", e.Type)
	}
	if e.PointerID != 42 {
		t.Errorf("expected PointerID 42, got %d", e.PointerID)
	}
	if e.Phase != TouchDown {
		t.Errorf("expected TouchDown, got %v", e.Phase)
	}
	if e.X != 100.0 || e.Y != 200.0 {
		t.Errorf("expected (100, 200), got (%f, %f)", e.X, e.Y)
	}
	if e.Pressure != 0.75 {
		t.Errorf("expected Pressure 0.75, got %f", e.Pressure)
	}
	if e.Source != 0x1002 {
		t.Errorf("expected Source 0x1002 (touchscreen), got %d", e.Source)
	}
	if e.DeviceID != 1 {
		t.Errorf("expected DeviceID 1, got %d", e.DeviceID)
	}
	if e.ToolType != 1 {
		t.Errorf("expected ToolType 1 (finger), got %d", e.ToolType)
	}
	if e.EventTime != 1234567890 {
		t.Errorf("expected EventTime 1234567890, got %d", e.EventTime)
	}
}

func TestKeyEventFields(t *testing.T) {
	q := NewEventQueue()

	event := Event{
		Type:      EventTypeKey,
		KeyCode:   61,  // AKEYCODE_SPACE
		Action:    0,    // AKEY_EVENT_ACTION_DOWN
		MetaState: 1,    // AMETA_SHIFT_ON
		Key:       platform.KeySpace,
		Modifiers: platform.ModShift,
		Source:    0x101, // AINPUT_SOURCE_KEYBOARD
		DeviceID:  2,
		EventTime: 987654321,
	}
	q.Push(event)

	events := q.Poll()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Type != EventTypeKey {
		t.Errorf("expected EventTypeKey, got %v", e.Type)
	}
	if e.KeyCode != 61 {
		t.Errorf("expected KeyCode 61 (SPACE), got %d", e.KeyCode)
	}
	if e.Action != 0 {
		t.Errorf("expected Action 0 (DOWN), got %d", e.Action)
	}
	if e.Source != 0x101 {
		t.Errorf("expected Source 0x101 (keyboard), got %d", e.Source)
	}
	if e.DeviceID != 2 {
		t.Errorf("expected DeviceID 2, got %d", e.DeviceID)
	}
	if e.Key != platform.KeySpace {
		t.Errorf("expected KeySpace, got %v", e.Key)
	}
	if e.Modifiers != platform.ModShift {
		t.Errorf("expected ModShift, got %v", e.Modifiers)
	}
}

func TestTouchCancelPhase(t *testing.T) {
	q := NewEventQueue()

	q.Push(Event{Type: EventTypeTouch, PointerID: 1, Phase: TouchDown, X: 10, Y: 20})
	q.Push(Event{Type: EventTypeTouch, PointerID: 1, Phase: TouchCancel, X: 10, Y: 20})

	events := q.Poll()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Phase != TouchDown {
		t.Errorf("event 0: expected TouchDown, got %v", events[0].Phase)
	}
	if events[1].Phase != TouchCancel {
		t.Errorf("event 1: expected TouchCancel, got %v", events[1].Phase)
	}
}

func TestWindowInsetsEvent_fields(t *testing.T) {
	q := NewEventQueue()

	q.Push(Event{
		Type:         EventTypeWindowInsets,
		InsetTop:     100,  // status bar
		InsetBottom:  168,  // nav bar on gesture nav
		InsetLeft:    0,
		InsetRight:   0,
		CutoutLeft:   0,
		CutoutTop:    80,   // display cutout (notch)
		CutoutRight:  0,
		CutoutBottom: 0,
	})

	events := q.Poll()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Type != EventTypeWindowInsets {
		t.Errorf("expected EventTypeWindowInsets, got %v", e.Type)
	}
	if e.InsetTop != 100 {
		t.Errorf("expected InsetTop=100, got %d", e.InsetTop)
	}
	if e.InsetBottom != 168 {
		t.Errorf("expected InsetBottom=168, got %d", e.InsetBottom)
	}
	if e.InsetLeft != 0 || e.InsetRight != 0 {
		t.Errorf("expected zero horizontal insets, got left=%d right=%d", e.InsetLeft, e.InsetRight)
	}
	if e.CutoutTop != 80 {
		t.Errorf("expected CutoutTop=80, got %d", e.CutoutTop)
	}
	if e.CutoutLeft != 0 || e.CutoutRight != 0 || e.CutoutBottom != 0 {
		t.Errorf("expected zero cutout for other sides, got L=%d R=%d B=%d",
			e.CutoutLeft, e.CutoutRight, e.CutoutBottom)
	}
}

func TestSavedState_roundTrip(t *testing.T) {
	ClearSavedState()
	data := GetSavedState()
	if data != nil {
		t.Fatal("expected nil before any save")
	}

	testData := []byte("test view state data")
	SetSavedState(testData)

	retrieved := GetSavedState()
	if string(retrieved) != string(testData) {
		t.Fatalf("expected %q, got %q", string(testData), string(retrieved))
	}

	ClearSavedState()
	if cleared := GetSavedState(); cleared != nil {
		t.Fatal("expected nil after clear")
	}
}

func TestSavedState_emptyDataIsNoOp(t *testing.T) {
	ClearSavedState()
	SetSavedState([]byte{})
	if data := GetSavedState(); data != nil {
		t.Fatal("expected nil for empty data")
	}
}

func TestSavedState_concurrentAccess(t *testing.T) {
	ClearSavedState()
	done := make(chan struct{})
	go func() {
		SetSavedState([]byte("concurrent"))
		_ = GetSavedState()
		done <- struct{}{}
	}()
	go func() {
		SetSavedState([]byte("access"))
		_ = GetSavedState()
		done <- struct{}{}
	}()
	<-done
	<-done
	// Should not race or panic.
}

func TestPermissionResult_routing(t *testing.T) {
	q := NewEventQueue()

	// Permission result should be delivered as a side-effect through the
	// registered handler, not as a bridge event. But we can verify the
	// bridge correctly pushes the nativePermissionResult callback.
	// The actual routing is tested in the permissions_test.go via the
	// SetPermissionResultHandler mechanism.
	_ = q
}

func TestVsyncEvent_fields(t *testing.T) {
	q := NewEventQueue()

	q.Push(Event{Type: EventTypeVsync, FrameTimeNanos: 1000000000}) // 1 second
	q.Push(Event{Type: EventTypeVsync, FrameTimeNanos: 1016666666}) // ~16.6ms later (60 Hz)

	events := q.Poll()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].FrameTimeNanos != 1000000000 {
		t.Errorf("expected FrameTimeNanos=1000000000, got %d", events[0].FrameTimeNanos)
	}
	if events[1].FrameTimeNanos != 1016666666 {
		t.Errorf("expected FrameTimeNanos=1016666666, got %d", events[1].FrameTimeNanos)
	}
}

func TestAudioFocusEvent_fields(t *testing.T) {
	q := NewEventQueue()

	q.Push(Event{Type: EventTypeAudioFocusChange, FocusChange: -2}) // AUDIOFOCUS_LOSS_TRANSIENT
	q.Push(Event{Type: EventTypeAudioFocusChange, FocusChange: 1})  // AUDIOFOCUS_GAIN
	q.Push(Event{Type: EventTypeAudioFocusChange, FocusChange: -3}) // AUDIOFOCUS_LOSS_TRANSIENT_CAN_DUCK

	events := q.Poll()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	if events[0].FocusChange != -2 {
		t.Errorf("expected FocusChange=-2 (transient loss), got %d", events[0].FocusChange)
	}
	if events[1].FocusChange != 1 {
		t.Errorf("expected FocusChange=1 (gain), got %d", events[1].FocusChange)
	}
	if events[2].FocusChange != -3 {
		t.Errorf("expected FocusChange=-3 (can duck), got %d", events[2].FocusChange)
	}
}

func TestConfigurationChangedEvent_fields(t *testing.T) {
	q := NewEventQueue()

	q.Push(Event{
		Type:          EventTypeConfigurationChanged,
		Orientation:   2, // ACONFIGURATION_ORIENTATION_LAND
		ScreenWidthDp: 800,
		ScreenHeightDp: 480,
		Density:       320, // ACONFIGURATION_DENSITY_XHIGH
		UiModeNight:   2,   // ACONFIGURATION_UI_MODE_NIGHT_YES
		FontScale:     1.25,
		Language:      "fr",
		Country:       "CA",
	})

	events := q.Poll()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Type != EventTypeConfigurationChanged {
		t.Errorf("expected EventTypeConfigurationChanged, got %v", e.Type)
	}
	if e.Orientation != 2 {
		t.Errorf("expected Orientation=2 (landscape), got %d", e.Orientation)
	}
	if e.ScreenWidthDp != 800 {
		t.Errorf("expected ScreenWidthDp=800, got %d", e.ScreenWidthDp)
	}
	if e.ScreenHeightDp != 480 {
		t.Errorf("expected ScreenHeightDp=480, got %d", e.ScreenHeightDp)
	}
	if e.Density != 320 {
		t.Errorf("expected Density=320, got %d", e.Density)
	}
	if e.UiModeNight != 2 {
		t.Errorf("expected UiModeNight=2 (night), got %d", e.UiModeNight)
	}
	if e.FontScale != 1.25 {
		t.Errorf("expected FontScale=1.25, got %f", e.FontScale)
	}
	if e.Language != "fr" {
		t.Errorf("expected Language='fr', got %q", e.Language)
	}
	if e.Country != "CA" {
		t.Errorf("expected Country='CA', got %q", e.Country)
	}
}

func TestIMEComposeCommit_orderingPreserved(t *testing.T) {
	q := NewEventQueue()

	// Simulate a typical IME composition session:
	// 1. Begin composing "hel"
	// 2. Update composition to "hell"
	// 3. Keyboard key press (hardware key interleaved)
	// 4. Update composition to "hello"
	// 5. Commit "hello"
	// 6. Keyboard key press (Enter)
	q.Push(Event{Type: EventTypeIMECompose, Text: "hel", CursorPos: 3})
	q.Push(Event{Type: EventTypeIMECompose, Text: "hell", CursorPos: 4})
	q.Push(Event{Type: EventTypeKey, KeyCode: 62, Action: 0}) // hardware key interleaved
	q.Push(Event{Type: EventTypeIMECompose, Text: "hello", CursorPos: 5})
	q.Push(Event{Type: EventTypeIMECommit, Text: "hello"})
	q.Push(Event{Type: EventTypeKey, KeyCode: 66, Action: 0}) // Enter key

	events := q.Poll()
	if len(events) != 6 {
		t.Fatalf("expected 6 events, got %d", len(events))
	}

	expected := []struct {
		typ  EventType
		text string
	}{
		{EventTypeIMECompose, "hel"},
		{EventTypeIMECompose, "hell"},
		{EventTypeKey, ""},
		{EventTypeIMECompose, "hello"},
		{EventTypeIMECommit, "hello"},
		{EventTypeKey, ""},
	}
	for i, exp := range expected {
		if events[i].Type != exp.typ {
			t.Errorf("event %d: expected Type %v, got %v", i, exp.typ, events[i].Type)
		}
		if exp.text != "" && events[i].Text != exp.text {
			t.Errorf("event %d: expected Text %q, got %q", i, exp.text, events[i].Text)
		}
	}
}

func TestIMEKeyEvent_hardwareAndSoft_useSamePath(t *testing.T) {
	q := NewEventQueue()

	// IME sendKeyEvent (from soft keyboard action keys)
	q.Push(Event{Type: EventTypeKey, KeyCode: 66, Action: 0, Key: platform.KeyEnter, Source: 0x101})
	// Hardware key event (from physical keyboard)
	q.Push(Event{Type: EventTypeKey, KeyCode: 66, Action: 0, Key: platform.KeyEnter, Source: 0x101, DeviceID: 1})

	events := q.Poll()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != EventTypeKey || events[1].Type != EventTypeKey {
		t.Fatal("both events should be EventTypeKey")
	}
	if events[0].KeyCode != 66 || events[1].KeyCode != 66 {
		t.Fatal("both events should have KeyCode 66 (Enter)")
	}
}

func TestIMEAction_performEditorAction(t *testing.T) {
	q := NewEventQueue()

	// Simulate performEditorAction for IME_ACTION_DONE → KEYCODE_ENTER (DOWN + UP)
	q.Push(Event{Type: EventTypeKey, KeyCode: 66, Action: 0, Key: platform.KeyEnter})
	q.Push(Event{Type: EventTypeKey, KeyCode: 66, Action: 1, Key: platform.KeyEnter})

	// Simulate performEditorAction for IME_ACTION_NEXT → KEYCODE_TAB (DOWN + UP)
	q.Push(Event{Type: EventTypeKey, KeyCode: 61, Action: 0, Key: platform.KeyTab})
	q.Push(Event{Type: EventTypeKey, KeyCode: 61, Action: 1, Key: platform.KeyTab})

	events := q.Poll()
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	// Done → Enter (DOWN)
	if events[0].KeyCode != 66 || events[0].Action != 0 {
		t.Errorf("event 0: expected KeyCode=66 (Enter) Action=0, got KeyCode=%d Action=%d", events[0].KeyCode, events[0].Action)
	}
	// Done → Enter (UP)
	if events[1].KeyCode != 66 || events[1].Action != 1 {
		t.Errorf("event 1: expected KeyCode=66 (Enter) Action=1, got KeyCode=%d Action=%d", events[1].KeyCode, events[1].Action)
	}
	// Next → Tab (DOWN)
	if events[2].KeyCode != 61 || events[2].Action != 0 {
		t.Errorf("event 2: expected KeyCode=61 (Tab) Action=0, got KeyCode=%d Action=%d", events[2].KeyCode, events[2].Action)
	}
	// Next → Tab (UP)
	if events[3].KeyCode != 61 || events[3].Action != 1 {
		t.Errorf("event 3: expected KeyCode=61 (Tab) Action=1, got KeyCode=%d Action=%d", events[3].KeyCode, events[3].Action)
	}
}

func TestIMEInterleaving_composeAndHardwareKey(t *testing.T) {
	q := NewEventQueue()

	// User types "a" via IME, then presses hardware arrow key, then types "b" via IME.
	// Order must be preserved.
	q.Push(Event{Type: EventTypeIMECompose, Text: "a", CursorPos: 1})
	q.Push(Event{Type: EventTypeIMECommit, Text: "a"})
	q.Push(Event{Type: EventTypeKey, KeyCode: 21, Action: 0, Key: platform.KeyLeft})  // hardware left arrow
	q.Push(Event{Type: EventTypeKey, KeyCode: 21, Action: 1, Key: platform.KeyLeft})
	q.Push(Event{Type: EventTypeIMECompose, Text: "b", CursorPos: 1})
	q.Push(Event{Type: EventTypeIMECommit, Text: "b"})

	events := q.Poll()
	if len(events) != 6 {
		t.Fatalf("expected 6 events, got %d", len(events))
	}

	// Verify order: compose(a), commit(a), key(LEFT_DOWN), key(LEFT_UP), compose(b), commit(b)
	types := make([]EventType, 6)
	texts := make([]string, 6)
	for i, e := range events {
		types[i] = e.Type
		texts[i] = e.Text
	}
	expectedTypes := []EventType{
		EventTypeIMECompose, EventTypeIMECommit,
		EventTypeKey, EventTypeKey,
		EventTypeIMECompose, EventTypeIMECommit,
	}
	for i, et := range expectedTypes {
		if types[i] != et {
			t.Errorf("event %d: expected Type %v, got %v", i, et, types[i])
		}
	}
	if texts[0] != "a" || texts[1] != "a" || texts[4] != "b" || texts[5] != "b" {
		t.Errorf("unexpected text sequence: %v", texts)
	}
}

func TestMultiTouchPointerIDs(t *testing.T) {
	q := NewEventQueue()

	// Simulate two-finger gesture: finger 0 down, finger 1 down, both move, finger 1 up, finger 0 up
	q.Push(Event{Type: EventTypeTouch, PointerID: 0, Phase: TouchDown, X: 100, Y: 200})
	q.Push(Event{Type: EventTypeTouch, PointerID: 1, Phase: TouchDown, X: 300, Y: 400})
	q.Push(Event{Type: EventTypeTouch, PointerID: 0, Phase: TouchMove, X: 110, Y: 210})
	q.Push(Event{Type: EventTypeTouch, PointerID: 1, Phase: TouchMove, X: 290, Y: 390})
	q.Push(Event{Type: EventTypeTouch, PointerID: 1, Phase: TouchUp, X: 285, Y: 385})
	q.Push(Event{Type: EventTypeTouch, PointerID: 0, Phase: TouchUp, X: 115, Y: 215})

	events := q.Poll()
	if len(events) != 6 {
		t.Fatalf("expected 6 events, got %d", len(events))
	}

	expected := []struct {
		pointerID int32
		phase     TouchPhase
	}{
		{0, TouchDown},
		{1, TouchDown},
		{0, TouchMove},
		{1, TouchMove},
		{1, TouchUp},
		{0, TouchUp},
	}
	for i, exp := range expected {
		if events[i].PointerID != exp.pointerID {
			t.Errorf("event %d: expected PointerID %d, got %d", i, exp.pointerID, events[i].PointerID)
		}
		if events[i].Phase != exp.phase {
			t.Errorf("event %d: expected Phase %v, got %v", i, exp.phase, events[i].Phase)
		}
}
}
