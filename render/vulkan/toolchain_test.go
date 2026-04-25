package vulkan

import (
	"strings"
	"testing"
)

func TestCheckRustToolchain_missingCargoReportsUsefulError(t *testing.T) {
	oldLookPath := lookPath
	lookPath = func(file string) (string, error) {
		return "", errLookPathMissing
	}
	t.Cleanup(func() {
		lookPath = oldLookPath
	})

	err := CheckRustToolchain()
	if err == nil {
		t.Fatal("expected toolchain check to fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "cargo") {
		t.Fatalf("expected error to mention cargo, got %q", err)
	}
}

var errLookPathMissing = &missingPathError{}

type missingPathError struct{}

func (*missingPathError) Error() string { return "missing" }
