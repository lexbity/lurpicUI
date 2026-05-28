package store

// projectionActiveFunc is set by the runtime to detect store mutations during
// the projection phase. Any mutation while projecting violates the frame
// pipeline order (Principle 9) and must panic.
var projectionActiveFunc func() bool

// SetProjectionActiveCheck installs the function that reports whether the
// projection phase is currently executing. Called once at runtime startup.
func SetProjectionActiveCheck(fn func() bool) {
	projectionActiveFunc = fn
}

// assertNotProjecting panics with a descriptive message if a store mutation
// is attempted during the projection phase.
func assertNotProjecting() {
	if projectionActiveFunc != nil && projectionActiveFunc() {
		panic("store: mutation during projection phase — violates frame pipeline order (Principle 9)")
	}
}
