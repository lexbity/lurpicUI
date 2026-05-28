package testkit

import (
	"testing"

	"go.uber.org/goleak"
)

// CheckNoLeaks registers a cleanup on t that verifies no goroutines were
// leaked during the test. Call at the top of any test that creates goroutines.
func CheckNoLeaks(t testing.TB) {
	t.Helper()
	t.Cleanup(func() {
		goleak.VerifyNone(t)
	})
}
