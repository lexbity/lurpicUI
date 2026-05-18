//go:build !android
// +build !android

package android

import (
	"errors"

	"codeburg.org/lexbit/lurpicui/platform"
)

type Permission string

const (
	PermissionCamera     Permission = "android.permission.CAMERA"
	PermissionMicrophone Permission = "android.permission.RECORD_AUDIO"
	PermissionStorage    Permission = "android.permission.READ_EXTERNAL_STORAGE"
)

type PermissionResult int

const (
	PermissionGranted PermissionResult = iota
	PermissionDenied
	PermissionDeniedPermanent
)

// NewApp returns an error on non-Android platforms.
func NewApp() (platform.App, error) {
	return nil, errors.New("android platform: not yet implemented")
}

func RequestPermission(permission Permission) (<-chan PermissionResult, error) {
	return nil, errors.New("android platform: not available on this platform")
}

func CheckPermission(permission Permission) (PermissionResult, error) {
	return PermissionDenied, errors.New("android platform: not available on this platform")
}
