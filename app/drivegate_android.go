//go:build android

package app

import "codeburg.org/lexbit/lurpicui/platform/android"

// androidDriveGate gates LDP enablement on Android builds (LDP §3.8): the
// platform bridge carries the decision — debuggable builds (lurpicdrive
// build tag, set by the lurpic CLI) may drive; release builds refuse at
// startup with the stub's unavailable error.
func androidDriveGate() error {
	if android.DriveGateEnabled() {
		return nil
	}
	return android.DriveGateError()
}
