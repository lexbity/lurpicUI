//go:build android

package app

import (
	"fmt"
	"os"

	"codeburg.org/lexbit/lurpicui/platform/android"
)

// #cgo LDFLAGS: -landroid
// #include <stdlib.h>
// #include <android/asset_manager.h>
// #include <android/asset_manager_jni.h>
import "C"

import "unsafe"

const maxBootstrapAssetSize = 1 << 20 // 1 MB

// Asset reads a bundled asset file from the APK's assets directory.
//
// **Bootstrap-only API.** Use this only for small files (configs, fonts)
// needed before the runtime's asset pipeline (Manager + PakFS) is
// available. All large media (images, textures, audio) must be loaded
// through the Manager via facet asset handles so they benefit from
// streaming, progressive LODs, cache budgeting, and eviction.
//
// On Android this reads from the APK via AAssetManager, bypassing the
// streaming/cache/budget system entirely. Files larger than
// MaxBootstrapAssetSize (1 MiB) produce a diagnostic warning on stderr.
func Asset(path string) ([]byte, error) {
	mgr := android.AssetManager()
	if mgr == nil {
		return nil, fmt.Errorf("asset manager not available")
	}

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	asset := C.AAssetManager_open((*C.AAssetManager)(mgr), cPath, C.AASSET_MODE_STREAMING)
	if asset == nil {
		return nil, fmt.Errorf("asset not found: %s", path)
	}
	defer C.AAsset_close(asset)

	length := C.AAsset_getLength(asset)
	if length < 0 {
		return nil, fmt.Errorf("failed to get asset length: %s", path)
	}

	buf := make([]byte, length)
	if length > 0 {
		read := C.AAsset_read(asset, unsafe.Pointer(&buf[0]), C.size_t(length))
		if read < 0 {
			return nil, fmt.Errorf("failed to read asset: %s", path)
		}
		buf = buf[:read]
	}

	if len(buf) > maxBootstrapAssetSize {
		fmt.Fprintf(os.Stderr, "app: Asset(%q) read %d bytes — large media should use the asset manager (Manager.LoadImage etc.)\n", path, len(buf))
	}

	return buf, nil
}
