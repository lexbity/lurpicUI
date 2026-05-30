package internal

import "fmt"

// ResultCode mirrors the Rust-side RenderResult enum.
type ResultCode int32

const (
	ResultOK            ResultCode = 0
	ResultInitFailed    ResultCode = 1
	ResultOutOfMemory   ResultCode = 2
	ResultInvalidHandle ResultCode = 3
	ResultVulkanError   ResultCode = 4
	ResultUnsupported   ResultCode = 5
	ResultPanic         ResultCode = 1000
	ResultUnknown       ResultCode = 1001
)

// Handle is the opaque Rust-side resource identifier.
type Handle uint64

type ResultError interface {
	error
	ResultCode() ResultCode
}

type baseResultError struct {
	code    ResultCode
	message string
}

func (e baseResultError) Error() string {
	if e.message == "" {
		return fmt.Sprintf("vulkan: %s", e.code.String())
	}
	return fmt.Sprintf("vulkan: %s: %s", e.code.String(), e.message)
}

func (e baseResultError) ResultCode() ResultCode { return e.code }

type InitFailedError struct{ baseResultError }
type OutOfMemoryError struct{ baseResultError }
type InvalidHandleError struct{ baseResultError }
type UnsupportedError struct{ baseResultError }
type VulkanError struct{ baseResultError }
type PanicError struct{ baseResultError }
type UnknownError struct{ baseResultError }

func (c ResultCode) String() string {
	switch c {
	case ResultOK:
		return "ok"
	case ResultInitFailed:
		return "init_failed"
	case ResultOutOfMemory:
		return "out_of_memory"
	case ResultInvalidHandle:
		return "invalid_handle"
	case ResultUnsupported:
		return "unsupported"
	case ResultVulkanError:
		return "vulkan_error"
	case ResultPanic:
		return "panic"
	default:
		return "unknown"
	}
}

// TranslateResult maps a Rust status code plus the Rust-side error message into
// a typed Go error.
func TranslateResult(code ResultCode, message string) error {
	switch code {
	case ResultOK:
		return nil
	case ResultInitFailed:
		return &InitFailedError{baseResultError{code: code, message: message}}
	case ResultOutOfMemory:
		return &OutOfMemoryError{baseResultError{code: code, message: message}}
	case ResultInvalidHandle:
		return &InvalidHandleError{baseResultError{code: code, message: message}}
	case ResultUnsupported:
		return &UnsupportedError{baseResultError{code: code, message: message}}
	case ResultVulkanError:
		return &VulkanError{baseResultError{code: code, message: message}}
	case ResultPanic:
		return &PanicError{baseResultError{code: code, message: message}}
	default:
		return &UnknownError{baseResultError{code: code, message: message}}
	}
}

func ErrorCode(err error) (ResultCode, bool) {
	switch e := err.(type) {
	case *InitFailedError:
		return e.code, true
	case *OutOfMemoryError:
		return e.code, true
	case *InvalidHandleError:
		return e.code, true
	case *UnsupportedError:
		return e.code, true
	case *VulkanError:
		return e.code, true
	case *PanicError:
		return e.code, true
	case *UnknownError:
		return e.code, true
	case interface{ ResultCode() ResultCode }:
		return e.ResultCode(), true
	default:
		return 0, false
	}
}

func IsInvalidHandle(err error) bool {
	code, ok := ErrorCode(err)
	return ok && code == ResultInvalidHandle
}

func IsUnsupported(err error) bool {
	code, ok := ErrorCode(err)
	return ok && code == ResultUnsupported
}
