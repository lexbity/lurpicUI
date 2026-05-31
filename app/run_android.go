//go:build android

package app

import (
	"fmt"
	"os"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/platform/android"
	"codeburg.org/lexbit/lurpicui/runtime"
)

// On Android the platform App is the NativeActivity-backed bridge.
var newPlatformApp = android.NewApp

func init() {
	initAssetManager = openAndroidAssetManager
}

func openAndroidAssetManager(rtConfig *runtime.Config) {
	if rtConfig.AssetManager != nil {
		return
	}
	pak, err := android.OpenPlatformPak()
	if err != nil {
		fmt.Fprintf(os.Stderr, "app/android: open platform pak: %v\n", err)
		return
	}
	reg := assets.NewAssetRegistryStore()
	// Load the UUID registry from the APK's assets when available.
	// The cook pipeline produces uuid_registry.json alongside assets.pak.
	idReg := loadAndroidIDRegistry()
	rtConfig.AssetManager = assets.NewManager(reg, pak, assets.BackendSoftware, nil, idReg)
	rtConfig.AssetRegistry = reg
}

// loadAndroidIDRegistry reads the UUID registry from the APK's assets.
// Returns nil when the file is not bundled (development builds without cook).
func loadAndroidIDRegistry() assets.PathIDRegistry {
	data, err := android.ReadAPKAsset("uuid_registry.json")
	if err != nil {
		return nil
	}
	reg, err := assets.ParseJSONPathRegistry(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "app/android: invalid uuid_registry.json: %v\n", err)
		return nil
	}
	return reg
}
