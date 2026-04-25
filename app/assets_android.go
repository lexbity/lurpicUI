//go:build android
// +build android

package app

import (
	"fmt"
)

// #cgo LDFLAGS: -landroid
// #include <android/asset_manager.h>
// #include <android/asset_manager_jni.h>
import "C"

import (
	"unsafe"

	"codeburg.org/lexbit/lurpicui/platform/android/internal/bridge"
)

// Asset reads a bundled asset file from the APK's assets directory.
//
// On Android, this uses the native AssetManager to read files that were
// bundled into the APK during the build process. The asset path should be
// relative to the assets/ directory in the project.
//
// Example:
//
//	data, err := app.Asset("fonts/regular.ttf")
//	if err != nil {
//	    log.Fatal(err)
//	}
func Asset(path string) ([]byte, error) {
	// Get the AssetManager from the bridge
	mgr := bridge.GetAssetManager()
	if mgr == nil {
		return nil, fmt.Errorf("asset manager not available")
	}

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	// Open the asset
	asset := C.AAssetManager_open(mgr, cPath, C.AASSET_MODE_STREAMING)
	if asset == nil {
		return nil, fmt.Errorf("asset not found: %s", path)
	}
	defer C.AAsset_close(asset)

	// Get the length
	length := C.AAsset_getLength(asset)
	if length < 0 {
		return nil, fmt.Errorf("failed to get asset length: %s", path)
	}

	// Read the data
	buf := make([]byte, length)
	if length > 0 {
		read := C.AAsset_read(asset, unsafe.Pointer(&buf[0]), C.size_t(length))
		if read < 0 {
			return nil, fmt.Errorf("failed to read asset: %s", path)
		}
		// Adjust buffer to actual bytes read
		buf = buf[:read]
	}

	return buf, nil
}
