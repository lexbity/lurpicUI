//go:build android

package android

/*
#cgo LDFLAGS: -landroid
#include <stdlib.h>
#include <android/asset_manager.h>

// bridge_apkasset_open opens a bundled APK asset for streaming reads.
static AAsset* bridge_apkasset_open(AAssetManager* mgr, const char* path) {
    return AAssetManager_open(mgr, path, AASSET_MODE_STREAMING);
}

// bridge_apkasset_length returns the total length of the asset.
static int64_t bridge_apkasset_length(AAsset* asset) {
    return (int64_t)AAsset_getLength(asset);
}

// bridge_apkasset_read reads up to count bytes from the asset.
static int64_t bridge_apkasset_read(AAsset* asset, void* buf, int64_t count) {
    return (int64_t)AAsset_read(asset, buf, (size_t)count);
}

// bridge_apkasset_close closes the asset.
static void bridge_apkasset_close(AAsset* asset) {
    AAsset_close(asset);
}
*/
import "C"

import (
	"fmt"
	"io"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/platform/android/internal/bridge"
)

// apkAsset implements assets.APKAsset as a streaming reader over a native
// Android AAsset handle.
type apkAsset struct {
	handle *C.AAsset
	length int64
}

func (a *apkAsset) Read(p []byte) (int, error) {
	if a == nil || a.handle == nil {
		return 0, fmt.Errorf("apk asset: closed")
	}
	if len(p) == 0 {
		return 0, nil
	}
	n := int64(C.bridge_apkasset_read(a.handle, unsafe.Pointer(&p[0]), C.int64_t(len(p))))
	if n < 0 {
		return 0, fmt.Errorf("apk asset: read error")
	}
	if n == 0 {
		return 0, io.EOF
	}
	return int(n), nil
}

func (a *apkAsset) Close() error {
	if a != nil && a.handle != nil {
		C.bridge_apkasset_close(a.handle)
		a.handle = nil
	}
	return nil
}

func (a *apkAsset) Length() int64 {
	if a == nil {
		return 0
	}
	return a.length
}

// androidExtractionContext implements assets.AndroidExtractionContext using
// the Android NativeActivity's files directory and AAssetManager.
type androidExtractionContext struct {
	mgr     *C.AAssetManager
	storage *Storage
}

func (c *androidExtractionContext) FilesDir() string {
	if c == nil || c.storage == nil {
		return ""
	}
	return c.storage.FilesDir()
}

func (c *androidExtractionContext) CacheDir() string {
	if c == nil || c.storage == nil {
		return ""
	}
	return c.storage.CacheDir()
}

func (c *androidExtractionContext) OpenAPKAsset(name string) (assets.APKAsset, error) {
	if c == nil || c.mgr == nil {
		return nil, fmt.Errorf("apk asset manager not available")
	}
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	handle := C.bridge_apkasset_open(c.mgr, cName)
	if handle == nil {
		return nil, fmt.Errorf("apk asset not found: %s", name)
	}
	length := int64(C.bridge_apkasset_length(handle))
	return &apkAsset{handle: handle, length: length}, nil
}

func (c *androidExtractionContext) SetExtractionProgress(progress float32) {
	bridge.SetExtractionProgress(progress)
}

func (c *androidExtractionContext) OpenAPKAssetFD(name string) (fd int, offset int64, length int64, err error) {
	return bridge.OpenAPKAssetFD(name)
}

// ReadAPKAsset reads an entire APK asset file into memory. Returns nil + error
// when the asset is not found. This is the bootstrap-only path for small files
// (configs, UUID registry) needed before the runtime's Manager is available.
func ReadAPKAsset(name string) ([]byte, error) {
	amgr := bridge.GetAssetManager()
	if amgr == nil {
		return nil, fmt.Errorf("android: asset manager not available")
	}
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	handle := C.bridge_apkasset_open((*C.AAssetManager)(amgr), cName)
	if handle == nil {
		return nil, fmt.Errorf("android: asset not found: %s", name)
	}
	defer C.bridge_apkasset_close(handle)
	length := int64(C.bridge_apkasset_length(handle))
	if length <= 0 {
		return nil, fmt.Errorf("android: zero-length asset: %s", name)
	}
	buf := make([]byte, length)
	if length > 0 {
		n := int64(C.bridge_apkasset_read(handle, unsafe.Pointer(&buf[0]), C.int64_t(length)))
		if n <= 0 {
			return nil, fmt.Errorf("android: failed to read asset: %s", name)
		}
		buf = buf[:n]
	}
	return buf, nil
}

// OpenPlatformPak extracts assets.pak from the APK (if changed) and returns
// a PakFS AssetSource. Call this during Android boot before creating the
// runtime, then pass the result to NewManager.
//
// Returns nil without error when the APK has no assets.pak (asset-free app).
func OpenPlatformPak() (*assets.PakFS, error) {
	activity := bridge.GetActivity()
	if activity == nil {
		return nil, fmt.Errorf("android: no activity (boot not complete)")
	}
	amgr := bridge.GetAssetManager()
	if amgr == nil {
		return nil, fmt.Errorf("android: no asset manager")
	}
	ctx := &androidExtractionContext{
		mgr:     (*C.AAssetManager)(amgr),
		storage: NewStorage(activity),
	}
	// Probe for the pak — asset-free apps skip extraction.
	pak, err := assets.OpenAndroidPak(ctx)
	if err != nil {
		return nil, fmt.Errorf("android: open pak: %w", err)
	}
	return pak, nil
}
