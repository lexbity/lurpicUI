//go:build linux && cgo

package vulkan

import (
	"errors"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/render/vulkan/internal"
)

func TestIsUnsupported_detection(t *testing.T) {
	err := internal.TranslateResult(internal.ResultUnsupported, "no ICD")
	if !vulkanIsUnsupportedHelper(err) {
		t.Fatalf("expected IsUnsupported to return true for UnsupportedError, got %v", err)
	}
	if vulkanIsUnsupportedHelper(errors.New("some other error")) {
		t.Fatal("expected IsUnsupported to return false for generic error")
	}
	if vulkanIsUnsupportedHelper(nil) {
		t.Fatal("expected IsUnsupported to return false for nil")
	}
}

// vulkanIsUnsupportedHelper calls the package-level IsUnsupported function.
func vulkanIsUnsupportedHelper(err error) bool {
	return IsUnsupported(err)
}

func TestFFIResultTranslation(t *testing.T) {
	if err := BuildRustLibrary(); err != nil {
		t.Fatalf("BuildRustLibrary: %v", err)
	}
	if err := testResetRustState(); err != nil {
		t.Fatalf("testResetRustState: %v", err)
	}

	if err := testResultOK(); err != nil {
		t.Fatalf("testResultOK: %v", err)
	}

	err := testResultError()
	var initErr *internal.InitFailedError
	if !errors.As(err, &initErr) {
		t.Fatalf("expected InitFailedError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "simulated initialization failure") {
		t.Fatalf("unexpected error message: %v", err)
	}

	err = testResultPanic()
	var panicErr *internal.PanicError
	if !errors.As(err, &panicErr) {
		t.Fatalf("expected PanicError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "simulated boundary panic") {
		t.Fatalf("unexpected panic message: %v", err)
	}
}

func TestFFIHandleRegistry(t *testing.T) {
	if err := BuildRustLibrary(); err != nil {
		t.Fatalf("BuildRustLibrary: %v", err)
	}
	if err := testResetRustState(); err != nil {
		t.Fatalf("testResetRustState: %v", err)
	}

	baselineDestroy, err := testDestroyCount()
	if err != nil {
		t.Fatalf("testDestroyCount: %v", err)
	}
	baselineDrop, err := testDropCount()
	if err != nil {
		t.Fatalf("testDropCount: %v", err)
	}

	handle, err := testHandleCreate()
	if err != nil {
		t.Fatalf("testHandleCreate: %v", err)
	}
	if handle == 0 {
		t.Fatal("expected non-zero handle")
	}

	if err := testHandleUse(handle); err != nil {
		t.Fatalf("testHandleUse(valid): %v", err)
	}

	invalidErr := testHandleUse(internal.Handle(0xdeadbeef))
	if !internal.IsInvalidHandle(invalidErr) {
		t.Fatalf("expected invalid handle error, got %T: %v", invalidErr, invalidErr)
	}
	if err := testResultOK(); err != nil {
		t.Fatalf("testResultOK(clear invalid state): %v", err)
	}

	if got, err := testDestroyCount(); err != nil {
		t.Fatalf("testDestroyCount(before destroy): %v", err)
	} else if got != baselineDestroy {
		t.Fatalf("destroy count before explicit destroy = %d, want %d", got, baselineDestroy)
	}
	if got, err := testDropCount(); err != nil {
		t.Fatalf("testDropCount(before destroy): %v", err)
	} else if got != baselineDrop {
		t.Fatalf("drop count before explicit destroy = %d, want %d", got, baselineDrop)
	}

	if err := testHandleDestroy(handle); err != nil {
		t.Fatalf("testHandleDestroy: %v", err)
	}

	if err := testHandleUse(handle); !internal.IsInvalidHandle(err) {
		t.Fatalf("expected destroyed handle to be invalid, got %T: %v", err, err)
	}
	if err := testResultOK(); err != nil {
		t.Fatalf("testResultOK(clear destroyed state): %v", err)
	}

	if got, err := testDestroyCount(); err != nil {
		t.Fatalf("testDestroyCount(after destroy): %v", err)
	} else if got != baselineDestroy+1 {
		t.Fatalf("destroy count after explicit destroy = %d, want %d", got, baselineDestroy+1)
	}
	if got, err := testDropCount(); err != nil {
		t.Fatalf("testDropCount(after destroy): %v", err)
	} else if got != baselineDrop {
		t.Fatalf("drop count after explicit destroy = %d, want %d", got, baselineDrop)
	}
}
