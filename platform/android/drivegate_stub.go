//go:build android && !lurpicdrive

package android

import "errors"

// errDriveUnavailable is the stub's refusal: release Android builds never
// carry the lurpicdrive tag, so the drive seam is unavailable at runtime —
// no socket, no token, no listener.
var errDriveUnavailable = errors.New("drive unavailable: release build (lurpicdrive tag not set)")

// DriveGateEnabled reports whether LDP drive may be enabled on this Android
// build. The stub (no lurpicdrive tag) always refuses.
func DriveGateEnabled() bool { return false }

// DriveGateError returns the refusal reason.
func DriveGateError() error { return errDriveUnavailable }
