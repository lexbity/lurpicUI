//go:build android && cgo
// +build android

package android

import (
	"errors"
	"strings"
	"sync"

	"codeburg.org/lexbit/lurpicui/platform/android/internal/bridge"
)

// Permission identifies an Android runtime permission.
type Permission string

const (
	PermissionCamera              Permission = "android.permission.CAMERA"
	PermissionMicrophone          Permission = "android.permission.RECORD_AUDIO"
	PermissionStorage             Permission = "android.permission.READ_EXTERNAL_STORAGE"
	PermissionPostNotifications   Permission = "android.permission.POST_NOTIFICATIONS"
	PermissionLocationFine        Permission = "android.permission.ACCESS_FINE_LOCATION"
	PermissionLocationCoarse      Permission = "android.permission.ACCESS_COARSE_LOCATION"
)

// PermissionResult reports the current state of a runtime permission request.
type PermissionResult int

const (
	PermissionGranted PermissionResult = iota
	PermissionDenied
	PermissionDeniedPermanent
)

// DeniedCallback is called when a permission request is denied. The permanent
// flag indicates the system will not prompt again. Apps should use this to
// show rationale UI or degrade functionality gracefully.
type DeniedCallback func(permission Permission, permanent bool)

type pendingRequest struct {
	ch         chan PermissionResult
	permission Permission
}

var (
	pendingPermissionMu  sync.Mutex
	pendingPermissions   = make(map[int32]pendingRequest)
	nextPermissionCode   int32 = 1
	deniedHooksMu        sync.RWMutex
	deniedHooks          []DeniedCallback
)

func init() {
	bridge.SetPermissionResultHandler(handlePermissionResult)
}

// OnPermissionDenied registers a callback invoked when any permission request
// is denied. Multiple callbacks may be registered; they are called in order.
func OnPermissionDenied(cb DeniedCallback) {
	if cb == nil {
		return
	}
	deniedHooksMu.Lock()
	deniedHooks = append(deniedHooks, cb)
	deniedHooksMu.Unlock()
}

// RequestLocationWithPrecision requests location permission at the specified
// precision level. Coarse (city-block) requires ACCESS_COARSE_LOCATION; fine
// (meter-level) requires ACCESS_FINE_LOCATION. If the precise permission is
// denied but coarse would be granted, the function falls back to coarse
// automatically.
func RequestLocationWithPrecision(precise bool) <-chan PermissionResult {
	result := make(chan PermissionResult, 1)
	if precise {
		ch, err := RequestPermission(PermissionLocationFine)
		if err != nil {
			result <- PermissionDenied
			close(result)
			return result
		}
		go func() {
			r := <-ch
			if r == PermissionDenied || r == PermissionDeniedPermanent {
				// Fall back to coarse location.
				coarseCh, cerr := RequestPermission(PermissionLocationCoarse)
				if cerr != nil {
					result <- PermissionDenied
				} else {
					result <- <-coarseCh
				}
			} else {
				result <- r
			}
			close(result)
		}()
		return result
	}
	ch, err := RequestPermission(PermissionLocationCoarse)
	if err != nil {
		result <- PermissionDenied
		close(result)
		return result
	}
	go func() {
		result <- <-ch
		close(result)
	}()
	return result
}

// RequestPermission asks Android to grant the supplied runtime permission.
// The returned channel receives exactly one result when the OS responds.
func RequestPermission(permission Permission) (<-chan PermissionResult, error) {
	permission = Permission(strings.TrimSpace(string(permission)))
	if permission == "" {
		return nil, errors.New("android: permission is required")
	}
	if !bridge.IsPermissionDeclared(string(permission)) {
		return nil, errors.New("android: permission not declared in manifest")
	}
	ch := make(chan PermissionResult, 1)
	if ok, err := CheckPermission(permission); err == nil && ok == PermissionGranted {
		ch <- PermissionGranted
		close(ch)
		return ch, nil
	}
	requestCode := nextPermissionRequestCode()
	pendingPermissionMu.Lock()
	pendingPermissions[requestCode] = pendingRequest{ch: ch, permission: permission}
	pendingPermissionMu.Unlock()
	if err := bridge.RequestPermission(string(permission), requestCode); err != nil {
		pendingPermissionMu.Lock()
		delete(pendingPermissions, requestCode)
		pendingPermissionMu.Unlock()
		close(ch)
		return nil, err
	}
	return ch, nil
}

// CheckPermission reports the current permission state without prompting.
func CheckPermission(permission Permission) (PermissionResult, error) {
	permission = Permission(strings.TrimSpace(string(permission)))
	if permission == "" {
		return PermissionDenied, errors.New("android: permission is required")
	}
	if !bridge.IsPermissionDeclared(string(permission)) {
		return PermissionDenied, errors.New("android: permission not declared in manifest")
	}
	if bridge.CheckPermission(string(permission)) {
		return PermissionGranted, nil
	}
	return PermissionDenied, nil
}

func nextPermissionRequestCode() int32 {
	pendingPermissionMu.Lock()
	defer pendingPermissionMu.Unlock()
	code := nextPermissionCode
	nextPermissionCode++
	if nextPermissionCode <= 0 {
		nextPermissionCode = 1
	}
	return code
}

func handlePermissionResult(requestCode int32, granted bool, permanent bool) {
	pendingPermissionMu.Lock()
	pReq := pendingPermissions[requestCode]
	delete(pendingPermissions, requestCode)
	pendingPermissionMu.Unlock()

	result := PermissionDenied
	switch {
	case granted:
		result = PermissionGranted
	case permanent:
		result = PermissionDeniedPermanent
	}

	// Invoke denied callbacks on denial with the permission name.
	if !granted {
		deniedHooksMu.RLock()
		hooks := make([]DeniedCallback, len(deniedHooks))
		copy(hooks, deniedHooks)
		deniedHooksMu.RUnlock()
		for _, hook := range hooks {
			hook(pReq.permission, permanent)
		}
	}

	if pReq.ch == nil {
		return
	}
	pReq.ch <- result
	close(pReq.ch)
}
