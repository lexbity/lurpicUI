package linux

import "codeburg.org/lexbit/lurpicui/platform"

//nolint:unused
func testTranslateKeyPress(a *app) []platform.Event {
	return a.translateEvent(&testEvent{kind: testEventKeyPress, detail: 38, state: 1})
}

//nolint:unused
func testTranslatePointerButton(a *app) []platform.Event {
	return testTranslatePointerButtonWithDetail(a, 1, true)
}

//nolint:unused
func testTranslatePointerButtonWithDetail(a *app, detail uint8, press bool) []platform.Event {
	kind := testEventButtonRelease
	if press {
		kind = testEventButtonPress
	}
	return a.translateEvent(&testEvent{
		kind:   kind,
		detail: detail,
		eventX: 11,
		eventY: 22,
		state:  2,
	})
}

//nolint:unused
func testTranslateMotion(a *app) []platform.Event {
	return a.translateEvent(&testEvent{kind: testEventMotion, eventX: 33, eventY: 44})
}

//nolint:unused
func testTranslateEnterLeave(a *app, enter bool) []platform.Event {
	kind := testEventLeave
	if enter {
		kind = testEventEnter
	}
	return a.translateEvent(&testEvent{kind: kind, eventX: 5, eventY: 6, state: 7})
}

//nolint:unused
func testTranslateFocus(a *app, windowID uint32, focused bool) []platform.Event {
	kind := testEventFocusOut
	if focused {
		kind = testEventFocusIn
	}
	return a.translateEvent(&testEvent{kind: kind, window: windowID})
}

//nolint:unused
func testTranslateConfigure(a *app, windowID uint32, width, height uint16) []platform.Event {
	return a.translateEvent(&testEvent{kind: testEventConfigure, window: windowID, width: width, height: height})
}

//nolint:unused
func testTranslateClientMessage(a *app, windowID uint32) []platform.Event {
	return a.translateEvent(&testEvent{kind: testEventClientMessage, window: windowID, data32: a.atomWMDelete})
}

//nolint:unused
func testTranslateClientMessageWithData(a *app, windowID uint32, data uint32) []platform.Event {
	return a.translateEvent(&testEvent{kind: testEventClientMessage, window: windowID, data32: data})
}

//nolint:unused
func makeUnknownEvent() *testEvent {
	return &testEvent{kind: 0x7f}
}
