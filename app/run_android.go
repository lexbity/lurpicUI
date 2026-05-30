//go:build android

package app

import "codeburg.org/lexbit/lurpicui/platform/android"

// On Android the platform App is the NativeActivity-backed bridge.
var newPlatformApp = android.NewApp
