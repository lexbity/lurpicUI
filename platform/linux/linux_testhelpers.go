package linux

import "codeburg.org/lexbit/lurpicui/platform"

func testTranslateKeyPress(a *app) []platform.Event {
	return a.translateEvent(&testEvent{kind: testEventKeyPress, detail: 38, state: 1})
}

func testTranslatePointerButton(a *app) []platform.Event {
	return testTranslatePointerButtonWithDetail(a, 1, true)
}

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

func testTranslateMotion(a *app) []platform.Event {
	return a.translateEvent(&testEvent{kind: testEventMotion, eventX: 33, eventY: 44})
}

func testTranslateEnterLeave(a *app, enter bool) []platform.Event {
	kind := testEventLeave
	if enter {
		kind = testEventEnter
	}
	return a.translateEvent(&testEvent{kind: kind, eventX: 5, eventY: 6, state: 7})
}

func testTranslateFocus(a *app, windowID uint32, focused bool) []platform.Event {
	kind := testEventFocusOut
	if focused {
		kind = testEventFocusIn
	}
	return a.translateEvent(&testEvent{kind: kind, window: windowID})
}

func testTranslateConfigure(a *app, windowID uint32, width, height uint16) []platform.Event {
	return a.translateEvent(&testEvent{kind: testEventConfigure, window: windowID, width: width, height: height})
}

func testTranslateClientMessage(a *app, windowID uint32) []platform.Event {
	return a.translateEvent(&testEvent{kind: testEventClientMessage, window: windowID, data32: uint32(a.atomWMDelete)})
}

func testTranslateClientMessageWithData(a *app, windowID uint32, data uint32) []platform.Event {
	return a.translateEvent(&testEvent{kind: testEventClientMessage, window: windowID, data32: data})
}

func testHandleSelectionRequest(a *app, windowID uint32) {
	testHandleSelectionRequestWithProperty(a, windowID, 0)
}

func testHandleSelectionRequestWithProperty(a *app, windowID uint32, property uint32) {
	_ = property
	_ = windowID
}

func makeUnknownEvent() *testEvent {
	return &testEvent{kind: 0x7f}
}
