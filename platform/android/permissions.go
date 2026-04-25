//go:build android
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
	PermissionCamera     Permission = "android.permission.CAMERA"
	PermissionMicrophone Permission = "android.permission.RECORD_AUDIO"
	PermissionStorage    Permission = "android.permission.READ_EXTERNAL_STORAGE"
)

// PermissionResult reports the current state of a runtime permission request.
type PermissionResult int

const (
	PermissionGranted PermissionResult = iota
	PermissionDenied
	PermissionDeniedPermanent
)

var (
	pendingPermissionMu sync.Mutex
	pendingPermissions        = make(map[int32]chan PermissionResult)
	nextPermissionCode  int32 = 1
)

func init() {
	bridge.SetPermissionResultHandler(handlePermissionResult)
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
	pendingPermissions[requestCode] = ch
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
	ch := pendingPermissions[requestCode]
	delete(pendingPermissions, requestCode)
	pendingPermissionMu.Unlock()
	if ch == nil {
		return
	}
	result := PermissionDenied
	switch {
	case granted:
		result = PermissionGranted
	case permanent:
		result = PermissionDeniedPermanent
	}
	ch <- result
	close(ch)
}
