//go:build !android
// +build !android

package android

import (
	"errors"

	"codeburg.org/lexbit/lurpicui/platform"
)

type Permission string

const (
	PermissionCamera              Permission = "android.permission.CAMERA"
	PermissionMicrophone          Permission = "android.permission.RECORD_AUDIO"
	PermissionStorage             Permission = "android.permission.READ_EXTERNAL_STORAGE"
	PermissionPostNotifications   Permission = "android.permission.POST_NOTIFICATIONS"
	PermissionLocationFine        Permission = "android.permission.ACCESS_FINE_LOCATION"
	PermissionLocationCoarse      Permission = "android.permission.ACCESS_COARSE_LOCATION"
)

type PermissionResult int

const (
	PermissionGranted PermissionResult = iota
	PermissionDenied
	PermissionDeniedPermanent
)

// DeniedCallback is called when a permission is denied.
type DeniedCallback func(permission Permission, permanent bool)

// NewApp returns an error on non-Android platforms.
func NewApp() (platform.App, error) {
	return nil, errors.New("android platform: not yet implemented")
}

func OnPermissionDenied(cb DeniedCallback) {}

func RequestPermission(permission Permission) (<-chan PermissionResult, error) {
	return nil, errors.New("android platform: not available on this platform")
}

func CheckPermission(permission Permission) (PermissionResult, error) {
	return PermissionDenied, errors.New("android platform: not available on this platform")
}

func RequestLocationWithPrecision(precise bool) <-chan PermissionResult {
	ch := make(chan PermissionResult, 1)
	ch <- PermissionDenied
	close(ch)
	return ch
}
