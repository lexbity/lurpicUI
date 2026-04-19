//go:build linux && cgo

package linux

/*
#include <stdint.h>
#include <xcb/xcb.h>
#include <X11/keysym.h>
*/
import "C"

import (
	"unsafe"

	"codeburg.org/lexbit/lurpicui/platform"
)

func testTranslateKeyPress(a *app) []platform.Event {
	var keyPress C.xcb_key_press_event_t
	keyPress.response_type = C.XCB_KEY_PRESS
	keyPress.detail = 38
	keyPress.state = 1
	return a.translateEvent((*C.xcb_generic_event_t)(unsafe.Pointer(&keyPress)))
}

func testTranslatePointerButton(a *app) []platform.Event {
	return testTranslatePointerButtonWithDetail(a, 1, true)
}

func testTranslatePointerButtonWithDetail(a *app, detail uint8, press bool) []platform.Event {
	var button C.xcb_button_press_event_t
	if press {
		button.response_type = C.XCB_BUTTON_PRESS
	} else {
		button.response_type = C.XCB_BUTTON_RELEASE
	}
	button.detail = C.uint8_t(detail)
	button.event_x = 11
	button.event_y = 22
	button.state = 2
	return a.translateEvent((*C.xcb_generic_event_t)(unsafe.Pointer(&button)))
}

func testTranslateMotion(a *app) []platform.Event {
	var motion C.xcb_motion_notify_event_t
	motion.response_type = C.XCB_MOTION_NOTIFY
	motion.event_x = 33
	motion.event_y = 44
	return a.translateEvent((*C.xcb_generic_event_t)(unsafe.Pointer(&motion)))
}

func testTranslateEnterLeave(a *app, enter bool) []platform.Event {
	var ev C.xcb_enter_notify_event_t
	if enter {
		ev.response_type = C.XCB_ENTER_NOTIFY
	} else {
		ev.response_type = C.XCB_LEAVE_NOTIFY
	}
	ev.event_x = 5
	ev.event_y = 6
	ev.state = 7
	return a.translateEvent((*C.xcb_generic_event_t)(unsafe.Pointer(&ev)))
}

func testTranslateFocus(a *app, windowID uint32, focused bool) []platform.Event {
	var ev C.xcb_focus_in_event_t
	if focused {
		ev.response_type = C.XCB_FOCUS_IN
	} else {
		ev.response_type = C.XCB_FOCUS_OUT
	}
	ev.event = C.xcb_window_t(windowID)
	return a.translateEvent((*C.xcb_generic_event_t)(unsafe.Pointer(&ev)))
}

func testTranslateConfigure(a *app, windowID uint32, width, height uint16) []platform.Event {
	var ev C.xcb_configure_notify_event_t
	ev.response_type = C.XCB_CONFIGURE_NOTIFY
	ev.window = C.xcb_window_t(windowID)
	ev.width = C.uint16_t(width)
	ev.height = C.uint16_t(height)
	return a.translateEvent((*C.xcb_generic_event_t)(unsafe.Pointer(&ev)))
}

func testTranslateClientMessage(a *app, windowID uint32) []platform.Event {
	return testTranslateClientMessageWithData(a, windowID, uint32(a.atomWMDelete))
}

func testTranslateClientMessageWithData(a *app, windowID uint32, data uint32) []platform.Event {
	var ev C.xcb_client_message_event_t
	ev.response_type = C.XCB_CLIENT_MESSAGE
	ev.window = C.xcb_window_t(windowID)
	ev.format = 32
	(*[5]uint32)(unsafe.Pointer(&ev.data))[0] = data
	return a.translateEvent((*C.xcb_generic_event_t)(unsafe.Pointer(&ev)))
}

func testHandleSelectionRequest(a *app, windowID uint32) {
	testHandleSelectionRequestWithProperty(a, windowID, C.XCB_ATOM_NONE)
}

func testHandleSelectionRequestWithProperty(a *app, windowID uint32, property C.xcb_atom_t) {
	var ev C.xcb_selection_request_event_t
	ev.target = a.atomUTF8String
	ev.selection = a.atomClipboard
	ev.requestor = C.xcb_window_t(windowID)
	ev.property = property
	a.handleSelectionRequest((*C.xcb_generic_event_t)(unsafe.Pointer(&ev)))
}

func makeUnknownEvent() *C.xcb_generic_event_t {
	var ev C.xcb_generic_event_t
	ev.response_type = 0x7f
	return &ev
}
