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
	if os.Getenv("LURPIC_SKIP_ANDROID_PAK") == "1" {
		fmt.Fprintln(os.Stderr, "app/android: skipping platform pak bootstrap via LURPIC_SKIP_ANDROID_PAK=1")
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
	backendType := assetBackendFromRuntimeConfig(rtConfig)
	rtConfig.AssetManager = assets.NewManager(reg, pak, backendType, nil, idReg)
	rtConfig.AssetRegistry = reg
}

// loadAndroidIDRegistry reads the UUID registry from the APK's assets. It
// returns nil when the file is missing or unparseable, but warns first: a nil
// registry means every path-based asset load resolves to an empty handle, so a
// missing uuid_registry.json must not be silent.
func loadAndroidIDRegistry() assets.PathIDRegistry {
	data, err := android.ReadAPKAsset("uuid_registry.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "app/android: uuid_registry.json not bundled (%v); "+
			"path-based asset lookups will return empty handles\n", err)
		return nil
	}
	reg, err := assets.ParseJSONPathRegistry(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "app/android: invalid uuid_registry.json: %v\n", err)
		return nil
	}
	return reg
}
