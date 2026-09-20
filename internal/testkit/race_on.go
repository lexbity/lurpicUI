//go:build race

package testkit

// RaceEnabled reports whether the binary was built with the race detector.
// See race_off.go for the alive-coverage suite's use.
const RaceEnabled = true