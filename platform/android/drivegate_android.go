//go:build android && lurpicdrive

package android

// DriveGateEnabled reports whether LDP drive may be enabled on this Android
// build (LDP §3.8): the lurpicdrive build tag is set by the lurpic CLI for
// debuggable builds only.
func DriveGateEnabled() bool { return true }

// DriveGateError returns nil when drive is available.
func DriveGateError() error { return nil }
