//go:build !android

package app

// androidDriveGate is a no-op off Android: desktop drive enablement is
// governed by LURPIC_DRIVE alone (QP-4).
func androidDriveGate() error { return nil }
