package android

import (
	"unsafe"

	"codeburg.org/lexbit/lurpicui/platform/android/internal/bridge"
)

// AssetManager returns the native AAssetManager* captured from the Android
// NativeActivity, as an unsafe.Pointer for JNI asset access. It returns nil
// before the activity has been created or on non-Android platforms.
//
// This is the public accessor through which packages outside platform/android
// (such as app) reach the asset manager, since platform/android/internal/bridge
// is not importable across the internal boundary.
func AssetManager() unsafe.Pointer {
	return bridge.GetAssetManager()
}
