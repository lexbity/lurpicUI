// Package display contains the cgo-backed Linux/X11 window, event, surface,
// and clipboard implementation used by the platform/linux backend.
//
// The XCB connection, epoll file descriptor, and shared-memory surfaces are
// owned here because they must be managed on the platform side and drained by
// the runtime through the platform.App event queue.
//
// Code outside platform/linux should not import this package.
package display
